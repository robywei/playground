package domain

import (
	"testing"

	"babywei-bakery/internal/store"
)

func TestCostPerGramWeightedAverage(t *testing.T) {
	// 麵粉兩批：120元/1000g 與 150元/500g → (120+150)/(1000+500) = 0.18
	got := CostPerGram([]store.Purchase{
		{Name: "高筋麵粉", Price: 120, WeightG: 1000},
		{Name: "高筋麵粉", Price: 150, WeightG: 500},
	})
	closeTo(t, got["高筋麵粉"], 0.18, "麵粉單位成本")
}

func TestCostPerGramIsNotLatestPurchase(t *testing.T) {
	// 刻意偏離原型：原型取最近一筆（0.3），改為加權平均（0.2）
	got := CostPerGram([]store.Purchase{
		{Name: "糖", Price: 100, WeightG: 1000, PurchaseDate: "2026-01-01"},
		{Name: "糖", Price: 300, WeightG: 1000, PurchaseDate: "2026-09-01"},
	})
	closeTo(t, got["糖"], 0.2, "糖單位成本（加權平均而非最近一筆）")
}

func TestCostPerGramUnknownIngredientIsZero(t *testing.T) {
	got := CostPerGram(nil)
	if v, ok := got["不存在"]; ok && v != 0 {
		t.Errorf("未進貨材料應為 0，實際 %v", v)
	}
}

func testRecipes() Recipes {
	return Recipes{
		Doughs: map[string]store.Dough{
			"d1": {ID: "d1", Name: "基礎吐司", Ingredients: []store.DoughItem{
				{Name: "麵粉", Pct: 100},
				{Name: "水", Pct: 100},
			}},
		},
		Fillings: map[string]store.Filling{
			"f1": {ID: "f1", Name: "南瓜泥", Ingredients: []store.FillingItem{
				{Name: "南瓜泥", WeightG: 100},
			}},
		},
	}
}

func TestProductUnitCost(t *testing.T) {
	costPerG := map[string]float64{"麵粉": 0.2, "水": 0, "南瓜泥": 0.5}
	p := store.Product{DoughID: "d1", DoughWeightG: 200, Fill1ID: "f1", Fill1WeightG: 80}
	// 麵團 200g → 麵粉 100g + 水 100g → 100×0.2 = 20
	// 配料 80g 全是南瓜泥 → 80×0.5 = 40
	closeTo(t, ProductUnitCost(p, testRecipes(), costPerG), 60, "單顆成本")
}

func TestProductUnitCostMissingRecipeIsZero(t *testing.T) {
	p := store.Product{DoughID: "不存在", DoughWeightG: 200}
	closeTo(t, ProductUnitCost(p, testRecipes(), nil), 0, "配方不存在時的成本")
}

func TestProductBreakdownIsSortedAndSumsToUnitCost(t *testing.T) {
	costPerG := map[string]float64{"麵粉": 0.2, "水": 0, "南瓜泥": 0.5}
	p := store.Product{DoughID: "d1", DoughWeightG: 200, Fill1ID: "f1", Fill1WeightG: 80}
	rows := ProductBreakdown(p, testRecipes(), costPerG)
	if len(rows) != 3 {
		t.Fatalf("明細列數 = %d, want 3", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].IngredientName > rows[i].IngredientName {
			t.Errorf("未依名稱排序: %q 在 %q 之前", rows[i-1].IngredientName, rows[i].IngredientName)
		}
	}
	var sum float64
	for _, r := range rows {
		sum += r.CostTWD
	}
	closeTo(t, sum, ProductUnitCost(p, testRecipes(), costPerG), "明細加總須等於單顆成本")
}

func TestProductConsumptionScalesWithQty(t *testing.T) {
	p := store.Product{DoughID: "d1", DoughWeightG: 200, Fill1ID: "f1", Fill1WeightG: 80}
	got := ProductConsumption(p, testRecipes(), 10)
	closeTo(t, got["麵粉"], 1000, "麵粉消耗")
	closeTo(t, got["水"], 1000, "水消耗")
	closeTo(t, got["南瓜泥"], 800, "南瓜泥消耗")
}

func TestProductConsumptionMergesSharedIngredient(t *testing.T) {
	r := Recipes{
		Doughs: map[string]store.Dough{
			"d1": {ID: "d1", Ingredients: []store.DoughItem{{Name: "糖", Pct: 100}}},
		},
		Fillings: map[string]store.Filling{
			"f1": {ID: "f1", Ingredients: []store.FillingItem{{Name: "糖", WeightG: 100}}},
		},
	}
	p := store.Product{DoughID: "d1", DoughWeightG: 30, Fill1ID: "f1", Fill1WeightG: 20}
	got := ProductConsumption(p, r, 1)
	closeTo(t, got["糖"], 50, "糖消耗（麵團 30 + 配料 20）")
}

func TestProductConsumptionSkipsEmptyFillingID(t *testing.T) {
	p := store.Product{DoughID: "d1", DoughWeightG: 100, Fill1ID: "", Fill1WeightG: 999}
	got := ProductConsumption(p, testRecipes(), 1)
	if _, ok := got["南瓜泥"]; ok {
		t.Error("Fill1ID 為空時不應計入任何配料")
	}
	closeTo(t, got["麵粉"], 50, "麵粉")
}
