package store

import (
	"errors"
	"testing"
)

func TestDoughRoundTripPreservesOrder(t *testing.T) {
	db := memDB(t)
	in := Dough{Name: "基礎吐司麵團", Ingredients: []DoughItem{
		{Name: "高筋麵粉", Pct: 100},
		{Name: "牛奶", Pct: 65},
		{Name: "糖", Pct: 8},
		{Name: "無鹽奶油", Pct: 10},
	}}
	created, err := db.CreateDough(in)
	if err != nil {
		t.Fatalf("CreateDough: %v", err)
	}
	got, err := db.ListDoughs()
	if err != nil {
		t.Fatalf("ListDoughs: %v", err)
	}
	if len(got) != 1 || got[0].ID != created.ID {
		t.Fatalf("取回 %d 份配方: %+v", len(got), got)
	}
	if len(got[0].Ingredients) != len(in.Ingredients) {
		t.Fatalf("材料數 = %d, want %d", len(got[0].Ingredients), len(in.Ingredients))
	}
	for i, want := range in.Ingredients {
		if got[0].Ingredients[i] != want {
			t.Errorf("材料[%d] = %+v, want %+v（順序須還原）", i, got[0].Ingredients[i], want)
		}
	}
}

func TestUpdateDoughReplacesIngredients(t *testing.T) {
	db := memDB(t)
	g, _ := db.CreateDough(Dough{Name: "A", Ingredients: []DoughItem{
		{Name: "麵粉", Pct: 100}, {Name: "水", Pct: 70}, {Name: "鹽", Pct: 2},
	}})
	g.Name = "A 改版"
	g.Ingredients = []DoughItem{{Name: "麵粉", Pct: 100}, {Name: "牛奶", Pct: 60}}
	if err := db.UpdateDough(g); err != nil {
		t.Fatalf("UpdateDough: %v", err)
	}
	got, _ := db.ListDoughs()
	if got[0].Name != "A 改版" {
		t.Errorf("名稱 = %q, want A 改版", got[0].Name)
	}
	if len(got[0].Ingredients) != 2 {
		t.Fatalf("材料數 = %d, want 2（舊材料須被完全取代）", len(got[0].Ingredients))
	}
	for _, it := range got[0].Ingredients {
		if it.Name == "水" || it.Name == "鹽" {
			t.Errorf("殘留舊材料 %q", it.Name)
		}
	}
}

func TestUpdateDoughNotFound(t *testing.T) {
	db := memDB(t)
	err := db.UpdateDough(Dough{ID: "d_nope", Name: "X"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestDeleteDoughCascadesIngredients(t *testing.T) {
	db := memDB(t)
	g, _ := db.CreateDough(Dough{Name: "A", Ingredients: []DoughItem{{Name: "麵粉", Pct: 100}}})
	if err := db.DeleteDough(g.ID); err != nil {
		t.Fatalf("DeleteDough: %v", err)
	}
	var n int
	if err := db.SQL().QueryRow("SELECT count(*) FROM dough_ingredients").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("CASCADE 未生效，殘留 %d 筆材料", n)
	}
}

func TestDeleteDoughNotFound(t *testing.T) {
	db := memDB(t)
	if err := db.DeleteDough("d_nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestCreateDoughRollsBackOnBadIngredient(t *testing.T) {
	db := memDB(t)
	// pct <= 0 觸發 CHECK，整份配方都不該留下
	_, err := db.CreateDough(Dough{Name: "壞配方", Ingredients: []DoughItem{
		{Name: "麵粉", Pct: 100},
		{Name: "壞的", Pct: 0},
	}})
	if err == nil {
		t.Fatal("pct = 0 應被拒絕")
	}
	got, _ := db.ListDoughs()
	if len(got) != 0 {
		t.Errorf("交易未回滾，殘留 %d 份配方", len(got))
	}
}

func TestDoughNameIsUnique(t *testing.T) {
	db := memDB(t)
	if _, err := db.CreateDough(Dough{Name: "重複"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateDough(Dough{Name: "重複"}); err == nil {
		t.Error("同名配方應被 UNIQUE 拒絕")
	}
}

func TestFillingRoundTripPreservesOrder(t *testing.T) {
	db := memDB(t)
	in := Filling{Name: "奶酥餡", Ingredients: []FillingItem{
		{Name: "無鹽奶油", WeightG: 100},
		{Name: "糖粉", WeightG: 60},
		{Name: "奶粉", WeightG: 40},
	}}
	if _, err := db.CreateFilling(in); err != nil {
		t.Fatalf("CreateFilling: %v", err)
	}
	got, err := db.ListFillings()
	if err != nil {
		t.Fatalf("ListFillings: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("取回 %d 份配料", len(got))
	}
	for i, want := range in.Ingredients {
		if got[0].Ingredients[i] != want {
			t.Errorf("材料[%d] = %+v, want %+v", i, got[0].Ingredients[i], want)
		}
	}
}

func TestUpdateFillingReplacesIngredients(t *testing.T) {
	db := memDB(t)
	f, _ := db.CreateFilling(Filling{Name: "A", Ingredients: []FillingItem{
		{Name: "南瓜泥", WeightG: 100}, {Name: "糖", WeightG: 20},
	}})
	f.Ingredients = []FillingItem{{Name: "南瓜泥", WeightG: 120}}
	if err := db.UpdateFilling(f); err != nil {
		t.Fatalf("UpdateFilling: %v", err)
	}
	got, _ := db.ListFillings()
	if len(got[0].Ingredients) != 1 {
		t.Errorf("材料數 = %d, want 1", len(got[0].Ingredients))
	}
	if got[0].Ingredients[0].WeightG != 120 {
		t.Errorf("重量 = %v, want 120", got[0].Ingredients[0].WeightG)
	}
}

func TestDeleteFillingCascades(t *testing.T) {
	db := memDB(t)
	f, _ := db.CreateFilling(Filling{Name: "A", Ingredients: []FillingItem{{Name: "南瓜泥", WeightG: 100}}})
	if err := db.DeleteFilling(f.ID); err != nil {
		t.Fatalf("DeleteFilling: %v", err)
	}
	var n int
	if err := db.SQL().QueryRow("SELECT count(*) FROM filling_ingredients").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("CASCADE 未生效，殘留 %d 筆", n)
	}
}

func TestListRecipesEmptyReturnsEmptySlice(t *testing.T) {
	db := memDB(t)
	if got, _ := db.ListDoughs(); got == nil {
		t.Error("ListDoughs 應回傳空 slice 而非 nil")
	}
	if got, _ := db.ListFillings(); got == nil {
		t.Error("ListFillings 應回傳空 slice 而非 nil")
	}
}
