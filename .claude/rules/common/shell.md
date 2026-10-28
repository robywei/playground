---
paths: ["**/*.sh", "**/githooks/*"]
---

# Shell 腳本（讀寫 `.sh`、githooks 時）

> 由 repo-template 管理（基礎層），要改請改範本；本 repo 專屬規則寫在 repo 自己的 CLAUDE.md

腳本是 bash 語意，Bash 工具卻跑 zsh → **手測一律 `bash <檔>`**；macOS 的 `/bin/bash` 是 3.2，
和 PATH 上的新版 bash 行為不同，兩個都可能是 shebang 實際跑到的那一支。

## 寫第一行就要對的

- 用到陣列、`[[ ]]`、`local`、`${BASH_SOURCE[0]}` 就寫 `#!/usr/bin/env bash`，不寫 `#!/bin/sh`
- shellcheck directive 一律**單井號** `# shellcheck disable=…`（雙井號會被當普通註解、靜默失效）；
  `disable` 只管下一個指令，錯誤在遠處觸發（例：`source` 造成的 SC2154）就放在 shebang 下方的檔案層級
- 變數後緊接中文或全形標點一律寫 `${VAR}`：bash 3.2 會把全形字吃進變數名 → `set -u` 下 unbound，
  而 zsh 與新版 bash 正常，手測複現不出來
- 錯誤訊息、說明文字裡別沿用 markdown 的反引號 —— 在雙引號內會被當成命令替換**真的執行** → 純文字用單引號
- 含 `\n` 的 JSON 交給下一個指令用 `printf '%s\n'`，不用 `echo`；從 JSON 取欄位一律 `jq`，不要 `grep` 排版
- source 或呼叫同 repo 的檔用絕對路徑：`SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`，不要裸 `cd foo`
- 暫存、快取、log 放 `${TMPDIR:-/tmp}` 或 `mktemp`；共用 `/tmp` 底下名稱固定的檔**不可 `source`／`eval`**（同機別的使用者能先放好內容）
  → 狀態檔用 `sed -E` 只取格式正確的值，快取目錄用 `umask 077` 建、`[ -O <目錄> ]` 確認是自己的

## `set -euo pipefail` 的陷阱

- `x=$(cmd)` 失敗會在賦值那行**直接結束**，下一行的錯誤處理永遠不跑、什麼都不印 → `x=$(cmd) || rc=$?`
- 可能未設定的變數寫 `${VAR:-}`；要能表達「刻意傳空」用 `${N-預設}`（`${N:-預設}` 會把空值也換掉）
- `local a="$1" b="$a/x"` 會 unbound（同一行的 `a` 還沒賦值）→ 拆成兩句
- 空陣列展開在 bash 3.2 就是 unbound → 「可選旗標」不要用陣列，兩條路徑各自寫完整
- `echo "$big" | grep -q p` 在 `pipefail` 下會把「命中」判成失敗（`grep -q` 提早關管線）→ `grep -c` 或 `[[ $big == *p* ]]`
- 管線上游失敗時，下游已經把「空輸入」處理掉了（`pipefail` 的 rc 是事後的）→ 先落檔、驗過再交給下游

## 誠實回報

- 「有問題」一定要改變**退出碼**（印警告沒人看）；迴圈一筆都沒跑到要當失敗，不要印「0 筆異常」
- 背景 job 收 pid 逐一 `wait`（裸 `wait` 只回最後一個）
- 帶值旗標先驗 `$# -ge 2`（`shift 2 || true` 會讓迴圈空轉不結束）；不認得的參數 fail-loud，不要靜默忽略
- 冪等檢查要比對內容，不能只看「存在」；log 檔名要帶上每一個會變的維度（否則重跑互相覆蓋）
- 機密不要當位置參數傳（`ps` 看得到、`set -x` 會印）

## 可攜性（GNU／BSD 都要能跑）

- 腳本內的 `sed` 限縮在共通子集：`sed -i.bak '…' f && rm f.bak`、`[[:space:]]` 不寫 `\s`；多行插入改用 `awk`／`printf`
- `sed 's#…#…#'` 的取代內容不能含 `#`（GNU 靜默改壞、exit 0）
- BSD `date -v` 要放在 `-f` 與 operand 之前，否則被當普通參數、照樣 exit 0
- 間接展開用 `${!VAR}`，不用 `eval`；`${var/pat/repl}` 裡未跳脫的 `}` 會提早結束展開（regex 量詞 `{8}` 首當其衝）
