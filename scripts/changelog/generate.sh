#!/bin/bash
##
## generate.sh — 用 git-cliff 重新生成本 repo 的 CHANGELOG.md
## ============================================================================
## 🔁 本檔由 repo-template 管理（基礎層）：要改請改範本、發版後 `copier update`，
##    不要直接改這裡 —— 直接改會被當成本地客製保留下來，從此和其他 repo 分岔。
##
## 🔑 生成範圍起點 = 歸檔截止月的下一個月 1 號；從未歸檔 → 完整歷史（第一筆 commit 起）。
##    有遷移前的手寫內容（歸檔檔裡的 slash 月標題）時，再夾到遷移日（`.cliff.toml` 進版控那天）以後：
##
##      從未歸檔、沒有手寫歷史 → 完整歷史（歸檔不是必要行為）
##      從未歸檔、有手寫歷史   → 遷移日起累積（遷移前的內容已在歸檔檔裡）
##      10/10 歸檔到 9 月      → 2026-10-01 起
##      11/05 歸檔到 10 月     → 2026-11-01 起
##
##    「什麼時候歸檔、要不要歸檔」完全自由,本腳本對三種狀態都給出正確結果。
##    覺得主檔太長就歸檔一次（`/git:changelog-monthly`）,主檔下次生成立刻縮短 ——
##    **歸檔是「修剪」動作,不是排程義務。**
##
## 本腳本**只生成,不 commit / 不 push**。呼叫者決定要不要提交：
##   - 人手動跑 → 自己 commit（獨立一筆 `chore(changelog)`,見 /git:changelog）
##   - 排程跑   → 由排程以 bot 身分 commit + push（若 repo 有設定每日排程）
##
## 用法:
##   ./scripts/changelog/generate.sh              # 生成（工作區留下改動）
##   ./scripts/changelog/generate.sh --dry-run    # 只顯示差異,結束後還原
##
## ! 刻意不開 set -e：多處以非零回傳值做流程判斷（git diff --quiet 等）,
##   開了會在第一個「正常的非零」就中止。改為明確檢查回傳值並 exit。
##
## 🔴 **shebang 必須是 bash,不要改成 zsh。** 算歸檔截止月時用的 glob
##   `docs/history/archive/*/CHANGELOG.md` 在「完全沒有歸檔檔案」時匹配不到 ——
##   bash 會把它原樣傳給 grep（找不到檔、stderr 已丟棄、CUTOFF 為空 → 正確退回完整歷史或遷移日）,
##   而 **zsh 會直接中止整支腳本**（`no matches found`）。2026-08-28 實測六種邊界
##   （沒有 docs/、archive 空的、年目錄沒檔、只有 slash、跨年多檔、檔案空白）
##   在 bash 下全部正確、rc=0。
##

set -uo pipefail

BASEDIR=$(cd "$(dirname "$0")" && pwd)
cd "${BASEDIR}/../.." || exit 1

## ---- 顏色（各 repo 的 color helper 位置不同,兩個都試；缺了就不上色）----
if [ -f common/color.sh ]; then
  # shellcheck source=/dev/null
  source common/color.sh
elif [ -f scripts/lib/color.sh ]; then
  # shellcheck source=/dev/null
  source scripts/lib/color.sh
else
  red='' green='' yellow='' cyan='' reset=''
fi

msg() { printf "%b%-8s %s\n%b" "$2" "$1" "$3" "${reset}"; }

DRY_RUN=false
case "${1:-}" in
  "")        ;;
  --dry-run) DRY_RUN=true ;;
  *)         msg "[ERROR]" "${red}" "無法辨識的參數: $1（用法: $0 [--dry-run]）"; exit 1 ;;
esac
if [ "$#" -gt 1 ]; then
  msg "[ERROR]" "${red}" "多餘的參數: ${*:2}"
  exit 1
fi

## ---- 前置 -------------------------------------------------------------------
if ! command -v git-cliff > /dev/null 2>&1; then
  msg "[ERROR]" "${red}" "找不到 git-cliff —— brew install git-cliff"
  exit 1
fi
if ! git ls-files --error-unmatch .cliff.toml > /dev/null 2>&1; then
  msg "[ERROR]" "${red}" '.cliff.toml 不在 git 追蹤中 —— 本 repo 還沒遷到 git-cliff'
  exit 1
fi
## 🔴 淺 clone 下「-o 全量覆寫」會把整份 CHANGELOG 換成只剩一筆,且沒有錯誤訊息
if [ "$(git rev-parse --is-shallow-repository)" = "true" ]; then
  msg "[ERROR]" "${red}" "這是淺 clone,生成會毀掉整份 CHANGELOG —— 先 git fetch --unshallow"
  exit 1
fi

## ---- 時區 -------------------------------------------------------------------
## 範圍邊界（下面的 `--before "<日期> 00:00"`）由 git 依 TZ 解讀，`.cliff.toml` 的日期分段也讀同一個 TZ ——
## 兩邊用同一個時區，月初、遷移日凌晨的 commit 才不會一邊算進、一邊算出。沒設 TZ 時用 Asia/Taipei（與 `.cliff.toml` 的預設相同）。
export TZ="${TZ:-Asia/Taipei}"

## ---- 算生成範圍 -------------------------------------------------------------
## 遷移日：`.cliff.toml` 的新增 commit 日期。**不寫死常數** —— 各 repo 遷移日不同,
## 寫死就得逐 repo 維護,而漏改的症狀是「範圍含遷移前的 commit」（那段已在歸檔檔裡）。
EPOCH=$(git log --diff-filter=A --format=%ad --date=short -1 -- .cliff.toml)
if [ -z "$EPOCH" ]; then
  msg "[ERROR]" "${red}" "讀不到 .cliff.toml 的新增日 —— 不生成,避免算出錯誤範圍"
  exit 1
fi

## 歸檔截止月＝歸檔檔裡**最新**的已歸檔月份。
## 🔴 只認 dash（`## 📆 2026-09`）,刻意**不認** slash（`## 📆 2026/09`）：
##    dash  ＝ git-cliff 生成的區塊,也就是「這個月被歸檔過」的唯一證據。
##    slash ＝ 手寫時代的內容,遷移當天整批搬入,與生成範圍無關。
##    認了 slash 的後果：四個 repo 的歸檔檔都已有 slash 的 `2026/08`（遷移搬入的）
##    → 誤判「8 月已歸檔」→ 起點跳到 9 月 → **遷移日之後的條目一筆都不生成,
##    而且沒有任何錯誤訊息**。
## ⚠️ 判斷依據是歸檔檔的**內容**,不是歸檔 commit 的時間 —— 後者會被 rebase 打亂,
##    而內容裡的月標題「歸檔到哪就是哪」,自帶答案。
CUTOFF=$(grep -hoE '^## 📆 [0-9]{4}-[0-9]{2}' docs/history/archive/*/CHANGELOG.md 2> /dev/null \
  | sed 's/^## 📆 //' | sort | tail -1)

## 手寫時代的證據＝歸檔檔裡的 slash 月標題（遷移當天整批搬入）。有它才夾到遷移日（見下方）。
## 🔴 沒有手寫歷史的 repo 不可以夾：從零導入時遷移日＝導入當天，主檔會只剩那一天、之前的 commit 全部不見，
##    而且不報錯（2026-10-28 實測：完整歷史 54 筆的 repo 只生成當天 5 筆；當天才建立的 repo 直接算不出起點）。
HANDWRITTEN=$(grep -lE '^## 📆 [0-9]{4}/[0-9]{2}' docs/history/archive/*/CHANGELOG.md 2> /dev/null)

SINCE=""
if [ -n "$CUTOFF" ]; then
  ## 月份加一：純整數運算,不用 `date -v+1m` —— 後者的 `-v` 必須在 `-f` 之前,
  ## 放在 operand 之後會被當普通參數 → 輸出預設格式、月份沒加,**而且不報錯**。
  CY=${CUTOFF%-*}
  CM=${CUTOFF#*-}
  CM=${CM#0}                                    ## 去前導 0,避免被當八進位
  TM=$((CY * 12 + CM))                          ## CM 已是 1-based → 直接加一個月
  SINCE=$(printf '%04d-%02d-01' $((TM / 12)) $((TM % 12 + 1)))
fi
## 有手寫歷史就夾到遷移日：遷移日之前的 commit 已隨手寫內容整批進歸檔檔,
## 不夾的話會把它們重新生成回主檔,與歸檔檔重複（實測會重疊 15 個月）。
if [ -n "$HANDWRITTEN" ] && { [ -z "$SINCE" ] || [[ "$SINCE" < "$EPOCH" ]]; }; then
  SINCE="$EPOCH"
fi

## 起點＝SINCE 之前的最後一筆 commit。SINCE 為空（完整歷史）、或 SINCE 之前沒有任何 commit
## （repo 比 SINCE 新，所有 commit 都在範圍內）→ BASE 留空＝生成完整歷史。
## ⚠️ `--before` 一定要帶 `00:00`：純日期會被 git 的 approxidate 解析成「當天當下」,
##    回傳的是 HEAD 自己 → range 變空 → 生成出只有檔頭的空 CHANGELOG,無錯誤訊息。
BASE=""
if [ -n "$SINCE" ]; then
  BASE=$(git rev-list -1 --before="${SINCE} 00:00" HEAD)
fi
if [ -n "$BASE" ]; then
  COUNT=$(git rev-list --count "${BASE}..HEAD")
  msg "[INFO]" "${cyan}" "範圍 ${SINCE} 起（${COUNT} 筆 commit${CUTOFF:+，已歸檔至 ${CUTOFF}}）"
else
  COUNT=$(git rev-list --count HEAD)
  msg "[INFO]" "${cyan}" "範圍：完整歷史（${COUNT} 筆 commit）"
fi
if [ "$COUNT" -eq 0 ]; then
  msg "[INFO]" "${cyan}" "該範圍沒有任何 commit,不生成"
  exit 0
fi

## 生成前先講清楚：CHANGELOG.md 已是 dirty 的話,下面的覆寫與（dry-run 時的）
## `git checkout --` 會蓋掉那份未提交的內容。它是生成物、隨時可重建,所以不中止,
## 但要留痕 —— 否則「我剛跑過 git-cliff 沒提交」的人會找不到原因。
## 🔴 CHANGELOG.md 還沒進版控（第一次生成）時，`git diff --quiet` 一律回「沒變更」、`git checkout --` 也還原不了 ——
##    沒分開處理時，第一次生成被判成「已是最新」直接結束，連 lint 閘門都沒跑（2026-10-28 實測）。
##    → 未追蹤時一律當成有變更，還原改用生成前的備份（原本沒有這個檔就刪掉）。
TRACKED=false
if git ls-files --error-unmatch CHANGELOG.md > /dev/null 2>&1; then
  TRACKED=true
fi
BACKUP=""
trap '[ -z "${BACKUP}" ] || rm -f "${BACKUP}"' EXIT
if [ "$TRACKED" = true ]; then
  if ! git diff --quiet -- CHANGELOG.md; then
    msg "[WARN]" "${yellow}" "CHANGELOG.md 本來就有未提交的變更,將被本次生成覆寫"
  fi
elif [ -f CHANGELOG.md ]; then
  msg "[WARN]" "${yellow}" "CHANGELOG.md 還沒進版控,將被本次生成覆寫（失敗或 dry-run 時放回原本的內容）"
  BACKUP=$(mktemp) || exit 1
  cp CHANGELOG.md "$BACKUP" || exit 1
fi

restore_changelog() {
  if [ "$TRACKED" = true ]; then
    git checkout -- CHANGELOG.md
  elif [ -n "$BACKUP" ]; then
    cp "$BACKUP" CHANGELOG.md
  else
    rm -f CHANGELOG.md
  fi
}

## ---- 生成 -------------------------------------------------------------------
if [ -n "$BASE" ]; then
  OUT=$(git-cliff "${BASE}..HEAD" -o CHANGELOG.md 2>&1)
else
  OUT=$(git-cliff -o CHANGELOG.md 2>&1)
fi
RC=$?
if [ "$RC" -ne 0 ]; then
  msg "[ERROR]" "${red}" "git-cliff 失敗 (rc=${RC}): $(printf '%s' "$OUT" | tail -1)"
  restore_changelog 2> /dev/null
  exit 1
fi

## 🔴 「只剩檔頭的空檔」是這條路上最陰的失敗：退出碼 0、檔案存在、沒有警告。
##    條目數當場驗一次,0 筆就還原（範圍算錯時唯一攔得住的地方）。
ENTRIES=$(grep -c '^- ' CHANGELOG.md)
if [ "$ENTRIES" -eq 0 ]; then
  msg "[ERROR]" "${red}" "生成 0 個條目（範圍 ${COUNT} 筆 commit,預期 ≥1）—— 已還原並中止"
  restore_changelog
  exit 1
fi

if [ "$TRACKED" = true ] && git diff --quiet -- CHANGELOG.md; then
  msg "[DONE]" "${green}" "CHANGELOG.md 已是最新（${ENTRIES} 個條目）—— 無變更"
  exit 0
fi

## ---- 閘門 -------------------------------------------------------------------
## ⚠️ 失敗時**不還原** —— 失敗項可能是別人改的檔（本檢查掃全量 md）,
##    還原掉自己這份正確的生成物只會讓人更難排錯。
if ! ./scripts/lint/lint.sh md; then
  msg "[ERROR]" "${red}" "lint.sh md 未通過 —— 生成物留在工作區,先修再提交"
  exit 1
fi

if [ "$DRY_RUN" = true ]; then
  msg "[INFO]" "${cyan}" "--dry-run：以下是會被提交的差異,結束後還原工作區"
  if [ "$TRACKED" = true ]; then
    git --no-pager diff --stat -- CHANGELOG.md
  else
    msg "[INFO]" "${cyan}" "CHANGELOG.md 是新檔（$(wc -l < CHANGELOG.md | tr -d ' ') 行）"
  fi
  restore_changelog
  msg "[DONE]" "${green}" "dry-run 完成（${ENTRIES} 個條目）,工作區已還原"
  exit 0
fi

msg "[DONE]" "${green}" "已生成（${ENTRIES} 個條目）—— 未提交,請開獨立一筆 chore(changelog)"
exit 0
