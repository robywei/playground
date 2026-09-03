package domain

import (
	"math"
	"testing"

	"babywei-bakery/internal/store"
)

func closeTo(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestScaleBakerPercent(t *testing.T) {
	items := FromDough([]store.DoughItem{
		{Name: "高筋麵粉", Pct: 100},
		{Name: "牛奶", Pct: 65},
	})
	got := Scale(items, 330)
	closeTo(t, got["高筋麵粉"], 200, "麵粉")
	closeTo(t, got["牛奶"], 130, "牛奶")
}

func TestScaleNormalizesWhenBaseIsNot100(t *testing.T) {
	items := FromDough([]store.DoughItem{
		{Name: "A", Pct: 50},
		{Name: "B", Pct: 50},
	})
	got := Scale(items, 100)
	closeTo(t, got["A"], 50, "A")
	closeTo(t, got["B"], 50, "B")
}

func TestScaleAbsoluteGrams(t *testing.T) {
	items := FromFilling([]store.FillingItem{
		{Name: "南瓜泥", WeightG: 100},
		{Name: "糖", WeightG: 50},
	})
	got := Scale(items, 300)
	closeTo(t, got["南瓜泥"], 200, "南瓜泥")
	closeTo(t, got["糖"], 100, "糖")
}

func TestScalePreservesTotal(t *testing.T) {
	items := FromDough([]store.DoughItem{
		{Name: "A", Pct: 100}, {Name: "B", Pct: 65},
		{Name: "C", Pct: 8}, {Name: "D", Pct: 1.5},
	})
	got := Scale(items, 1234.5)
	var sum float64
	for _, v := range got {
		sum += v
	}
	closeTo(t, sum, 1234.5, "各材料用量總和")
}

func TestScaleDuplicateNamesAreMerged(t *testing.T) {
	items := FromDough([]store.DoughItem{
		{Name: "糖", Pct: 5},
		{Name: "糖", Pct: 5},
	})
	got := Scale(items, 100)
	if len(got) != 1 {
		t.Fatalf("結果應合併為 1 項，實際 %d 項", len(got))
	}
	closeTo(t, got["糖"], 100, "糖")
}

func TestScaleEdgeCases(t *testing.T) {
	if got := Scale(nil, 100); len(got) != 0 {
		t.Errorf("空配方應回傳空 map，實際 %v", got)
	}
	items := FromDough([]store.DoughItem{{Name: "A", Pct: 100}})
	if got := Scale(items, 0); len(got) != 0 {
		t.Errorf("需求總重 0 應回傳空 map，實際 %v", got)
	}
	if got := Scale(items, -5); len(got) != 0 {
		t.Errorf("需求總重負值應回傳空 map，實際 %v", got)
	}
}
