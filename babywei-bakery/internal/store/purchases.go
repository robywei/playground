package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound 表示目標列不存在。呼叫端據此回傳 404。
var ErrNotFound = errors.New("找不到指定的資料")

// ListPurchases 列出進貨紀錄，依購入日期新到舊。
// from / to 為空字串代表該側不設限；q 為空代表不做關鍵字過濾，
// 否則比對材料名稱、品牌、購入管道（不分大小寫）。
func (d *DB) ListPurchases(from, to, q string) ([]Purchase, error) {
	query := `SELECT id, name, brand, purchase_date, channel, price, weight_g
	          FROM purchases WHERE 1=1`
	var args []any
	if from != "" {
		query += " AND purchase_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND purchase_date <= ?"
		args = append(args, to)
	}
	if q != "" {
		// LIKE 對 ASCII 預設不分大小寫；中文無大小寫之分，一併用 lower 保險。
		query += ` AND (lower(name) LIKE ? OR lower(brand) LIKE ? OR lower(channel) LIKE ?)`
		like := "%" + strings.ToLower(q) + "%"
		args = append(args, like, like, like)
	}
	query += " ORDER BY purchase_date DESC, id DESC"

	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢進貨紀錄: %w", err)
	}
	defer rows.Close()

	out := []Purchase{}
	for rows.Next() {
		var p Purchase
		if err := rows.Scan(&p.ID, &p.Name, &p.Brand, &p.PurchaseDate,
			&p.Channel, &p.Price, &p.WeightG); err != nil {
			return nil, fmt.Errorf("掃描進貨紀錄: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreatePurchase 新增一筆進貨紀錄。ID 為空時自動產生。
func (d *DB) CreatePurchase(p Purchase) (Purchase, error) {
	if p.ID == "" {
		p.ID = NewID("p")
	}
	_, err := d.sql.Exec(
		`INSERT INTO purchases(id, name, brand, purchase_date, channel, price, weight_g)
		 VALUES(?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Brand, p.PurchaseDate, p.Channel, p.Price, p.WeightG)
	if err != nil {
		return Purchase{}, fmt.Errorf("新增進貨紀錄: %w", err)
	}
	return p, nil
}

// UpdatePurchase 以完整欄位取代既有紀錄。找不到時回傳 ErrNotFound。
func (d *DB) UpdatePurchase(p Purchase) error {
	res, err := d.sql.Exec(
		`UPDATE purchases SET name=?, brand=?, purchase_date=?, channel=?, price=?, weight_g=?
		 WHERE id=?`,
		p.Name, p.Brand, p.PurchaseDate, p.Channel, p.Price, p.WeightG, p.ID)
	if err != nil {
		return fmt.Errorf("更新進貨紀錄: %w", err)
	}
	return requireOneRow(res, "進貨紀錄")
}

// DeletePurchase 刪除一筆進貨紀錄。找不到時回傳 ErrNotFound。
func (d *DB) DeletePurchase(id string) error {
	res, err := d.sql.Exec("DELETE FROM purchases WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("刪除進貨紀錄: %w", err)
	}
	return requireOneRow(res, "進貨紀錄")
}

// requireOneRow 把「WHERE 沒命中」轉成 ErrNotFound。
// SQLite 的 UPDATE / DELETE 不會因為條件沒中而報錯，必須自己檢查。
func requireOneRow(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("讀取影響列數: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", what, ErrNotFound)
	}
	return nil
}
