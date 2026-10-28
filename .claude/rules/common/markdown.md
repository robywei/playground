---
paths: ["**/*.md"]
---

# Markdown（讀寫 `.md` 時）

> 由 repo-template 管理（基礎層），要改請改範本；本 repo 專屬規則寫在 repo 自己的 CLAUDE.md

規則本身的唯一來源是 `.markdownlint-cli2.jsonc`（也由範本管理）。這裡只列工具修不了、或會修錯的。

## 檢查

- **一律跑 `./scripts/lint/lint.sh md`**，不要裸跑 `markdownlint`：另一支 cli v1 不讀 `.markdownlint-cli2.jsonc`、
  會噴一片 MD013；markdownlint 也不驗連結目標存不存在
- 🔴 看 `Linting: N files` 對帳檔案數 —— `Linting: 0 files` 照樣 exit 0，是假綠燈；
  `Summary: 0 issues in 0 files` 才是「檢查過且乾淨」
- 🔴 **設定檔是從目前目錄往上找**：在子目錄跑 markdownlint，找不到設定就靜默改用預設值（例如 MD013 變回 80），
  兩邊都不報「設定沒載入」；cli2 在子目錄不帶參數則只印用法、一個檔都沒檢查 → 一律從 repo 根跑
- IDE 報錯而 `lint.sh md` 全綠時，先看磁碟上那一行 —— 編輯器 buffer 可能停在檔案被外部覆寫之前

## 寫的時候就要對（`--fix` 修不了或會修錯）

- **MD013 行長 120** 是 `--fix` 不處理的 → 用 `scripts/lint/md-wrap.py` 折，不要手工折
  （續行開頭是 `-` `+` `*` `#` `>` 會變成清單、標題或引言，lint 不報）
- 🔴 **`--fix` 會靜默改壞內容，跑完必須 `git diff`**：glob（`*.a.xyz`）被當 emphasis 吃掉空格、`www.` 開頭被包成連結
  → glob、萬用字元、`www.` 開頭一律先用反引號包住
- ⚠️ **表格單元格內的 `|` 一律寫 `\|`**（code span 內也要）—— 否則撐爆欄數，整張表不被當表格、lint 也不報
- 行首的 `#` 一律被當標題（`#2 不做` → `--fix` 改成一級標題）→ 編號移到行中或用反引號
- 程式碼圍欄要標語言（`bash`、`text`、`yaml`）；blockquote 中間不留空行（續寫用 `>`）
- 有序清單中間夾的 code block 要縮排進清單項，否則編號從 1 重算；會被引用的編號寫成固定 ID（`P1-3`），不靠序號
- 🔴 **示範 markdownlint 的 inline 指令（HTML 註解）會真的生效** —— 連 code span、code fence 內都算 →
  引用時把角括號寫成 HTML entity（反引號救不了）。`disable-next-line` 的說明要放在指令**前面**；
  改完插一行必定違規的內容跑一次 lint，確認仍被報出

## 內容

- 不要複製「別的檔案已經是權威」的內容（規則、欄位值、筆數）—— 必然漂移，而且漂移後看起來仍然權威；只留指標與查現況的指令
- 跨 repo 引用一律用完整 GitHub URL —— 相對路徑假設對方 clone 在特定位置，不成立時就是斷鏈
