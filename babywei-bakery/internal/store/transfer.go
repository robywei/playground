package store

import (
	"database/sql"
	"fmt"
)

// Snapshot 是資料庫的全量內容，用於匯出與匯入。
type Snapshot struct {
	Purchases      []Purchase      `json:"purchases"`
	Doughs         []Dough         `json:"doughs"`
	Fillings       []Filling       `json:"fillings"`
	Products       []Product       `json:"products"`
	Sales          []Sale          `json:"sales"`
	ProductionLogs []ProductionLog `json:"productionLogs"`
}

// ImportReport 回報匯入結果。
type ImportReport struct {
	Counts   map[string]int `json:"counts"`
	Warnings []string       `json:"warnings"`
}

// ExportAll 匯出全部資料。
func (d *DB) ExportAll() (Snapshot, error) {
	var s Snapshot
	var err error
	if s.Purchases, err = d.ListPurchases("", "", ""); err != nil {
		return Snapshot{}, err
	}
	if s.Doughs, err = d.ListDoughs(); err != nil {
		return Snapshot{}, err
	}
	if s.Fillings, err = d.ListFillings(); err != nil {
		return Snapshot{}, err
	}
	if s.Products, err = d.ListProducts(); err != nil {
		return Snapshot{}, err
	}
	if s.Sales, err = d.ListSales("", ""); err != nil {
		return Snapshot{}, err
	}
	if s.ProductionLogs, err = d.ListProductionLogs(); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

// 匯入時清空的順序：被引用者最後清，避免外鍵擋住。
var truncateOrder = []string{
	"production_consumption", "production_logs", "sales", "products",
	"dough_ingredients", "doughs", "filling_ingredients", "fillings", "purchases",
}

// ImportSnapshot 以 snapshot 完全取代資料庫內容。
//
// 這是破壞性操作：先清空全部表再寫入，任一步失敗整體回滾。
// 呼叫端應先呼叫 Backup()。
func (d *DB) ImportSnapshot(s Snapshot) (ImportReport, error) {
	rep := ImportReport{Counts: map[string]int{}, Warnings: []string{}}

	err := d.inTx(func(tx *sql.Tx) error {
		for _, table := range truncateOrder {
			if _, err := tx.Exec("DELETE FROM " + table); err != nil {
				return fmt.Errorf("清空 %s: %w", table, err)
			}
		}

		for _, p := range s.Purchases {
			if p.ID == "" {
				p.ID = NewID("p")
			}
			if _, err := tx.Exec(
				`INSERT INTO purchases(id,name,brand,purchase_date,channel,price,weight_g)
				 VALUES(?,?,?,?,?,?,?)`,
				p.ID, p.Name, p.Brand, p.PurchaseDate, p.Channel, p.Price, p.WeightG); err != nil {
				return fmt.Errorf("匯入進貨 %q: %w", p.Name, err)
			}
		}
		rep.Counts["purchases"] = len(s.Purchases)

		for _, g := range s.Doughs {
			if _, err := tx.Exec("INSERT INTO doughs(id,name) VALUES(?,?)", g.ID, g.Name); err != nil {
				return fmt.Errorf("匯入產品配方 %q: %w", g.Name, err)
			}
			if err := insertDoughItems(tx, g.ID, g.Ingredients); err != nil {
				return err
			}
		}
		rep.Counts["doughs"] = len(s.Doughs)

		for _, f := range s.Fillings {
			if _, err := tx.Exec("INSERT INTO fillings(id,name) VALUES(?,?)", f.ID, f.Name); err != nil {
				return fmt.Errorf("匯入配料 %q: %w", f.Name, err)
			}
			if err := insertFillingItems(tx, f.ID, f.Ingredients); err != nil {
				return err
			}
		}
		rep.Counts["fillings"] = len(s.Fillings)

		for _, p := range s.Products {
			if _, err := tx.Exec(
				`INSERT INTO products(id,name,price,dough_id,dough_weight_g,
				                      fill1_id,fill1_weight_g,fill2_id,fill2_weight_g)
				 VALUES(?,?,?,?,?,?,?,?,?)`,
				p.ID, p.Name, p.Price, p.DoughID, p.DoughWeightG,
				nullable(p.Fill1ID), p.Fill1WeightG, nullable(p.Fill2ID), p.Fill2WeightG); err != nil {
				return fmt.Errorf("匯入商品 %q: %w", p.Name, err)
			}
		}
		rep.Counts["products"] = len(s.Products)

		for _, sa := range s.Sales {
			if sa.ID == "" {
				sa.ID = NewID("s")
			}
			if _, err := tx.Exec(
				`INSERT INTO sales(id,sale_date,product_id,product_name,qty,unit_cost,unit_price)
				 VALUES(?,?,?,?,?,?,?)`,
				sa.ID, sa.SaleDate, nullable(sa.ProductID), sa.ProductName,
				sa.Qty, sa.UnitCost, sa.UnitPrice); err != nil {
				return fmt.Errorf("匯入出貨 %q: %w", sa.ProductName, err)
			}
		}
		rep.Counts["sales"] = len(s.Sales)

		var consumptionRows int
		for _, l := range s.ProductionLogs {
			if l.ID == "" {
				l.ID = NewID("log")
			}
			if _, err := tx.Exec(
				`INSERT INTO production_logs(id,logged_date,product_id,product_name,qty)
				 VALUES(?,?,?,?,?)`,
				l.ID, l.LoggedDate, nullable(l.ProductID), l.ProductName, l.Qty); err != nil {
				return fmt.Errorf("匯入生產紀錄 %q: %w", l.ProductName, err)
			}
			for _, c := range l.Consumption {
				if _, err := tx.Exec(
					`INSERT INTO production_consumption(log_id,ingredient_name,consumed_g)
					 VALUES(?,?,?)`, l.ID, c.IngredientName, c.ConsumedG); err != nil {
					return fmt.Errorf("匯入消耗明細 %q: %w", c.IngredientName, err)
				}
				consumptionRows++
			}
		}
		rep.Counts["productionLogs"] = len(s.ProductionLogs)
		rep.Counts["productionConsumption"] = consumptionRows
		return nil
	})
	if err != nil {
		return ImportReport{}, err
	}
	return rep, nil
}
