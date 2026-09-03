# BabyWei Bakery

麵包店的成本、庫存、配方與利潤管理。單機執行，資料存在自己的電腦上。

Vue 3 前端經 `vite build` 後由 Go 的 `go:embed` 編進 binary，交付物是**單一執行檔**
包成的 `.app` —— 目標 Mac 不需要預裝 Node、Python 或任何 runtime。

- 設計文件：[`docs/superpowers/specs/`](docs/superpowers/specs/)
- 實作計劃：[`docs/superpowers/plans/`](docs/superpowers/plans/)
- 原始參考範本：[`templates/index.html`](templates/index.html)（V3.8 單檔原型，不再維護）

## 快速開始

開發時前後端分開跑，`/api` 由 Vite 代理到後端：

```sh
## 終端機 A —— 後端
go -C server run . 

## 終端機 B —— 前端（有 HMR）
cd web && npm install && npm run dev
```

開 <http://localhost:5173>。後端在 8787，被占用時自動往後遞增。

只想跑完整的單一 binary（不需要 HMR）：

```sh
./scripts/build.sh && ./bin/bakery
```

## 目錄結構

```text
server/            Go 後端。分層是 store ← domain ← api
  internal/store/    持久化。不 import domain，否則循環依賴
  internal/domain/   純計算：配方換算、成本、庫存、報表
  internal/api/      路由與 handler，串接前兩者
  internal/assets/   go:embed 前端產出
web/               Vue 3 前端。src/api.js 是唯一的 fetch 封裝層
scripts/           建置腳本
bin/               開發建置產出（gitignore）
release/           交付包（gitignore）
data/              執行期資料（gitignore）
```

`internal/domain/` 的四個檔案是純函數，不碰資料庫也不碰 HTTP。成本、庫存、
利潤這三項計算是這套系統的全部價值，也是最容易算錯的地方 —— 隔離出來才能
獨立測試。**新增計算邏輯放這裡，不要塞進 handler。**

`bin/` 刻意放在專案層級而非 `server/` 底下：那個 binary 內含 97 KB 的前端
bundle，是完整產品而不是後端產物。

## 測試

```sh
go -C server test ./...
```

前端不做 E2E（成本不成比例），改以 API 層的整合測試涵蓋。

## 交付

```sh
## 用法: ./scripts/build-release.sh [版本] [universal|arm64|amd64]
./scripts/build-release.sh 0.1.0 universal
```

產出 `release/BabyWei Bakery-<版本>-<架構>.zip`，內含 `.app`、空的 `data/`
與安裝說明。

⚠️ **不確定對方的 Mac 是 Apple Silicon 還是 Intel 就用 `universal`。**
架構不符時 macOS 只會說「應用程式無法在此 Mac 上使用」，收到的人無從判斷原因。
universal 約 9.4 MB、arm64 約 4.5 MB。

⚠️ **交付後對方首次使用前必須執行一次**
`xattr -dr com.apple.quarantine "<資料夾路徑>"`。未經 Apple 公證的程式會被
Gatekeeper 擋下，畫面只說「無法驗證開發者」。這一行寫在包內的 `安裝說明.txt`。

## 資料與備份

| 項目 | 位置 |
| --- | --- |
| 資料庫 | `data/babywei.db`（SQLite，單一檔案） |
| 自動備份 | `data/backups/`，每次啟動 `VACUUM INTO` 一份，保留最近 30 份 |
| 手動匯出 | 介面的「利潤與報表」分頁 → 下載完整備份 JSON |

`journal_mode` 保持預設的 `delete` 而非 WAL：單一使用者用不到 WAL 的並發優勢，
而 `-wal` / `-shm` 兩個附屬檔案會破壞「複製整個資料夾即完整備份」的保證。

## 踩雷筆記

- **SQLite 外鍵預設是關的。** DSN 必須帶 `?_pragma=foreign_keys(1)`，否則所有
  `ON DELETE CASCADE` 靜默失效、孤兒資料能寫進去。必須走 DSN 而非 `PRAGMA`
  陳述式 —— `database/sql` 有連線池，逐次 PRAGMA 只作用在其中一條連線上。
- **`go:embed` 不能跨越 `..`。** 所以 `web/vite.config.js` 的 `outDir` 指向
  `../server/internal/assets/dist`，而該目錄的佔位 `index.html` **必須進版控**
  —— 目錄不存在時是編譯錯誤，乾淨 clone 會直接建不起來。
- **`xattr -dr` 在程式執行中會失敗**（macOS 保護執行中程式的屬性），
  訊息是 `Operation not permitted`。先關掉再清。
- **`fsevents` 的 npm install-script 警告可以忽略。** 它是 Vite 的
  optionalDependency，套件自帶預編譯 binary，`node-gyp rebuild` 只是後備方案。
  核准它只會多要求 Xcode CLT。
