# BabyWei Bakery 設計文件

- 日期：2026-09-03
- 狀態：待審核
- 範本來源：`templates/index.html`（V3.8，759 行單檔原型）

## 1. 背景

`templates/index.html` 是一份可運作的單檔原型，涵蓋麵包店的進貨成本、原料庫存、配方管理、生產換算與利潤報表。它已經驗證了領域邏輯與介面流程，但有兩個根本問題：

1. **資料不保存**。標題與介面徽章宣稱「Google 試算表雲端同步版」，但程式碼中沒有任何 `fetch`，持久化只有 `localStorage.getItem("babywei_local")`。Safari 在 7 天無互動後會清除 localStorage，使用者清瀏覽資料也會全毀，且無法備份。那個「雲端同步就緒 ☁️」徽章是寫死的裝飾字串。
2. **領域邏輯無法測試**。成本、庫存、利潤三項計算全部埋在 DOM 操作中（例如 `calculateProd()` 一邊計算一邊組 `innerHTML`），共 34 個全域函數搭配 inline `onclick`。

本專案將它改造成可交付、資料可靠保存的本地應用程式。

## 2. 使用者與使用情境

- **唯一使用者**：專案發起人的配偶，非技術背景。
- **使用方式**：雙擊 `.app` 圖示啟動，在瀏覽器中操作。不接觸終端機。
- **執行環境**：單一台 Mac，單人使用，無並發。服務僅綁 `127.0.0.1`。
- **交付方式**：一個 `.zip`，解壓縮後即可執行，不需要事先安裝任何執行環境。
- **備份方式**：複製整個解壓目錄即為完整備份。

## 3. 非目標（YAGNI）

以下明確不做：

- 使用者認證與權限（單人單機）
- Google Sheets 雙向同步（改為 CSV 匯出）
- 遠端存取、HTTPS、部署
- Docker
- 多裝置或 LAN 共用
- 前端 E2E 測試（成本不成比例）

## 4. 技術選型

| 層 | 選型 | 版本 |
|---|---|---|
| 前端框架 | Vue 3 | 3.x |
| 前端建置 | Vite | 最新穩定版 |
| 後端 | Go `net/http` | 1.27 |
| 資料庫 | SQLite via `modernc.org/sqlite`（純 Go，無 CGO） | SQLite 3.53.4 |
| 前端內嵌 | Go `embed.FS` | — |
| 測試 | Go `testing` + `net/http/httptest` | — |
| 打包 | `zip -9` | — |

### 4.1 選型依據（實測）

**為什麼不用系統 Python**：`/usr/bin/python3` 是 118,640 bytes 的 shim，連結 `libxcselect.dylib`，內部識別為 `com.apple.dt.xcode_select.tool-shim-public`。它解析到 `/Library/Developer/CommandLineTools/.../Python3.framework/Versions/3.9/bin/python3`，也就是**隨 Xcode Command Line Tools 而來的 Python 3.9.6**，並非 macOS 本身提供。在未安裝 CLT 的 Mac 上執行 `python3` 會觸發「需要安裝命令列開發者工具」對話框（1–2 GB 下載），直接違背「解壓即可執行」。此外 Python 3.9 已於 2025-10 終止安全支援，隨附的 pip 為 2021 年的 21.2.4，且 Apple 已標示該 Python 僅供 CLT 用途、不保證未來版本繼續提供。

**為什麼不用 Node**：可行但代價高。Homebrew 安裝的 `node` 是 50 KB shim，`otool -L` 顯示其動態連結 20 餘個 `/opt/homebrew/opt/*` dylib，不可搬移；必須改用 nvm 或官方 tarball 的 binary（僅依賴 `/System/Library` 與 `/usr/lib/libSystem`，可搬移）。但該 binary 為 116.3 MB（zip -9 後 37.3 MB），且需額外的 launcher 腳本與健康檢查輪詢。

**為什麼選 Go**：實測建置一份 `net/http` + `modernc.org/sqlite` + `embed.FS` 的服務，得到 7.7 MB binary（zip -9 後 3.3 MB），僅依賴系統框架，且 Go 在 darwin/arm64 自動 ad-hoc 簽章（`flags=0x20002(adhoc,linker-signed)`），這在 Apple Silicon 上是執行的必要條件。交付物塌縮為單一檔案，使用者的 Mac 不需要任何預裝執行環境。

功能面驗證（同一份測試程式）：SQLite 版本 3.53.4、`journal_mode` 預設 `delete`、`VACUUM INTO` 可用、交易 rollback 正常、`embed.FS` 靜態檔服務正常。

### 4.2 交付大小對照

| | Go | Node | Python |
|---|---|---|---|
| 交付 zip | 3.3 MB | 37.3 MB | ~25–40 MB（需自帶 CPython） |
| 解壓後 | 7.7 MB | 116.3 MB | 60–120 MB |
| 交付檔案數 | 1（前端已 embed） | node + server/ + public/ | framework 整棵樹 |
| 使用者 Mac 需求 | 無 | 無（已打包） | CLT 1–2 GB，或自帶 |
| Universal（含 Intel Mac） | 14.1 MB / zip 6.1 MB | 各架構各一份 116 MB | 複雜 |

## 5. 架構

### 5.1 開發 repo

專案位於 playground repo 下的 `babywei-bakery/` 子目錄。目錄名採全小寫 kebab-case：`web/package.json` 的 `name` 欄位不允許大寫（npm 對新套件名一律拒絕），Go module path 中的大寫字母會在 module cache 被轉義為 `!` + 小寫，且混用大小寫的路徑在不分大小寫的 macOS 與分大小寫的 CI 之間是 git 的常見地雷。此命名僅適用於原始碼；交付給使用者的 bundle 仍為 `BabyWei Bakery.app`（見 5.2）。

```
babywei-bakery/
├── go.mod                      # module babywei-bakery
├── main.go                     # 進入點：解析路徑、起服務、開瀏覽器
├── internal/
│   ├── store/
│   │   ├── db.go               # 連線、migration、啟動備份
│   │   └── queries.go          # 所有 SQL
│   ├── domain/
│   │   ├── cost.go             # 加權平均單位成本、商品單顆成本
│   │   ├── recipe.go           # 配方換算（Baker % 與絕對克數）
│   │   ├── inventory.go        # 庫存推算
│   │   └── report.go           # 利潤統計
│   ├── api/
│   │   ├── router.go
│   │   └── handlers_*.go
│   └── assets/
│       └── embed.go            # //go:embed dist
├── schema/
│   └── 001_init.sql
├── web/                        # Vite + Vue 3 前端原始碼
│   ├── package.json
│   ├── vite.config.js          # build.outDir 指向 internal/assets/dist
│   ├── index.html
│   └── src/
│       ├── main.js
│       ├── App.vue
│       ├── api.js              # 唯一的 fetch 封裝層
│       └── components/         # 7 個 tab 對應 7 個元件
├── scripts/
│   └── build-release.sh
├── docs/superpowers/specs/
├── templates/index.html        # 原始參考範本，不再維護
└── data/                       # 執行期產生，已 gitignore
```

版控排除項（設於 playground repo 根目錄的 `.gitignore`）：`babywei-bakery/data/`、`babywei-bakery/bakery`、`babywei-bakery/release/`、`node_modules/`。

`internal/domain/` 是刻意的邊界：這四個檔案是純函數，輸入資料結構、輸出數字，不碰資料庫也不碰 HTTP。它們承載這套系統的全部價值，也是最容易算錯的地方，必須能獨立測試。

### 5.2 交付結構（解壓後）

```
BabyWei Bakery/
├── BabyWei Bakery.app/
│   └── Contents/
│       ├── Info.plist
│       └── MacOS/bakery            # Go binary，前端已 embed
├── data/                           # 使用者資料，升級時絕不觸碰
│   ├── babywei.db
│   └── backups/
└── 安裝說明.txt
```

`data/` 與 `.app` 分離，因此後續版本只需替換 `.app`，資料不受影響。

`bakery` 透過 `os.Executable()` 向上解析四層取得 `BabyWei Bakery/` 目錄（`.app/Contents/MacOS/bakery` → `MacOS` → `Contents` → `.app` → `BabyWei Bakery/`），再定位其下的 `data/`。若 `data/` 不存在則建立並寫入範例資料。

開發時 binary 不在 `.app` 內，路徑推導不成立，因此環境變數 `BAKERY_DATA_DIR` 的優先序高於路徑推導。

## 6. 資料模型

SQLite，`journal_mode=delete`（預設）。不使用 WAL：本應用只有單一使用者，WAL 的並發讀取優勢用不到，而它會額外產生 `-wal` 與 `-shm` 檔案，破壞「複製資料夾即完整備份」的保證。DELETE 模式在乾淨關閉後只留單一 `.db` 檔。

**外鍵約束必須在 DSN 明確開啟。** SQLite 的 `foreign_keys` 預設為關閉，`modernc.org/sqlite` 亦然。實測結果：使用預設 DSN 時 `PRAGMA foreign_keys` 回傳 0，本節所有 `ON DELETE CASCADE` **靜默失效**（刪除父列後子列殘留），且可成功插入指向不存在父列的孤兒資料而無任何錯誤。因此連線 DSN 必須為：

```
data/babywei.db?_pragma=foreign_keys(1)
```

以 DSN 參數而非 `db.Exec("PRAGMA ...")` 設定，是因為 `database/sql` 會維護連線池，逐次執行 PRAGMA 只會作用在池中某一條連線上。開啟後實測 CASCADE 正常、孤兒插入回傳 `FOREIGN KEY constraint failed (787)`。

```sql
-- 進貨紀錄（原 db.costDB）
CREATE TABLE purchases (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  brand         TEXT NOT NULL DEFAULT '',
  purchase_date TEXT NOT NULL,                    -- YYYY-MM-DD
  channel       TEXT NOT NULL DEFAULT '',
  price         REAL NOT NULL CHECK (price >= 0),
  weight_g      REAL NOT NULL CHECK (weight_g > 0)
);
CREATE INDEX idx_purchases_name ON purchases(name);
CREATE INDEX idx_purchases_date ON purchases(purchase_date);

-- 產品配方（原 db.dough），以 Baker's % 表示
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

-- 配料（原 db.fillings），以絕對克數表示
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

-- 商品（原 db.products）
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

-- 出貨紀錄（原 db.sales）
CREATE TABLE sales (
  id           TEXT PRIMARY KEY,
  sale_date    TEXT NOT NULL,                     -- YYYY-MM-DD
  product_id   TEXT REFERENCES products(id) ON DELETE SET NULL,
  product_name TEXT NOT NULL,                     -- 快照
  qty          INTEGER NOT NULL CHECK (qty > 0),
  unit_cost    REAL NOT NULL,                     -- 快照
  unit_price   REAL NOT NULL                      -- 快照
);
CREATE INDEX idx_sales_date ON sales(sale_date);

-- 生產紀錄（原 db.productionLogs）
CREATE TABLE production_logs (
  id           TEXT PRIMARY KEY,
  logged_date  TEXT NOT NULL,                     -- YYYY-MM-DD
  product_id   TEXT REFERENCES products(id) ON DELETE SET NULL,
  product_name TEXT NOT NULL,                     -- 快照
  qty          INTEGER NOT NULL CHECK (qty > 0)
);
CREATE INDEX idx_production_date ON production_logs(logged_date);

-- 生產原料消耗（新增，見 6.1）
CREATE TABLE production_consumption (
  log_id          TEXT NOT NULL REFERENCES production_logs(id) ON DELETE CASCADE,
  ingredient_name TEXT NOT NULL,
  consumed_g      REAL NOT NULL CHECK (consumed_g >= 0),
  PRIMARY KEY (log_id, ingredient_name)
);
```

Schema 版本以 `PRAGMA user_version` 追蹤，遷移檔為 `schema/NNN_*.sql`，依序執行。

### 6.1 對範本行為的三處修正

**(a) 生產消耗改為寫入時快照 —— 修正兩個 bug**

範本的 `renderInventory()` 從 `log.productId` 反查**當前**的商品與配方定義，即時重算歷史消耗量：

```js
let p = (db.products || []).find(x => x.id === log.productId);
if(p) { getUsage(... p.doughWeight * log.qty ...); }
```

這導致兩個缺陷：

1. 修改配方或商品用量後，**過去所有生產紀錄的庫存扣減量會回溯變動**。
2. 更嚴重：刪除商品後 `if(p)` 為 false，該批生產的消耗**直接歸零**，庫存數字憑空增加，且沒有任何警示。

修正方式：在確認生產的當下，於同一個交易內計算每項原料的消耗克數並寫入 `production_consumption`。庫存查詢改為直接加總該表，不再依賴當前配方定義。

**(b) 未曾進貨的材料消耗會被靜默丟棄**

範本的 `getUsage()` 中有 `if(ingMap[name]) ingMap[name].totalUsed += use;`，而 `ingMap` 只由進貨紀錄建立。若配方使用了某項從未進貨的材料，它的消耗會被無聲丟棄，且該材料不會出現在庫存表上。

修正方式：庫存表以「進貨紀錄」與「消耗紀錄」兩者的材料名稱聯集為列。從未進貨但已消耗的材料，總進貨量顯示 0、剩餘為負值，並標示為異常，讓使用者看得見漏登的進貨。

**(c) 寫死的日期**

範本有兩處寫死 `"2026-09-03"`：`load()` 中的 `todayStr`（初始化所有日期欄位），以及 `confirmProductionAndDeduct()` 中的 `date`。兩處都改為取當下日期。

`sales` 的 `unitCost` / `unitPrice` 在範本中**已經是快照**（`confirmSale` 寫入時就存下來），此處保留該正確行為。

## 7. 領域邏輯

### 7.1 材料單位成本：全期加權平均

範本取「最近一筆進貨價」（`getCostPerG` 依 `purchaseDate` 倒序取第一筆）。改為全期加權平均：

```
cost_per_g(材料) = Σ(該材料所有進貨的 price) / Σ(該材料所有進貨的 weight_g)
```

無進貨紀錄時回傳 0。

**已知取捨**：這是全期加權平均，非移動加權平均。若麵粉由每公斤 120 元漲至 150 元，舊進貨紀錄會永久將均價往下拉，即使舊貨早已用完。移動加權平均需追蹤每批進貨的剩餘量，較準確但複雜度顯著提高。決定先實作全期加權平均，並將其隔離在 `internal/domain/cost.go` 的單一函數中，日後可替換而不影響其他部分。

### 7.2 配方換算

沿用範本已驗證的公式（`buildProdTable`）。

產品配方（Baker's %）：
```
sum      = Σ pct
用量(材料) = 需求總重 × pct(材料) / sum
```

配料（絕對克數）：
```
sum      = Σ weight_g
用量(材料) = 需求總重 × weight_g(材料) / sum
```

兩者皆為正規化計算，因此主粉基準不必剛好等於 100% 也能得到正確比例。介面仍提示以 100% 為基準，但不強制驗證。

### 7.3 商品單顆成本

```
單顆成本 = Σ(產品配方各材料用量 × cost_per_g)
         + Σ(配料1各材料用量 × cost_per_g)
         + Σ(配料2各材料用量 × cost_per_g)
```

各段的用量以 7.2 的公式、依商品設定的 `dough_weight_g` / `fill1_weight_g` / `fill2_weight_g` 換算。

### 7.4 庫存

```
總進貨(材料) = Σ purchases.weight_g       WHERE name = 材料
總消耗(材料) = Σ production_consumption.consumed_g  WHERE ingredient_name = 材料
剩餘(材料)   = 總進貨 - 總消耗
```

列為兩表材料名稱的聯集。狀態門檻沿用範本：`剩餘 <= 0` 為見底、`< 200g` 為偏低、其餘為充足。

### 7.5 利潤報表

```
營收 = Σ (sales.qty × sales.unit_price)
成本 = Σ (sales.qty × sales.unit_cost)
利潤 = 營收 - 成本
```

儀表板區間沿用範本：本日、本月、本季、本年度，均以當下日期推算。查詢報表接受任意起迄日期區間。

## 8. API

全部為 JSON，僅綁 `127.0.0.1`，無認證。

```
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

GET    /api/products              # 含計算出的單顆成本
POST   /api/products
PATCH  /api/products/{id}
DELETE /api/products/{id}

GET    /api/sales?from=&to=
POST   /api/sales                 # 寫入時快照 unit_cost / unit_price
DELETE /api/sales/{id}

POST   /api/production/preview    # 換算但不寫入，供「今日生產」畫面即時試算
POST   /api/production            # 交易內寫入 log + consumption

GET    /api/inventory
GET    /api/reports/summary       # 本日/本月/本季/本年度
GET    /api/reports/sales?from=&to=

GET    /api/export/backup.json    # 全量匯出
POST   /api/import                # 匯入 localStorage 格式的 JSON（見第 11 節）

GET    /healthz
```

CSV 匯出（採購明細、銷售報表）與 A4 生產表列印維持在前端實作，沿用範本邏輯。

錯誤回應統一為 `{"error": "訊息"}`，搭配適當的 HTTP 狀態碼。驗證失敗回 400，找不到資源回 404。

## 9. 資料保留與備份

| 項目 | 做法 |
|---|---|
| 資料庫位置 | `BabyWei Bakery/data/babywei.db` |
| journal 模式 | `delete`（單檔，見第 6 節） |
| 自動備份 | **每次啟動**執行 `VACUUM INTO data/backups/YYYY-MM-DD-HHMMSS.db` |
| 備份輪替 | 保留最近 30 份，超出者由舊到新刪除 |
| Schema 遷移 | `PRAGMA user_version` + `schema/NNN_*.sql` |
| 手動匯出 | `GET /api/export/backup.json` |

備份是啟動時自動執行，不設任何使用者操作按鈕：使用者不會記得備份，因此不能依賴她記得。

`data/backups/` 內每份皆由 `VACUUM INTO` 產生，本質上是原子快照。因此即使在服務執行中複製整個資料夾，備份目錄內的檔案仍保證一致。

安裝說明中列出兩種備份方式：複製整個 `BabyWei Bakery/` 資料夾（最省事，約 8 MB），或只複製 `data/` 資料夾（最快，數 MB）。

## 10. 交付與安裝

`scripts/build-release.sh` 的步驟：

1. `cd web && npm ci && npx vite build` → 產出至 `internal/assets/dist`
2. `go build -ldflags="-s -w" -o bakery .`（darwin/arm64）
3. 組出 `.app` bundle 結構與 `Info.plist`
4. 建立 `data/` 空目錄與 `安裝說明.txt`
5. `zip -9 -r "BabyWei Bakery.zip" "BabyWei Bakery"`
6. 印出 SHA-256 checksum

`.app` 內就是 Go binary 本身，不需要 shell wrapper。binary 啟動後自行 `exec.Command("open", "http://127.0.0.1:8787")` 開啟瀏覽器。若埠已被占用，改試後續埠號並以實際埠開啟。

### 10.1 Gatekeeper

.zip 不論以 AirDrop、雲端硬碟或 USB 傳遞，解壓後檔案都會帶 `com.apple.quarantine` 擴充屬性。未經 Apple 公證的 `.app` 雙擊時只會顯示「無法打開，因為無法驗證開發者」，使用者無從判斷。

處理方式：首次安裝時由專案發起人執行一次

```
xattr -dr com.apple.quarantine "/path/to/BabyWei Bakery"
```

此步驟必須寫入 `安裝說明.txt`。永久解法是申請 Apple Developer 帳號（年費 99 美元）進行簽章與公證，目前不採用。

Go 在 darwin/arm64 已自動 ad-hoc 簽章，這是 Apple Silicon 執行的必要條件，無需額外處理。

## 11. 從現有資料遷移

`POST /api/import` 接受範本 `localStorage` 的 `babywei_local` 鍵值原始 JSON 結構：

```
{ costDB: [...], dough: [...], fillings: [...], products: [...], sales: [...], productionLogs: [...] }
```

欄位對應：`costDB[].weight` → `purchases.weight_g`、`costDB[].purchaseDate` → `purchases.purchase_date`、`dough[].ingredients[].pct` → `dough_ingredients.pct`、`fillings[].ingredients[].weight` → `filling_ingredients.weight_g`、`products[].doughWeight` → `products.dough_weight_g`（其餘同理）。

`productionLogs` 缺少消耗明細（範本從未儲存），匯入時以**匯入當下**的配方定義回推並寫入 `production_consumption`。這是一次性的近似，匯入後即固定。匯入結果回報每張表的筆數與任何無法對應的項目。

匯入為破壞性操作：在交易內先清空所有表再寫入，失敗則整體回滾。執行前先產生一份 `VACUUM INTO` 備份。

## 12. 測試策略

- **`internal/domain/`** —— 表格驅動單元測試，這是重點投資的地方。涵蓋：加權平均（含無進貨、單筆、多筆）、Baker % 與絕對克數換算、商品單顆成本、庫存聯集與負值、四個報表區間邊界。
- **`internal/store/`** —— 以 `:memory:` 資料庫測試 migration、`VACUUM INTO` 備份與輪替、確認生產的交易原子性（刻意讓中途失敗，驗證回滾）。
- **`internal/api/`** —— `net/http/httptest` 打真實 HTTP，`:memory:` 資料庫。涵蓋各端點的正常路徑、驗證失敗、404，以及匯入端點的欄位對應。
- **前端** —— 不做 E2E。`web/src/api.js` 若有純資料轉換邏輯則單元測試，元件不測。

## 13. 實作階段

**階段 1：資料層與領域邏輯**
schema、migration、啟動備份與輪替、`internal/domain/` 四個模組及其單元測試。此階段結束時所有計算邏輯已可獨立驗證。

**階段 2：API 層**
`net/http` router、第 8 節全部端點、`/api/import`、httptest 測試。此階段結束時後端功能完整，可用 curl 操作。

**階段 3：前端**
Vite + Vue 3 專案、7 個 tab 對應元件、`api.js` 取代所有 localStorage 呼叫、CSV 匯出與 A4 列印移植。開發時以 vite dev proxy 連後端。

**階段 4：交付**
`embed.FS` 整合、`.app` bundle、`build-release.sh`、`安裝說明.txt`、實機驗證解壓即可執行。

## 14. 待決事項

無。以下決定已確認：

- 前端框架：Vue 3 + Vite
- 後端語言：Go
- 成本算法：全期加權平均（移動加權平均列為日後可替換選項，見 7.1）
- 寫死日期：改為取當下日期
- 生產消耗快照：採用
- Google Sheets 同步：不做，僅保留 CSV 匯出
- 啟動方式：雙擊 `.app`，不常駐
