package api

import (
	"net/http"
	"strings"

	"babywei-bakery/internal/domain"
	"babywei-bakery/internal/store"
)

func (s *Server) routeCRUD(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/purchases", s.listPurchases)
	mux.HandleFunc("POST /api/purchases", s.createPurchase)
	mux.HandleFunc("PATCH /api/purchases/{id}", s.updatePurchase)
	mux.HandleFunc("DELETE /api/purchases/{id}", s.deletePurchase)

	mux.HandleFunc("GET /api/doughs", s.listDoughs)
	mux.HandleFunc("POST /api/doughs", s.createDough)
	mux.HandleFunc("PATCH /api/doughs/{id}", s.updateDough)
	mux.HandleFunc("DELETE /api/doughs/{id}", s.deleteDough)

	mux.HandleFunc("GET /api/fillings", s.listFillings)
	mux.HandleFunc("POST /api/fillings", s.createFilling)
	mux.HandleFunc("PATCH /api/fillings/{id}", s.updateFilling)
	mux.HandleFunc("DELETE /api/fillings/{id}", s.deleteFilling)

	mux.HandleFunc("GET /api/products", s.listProducts)
	mux.HandleFunc("POST /api/products", s.createProduct)
	mux.HandleFunc("PATCH /api/products/{id}", s.updateProduct)
	mux.HandleFunc("DELETE /api/products/{id}", s.deleteProduct)
}

// --- 進貨 ---

func validatePurchase(p store.Purchase) string {
	switch {
	case strings.TrimSpace(p.Name) == "":
		return "請輸入材料名稱"
	case p.PurchaseDate == "":
		return "請選擇購入日期"
	case p.WeightG <= 0:
		return "進貨總重量必須大於 0"
	case p.Price < 0:
		return "進貨總價格不可為負數"
	}
	return ""
}

func (s *Server) listPurchases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	got, err := s.db.ListPurchases(q.Get("from"), q.Get("to"), q.Get("q"))
	if err != nil {
		writeStoreError(w, err, "查詢進貨紀錄失敗")
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) createPurchase(w http.ResponseWriter, r *http.Request) {
	var p store.Purchase
	if !decodeJSON(w, r, &p) {
		return
	}
	if msg := validatePurchase(p); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	got, err := s.db.CreatePurchase(p)
	if err != nil {
		writeStoreError(w, err, "新增進貨紀錄失敗")
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) updatePurchase(w http.ResponseWriter, r *http.Request) {
	var p store.Purchase
	if !decodeJSON(w, r, &p) {
		return
	}
	p.ID = r.PathValue("id")
	if msg := validatePurchase(p); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.db.UpdatePurchase(p); err != nil {
		writeStoreError(w, err, "更新進貨紀錄失敗")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deletePurchase(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeletePurchase(r.PathValue("id")); err != nil {
		writeStoreError(w, err, "刪除進貨紀錄失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- 產品配方 ---

func validateDough(g store.Dough) string {
	if strings.TrimSpace(g.Name) == "" {
		return "請輸入配方名稱"
	}
	if len(g.Ingredients) == 0 {
		return "配方至少需要一項材料"
	}
	for _, it := range g.Ingredients {
		if strings.TrimSpace(it.Name) == "" {
			return "材料名稱不可空白"
		}
		if it.Pct <= 0 {
			return "材料 " + it.Name + " 的百分比必須大於 0"
		}
	}
	return ""
}

func (s *Server) listDoughs(w http.ResponseWriter, r *http.Request) {
	got, err := s.db.ListDoughs()
	if err != nil {
		writeStoreError(w, err, "查詢產品配方失敗")
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) createDough(w http.ResponseWriter, r *http.Request) {
	var g store.Dough
	if !decodeJSON(w, r, &g) {
		return
	}
	if msg := validateDough(g); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	got, err := s.db.CreateDough(g)
	if err != nil {
		writeStoreError(w, err, "新增產品配方失敗")
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) updateDough(w http.ResponseWriter, r *http.Request) {
	var g store.Dough
	if !decodeJSON(w, r, &g) {
		return
	}
	g.ID = r.PathValue("id")
	if msg := validateDough(g); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.db.UpdateDough(g); err != nil {
		writeStoreError(w, err, "更新產品配方失敗")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) deleteDough(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeleteDough(r.PathValue("id")); err != nil {
		writeStoreError(w, err, "刪除產品配方失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- 配料 ---

func validateFilling(f store.Filling) string {
	if strings.TrimSpace(f.Name) == "" {
		return "請輸入配料名稱"
	}
	if len(f.Ingredients) == 0 {
		return "配料至少需要一項材料"
	}
	for _, it := range f.Ingredients {
		if strings.TrimSpace(it.Name) == "" {
			return "材料名稱不可空白"
		}
		if it.WeightG <= 0 {
			return "材料 " + it.Name + " 的重量必須大於 0"
		}
	}
	return ""
}

func (s *Server) listFillings(w http.ResponseWriter, r *http.Request) {
	got, err := s.db.ListFillings()
	if err != nil {
		writeStoreError(w, err, "查詢配料失敗")
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) createFilling(w http.ResponseWriter, r *http.Request) {
	var f store.Filling
	if !decodeJSON(w, r, &f) {
		return
	}
	if msg := validateFilling(f); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	got, err := s.db.CreateFilling(f)
	if err != nil {
		writeStoreError(w, err, "新增配料失敗")
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) updateFilling(w http.ResponseWriter, r *http.Request) {
	var f store.Filling
	if !decodeJSON(w, r, &f) {
		return
	}
	f.ID = r.PathValue("id")
	if msg := validateFilling(f); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.db.UpdateFilling(f); err != nil {
		writeStoreError(w, err, "更新配料失敗")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) deleteFilling(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeleteFilling(r.PathValue("id")); err != nil {
		writeStoreError(w, err, "刪除配料失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- 商品 ---

// productResponse 是商品加上計算欄位。成本不是 store 的欄位 ——
// 它由 domain 算出，在此組裝，不污染 store 的型別。
type productResponse struct {
	store.Product
	UnitCostTWD float64                       `json:"unitCostTwd"`
	Breakdown   []domain.ProductCostBreakdown `json:"breakdown"`
}

func validateProduct(p store.Product) string {
	switch {
	case strings.TrimSpace(p.Name) == "":
		return "請輸入商品名稱"
	case p.Price < 0:
		return "售價不可為負數"
	case p.DoughID == "":
		return "請選擇產品配方"
	case p.DoughWeightG <= 0:
		return "配方使用重量必須大於 0"
	case p.Fill1WeightG < 0 || p.Fill2WeightG < 0:
		return "配料重量不可為負數"
	case p.Fill1ID != "" && p.Fill1WeightG <= 0:
		return "已選擇配料 1，重量必須大於 0"
	case p.Fill2ID != "" && p.Fill2WeightG <= 0:
		return "已選擇配料 2，重量必須大於 0"
	}
	return ""
}

// costContext 一次載入計算成本所需的配方與單位成本表。
func (s *Server) costContext() (store.RecipeSet, map[string]float64, error) {
	recipes, err := s.db.RecipeSet()
	if err != nil {
		return store.RecipeSet{}, nil, err
	}
	purchases, err := s.db.ListPurchases("", "", "")
	if err != nil {
		return store.RecipeSet{}, nil, err
	}
	return recipes, domain.CostPerGram(purchases), nil
}

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	products, err := s.db.ListProducts()
	if err != nil {
		writeStoreError(w, err, "查詢商品失敗")
		return
	}
	recipes, costPerG, err := s.costContext()
	if err != nil {
		writeStoreError(w, err, "計算商品成本失敗")
		return
	}
	out := make([]productResponse, 0, len(products))
	for _, p := range products {
		out = append(out, productResponse{
			Product:     p,
			UnitCostTWD: domain.ProductUnitCost(p, recipes, costPerG),
			Breakdown:   domain.ProductBreakdown(p, recipes, costPerG),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createProduct(w http.ResponseWriter, r *http.Request) {
	var p store.Product
	if !decodeJSON(w, r, &p) {
		return
	}
	if msg := validateProduct(p); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	got, err := s.db.CreateProduct(p)
	if err != nil {
		writeStoreError(w, err, "新增商品失敗")
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) updateProduct(w http.ResponseWriter, r *http.Request) {
	var p store.Product
	if !decodeJSON(w, r, &p) {
		return
	}
	p.ID = r.PathValue("id")
	if msg := validateProduct(p); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.db.UpdateProduct(p); err != nil {
		writeStoreError(w, err, "更新商品失敗")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProduct(w http.ResponseWriter, r *http.Request) {
	if err := s.db.DeleteProduct(r.PathValue("id")); err != nil {
		writeStoreError(w, err, "刪除商品失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
