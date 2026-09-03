// Package store 負責 SQLite 的連線、schema 遷移、備份，以及所有 SQL。
package store

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// backupKeep 是 data/backups/ 保留的備份份數。
const backupKeep = 30

// dbFileName 是資料庫檔名。
const dbFileName = "babywei.db"

// DB 包住 *sql.DB，並持有資料目錄以便做備份。
type DB struct {
	sql     *sql.DB
	dataDir string
}

// SQL 回傳底層的 *sql.DB。
func (d *DB) SQL() *sql.DB { return d.sql }

// Close 關閉連線。
func (d *DB) Close() error { return d.sql.Close() }

// Open 開啟 <dataDir>/babywei.db，先做啟動備份再跑 migration。
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("建立資料目錄: %w", err)
	}
	d, err := open(filepath.Join(dataDir, dbFileName), dataDir)
	if err != nil {
		return nil, err
	}
	// 備份排在 migration 之前：遷移失敗時手上還有遷移前的快照。
	if err := d.backup(); err != nil {
		d.Close()
		return nil, err
	}
	if err := d.migrate(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// OpenMemory 開啟記憶體資料庫並跑 migration，不做備份。供測試用。
func OpenMemory() (*DB, error) {
	d, err := open(":memory:", "")
	if err != nil {
		return nil, err
	}
	if err := d.migrate(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// OpenExistingForTest 開啟既有的資料庫檔而不做遷移或備份，
// 供測試檢查備份檔內容用。
func OpenExistingForTest(dbPath string) (*DB, error) {
	return open(dbPath, "")
}

func open(dbPath, dataDir string) (*DB, error) {
	// foreign_keys 必須走 DSN：database/sql 有連線池，逐次 PRAGMA
	// 只會作用在池中某一條連線上。
	dsn := dbPath + "?_pragma=foreign_keys(1)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("開啟資料庫: %w", err)
	}
	if dbPath == ":memory:" {
		// 記憶體資料庫是 per-connection 的，多條連線會看到不同的空庫。
		sqlDB.SetMaxOpenConns(1)
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("連線資料庫: %w", err)
	}
	return &DB{sql: sqlDB, dataDir: dataDir}, nil
}

// SchemaVersion 回傳 PRAGMA user_version。
func (d *DB) SchemaVersion() (int, error) {
	var v int
	if err := d.sql.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("讀取 user_version: %w", err)
	}
	return v, nil
}

// migrate 依檔名順序執行尚未套用的遷移檔。
func (d *DB) migrate() error {
	names, err := fs.Glob(schemaFS, "schema/*.sql")
	if err != nil {
		return fmt.Errorf("列出遷移檔: %w", err)
	}
	sort.Strings(names)

	cur, err := d.SchemaVersion()
	if err != nil {
		return err
	}
	for i, name := range names {
		version := i + 1
		if version <= cur {
			continue
		}
		body, err := schemaFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("讀取 %s: %w", name, err)
		}
		tx, err := d.sql.Begin()
		if err != nil {
			return fmt.Errorf("開始交易: %w", err)
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("套用 %s: %w", name, err)
		}
		// PRAGMA user_version 不接受參數綁定。
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("寫入 user_version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交 %s: %w", name, err)
		}
	}
	return nil
}

// Backup 立即產生一份備份快照並輪替。匯入等破壞性操作前應呼叫它。
func (d *DB) Backup() error { return d.backup() }

// backup 以 VACUUM INTO 產生一份原子快照，並輪替至 backupKeep 份。
// 資料庫檔不存在或為空（首次啟動）時不做任何事。
func (d *DB) backup() error {
	if d.dataDir == "" {
		return nil
	}
	st, err := os.Stat(filepath.Join(d.dataDir, dbFileName))
	if err != nil || st.Size() == 0 {
		return nil // 首次啟動，沒有東西可備份
	}
	bdir := filepath.Join(d.dataDir, "backups")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		return fmt.Errorf("建立備份目錄: %w", err)
	}
	target := filepath.Join(bdir, time.Now().Format("2006-01-02-150405")+".db")
	if _, err := os.Stat(target); err != nil {
		// 同一秒內重複啟動時沿用既有那份，不覆蓋。
		if _, err := d.sql.Exec("VACUUM INTO ?", target); err != nil {
			return fmt.Errorf("備份至 %s: %w", target, err)
		}
	}
	return d.rotateBackups(bdir)
}

func (d *DB) rotateBackups(bdir string) error {
	entries, err := os.ReadDir(bdir)
	if err != nil {
		return fmt.Errorf("讀取備份目錄: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
			names = append(names, e.Name())
		}
	}
	if len(names) <= backupKeep {
		return nil
	}
	sort.Strings(names) // 檔名是時間戳，字典序即時間序
	for _, name := range names[:len(names)-backupKeep] {
		if err := os.Remove(filepath.Join(bdir, name)); err != nil {
			return fmt.Errorf("刪除舊備份 %s: %w", name, err)
		}
	}
	return nil
}

// NewID 產生 "<prefix>_<16 進位 8 碼>" 形式的識別碼。
func NewID(prefix string) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失敗代表系統熵源異常，繼續執行只會產生撞號風險。
		panic(fmt.Sprintf("產生識別碼: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(b)
}
