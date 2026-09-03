package api

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"babywei-bakery/internal/store"
)

func testFS() fs.FS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>t</title>")},
	}
}

func newTestServer(t *testing.T) (http.Handler, *store.DB) {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db, testFS()), db
}

// do 送出請求並回傳回應。body 為 nil 時不帶 body。
func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf *bytes.Buffer
	var req *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewBuffer(b)
		req = httptest.NewRequest(method, path, buf)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeInto(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("解析回應失敗: %v\nbody: %s", err, rec.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, "GET", "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("ok")) {
		t.Errorf("body = %q, 應含 ok", rec.Body.String())
	}
}

func TestStaticIndexServed(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, "GET", "/", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct[:9] != "text/html" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestSPAFallback(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, "GET", "/some/frontend/route", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200（非 /api 路徑應回 index.html）", rec.Code)
	}
}

func TestUnknownAPIReturnsJSON404(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, "GET", "/api/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body map[string]string
	decodeInto(t, rec, &body)
	if body["error"] == "" {
		t.Error("未知 API 路徑應回 JSON 錯誤，而非 HTML —— 否則前端只會看到 JSON parse 失敗")
	}
}

func TestMalformedJSONReturns400(t *testing.T) {
	h, _ := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/purchases", bytes.NewBufferString("{不是 JSON"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPostPurchaseReturns201WithID(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, "POST", "/api/purchases", store.Purchase{
		Name: "高筋麵粉", PurchaseDate: "2026-09-03", Price: 120, WeightG: 1000,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var got store.Purchase
	decodeInto(t, rec, &got)
	if got.ID == "" {
		t.Error("回應的 id 為空")
	}
}

func TestPostPurchaseValidation(t *testing.T) {
	h, _ := newTestServer(t)
	cases := []struct {
		name string
		body store.Purchase
	}{
		{"缺名稱", store.Purchase{PurchaseDate: "2026-09-03", Price: 1, WeightG: 1}},
		{"名稱只有空白", store.Purchase{Name: "   ", PurchaseDate: "2026-09-03", Price: 1, WeightG: 1}},
		{"缺日期", store.Purchase{Name: "X", Price: 1, WeightG: 1}},
		{"重量為 0", store.Purchase{Name: "X", PurchaseDate: "2026-09-03", Price: 1, WeightG: 0}},
		{"價格為負", store.Purchase{Name: "X", PurchaseDate: "2026-09-03", Price: -1, WeightG: 1}},
	}
	for _, c := range cases {
		rec := do(t, h, "POST", "/api/purchases", c.body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", c.name, rec.Code)
			continue
		}
		var body map[string]string
		decodeInto(t, rec, &body)
		if body["error"] == "" {
			t.Errorf("%s: 錯誤訊息為空", c.name)
		}
	}
}

func TestDeletePurchaseTwiceReturns404(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, "POST", "/api/purchases", store.Purchase{
		Name: "糖", PurchaseDate: "2026-09-03", Price: 40, WeightG: 1000,
	})
	var p store.Purchase
	decodeInto(t, rec, &p)

	if rec := do(t, h, "DELETE", "/api/purchases/"+p.ID, nil); rec.Code != http.StatusNoContent {
		t.Errorf("第一次刪除 status = %d, want 204", rec.Code)
	}
	if rec := do(t, h, "DELETE", "/api/purchases/"+p.ID, nil); rec.Code != http.StatusNotFound {
		t.Errorf("第二次刪除 status = %d, want 404", rec.Code)
	}
}

// seedForProduct 建立配方、配料與進貨，回傳 doughID / fillingID。
func seedForProduct(t *testing.T, h http.Handler) (string, string) {
	t.Helper()
	for _, p := range []store.Purchase{
		{Name: "麵粉", PurchaseDate: "2026-09-03", Price: 200, WeightG: 1000},  // 0.2/g
		{Name: "水", PurchaseDate: "2026-09-03", Price: 0, WeightG: 1000},     // 0/g
		{Name: "南瓜泥", PurchaseDate: "2026-09-03", Price: 500, WeightG: 1000}, // 0.5/g
	} {
		if rec := do(t, h, "POST", "/api/purchases", p); rec.Code != http.StatusCreated {
			t.Fatalf("seed purchase: %s", rec.Body.String())
		}
	}
	var d store.Dough
	rec := do(t, h, "POST", "/api/doughs", store.Dough{
		Name: "基礎吐司", Ingredients: []store.DoughItem{{Name: "麵粉", Pct: 100}, {Name: "水", Pct: 100}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed dough: %s", rec.Body.String())
	}
	decodeInto(t, rec, &d)

	var f store.Filling
	rec = do(t, h, "POST", "/api/fillings", store.Filling{
		Name: "南瓜餡", Ingredients: []store.FillingItem{{Name: "南瓜泥", WeightG: 100}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed filling: %s", rec.Body.String())
	}
	decodeInto(t, rec, &f)
	return d.ID, f.ID
}

func TestGetProductsIncludesUnitCostAndBreakdown(t *testing.T) {
	h, _ := newTestServer(t)
	dID, fID := seedForProduct(t, h)

	if rec := do(t, h, "POST", "/api/products", store.Product{
		Name: "南瓜吐司", Price: 180, DoughID: dID, DoughWeightG: 200,
		Fill1ID: fID, Fill1WeightG: 80,
	}); rec.Code != http.StatusCreated {
		t.Fatalf("建立商品: %s", rec.Body.String())
	}

	rec := do(t, h, "GET", "/api/products", nil)
	var got []struct {
		Name        string  `json:"name"`
		UnitCostTWD float64 `json:"unitCostTwd"`
		Breakdown   []struct {
			IngredientName string  `json:"ingredientName"`
			CostTWD        float64 `json:"costTwd"`
		} `json:"breakdown"`
	}
	decodeInto(t, rec, &got)
	if len(got) != 1 {
		t.Fatalf("商品數 = %d", len(got))
	}
	// 麵團 200g → 麵粉 100g×0.2 = 20；配料 80g 南瓜泥×0.5 = 40 → 共 60
	if got[0].UnitCostTWD != 60 {
		t.Errorf("unitCostTwd = %v, want 60", got[0].UnitCostTWD)
	}
	if len(got[0].Breakdown) != 3 {
		t.Errorf("成本明細列數 = %d, want 3", len(got[0].Breakdown))
	}
}

func TestPostProductValidation(t *testing.T) {
	h, _ := newTestServer(t)
	dID, fID := seedForProduct(t, h)
	cases := []struct {
		name string
		body store.Product
	}{
		{"缺名稱", store.Product{Price: 1, DoughID: dID, DoughWeightG: 1}},
		{"缺配方", store.Product{Name: "X", Price: 1, DoughWeightG: 1}},
		{"配方重量為 0", store.Product{Name: "X", Price: 1, DoughID: dID, DoughWeightG: 0}},
		{"售價為負", store.Product{Name: "X", Price: -1, DoughID: dID, DoughWeightG: 1}},
		{"選了配料但重量為 0", store.Product{Name: "X", Price: 1, DoughID: dID, DoughWeightG: 1, Fill1ID: fID}},
	}
	for _, c := range cases {
		if rec := do(t, h, "POST", "/api/products", c.body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", c.name, rec.Code)
		}
	}
}

func TestPostProductUnknownDoughReturns404(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, "POST", "/api/products", store.Product{
		Name: "X", Price: 1, DoughID: "d_nope", DoughWeightG: 1,
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteDoughInUseReturns409(t *testing.T) {
	h, _ := newTestServer(t)
	dID, _ := seedForProduct(t, h)
	if rec := do(t, h, "POST", "/api/products", store.Product{
		Name: "X", Price: 1, DoughID: dID, DoughWeightG: 100,
	}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	rec := do(t, h, "DELETE", "/api/doughs/"+dID, nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409（不是 500）; body: %s", rec.Code, rec.Body.String())
	}
}

func TestEmptyListsSerializeAsArray(t *testing.T) {
	h, _ := newTestServer(t)
	for _, path := range []string{
		"/api/purchases", "/api/doughs", "/api/fillings",
		"/api/products", "/api/sales", "/api/inventory",
	} {
		rec := do(t, h, "GET", path, nil)
		if got := bytes.TrimSpace(rec.Body.Bytes()); string(got) != "[]" {
			t.Errorf("%s 空集合回傳 %q, want []（null 會讓前端要多一層防禦）", path, got)
		}
	}
}
