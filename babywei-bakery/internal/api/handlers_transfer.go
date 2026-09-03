package api

import (
	"fmt"
	"net/http"

	"babywei-bakery/internal/domain"
	"babywei-bakery/internal/store"
)

func (s *Server) routeTransfer(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/export/backup.json", s.exportBackup)
	mux.HandleFunc("POST /api/import", s.importLegacy)
}

func (s *Server) exportBackup(w http.ResponseWriter, r *http.Request) {
	snap, err := s.db.ExportAll()
	if err != nil {
		writeStoreError(w, err, "匯出資料失敗")
		return
	}
	w.Header().Set("Content-Disposition",
		`attachment; filename="babywei-backup-`+today()+`.json"`)
	writeJSON(w, http.StatusOK, snap)
}

// legacySnapshot 是原型 localStorage 鍵 babywei_local 的結構。
// 欄位名稱與新資料模型不同，對應關係見下方 toSnapshot。
type legacySnapshot struct {
	CostDB []struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		Brand        string  `json:"brand"`
		PurchaseDate string  `json:"purchaseDate"`
		Channel      string  `json:"channel"`
		Price        float64 `json:"price"`
		Weight       float64 `json:"weight"`
	} `json:"costDB"`
	Dough []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Ingredients []struct {
			Name string  `json:"name"`
			Pct  float64 `json:"pct"`
		} `json:"ingredients"`
	} `json:"dough"`
	Fillings []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Ingredients []struct {
			Name   string  `json:"name"`
			Weight float64 `json:"weight"`
		} `json:"ingredients"`
	} `json:"fillings"`
	Products []struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Price       float64 `json:"price"`
		DoughID     string  `json:"doughId"`
		DoughWeight float64 `json:"doughWeight"`
		Fill1ID     string  `json:"fill1Id"`
		Fill1Weight float64 `json:"fill1Weight"`
		Fill2ID     string  `json:"fill2Id"`
		Fill2Weight float64 `json:"fill2Weight"`
	} `json:"products"`
	Sales []struct {
		ID          string  `json:"id"`
		Date        string  `json:"date"`
		ProductID   string  `json:"productId"`
		ProductName string  `json:"productName"`
		Qty         int     `json:"qty"`
		UnitCost    float64 `json:"unitCost"`
		UnitPrice   float64 `json:"unitPrice"`
	} `json:"sales"`
	ProductionLogs []struct {
		ID          string `json:"id"`
		Date        string `json:"date"`
		ProductID   string `json:"productId"`
		ProductName string `json:"productName"`
		Qty         int    `json:"qty"`
	} `json:"productionLogs"`
}

// toSnapshot 把原型結構轉為新資料模型，並回報無法對應的項目。
//
// 原型的 productionLogs 沒有消耗明細，這裡以「匯入資料中的配方」回推並寫死。
// 這是一次性近似 —— 它還原不了當時的真實消耗，因此一律列入 Warnings。
func (l legacySnapshot) toSnapshot() (store.Snapshot, []string) {
	warnings := []string{}
	var s store.Snapshot

	for _, c := range l.CostDB {
		if c.Weight <= 0 {
			warnings = append(warnings,
				fmt.Sprintf("進貨「%s」重量為 %v，不符合大於 0 的限制，已略過", c.Name, c.Weight))
			continue
		}
		s.Purchases = append(s.Purchases, store.Purchase{
			ID: c.ID, Name: c.Name, Brand: c.Brand, PurchaseDate: c.PurchaseDate,
			Channel: c.Channel, Price: c.Price, WeightG: c.Weight,
		})
	}
	for _, g := range l.Dough {
		d := store.Dough{ID: g.ID, Name: g.Name}
		for _, it := range g.Ingredients {
			if it.Pct <= 0 {
				warnings = append(warnings,
					fmt.Sprintf("配方「%s」的材料「%s」百分比為 %v，已略過", g.Name, it.Name, it.Pct))
				continue
			}
			d.Ingredients = append(d.Ingredients, store.DoughItem{Name: it.Name, Pct: it.Pct})
		}
		s.Doughs = append(s.Doughs, d)
	}
	for _, f := range l.Fillings {
		fl := store.Filling{ID: f.ID, Name: f.Name}
		for _, it := range f.Ingredients {
			if it.Weight <= 0 {
				warnings = append(warnings,
					fmt.Sprintf("配料「%s」的材料「%s」重量為 %v，已略過", f.Name, it.Name, it.Weight))
				continue
			}
			fl.Ingredients = append(fl.Ingredients, store.FillingItem{Name: it.Name, WeightG: it.Weight})
		}
		s.Fillings = append(s.Fillings, fl)
	}
	for _, p := range l.Products {
		s.Products = append(s.Products, store.Product{
			ID: p.ID, Name: p.Name, Price: p.Price,
			DoughID: p.DoughID, DoughWeightG: p.DoughWeight,
			Fill1ID: p.Fill1ID, Fill1WeightG: p.Fill1Weight,
			Fill2ID: p.Fill2ID, Fill2WeightG: p.Fill2Weight,
		})
	}
	for _, sa := range l.Sales {
		s.Sales = append(s.Sales, store.Sale{
			ID: sa.ID, SaleDate: sa.Date, ProductID: sa.ProductID,
			ProductName: sa.ProductName, Qty: sa.Qty,
			UnitCost: sa.UnitCost, UnitPrice: sa.UnitPrice,
		})
	}

	// 回推消耗：用匯入資料自己的配方，而非資料庫現有的。
	recipes := store.RecipeSet{
		Doughs:   map[string]store.Dough{},
		Fillings: map[string]store.Filling{},
	}
	for _, d := range s.Doughs {
		recipes.Doughs[d.ID] = d
	}
	for _, f := range s.Fillings {
		recipes.Fillings[f.ID] = f
	}
	products := map[string]store.Product{}
	for _, p := range s.Products {
		products[p.ID] = p
	}

	if len(l.ProductionLogs) > 0 {
		warnings = append(warnings,
			fmt.Sprintf("原型的 %d 筆生產紀錄沒有原料消耗明細，已用匯入資料中的配方回推。"+
				"這是一次性近似，還原不了當時的真實消耗。", len(l.ProductionLogs)))
	}
	for _, lg := range l.ProductionLogs {
		log := store.ProductionLog{
			ID: lg.ID, LoggedDate: lg.Date, ProductID: lg.ProductID,
			ProductName: lg.ProductName, Qty: lg.Qty,
		}
		p, ok := products[lg.ProductID]
		if !ok {
			warnings = append(warnings,
				fmt.Sprintf("生產紀錄「%s」的商品已不存在，消耗明細留空，庫存不會扣除這批", lg.ProductName))
			log.ProductID = ""
		} else {
			for name, grams := range domain.ProductConsumption(p, recipes, lg.Qty) {
				log.Consumption = append(log.Consumption,
					store.Consumption{IngredientName: name, ConsumedG: grams})
			}
		}
		s.ProductionLogs = append(s.ProductionLogs, log)
	}
	return s, warnings
}

// validateRefs 先檢查引用完整性再開始寫。比靠交易回滾更能給出可讀的錯誤。
func validateRefs(s store.Snapshot) error {
	doughs := map[string]bool{}
	for _, d := range s.Doughs {
		doughs[d.ID] = true
	}
	fillings := map[string]bool{}
	for _, f := range s.Fillings {
		fillings[f.ID] = true
	}
	for _, p := range s.Products {
		if !doughs[p.DoughID] {
			return fmt.Errorf("商品「%s」引用的產品配方 %q 不存在", p.Name, p.DoughID)
		}
		if p.Fill1ID != "" && !fillings[p.Fill1ID] {
			return fmt.Errorf("商品「%s」引用的配料 1 %q 不存在", p.Name, p.Fill1ID)
		}
		if p.Fill2ID != "" && !fillings[p.Fill2ID] {
			return fmt.Errorf("商品「%s」引用的配料 2 %q 不存在", p.Name, p.Fill2ID)
		}
	}
	return nil
}

func (s *Server) importLegacy(w http.ResponseWriter, r *http.Request) {
	var legacy legacySnapshot
	if !decodeJSON(w, r, &legacy) {
		return
	}
	snap, warnings := legacy.toSnapshot()
	if err := validateRefs(snap); err != nil {
		writeError(w, http.StatusBadRequest, "匯入資料的引用不完整: "+err.Error())
		return
	}
	// 破壞性操作前先留一份快照
	if err := s.db.Backup(); err != nil {
		writeStoreError(w, err, "匯入前備份失敗")
		return
	}
	rep, err := s.db.ImportSnapshot(snap)
	if err != nil {
		writeStoreError(w, err, "匯入資料失敗")
		return
	}
	rep.Warnings = append(rep.Warnings, warnings...)
	writeJSON(w, http.StatusOK, rep)
}
