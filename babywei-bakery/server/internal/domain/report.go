package domain

import (
	"fmt"
	"time"

	"babywei-bakery/internal/store"
)

// Totals 是一組出貨紀錄的財務加總。
type Totals struct {
	RevenueTWD float64 `json:"revenueTwd"`
	CostTWD    float64 `json:"costTwd"`
	ProfitTWD  float64 `json:"profitTwd"`
}

// SalesTotals 加總營收、成本與利潤。單價與單位成本取自出貨當下的快照。
func SalesTotals(sales []store.Sale) Totals {
	var t Totals
	for _, s := range sales {
		q := float64(s.Qty)
		t.RevenueTWD += q * s.UnitPrice
		t.CostTWD += q * s.UnitCost
	}
	t.ProfitTWD = t.RevenueTWD - t.CostTWD
	return t
}

// FilterByDateRange 篩出 SaleDate 落在 [from, to] 的紀錄，含端點。
// from 或 to 為空字串代表該側不設限。日期是 YYYY-MM-DD，字典序即時間序。
func FilterByDateRange(sales []store.Sale, from, to string) []store.Sale {
	out := make([]store.Sale, 0, len(sales))
	for _, s := range sales {
		if from != "" && s.SaleDate < from {
			continue
		}
		if to != "" && s.SaleDate > to {
			continue
		}
		out = append(out, s)
	}
	return out
}

// Summary 以 now 為基準算出本日 / 本月 / 本季 / 本年度的加總。
// 上界一律是 now 當天，避免未來日期的紀錄混入。
func Summary(sales []store.Sale, now time.Time) map[string]Totals {
	y, m, d := now.Date()
	quarterStartMonth := (int(m)-1)/3*3 + 1
	today := fmt.Sprintf("%04d-%02d-%02d", y, int(m), d)

	starts := map[string]string{
		"day":     today,
		"month":   fmt.Sprintf("%04d-%02d-01", y, int(m)),
		"quarter": fmt.Sprintf("%04d-%02d-01", y, quarterStartMonth),
		"year":    fmt.Sprintf("%04d-01-01", y),
	}
	out := make(map[string]Totals, len(starts))
	for key, from := range starts {
		out[key] = SalesTotals(FilterByDateRange(sales, from, today))
	}
	return out
}
