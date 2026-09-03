package api

import (
	"net/http"
	"testing"

	"babywei-bakery/internal/domain"
	"babywei-bakery/internal/store"
)

// legacySample 是原型 templates/index.html 內建範例資料的結構與內容。
const legacySample = `{
  "costDB": [
    {"id":"c1","name":"高筋麵粉","brand":"水手牌","purchaseDate":"2026-09-03","channel":"烘焙材料行","price":120,"weight":1000},
    {"id":"c2","name":"牛奶","brand":"光泉","purchaseDate":"2026-09-03","channel":"全聯","price":90,"weight":1000},
    {"id":"c8","name":"南瓜泥","brand":"","purchaseDate":"2026-09-03","channel":"傳統市場","price":60,"weight":500}
  ],
  "dough": [
    {"id":"d1","name":"基礎吐司麵團","ingredients":[
      {"name":"高筋麵粉","pct":100},{"name":"牛奶","pct":65}]}
  ],
  "fillings": [
    {"id":"f1","name":"南瓜泥","ingredients":[{"name":"南瓜泥","weight":100}]}
  ],
  "products": [
    {"id":"p1","name":"南瓜藜麥吐司","price":180,"doughId":"d1","doughWeight":480,
     "fill1Id":"f1","fill1Weight":80,"fill2Id":"","fill2Weight":0}
  ],
  "sales": [
    {"id":"s1","date":"2026-09-02","productId":"p1","productName":"南瓜藜麥吐司",
     "qty":3,"unitCost":55.5,"unitPrice":180}
  ],
  "productionLogs": [
    {"id":"log1","date":"2026-09-02","productId":"p1","productName":"南瓜藜麥吐司","qty":2}
  ]
}`

func postRaw(t *testing.T, h http.Handler, path, body string) *httpRecorder {
	t.Helper()
	return doRaw(t, h, "POST", path, body)
}

func TestImportLegacyFieldMapping(t *testing.T) {
	h, _ := newTestServer(t)
	rec := postRaw(t, h, "/api/import", legacySample)
	if rec.Code != http.StatusOK {
		t.Fatalf("匯入失敗 (%d): %s", rec.Code, rec.Body.String())
	}
	var rep store.ImportReport
	decodeIntoRaw(t, rec, &rep)

	want := map[string]int{
		"purchases": 3, "doughs": 1, "fillings": 1,
		"products": 1, "sales": 1, "productionLogs": 1,
	}
	for k, v := range want {
		if rep.Counts[k] != v {
			t.Errorf("counts[%q] = %d, want %d", k, rep.Counts[k], v)
		}
	}

	// weight → weightG
	var purchases []store.Purchase
	decodeInto(t, do(t, h, "GET", "/api/purchases", nil), &purchases)
	for _, p := range purchases {
		if p.WeightG == 0 {
			t.Errorf("進貨 %q 的 weightG 未對應", p.Name)
		}
	}
	// doughWeight → doughWeightG，pct 與 weight 分別對應
	var products []store.Product
	decodeInto(t, do(t, h, "GET", "/api/products", nil), &products)
	if len(products) != 1 || products[0].DoughWeightG != 480 || products[0].Fill1WeightG != 80 {
		t.Errorf("商品欄位對應錯誤: %+v", products)
	}
	// date → saleDate，金額快照原樣保留
	var sales []store.Sale
	decodeInto(t, do(t, h, "GET", "/api/sales", nil), &sales)
	if len(sales) != 1 || sales[0].SaleDate != "2026-09-02" || sales[0].UnitCost != 55.5 {
		t.Errorf("出貨欄位對應錯誤: %+v", sales)
	}
}

func TestImportBackfillsConsumption(t *testing.T) {
	h, _ := newTestServer(t)
	if rec := postRaw(t, h, "/api/import", legacySample); rec.Code != http.StatusOK {
		t.Fatalf("匯入失敗: %s", rec.Body.String())
	}
	// 原型的 productionLogs 沒有消耗明細，匯入時必須回推
	var inv []domain.InventoryRow
	decodeInto(t, do(t, h, "GET", "/api/inventory", nil), &inv)

	// 生產 2 顆：麵團 480×2=960g 按 100:65 分配 → 麵粉 960×100/165
	var flourUsed float64
	for _, r := range inv {
		if r.Name == "高筋麵粉" {
			flourUsed = r.TotalUsedG
		}
	}
	wantFlour := 960.0 * 100 / 165
	if diff := flourUsed - wantFlour; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("麵粉消耗 = %v, want %v（消耗須被回推）", flourUsed, wantFlour)
	}
}

func TestImportWarnsAboutBackfill(t *testing.T) {
	h, _ := newTestServer(t)
	rec := postRaw(t, h, "/api/import", legacySample)
	var rep store.ImportReport
	decodeIntoRaw(t, rec, &rep)
	if len(rep.Warnings) == 0 {
		t.Error("回推消耗是一次性近似，必須列入 Warnings 而非靜默進行")
	}
}

func TestImportRejectsBrokenReferences(t *testing.T) {
	h, _ := newTestServer(t)
	// 先放一些既有資料
	if rec := do(t, h, "POST", "/api/purchases", store.Purchase{
		Name: "既有材料", PurchaseDate: "2026-09-03", Price: 1, WeightG: 1,
	}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}

	broken := `{"costDB":[],"dough":[],"fillings":[],
	  "products":[{"id":"p1","name":"壞商品","price":1,"doughId":"不存在","doughWeight":1}],
	  "sales":[],"productionLogs":[]}`
	rec := postRaw(t, h, "/api/import", broken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("引用不完整應回 400，實際 %d: %s", rec.Code, rec.Body.String())
	}

	// 既有資料必須完整保留
	var purchases []store.Purchase
	decodeInto(t, do(t, h, "GET", "/api/purchases", nil), &purchases)
	if len(purchases) != 1 {
		t.Errorf("匯入失敗後既有資料被破壞，剩 %d 筆", len(purchases))
	}
}

func TestImportIsDestructive(t *testing.T) {
	h, _ := newTestServer(t)
	if rec := do(t, h, "POST", "/api/purchases", store.Purchase{
		Name: "會被清掉的", PurchaseDate: "2026-09-03", Price: 1, WeightG: 1,
	}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	if rec := postRaw(t, h, "/api/import", legacySample); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	var purchases []store.Purchase
	decodeInto(t, do(t, h, "GET", "/api/purchases", nil), &purchases)
	for _, p := range purchases {
		if p.Name == "會被清掉的" {
			t.Error("匯入是破壞性操作，舊資料不該殘留")
		}
	}
	if len(purchases) != 3 {
		t.Errorf("進貨筆數 = %d, want 3", len(purchases))
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	h, db := newTestServer(t)
	if rec := postRaw(t, h, "/api/import", legacySample); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	before, err := db.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}

	rec := do(t, h, "GET", "/api/export/backup.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("匯出失敗: %s", rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); cd == "" {
		t.Error("匯出應帶 Content-Disposition 讓瀏覽器下載")
	}
	var snap store.Snapshot
	decodeInto(t, rec, &snap)

	if len(snap.Purchases) != len(before.Purchases) ||
		len(snap.Products) != len(before.Products) ||
		len(snap.ProductionLogs) != len(before.ProductionLogs) {
		t.Errorf("匯出內容與資料庫不一致")
	}
	if len(snap.ProductionLogs) > 0 && len(snap.ProductionLogs[0].Consumption) == 0 {
		t.Error("匯出的生產紀錄應含消耗明細")
	}
}
