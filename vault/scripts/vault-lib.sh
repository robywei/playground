#!/bin/bash
## Vault CLI 共用函式庫 —— 讀取結果分類（讀到 / 不存在 / 讀取失敗）、登入與寫入權預檢
##
## 🔁 **本檔由 repo-template 管理**（元件 `vault_scripts`，各 repo 一律放 `vault/scripts/vault-lib.sh`）：
##    要改請改範本、發版後 `copier update`，不要直接改這裡。
##    → 每個 repo 各放一份，**只 clone 單一 repo 也能獨立運行**；不可寫死任何 repo 的路徑。
##
## 🔑 為什麼需要這支（2026-10-06 實測）：`vault read` / `list` / `kv get` / `kv list` 對
##    「路徑不存在」與「403（含未登入、token 過期、policy 不涵蓋）」的**退出碼都是 2**，
##    唯一差別在 stderr：不存在是 `No value found at <path>`，403 是 `Code: 403` + `* permission denied`。
##    各腳本自己寫 `x=$(vault … 2>/dev/null)` 再判空，就會把「讀不到」當成「不存在」——
##    接著就會**做出寫入 / 覆蓋 / 放行**（例：把既有 secret 當成新增而整份覆寫、跳過本該擋下的檢查）。
##    讀取一律走 `vault_try`，不要再各寫一套。
##
## 用法（source 後呼叫；本檔只定義函式，不改 `set` 選項、不輸出任何東西）：
##   source "<本檔路徑>"
##   vault_require_token || exit 1
##   rc=0; json=$(vault_kv_data "$path") || rc=$?
##   case $rc in 0) 讀到 ;; 3) 不存在 ;; *) 讀取失敗（原因已印到 stderr）;; esac
##
## 函式一覽：
##   vault_try <vault 子命令…>     通用：執行並分類（其餘函式都建在它上面）
##   vault_require_token           有沒有有效 token
##   vault_kv_data <path>          讀一筆 secret 的 data JSON
##   vault_kv_is_dir <path>        0 目錄 / 1 不是 / 2 讀取失敗
##   vault_kv_walk <base>          遞迴列出所有 secret 相對路徑（失敗的子樹印 `!<path>`）
##   vault_kv_mount <path>         `<mount/> <版本>`
##   vault_kv_acl_path <path>      ACL 用路徑（v2 插 `data/`）
##   vault_kv_can <path> <cap…>    寫入前預檢 capability
##
## 回傳碼（除 vault_kv_is_dir 外一致）：0 = 成功 / 讀到；3 = 不存在（`No value found at`）；1 = 讀取失敗
## 依賴：vault、jq、grep、sed
## ⚠️ 呼叫端開了 `set -e` 時，`x=$(vault_try …)` 回非零會在**賦值那行**中止 —— 一律寫成
##    `x=$(vault_try …) || rc=$?` 或 `if x=$(vault_try …); then`
## ⚠️ 相容 bash 3.2（macOS `/bin/bash`）與 zsh（source 型呼叫端如 env.sh 會在使用者的 zsh 裡跑）：
##    不用陣列、`[[ =~ ]]`、`local -n`；所有變數皆 `local`

## 執行一個 vault 子命令並分類結果。stdout 原樣輸出；讀取失敗時把精簡原因印到 stderr。
##   vault_try kv get -format=json cluster-admin/FOO
##   vault_try list -format=json auth/jwt/role
vault_try() {
  local _vl_err _vl_rc=0
  ## stdout 經 fd 3 原樣送出、stderr 留在變數 —— 不落暫存檔（bash 3.2 / zsh 皆可）
  { _vl_err=$(vault "$@" 2>&1 1>&3 3>&-) || _vl_rc=$?; } 3>&1
  if [ "${_vl_rc}" -eq 0 ]; then
    return 0
  fi
  case "${_vl_err}" in
    "No value found at"*|*"
No value found at"*) return 3 ;;
  esac
  ## `list -format=json` 對不存在的路徑**不印任何 stderr**（stdout 是 `{}`、rc=2）——
  ## 只有它這樣（get / read 都會印 `No value found`，2026-10-06 對 dev server 實測）。
  ## 403 / 網路錯誤一定有 stderr，所以「list ＋ rc=2 ＋ stderr 全空」只可能是不存在
  if [ "${_vl_rc}" -eq 2 ] && [ -z "$(printf '%s' "${_vl_err}" | tr -d '[:space:]')" ]; then
    case "$1 $2" in "list "*|"kv list") return 3 ;; esac
  fi
  ## 標籤取「子命令 ＋ 子命令後第一個非旗標參數（path）」，不印旗標與 key=value
  local _vl_sub="$1" _vl_nsub=1 _vl_path="" _vl_i=0 _vl_a _vl_what _vl_kind="失敗"
  case "$1" in kv|token|auth|secrets|policy) _vl_sub="$1 $2"; _vl_nsub=2 ;; esac
  if [ "${_vl_sub}" = "kv metadata" ]; then _vl_sub="kv metadata $3"; _vl_nsub=3; fi
  for _vl_a in "$@"; do
    _vl_i=$((_vl_i + 1))
    [ "${_vl_i}" -le "${_vl_nsub}" ] && continue
    case "${_vl_a}" in -*) continue ;; esac
    _vl_path="${_vl_a}"
    break
  done
  _vl_what="vault ${_vl_sub}${_vl_path:+ ${_vl_path}}"
  ## 讀取類才加「不代表不存在」；寫入 / 刪除包在 vault_try 裡時只說失敗
  case "${_vl_sub}" in
    read|list|"kv get"|"kv list"|"kv metadata get"|"token lookup"|"token capabilities"|"policy read"|"policy list"|"auth list"|"secrets list")
      _vl_kind="讀取失敗（不代表不存在）" ;;
  esac
  vault_print_error "${_vl_what}" "${_vl_err}" "${_vl_kind}" >&2
  return 1
}

## 把 vault 的錯誤輸出縮成一行（`Code: 403` ＋最內層的 `* permission denied`）。
## 403 的 stderr 是巢狀清單（`* 1 error occurred:` 底下才是真正原因）→ 取最後一個 `* ` 行。
vault_print_error() {
  local _vl_what="$1" _vl_text="$2" _vl_kind="${3:-失敗}" _vl_code _vl_reason
  _vl_code=$(printf '%s\n' "${_vl_text}" | grep -m1 'Code: ' | sed 's/^[[:space:]]*//; s/\. Errors:$//')
  _vl_reason=$(printf '%s\n' "${_vl_text}" | grep '\* ' | tail -1 | sed 's/^[[:space:]]*\* //')
  if [ -z "${_vl_code}${_vl_reason}" ]; then
    _vl_reason=$(printf '%s\n' "${_vl_text}" | grep -v '^[[:space:]]*$' | head -1)
  fi
  printf '[VAULT]  %s %s：%s%s%s\n' "${_vl_what}" "${_vl_kind}" \
    "${_vl_code}" "${_vl_code:+${_vl_reason:+ · }}" "${_vl_reason:-（無錯誤訊息）}"
}

## 確認目前有有效的 Vault token（環境變數 `VAULT_TOKEN` 或 token helper）。
## 只擋得住「沒登入 / 過期」，擋不住「已登入但 policy 不涵蓋」—— 後者靠 vault_try 的分類。
vault_require_token() {
  if vault token lookup >/dev/null 2>&1; then
    return 0
  fi
  printf '[ERROR]  沒有有效的 Vault token（未登入或已過期，VAULT_ADDR=%s）\n' "${VAULT_ADDR:-<未設定>}" >&2
  printf '[SAMPLE] vault login   ## 或使用 repo 內的登入腳本\n' >&2
  return 1
}

## // ------------------------------------------------------------------- // ##
## KV 讀取（以下回傳碼同 vault_try；「失敗原因」都已印到 stderr）

## 讀一筆 KV secret 的 data（stdout 為 JSON；v1 / v2 皆可，`vault kv get` 自己判斷）
## ⚠️ 刻意不退回 `vault read`：在 v2 mount 上那條會回 404，等於把讀取失敗洗成「不存在」
## ⚠️ v2 判準看結構（`.data.metadata` 是含 `version` 的物件），不用 `.data.data // .data`：
##    v2 最新版本被**軟刪除**時 `kv get` 照樣 rc=0、`.data.data` 是 null，`//` 會退回整包
##    `{data:null, metadata:{…}}` 當成內容（2026-10-06 dev server 實測）→ 這種情況回 3（視同不存在，
##    與 Vault UI 一致）；v1 secret 剛好有 `data` key 時也不會被誤拆
vault_kv_data() {
  local _vl_raw _vl_rc=0 _vl_out
  _vl_raw=$(vault_try kv get -format=json "$1") || _vl_rc=$?
  if [ "${_vl_rc}" -ne 0 ]; then
    return "${_vl_rc}"
  fi
  _vl_out=$(printf '%s\n' "${_vl_raw}" | jq '
    if (.data | type) == "object" and (.data.metadata | type) == "object" and (.data.metadata | has("version"))
    then .data.data else .data end') || return 1
  if [ "${_vl_out}" = "null" ]; then
    return 3
  fi
  printf '%s\n' "${_vl_out}"
}

## KV 路徑是不是目錄：0 = 目錄、1 = 不是（葉節點或不存在，兩者 `kv list` 都回 `No value found`）、
## 2 = 讀取失敗。🔴 讀取失敗**不可**落到「不是目錄」—— 那會把目錄當成單一 secret 處理
vault_kv_is_dir() {
  local _vl_out _vl_rc=0
  _vl_out=$(vault_try kv list -format=json "${1%/}") || _vl_rc=$?
  case "${_vl_rc}" in
    0) if printf '%s\n' "${_vl_out}" | jq -e 'type == "array"' >/dev/null 2>&1; then return 0; fi
       return 1 ;;
    3) return 1 ;;
    *) return 2 ;;
  esac
}

## 遞迴列出 KV 目錄底下所有 secret，stdout 一行一個**相對於 base 的路徑**（未排序）。
## 列不出來的子目錄印成 `!<完整路徑>` —— 本函式通常跑在 `$( )` 子 shell，計數器帶不回呼叫端，
## 所以用標記行讓呼叫端分流（secret 名稱不會以 `!` 開頭）。**呼叫端必須處理 `!` 行**，
## 否則就退回「子樹列不出來 → 靜默漏掉 → 印完成」的舊病。
##   vault_kv_walk secret/myapp/config
vault_kv_walk() {
  local _vl_base="${1%/}" _vl_cur="${2:-}" _vl_full _vl_items _vl_item _vl_rel _vl_rc=0
  _vl_full="${_vl_base}${_vl_cur:+/${_vl_cur}}"
  _vl_items=$(vault_try kv list -format=json "${_vl_full}") || _vl_rc=$?
  case "${_vl_rc}" in
    0) ;;
    3) return 0 ;;
    *) printf '!%s\n' "${_vl_full}"; return 0 ;;
  esac
  _vl_items=$(printf '%s\n' "${_vl_items}" | jq -r '.[]')
  while IFS= read -r _vl_item; do
    [ -z "${_vl_item}" ] && continue
    _vl_rel="${_vl_cur:+${_vl_cur}/}${_vl_item%/}"
    case "${_vl_item}" in
      */) vault_kv_walk "${_vl_base}" "${_vl_rel}" ;;
      *)  printf '%s\n' "${_vl_rel}" ;;
    esac
  done <<EOF
${_vl_items}
EOF
}

## 印出 KV 路徑所屬的 mount 與版本（`<mount/> <1|2>`），用 Vault 自己的 mount 查詢、不假設 mount 名
vault_kv_mount() {
  local _vl_json _vl_mount _vl_ver
  _vl_json=$(vault_try read -format=json "sys/internal/ui/mounts/${1%/}") || return 1
  _vl_mount=$(printf '%s\n' "${_vl_json}" | jq -r '.data.path // empty')
  _vl_ver=$(printf '%s\n' "${_vl_json}" | jq -r '.data.options.version // "1"')
  if [ -z "${_vl_mount}" ]; then
    printf '[VAULT]  找不到 %s 所屬的 mount\n' "$1" >&2
    return 1
  fi
  printf '%s %s\n' "${_vl_mount}" "${_vl_ver}"
}

## 把 KV 路徑換成 ACL 用的路徑（v2 要插 `data/`）；查不到 mount 回 1
vault_kv_acl_path() {
  local _vl_path="${1%/}" _vl_info _vl_mount _vl_ver
  _vl_info=$(vault_kv_mount "${_vl_path}") || return 1
  _vl_mount="${_vl_info% *}"
  _vl_ver="${_vl_info##* }"
  if [ "${_vl_ver}" = "2" ]; then
    printf '%sdata/%s\n' "${_vl_mount}" "${_vl_path#"${_vl_mount}"}"
  else
    printf '%s\n' "${_vl_path}"
  fi
}

## 寫入前預檢：目前 token 對 KV 路徑是否具備全部指定的 capability（不寫入、不讀值）。
##   vault_kv_can cluster-admin/ALIYUN/foo create update || exit 1
## root token 視為全部具備；缺任何一項回 1 並列出缺哪些。
vault_kv_can() {
  local _vl_path="$1" _vl_acl _vl_caps _vl_need _vl_missing=""
  shift
  _vl_acl=$(vault_kv_acl_path "${_vl_path}") || return 1
  _vl_caps=$(vault_try token capabilities "${_vl_acl}") || return 1
  case ", ${_vl_caps}, " in *", root, "*) return 0 ;; esac
  for _vl_need in "$@"; do
    case ", ${_vl_caps}, " in *", ${_vl_need}, "*) ;; *) _vl_missing="${_vl_missing} ${_vl_need}" ;; esac
  done
  if [ -n "${_vl_missing}" ]; then
    printf '[VAULT]  token 對 %s 缺少權限：%s（現有：%s）\n' "${_vl_acl}" "${_vl_missing# }" "${_vl_caps}" >&2
    return 1
  fi
  return 0
}
