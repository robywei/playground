# Markdown 撰寫與檢查

寫或改本 repo 任何 `.md` 之前讀這份。

規則的 source of truth 是 [`.markdownlint-cli2.jsonc`](../../.markdownlint-cli2.jsonc)
—— **刻意只有這一份**。`globs`（檢查範圍）、`ignores`（排除）、`config`（規則）
全在裡面，不另外放 `.markdownlint.jsonc` 或 `.markdownlintignore`。

## 🔴 一律走 `./scripts/lint.sh md`，不要裸跑

```sh
./scripts/lint.sh md
```

裸跑會踩到下面兩類靜默失敗。腳本的存在就是為了消掉它們：它一律 `cd` 到 repo 根，
並檢查「真的有檔案被檢查過」。

## 🔴 兩種假綠燈：exit 0 不等於檢查過

2026-09-03 在本 repo 實測，兩支工具**都會 exit 0**：

| 指令 | 輸出 | exit |
| --- | --- | --- |
| `markdownlint <不存在的檔>` | 整份 usage 說明 | **0** |
| `markdownlint-cli2 <glob 沒命中>` | `Linting: 0 files` | **0** |

於是 `markdownlint <檔> && echo "md OK"` **會印出 `md OK`**。

兩者的可辨識訊號不同：cli2 至少印得出 `Linting: N files` 可以比對；cli v1 印的是
usage，**沒有任何可比對的字串**。這是選 cli2 的其中一個理由。

⚠️ 另外注意 `Summary: 0 issues in 0 files` 的 `0 files` 是「**0 個有問題的**檔案」，
那是成功輸出。它與 `Linting: 0 files` 長得像但意思相反：後者代表**沒檢查到東西**。

## 🔴 設定檔按 cwd 往上找 —— 在子目錄跑會靜默改用預設值

2026-09-03 實測，從 `babywei-bakery/` 執行：

```sh
markdownlint-cli2 'docs/**/*.md'   ## → 19 筆 MD013
```

而 `MD013` 在設定檔裡是 `false`。原因是該子目錄下沒有設定檔，工具往上找的起點是
cwd 而非 repo 根，於是**靜默改用預設值**，且兩邊都不報「設定沒載入」。

cli v1 有同樣的問題。從 repo 根跑則正常通過。

⚠️ cli2 在子目錄**不帶參數**跑更糟：只印使用說明，**一個檔案都沒檢查**。

## ⚠️ `--fix` 會靜默改壞內容，跑完必須 `git diff`

已知兩類（皆不會報錯）：

- **MD037**：把標題裡的 `*.a.xyz / *.b.xyz` 當成 emphasis 標記、吃掉中間空格變成
  `*.a.xyz /*.b.xyz` —— 語意當場錯掉。**glob 與通配符先用反引號包起來**再讓它跑
- **有序清單編號**會被重新編號（原本刻意的 `4.` `5.` 變成 `1.` `2.`）

## ⚠️ 表格單元格內的 `|` 一律寫 `\|`

例：`` `cmd a\|b\|c` ``。不轉義會被當欄位分隔，撐爆欄數並觸發 MD055 / MD056
連鎖誤報，而且**表格在渲染時直接壞掉**。**連在 code span 裡也一樣要轉義。**

同源陷阱：**在表格列尾追加文字**很容易多出一欄。改表格後看有沒有報 MD055
（缺結尾 pipe）/ MD056（欄數不符）—— 它們通常成對出現，指的是同一個錯。

## 常踩的規則

| 規則 | 內容 |
| --- | --- |
| MD013 | **已停用** —— 中文散文採軟換行，一個段落一行，不手動斷行（理由見設定檔註解） |
| MD022 | 標題前後需空行 |
| MD031 | 程式碼圍欄前後需空行 |
| MD032 | 清單前後需空行 |
| MD040 | 程式碼圍欄須標語言（`sh` / `text` / `sql` / `js` / `http`；目錄樹與公式用 `text`） |
| MD036 | 不可用粗體代替標題 —— `**(a) 標題**` 要寫成真正的 `#### (a) 標題` |
| MD009 | 不可有行尾空白 |
| MD018 | **行首的 `#` 一律被當標題** —— 寫「`#2` 不做」這種編號時移到行中或用 `` ` `` 包住 |
| MD028 | blockquote 中間不可有空行 —— 續寫同一段引言要用 `>`（空的 `>` 也算） |

## 版本一致性

編輯器與閘門跑的是**同一個引擎**，因此即時提示與 commit 前檢查結果一致：

| 來源 | 版本（2026-09-03 實測） |
| --- | --- |
| VS Code `DavidAnson.vscode-markdownlint` 0.62.1 | dependencies: `markdownlint-cli2` 0.23.2 |
| 本機 `markdownlint-cli2` | v0.23.2（markdownlint v0.41.1） |

⚠️ 機器上另有 `markdownlint`（cli v1，v0.47.0）—— **不同的 binary、markdownlint 版本
差 6 個 minor**。它有 cli2 沒有的規則（如 `MD060`），所以同一批檔案可能「裸跑一片紅、
閘門全綠」。閘門的權威是 `./scripts/lint.sh md`。
