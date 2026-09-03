package domain

import (
	"testing"
	"time"

	"babywei-bakery/internal/store"
)

func TestSalesTotals(t *testing.T) {
	got := SalesTotals([]store.Sale{
		{Qty: 10, UnitPrice: 180, UnitCost: 60},
		{Qty: 2, UnitPrice: 150, UnitCost: 50},
	})
	closeTo(t, got.RevenueTWD, 10*180+2*150, "營收")
	closeTo(t, got.CostTWD, 10*60+2*50, "成本")
	closeTo(t, got.ProfitTWD, (1800-600)+(300-100), "利潤")
}

func TestSalesTotalsEmpty(t *testing.T) {
	got := SalesTotals(nil)
	closeTo(t, got.RevenueTWD, 0, "營收")
	closeTo(t, got.ProfitTWD, 0, "利潤")
}

func TestFilterByDateRangeInclusive(t *testing.T) {
	sales := []store.Sale{
		{ID: "a", SaleDate: "2026-08-31"},
		{ID: "b", SaleDate: "2026-09-01"},
		{ID: "c", SaleDate: "2026-09-30"},
		{ID: "d", SaleDate: "2026-10-01"},
	}
	got := FilterByDateRange(sales, "2026-09-01", "2026-09-30")
	if len(got) != 2 || got[0].ID != "b" || got[1].ID != "c" {
		t.Errorf("區間應含端點，實際 %+v", got)
	}
}

func TestFilterByDateRangeOpenEnded(t *testing.T) {
	sales := []store.Sale{
		{ID: "a", SaleDate: "2026-01-01"},
		{ID: "b", SaleDate: "2026-12-31"},
	}
	if got := FilterByDateRange(sales, "", ""); len(got) != 2 {
		t.Errorf("兩側不設限應回傳全部，實際 %d 筆", len(got))
	}
	if got := FilterByDateRange(sales, "2026-06-01", ""); len(got) != 1 || got[0].ID != "b" {
		t.Errorf("只設下界，實際 %+v", got)
	}
	if got := FilterByDateRange(sales, "", "2026-06-01"); len(got) != 1 || got[0].ID != "a" {
		t.Errorf("只設上界，實際 %+v", got)
	}
}

func TestSummaryBuckets(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.Local)
	sales := []store.Sale{
		{SaleDate: "2026-09-03", Qty: 1, UnitPrice: 100},  // 本日
		{SaleDate: "2026-09-01", Qty: 1, UnitPrice: 200},  // 本月
		{SaleDate: "2026-07-15", Qty: 1, UnitPrice: 400},  // 本季
		{SaleDate: "2026-02-01", Qty: 1, UnitPrice: 800},  // 本年度
		{SaleDate: "2025-12-31", Qty: 1, UnitPrice: 9999}, // 去年，全不算
	}
	got := Summary(sales, now)
	closeTo(t, got["day"].RevenueTWD, 100, "本日營收")
	closeTo(t, got["month"].RevenueTWD, 300, "本月營收")
	closeTo(t, got["quarter"].RevenueTWD, 700, "本季營收")
	closeTo(t, got["year"].RevenueTWD, 1500, "本年度營收")
}

func TestSummaryExcludesFutureDates(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.Local)
	got := Summary([]store.Sale{
		{SaleDate: "2026-09-03", Qty: 1, UnitPrice: 100},
		{SaleDate: "2026-09-30", Qty: 1, UnitPrice: 9999}, // 未來
	}, now)
	closeTo(t, got["month"].RevenueTWD, 100, "本月營收應排除未來日期")
}

func TestSummaryQuarterBoundaries(t *testing.T) {
	cases := []struct{ now, inQuarter, notIn string }{
		{"2026-01-15", "2026-01-01", "2025-12-31"}, // Q1
		{"2026-05-15", "2026-04-01", "2026-03-31"}, // Q2
		{"2026-08-15", "2026-07-01", "2026-06-30"}, // Q3
		{"2026-11-15", "2026-10-01", "2026-09-30"}, // Q4
	}
	for _, c := range cases {
		now, err := time.ParseInLocation("2006-01-02", c.now, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		got := Summary([]store.Sale{
			{SaleDate: c.inQuarter, Qty: 1, UnitPrice: 10},
			{SaleDate: c.notIn, Qty: 1, UnitPrice: 10},
		}, now)
		closeTo(t, got["quarter"].RevenueTWD, 10, "基準 "+c.now+" 的本季營收")
	}
}
