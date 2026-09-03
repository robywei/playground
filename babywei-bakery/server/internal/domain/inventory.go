package domain

import (
	"sort"

	"babywei-bakery/internal/store"
)

// lowStockThresholdG 是「庫存偏低」的克數門檻（沿用原型）。
const lowStockThresholdG = 200

// InventoryRow 是庫存表的一列。
type InventoryRow struct {
	Name         string  `json:"name"`
	Brand        string  `json:"brand"`
	TotalBoughtG float64 `json:"totalBoughtG"`
	TotalUsedG   float64 `json:"totalUsedG"`
	RemainingG   float64 `json:"remainingG"`
	Status       string  `json:"status"` // out | low | ok
}

// Inventory 以進貨紀錄與生產消耗快照算出庫存。
//
// 消耗量只來自 logs 的 Consumption 快照 —— 本函數收不到配方，因此改配方
// 或刪除商品都不可能影響歷史庫存數字。這是刻意的簽章設計。
//
// 列為「有進貨」與「有消耗」兩者材料名稱的聯集：從未進貨卻有消耗的材料
// 會以總進貨 0、剩餘負值呈現，讓漏登的進貨看得見。
func Inventory(purchases []store.Purchase, logs []store.ProductionLog) []InventoryRow {
	bought := make(map[string]float64)
	brand := make(map[string]string)
	for _, p := range purchases {
		bought[p.Name] += p.WeightG
		if p.Brand != "" {
			brand[p.Name] = p.Brand
		}
	}
	used := make(map[string]float64)
	for _, l := range logs {
		for _, c := range l.Consumption {
			used[c.IngredientName] += c.ConsumedG
		}
	}

	names := make(map[string]struct{}, len(bought)+len(used))
	for n := range bought {
		names[n] = struct{}{}
	}
	for n := range used {
		names[n] = struct{}{}
	}

	rows := make([]InventoryRow, 0, len(names))
	for n := range names {
		remaining := bought[n] - used[n]
		rows = append(rows, InventoryRow{
			Name:         n,
			Brand:        brand[n],
			TotalBoughtG: bought[n],
			TotalUsedG:   used[n],
			RemainingG:   remaining,
			Status:       stockStatus(remaining),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

func stockStatus(remainingG float64) string {
	switch {
	case remainingG <= 0:
		return "out"
	case remainingG < lowStockThresholdG:
		return "low"
	default:
		return "ok"
	}
}
