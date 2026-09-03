package domain

import "babywei-bakery/internal/store"

// Recipes 是配方查表，key 為 ID。
type Recipes struct {
	Doughs   map[string]store.Dough
	Fillings map[string]store.Filling
}

// CostPerGram 以全期加權平均算出各材料的每克成本：
//
//	cost_per_g(材料) = Σ price / Σ weight_g
//
// 這是刻意的取捨。它不是移動加權平均 —— 舊進貨會持續影響均價，即使該批
// 早已用完。移動加權平均需追蹤每批剩餘量，較準但複雜度顯著提高。
// 要改算法只需改本函數，呼叫端不受影響。
func CostPerGram(purchases []store.Purchase) map[string]float64 {
	type acc struct{ price, weight float64 }
	sums := make(map[string]acc)
	for _, p := range purchases {
		a := sums[p.Name]
		a.price += p.Price
		a.weight += p.WeightG
		sums[p.Name] = a
	}
	out := make(map[string]float64, len(sums))
	for name, a := range sums {
		if a.weight > 0 {
			out[name] = a.price / a.weight
		}
	}
	return out
}

// components 收集一項商品各配方段落按 qty 換算後的材料用量。
func components(p store.Product, r Recipes, qty int) []map[string]float64 {
	q := float64(qty)
	var parts []map[string]float64

	if d, ok := r.Doughs[p.DoughID]; ok {
		parts = append(parts, Scale(FromDough(d.Ingredients), p.DoughWeightG*q))
	}
	for _, f := range []struct {
		id      string
		weightG float64
	}{
		{p.Fill1ID, p.Fill1WeightG},
		{p.Fill2ID, p.Fill2WeightG},
	} {
		if f.id == "" {
			continue
		}
		if fl, ok := r.Fillings[f.id]; ok {
			parts = append(parts, Scale(FromFilling(fl.Ingredients), f.weightG*q))
		}
	}
	return parts
}

// ProductUnitCost 算出一顆商品的原料成本。配方不存在的段落貢獻 0。
func ProductUnitCost(p store.Product, r Recipes, costPerG map[string]float64) float64 {
	var total float64
	for _, part := range components(p, r, 1) {
		for name, grams := range part {
			total += grams * costPerG[name]
		}
	}
	return total
}

// ProductCostBreakdown 是單顆成本的逐項明細，讓使用者看得出錢花在哪。
type ProductCostBreakdown struct {
	IngredientName string  `json:"ingredientName"`
	GramsPerUnit   float64 `json:"gramsPerUnit"`
	CostPerGram    float64 `json:"costPerGram"`
	CostTWD        float64 `json:"costTwd"`
}

// ProductBreakdown 算出單顆商品各材料的用量與成本貢獻，依材料名稱排序。
func ProductBreakdown(p store.Product, r Recipes, costPerG map[string]float64) []ProductCostBreakdown {
	merged := make(map[string]float64)
	for _, part := range components(p, r, 1) {
		for name, grams := range part {
			merged[name] += grams
		}
	}
	return sortedBreakdown(merged, costPerG)
}

// ProductConsumption 算出生產 qty 顆商品所消耗的各材料克數。
// 麵團與配料用到同一材料時，用量相加。
func ProductConsumption(p store.Product, r Recipes, qty int) map[string]float64 {
	out := make(map[string]float64)
	for _, part := range components(p, r, qty) {
		for name, grams := range part {
			out[name] += grams
		}
	}
	return out
}
