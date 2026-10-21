# shellcheck shell=bash
##
## lib.sh — lint 共用函式庫：共同介面＋基礎檢查（md、shell、trivy、conflict）
## ============================================================================
## 🔁 本檔由 repo-template 管理（基礎層）：要改請改範本、發版後 `copier update`，不要直接改這裡。
##    本 repo 專屬的檢查寫在 scripts/lint/lint.sh（repo 擁有）。
##
## 用法（scripts/lint/lint.sh）：
##   source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/lib.sh" || exit 1
##   REPO_CHECKS="render neutral"                 # 本 repo 專屬檢查項；check_<名>() 由 lint.sh 定義（名字的 - 換成 _）
##   REPO_TOOLS=("copier|pipx install copier")    # 選用：--doctor 要多列的工具（指令|安裝方式）
##   check_render() { …; }
##   lint_main "$@"
##
## 介面：lint.sh [檢查項…] [--fix] [--list] [--doctor] [--help]
##   不帶檢查項（或 all）＝完整檢查：基礎項＋本 repo 的項目；pre-push 跑的就是這個
##   --strict 為相容舊呼叫而接受，沒有作用：缺工具一律失敗
## 退出碼：0 通過（含使用者指定跳過的項目，總結會列出）／1 有項目未通過或跑不起來／2 用法錯誤
##
## 跳過：SKIP_CHECKS=md,shell <指令> —— 只對「跑不起來」（工具沒裝、執行失敗）生效。
##   工具在、有發現時無效：接受風險要寫進版控的逐筆例外（scripts/trivy/.trivyignore.yaml、
##   <!-- markdownlint-disable … -->、# shellcheck disable=…）
##
## lint.sh 可在 lint_main 之前設定：
##   SHELLCHECK_SEVERITY   error（預設）或 warning；SC3xxx（shebang 與語法不符）不論設定一律擋
##   SHELLCHECK_EXCLUDE    交給 shellcheck -e 的代碼，逗號分隔
##   md_extra()            基礎 markdownlint 之後執行的本 repo 子段（例如斷鏈檢查）；
##                         markdownlint 沒裝也照跑，失敗時 record_fail md
## 給 check_<名>() 用：msg、header、record_fail、record_warn、need_tool、cannot_run、list_files、list_shell_files、$FIX
## 以上與 check_md 等基礎檢查都是唯讀函式：lint.sh 不可定義同名函式（會報錯並保留函式庫的版本）
##
## 從舊版 lint.sh 遷移（各自實作 md／shell／trivy 的 repo）：改成 source 本檔、刪掉同名函式與基礎檢查，
## 舊的 tool_missing／record_skip（缺工具＝跳過）改成 need_tool／cannot_run（缺工具＝失敗）
##
## ⚠️ 相容 bash 3.2（macOS /bin/bash）：不用關聯陣列、mapfile、${var,,}
## ⚠️ lint.sh 可以開 set -u；set -e 在 lint_main 裡會被關掉（檢查自己判斷退出碼），check_<名>() 裡要自己處理錯誤
##

LINT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${LINT_DIR}/../.." && pwd)"
cd "${REPO_ROOT}" || exit 1

## 顏色自給自足；reset 用 ESC[0m（tput sgr0／init 會多吐控制序列，進 log 變成字面的 `(B`）
if [ -t 1 ]; then
  red=$'\033[31m' green=$'\033[32m' yellow=$'\033[33m' cyan=$'\033[36m' reset=$'\033[0m'
else
  red='' green='' yellow='' cyan='' reset=''
fi

## readonly：lint.sh 改寫它時只會報錯、基礎項照跑（pre-push 依它核對基礎項目確實有執行）
readonly BASE_CHECKS="md shell trivy conflict"
REPO_CHECKS="${REPO_CHECKS:-}"
FIX=false
FAILED=""
SKIPPED=""
WARNED=""
CURRENT=""

msg() { printf '%s%-7s%s %s\n' "$2" "$1" "${reset}" "$3"; }
header() { printf '\n%s## ---------- %s ----------%s\n' "${cyan}" "$1" "${reset}"; }
in_list() { case " $2 " in *" $1 "*) return 0 ;; esac; return 1; }
record_fail() { in_list "$1" "${FAILED}" || FAILED="${FAILED} $1"; }
record_warn() { in_list "$1" "${WARNED}" || WARNED="${WARNED} $1"; }
skip_requested() { case ",${SKIP_CHECKS:-}," in *",$1,"*) return 0 ;; esac; return 1; }

## cannot_run <原因> [提示]：目前的檢查項跑不起來。使用者有指定跳過才記成「未檢查」，否則失敗
cannot_run() {
  if skip_requested "${CURRENT}"; then
    msg "[SKIP]" "${yellow}" "${CURRENT} 未檢查（使用者指定跳過）：$1"
    in_list "${CURRENT}" "${SKIPPED}" || SKIPPED="${SKIPPED} ${CURRENT}"
  else
    msg "[FAIL]" "${red}" "$1${2:+ —— $2}"
    msg "" "" "確定先不處理：SKIP_CHECKS=${CURRENT} <原指令>（只對跑不起來有效，並在回報寫明沒檢查）"
    record_fail "${CURRENT}"
  fi
}

## need_tool <指令> <安裝方式>：有裝回 0；沒裝走 cannot_run 回 1
need_tool() {
  command -v "$1" > /dev/null && return 0
  cannot_run "找不到 $1" "安裝：$2"
  return 1
}

## list_files [pathspec…]：已追蹤＋未追蹤但未被 ignore 的檔，一行一個；工作區已刪除（還沒 commit）的不列
## 讀 git 用 -z（不轉義任何字元：非 ASCII、引號、反斜線、tab），輸出才換成一行一個 —— 只有檔名含換行會出錯
## 用法：while IFS= read -r f; do …; done < <(list_files '*.md')
list_files() {
  local f
  git ls-files -z --cached --others --exclude-standard -- "$@" |
    while IFS= read -r -d '' f; do [ -f "$f" ] && printf '%s\n' "$f"; done
}

## list_shell_files：*.sh、*.bash，以及沒有副檔名但 shebang 是 sh／bash／dash／ksh 的檔（例如 githooks），一行一個
list_shell_files() {
  local f line re='^#!.*[/ ](ba|da|k)?sh([[:space:]]|$)'
  list_files | while IFS= read -r f; do
    case "${f##*/}" in
      *.sh | *.bash) printf '%s\n' "$f" ;;
      *.*) ;;
      *) line="" && IFS= read -r line < "$f" && [[ "$line" =~ $re ]] && printf '%s\n' "$f" ;;
    esac
  done
}

## ---------------------------------------------------------------------------
## 基礎檢查
## ---------------------------------------------------------------------------

check_md() {
  header "md"
  if need_tool markdownlint-cli2 "npm i -g markdownlint-cli2"; then
    local files=() f out rc n
    ## 每個路徑加 : 前綴＝字面路徑：檔名裡的 [ ( ! 不會被當成 glob（否則靜默少掃；-- 無效，實測）
    while IFS= read -r f; do files+=(":$f"); done < <(list_files '*.md')
    if [ "${#files[@]}" -eq 0 ]; then
      msg "[FAIL]" "${red}" "沒有找到任何 .md —— 檢查不到東西不算通過"
      record_fail md
    else
      [ "${FIX}" = true ] && markdownlint-cli2 --fix "${files[@]}" > /dev/null 2>&1
      out=$(mktemp)
      markdownlint-cli2 "${files[@]}" > "${out}" 2>&1
      rc=$?
      n=$(sed -n 's/^Linting: \([0-9][0-9]*\) file.*/\1/p' "${out}")
      if [ "${rc}" -eq 0 ] && [ "${n:-0}" -eq "${#files[@]}" ]; then
        msg "[ OK ]" "${green}" "markdownlint：${n} 檔"
      elif [ "${rc}" -eq 1 ]; then
        command -p grep -E ' error ' "${out}"
        msg "[FAIL]" "${red}" "markdownlint：${#files[@]} 檔中有違規（格式類可試 lint.sh md --fix，跑完看 git diff）"
        record_fail md
      elif [ "${rc}" -eq 0 ]; then
        msg "[FAIL]" "${red}" "markdownlint 說掃了 ${n:-0} 檔，交給它 ${#files[@]} 檔 —— 對不上就不算通過"
        record_fail md
      else
        tail -5 "${out}"
        cannot_run "markdownlint-cli2 執行失敗（rc=${rc}）"
      fi
      rm -f "${out}"
    fi
  fi
  if declare -F md_extra > /dev/null; then md_extra; fi
}

check_shell() {
  header "shell"
  need_tool shellcheck "brew install shellcheck" || return 0
  local files=() f out err rc pat
  while IFS= read -r f; do files+=("$f"); done < <(list_shell_files)
  if [ "${#files[@]}" -eq 0 ]; then
    msg "[FAIL]" "${red}" "沒有找到任何 shell 腳本（至少有 scripts/githooks/pre-push）—— 檔案選取壞了"
    record_fail shell
    return 0
  fi
  case "${SHELLCHECK_SEVERITY:-error}" in
    error) pat=' error: ' ;;
    warning) pat=' (error|warning): ' ;;
    *) cannot_run "SHELLCHECK_SEVERITY 只能是 error 或 warning（現在是 ${SHELLCHECK_SEVERITY}）"; return 0 ;;
  esac
  out=$(mktemp) err=$(mktemp)
  ## 全部等級都要 shellcheck 輸出，再由這裡挑：SC3xxx 屬 warning，-S error 會把它濾掉
  shellcheck -f gcc ${SHELLCHECK_EXCLUDE:+-e "${SHELLCHECK_EXCLUDE}"} "${files[@]}" > "${out}" 2> "${err}"
  rc=$?
  if [ "${rc}" -gt 1 ]; then
    cat "${err}"
    cannot_run "shellcheck 執行失敗（rc=${rc}）"
  else
    ## grep 自己失敗（rc≥2）不能被當成「沒有命中」—— 那會把 shellcheck 抓到的 error 吞成 OK
    LC_ALL=C command -p grep -E "${pat}|\[SC3[0-9]{3}\]" "${out}"
    case $? in
      0)
        msg "[FAIL]" "${red}" "shellcheck（${SHELLCHECK_SEVERITY:-error} 級＋SC3xxx）：${#files[@]} 個腳本中有問題"
        record_fail shell
        ;;
      1) msg "[ OK ]" "${green}" "shellcheck（${SHELLCHECK_SEVERITY:-error} 級＋SC3xxx）：${#files[@]} 個腳本" ;;
      *) cannot_run "篩選 shellcheck 結果的 grep 執行失敗" ;;
    esac
  fi
  rm -f "${out}" "${err}"
}

check_trivy() {
  header "trivy"
  need_tool trivy "brew install trivy" || return 0
  local rc
  QUIET=true "${REPO_ROOT}/scripts/trivy/trivy-cli.sh" repo "${REPO_ROOT}"
  rc=$?
  case "${rc}" in
    0) msg "[ OK ]" "${green}" "trivy：沒有發現" ;;
    1)
      msg "[FAIL]" "${red}" "trivy 有發現：修正，或逐筆寫進 scripts/trivy/.trivyignore.yaml（statement、expired_at）"
      record_fail trivy
      ;;
    *) cannot_run "trivy 無法執行（rc=${rc}，見上方輸出：漏洞庫下載、cache lock…）" ;;
  esac
}

## 衝突標記：copier update 的衝突 rc 仍是 0，標記擋不住 git commit -a
## 只認 < 與 > 七個字元加空白或行尾（======= 單獨一行在一般文件有合法用途，x 實測寬鬆樣式有誤報）
check_conflict() {
  header "conflict"
  local out rc
  out=$(mktemp)
  git -c core.quotepath=false grep --untracked -n -I -E '^(<{7}|>{7})( |$)' > "${out}" 2>&1
  rc=$?
  case "${rc}" in
    1) msg "[ OK ]" "${green}" "沒有衝突標記" ;;
    0)
      head -20 "${out}"
      msg "[FAIL]" "${red}" "有殘留的衝突標記（合併後再 commit）"
      record_fail conflict
      ;;
    *)
      cat "${out}"
      cannot_run "git grep 執行失敗（rc=${rc}）"
      ;;
  esac
  rm -f "${out}"
}

## ---------------------------------------------------------------------------
## 主流程
## ---------------------------------------------------------------------------

lint_usage() {
  printf '用法: %s [檢查項…] [--fix] [--list] [--doctor]\n' "scripts/lint/lint.sh"
  printf '  檢查項：%s\n' "${BASE_CHECKS} ${REPO_CHECKS}"
  printf '  不帶檢查項＝完整檢查；跳過跑不起來的項目：SKIP_CHECKS=<項目,…>（說明見 scripts/lint/lib.sh 檔頭）\n'
}

## --doctor：列出必要工具與版本（install-hooks.sh 也呼叫它）；缺任何一個回 1
lint_doctor() {
  local entry cmd hint missing=0 ver
  for entry in "git|https://git-scm.com/" "markdownlint-cli2|npm i -g markdownlint-cli2" \
    "shellcheck|brew install shellcheck" "trivy|brew install trivy" ${REPO_TOOLS[@]+"${REPO_TOOLS[@]}"}; do
    [ -n "${entry}" ] || continue
    cmd="${entry%%|*}" hint="${entry#*|}"
    if command -v "${cmd}" > /dev/null; then
      ver=$("${cmd}" --version 2>&1 | head -1)
      msg "[ OK ]" "${green}" "${cmd}  ${ver}"
    else
      msg "[MISS]" "${red}" "${cmd}  安裝：${hint}"
      missing=1
    fi
  done
  return "${missing}"
}

lint_main() {
  ## 檢查一律自己看退出碼（grep 沒命中＝1 是正常結果）；lint.sh 開了 set -e 時這些都會讓整支中止 → 關掉
  set +e
  local all arg checks="" c fn skips
  all="${BASE_CHECKS}${REPO_CHECKS:+ ${REPO_CHECKS}}"
  for arg in "$@"; do
    case "${arg}" in
      --fix) FIX=true ;;
      --strict | all) ;;
      --list) echo "${all}" && exit 0 ;;
      --doctor) lint_doctor; exit $? ;;
      -h | --help) lint_usage && exit 0 ;;
      -*) msg "[ERROR]" "${red}" "未知選項：${arg}" && lint_usage >&2 && exit 2 ;;
      *)
        in_list "${arg}" "${all}" || { msg "[ERROR]" "${red}" "未知檢查項：${arg}（可用：${all}）" && exit 2; }
        checks="${checks} ${arg}"
        ;;
    esac
  done
  [ -n "${checks}" ] || checks="${all}"
  for c in ${all}; do
    fn="check_${c//-/_}"
    declare -F "${fn}" > /dev/null || { msg "[ERROR]" "${red}" "${c} 沒有定義 ${fn}()" && exit 2; }
  done
  skips="${SKIP_CHECKS:-}"   # :- 不可省：lint.sh 開 set -u 時 SKIP_CHECKS 常是未設定
  skips="${skips//,/ }"
  for c in ${skips}; do
    in_list "${c}" "${all}" || msg "[WARN]" "${yellow}" "SKIP_CHECKS 裡的 ${c} 不是檢查項（可用：${all}）"
  done

  ## pre-push 以 LINT_REPORT 核對基礎項目確實有執行（lint.sh 沒走 lint_main 時報告是空的 → 擋）
  [ -n "${LINT_REPORT:-}" ] && for c in ${BASE_CHECKS}; do echo "base ${c}"; done > "${LINT_REPORT}"
  for c in ${checks}; do
    CURRENT="${c}"
    "check_${c//-/_}"
    [ -n "${LINT_REPORT:-}" ] && echo "ran ${c}" >> "${LINT_REPORT}"
  done
  CURRENT=""

  header "總結"
  for c in ${skips}; do
    in_list "${c}" "${checks}" && ! in_list "${c}" "${SKIPPED}" &&
      msg "[NOTE]" "${yellow}" "${c} 在 SKIP_CHECKS 但照常執行了：SKIP_CHECKS 只對跑不起來有效"
  done
  [ -z "${WARNED}" ] || msg "[WARN]" "${yellow}" "有警告（不擋）：${WARNED# }"
  [ -z "${SKIPPED}" ] || msg "[SKIP]" "${yellow}" "未檢查（使用者指定跳過）：${SKIPPED# } —— 本次結果不等於全綠"
  if [ -n "${FAILED}" ]; then
    msg "[FAIL]" "${red}" "未通過：${FAILED# }"
    exit 1
  fi
  if [ -n "${SKIPPED}" ]; then
    msg "[ OK ]" "${green}" "其餘檢查通過"
  else
    msg "[ OK ]" "${green}" "全部通過"
  fi
  exit 0
}

## 函式庫的函式設成唯讀：lint.sh 定義同名函式（例如遷移時留下舊的 check_md、record_fail）只會報錯、不會蓋掉，
## 否則基礎檢查可以靜默變成空殼而 pre-push 照樣放行。本 repo 的檢查用別的名字，或用 md_extra 掛點
readonly -f msg header in_list record_fail record_warn skip_requested cannot_run need_tool list_files list_shell_files \
  check_md check_shell check_trivy check_conflict lint_usage lint_doctor lint_main
