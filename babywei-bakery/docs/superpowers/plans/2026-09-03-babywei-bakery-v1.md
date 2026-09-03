# BabyWei Bakery 初版實作計劃

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `templates/index.html` 單檔原型改造成資料持久化的本地應用程式，做到可在瀏覽器完整操作、並以單一 Go binary 執行。

**Architecture:** Go `net/http` 後端 + SQLite（`modernc.org/sqlite`，純 Go 無 CGO），前端 Vue 3 + Vite 經 `vite build` 產出後以 `embed.FS` 編進 binary。領域計算（成本、配方換算、庫存、報表）隔離為純函數，不碰資料庫也不碰 HTTP，可獨立測試。

**Tech Stack:** Go 1.27、`modernc.org/sqlite` v1.58.0、Vue 3.5.42、Vite 8.2.2、`@vitejs/plugin-vue` 6.0.8

**Spec:** [`../specs/2026-09-03-babywei-bakery-design.md`](../specs/2026-09-03-babywei-bakery-design.md)

**範圍:** 本計劃涵蓋 spec 第 13 節的階段 1–3。階段 4（`.app` bundle、`build-release.sh`、zip 交付）另立計劃 —— 本計劃完成時應用程式已可用單一 binary 執行並在瀏覽器操作。

## Global Constraints

- **module 名稱**：`babywei-bakery`（全小寫 kebab-case，理由見 repo 根 `CLAUDE.md`）
- **Go 版本**：`go.mod` 寫 `go 1.27`
- **SQLite DSN 必須帶 `?_pragma=foreign_keys(1)`** —— 預設 `foreign_keys = 0`，所有 `ON DELETE CASCADE` 會靜默失效且可插入孤兒資料。必須用 DSN 參數而非 `db.Exec("PRAGMA ...")`，因為 `database/sql` 有連線池
- **`journal_mode` 保持預設 `delete`，不要改 WAL** —— 單一使用者，WAL 的並發優勢用不到，而 `-wal` / `-shm` 檔案會破壞「複製資料夾即完整備份」
- **服務只綁 `127.0.0.1:8787`**，無認證
- **資料庫路徑**：`BAKERY_DATA_DIR` 環境變數優先；否則由 `os.Executable()` 向上解析四層取得 `BabyWei Bakery/`，再定位其下 `data/`
- **所有金額與重量用 `float64`**，克數與價格皆可為小數
- **日期一律 `YYYY-MM-DD` 字串**，取當下日期用 `time.Now()`，**不可寫死**
- **成本算法為全期加權平均** `Σ(price) / Σ(weight_g)`，隔離在 `internal/domain/cost.go` 單一函數
- **所有 `.md` 須通過 `./scripts/lint.sh md`**（在 playground repo 根執行）
- **commit 走 Conventional Commits**，中文主旨、路徑用反引號，規範見 repo 根 `CLAUDE.md`

---

## File Structure

| 檔案 | 責任 |
| --- | --- |
| `go.mod` | module 宣告與依賴 |
| `main.go` | 進入點：解析資料目錄、開 DB、起服務、開瀏覽器 |
| `internal/store/db.go` | 連線、migration、啟動備份與輪替 |
| `internal/store/model.go` | 資料結構定義（跨 store / domain / api 共用） |
| `internal/store/purchases.go` | `purchases` CRUD |
| `internal/store/recipes.go` | `doughs` / `fillings` 及其 ingredients CRUD |
| `internal/store/products.go` | `products` CRUD |
| `internal/store/sales.go` | `sales` CRUD |
| `internal/store/production.go` | `production_logs` + `production_consumption`（交易） |
| `internal/store/transfer.go` | 全量匯出 / 匯入 |
| `internal/domain/recipe.go` | 配方換算（Baker % 與絕對克數）—— 最底層，其他 domain 依賴它 |
| `internal/domain/cost.go` | 加權平均單位成本、商品單顆成本 |
| `internal/domain/inventory.go` | 庫存推算 |
| `internal/domain/report.go` | 利潤統計與區間 |
| `internal/api/router.go` | 路由與靜態檔 |
| `internal/api/handlers_crud.go` | purchases / doughs / fillings / products 端點 |
| `internal/api/handlers_ops.go` | sales / production / inventory / reports 端點 |
| `internal/api/handlers_transfer.go` | export / import 端點 |
| `internal/api/respond.go` | JSON 回應與錯誤格式 |
| `internal/assets/embed.go` | `//go:embed` 前端產出 |
| `schema/001_init.sql` | 初始 schema |
| `web/` | Vite + Vue 3 前端原始碼 |

`internal/domain/` 的四個檔案是純函數，簽章只吃 `internal/store` 的資料結構、回傳數字或結構，**不 import `database/sql` 也不 import `net/http`**。這個邊界是本專案測試策略的基礎。

---

## 對 spec 的一處刻意偏離

Spec 第 5.1 節把 schema 放在專案根的 `schema/`。實作改放 **`internal/store/schema/`**，因為 Go 的 `//go:embed` **不能跨越 `..`** —— 放在根目錄的話 `internal/store/db.go` 無法嵌入它，只能改由 `main.go` 嵌入再傳進來，或在測試中用相對路徑讀檔（脆弱）。遷移檔本來就屬於 store 套件，與使用它的程式碼放在一起也是更好的 co-location。

---

### Task 1: 專案骨架、schema、migration 與啟動備份

**Files:**

- Create: `babywei-bakery/go.mod`
- Create: `babywei-bakery/internal/store/schema/001_init.sql`
- Create: `babywei-bakery/internal/store/model.go`
- Create: `babywei-bakery/internal/store/db.go`
- Test: `babywei-bakery/internal/store/db_test.go`

**Interfaces:**

- Consumes: 無（第一個 task）
- Produces:
  - `store.Open(dataDir string) (*store.DB, error)` —— 開啟 `<dataDir>/babywei.db`，跑 migration，做啟動備份
  - `store.OpenMemory() (*store.DB, error)` —— `:memory:` 版本，供測試用，跑 migration 但不備份
  - `(*store.DB).Close() error`
  - `(*store.DB).SQL() *sql.DB` —— 供同套件其他檔案使用
  - `(*store.DB).SchemaVersion() (int, error)`
  - 型別：`store.Purchase`、`store.DoughItem`、`store.Dough`、`store.FillingItem`、`store.Filling`、`store.Product`、`store.Sale`、`store.ProductionLog`、`store.Consumption`

- [ ] **Step 1: 建立 module 與依賴**

```bash
cd ~/Git/robywei/playground/babywei-bakery
go mod init babywei-bakery
go get modernc.org/sqlite@v1.58.0
```

`go.mod` 應含 `go 1.27` 與 `require modernc.org/sqlite v1.58.0`。

- [ ] **Step 2: 寫 schema**

建立 `internal/store/schema/001_init.sql`：

```sql
CREATE TABLE purchases (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  brand         TEXT NOT NULL DEFAULT '',
  purchase_date TEXT NOT NULL,
  channel       TEXT NOT NULL DEFAULT '',
  price         REAL NOT NULL CHECK (price >= 0),
  weight_g      REAL NOT NULL CHECK (weight_g > 0)
);
CREATE INDEX idx_purchases_name ON purchases(name);
CREATE INDEX idx_purchases_date ON purchases(purchase_date);

CREATE TABLE doughs (
  id   TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE dough_ingredients (
  dough_id TEXT NOT NULL REFERENCES doughs(id) ON DELETE CASCADE,
  name     TEXT NOT NULL,
  pct      REAL NOT NULL CHECK (pct > 0),
  sort     INTEGER NOT NULL,
  PRIMARY KEY (dough_id, sort)
);

CREATE TABLE fillings (
  id   TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE filling_ingredients (
  filling_id TEXT NOT NULL REFERENCES fillings(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  weight_g   REAL NOT NULL CHECK (weight_g > 0),
  sort       INTEGER NOT NULL,
  PRIMARY KEY (filling_id, sort)
);

CREATE TABLE products (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL UNIQUE,
  price           REAL NOT NULL CHECK (price >= 0),
  dough_id        TEXT NOT NULL REFERENCES doughs(id) ON DELETE RESTRICT,
  dough_weight_g  REAL NOT NULL CHECK (dough_weight_g > 0),
  fill1_id        TEXT REFERENCES fillings(id) ON DELETE RESTRICT,
  fill1_weight_g  REAL NOT NULL DEFAULT 0 CHECK (fill1_weight_g >= 0),
  fill2_id        TEXT REFERENCES fillings(id) ON DELETE RESTRICT,
  fill2_weight_g  REAL NOT NULL DEFAULT 0 CHECK (fill2_weight_g >= 0)
);

CREATE TABLE sales (
  id           TEXT PRIMARY KEY,
  sale_date    TEXT NOT NULL,
  product_id   TEXT REFERENCES products(id) ON DELETE SET NULL,
  product_name TEXT NOT NULL,
  qty          INTEGER NOT NULL CHECK (qty > 0),
  unit_cost    REAL NOT NULL,
  unit_price   REAL NOT NULL
);
CREATE INDEX idx_sales_date ON sales(sale_date);

CREATE TABLE production_logs (
  id           TEXT PRIMARY KEY,
  logged_date  TEXT NOT NULL,
  product_id   TEXT REFERENCES products(id) ON DELETE SET NULL,
  product_name TEXT NOT NULL,
  qty          INTEGER NOT NULL CHECK (qty > 0)
);
CREATE INDEX idx_production_date ON production_logs(logged_date);

CREATE TABLE production_consumption (
  log_id          TEXT NOT NULL REFERENCES production_logs(id) ON DELETE CASCADE,
  ingredient_name TEXT NOT NULL,
  consumed_g      REAL NOT NULL CHECK (consumed_g >= 0),
  PRIMARY KEY (log_id, ingredient_name)
);
```

- [ ] **Step 3: 寫資料結構**

建立 `internal/store/model.go`：

```go
package store

// Purchase 是一筆進貨紀錄。
type Purchase struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Brand        string  `json:"brand"`
	PurchaseDate string  `json:"purchaseDate"`
	Channel      string  `json:"channel"`
	Price        float64 `json:"price"`
	WeightG      float64 `json:"weightG"`
}

// DoughItem 是產品配方中的一項材料，以 Baker's % 表示。
type DoughItem struct {
	Name string  `json:"name"`
	Pct  float64 `json:"pct"`
}

// Dough 是一份產品配方。
type Dough struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Ingredients []DoughItem `json:"ingredients"`
}

// FillingItem 是配料中的一項材料，以絕對克數表示。
type FillingItem struct {
	Name    string  `json:"name"`
	WeightG float64 `json:"weightG"`
}

// Filling 是一份配料配方。
type Filling struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Ingredients []FillingItem `json:"ingredients"`
}

// Product 是一項可販售商品。Fill1ID / Fill2ID 為空字串表示未使用該配料。
type Product struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	DoughID      string  `json:"doughId"`
	DoughWeightG float64 `json:"doughWeightG"`
	Fill1ID      string  `json:"fill1Id"`
	Fill1WeightG float64 `json:"fill1WeightG"`
	Fill2ID      string  `json:"fill2Id"`
	Fill2WeightG float64 `json:"fill2WeightG"`
}

// Sale 是一筆出貨紀錄。UnitCost 與 UnitPrice 是寫入當下的快照，
// 之後改配方或改售價都不影響已成立的紀錄。
type Sale struct {
	ID          string  `json:"id"`
	SaleDate    string  `json:"saleDate"`
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	Qty         int     `json:"qty"`
	UnitCost    float64 `json:"unitCost"`
	UnitPrice   float64 `json:"unitPrice"`
}

// Consumption 是一批生產對單一材料的消耗量，於確認生產時寫死。
type Consumption struct {
	IngredientName string  `json:"ingredientName"`
	ConsumedG      float64 `json:"consumedG"`
}

// ProductionLog 是一批生產紀錄。Consumption 是當下算出的原料消耗快照，
// 庫存一律以它為依據，不從當前配方回推。
type ProductionLog struct {
	ID          string        `json:"id"`
	LoggedDate  string        `json:"loggedDate"`
	ProductID   string        `json:"productId"`
	ProductName string        `json:"productName"`
	Qty         int           `json:"qty"`
	Consumption []Consumption `json:"consumption"`
}
```

- [ ] **Step 4: 寫失敗的測試**

建立 `internal/store/db_test.go`：

```go
package store

import (
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

	// 孤兒插入必須被拒絕
	_, err = db.SQL().Exec(
		"INSERT INTO dough_ingredients(dough_id, name, pct, sort) VALUES('nope','麵粉',100,0)")
	if err == nil {
		t.Error("插入孤兒 dough_ingredients 應該失敗，實際成功")
	}

	// CASCADE 必須生效
	if _, err := db.SQL().Exec("INSERT INTO doughs(id,name) VALUES('d1','基礎')"); err != nil {
		t.Fatalf("insert dough: %v", err)
	}
	if _, err := db.SQL().Exec(
		"INSERT INTO dough_ingredients(dough_id,name,pct,sort) VALUES('d1','麵粉',100,0)"); err != nil {
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

func TestOpenCreatesBackupOnStartup(t *testing.T) {
	dir := t.TempDir()

	db1, err := Open(dir)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := db1.SQL().Exec("INSERT INTO doughs(id,name) VALUES('d1','基礎')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	db1.Close()

	// 第二次開啟應產生一份備份
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
		t.Fatal("啟動未產生備份")
	}
}

func TestBackupRotationKeepsLimit(t *testing.T) {
	dir := t.TempDir()
	bdir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 先塞 35 個假備份
	for i := 0; i < 35; i++ {
		name := filepath.Join(bdir, "2026-01-01-0000"+string(rune('a'+i))+".db")
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
	if len(entries) > backupKeep {
		t.Errorf("備份數 = %d, 應輪替至 <= %d", len(entries), backupKeep)
	}
}
```

- [ ] **Step 5: 執行測試確認失敗**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/store/
```

預期：編譯失敗，`undefined: OpenMemory`、`undefined: Open`、`undefined: backupKeep`。

- [ ] **Step 6: 實作 `db.go`**

建立 `internal/store/db.go`：

```go
// Package store 負責 SQLite 的連線、schema 遷移、備份，以及所有 SQL。
package store

import (
	"database/sql"
	"embed"
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
	dbPath := filepath.Join(dataDir, "babywei.db")

	d, err := open(dbPath, dataDir)
	if err != nil {
		return nil, err
	}
	// 備份要在 migration 之前，這樣遷移失敗時手上還有遷移前的快照
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

func open(dbPath, dataDir string) (*DB, error) {
	// foreign_keys 必須走 DSN：database/sql 有連線池，逐次 PRAGMA
	// 只會作用在池中某一條連線上。
	dsn := dbPath + "?_pragma=foreign_keys(1)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("開啟資料庫: %w", err)
	}
	if dbPath == ":memory:" {
		// 記憶體資料庫是 per-connection 的，多條連線會看到不同的空庫
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
		// user_version 不接受參數綁定
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

// backup 以 VACUUM INTO 產生一份原子快照，並輪替至 backupKeep 份。
// 資料庫檔不存在（首次啟動）時不做任何事。
func (d *DB) backup() error {
	if d.dataDir == "" {
		return nil
	}
	dbPath := filepath.Join(d.dataDir, "babywei.db")
	if st, err := os.Stat(dbPath); err != nil || st.Size() == 0 {
		return nil // 首次啟動，沒有東西可備份
	}
	bdir := filepath.Join(d.dataDir, "backups")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		return fmt.Errorf("建立備份目錄: %w", err)
	}
	target := filepath.Join(bdir, time.Now().Format("2006-01-02-150405")+".db")
	if _, err := os.Stat(target); err == nil {
		return d.rotateBackups(bdir) // 同秒內重複啟動，沿用既有那份
	}
	if _, err := d.sql.Exec("VACUUM INTO ?", target); err != nil {
		return fmt.Errorf("備份至 %s: %w", target, err)
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
```

- [ ] **Step 7: 執行測試確認通過**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/store/ -v
```

預期：5 個測試全部 PASS。

- [ ] **Step 8: Commit**

```bash
cd ~/Git/robywei/playground
git add babywei-bakery/go.mod babywei-bakery/go.sum babywei-bakery/internal/store/
git commit -F - <<'MSG'
feat(babywei-bakery): 🗄️ 資料層 —— schema、migration 與啟動備份

- `internal/store/schema/001_init.sql` - 9 張表
- `internal/store/db.go` - 連線、`PRAGMA user_version` 遷移、`VACUUM INTO` 備份輪替
- `internal/store/model.go` - 跨層共用的資料結構

DSN 帶 `?_pragma=foreign_keys(1)`：SQLite 預設關閉外鍵，所有 `ON DELETE CASCADE`
會靜默失效且可插入孤兒資料。走 DSN 而非 `PRAGMA` 陳述式，因為 `database/sql`
有連線池，逐次 PRAGMA 只作用在其中一條連線上。

備份排在 migration 之前，遷移失敗時手上還有遷移前的快照。
MSG
```

---

### Task 2: `domain/recipe.go` —— 配方換算

領域層最底層，`cost.go` 與 `production` 都依賴它。

**Files:**

- Create: `babywei-bakery/internal/domain/recipe.go`
- Test: `babywei-bakery/internal/domain/recipe_test.go`

**Interfaces:**

- Consumes: `store.DoughItem`、`store.FillingItem`
- Produces:
  - `domain.Component{Name string; Ratio float64}`
  - `domain.FromDough([]store.DoughItem) []Component`
  - `domain.FromFilling([]store.FillingItem) []Component`
  - `domain.Scale(items []Component, totalG float64) map[string]float64`

- [ ] **Step 1: 寫失敗的測試**

```go
package domain

import (
	"math"
	"testing"

	"babywei-bakery/internal/store"
)

func closeTo(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestScaleBakerPercent(t *testing.T) {
	// 麵粉 100% + 水 65% = 165 份，總重 330g → 麵粉 200g、水 130g
	items := FromDough([]store.DoughItem{
		{Name: "高筋麵粉", Pct: 100},
		{Name: "牛奶", Pct: 65},
	})
	got := Scale(items, 330)
	closeTo(t, got["高筋麵粉"], 200, "麵粉")
	closeTo(t, got["牛奶"], 130, "牛奶")
}

func TestScaleNormalizesWhenBaseIsNot100(t *testing.T) {
	// 公式正規化，主粉基準不是 100% 也能得到正確比例
	items := FromDough([]store.DoughItem{
		{Name: "A", Pct: 50},
		{Name: "B", Pct: 50},
	})
	got := Scale(items, 100)
	closeTo(t, got["A"], 50, "A")
	closeTo(t, got["B"], 50, "B")
}

func TestScaleAbsoluteGrams(t *testing.T) {
	// 配料以絕對克數管理比例：100g + 50g = 150 份，需求 300g → 200g / 100g
	items := FromFilling([]store.FillingItem{
		{Name: "南瓜泥", WeightG: 100},
		{Name: "糖", WeightG: 50},
	})
	got := Scale(items, 300)
	closeTo(t, got["南瓜泥"], 200, "南瓜泥")
	closeTo(t, got["糖"], 100, "糖")
}

func TestScaleSumsPreservesTotal(t *testing.T) {
	items := FromDough([]store.DoughItem{
		{Name: "A", Pct: 100}, {Name: "B", Pct: 65},
		{Name: "C", Pct: 8}, {Name: "D", Pct: 1.5},
	})
	got := Scale(items, 1234.5)
	var sum float64
	for _, v := range got {
		sum += v
	}
	closeTo(t, sum, 1234.5, "各材料用量總和")
}

func TestScaleDuplicateNamesAreMerged(t *testing.T) {
	// 同一材料在配方中出現兩次，用量要相加而非互相覆蓋
	items := FromDough([]store.DoughItem{
		{Name: "糖", Pct: 5},
		{Name: "糖", Pct: 5},
	})
	got := Scale(items, 100)
	if len(got) != 1 {
		t.Fatalf("結果應合併為 1 項，實際 %d 項", len(got))
	}
	closeTo(t, got["糖"], 100, "糖")
}

func TestScaleEdgeCases(t *testing.T) {
	if got := Scale(nil, 100); len(got) != 0 {
		t.Errorf("空配方應回傳空 map，實際 %v", got)
	}
	items := FromDough([]store.DoughItem{{Name: "A", Pct: 100}})
	if got := Scale(items, 0); len(got) != 0 {
		t.Errorf("需求總重 0 應回傳空 map，實際 %v", got)
	}
}
```

- [ ] **Step 2: 執行確認失敗**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/domain/
```

預期：`undefined: FromDough`、`undefined: Scale`。

- [ ] **Step 3: 實作**

```go
// Package domain 是純計算層：成本、配方換算、庫存、報表。
// 這裡的函數不碰資料庫也不碰 HTTP，可獨立測試。
package domain

import "babywei-bakery/internal/store"

// Component 是配方中的一項材料。Ratio 對產品配方是 Baker's %、
// 對配料是絕對克數 —— 換算會正規化，所以單位不影響結果。
type Component struct {
	Name  string
	Ratio float64
}

// FromDough 把產品配方材料轉為換算用的 Component。
func FromDough(items []store.DoughItem) []Component {
	out := make([]Component, 0, len(items))
	for _, it := range items {
		out = append(out, Component{Name: it.Name, Ratio: it.Pct})
	}
	return out
}

// FromFilling 把配料材料轉為換算用的 Component。
func FromFilling(items []store.FillingItem) []Component {
	out := make([]Component, 0, len(items))
	for _, it := range items {
		out = append(out, Component{Name: it.Name, Ratio: it.WeightG})
	}
	return out
}

// Scale 把配方按 totalG 的需求總重換算成各材料的實際用量（克）。
//
//	用量(材料) = totalG × Ratio(材料) / Σ Ratio
//
// 同名材料的用量會相加。配方為空、總重為 0 或比例總和為 0 時回傳空 map。
func Scale(items []Component, totalG float64) map[string]float64 {
	out := make(map[string]float64)
	if len(items) == 0 || totalG <= 0 {
		return out
	}
	var sum float64
	for _, it := range items {
		sum += it.Ratio
	}
	if sum <= 0 {
		return out
	}
	for _, it := range items {
		out[it.Name] += totalG * it.Ratio / sum
	}
	return out
}
```

- [ ] **Step 4: 執行確認通過**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/domain/ -v
```

預期：6 個測試全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add babywei-bakery/internal/domain/
git commit -m "feat(babywei-bakery): 🧮 \`domain/recipe.go\` 配方換算（Baker % 與絕對克數共用正規化公式）"
```

---

### Task 3: `domain/cost.go` —— 加權平均與商品單顆成本

**Files:**

- Create: `babywei-bakery/internal/domain/cost.go`
- Test: `babywei-bakery/internal/domain/cost_test.go`

**Interfaces:**

- Consumes: `store.Purchase`、`store.Product`、`store.Dough`、`store.Filling`、`domain.Scale`
- Produces:
  - `domain.CostPerGram(purchases []store.Purchase) map[string]float64`
  - `domain.Recipes{Doughs map[string]store.Dough; Fillings map[string]store.Filling}`
  - `domain.ProductUnitCost(p store.Product, r Recipes, costPerG map[string]float64) float64`
  - `domain.ProductConsumption(p store.Product, r Recipes, qty int) map[string]float64`

- [ ] **Step 1: 寫失敗的測試**

```go
package domain

import (
	"testing"

	"babywei-bakery/internal/store"
)

func TestCostPerGramWeightedAverage(t *testing.T) {
	// 麵粉兩批：120元/1000g 與 150元/500g
	// 全期加權平均 = (120+150)/(1000+500) = 0.18 元/g
	got := CostPerGram([]store.Purchase{
		{Name: "高筋麵粉", Price: 120, WeightG: 1000},
		{Name: "高筋麵粉", Price: 150, WeightG: 500},
	})
	closeTo(t, got["高筋麵粉"], 0.18, "麵粉單位成本")
}

func TestCostPerGramIsNotLatestPurchase(t *testing.T) {
	// 這是刻意偏離原型的行為：原型取最近一筆（0.3），改為加權平均
	got := CostPerGram([]store.Purchase{
		{Name: "糖", Price: 100, WeightG: 1000, PurchaseDate: "2026-01-01"},
		{Name: "糖", Price: 300, WeightG: 1000, PurchaseDate: "2026-09-01"},
	})
	closeTo(t, got["糖"], 0.2, "糖單位成本（加權平均而非最近一筆）")
}

func TestCostPerGramUnknownIngredientIsZero(t *testing.T) {
	got := CostPerGram(nil)
	if v, ok := got["不存在"]; ok && v != 0 {
		t.Errorf("未進貨材料應為 0，實際 %v", v)
	}
}

func testRecipes() Recipes {
	return Recipes{
		Doughs: map[string]store.Dough{
			"d1": {ID: "d1", Name: "基礎吐司", Ingredients: []store.DoughItem{
				{Name: "麵粉", Pct: 100},
				{Name: "水", Pct: 100},
			}},
		},
		Fillings: map[string]store.Filling{
			"f1": {ID: "f1", Name: "南瓜泥", Ingredients: []store.FillingItem{
				{Name: "南瓜泥", WeightG: 100},
			}},
		},
	}
}

func TestProductUnitCost(t *testing.T) {
	costPerG := map[string]float64{"麵粉": 0.2, "水": 0, "南瓜泥": 0.5}
	p := store.Product{
		DoughID: "d1", DoughWeightG: 200,
		Fill1ID: "f1", Fill1WeightG: 80,
	}
	// 麵團 200g 拆成 麵粉 100g + 水 100g → 100×0.2 = 20
	// 配料 80g 全是南瓜泥 → 80×0.5 = 40
	closeTo(t, ProductUnitCost(p, testRecipes(), costPerG), 60, "單顆成本")
}

func TestProductUnitCostMissingRecipeIsZero(t *testing.T) {
	p := store.Product{DoughID: "不存在", DoughWeightG: 200}
	closeTo(t, ProductUnitCost(p, testRecipes(), nil), 0, "配方不存在時的成本")
}

func TestProductConsumptionScalesWithQty(t *testing.T) {
	p := store.Product{
		DoughID: "d1", DoughWeightG: 200,
		Fill1ID: "f1", Fill1WeightG: 80,
	}
	got := ProductConsumption(p, testRecipes(), 10)
	closeTo(t, got["麵粉"], 1000, "麵粉消耗")
	closeTo(t, got["水"], 1000, "水消耗")
	closeTo(t, got["南瓜泥"], 800, "南瓜泥消耗")
}

func TestProductConsumptionMergesSharedIngredient(t *testing.T) {
	// 麵團與配料用到同一材料時，消耗量必須相加
	r := Recipes{
		Doughs: map[string]store.Dough{
			"d1": {ID: "d1", Ingredients: []store.DoughItem{{Name: "糖", Pct: 100}}},
		},
		Fillings: map[string]store.Filling{
			"f1": {ID: "f1", Ingredients: []store.FillingItem{{Name: "糖", WeightG: 100}}},
		},
	}
	p := store.Product{DoughID: "d1", DoughWeightG: 30, Fill1ID: "f1", Fill1WeightG: 20}
	got := ProductConsumption(p, r, 1)
	closeTo(t, got["糖"], 50, "糖消耗（麵團 30 + 配料 20）")
}
```

- [ ] **Step 2: 執行確認失敗**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/domain/ -run 'Cost|Product'
```

- [ ] **Step 3: 實作**

```go
package domain

import "babywei-bakery/internal/store"

// Recipes 是配方查表，key 為 ID。
type Recipes struct {
	Doughs   map[string]store.Dough
	Fillings map[string]store.Filling
}

// CostPerGram 以全期加權平均算出各材料的每克成本：
//
//	cost_per_g(材料) = Σ price / Σ weight_g
//
// 這是刻意的取捨。它不是移動加權平均 —— 舊進貨會持續影響均價，
// 即使該批早已用完。移動加權平均需追蹤每批剩餘量，較準但複雜度顯著提高。
// 要改算法只需改本函數，呼叫端不受影響。
func CostPerGram(purchases []store.Purchase) map[string]float64 {
	type acc struct{ price, weight float64 }
	sums := make(map[string]acc)
	for _, p := range purchases {
		a := sums[p.Name]
		a.price += p.Price
		a.weight += p.WeightG
		sums[p.Name] = a
	}
	out := make(map[string]float64, len(sums))
	for name, a := range sums {
		if a.weight > 0 {
			out[name] = a.price / a.weight
		}
	}
	return out
}

// components 收集一項商品所有配方段落的 Component 與對應需求總重。
func components(p store.Product, r Recipes, qty int) []map[string]float64 {
	q := float64(qty)
	var parts []map[string]float64

	if d, ok := r.Doughs[p.DoughID]; ok {
		parts = append(parts, Scale(FromDough(d.Ingredients), p.DoughWeightG*q))
	}
	for _, f := range []struct {
		id      string
		weightG float64
	}{
		{p.Fill1ID, p.Fill1WeightG},
		{p.Fill2ID, p.Fill2WeightG},
	} {
		if f.id == "" {
			continue
		}
		if fl, ok := r.Fillings[f.id]; ok {
			parts = append(parts, Scale(FromFilling(fl.Ingredients), f.weightG*q))
		}
	}
	return parts
}

// ProductUnitCost 算出一顆商品的原料成本。配方不存在的段落貢獻 0。
func ProductUnitCost(p store.Product, r Recipes, costPerG map[string]float64) float64 {
	var total float64
	for _, part := range components(p, r, 1) {
		for name, grams := range part {
			total += grams * costPerG[name]
		}
	}
	return total
}

// ProductConsumption 算出生產 qty 顆商品所消耗的各材料克數。
// 麵團與配料用到同一材料時，用量相加。
func ProductConsumption(p store.Product, r Recipes, qty int) map[string]float64 {
	out := make(map[string]float64)
	for _, part := range components(p, r, qty) {
		for name, grams := range part {
			out[name] += grams
		}
	}
	return out
}
```

- [ ] **Step 4: 執行確認通過**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/domain/ -v
```

- [ ] **Step 5: Commit**

```bash
git add babywei-bakery/internal/domain/
git commit -m "feat(babywei-bakery): 💰 \`domain/cost.go\` 全期加權平均成本與商品單顆成本"
```

---

### Task 4: `domain/inventory.go` —— 庫存推算

**Files:**

- Create: `babywei-bakery/internal/domain/inventory.go`
- Test: `babywei-bakery/internal/domain/inventory_test.go`

**Interfaces:**

- Consumes: `store.Purchase`、`store.ProductionLog`
- Produces:
  - `domain.InventoryRow{Name, Brand string; TotalBoughtG, TotalUsedG, RemainingG float64; Status string}`
  - `domain.Inventory(purchases []store.Purchase, logs []store.ProductionLog) []InventoryRow`
  - `Status` 取值：`"out"`（剩餘 <= 0）、`"low"`（< 200g）、`"ok"`

- [ ] **Step 1: 寫失敗的測試**

```go
package domain

import (
	"testing"

	"babywei-bakery/internal/store"
)

func rowByName(rows []InventoryRow, name string) (InventoryRow, bool) {
	for _, r := range rows {
		if r.Name == name {
			return r, true
		}
	}
	return InventoryRow{}, false
}

func TestInventorySubtractsSnapshotConsumption(t *testing.T) {
	purchases := []store.Purchase{
		{Name: "麵粉", Brand: "水手牌", Price: 120, WeightG: 1000},
		{Name: "麵粉", Brand: "水手牌", Price: 120, WeightG: 1000},
	}
	logs := []store.ProductionLog{
		{ID: "l1", Consumption: []store.Consumption{{IngredientName: "麵粉", ConsumedG: 750}}},
	}
	rows := Inventory(purchases, logs)
	r, ok := rowByName(rows, "麵粉")
	if !ok {
		t.Fatal("麵粉不在庫存表中")
	}
	closeTo(t, r.TotalBoughtG, 2000, "總進貨")
	closeTo(t, r.TotalUsedG, 750, "已消耗")
	closeTo(t, r.RemainingG, 1250, "剩餘")
	if r.Status != "ok" {
		t.Errorf("狀態 = %q, want ok", r.Status)
	}
}

func TestInventoryUsesSnapshotNotCurrentRecipe(t *testing.T) {
	// 這是修正原型的關鍵行為：消耗量來自 production_consumption 快照，
	// 不從當前配方回推。函數簽章根本收不到配方，所以改配方或刪商品
	// 都不可能影響歷史庫存。
	logs := []store.ProductionLog{
		{ID: "l1", ProductID: "", ProductName: "已刪除的商品",
			Consumption: []store.Consumption{{IngredientName: "麵粉", ConsumedG: 500}}},
	}
	rows := Inventory([]store.Purchase{{Name: "麵粉", WeightG: 1000, Price: 100}}, logs)
	r, _ := rowByName(rows, "麵粉")
	closeTo(t, r.TotalUsedG, 500, "商品已刪除，消耗仍須計入")
}

func TestInventoryIncludesNeverPurchasedIngredient(t *testing.T) {
	// 修正原型第二個 bug：配方用到但從未進貨的材料，
	// 原型會靜默丟棄其消耗且不顯示該材料。這裡必須列出且剩餘為負。
	logs := []store.ProductionLog{
		{ID: "l1", Consumption: []store.Consumption{{IngredientName: "漏登的酵母", ConsumedG: 30}}},
	}
	rows := Inventory(nil, logs)
	r, ok := rowByName(rows, "漏登的酵母")
	if !ok {
		t.Fatal("從未進貨但已消耗的材料必須出現在庫存表")
	}
	closeTo(t, r.TotalBoughtG, 0, "總進貨")
	closeTo(t, r.RemainingG, -30, "剩餘應為負")
	if r.Status != "out" {
		t.Errorf("狀態 = %q, want out", r.Status)
	}
}

func TestInventoryStatusThresholds(t *testing.T) {
	cases := []struct {
		boughtG, usedG float64
		want           string
	}{
		{1000, 0, "ok"},
		{1000, 801, "low"},   // 剩 199
		{1000, 800, "ok"},    // 剩 200，門檻是 < 200
		{1000, 1000, "out"},  // 剩 0
		{1000, 1200, "out"},  // 剩 -200
	}
	for _, c := range cases {
		rows := Inventory(
			[]store.Purchase{{Name: "X", WeightG: c.boughtG, Price: 1}},
			[]store.ProductionLog{{ID: "l", Consumption: []store.Consumption{
				{IngredientName: "X", ConsumedG: c.usedG}}}},
		)
		r, _ := rowByName(rows, "X")
		if r.Status != c.want {
			t.Errorf("進貨 %v 消耗 %v → 狀態 %q, want %q", c.boughtG, c.usedG, r.Status, c.want)
		}
	}
}

func TestInventoryIsSortedByName(t *testing.T) {
	rows := Inventory([]store.Purchase{
		{Name: "糖", WeightG: 100, Price: 1},
		{Name: "麵粉", WeightG: 100, Price: 1},
		{Name: "鹽", WeightG: 100, Price: 1},
	}, nil)
	if len(rows) != 3 {
		t.Fatalf("列數 = %d, want 3", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Name > rows[i].Name {
			t.Errorf("未依名稱排序: %q 在 %q 之前", rows[i-1].Name, rows[i].Name)
		}
	}
}

func TestInventoryBrandTakesLatestNonEmpty(t *testing.T) {
	rows := Inventory([]store.Purchase{
		{Name: "麵粉", Brand: "", WeightG: 100, Price: 1},
		{Name: "麵粉", Brand: "水手牌", WeightG: 100, Price: 1},
	}, nil)
	r, _ := rowByName(rows, "麵粉")
	if r.Brand != "水手牌" {
		t.Errorf("品牌 = %q, want 水手牌", r.Brand)
	}
}
```

- [ ] **Step 2: 執行確認失敗**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/domain/ -run Inventory
```

- [ ] **Step 3: 實作**

```go
package domain

import (
	"sort"

	"babywei-bakery/internal/store"
)

// lowStockThresholdG 是「庫存偏低」的克數門檻（沿用原型）。
const lowStockThresholdG = 200

// InventoryRow 是庫存表的一列。
type InventoryRow struct {
	Name         string  `json:"name"`
	Brand        string  `json:"brand"`
	TotalBoughtG float64 `json:"totalBoughtG"`
	TotalUsedG   float64 `json:"totalUsedG"`
	RemainingG   float64 `json:"remainingG"`
	Status       string  `json:"status"` // out | low | ok
}

// Inventory 以進貨紀錄與生產消耗快照算出庫存。
//
// 消耗量只來自 logs 的 Consumption 快照 —— 本函數收不到配方，因此改配方
// 或刪除商品都不可能影響歷史庫存數字。這是刻意的簽章設計。
//
// 列為「有進貨」與「有消耗」兩者材料名稱的聯集：從未進貨卻有消耗的材料
// 會以總進貨 0、剩餘負值呈現，讓漏登的進貨看得見。
func Inventory(purchases []store.Purchase, logs []store.ProductionLog) []InventoryRow {
	bought := make(map[string]float64)
	brand := make(map[string]string)
	for _, p := range purchases {
		bought[p.Name] += p.WeightG
		if p.Brand != "" {
			brand[p.Name] = p.Brand
		}
	}
	used := make(map[string]float64)
	for _, l := range logs {
		for _, c := range l.Consumption {
			used[c.IngredientName] += c.ConsumedG
		}
	}

	names := make(map[string]struct{}, len(bought)+len(used))
	for n := range bought {
		names[n] = struct{}{}
	}
	for n := range used {
		names[n] = struct{}{}
	}

	rows := make([]InventoryRow, 0, len(names))
	for n := range names {
		remaining := bought[n] - used[n]
		rows = append(rows, InventoryRow{
			Name:         n,
			Brand:        brand[n],
			TotalBoughtG: bought[n],
			TotalUsedG:   used[n],
			RemainingG:   remaining,
			Status:       stockStatus(remaining),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

func stockStatus(remainingG float64) string {
	switch {
	case remainingG <= 0:
		return "out"
	case remainingG < lowStockThresholdG:
		return "low"
	default:
		return "ok"
	}
}
```

- [ ] **Step 4: 執行確認通過**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/domain/ -v -run Inventory
```

- [ ] **Step 5: Commit**

```bash
git add babywei-bakery/internal/domain/
git commit -m "feat(babywei-bakery): 📦 \`domain/inventory.go\` 庫存以消耗快照推算，並列出漏登進貨的材料"
```

---

### Task 5: `domain/report.go` —— 利潤統計與區間

**Files:**

- Create: `babywei-bakery/internal/domain/report.go`
- Test: `babywei-bakery/internal/domain/report_test.go`

**Interfaces:**

- Consumes: `store.Sale`
- Produces:
  - `domain.Totals{RevenueTWD, CostTWD, ProfitTWD float64}`
  - `domain.SalesTotals(sales []store.Sale) Totals`
  - `domain.FilterByDateRange(sales []store.Sale, from, to string) []store.Sale` —— `from` / `to` 為空字串代表該側不設限
  - `domain.Summary(sales []store.Sale, now time.Time) map[string]Totals` —— key 為 `"day"` / `"month"` / `"quarter"` / `"year"`

- [ ] **Step 1: 寫失敗的測試**

```go
package domain

import (
	"testing"
	"time"

	"babywei-bakery/internal/store"
)

func TestSalesTotals(t *testing.T) {
	got := SalesTotals([]store.Sale{
		{Qty: 10, UnitPrice: 180, UnitCost: 60},
		{Qty: 2, UnitPrice: 150, UnitCost: 50},
	})
	closeTo(t, got.RevenueTWD, 10*180+2*150, "營收")
	closeTo(t, got.CostTWD, 10*60+2*50, "成本")
	closeTo(t, got.ProfitTWD, (1800-600)+(300-100), "利潤")
}

func TestSalesTotalsEmpty(t *testing.T) {
	got := SalesTotals(nil)
	closeTo(t, got.RevenueTWD, 0, "營收")
	closeTo(t, got.ProfitTWD, 0, "利潤")
}

func TestFilterByDateRangeInclusive(t *testing.T) {
	sales := []store.Sale{
		{ID: "a", SaleDate: "2026-08-31"},
		{ID: "b", SaleDate: "2026-09-01"},
		{ID: "c", SaleDate: "2026-09-30"},
		{ID: "d", SaleDate: "2026-10-01"},
	}
	got := FilterByDateRange(sales, "2026-09-01", "2026-09-30")
	if len(got) != 2 || got[0].ID != "b" || got[1].ID != "c" {
		t.Errorf("區間應含端點，實際 %+v", got)
	}
}

func TestFilterByDateRangeOpenEnded(t *testing.T) {
	sales := []store.Sale{
		{ID: "a", SaleDate: "2026-01-01"},
		{ID: "b", SaleDate: "2026-12-31"},
	}
	if got := FilterByDateRange(sales, "", ""); len(got) != 2 {
		t.Errorf("兩側不設限應回傳全部，實際 %d 筆", len(got))
	}
	if got := FilterByDateRange(sales, "2026-06-01", ""); len(got) != 1 || got[0].ID != "b" {
		t.Errorf("只設下界，實際 %+v", got)
	}
	if got := FilterByDateRange(sales, "", "2026-06-01"); len(got) != 1 || got[0].ID != "a" {
		t.Errorf("只設上界，實際 %+v", got)
	}
}

func TestSummaryBuckets(t *testing.T) {
	// 基準日 2026-09-03（第三季）
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.Local)
	sales := []store.Sale{
		{SaleDate: "2026-09-03", Qty: 1, UnitPrice: 100, UnitCost: 0}, // 本日
		{SaleDate: "2026-09-01", Qty: 1, UnitPrice: 200, UnitCost: 0}, // 本月
		{SaleDate: "2026-07-15", Qty: 1, UnitPrice: 400, UnitCost: 0}, // 本季
		{SaleDate: "2026-02-01", Qty: 1, UnitPrice: 800, UnitCost: 0}, // 本年度
		{SaleDate: "2025-12-31", Qty: 1, UnitPrice: 9999, UnitCost: 0}, // 去年，全部區間都不算
	}
	got := Summary(sales, now)
	closeTo(t, got["day"].RevenueTWD, 100, "本日營收")
	closeTo(t, got["month"].RevenueTWD, 300, "本月營收")
	closeTo(t, got["quarter"].RevenueTWD, 700, "本季營收")
	closeTo(t, got["year"].RevenueTWD, 1500, "本年度營收")
}

func TestSummaryQuarterBoundaries(t *testing.T) {
	cases := []struct {
		now       string
		inQuarter string
		notIn     string
	}{
		{"2026-01-15", "2026-01-01", "2025-12-31"}, // Q1
		{"2026-05-15", "2026-04-01", "2026-03-31"}, // Q2
		{"2026-08-15", "2026-07-01", "2026-06-30"}, // Q3
		{"2026-11-15", "2026-10-01", "2026-09-30"}, // Q4
	}
	for _, c := range cases {
		now, err := time.ParseInLocation("2006-01-02", c.now, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		got := Summary([]store.Sale{
			{SaleDate: c.inQuarter, Qty: 1, UnitPrice: 10},
			{SaleDate: c.notIn, Qty: 1, UnitPrice: 10},
		}, now)
		closeTo(t, got["quarter"].RevenueTWD, 10, "基準 "+c.now+" 的本季營收")
	}
}
```

- [ ] **Step 2: 執行確認失敗**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/domain/ -run 'Totals|Filter|Summary'
```

- [ ] **Step 3: 實作**

```go
package domain

import (
	"fmt"
	"time"

	"babywei-bakery/internal/store"
)

// Totals 是一組出貨紀錄的財務加總。
type Totals struct {
	RevenueTWD float64 `json:"revenueTwd"`
	CostTWD    float64 `json:"costTwd"`
	ProfitTWD  float64 `json:"profitTwd"`
}

// SalesTotals 加總營收、成本與利潤。單價與單位成本取自出貨當下的快照。
func SalesTotals(sales []store.Sale) Totals {
	var t Totals
	for _, s := range sales {
		q := float64(s.Qty)
		t.RevenueTWD += q * s.UnitPrice
		t.CostTWD += q * s.UnitCost
	}
	t.ProfitTWD = t.RevenueTWD - t.CostTWD
	return t
}

// FilterByDateRange 篩出 SaleDate 落在 [from, to] 的紀錄，含端點。
// from 或 to 為空字串代表該側不設限。日期是 YYYY-MM-DD，字典序即時間序。
func FilterByDateRange(sales []store.Sale, from, to string) []store.Sale {
	out := make([]store.Sale, 0, len(sales))
	for _, s := range sales {
		if from != "" && s.SaleDate < from {
			continue
		}
		if to != "" && s.SaleDate > to {
			continue
		}
		out = append(out, s)
	}
	return out
}

// Summary 以 now 為基準算出本日 / 本月 / 本季 / 本年度的加總。
func Summary(sales []store.Sale, now time.Time) map[string]Totals {
	y, m, d := now.Date()
	quarterStartMonth := (int(m)-1)/3*3 + 1

	ranges := map[string][2]string{
		"day":     {fmt.Sprintf("%04d-%02d-%02d", y, int(m), d), ""},
		"month":   {fmt.Sprintf("%04d-%02d-01", y, int(m)), ""},
		"quarter": {fmt.Sprintf("%04d-%02d-01", y, quarterStartMonth), ""},
		"year":    {fmt.Sprintf("%04d-01-01", y), ""},
	}
	// 上界一律是今天，避免未來日期的紀錄混入
	today := fmt.Sprintf("%04d-%02d-%02d", y, int(m), d)

	out := make(map[string]Totals, len(ranges))
	for key, r := range ranges {
		out[key] = SalesTotals(FilterByDateRange(sales, r[0], today))
	}
	return out
}
```

- [ ] **Step 4: 執行確認通過**

```bash
cd ~/Git/robywei/playground/babywei-bakery && go test ./internal/domain/ -v
```

預期：Task 2–5 的全部測試 PASS。

- [ ] **Step 5: Commit**

```bash
git add babywei-bakery/internal/domain/
git commit -m "feat(babywei-bakery): 📊 \`domain/report.go\` 利潤統計與本日/月/季/年度區間"
```

---

## Task 6 起的密度說明

Task 1–5 是領域邏輯與資料層基礎，正確性關鍵且是本專案的價值所在，因此完整寫出程式碼與測試。Task 6 之後是重複性高的 CRUD 與 UI，改為給出**精確的簽章、必測的案例、以及不明顯的實作決定** —— 樣板碼由實作者依 Task 1–5 已建立的模式產出。每個 task 仍以「測試先寫、跑到紅、實作、跑到綠、commit」的循環執行。

---

### Task 6: store CRUD —— purchases / doughs / fillings

**Files:**

- Create: `babywei-bakery/internal/store/purchases.go`
- Create: `babywei-bakery/internal/store/recipes.go`
- Test: `babywei-bakery/internal/store/purchases_test.go`
- Test: `babywei-bakery/internal/store/recipes_test.go`

**Interfaces:**

- Consumes: `(*store.DB).SQL()`、`store.Purchase`、`store.Dough`、`store.Filling`
- Produces:
  - `(*DB) ListPurchases(from, to, q string) ([]Purchase, error)` —— `from`/`to` 為空不設限；`q` 比對 name / brand / channel（不分大小寫，空字串不過濾）；依 `purchase_date DESC` 排序
  - `(*DB) CreatePurchase(p Purchase) (Purchase, error)` —— `ID` 為空時自動產生
  - `(*DB) UpdatePurchase(p Purchase) error`
  - `(*DB) DeletePurchase(id string) error`
  - `(*DB) ListDoughs() ([]Dough, error)` / `CreateDough` / `UpdateDough` / `DeleteDough(id string) error`
  - `(*DB) ListFillings() ([]Filling, error)` / `CreateFilling` / `UpdateFilling` / `DeleteFilling(id string) error`
  - `store.NewID(prefix string) string` —— 產生 `<prefix>_<8 碼隨機>`，用 `crypto/rand`

**必測案例:**

| 測試 | 判準 |
| --- | --- |
| `TestCreatePurchaseGeneratesID` | 傳入空 `ID` 時回傳的 `ID` 非空且有前綴 `p_` |
| `TestListPurchasesFiltersByDateRange` | 含端點；`from`/`to` 為空時該側不設限 |
| `TestListPurchasesFiltersByKeyword` | `q` 命中 name、brand、channel 任一即保留；大小寫不敏感 |
| `TestListPurchasesSortedByDateDesc` | 最新的在最前 |
| `TestUpdatePurchaseNotFound` | 更新不存在的 id 回傳非 nil error |
| `TestDeletePurchaseNotFound` | 同上 |
| `TestDoughRoundTrip` | Create 後 List 取回的 `Ingredients` 順序與寫入時相同（依 `sort` 欄位） |
| `TestUpdateDoughReplacesIngredients` | 更新配方時舊材料被完全取代，不殘留 |
| `TestDeleteDoughCascadesIngredients` | 刪配方後 `dough_ingredients` 歸零 |
| `TestDeleteDoughInUseIsRejected` | 有商品引用時回傳 error（`ON DELETE RESTRICT`） |
| `TestFillingRoundTrip` / `TestUpdateFillingReplacesIngredients` / `TestDeleteFillingCascades` | 同 dough 三項 |

**不明顯的實作決定:**

- `UpdateDough` / `UpdateFilling` 一律「刪除全部材料再重新插入」，並包在**同一個交易**內。逐筆 diff 更新在材料改名或換順序時容易留下孤兒，且沒有效能上的必要。
- `sort` 欄位由插入時的 slice index 決定，讀取時 `ORDER BY sort`，讓使用者編輯的順序可還原。
- `Update*` 要用 `RowsAffected()` 判斷是否命中，命中 0 筆回傳 error。SQLite 的 `UPDATE` 不會因為 WHERE 沒中而報錯。

- [ ] **Step 1: 寫上表的失敗測試**（以 `OpenMemory()` 建庫）
- [ ] **Step 2: `go test ./internal/store/` 確認紅**
- [ ] **Step 3: 實作 `purchases.go` 與 `recipes.go`**
- [ ] **Step 4: `go test ./internal/store/ -v` 確認綠**
- [ ] **Step 5: Commit** —— `feat(babywei-bakery): 🗄️ store CRUD —— 進貨、產品配方、配料`

---

### Task 7: store CRUD —— products / sales / production

**Files:**

- Create: `babywei-bakery/internal/store/products.go`
- Create: `babywei-bakery/internal/store/sales.go`
- Create: `babywei-bakery/internal/store/production.go`
- Test: 對應三份 `_test.go`

**Interfaces:**

- Consumes: Task 6 的 store 方法、`domain.CostPerGram`、`domain.ProductUnitCost`、`domain.ProductConsumption`
- Produces:
  - `(*DB) ListProducts() ([]Product, error)` / `CreateProduct` / `UpdateProduct` / `DeleteProduct(id string) error`
  - `(*DB) Recipes() (domain.Recipes, error)` —— 一次載入全部配方供 domain 計算用
  - `(*DB) ListSales(from, to string) ([]Sale, error)` —— 依 `sale_date DESC`
  - `(*DB) CreateSale(saleDate, productID string, qty int) (Sale, error)` —— **在函數內算出 `unit_cost` 與 `unit_price` 快照後才寫入**
  - `(*DB) DeleteSale(id string) error`
  - `(*DB) ListProductionLogs() ([]ProductionLog, error)` —— 含 `Consumption`
  - `(*DB) ConfirmProduction(loggedDate, productID string, qty int) (ProductionLog, error)`

**必測案例:**

| 測試 | 判準 |
| --- | --- |
| `TestCreateSaleSnapshotsCostAndPrice` | 建立出貨後修改商品售價與進貨價，重新 `ListSales` 的 `UnitCost` / `UnitPrice` **不變** |
| `TestCreateSaleUnknownProduct` | 商品不存在時回傳 error，且不寫入任何列 |
| `TestConfirmProductionWritesConsumption` | 回傳的 `Consumption` 與 `domain.ProductConsumption` 一致，且已落庫 |
| `TestConfirmProductionIsAtomic` | 刻意讓 consumption 插入失敗（塞入負數 `consumed_g` 觸發 `CHECK`），驗證 `production_logs` 也沒有殘留 |
| `TestConfirmProductionSurvivesRecipeChange` | 確認生產後改配方，`ListProductionLogs` 的消耗量不變 |
| `TestDeleteProductKeepsSalesHistory` | 刪商品後 `sales` 該列仍存在，`ProductID` 變空、`ProductName` 保留 |
| `TestDeleteProductKeepsProductionConsumption` | 同上，`production_consumption` 不受影響 |

**不明顯的實作決定:**

- `CreateSale` 與 `ConfirmProduction` 必須**在一個交易內**完成「讀配方 → 算數字 → 寫入」。若中間隔了一次 commit，並發或錯誤路徑可能寫出半套資料。
- `ConfirmProduction` 先 `INSERT production_logs` 取得 `log_id`，再批次 `INSERT production_consumption`，任一失敗整體 `Rollback`。
- `sales.product_id` / `production_logs.product_id` 是 `ON DELETE SET NULL`，掃描時要用 `sql.NullString` 讀，轉成 Go 的空字串。
- `ListProducts` 不在 store 層附加成本 —— 成本是 domain 的責任，由 API 層組裝。這保持 store 只做持久化。

- [ ] **Step 1: 寫上表的失敗測試**
- [ ] **Step 2: 確認紅**
- [ ] **Step 3: 實作三個檔案**
- [ ] **Step 4: 確認綠**
- [ ] **Step 5: Commit** —— `feat(babywei-bakery): 🗄️ store CRUD —— 商品、出貨與生產（消耗快照走交易）`

---

### Task 8: HTTP server 骨架

**Files:**

- Create: `babywei-bakery/internal/api/respond.go`
- Create: `babywei-bakery/internal/api/router.go`
- Create: `babywei-bakery/internal/assets/embed.go`
- Create: `babywei-bakery/main.go`
- Test: `babywei-bakery/internal/api/router_test.go`

**Interfaces:**

- Consumes: `*store.DB`、`assets.FS`
- Produces:
  - `api.New(db *store.DB, static fs.FS) http.Handler`
  - `api.writeJSON(w http.ResponseWriter, status int, v any)`
  - `api.writeError(w http.ResponseWriter, status int, msg string)` —— 輸出 `{"error": "訊息"}`
  - `assets.FS fs.FS` —— `//go:embed` 前端產出，Task 18 前先放一個 placeholder `index.html`
  - `main()` —— 解析資料目錄、`store.Open`、起服務、`open` 瀏覽器

**必測案例:**

| 測試 | 判準 |
| --- | --- |
| `TestHealthz` | `GET /healthz` 回 200 且 body 含 `"ok"` |
| `TestStaticIndexServed` | `GET /` 回 200，`Content-Type` 為 `text/html` |
| `TestSPAFallback` | `GET /任意不存在路徑`（非 `/api` 前綴）回 `index.html` 而非 404 |
| `TestAPINotFoundReturnsJSON` | `GET /api/nope` 回 404 且 body 是 `{"error": ...}` |
| `TestMethodNotAllowedReturnsJSON` | 對只支援 GET 的端點下 POST 回 405 JSON |

**不明顯的實作決定:**

- 用 Go 1.22+ 的 `http.ServeMux` 路徑模式（`"GET /api/purchases/{id}"`），不需要第三方 router。
- `/api` 前綴的 404 必須回 JSON，其餘路徑回 `index.html`（SPA fallback）。順序寫錯會讓 API 的 typo 回傳一頁 HTML，前端只會看到 JSON parse 錯誤，難以診斷。
- **資料目錄解析**：`BAKERY_DATA_DIR` 優先；否則從 `os.Executable()` 起 `filepath.Dir` 四次取得 `BabyWei Bakery/`，再接 `data/`。開發時 binary 不在 `.app` 內，一律用環境變數。
- 埠被占用時往後試 `8788`、`8789`…最多 10 個，並以實際埠開啟瀏覽器。
- `main.go` 開瀏覽器用 `exec.Command("open", url)`，失敗只印訊息不中止服務。

- [ ] **Step 1–5**：同前循環。Commit —— `feat(babywei-bakery): 🌐 HTTP 服務骨架、JSON 錯誤格式與 SPA fallback`

---

### Task 9: API —— purchases / doughs / fillings / products

**Files:**

- Create: `babywei-bakery/internal/api/handlers_crud.go`
- Modify: `babywei-bakery/internal/api/router.go`（註冊路由）
- Test: `babywei-bakery/internal/api/handlers_crud_test.go`

**Interfaces:**

- Consumes: Task 6–7 的 store 方法、`api.writeJSON` / `writeError`
- Produces: spec 第 8 節前四組端點

```http
GET    /api/purchases?from=&to=&q=
POST   /api/purchases
PATCH  /api/purchases/{id}
DELETE /api/purchases/{id}
GET    /api/doughs
POST   /api/doughs
PATCH  /api/doughs/{id}
DELETE /api/doughs/{id}
GET    /api/fillings
POST   /api/fillings
PATCH  /api/fillings/{id}
DELETE /api/fillings/{id}
GET    /api/products
POST   /api/products
PATCH  /api/products/{id}
DELETE /api/products/{id}
```

**必測案例（用 `net/http/httptest` + `store.OpenMemory()`）:**

| 測試 | 判準 |
| --- | --- |
| `TestPostPurchaseReturns201WithID` | 201，回傳 body 的 `id` 非空 |
| `TestPostPurchaseValidation` | 缺 `name`、`weightG <= 0`、`price < 0` 各回 400 且 `error` 訊息可讀 |
| `TestGetPurchasesQueryParams` | `from`/`to`/`q` 正確傳遞到 store |
| `TestPatchPurchaseNotFound` | 404 |
| `TestDeletePurchaseTwice` | 第一次 204、第二次 404 |
| `TestGetProductsIncludesUnitCost` | 回傳每項商品的 `unitCost`（由 `domain.ProductUnitCost` 算出，非 store 欄位） |
| `TestPostProductValidation` | `doughId` 不存在回 400；`doughWeightG <= 0` 回 400 |
| `TestDeleteDoughInUseReturns409` | 被商品引用時回 409 而非 500 |
| `TestMalformedJSONReturns400` | body 不是合法 JSON 時回 400 |

**不明顯的實作決定:**

- `GET /api/products` 的回應是 `store.Product` 加上一個 `unitCost` 欄位。定義一個 `productResponse` 結構嵌入 `store.Product` 並加欄位，不要污染 store 的型別。
- 外鍵約束失敗（`ON DELETE RESTRICT`）要轉成 **409 Conflict**，不是 500。判斷方式是檢查 error 字串含 `FOREIGN KEY constraint failed`。
- `PATCH` 採「全欄位取代」語意（前端一律送完整物件），不做部分更新 —— 對這個規模的表單，部分更新的複雜度不划算。文件要寫清楚。

- [ ] **Step 1–5**：同前循環。Commit —— `feat(babywei-bakery): 🌐 API —— 進貨、配方、配料、商品 CRUD`

---

### Task 10: API —— sales / production / inventory / reports

**Files:**

- Create: `babywei-bakery/internal/api/handlers_ops.go`
- Modify: `babywei-bakery/internal/api/router.go`
- Test: `babywei-bakery/internal/api/handlers_ops_test.go`

**Interfaces:**

```http
GET    /api/sales?from=&to=
POST   /api/sales
DELETE /api/sales/{id}
POST   /api/production/preview
POST   /api/production
GET    /api/inventory
GET    /api/reports/summary
GET    /api/reports/sales?from=&to=
```

**必測案例:**

| 測試 | 判準 |
| --- | --- |
| `TestPostSaleSnapshots` | 回應含 `unitCost` / `unitPrice`；之後改售價不影響 `GET /api/sales` |
| `TestPostProductionPreviewDoesNotPersist` | 回傳換算結果，但 `GET /api/inventory` 的消耗量不變 |
| `TestPostProductionDeductsInventory` | 確認生產後 `GET /api/inventory` 的 `totalUsedG` 增加對應克數 |
| `TestPostProductionQtyValidation` | `qty <= 0` 回 400 |
| `TestGetInventoryIncludesNeverPurchased` | 漏登進貨的材料出現在回應中且 `remainingG` 為負 |
| `TestGetReportsSummaryFourBuckets` | 回應含 `day` / `month` / `quarter` / `year` 四個 key |
| `TestGetReportsSalesRange` | 區間篩選含端點，並回傳 `totals` |

**不明顯的實作決定:**

- `/api/production/preview` 與 `/api/production` 共用同一段換算邏輯，差別只在**是否進交易寫入**。抽一個內部函數回傳 `map[string]float64`，兩個 handler 都呼叫它。
- `GET /api/reports/sales` 的回應是 `{"sales": [...], "totals": {...}}`，讓前端不必自己加總（加總邏輯只存在 domain 一處）。
- `POST /api/production` 的 `loggedDate` 若前端沒送，後端用 `time.Now()` 填 —— **不可寫死日期**，這是修正原型的第三個 bug。

- [ ] **Step 1–5**：同前循環。Commit —— `feat(babywei-bakery): 🌐 API —— 出貨、生產、庫存與報表`

---

### Task 11: API —— 匯出與匯入

**Files:**

- Create: `babywei-bakery/internal/store/transfer.go`
- Create: `babywei-bakery/internal/api/handlers_transfer.go`
- Modify: `babywei-bakery/internal/api/router.go`
- Test: `babywei-bakery/internal/store/transfer_test.go`、`babywei-bakery/internal/api/handlers_transfer_test.go`

**Interfaces:**

```http
GET  /api/export/backup.json
POST /api/import
```

- `(*DB) ExportAll() (store.Snapshot, error)`
- `(*DB) ImportLegacy(raw []byte) (store.ImportReport, error)`
- `store.Snapshot` —— 六個 slice：`Purchases` / `Doughs` / `Fillings` / `Products` / `Sales` / `ProductionLogs`
- `store.ImportReport{Counts map[string]int; Warnings []string}`

**匯入的欄位對應（原型 `localStorage` 的 `babywei_local`）:**

| 原型欄位 | 目標 |
| --- | --- |
| `costDB[].weight` | `purchases.weight_g` |
| `costDB[].purchaseDate` | `purchases.purchase_date` |
| `dough[].ingredients[].pct` | `dough_ingredients.pct` |
| `fillings[].ingredients[].weight` | `filling_ingredients.weight_g` |
| `products[].doughWeight` | `products.dough_weight_g` |
| `products[].fill1Weight` / `fill2Weight` | `products.fill1_weight_g` / `fill2_weight_g` |
| `sales[].date` | `sales.sale_date` |
| `productionLogs[].date` | `production_logs.logged_date` |

**必測案例:**

| 測試 | 判準 |
| --- | --- |
| `TestExportImportRoundTrip` | 匯出再匯入，六張表筆數與內容一致 |
| `TestImportLegacyFieldMapping` | 用 spec 附的原型 `sample` 結構匯入，各欄位落到正確的目標欄位 |
| `TestImportBackfillsConsumption` | 原型的 `productionLogs` 沒有消耗明細，匯入後 `production_consumption` 已用匯入當下的配方回推填入 |
| `TestImportIsAtomic` | 刻意讓中途失敗（`products` 引用不存在的 `doughId`），驗證原有資料完整保留、沒有被清空 |
| `TestImportCreatesBackupFirst` | 匯入前 `data/backups/` 多一份 |
| `TestImportReportsUnmappable` | 無法對應的項目出現在 `Warnings`，不靜默丟棄 |

**不明顯的實作決定:**

- 匯入是**破壞性**操作：在同一交易內先 `DELETE FROM` 全部表再寫入，任一步失敗整體 `Rollback`。執行前先 `VACUUM INTO` 一份備份。
- 原型的 `productionLogs` 只有 `productId` 與 `qty`，沒有消耗明細。匯入時以**匯入當下的配方**回推並寫死 —— 這是一次性近似，必須在 `Warnings` 中明確告知，因為它無法還原成當時的真實消耗。
- 匯入前先驗證整份 JSON 的引用完整性（`products.doughId` 指向的 `dough` 是否存在），再開始寫。先驗證比靠交易回滾更能給出可讀的錯誤訊息。

- [ ] **Step 1–5**：同前循環。Commit —— `feat(babywei-bakery): 🔄 全量匯出與原型資料匯入`

---

### Task 12: Vite + Vue 3 骨架、`api.js` 與 tab 框架

**Files:**

- Create: `babywei-bakery/web/package.json`
- Create: `babywei-bakery/web/vite.config.js`
- Create: `babywei-bakery/web/index.html`
- Create: `babywei-bakery/web/src/main.js`
- Create: `babywei-bakery/web/src/api.js`
- Create: `babywei-bakery/web/src/style.css`
- Create: `babywei-bakery/web/src/App.vue`
- Modify: playground repo 根 `.gitignore`（已含 `node_modules/`，確認生效）

**Interfaces:**

- Consumes: Task 8–11 的 API 端點
- Produces:
  - `api.js` 匯出：`listPurchases`、`createPurchase`、`updatePurchase`、`deletePurchase`、`listDoughs`、`createDough`、`updateDough`、`deleteDough`、`listFillings`、`createFilling`、`updateFilling`、`deleteFilling`、`listProducts`、`createProduct`、`updateProduct`、`deleteProduct`、`listSales`、`createSale`、`deleteSale`、`previewProduction`、`confirmProduction`、`getInventory`、`getSummary`、`getSalesReport`、`exportBackup`、`importLegacy`
  - `App.vue` 提供 7 個 tab 的切換框架，每個 tab 一個子元件

- [ ] **Step 1: 建立前端專案**

```bash
cd ~/Git/robywei/playground/babywei-bakery
mkdir -p web/src/components
cd web
npm init -y
npm pkg set name=babywei-bakery type=module private=true
npm pkg set scripts.dev="vite" scripts.build="vite build"
npm i vue@3.5.42
npm i -D vite@8.2.2 @vitejs/plugin-vue@6.0.8
```

- [ ] **Step 2: `vite.config.js`**

```js
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  // build 產出直接落在 Go 的 embed 目錄。go:embed 不能跨越 ..，
  // 所以產出必須實際位於 internal/assets/ 之下。
  build: {
    outDir: '../internal/assets/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    // 開發時前端在 5173、後端在 8787。/api 一律代理過去，
    // 讓 api.js 用相對路徑即可，production 與 dev 走同一份程式碼。
    proxy: {
      '/api': { target: 'http://127.0.0.1:8787', changeOrigin: true },
    },
  },
})
```

- [ ] **Step 3: `src/api.js`**

單一 fetch 封裝層 —— **前端不得有第二處呼叫 `fetch`**。所有函數用相對路徑（`/api/...`），錯誤時丟出帶後端 `error` 訊息的 `Error`。

```js
async function request(path, { method = 'GET', body } = {}) {
  const res = await fetch(path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  if (res.status === 204) return null
  const text = await res.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      throw new Error(`後端回應不是合法 JSON（HTTP ${res.status}）`)
    }
  }
  if (!res.ok) throw new Error(data?.error || `HTTP ${res.status}`)
  return data
}

const qs = (params) => {
  const s = new URLSearchParams(
    Object.entries(params).filter(([, v]) => v !== '' && v != null),
  ).toString()
  return s ? `?${s}` : ''
}

export const listPurchases = (from = '', to = '', q = '') =>
  request(`/api/purchases${qs({ from, to, q })}`)
export const createPurchase = (p) => request('/api/purchases', { method: 'POST', body: p })
export const updatePurchase = (p) =>
  request(`/api/purchases/${encodeURIComponent(p.id)}`, { method: 'PATCH', body: p })
export const deletePurchase = (id) =>
  request(`/api/purchases/${encodeURIComponent(id)}`, { method: 'DELETE' })
// doughs / fillings / products / sales 依同一模式，端點見 Task 9–11
```

- [ ] **Step 4: `src/App.vue` tab 框架**

7 個 tab 對應 7 個元件，沿用原型的名稱與順序：成本庫（進貨管理）、庫存管理、產品配方、配料（克數）、商品設定、今日生產、利潤與報表。

樣式從 `templates/index.html` 的 `<style>` 移植到 `src/style.css`，**不改視覺** —— 這一版的目標是行為正確，不重新設計介面。

移除原型標題中的「Google 試算表雲端同步版」與寫死的「雲端同步就緒 ☁️」徽章：本版不做同步，留著是誤導。

- [ ] **Step 5: 驗證 dev server 能起來並打到後端**

```bash

## 終端機 A
cd ~/Git/robywei/playground/babywei-bakery
BAKERY_DATA_DIR=$(pwd)/data go run .

## 終端機 B
cd ~/Git/robywei/playground/babywei-bakery/web && npm run dev
```

預期：瀏覽 `http://localhost:5173` 看到 7 個 tab；DevTools 的 Network 顯示 `/api/*` 經 proxy 回 200。

- [ ] **Step 6: Commit** —— `feat(babywei-bakery): 🎨 Vite + Vue 3 骨架、\`api.js\` 與 7 個 tab 框架`

---

### Task 13: 成本庫與庫存 tab

**Files:**

- Create: `babywei-bakery/web/src/components/PurchasesTab.vue`
- Create: `babywei-bakery/web/src/components/InventoryTab.vue`
- Modify: `babywei-bakery/web/src/App.vue`

**行為要求:**

- 新進貨表單：材料名稱（`<datalist>` 提供既有名稱）、品牌、購入日期（**預設今天，由 `new Date()` 取得**）、購入管道、總價格、總重量
- 採購明細查詢：起迄日期 + 關鍵字，結果表格顯示每克單位成本（4 位小數）
- 匯出採購明細 CSV：沿用原型格式，**保留 UTF-8 BOM**（`﻿`），否則 Excel 開啟中文會亂碼
- 庫存表：材料、品牌、總進貨量、已生產消耗、目前剩餘、狀態徽章；`status` 為 `out` / `low` / `ok` 分別對應紅 / 橙 / 綠
- 剩餘為負值時額外提示「可能有漏登的進貨」

- [ ] **Step 1–3**：實作兩個元件並掛進 `App.vue`
- [ ] **Step 4: 手動驗收** —— 新增一筆進貨 → 重新載入頁面 → 資料仍在（驗證持久化）；庫存表出現該材料
- [ ] **Step 5: Commit** —— `feat(babywei-bakery): 🎨 成本庫與庫存 tab`

---

### Task 14: 產品配方與配料 tab

**Files:**

- Create: `babywei-bakery/web/src/components/DoughsTab.vue`
- Create: `babywei-bakery/web/src/components/FillingsTab.vue`
- Modify: `babywei-bakery/web/src/App.vue`

**行為要求:**

- 兩個 tab 結構相同，差別只在單位：配方是 Baker's %、配料是絕對克數
- 搜尋框（`<datalist>`）載入既有配方進入編輯狀態
- 材料列可新增 / 刪除 / 調整順序，順序會被保存（對應 `sort` 欄位）
- 即時預覽：顯示按當前比例換算的結果，讓使用者輸入時就看得到
- 刪除被商品引用的配方時，後端回 409 —— 前端要顯示可讀訊息（「此配方正被商品使用，請先移除該商品」），不可顯示原始 SQL 錯誤

- [ ] **Step 1–3**：實作兩個元件
- [ ] **Step 4: 手動驗收** —— 建配方 → 存 → 重新載入 → 材料順序與內容一致；試刪被引用的配方，確認看到可讀訊息
- [ ] **Step 5: Commit** —— `feat(babywei-bakery): 🎨 產品配方與配料 tab`

---

### Task 15: 商品設定 tab

**Files:**

- Create: `babywei-bakery/web/src/components/ProductsTab.vue`
- Modify: `babywei-bakery/web/src/App.vue`

**行為要求:**

- 商品名稱、預計售價、產品配方（必選）+ 每個使用重量、配料 1 / 2（選填）+ 重量
- 顯示後端算出的**單顆成本**與**毛利率**（`(售價 − 成本) / 售價`），售價為 0 時顯示「—」而非 `Infinity` 或 `NaN`
- 成本明細：列出各材料的用量與成本貢獻，讓使用者看得出錢花在哪
- 表單驗證在前端先攔一次（配方必選、重量須 > 0），但**後端仍須驗證** —— 前端驗證是體驗，不是防線

- [ ] **Step 1–3**：實作元件
- [ ] **Step 4: 手動驗收** —— 建商品 → 單顆成本非 0 且與手算一致 → 改一筆進貨價 → 成本跟著變（驗證加權平均生效）
- [ ] **Step 5: Commit** —— `feat(babywei-bakery): 🎨 商品設定 tab，含單顆成本與毛利率`

---

### Task 16: 今日生產 tab

**Files:**

- Create: `babywei-bakery/web/src/components/ProductionTab.vue`
- Modify: `babywei-bakery/web/src/App.vue`

**行為要求:**

- 選商品 + 輸入生產數量 → 呼叫 `POST /api/production/preview` 顯示換算結果（**不寫入**）
- 指標卡：單顆總重、本批配方總重、本批配料總重
- 換算表：產品配方與各配料分開列出，顯示原比例與本批用量
- 「產生 A4 PDF 生產表」：沿用原型的 `window.print()` 做法，開新視窗寫入列印用 HTML
- 「確認完成生產並扣除庫存」：呼叫 `POST /api/production`，成功後**必須重新載入庫存**，否則使用者會看到過期數字
- 確認生產前顯示明確提示：這個動作會扣庫存且無法在介面上撤銷

- [ ] **Step 1–3**：實作元件
- [ ] **Step 4: 手動驗收** —— preview 不改庫存；確認生產後庫存的「已生產消耗」增加對應克數；**之後去改配方，回來看庫存數字不變**（驗證消耗快照）
- [ ] **Step 5: Commit** —— `feat(babywei-bakery): 🎨 今日生產 tab，preview 與確認扣庫存分離`

---

### Task 17: 利潤與報表 tab

**Files:**

- Create: `babywei-bakery/web/src/components/ReportsTab.vue`
- Modify: `babywei-bakery/web/src/App.vue`

**行為要求:**

- 儀表板四張卡：本日 / 本月 / 本季 / 本年度的利潤與營收（來自 `GET /api/reports/summary`）
- 利潤為負時套用警示色（沿用原型的 `.profit-warn`）
- 新增出貨紀錄：日期（**預設今天**）、商品（`<datalist>` 搜尋）、數量
- 歷史查詢：起迄日期 → 表格 + 總營收 / 總成本 / 預估毛利（**加總取自後端 `totals`，前端不自己算**）
- 匯出銷售 CSV：沿用原型格式，**保留 UTF-8 BOM**
- 全量備份下載：呼叫 `GET /api/export/backup.json` 觸發下載
- 匯入原型資料：貼上 `localStorage` 的 JSON → `POST /api/import` → 顯示 `Counts` 與 `Warnings`。**匯入是破壞性的，UI 必須明確警告並要求二次確認**

- [ ] **Step 1–3**：實作元件
- [ ] **Step 4: 手動驗收** —— 新增出貨 → 儀表板數字增加 → 改該商品售價 → **已成立的出貨紀錄金額不變**（驗證快照）
- [ ] **Step 5: Commit** —— `feat(babywei-bakery): 🎨 利潤與報表 tab，含備份下載與原型資料匯入`

---

### Task 18: `embed.FS` 整合與單一 binary 驗證

**Files:**

- Modify: `babywei-bakery/internal/assets/embed.go`
- Create: `babywei-bakery/scripts/build.sh`
- Modify: playground repo 根 `.gitignore`

**Interfaces:**

- Consumes: `web/` 的 `vite build` 產出
- Produces: `babywei-bakery/bakery` —— 前端已內嵌的單一執行檔

- [ ] **Step 1: `internal/assets/embed.go`**

```go
// Package assets 持有編進 binary 的前端產出。
package assets

import (
	"embed"
	"io/fs"
)

// dist 由 web/ 的 `vite build` 產生。用 all: 前綴才會包含以 . 或 _
// 開頭的檔案（Vite 的 assets 目錄下會有）。
//
//go:embed all:dist
var dist embed.FS

// FS 回傳以 dist/ 為根的檔案系統。
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // 建置期就該發現，不是執行期錯誤
	}
	return sub
}
```

⚠️ `//go:embed all:dist` 在 `internal/assets/dist/` 不存在時**編譯就會失敗**。因此 repo 必須保留一份佔位 `internal/assets/dist/index.html`（納入版控），讓乾淨 clone 也能 `go build`。`vite build` 的 `emptyOutDir: true` 會覆蓋它。

- [ ] **Step 2: `scripts/build.sh`**

```bash

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "==> 建置前端"
(cd web && npm ci && npm run build)

echo "==> 建置 binary"
go build -ldflags="-s -w" -o bakery .

printf '==> 完成: %s (%.1f MB)\n' "$ROOT/bakery" \
  "$(echo "scale=1; $(stat -f %z bakery)/1048576" | bc)"
```

- [ ] **Step 3: 建置並驗證單一 binary**

```bash
cd ~/Git/robywei/playground/babywei-bakery
./scripts/build.sh
BAKERY_DATA_DIR=/tmp/bakery-smoke ./bakery
```

預期：瀏覽器自動開啟 `http://127.0.0.1:8787`，7 個 tab 都能操作，`/tmp/bakery-smoke/babywei.db` 被建立。

- [ ] **Step 4: 驗證持久化與備份**

```bash

## 在介面新增一筆進貨後停止服務，再重啟
ls -la /tmp/bakery-smoke/
ls -la /tmp/bakery-smoke/backups/
sqlite3 /tmp/bakery-smoke/babywei.db 'SELECT count(*) FROM purchases;'
```

預期：`babywei.db` 存在且**沒有 `-wal` / `-shm` 檔案**；第二次啟動後 `backups/` 有一份快照；進貨筆數與介面一致。

- [ ] **Step 5: 驗證 binary 可搬移**

```bash
otool -L bakery | sed -n '2,10p'
codesign -dv bakery 2>&1 | grep Signature
```

預期：只依賴 `/usr/lib` 與 `/System/Library`（無 Homebrew 路徑）；`Signature=adhoc`。

- [ ] **Step 6: Commit** —— `feat(babywei-bakery): 📦 前端 embed 進 binary，單一執行檔可獨立運行`

---

## Self-Review

### Spec 覆蓋

| Spec 節 | 對應 Task |
| --- | --- |
| 5.1 開發 repo 結構 | Task 1、8、12（含一處刻意偏離：schema 移入 `internal/store/`） |
| 5.2 交付結構與資料目錄解析 | Task 8（`BAKERY_DATA_DIR` 與四層解析） |
| 6 資料模型（9 張表） | Task 1 |
| 6 外鍵 DSN | Task 1（`TestForeignKeysEnforced`） |
| 6.1(a) 生產消耗快照 | Task 4（`TestInventoryUsesSnapshotNotCurrentRecipe`）、Task 7（`TestConfirmProductionSurvivesRecipeChange`） |
| 6.1(b) 未進貨材料不被丟棄 | Task 4（`TestInventoryIncludesNeverPurchasedIngredient`） |
| 6.1(c) 寫死日期 | Task 10、13、17（預設今天一律 `new Date()` / `time.Now()`） |
| 7.1 全期加權平均 | Task 3（`TestCostPerGramWeightedAverage`、`TestCostPerGramIsNotLatestPurchase`） |
| 7.2 配方換算 | Task 2 |
| 7.3 商品單顆成本 | Task 3 |
| 7.4 庫存 | Task 4 |
| 7.5 利潤報表 | Task 5 |
| 8 API 端點 | Task 9、10、11 |
| 9 資料保留與備份 | Task 1（備份與輪替）、Task 18 Step 4（實機驗證無 `-wal`） |
| 10 交付與安裝 | **不在本計劃** —— 階段 4 另立 |
| 11 從現有資料遷移 | Task 11 |
| 12 測試策略 | Task 1–11 各自的測試；前端不做 E2E（Task 13–17 為手動驗收） |
| 13 實作階段 1–3 | Task 1–18 |

**已知缺口（刻意）:** spec 第 10 節（`.app` bundle、`build-release.sh`、zip、Gatekeeper 說明）不在本計劃。本計劃完成時應用程式可用 `./bakery` 執行，但還不是雙擊可用的交付包。

### 型別一致性

- `store.Purchase.WeightG` 全程用 `WeightG`（非 `Weight`）；JSON 為 `weightG`
- `domain.Component.Ratio` 是唯一的換算輸入欄位名；`FromDough` / `FromFilling` 是唯二的轉換入口
- `domain.Totals` 的欄位是 `RevenueTWD` / `CostTWD` / `ProfitTWD`，JSON 為 `revenueTwd` / `costTwd` / `profitTwd`；Task 5、10、17 一致
- `InventoryRow.Status` 取值固定為 `"out"` / `"low"` / `"ok"`；Task 4 定義、Task 10 傳遞、Task 13 消費
- `ConfirmProduction` 全程用此名（非 `Produce` 或 `ConfirmAndDeduct`）；Task 7 定義、Task 10 呼叫
- `store.ProductionLog.Consumption` 是 `[]Consumption`（非 map）；Task 1 定義、Task 4 與 7 消費
