package store

import (
	"database/sql"
	"fmt"
	"sort"
)

// ListProductionLogs 列出生產紀錄（含消耗快照），依日期新到舊。
func (d *DB) ListProductionLogs() ([]ProductionLog, error) {
	rows, err := d.sql.Query(
		`SELECT id, logged_date, product_id, product_name, qty
		 FROM production_logs ORDER BY logged_date DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("查詢生產紀錄: %w", err)
	}
	defer rows.Close()

	out := []ProductionLog{}
	index := map[string]int{}
	for rows.Next() {
		var l ProductionLog
		var pid sql.NullString
		if err := rows.Scan(&l.ID, &l.LoggedDate, &pid, &l.ProductName, &l.Qty); err != nil {
			return nil, fmt.Errorf("掃描生產紀錄: %w", err)
		}
		l.ProductID = pid.String
		l.Consumption = []Consumption{}
		index[l.ID] = len(out)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	crows, err := d.sql.Query(
		`SELECT log_id, ingredient_name, consumed_g
		 FROM production_consumption ORDER BY log_id, ingredient_name`)
	if err != nil {
		return nil, fmt.Errorf("查詢生產消耗: %w", err)
	}
	defer crows.Close()
	for crows.Next() {
		var logID string
		var c Consumption
		if err := crows.Scan(&logID, &c.IngredientName, &c.ConsumedG); err != nil {
			return nil, fmt.Errorf("掃描生產消耗: %w", err)
		}
		if i, ok := index[logID]; ok {
			out[i].Consumption = append(out[i].Consumption, c)
		}
	}
	return out, crows.Err()
}

// ConfirmProduction 在單一交易內寫入生產紀錄與其原料消耗快照。
//
// consumption 由呼叫端（api 層）以 domain.ProductConsumption 算好再傳進來。
// 消耗量在此刻寫死：之後改配方或刪商品都不會回溯改變歷史庫存數字，
// 這是原型行為的關鍵修正。
//
// 任一步失敗整體回滾 —— 不能出現「有生產紀錄但沒有消耗明細」的半套資料，
// 那會讓庫存憑空增加。
func (d *DB) ConfirmProduction(l ProductionLog, consumption map[string]float64) (ProductionLog, error) {
	if l.ID == "" {
		l.ID = NewID("log")
	}
	// 輸出順序穩定，方便測試與前端顯示
	names := make([]string, 0, len(consumption))
	for n := range consumption {
		names = append(names, n)
	}
	sort.Strings(names)

	l.Consumption = make([]Consumption, 0, len(names))
	for _, n := range names {
		l.Consumption = append(l.Consumption, Consumption{IngredientName: n, ConsumedG: consumption[n]})
	}

	err := d.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`INSERT INTO production_logs(id, logged_date, product_id, product_name, qty)
			 VALUES(?,?,?,?,?)`,
			l.ID, l.LoggedDate, nullable(l.ProductID), l.ProductName, l.Qty); err != nil {
			return fmt.Errorf("新增生產紀錄: %w", err)
		}
		stmt, err := tx.Prepare(
			`INSERT INTO production_consumption(log_id, ingredient_name, consumed_g)
			 VALUES(?,?,?)`)
		if err != nil {
			return fmt.Errorf("準備寫入消耗明細: %w", err)
		}
		defer stmt.Close()
		for _, c := range l.Consumption {
			if _, err := stmt.Exec(l.ID, c.IngredientName, c.ConsumedG); err != nil {
				return fmt.Errorf("寫入消耗明細 %q: %w", c.IngredientName, err)
			}
		}
		return nil
	})
	if err != nil {
		return ProductionLog{}, err
	}
	return l, nil
}
