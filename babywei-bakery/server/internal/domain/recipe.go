// Package domain 是純計算層：成本、配方換算、庫存、報表。
// 這裡的函數不碰資料庫也不碰 HTTP，可獨立測試。
package domain

import "babywei-bakery/internal/store"

// Component 是配方中的一項材料。Ratio 對產品配方是 Baker's %、
// 對配料是絕對克數 —— 換算會正規化，所以單位不影響結果。
type Component struct {
	Name  string
	Ratio float64
}

// FromDough 把產品配方材料轉為換算用的 Component。
func FromDough(items []store.DoughItem) []Component {
	out := make([]Component, 0, len(items))
	for _, it := range items {
		out = append(out, Component{Name: it.Name, Ratio: it.Pct})
	}
	return out
}

// FromFilling 把配料材料轉為換算用的 Component。
func FromFilling(items []store.FillingItem) []Component {
	out := make([]Component, 0, len(items))
	for _, it := range items {
		out = append(out, Component{Name: it.Name, Ratio: it.WeightG})
	}
	return out
}

// Scale 把配方按 totalG 的需求總重換算成各材料的實際用量（克）。
//
//	用量(材料) = totalG × Ratio(材料) / Σ Ratio
//
// 同名材料的用量會相加。配方為空、總重非正或比例總和非正時回傳空 map。
func Scale(items []Component, totalG float64) map[string]float64 {
	out := make(map[string]float64)
	if len(items) == 0 || totalG <= 0 {
		return out
	}
	var sum float64
	for _, it := range items {
		sum += it.Ratio
	}
	if sum <= 0 {
		return out
	}
	for _, it := range items {
		out[it.Name] += totalG * it.Ratio / sum
	}
	return out
}
