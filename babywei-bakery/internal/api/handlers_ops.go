package api

import (
	"fmt"
	"net/http"
	"time"

	"babywei-bakery/internal/domain"
	"babywei-bakery/internal/store"
)

func (s *Server) routeOps(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sales", s.listSales)
	mux.HandleFunc("POST /api/sales", s.createSale)
	mux.HandleFunc("DELETE /api/sales/{id}", s.deleteSale)

	mux.HandleFunc("POST /api/production/preview", s.previewProduction)
	mux.HandleFunc("POST /api/production", s.confirmProduction)
	mux.HandleFunc("GET /api/production", s.listProduction)

	mux.HandleFunc("GET /api/inventory", s.getInventory)
	mux.HandleFunc("GET /api/reports/summary", s.getSummary)
	mux.HandleFunc("GET /api/reports/sales", s.getSalesReport)
}

// today 回傳當下日期。所有預設日期都走這裡 —— 原型有兩處寫死 "2026-09-03"。
func today() string { return time.Now().Format("2006-01-02") }

// --- 出貨 ---

type saleRequest struct {
	SaleDate  string `json:"saleDate"`
	ProductID string `json:"productId"`
	Qty       int    `json:"qty"`
}

func (s *Server) listSales(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	got, err := s.db.ListSales(q.Get("from"), q.Get("to"))
	if err != nil {
		writeStoreError(w, err, "查詢出貨紀錄失敗")
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) createSale(w http.ResponseWriter, r *http.Request) {
	var req saleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Qty <= 0 {
		writeError(w, http.StatusBadRequest, "出貨數量必須大於 0")
		return
	}
	if req.SaleDate == "" {
		req.SaleDate = today()
	}
	p, err := s.db.GetProduct(req.ProductID)
	if err != nil {
		writeStoreError(w, err, "查詢商品失敗")
		return
	}
	recipes, costPerG, err := s.costContext()
	if err != nil {
		writeStoreError(w, err, "計算成本失敗")
		return
	}
	// 金額在此刻算好並寫死。之後改售價或改進貨價都不影響已成立的紀錄。
	got, err := s.db.CreateSale(store.Sale{
		SaleDate:    req.SaleDate,
		ProductID:   p.ID,
		ProductName: p.Name,
		Qty:         req.Qty,
		UnitCost:    domain.ProductUnitCost(p, recipes, costPerG),
		UnitPrice:   p.Price,
	})
	if err != nil {
		writeStoreError(w, err, "新增出貨紀錄失敗")
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) deleteSale(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeleteSale(r.PathValue("id")); err != nil {
		writeStoreError(w, err, "刪除出貨紀錄失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- 生產 ---

type productionRequest struct {
	LoggedDate string `json:"loggedDate"`
	ProductID  string `json:"productId"`
	Qty        int    `json:"qty"`
}

// productionScale 是換算結果，preview 與確認生產共用同一段計算。
type productionScale struct {
	Product     store.Product       `json:"product"`
	Qty         int                 `json:"qty"`
	UnitWeightG float64             `json:"unitWeightG"`
	DoughTotalG float64             `json:"doughTotalG"`
	FillTotalG  float64             `json:"fillTotalG"`
	Sections    []productionSection `json:"sections"`
	Consumption map[string]float64  `json:"consumption"`
}

type productionSection struct {
	Title string           `json:"title"`
	Unit  string           `json:"unit"` // pct | gram
	Items []productionItem `json:"items"`
}

type productionItem struct {
	Name   string  `json:"name"`
	Ratio  float64 `json:"ratio"`
	UsageG float64 `json:"usageG"`
}

// scaleProduction 算出一批生產的換算結果。preview 與確認共用，
// 差別只在呼叫端是否把 Consumption 寫入資料庫。
func (s *Server) scaleProduction(productID string, qty int) (productionScale, error) {
	p, err := s.db.GetProduct(productID)
	if err != nil {
		return productionScale{}, err
	}
	recipes, err := s.db.RecipeSet()
	if err != nil {
		return productionScale{}, fmt.Errorf("載入配方: %w", err)
	}
	q := float64(qty)

	out := productionScale{
		Product:     p,
		Qty:         qty,
		UnitWeightG: p.DoughWeightG + p.Fill1WeightG + p.Fill2WeightG,
		DoughTotalG: p.DoughWeightG * q,
		FillTotalG:  (p.Fill1WeightG + p.Fill2WeightG) * q,
		Consumption: domain.ProductConsumption(p, recipes, qty),
		Sections:    []productionSection{},
	}

	if g, ok := recipes.Doughs[p.DoughID]; ok {
		usage := domain.Scale(domain.FromDough(g.Ingredients), p.DoughWeightG*q)
		items := make([]productionItem, 0, len(g.Ingredients))
		for _, it := range g.Ingredients {
			items = append(items, productionItem{Name: it.Name, Ratio: it.Pct, UsageG: usage[it.Name]})
		}
		out.Sections = append(out.Sections, productionSection{
			Title: "產品配方｜" + g.Name, Unit: "pct", Items: items,
		})
	}
	for i, f := range []struct {
		id      string
		weightG float64
	}{{p.Fill1ID, p.Fill1WeightG}, {p.Fill2ID, p.Fill2WeightG}} {
		if f.id == "" {
			continue
		}
		fl, ok := recipes.Fillings[f.id]
		if !ok {
			continue
		}
		usage := domain.Scale(domain.FromFilling(fl.Ingredients), f.weightG*q)
		items := make([]productionItem, 0, len(fl.Ingredients))
		for _, it := range fl.Ingredients {
			items = append(items, productionItem{Name: it.Name, Ratio: it.WeightG, UsageG: usage[it.Name]})
		}
		out.Sections = append(out.Sections, productionSection{
			Title: fmt.Sprintf("配料 %d｜%s", i+1, fl.Name), Unit: "gram", Items: items,
		})
	}
	return out, nil
}

func (s *Server) previewProduction(w http.ResponseWriter, r *http.Request) {
	var req productionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Qty <= 0 {
		writeError(w, http.StatusBadRequest, "生產數量必須大於 0")
		return
	}
	got, err := s.scaleProduction(req.ProductID, req.Qty)
	if err != nil {
		writeStoreError(w, err, "換算生產配方失敗")
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) confirmProduction(w http.ResponseWriter, r *http.Request) {
	var req productionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Qty <= 0 {
		writeError(w, http.StatusBadRequest, "生產數量必須大於 0")
		return
	}
	if req.LoggedDate == "" {
		req.LoggedDate = today()
	}
	scaled, err := s.scaleProduction(req.ProductID, req.Qty)
	if err != nil {
		writeStoreError(w, err, "換算生產配方失敗")
		return
	}
	got, err := s.db.ConfirmProduction(store.ProductionLog{
		LoggedDate:  req.LoggedDate,
		ProductID:   scaled.Product.ID,
		ProductName: scaled.Product.Name,
		Qty:         req.Qty,
	}, scaled.Consumption)
	if err != nil {
		writeStoreError(w, err, "寫入生產紀錄失敗")
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) listProduction(w http.ResponseWriter, r *http.Request) {
	got, err := s.db.ListProductionLogs()
	if err != nil {
		writeStoreError(w, err, "查詢生產紀錄失敗")
		return
	}
	writeJSON(w, http.StatusOK, got)
}

// --- 庫存與報表 ---

func (s *Server) getInventory(w http.ResponseWriter, r *http.Request) {
	purchases, err := s.db.ListPurchases("", "", "")
	if err != nil {
		writeStoreError(w, err, "查詢進貨紀錄失敗")
		return
	}
	logs, err := s.db.ListProductionLogs()
	if err != nil {
		writeStoreError(w, err, "查詢生產紀錄失敗")
		return
	}
	writeJSON(w, http.StatusOK, domain.Inventory(purchases, logs))
}

func (s *Server) getSummary(w http.ResponseWriter, r *http.Request) {
	sales, err := s.db.ListSales("", "")
	if err != nil {
		writeStoreError(w, err, "查詢出貨紀錄失敗")
		return
	}
	writeJSON(w, http.StatusOK, domain.Summary(sales, time.Now()))
}

// salesReport 一併回傳明細與加總，讓加總邏輯只存在 domain 一處。
type salesReport struct {
	Sales  []store.Sale  `json:"sales"`
	Totals domain.Totals `json:"totals"`
}

func (s *Server) getSalesReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sales, err := s.db.ListSales(q.Get("from"), q.Get("to"))
	if err != nil {
		writeStoreError(w, err, "查詢出貨紀錄失敗")
		return
	}
	writeJSON(w, http.StatusOK, salesReport{Sales: sales, Totals: domain.SalesTotals(sales)})
}
