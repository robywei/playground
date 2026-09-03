package domain

import (
	"testing"

	"babywei-bakery/internal/store"
)

func rowByName(rows []InventoryRow, name string) (InventoryRow, bool) {
	for _, r := range rows {
		if r.Name == name {
			return r, true
		}
	}
	return InventoryRow{}, false
}

func TestInventorySubtractsSnapshotConsumption(t *testing.T) {
	purchases := []store.Purchase{
		{Name: "麵粉", Brand: "水手牌", Price: 120, WeightG: 1000},
		{Name: "麵粉", Brand: "水手牌", Price: 120, WeightG: 1000},
	}
	logs := []store.ProductionLog{
		{ID: "l1", Consumption: []store.Consumption{{IngredientName: "麵粉", ConsumedG: 750}}},
	}
	r, ok := rowByName(Inventory(purchases, logs), "麵粉")
	if !ok {
		t.Fatal("麵粉不在庫存表中")
	}
	closeTo(t, r.TotalBoughtG, 2000, "總進貨")
	closeTo(t, r.TotalUsedG, 750, "已消耗")
	closeTo(t, r.RemainingG, 1250, "剩餘")
	if r.Status != "ok" {
		t.Errorf("狀態 = %q, want ok", r.Status)
	}
}

func TestInventoryUsesSnapshotNotCurrentRecipe(t *testing.T) {
	// 修正原型的關鍵行為：消耗量來自快照，本函數收不到配方，
	// 因此改配方或刪商品都不可能影響歷史庫存。
	logs := []store.ProductionLog{
		{ID: "l1", ProductID: "", ProductName: "已刪除的商品",
			Consumption: []store.Consumption{{IngredientName: "麵粉", ConsumedG: 500}}},
	}
	r, _ := rowByName(Inventory([]store.Purchase{{Name: "麵粉", WeightG: 1000, Price: 100}}, logs), "麵粉")
	closeTo(t, r.TotalUsedG, 500, "商品已刪除，消耗仍須計入")
}

func TestInventoryIncludesNeverPurchasedIngredient(t *testing.T) {
	// 修正原型第二個 bug：配方用到但從未進貨的材料，原型會靜默丟棄
	// 其消耗且不顯示該材料。
	logs := []store.ProductionLog{
		{ID: "l1", Consumption: []store.Consumption{{IngredientName: "漏登的酵母", ConsumedG: 30}}},
	}
	r, ok := rowByName(Inventory(nil, logs), "漏登的酵母")
	if !ok {
		t.Fatal("從未進貨但已消耗的材料必須出現在庫存表")
	}
	closeTo(t, r.TotalBoughtG, 0, "總進貨")
	closeTo(t, r.RemainingG, -30, "剩餘應為負")
	if r.Status != "out" {
		t.Errorf("狀態 = %q, want out", r.Status)
	}
}

func TestInventoryStatusThresholds(t *testing.T) {
	cases := []struct {
		boughtG, usedG float64
		want           string
	}{
		{1000, 0, "ok"},
		{1000, 801, "low"},  // 剩 199
		{1000, 800, "ok"},   // 剩 200，門檻是 < 200
		{1000, 1000, "out"}, // 剩 0
		{1000, 1200, "out"}, // 剩 -200
	}
	for _, c := range cases {
		rows := Inventory(
			[]store.Purchase{{Name: "X", WeightG: c.boughtG, Price: 1}},
			[]store.ProductionLog{{ID: "l", Consumption: []store.Consumption{
				{IngredientName: "X", ConsumedG: c.usedG}}}},
		)
		r, _ := rowByName(rows, "X")
		if r.Status != c.want {
			t.Errorf("進貨 %v 消耗 %v → 狀態 %q, want %q", c.boughtG, c.usedG, r.Status, c.want)
		}
	}
}

func TestInventoryIsSortedByName(t *testing.T) {
	rows := Inventory([]store.Purchase{
		{Name: "糖", WeightG: 100, Price: 1},
		{Name: "麵粉", WeightG: 100, Price: 1},
		{Name: "鹽", WeightG: 100, Price: 1},
	}, nil)
	if len(rows) != 3 {
		t.Fatalf("列數 = %d, want 3", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Name > rows[i].Name {
			t.Errorf("未依名稱排序: %q 在 %q 之前", rows[i-1].Name, rows[i].Name)
		}
	}
}

func TestInventoryBrandTakesLatestNonEmpty(t *testing.T) {
	rows := Inventory([]store.Purchase{
		{Name: "麵粉", Brand: "", WeightG: 100, Price: 1},
		{Name: "麵粉", Brand: "水手牌", WeightG: 100, Price: 1},
	}, nil)
	r, _ := rowByName(rows, "麵粉")
	if r.Brand != "水手牌" {
		t.Errorf("品牌 = %q, want 水手牌", r.Brand)
	}
}

func TestInventoryEmptyInputs(t *testing.T) {
	if rows := Inventory(nil, nil); len(rows) != 0 {
		t.Errorf("無資料應回傳空 slice，實際 %d 列", len(rows))
	}
}
