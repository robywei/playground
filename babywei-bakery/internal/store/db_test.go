package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMemoryRunsMigration(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	v, err := db.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != 1 {
		t.Errorf("schema version = %d, want 1", v)
	}

	for _, table := range []string{
		"purchases", "doughs", "dough_ingredients", "fillings",
		"filling_ingredients", "products", "sales",
		"production_logs", "production_consumption",
	} {
		var n int
		if err := db.SQL().QueryRow(
			"SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&n); err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %s missing", table)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	for i := range 3 {
		db, err := Open(dir)
		if err != nil {
			t.Fatalf("Open #%d: %v", i+1, err)
		}
		v, err := db.SchemaVersion()
		if err != nil {
			t.Fatalf("SchemaVersion #%d: %v", i+1, err)
		}
		if v != 1 {
			t.Errorf("開啟 #%d: schema version = %d, want 1", i+1, v)
		}
		db.Close()
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	var fk int
	if err := db.SQL().QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1 —— CASCADE 會靜默失效", fk)
	}

	if _, err := db.SQL().Exec(
		"INSERT INTO dough_ingredients(dough_id, name, pct, sort) VALUES('nope','麵粉',100,0)",
	); err == nil {
		t.Error("插入孤兒 dough_ingredients 應該失敗，實際成功")
	}

	if _, err := db.SQL().Exec("INSERT INTO doughs(id,name) VALUES('d1','基礎')"); err != nil {
		t.Fatalf("insert dough: %v", err)
	}
	if _, err := db.SQL().Exec(
		"INSERT INTO dough_ingredients(dough_id,name,pct,sort) VALUES('d1','麵粉',100,0)",
	); err != nil {
		t.Fatalf("insert ingredient: %v", err)
	}
	if _, err := db.SQL().Exec("DELETE FROM doughs WHERE id='d1'"); err != nil {
		t.Fatalf("delete dough: %v", err)
	}
	var n int
	if err := db.SQL().QueryRow("SELECT count(*) FROM dough_ingredients").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("CASCADE 未生效，殘留 %d 筆", n)
	}
}

func TestJournalModeIsDelete(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.SQL().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "delete" {
		t.Errorf("journal_mode = %q, want %q —— WAL 會破壞「複製資料夾即備份」", mode, "delete")
	}
}

func TestFirstOpenCreatesNoBackup(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	entries, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err == nil && len(entries) != 0 {
		t.Errorf("首次啟動不該有備份，實際 %d 份", len(entries))
	}
}

func TestSecondOpenCreatesBackup(t *testing.T) {
	dir := t.TempDir()

	db1, err := Open(dir)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := db1.SQL().Exec("INSERT INTO doughs(id,name) VALUES('d1','基礎')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	db1.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer db2.Close()

	entries, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatalf("read backups dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("第二次啟動未產生備份")
	}
	// 備份必須是可讀的完整資料庫，而非空檔
	bk, err := OpenExistingForTest(filepath.Join(dir, "backups", entries[0].Name()))
	if err != nil {
		t.Fatalf("開啟備份: %v", err)
	}
	defer bk.Close()
	var n int
	if err := bk.SQL().QueryRow("SELECT count(*) FROM doughs").Scan(&n); err != nil {
		t.Fatalf("查詢備份: %v", err)
	}
	if n != 1 {
		t.Errorf("備份內 doughs = %d 筆, want 1", n)
	}
}

func TestBackupRotationKeepsLimit(t *testing.T) {
	dir := t.TempDir()

	// 先產生一份真實資料庫 —— backup() 在資料庫不存在時會提前返回，
	// 空目錄下輪替根本不會執行。
	db0, err := Open(dir)
	if err != nil {
		t.Fatalf("initial Open: %v", err)
	}
	db0.Close()

	bdir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 塞 35 份假備份，檔名時間戳都早於現在
	for i := range 35 {
		name := filepath.Join(bdir, fmt.Sprintf("2020-01-01-%06d.db", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	entries, err := os.ReadDir(bdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != backupKeep {
		t.Errorf("備份數 = %d, 應輪替至 %d", len(entries), backupKeep)
	}
	// 保留的必須是最新的：假備份是 2020-01-01-*，新產生的是今天
	// 輪替後 2020 那批應只剩最後 29 份
	var old int
	for _, e := range entries {
		if len(e.Name()) >= 4 && e.Name()[:4] == "2020" {
			old++
		}
	}
	if old != backupKeep-1 {
		t.Errorf("殘留的舊備份 = %d 份, want %d（應刪最舊的）", old, backupKeep-1)
	}
}
