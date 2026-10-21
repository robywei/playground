# Agent 執行環境（下任何 Bash 指令前）

> 由 repo-template 管理（基礎層），要改請改範本；本 repo 專屬規則寫在 `.claude/custom/` 或 repo 自己的 CLAUDE.md

以下踩到時多半**輸出看起來正常**、或後半段靜默沒跑，只看有沒有報錯發現不了。
派工給子 agent 時，把 zsh 那幾條一起寫進 prompt（重犯者多半是子 agent）。

## Bash 工具跑的是 zsh，不是 bash

- **每次呼叫自帶 `cd <絕對路徑> &&`**，跨 repo 用 `git -C <路徑>` —— CWD 跨回合延續，相對路徑會安靜地掃錯地方
- **不做 word splitting**：`for f in $FILES`、`set -- $line` 把整串當成一個值、只跑一輪。批次迴圈包 `bash -c '…'`；
  含 `$(…)`、多層引號、`jq` 的寫進 scratchpad 檔再 `bash <檔>`；檔案清單用 `find … -print0 | xargs -0`
- 參數含 `*` `?` `[` `{` **一律單引號**（整個參數：`--include='*.md'`、帶 `?` 的 URL）—— glob 沒命中時
  那條指令**根本沒送出**，在 `for` 清單裡則中止整段；刻意的 glob 改用 `find`
- 🔴 **未加引號、以 `=` 開頭的字會中止整段**（`echo ===`、`[ "$a" == b ]`）→ 分隔線用 `echo ---`，相等比較用 `=` 或 `[[ ]]`
- 變數後緊接 `:` 或 `[` 寫成 `${var}`（`"$p:tag"` 的 `:t` 被當修飾符、`"$a[$i]"` 被當下標，都不報錯）；變數別取名 `path`、`UID`、`GID`
- **`$PIPESTATUS` 不存在**（印空字串、不報錯）；要量某指令的退出碼就**不要在它後面接 pipeline**：
  `cmd > <檔> 2>&1; echo "rc=$?"` 再看檔
- 以 `-` 開頭的路徑會被當成旗標 → 加 `./` 前綴或用 `--`

## 工具與改檔

- `grep`／`ls`／`find`／`sed` 可能被 wrapper、alias 或另一個版本取代 → 先 `type <指令>`，要確定行為就直呼 `/usr/bin/<指令>`
- 本機 `sed` 若是 GNU：`sed -i ''` 會把 script 當檔名（`can't read s/…`、一行都沒改）
- **改檔用 `scripts/tools/replace.py <plan.json>`**，不用 `sed -i` 或沒有斷言的 python：每個錨點斷言次數、
  多檔全部命中才寫（`--dry-run` 先看 diff）。⚠️ count 對了不代表每一處都該改；同一件事改多檔就放進同一份 plan
- 改檔與驗證用 `&&` 串、不用 `;` —— 改檔失敗時驗證照跑、照印成功
- 臨時腳本與 plan 用 Write 寫進 scratchpad 檔再執行，不用 heredoc 餵（內容裡出現結束標記就提早關閉，剩下的被當指令跑）
- `while read` 迴圈裡會讀 stdin 的指令（`ssh`、多數 CLI）補 `</dev/null`，否則吃掉剩下的輸入、只跑一輪
- 🔴 **窮盡式盤點不要用裸 `rg`**（預設跳過隱藏目錄與被 ignore 的檔，`.claude/`、`.github/` 整批漏掉）→
  `rg --hidden --no-ignore`；`grep -r` 若被包成 wrapper 也可能套 `.gitignore`
- `git grep` 預設只掃**已追蹤**的檔 → 盤點含新檔（還沒 `git add`）要加 `--untracked`，否則得到假的「沒有」
- `git ls-files` 會把非 ASCII 檔名轉義成 octal → 餵給別的指令時用 `git -c core.quotepath=false ls-files -z`
- **不要遮 stderr**（`2>/dev/null`）：權限錯誤與 `fatal` 會被讀成「零結果」或「成功」
- 多帳號時不要 `gh auth switch`（全域、影響同機其他 agent）→ 單次指令帶 `GH_TOKEN=…`；git 操作走 SSH

## 機密

- 取機密與使用它在**同一次呼叫**內完成，不要 `source ~/.zshrc`；本機的載入方式寫在本 repo 的 CLAUDE.md 或 custom
- 🔴 碰到機密的腳本**不要 `bash -x`／`set -x`**（xtrace 印出展開後的 token，進了 transcript 撤不回）
- 確認有沒有值只印布林：`[ -n "$TOKEN" ] && echo set`；`echo "${TOKEN:-MISSING}"` 在有值時印的就是值本身

## 產線與遠端

- 🔴 **產線資源的寫入一律停下來，把指令原文交回使用者**（不限哪支 CLI）。被擋時**不要拆開或改寫形式重試**
  —— 那是繞過，使用者已授權該動作也一樣；唯讀指令被擋才可以改寫得更單純再試
- 會改到遠端的動作（push、建 repo、改設定、API 寫入）先說明再執行；有外部副作用的那段單獨一次呼叫
- 「暫時改、之後還原」不是零成本（還原也是一次變更）；dry-run 只攔它自己那一段，之前的步驟已經發生
- 🔴 **工具呼叫被中斷／拒絕 ≠ 副作用沒發生**：`A && B && C` 常是前半段已執行 → 回讀對象的實際值；
  `git status` 乾淨也不算證據（可能已 commit，或被併行的 agent 一起提交走）
