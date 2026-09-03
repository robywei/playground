package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrInUse 表示目標被其他資料引用而無法刪除。呼叫端據此回傳 409。
var ErrInUse = errors.New("此項目正被其他資料引用")

// isForeignKeyViolation 判斷 error 是否為外鍵約束失敗。
// modernc.org/sqlite 不提供結構化的錯誤碼，只能比對訊息。
func isForeignKeyViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

// ListDoughs 列出所有產品配方，材料依儲存順序還原。
func (d *DB) ListDoughs() ([]Dough, error) {
	rows, err := d.sql.Query("SELECT id, name FROM doughs ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("查詢產品配方: %w", err)
	}
	defer rows.Close()

	out := []Dough{}
	index := map[string]int{}
	for rows.Next() {
		var g Dough
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, fmt.Errorf("掃描產品配方: %w", err)
		}
		g.Ingredients = []DoughItem{}
		index[g.ID] = len(out)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	irows, err := d.sql.Query(
		"SELECT dough_id, name, pct FROM dough_ingredients ORDER BY dough_id, sort")
	if err != nil {
		return nil, fmt.Errorf("查詢配方材料: %w", err)
	}
	defer irows.Close()
	for irows.Next() {
		var id string
		var it DoughItem
		if err := irows.Scan(&id, &it.Name, &it.Pct); err != nil {
			return nil, fmt.Errorf("掃描配方材料: %w", err)
		}
		if i, ok := index[id]; ok {
			out[i].Ingredients = append(out[i].Ingredients, it)
		}
	}
	return out, irows.Err()
}

// CreateDough 新增一份產品配方。ID 為空時自動產生。
func (d *DB) CreateDough(g Dough) (Dough, error) {
	if g.ID == "" {
		g.ID = NewID("d")
	}
	err := d.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT INTO doughs(id,name) VALUES(?,?)", g.ID, g.Name); err != nil {
			return fmt.Errorf("新增產品配方: %w", err)
		}
		return insertDoughItems(tx, g.ID, g.Ingredients)
	})
	if err != nil {
		return Dough{}, err
	}
	return g, nil
}

// UpdateDough 以完整內容取代既有配方。材料一律「全刪再全插」——
// 逐筆 diff 在材料改名或換順序時容易留下孤兒，且此規模下沒有效能必要。
func (d *DB) UpdateDough(g Dough) error {
	return d.inTx(func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE doughs SET name=? WHERE id=?", g.Name, g.ID)
		if err != nil {
			return fmt.Errorf("更新產品配方: %w", err)
		}
		if err := requireOneRow(res, "產品配方"); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM dough_ingredients WHERE dough_id=?", g.ID); err != nil {
			return fmt.Errorf("清除舊材料: %w", err)
		}
		return insertDoughItems(tx, g.ID, g.Ingredients)
	})
}

// DeleteDough 刪除一份產品配方。被商品引用時回傳 ErrInUse。
func (d *DB) DeleteDough(id string) error {
	res, err := d.sql.Exec("DELETE FROM doughs WHERE id=?", id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("產品配方: %w", ErrInUse)
		}
		return fmt.Errorf("刪除產品配方: %w", err)
	}
	return requireOneRow(res, "產品配方")
}

func insertDoughItems(tx *sql.Tx, doughID string, items []DoughItem) error {
	stmt, err := tx.Prepare(
		"INSERT INTO dough_ingredients(dough_id,name,pct,sort) VALUES(?,?,?,?)")
	if err != nil {
		return fmt.Errorf("準備寫入配方材料: %w", err)
	}
	defer stmt.Close()
	for i, it := range items {
		if _, err := stmt.Exec(doughID, it.Name, it.Pct, i); err != nil {
			return fmt.Errorf("寫入配方材料 %q: %w", it.Name, err)
		}
	}
	return nil
}

// ListFillings 列出所有配料，材料依儲存順序還原。
func (d *DB) ListFillings() ([]Filling, error) {
	rows, err := d.sql.Query("SELECT id, name FROM fillings ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("查詢配料: %w", err)
	}
	defer rows.Close()

	out := []Filling{}
	index := map[string]int{}
	for rows.Next() {
		var f Filling
		if err := rows.Scan(&f.ID, &f.Name); err != nil {
			return nil, fmt.Errorf("掃描配料: %w", err)
		}
		f.Ingredients = []FillingItem{}
		index[f.ID] = len(out)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	irows, err := d.sql.Query(
		"SELECT filling_id, name, weight_g FROM filling_ingredients ORDER BY filling_id, sort")
	if err != nil {
		return nil, fmt.Errorf("查詢配料材料: %w", err)
	}
	defer irows.Close()
	for irows.Next() {
		var id string
		var it FillingItem
		if err := irows.Scan(&id, &it.Name, &it.WeightG); err != nil {
			return nil, fmt.Errorf("掃描配料材料: %w", err)
		}
		if i, ok := index[id]; ok {
			out[i].Ingredients = append(out[i].Ingredients, it)
		}
	}
	return out, irows.Err()
}

// CreateFilling 新增一份配料。ID 為空時自動產生。
func (d *DB) CreateFilling(f Filling) (Filling, error) {
	if f.ID == "" {
		f.ID = NewID("f")
	}
	err := d.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT INTO fillings(id,name) VALUES(?,?)", f.ID, f.Name); err != nil {
			return fmt.Errorf("新增配料: %w", err)
		}
		return insertFillingItems(tx, f.ID, f.Ingredients)
	})
	if err != nil {
		return Filling{}, err
	}
	return f, nil
}

// UpdateFilling 以完整內容取代既有配料，材料全刪再全插。
func (d *DB) UpdateFilling(f Filling) error {
	return d.inTx(func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE fillings SET name=? WHERE id=?", f.Name, f.ID)
		if err != nil {
			return fmt.Errorf("更新配料: %w", err)
		}
		if err := requireOneRow(res, "配料"); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM filling_ingredients WHERE filling_id=?", f.ID); err != nil {
			return fmt.Errorf("清除舊材料: %w", err)
		}
		return insertFillingItems(tx, f.ID, f.Ingredients)
	})
}

// DeleteFilling 刪除一份配料。被商品引用時回傳 ErrInUse。
func (d *DB) DeleteFilling(id string) error {
	res, err := d.sql.Exec("DELETE FROM fillings WHERE id=?", id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("配料: %w", ErrInUse)
		}
		return fmt.Errorf("刪除配料: %w", err)
	}
	return requireOneRow(res, "配料")
}

func insertFillingItems(tx *sql.Tx, fillingID string, items []FillingItem) error {
	stmt, err := tx.Prepare(
		"INSERT INTO filling_ingredients(filling_id,name,weight_g,sort) VALUES(?,?,?,?)")
	if err != nil {
		return fmt.Errorf("準備寫入配料材料: %w", err)
	}
	defer stmt.Close()
	for i, it := range items {
		if _, err := stmt.Exec(fillingID, it.Name, it.WeightG, i); err != nil {
			return fmt.Errorf("寫入配料材料 %q: %w", it.Name, err)
		}
	}
	return nil
}

// inTx 在交易中執行 fn，fn 回傳非 nil 時整體回滾。
func (d *DB) inTx(fn func(*sql.Tx) error) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("開始交易: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交交易: %w", err)
	}
	return nil
}
