package store

import (
	"database/sql"
	"fmt"
)

// nullable 把空字串轉成 SQL NULL，讓選填的外鍵能通過約束。
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ListProducts 列出所有商品。
//
// 刻意不附加單顆成本 —— 成本是 domain 的責任，由 API 層組裝。
// 這讓 store 只做持久化。
func (d *DB) ListProducts() ([]Product, error) {
	rows, err := d.sql.Query(
		`SELECT id, name, price, dough_id, dough_weight_g,
		        fill1_id, fill1_weight_g, fill2_id, fill2_weight_g
		 FROM products ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("查詢商品: %w", err)
	}
	defer rows.Close()

	out := []Product{}
	for rows.Next() {
		var p Product
		var f1, f2 sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.DoughID, &p.DoughWeightG,
			&f1, &p.Fill1WeightG, &f2, &p.Fill2WeightG); err != nil {
			return nil, fmt.Errorf("掃描商品: %w", err)
		}
		p.Fill1ID, p.Fill2ID = f1.String, f2.String
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateProduct 新增一項商品。ID 為空時自動產生。
func (d *DB) CreateProduct(p Product) (Product, error) {
	if p.ID == "" {
		p.ID = NewID("prod")
	}
	_, err := d.sql.Exec(
		`INSERT INTO products(id,name,price,dough_id,dough_weight_g,
		                      fill1_id,fill1_weight_g,fill2_id,fill2_weight_g)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Price, p.DoughID, p.DoughWeightG,
		nullable(p.Fill1ID), p.Fill1WeightG, nullable(p.Fill2ID), p.Fill2WeightG)
	if err != nil {
		if isForeignKeyViolation(err) {
			return Product{}, fmt.Errorf("商品引用的配方或配料不存在: %w", ErrNotFound)
		}
		return Product{}, fmt.Errorf("新增商品: %w", err)
	}
	return p, nil
}

// UpdateProduct 以完整欄位取代既有商品。
func (d *DB) UpdateProduct(p Product) error {
	res, err := d.sql.Exec(
		`UPDATE products SET name=?, price=?, dough_id=?, dough_weight_g=?,
		        fill1_id=?, fill1_weight_g=?, fill2_id=?, fill2_weight_g=?
		 WHERE id=?`,
		p.Name, p.Price, p.DoughID, p.DoughWeightG,
		nullable(p.Fill1ID), p.Fill1WeightG, nullable(p.Fill2ID), p.Fill2WeightG, p.ID)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("商品引用的配方或配料不存在: %w", ErrNotFound)
		}
		return fmt.Errorf("更新商品: %w", err)
	}
	return requireOneRow(res, "商品")
}

// DeleteProduct 刪除一項商品。已成立的出貨與生產紀錄不受影響
// （外鍵是 ON DELETE SET NULL，名稱快照保留在該紀錄上）。
func (d *DB) DeleteProduct(id string) error {
	res, err := d.sql.Exec("DELETE FROM products WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("刪除商品: %w", err)
	}
	return requireOneRow(res, "商品")
}

// GetProduct 取得單一商品。
func (d *DB) GetProduct(id string) (Product, error) {
	var p Product
	var f1, f2 sql.NullString
	err := d.sql.QueryRow(
		`SELECT id, name, price, dough_id, dough_weight_g,
		        fill1_id, fill1_weight_g, fill2_id, fill2_weight_g
		 FROM products WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Price, &p.DoughID, &p.DoughWeightG,
			&f1, &p.Fill1WeightG, &f2, &p.Fill2WeightG)
	if err == sql.ErrNoRows {
		return Product{}, fmt.Errorf("商品 %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return Product{}, fmt.Errorf("查詢商品: %w", err)
	}
	p.Fill1ID, p.Fill2ID = f1.String, f2.String
	return p, nil
}
