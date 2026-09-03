package store

import (
	"database/sql"
	"fmt"
)

// ListSales 列出出貨紀錄，依出貨日期新到舊。
// from / to 為空字串代表該側不設限。
func (d *DB) ListSales(from, to string) ([]Sale, error) {
	query := `SELECT id, sale_date, product_id, product_name, qty, unit_cost, unit_price
	          FROM sales WHERE 1=1`
	var args []any
	if from != "" {
		query += " AND sale_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND sale_date <= ?"
		args = append(args, to)
	}
	query += " ORDER BY sale_date DESC, id DESC"

	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢出貨紀錄: %w", err)
	}
	defer rows.Close()

	out := []Sale{}
	for rows.Next() {
		var s Sale
		var pid sql.NullString
		if err := rows.Scan(&s.ID, &s.SaleDate, &pid, &s.ProductName,
			&s.Qty, &s.UnitCost, &s.UnitPrice); err != nil {
			return nil, fmt.Errorf("掃描出貨紀錄: %w", err)
		}
		s.ProductID = pid.String
		out = append(out, s)
	}
	return out, rows.Err()
}

// CreateSale 寫入一筆出貨紀錄。
//
// UnitCost 與 UnitPrice 由呼叫端（api 層）先以 domain 算好再傳進來 ——
// store 不能 import domain（分層是 store ← domain ← api）。這兩個值是
// 寫入當下的快照：之後改配方、改進貨價或改售價都不影響已成立的紀錄。
func (d *DB) CreateSale(s Sale) (Sale, error) {
	if s.ID == "" {
		s.ID = NewID("s")
	}
	_, err := d.sql.Exec(
		`INSERT INTO sales(id, sale_date, product_id, product_name, qty, unit_cost, unit_price)
		 VALUES(?,?,?,?,?,?,?)`,
		s.ID, s.SaleDate, nullable(s.ProductID), s.ProductName, s.Qty, s.UnitCost, s.UnitPrice)
	if err != nil {
		return Sale{}, fmt.Errorf("新增出貨紀錄: %w", err)
	}
	return s, nil
}

// DeleteSale 刪除一筆出貨紀錄。
func (d *DB) DeleteSale(id string) error {
	res, err := d.sql.Exec("DELETE FROM sales WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("刪除出貨紀錄: %w", err)
	}
	return requireOneRow(res, "出貨紀錄")
}
