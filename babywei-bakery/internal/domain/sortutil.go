package domain

import "sort"

// sortedBreakdown 把材料用量表轉為依名稱排序的明細，讓輸出順序穩定。
func sortedBreakdown(grams, costPerG map[string]float64) []ProductCostBreakdown {
	names := make([]string, 0, len(grams))
	for n := range grams {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]ProductCostBreakdown, 0, len(names))
	for _, n := range names {
		cpg := costPerG[n]
		out = append(out, ProductCostBreakdown{
			IngredientName: n,
			GramsPerUnit:   grams[n],
			CostPerGram:    cpg,
			CostTWD:        grams[n] * cpg,
		})
	}
	return out
}
