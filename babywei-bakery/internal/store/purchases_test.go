package store

import (
	"errors"
	"strings"
	"testing"
)

func memDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreatePurchaseGeneratesID(t *testing.T) {
	db := memDB(t)
	got, err := db.CreatePurchase(Purchase{
		Name: "高筋麵粉", PurchaseDate: "2026-09-03", Price: 120, WeightG: 1000,
	})
	if err != nil {
		t.Fatalf("CreatePurchase: %v", err)
	}
	if !strings.HasPrefix(got.ID, "p_") {
		t.Errorf("ID = %q, 應有前綴 p_", got.ID)
	}
}

func TestCreatePurchaseKeepsGivenID(t *testing.T) {
	db := memDB(t)
	got, err := db.CreatePurchase(Purchase{
		ID: "p_fixed", Name: "糖", PurchaseDate: "2026-09-03", Price: 40, WeightG: 1000,
	})
	if err != nil {
		t.Fatalf("CreatePurchase: %v", err)
	}
	if got.ID != "p_fixed" {
		t.Errorf("ID = %q, want p_fixed", got.ID)
	}
}

func seedPurchases(t *testing.T, db *DB) {
	t.Helper()
	for _, p := range []Purchase{
		{Name: "高筋麵粉", Brand: "水手牌", PurchaseDate: "2026-08-01", Channel: "烘焙材料行", Price: 120, WeightG: 1000},
		{Name: "牛奶", Brand: "光泉", PurchaseDate: "2026-09-01", Channel: "全聯", Price: 90, WeightG: 1000},
		{Name: "無鹽奶油", Brand: "鐵塔牌", PurchaseDate: "2026-09-15", Channel: "烘焙材料行", Price: 180, WeightG: 500},
	} {
		if _, err := db.CreatePurchase(p); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func TestListPurchasesSortedByDateDesc(t *testing.T) {
	db := memDB(t)
	seedPurchases(t, db)
	got, err := db.ListPurchases("", "", "")
	if err != nil {
		t.Fatalf("ListPurchases: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("筆數 = %d, want 3", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].PurchaseDate < got[i].PurchaseDate {
			t.Errorf("未依日期新到舊排序: %q 在 %q 之前", got[i-1].PurchaseDate, got[i].PurchaseDate)
		}
	}
}

func TestListPurchasesFiltersByDateRange(t *testing.T) {
	db := memDB(t)
	seedPurchases(t, db)

	got, err := db.ListPurchases("2026-09-01", "2026-09-15", "")
	if err != nil {
		t.Fatalf("ListPurchases: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("區間含端點應得 2 筆，實際 %d 筆", len(got))
	}
	if got, _ := db.ListPurchases("2026-09-02", "", ""); len(got) != 1 {
		t.Errorf("只設下界應得 1 筆，實際 %d 筆", len(got))
	}
	if got, _ := db.ListPurchases("", "2026-08-31", ""); len(got) != 1 {
		t.Errorf("只設上界應得 1 筆，實際 %d 筆", len(got))
	}
}

func TestListPurchasesFiltersByKeyword(t *testing.T) {
	db := memDB(t)
	seedPurchases(t, db)

	cases := []struct {
		q    string
		want int
		why  string
	}{
		{"麵粉", 1, "命中名稱"},
		{"光泉", 1, "命中品牌"},
		{"烘焙材料行", 2, "命中管道"},
		{"PCHOME", 0, "無命中"},
		{"", 3, "空關鍵字不過濾"},
	}
	for _, c := range cases {
		got, err := db.ListPurchases("", "", c.q)
		if err != nil {
			t.Fatalf("ListPurchases(%q): %v", c.q, err)
		}
		if len(got) != c.want {
			t.Errorf("q=%q (%s): %d 筆, want %d", c.q, c.why, len(got), c.want)
		}
	}
}

func TestListPurchasesKeywordIsCaseInsensitive(t *testing.T) {
	db := memDB(t)
	if _, err := db.CreatePurchase(Purchase{
		Name: "Bread Flour", PurchaseDate: "2026-09-03", Price: 1, WeightG: 1,
	}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"bread", "BREAD", "Bread"} {
		got, _ := db.ListPurchases("", "", q)
		if len(got) != 1 {
			t.Errorf("q=%q 應命中 1 筆，實際 %d 筆", q, len(got))
		}
	}
}

func TestUpdatePurchase(t *testing.T) {
	db := memDB(t)
	p, _ := db.CreatePurchase(Purchase{
		Name: "糖", PurchaseDate: "2026-09-03", Price: 40, WeightG: 1000,
	})
	p.Price = 55
	p.Brand = "台糖"
	if err := db.UpdatePurchase(p); err != nil {
		t.Fatalf("UpdatePurchase: %v", err)
	}
	got, _ := db.ListPurchases("", "", "")
	if got[0].Price != 55 || got[0].Brand != "台糖" {
		t.Errorf("更新未生效: %+v", got[0])
	}
}

func TestUpdatePurchaseNotFound(t *testing.T) {
	db := memDB(t)
	err := db.UpdatePurchase(Purchase{
		ID: "p_nope", Name: "糖", PurchaseDate: "2026-09-03", Price: 1, WeightG: 1,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestDeletePurchaseTwice(t *testing.T) {
	db := memDB(t)
	p, _ := db.CreatePurchase(Purchase{
		Name: "鹽", PurchaseDate: "2026-09-03", Price: 20, WeightG: 1000,
	})
	if err := db.DeletePurchase(p.ID); err != nil {
		t.Fatalf("第一次刪除: %v", err)
	}
	if err := db.DeletePurchase(p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("第二次刪除 error = %v, want ErrNotFound", err)
	}
}

func TestPurchaseCheckConstraints(t *testing.T) {
	db := memDB(t)
	if _, err := db.CreatePurchase(Purchase{
		Name: "X", PurchaseDate: "2026-09-03", Price: 1, WeightG: 0,
	}); err == nil {
		t.Error("weight_g = 0 應被 CHECK 拒絕")
	}
	if _, err := db.CreatePurchase(Purchase{
		Name: "X", PurchaseDate: "2026-09-03", Price: -1, WeightG: 1,
	}); err == nil {
		t.Error("price < 0 應被 CHECK 拒絕")
	}
}

func TestListPurchasesEmptyReturnsEmptySlice(t *testing.T) {
	db := memDB(t)
	got, err := db.ListPurchases("", "", "")
	if err != nil {
		t.Fatalf("ListPurchases: %v", err)
	}
	if got == nil {
		t.Error("應回傳空 slice 而非 nil —— JSON 會序列化成 null")
	}
}
