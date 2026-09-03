package store

import (
	"errors"
	"testing"
)

// seedRecipes 建立一份配方與一份配料，回傳其 ID。
func seedRecipes(t *testing.T, db *DB) (doughID, fillingID string) {
	t.Helper()
	g, err := db.CreateDough(Dough{Name: "基礎吐司", Ingredients: []DoughItem{
		{Name: "麵粉", Pct: 100}, {Name: "水", Pct: 100},
	}})
	if err != nil {
		t.Fatalf("seed dough: %v", err)
	}
	f, err := db.CreateFilling(Filling{Name: "南瓜泥", Ingredients: []FillingItem{
		{Name: "南瓜泥", WeightG: 100},
	}})
	if err != nil {
		t.Fatalf("seed filling: %v", err)
	}
	return g.ID, f.ID
}

func TestProductRoundTrip(t *testing.T) {
	db := memDB(t)
	dID, fID := seedRecipes(t, db)

	in := Product{Name: "南瓜吐司", Price: 180, DoughID: dID, DoughWeightG: 450,
		Fill1ID: fID, Fill1WeightG: 80}
	created, err := db.CreateProduct(in)
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	got, err := db.ListProducts()
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("商品數 = %d, want 1", len(got))
	}
	in.ID = created.ID
	if got[0] != in {
		t.Errorf("取回 %+v, want %+v", got[0], in)
	}
}

func TestProductEmptyFillingIDIsStoredAsNull(t *testing.T) {
	db := memDB(t)
	dID, _ := seedRecipes(t, db)
	// Fill1ID 為空字串：必須存成 NULL，否則外鍵會因為找不到 "" 而失敗
	if _, err := db.CreateProduct(Product{
		Name: "純吐司", Price: 100, DoughID: dID, DoughWeightG: 450,
	}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	got, _ := db.ListProducts()
	if got[0].Fill1ID != "" || got[0].Fill2ID != "" {
		t.Errorf("空配料應讀回空字串，實際 %q / %q", got[0].Fill1ID, got[0].Fill2ID)
	}
}

func TestCreateProductUnknownDough(t *testing.T) {
	db := memDB(t)
	_, err := db.CreateProduct(Product{
		Name: "X", Price: 1, DoughID: "d_nope", DoughWeightG: 1,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestDeleteDoughInUseIsRejected(t *testing.T) {
	db := memDB(t)
	dID, _ := seedRecipes(t, db)
	if _, err := db.CreateProduct(Product{
		Name: "X", Price: 1, DoughID: dID, DoughWeightG: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteDough(dID); !errors.Is(err, ErrInUse) {
		t.Errorf("error = %v, want ErrInUse（ON DELETE RESTRICT）", err)
	}
}

func TestDeleteFillingInUseIsRejected(t *testing.T) {
	db := memDB(t)
	dID, fID := seedRecipes(t, db)
	if _, err := db.CreateProduct(Product{
		Name: "X", Price: 1, DoughID: dID, DoughWeightG: 1, Fill1ID: fID, Fill1WeightG: 10,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteFilling(fID); !errors.Is(err, ErrInUse) {
		t.Errorf("error = %v, want ErrInUse", err)
	}
}

func TestGetProductNotFound(t *testing.T) {
	db := memDB(t)
	if _, err := db.GetProduct("prod_nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestRecipeSetIndexesByID(t *testing.T) {
	db := memDB(t)
	dID, fID := seedRecipes(t, db)
	set, err := db.RecipeSet()
	if err != nil {
		t.Fatalf("RecipeSet: %v", err)
	}
	if _, ok := set.Doughs[dID]; !ok {
		t.Errorf("配方 %q 不在 RecipeSet 中", dID)
	}
	if _, ok := set.Fillings[fID]; !ok {
		t.Errorf("配料 %q 不在 RecipeSet 中", fID)
	}
	if len(set.Doughs[dID].Ingredients) != 2 {
		t.Errorf("配方材料數 = %d, want 2", len(set.Doughs[dID].Ingredients))
	}
}
