package store

import (
	"errors"
	"testing"
)

func TestCreateSaleSnapshotsAreImmutable(t *testing.T) {
	db := memDB(t)
	dID, _ := seedRecipes(t, db)
	p, _ := db.CreateProduct(Product{Name: "吐司", Price: 180, DoughID: dID, DoughWeightG: 450})

	// 快照由 api 層算好傳入
	if _, err := db.CreateSale(Sale{
		SaleDate: "2026-09-03", ProductID: p.ID, ProductName: p.Name,
		Qty: 10, UnitCost: 60, UnitPrice: 180,
	}); err != nil {
		t.Fatalf("CreateSale: %v", err)
	}

	// 之後改售價
	p.Price = 999
	if err := db.UpdateProduct(p); err != nil {
		t.Fatal(err)
	}

	got, _ := db.ListSales("", "")
	if got[0].UnitPrice != 180 || got[0].UnitCost != 60 {
		t.Errorf("快照被改動: unitPrice=%v unitCost=%v, want 180 / 60",
			got[0].UnitPrice, got[0].UnitCost)
	}
}

func TestDeleteProductKeepsSalesHistory(t *testing.T) {
	db := memDB(t)
	dID, _ := seedRecipes(t, db)
	p, _ := db.CreateProduct(Product{Name: "吐司", Price: 180, DoughID: dID, DoughWeightG: 450})
	if _, err := db.CreateSale(Sale{
		SaleDate: "2026-09-03", ProductID: p.ID, ProductName: p.Name,
		Qty: 5, UnitCost: 60, UnitPrice: 180,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteProduct(p.ID); err != nil {
		t.Fatalf("DeleteProduct: %v", err)
	}

	got, _ := db.ListSales("", "")
	if len(got) != 1 {
		t.Fatalf("出貨紀錄應保留，實際 %d 筆", len(got))
	}
	if got[0].ProductID != "" {
		t.Errorf("ProductID = %q, want 空字串（ON DELETE SET NULL）", got[0].ProductID)
	}
	if got[0].ProductName != "吐司" {
		t.Errorf("ProductName = %q, 名稱快照應保留", got[0].ProductName)
	}
	if got[0].UnitPrice != 180 {
		t.Errorf("金額快照應保留，實際 %v", got[0].UnitPrice)
	}
}

func TestListSalesFiltersByDateRange(t *testing.T) {
	db := memDB(t)
	for _, d := range []string{"2026-08-31", "2026-09-01", "2026-09-30", "2026-10-01"} {
		if _, err := db.CreateSale(Sale{
			SaleDate: d, ProductName: "X", Qty: 1, UnitCost: 1, UnitPrice: 2,
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := db.ListSales("2026-09-01", "2026-09-30")
	if len(got) != 2 {
		t.Errorf("區間含端點應得 2 筆，實際 %d 筆", len(got))
	}
}

func TestConfirmProductionWritesConsumption(t *testing.T) {
	db := memDB(t)
	dID, _ := seedRecipes(t, db)
	p, _ := db.CreateProduct(Product{Name: "吐司", Price: 180, DoughID: dID, DoughWeightG: 200})

	log, err := db.ConfirmProduction(
		ProductionLog{LoggedDate: "2026-09-03", ProductID: p.ID, ProductName: p.Name, Qty: 10},
		map[string]float64{"麵粉": 1000, "水": 1000},
	)
	if err != nil {
		t.Fatalf("ConfirmProduction: %v", err)
	}
	if len(log.Consumption) != 2 {
		t.Fatalf("回傳消耗筆數 = %d, want 2", len(log.Consumption))
	}
	// 回傳的順序須穩定（依材料名稱）
	if log.Consumption[0].IngredientName > log.Consumption[1].IngredientName {
		t.Error("消耗明細未依名稱排序")
	}

	got, err := db.ListProductionLogs()
	if err != nil {
		t.Fatalf("ListProductionLogs: %v", err)
	}
	if len(got) != 1 || len(got[0].Consumption) != 2 {
		t.Fatalf("落庫的消耗明細不正確: %+v", got)
	}
}

func TestConfirmProductionIsAtomic(t *testing.T) {
	db := memDB(t)
	dID, _ := seedRecipes(t, db)
	p, _ := db.CreateProduct(Product{Name: "吐司", Price: 180, DoughID: dID, DoughWeightG: 200})

	// consumed_g 為負會觸發 CHECK，整筆生產紀錄都不該留下 ——
	// 否則會出現「有生產紀錄但沒有消耗明細」的半套資料，庫存憑空增加
	_, err := db.ConfirmProduction(
		ProductionLog{LoggedDate: "2026-09-03", ProductID: p.ID, ProductName: p.Name, Qty: 1},
		map[string]float64{"麵粉": 100, "水": -1},
	)
	if err == nil {
		t.Fatal("負數消耗應被 CHECK 拒絕")
	}
	got, _ := db.ListProductionLogs()
	if len(got) != 0 {
		t.Errorf("交易未回滾，殘留 %d 筆生產紀錄", len(got))
	}
	var n int
	if err := db.SQL().QueryRow("SELECT count(*) FROM production_consumption").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("交易未回滾，殘留 %d 筆消耗明細", n)
	}
}

func TestConfirmProductionSurvivesRecipeChange(t *testing.T) {
	db := memDB(t)
	dID, _ := seedRecipes(t, db)
	p, _ := db.CreateProduct(Product{Name: "吐司", Price: 180, DoughID: dID, DoughWeightG: 200})

	if _, err := db.ConfirmProduction(
		ProductionLog{LoggedDate: "2026-09-03", ProductID: p.ID, ProductName: p.Name, Qty: 1},
		map[string]float64{"麵粉": 100, "水": 100},
	); err != nil {
		t.Fatal(err)
	}

	// 大幅改動配方
	if err := db.UpdateDough(Dough{ID: dID, Name: "改過的配方", Ingredients: []DoughItem{
		{Name: "全麥麵粉", Pct: 100},
	}}); err != nil {
		t.Fatal(err)
	}

	got, _ := db.ListProductionLogs()
	if len(got[0].Consumption) != 2 {
		t.Fatalf("消耗明細筆數 = %d, want 2（改配方不該影響歷史）", len(got[0].Consumption))
	}
	for _, c := range got[0].Consumption {
		if c.ConsumedG != 100 {
			t.Errorf("%s 消耗 = %v, want 100", c.IngredientName, c.ConsumedG)
		}
	}
}

func TestDeleteProductKeepsProductionConsumption(t *testing.T) {
	db := memDB(t)
	dID, _ := seedRecipes(t, db)
	p, _ := db.CreateProduct(Product{Name: "吐司", Price: 180, DoughID: dID, DoughWeightG: 200})
	if _, err := db.ConfirmProduction(
		ProductionLog{LoggedDate: "2026-09-03", ProductID: p.ID, ProductName: p.Name, Qty: 1},
		map[string]float64{"麵粉": 100},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteProduct(p.ID); err != nil {
		t.Fatalf("DeleteProduct: %v", err)
	}

	got, _ := db.ListProductionLogs()
	if len(got) != 1 {
		t.Fatalf("生產紀錄應保留，實際 %d 筆", len(got))
	}
	if len(got[0].Consumption) != 1 || got[0].Consumption[0].ConsumedG != 100 {
		t.Errorf("消耗明細應保留: %+v", got[0].Consumption)
	}
	if got[0].ProductName != "吐司" {
		t.Errorf("名稱快照應保留，實際 %q", got[0].ProductName)
	}
}

func TestDeleteSaleNotFound(t *testing.T) {
	db := memDB(t)
	if err := db.DeleteSale("s_nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}
