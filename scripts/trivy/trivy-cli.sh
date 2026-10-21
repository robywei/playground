#!/usr/bin/env bash
##
## trivy-cli.sh — 本機 trivy 掃描入口：repo / image / conf / k8s；pre-push 的 trivy 檢查也呼叫它
## ============================================================================
## 🔁 本檔由 repo-template 管理（基礎層）：要改請改範本、發版後 `copier update`，
##    不要直接改這裡 —— 直接改會被當成本地客製保留下來，從此和其他 repo 分岔。
##    掃描資料（.trivyignore.yaml、config/secret-rules.yaml、選用的 config/trivy.yaml）歸本 repo，見 README.md。
##
## 用法：不帶參數印說明；模式、退出碼、KEV/EPSS 判讀見同目錄 README.md
##
## 🔑 設定檔一律相對「本目錄」解析（先 cd 到這裡），使用者給的路徑則相對「呼叫時的目錄」——
##    舊版兩者都相對本目錄，`trivy-cli.sh repo .` 因此掃到 scripts/trivy 自己（0 個設定檔、照樣綠燈）。
##
## ! 刻意不開 set -e / set -u：掃描失敗要繼續走到收尾回報；空陣列在 bash 3.2 下 set -u 會誤判 unbound。
##

set -o pipefail

CALLER_DIR="$PWD"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}" || exit 1

## ---- 顏色（自給自足，不依賴各 repo 的 color helper；非 tty 不輸出色碼）----
## reset 用 ESC[0m 而非 tput：`tput sgr0` 在 xterm-256color 會多吐 `ESC(B`，過 CI / kubectl logs 漏成字面 `(B`
if [ -t 1 ]; then
  red=$(printf '\033[31m') green=$(printf '\033[32m') yellow=$(printf '\033[33m')
  blue=$(printf '\033[34m') cyan=$(printf '\033[36m') bold=$(printf '\033[1m') reset=$(printf '\033[0m')
else
  red='' green='' yellow='' blue='' cyan='' bold='' reset=''
fi
msg() { printf "%b%-8s %s%b\n" "$2" "$1" "$3" "${reset}"; }

usage() {
  cat <<'EOF'
用法: trivy-cli.sh <MODE> [ARGS]

  repo  [PATH]                  掃 git repo（預設：本 repo 根目錄；git 忽略的檔不掃）
  image <IMG> [IMG ...]         掃容器映像；或 REGISTRY=<repo> TAG_LIST="t1 t2" trivy-cli.sh image
  conf  <PATH>                  掃設定檔（IaC / manifest）；或設 CONF_PATH
  k8s   <CONTEXT|ENV> [FLAGS]   掃叢集 workload；ENV 需要本 repo 的 env.sh 對照（見 README.md）

  QUIET=true   只在有發現時印輸出、有發現 rc=1（pre-push 用）
  KEV_EPSS=false   非 QUIET 時不做 KEV / EPSS 判讀
EOF
}

command -v trivy > /dev/null || { msg "[ERROR]" "${red}" "找不到 trivy（brew install trivy）—— 沒有掃描，不是通過"; exit 1; }

MODE="${1:-}"
[ -n "${MODE}" ] || { usage; exit 0; }
shift
QUIET="${QUIET:-false}"

## 選用設定：存在才帶。`-c` 指向不存在的檔是 FATAL（rc=1），QUIET 時會被讀成「有發現」
COMMON=(--ignorefile .trivyignore.yaml)
[ -f config/trivy.yaml ] && COMMON+=(-c config/trivy.yaml)

## 每次執行一個 log（帶模式與時間戳，同一天重跑不互相覆蓋）；KEV/EPSS 從它抽 CVE
LOG_DIR="${TRIVY_LOG_DIR:-${TMPDIR:-/tmp}/trivy}"
LOG_FILE="${LOG_DIR}/trivy_${MODE}_$(date '+%Y%m%d-%H%M%S').log"

resolve() { case "$1" in /*) printf '%s\n' "$1" ;; *) printf '%s\n' "${CALLER_DIR}/$1" ;; esac; }

## run_scan <標籤> <trivy 子指令與參數…>：回傳 trivy 自己的 rc
run_scan() {
  local label=$1 out rc
  shift
  if [ "${QUIET}" = true ]; then
    out=$(mktemp "${TMPDIR:-/tmp}/trivy-quiet.XXXXXX") || return 1
    trivy "$@" --exit-code 1 > "${out}" 2>&1
    rc=$?
    if [ "${rc}" -ne 0 ]; then
      msg "[FAIL]" "${red}" "${label}：有發現或 trivy 執行失敗（rc=${rc}）"
      cat "${out}"
    else
      msg "[ OK ]" "${green}" "${label}：沒有發現"
    fi
    rm -f "${out}"
    return "${rc}"
  fi
  mkdir -p "${LOG_DIR}" || return 1
  msg "[SCAN]" "${cyan}" "${label}：trivy $*"
  trivy "$@" 2>&1 | tee -a "${LOG_FILE}"
  return "${PIPESTATUS[0]}"
}

## k8s 的目標：本身是 kube context 就直接用（傳給 trivy，不做全域 use-context）；
## 否則交給本 repo 的 env.sh 把環境名對照成 context（各 repo 自己維護；它通常會順手切換目前的 context）
k8s_context() {
  local want=$1 f
  if kubectl config get-contexts -o name | grep -xF -- "${want}" > /dev/null; then
    printf '%s\n' "${want}"
    return 0
  fi
  for f in ../../common/env.sh ../lib/env.sh; do
    [ -f "${f}" ] || continue
    ENV="${want}"
    # shellcheck source=/dev/null
    source "${f}" "${want}" >&2
    [ -n "${CLUSTER_NAME:-}" ] && { printf '%s\n' "${CLUSTER_NAME}"; return 0; }
  done
  msg "[ERROR]" "${red}" "${want} 不是 kube context，本 repo 也沒有能對照它的 env.sh" >&2
  return 1
}

RC=0
TARGET=""
case "${MODE}" in
  repo)
    TARGET=$(resolve "${1:-$(git rev-parse --show-toplevel)}")
    ARGS=(--scanners "vuln,misconfig,secret")
    [ -f config/secret-rules.yaml ] && ARGS+=(--secret-config config/secret-rules.yaml)
    ## git 忽略的檔不掃：它們不會進版控（temp/、本機明碼匯出、.env…），掃到也無從修，只會逼人開全域豁免
    if git -C "${TARGET}" rev-parse --git-dir > /dev/null; then
      while IFS= read -r -d '' p; do
        case "${p}" in
          */) ARGS+=(--skip-dirs "${TARGET}/${p%/}") ;;
          *) ARGS+=(--skip-files "${TARGET}/${p}") ;;
        esac
      done < <(git -C "${TARGET}" ls-files --others --ignored --exclude-standard --directory -z)
    fi
    run_scan "repo ${TARGET}" repo "${COMMON[@]}" "${ARGS[@]}" "${TARGET}" || RC=$?
    ;;
  image)
    IMAGES=("$@")
    if [ "${#IMAGES[@]}" -eq 0 ] && [ -n "${REGISTRY:-}" ] && [ -n "${TAG_LIST:-}" ]; then
      for tag in ${TAG_LIST}; do IMAGES+=("${REGISTRY}:${tag}"); done
    fi
    [ "${#IMAGES[@]}" -gt 0 ] || { msg "[ERROR]" "${red}" "要給映像名稱，或設定 REGISTRY 與 TAG_LIST"; usage; exit 1; }
    for img in "${IMAGES[@]}"; do
      run_scan "image ${img}" image "${COMMON[@]}" "${img}" || RC=$?
    done
    ;;
  conf)
    [ -n "${1:-${CONF_PATH:-}}" ] || { msg "[ERROR]" "${red}" "要給路徑，或設定 CONF_PATH"; usage; exit 1; }
    TARGET=$(resolve "${1:-${CONF_PATH}}")
    run_scan "conf ${TARGET}" config "${COMMON[@]}" "${TARGET}" || RC=$?
    ;;
  k8s)
    [ -n "${1:-}" ] || { msg "[ERROR]" "${red}" "要給 kube context 或環境名"; usage; exit 1; }
    TARGET=$(k8s_context "$1") || exit 1
    shift
    ## 預設範圍：長駐 workload 的 CRITICAL；不起 node-collector（它建的 Job 沒有回收機制，中斷就永久殘留）。
    ## --report all 才有逐筆 CVE（summary 沒有，KEV/EPSS 會抽到 0 筆）。後面的參數可覆寫這些預設。
    run_scan "k8s ${TARGET}" k8s "${COMMON[@]}" --report all --scanners vuln,misconfig,secret -s CRITICAL \
      --disable-node-collector --include-kinds Deployment,StatefulSet,DaemonSet "$@" "${TARGET}" || RC=$?
    ;;
  -h | --help | help)
    usage
    exit 0
    ;;
  *)
    msg "[ERROR]" "${red}" "無法辨識的模式: ${MODE}"
    usage
    exit 1
    ;;
esac

## ---- KEV / EPSS 判讀（只在非 QUIET 模式；QUIET 是閘門，只看 rc）----
if [ "${QUIET}" != true ] && [ "${KEV_EPSS:-true}" = true ] && [ "${MODE}" != conf ] && [ -s "${LOG_FILE}" ]; then
  # shellcheck source-path=SCRIPTDIR
  source ./kev-epss.sh
  ## 沒有 CVE 就不抓情資（約 20 MB）、不產生只有表頭的 CSV —— 那會讓「沒有東西要修」看起來像有明細可看
  if ! grep -q 'CVE-[0-9]' "${LOG_FILE}"; then
    msg "[INFO]" "${blue}" "掃描結果沒有 CVE，不做 KEV / EPSS 判讀"
  elif ! kev_epss_fetch; then
    msg "[WARN]" "${yellow}" "情資取得失敗，本次沒有做 KEV / EPSS 判讀"
  else
    CSV="${LOG_FILE%.log}.enriched.csv"
    ## 第二個參數是 CSV 的 namespace 欄：只有 k8s 模式有意義（填 context）
    kev_epss_enrich "${LOG_FILE}" "$([ "${MODE}" = k8s ] && printf '%s' "${TARGET}")" > "${CSV}"
    kev_epss_summary "${CSV}"
    msg "[TASK]" "${bold}" "KEV + EPSS 安全底線（情資：KEV ${KEV_DATE:-未取得} / EPSS ${EPSS_DATE}）"
    printf '去重 CVE %s | KEV %s | EPSS>=%s %s | EPSS 未知 %s\n' \
      "${CVE_COUNT}" "${KEV_COUNT}" "${EPSS_THRESHOLD}" "${EPSS_COUNT}" "${UNKNOWN_COUNT}"
    ## KEV 沒取得（降級 EPSS-only）時「不在 KEV」無從判定，不能印成「未命中」
    [ "${KEV_OK}" = 1 ] || msg "[WARN]" "${yellow}" "KEV 未取得，只依 EPSS 判定（kev 欄是 unknown）"
    if [ "${P0_COUNT}" -gt 0 ]; then
      msg "[P0]" "${red}" "${P0_COUNT} 筆 —— 命中 KEV 或 EPSS >= ${EPSS_THRESHOLD}，優先處理"
      awk -F, 'NR==1 || $12=="Y"' "${CSV}" | cut -d, -f5,6,9,10,11 | sort -u
    else
      msg "[DONE]" "${green}" "P0 0 筆 —— 沒有 KEV 命中，EPSS 均低於 ${EPSS_THRESHOLD}"
    fi
    msg "[INFO]" "${blue}" "明細：${CSV}"
  fi
fi

exit "${RC}"
