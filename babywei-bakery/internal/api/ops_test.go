package api

import (
	"net/http"
	"testing"

	"babywei-bakery/internal/domain"
	"babywei-bakery/internal/store"
)

// seedProduct 建好配方、配料、進貨與一項商品，回傳商品 ID。
func seedProduct(t *testing.T, h http.Handler) string {
	t.Helper()
	dID, fID := seedForProduct(t, h)
	rec := do(t, h, "POST", "/api/products", store.Product{
		Name: "南瓜吐司", Price: 180, DoughID: dID, DoughWeightG: 200,
		Fill1ID: fID, Fill1WeightG: 80,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建立商品: %s", rec.Body.String())
	}
	var p store.Product
	decodeInto(t, rec, &p)
	return p.ID
}

func TestPostSaleSnapshotsSurvivePriceChange(t *testing.T) {
	h, _ := newTestServer(t)
	pID := seedProduct(t, h)

	rec := do(t, h, "POST", "/api/sales", map[string]any{
		"saleDate": "2026-09-03", "productId": pID, "qty": 10,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建立出貨: %s", rec.Body.String())
	}
	var sale store.Sale
	decodeInto(t, rec, &sale)
	if sale.UnitCost != 60 || sale.UnitPrice != 180 {
		t.Fatalf("快照 unitCost=%v unitPrice=%v, want 60 / 180", sale.UnitCost, sale.UnitPrice)
	}

	// 改售價與進貨價
	var products []store.Product
	decodeInto(t, do(t, h, "GET", "/api/products", nil), &products)
	products[0].Price = 999
	if rec := do(t, h, "PATCH", "/api/products/"+pID, products[0]); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if rec := do(t, h, "POST", "/api/purchases", store.Purchase{
		Name: "麵粉", PurchaseDate: "2026-09-10", Price: 9000, WeightG: 1000,
	}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}

	var got []store.Sale
	decodeInto(t, do(t, h, "GET", "/api/sales", nil), &got)
	if got[0].UnitPrice != 180 || got[0].UnitCost != 60 {
		t.Errorf("快照被改動: unitPrice=%v unitCost=%v", got[0].UnitPrice, got[0].UnitCost)
	}
}

func TestPostSaleDefaultsToToday(t *testing.T) {
	h, _ := newTestServer(t)
	pID := seedProduct(t, h)
	rec := do(t, h, "POST", "/api/sales", map[string]any{"productId": pID, "qty": 1})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建立出貨: %s", rec.Body.String())
	}
	var sale store.Sale
	decodeInto(t, rec, &sale)
	if sale.SaleDate != today() {
		t.Errorf("saleDate = %q, want %q（不可寫死日期）", sale.SaleDate, today())
	}
}

func TestPostSaleQtyValidation(t *testing.T) {
	h, _ := newTestServer(t)
	pID := seedProduct(t, h)
	for _, qty := range []int{0, -1} {
		rec := do(t, h, "POST", "/api/sales", map[string]any{"productId": pID, "qty": qty})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("qty=%d: status = %d, want 400", qty, rec.Code)
		}
	}
}

func TestProductionPreviewDoesNotPersist(t *testing.T) {
	h, _ := newTestServer(t)
	pID := seedProduct(t, h)

	rec := do(t, h, "POST", "/api/production/preview", map[string]any{
		"productId": pID, "qty": 10,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %s", rec.Body.String())
	}
	var scaled struct {
		UnitWeightG float64            `json:"unitWeightG"`
		DoughTotalG float64            `json:"doughTotalG"`
		FillTotalG  float64            `json:"fillTotalG"`
		Consumption map[string]float64 `json:"consumption"`
		Sections    []struct {
			Title string `json:"title"`
			Items []struct {
				Name   string  `json:"name"`
				UsageG float64 `json:"usageG"`
			} `json:"items"`
		} `json:"sections"`
	}
	decodeInto(t, rec, &scaled)
	if scaled.UnitWeightG != 280 {
		t.Errorf("單顆總重 = %v, want 280", scaled.UnitWeightG)
	}
	if scaled.DoughTotalG != 2000 || scaled.FillTotalG != 800 {
		t.Errorf("批次總重 dough=%v fill=%v, want 2000 / 800", scaled.DoughTotalG, scaled.FillTotalG)
	}
	if len(scaled.Sections) != 2 {
		t.Errorf("換算段落 = %d, want 2", len(scaled.Sections))
	}

	// 庫存不可被改動
	var inv []domain.InventoryRow
	decodeInto(t, do(t, h, "GET", "/api/inventory", nil), &inv)
	for _, r := range inv {
		if r.TotalUsedG != 0 {
			t.Errorf("preview 不該扣庫存，%s 已消耗 %v", r.Name, r.TotalUsedG)
		}
	}
}

func TestConfirmProductionDeductsInventory(t *testing.T) {
	h, _ := newTestServer(t)
	pID := seedProduct(t, h)

	rec := do(t, h, "POST", "/api/production", map[string]any{"productId": pID, "qty": 5})
	if rec.Code != http.StatusCreated {
		t.Fatalf("確認生產: %s", rec.Body.String())
	}
	var log store.ProductionLog
	decodeInto(t, rec, &log)
	if log.LoggedDate != today() {
		t.Errorf("loggedDate = %q, want %q（不可寫死日期）", log.LoggedDate, today())
	}

	var inv []domain.InventoryRow
	decodeInto(t, do(t, h, "GET", "/api/inventory", nil), &inv)
	want := map[string]float64{"麵粉": 500, "水": 500, "南瓜泥": 400}
	for _, r := range inv {
		if w, ok := want[r.Name]; ok && r.TotalUsedG != w {
			t.Errorf("%s 已消耗 = %v, want %v", r.Name, r.TotalUsedG, w)
		}
	}
}

func TestInventorySurvivesRecipeChange(t *testing.T) {
	h, _ := newTestServer(t)
	pID := seedProduct(t, h)
	if rec := do(t, h, "POST", "/api/production", map[string]any{"productId": pID, "qty": 5}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}

	// 大幅改配方
	var doughs []store.Dough
	decodeInto(t, do(t, h, "GET", "/api/doughs", nil), &doughs)
	doughs[0].Ingredients = []store.DoughItem{{Name: "全麥麵粉", Pct: 100}}
	if rec := do(t, h, "PATCH", "/api/doughs/"+doughs[0].ID, doughs[0]); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}

	var inv []domain.InventoryRow
	decodeInto(t, do(t, h, "GET", "/api/inventory", nil), &inv)
	for _, r := range inv {
		if r.Name == "麵粉" && r.TotalUsedG != 500 {
			t.Errorf("改配方後麵粉消耗 = %v, want 500（歷史不該回溯變動）", r.TotalUsedG)
		}
	}
}

func TestInventoryShowsNeverPurchasedIngredient(t *testing.T) {
	h, _ := newTestServer(t)
	// 配方用到「酵母」但從未進貨
	var d store.Dough
	rec := do(t, h, "POST", "/api/doughs", store.Dough{
		Name: "有漏登材料的配方", Ingredients: []store.DoughItem{{Name: "酵母", Pct: 100}},
	})
	decodeInto(t, rec, &d)
	rec = do(t, h, "POST", "/api/products", store.Product{
		Name: "X", Price: 1, DoughID: d.ID, DoughWeightG: 100,
	})
	var p store.Product
	decodeInto(t, rec, &p)
	if rec := do(t, h, "POST", "/api/production", map[string]any{"productId": p.ID, "qty": 1}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}

	var inv []domain.InventoryRow
	decodeInto(t, do(t, h, "GET", "/api/inventory", nil), &inv)
	var found bool
	for _, r := range inv {
		if r.Name == "酵母" {
			found = true
			if r.RemainingG != -100 || r.Status != "out" {
				t.Errorf("酵母 remaining=%v status=%q, want -100 / out", r.RemainingG, r.Status)
			}
		}
	}
	if !found {
		t.Error("從未進貨但已消耗的材料必須出現在庫存表")
	}
}

func TestReportsSummaryHasFourBuckets(t *testing.T) {
	h, _ := newTestServer(t)
	pID := seedProduct(t, h)
	if rec := do(t, h, "POST", "/api/sales", map[string]any{"productId": pID, "qty": 2}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	var summary map[string]domain.Totals
	decodeInto(t, do(t, h, "GET", "/api/reports/summary", nil), &summary)
	for _, k := range []string{"day", "month", "quarter", "year"} {
		if _, ok := summary[k]; !ok {
			t.Errorf("缺少區間 %q", k)
		}
	}
	if summary["day"].RevenueTWD != 360 {
		t.Errorf("本日營收 = %v, want 360", summary["day"].RevenueTWD)
	}
	if summary["day"].ProfitTWD != 360-120 {
		t.Errorf("本日利潤 = %v, want 240", summary["day"].ProfitTWD)
	}
}

func TestReportsSalesIncludesTotals(t *testing.T) {
	h, _ := newTestServer(t)
	pID := seedProduct(t, h)
	if rec := do(t, h, "POST", "/api/sales", map[string]any{
		"saleDate": "2026-09-15", "productId": pID, "qty": 3,
	}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	var report struct {
		Sales  []store.Sale  `json:"sales"`
		Totals domain.Totals `json:"totals"`
	}
	decodeInto(t, do(t, h, "GET", "/api/reports/sales?from=2026-09-01&to=2026-09-30", nil), &report)
	if len(report.Sales) != 1 {
		t.Fatalf("明細筆數 = %d, want 1", len(report.Sales))
	}
	if report.Totals.RevenueTWD != 540 {
		t.Errorf("總營收 = %v, want 540", report.Totals.RevenueTWD)
	}
	// 區間外
	decodeInto(t, do(t, h, "GET", "/api/reports/sales?from=2026-10-01&to=2026-10-31", nil), &report)
	if len(report.Sales) != 0 || report.Totals.RevenueTWD != 0 {
		t.Errorf("區間外應為空: %+v", report)
	}
}
