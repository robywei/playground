#!/usr/bin/env bash
## ============================================================================
## kev-epss.sh — KEV + EPSS 安全底線判準
##
## 🔁 本檔由 repo-template 管理（基礎層）：要改請改範本、發版後 `copier update`，不要直接改這裡。
##    本機（trivy-cli.sh）與叢集內的排程掃描都可以 source 同一份；只用 POSIX 工具（curl、jq、awk、gunzip），
##    GNU 與 BSD 環境都能跑。
##
## 判準：P0 = KEV 命中  OR  EPSS >= EPSS_THRESHOLD（預設 0.5）
##
## ⚠️ 為何不用 EPSS API：API 的 envelope **預設 limit=100**，把上千個 CVE 塞進
##    query string 只會拿回前 100 筆且**不報錯**。全量 CSV 沒有這個問題。
##
## 用法：
##   source kev-epss.sh            ## 取函式
##   ./kev-epss.sh <trivy.log>     ## 獨立執行，印出富化後的 CSV
## ============================================================================

KEV_URL="https://www.cisa.gov/sites/default/files/csv/known_exploited_vulnerabilities.csv"
## KEV 備援來源：NVD（NIST 官方）。`hasKev` 過濾出的就是 CISA KEV 那份名單。
## 🔴 為什麼需要它：部分雲端的出口 IP 會被 cisa.gov 擋（實測 403、Akamai Access Denied，
##    UA 偽裝無效、JSON endpoint 一樣擋），但 NVD 連得到。
##    能連 CISA 就走 CISA（1 MB CSV、快），連不到自動落到這條（18 MB JSON、約 80 秒）。
## ✅ 等價性已實測：兩邊各 1,721 筆，逐筆比對差集**雙向皆為 0**。
## ⚠️ 分頁取（每頁 500 筆）而非一次 2000 筆：全量回應 18.5 MB，實測一次抓在
##    180 秒逾時（只收到 11.8 MB）——NVD 的速度不穩，一次抓是把整份情資押在單一長連線上。
##    NVD 未帶 API key 的限流是 5 req/30s，4～5 頁在額度內。
KEV_NVD_URL="https://services.nvd.nist.gov/rest/json/cves/2.0?hasKev"
KEV_NVD_PAGE_SIZE="${KEV_NVD_PAGE_SIZE:-500}"
EPSS_URL_BASE="https://epss.empiricalsecurity.com/epss_scores"

## 預設放在 $TMPDIR（macOS 是每個使用者私有的目錄）；共用的 /tmp 只在沒有 TMPDIR 時退回
INTEL_DIR="${INTEL_DIR:-${TMPDIR:-/tmp}/trivy-intel}"
## 快取有效期（分鐘）。過期才重抓 —— 沒有這條時本機快取永不過期，隔週掃描仍用舊情資且不提示
INTEL_MAX_AGE_MIN="${INTEL_MAX_AGE_MIN:-720}"
EPSS_THRESHOLD="${EPSS_THRESHOLD:-0.5}"

KEV_DATE=""
EPSS_DATE=""

## 兩個來源各自的取得狀態（1 = 成功）。
## 🔴 刻意分開：KEV 與 EPSS 是**獨立的兩個來源**，任一個失敗都不該讓另一個一起作廢。
##    實例：出口 IP 被 cisa.gov 擋、但 EPSS 完全正常 —— 此時應降級為 EPSS-only，
##    而不是退回純 severity 判準（那等於放棄唯一還拿得到的情資）。
KEV_OK=0
EPSS_OK=0
## 實際取得 KEV 的來源（cisa / nvd），空字串 = 未取得。供告警版面標示。
KEV_SOURCE=""

## 回傳 N 天前的 UTC 日期（YYYY-MM-DD）
## ⚠️ 兩套寫法缺一不可：容器多是 GNU/busybox date（-d @epoch），macOS 是 BSD date（-v-Nd）
_utc_days_ago() {
    _n="$1"
    date -u -d "@$(( $(date -u +%s) - _n * 86400 ))" +%Y-%m-%d 2>/dev/null \
        || date -u -v-"${_n}"d +%Y-%m-%d 2>/dev/null
}

## 下載 KEV + EPSS 到 $INTEL_DIR
## 回傳 0 = EPSS 至少取得（可能是 EPSS-only 降級，看 KEV_OK）
##      1 = 連 EPSS 都沒有 → 呼叫端退回 severity 判準並在告警標示「情資未取得」
kev_epss_fetch() {
    KEV_OK=0
    EPSS_OK=0
    ( umask 077 && mkdir -p "${INTEL_DIR}" ) || return 1
    ## 🔴 快取目錄必須是自己的：共用 /tmp 下別人先建好的同名目錄，裡面的檔不可信
    if [ ! -O "${INTEL_DIR}" ]; then
        echo "[ERROR] ${INTEL_DIR} 不是目前使用者擁有的目錄，不使用（改設 INTEL_DIR）" >&2
        return 1
    fi

    ## 🔴 快取：同一輪掃描常由多個獨立行程各呼叫一次（例：排程逐 namespace 各跑一次），
    ##    沒有這段就會把 NVD 18.5 MB + EPSS 2.5 MB 重抓 N 次。變數不跨行程，狀態落在 meta 檔裡帶過去。
    ## ⚠️ 判斷條件**不能要求 kev-ids.txt 存在** —— KEV 降級（EPSS-only）時它本來就不在，
    ##    那樣寫會讓降級情況下每次都重抓 EPSS，正好是最該省的時候沒省到。
    ##    以 meta 檔為準：它只在 EPSS 取得成功時才寫；超過 INTEL_MAX_AGE_MIN 分鐘就視為過期。
    if [ -s "${INTEL_DIR}/epss.csv" ] && [ -s "${INTEL_DIR}/meta" ] \
       && [ -n "$(find "${INTEL_DIR}/meta" -mmin -"${INTEL_MAX_AGE_MIN}")" ]; then
        ## 只取格式正確的值，不 source（meta 是資料，不是程式）
        KEV_DATE=$(sed -n -E 's/^KEV_DATE=([0-9-]*)$/\1/p' "${INTEL_DIR}/meta")
        EPSS_DATE=$(sed -n -E 's/^EPSS_DATE=([0-9-]*)$/\1/p' "${INTEL_DIR}/meta")
        KEV_SOURCE=$(sed -n -E 's/^KEV_SOURCE=(cisa|nvd)$/\1/p' "${INTEL_DIR}/meta")
        EPSS_OK=1
        if [ -s "${INTEL_DIR}/kev-ids.txt" ] && [ -n "${KEV_SOURCE:-}" ]; then
            KEV_OK=1
            echo "[INFO]  情資沿用本輪快取（KEV ${KEV_SOURCE} / EPSS ${EPSS_DATE}）" >&2
        else
            KEV_OK=0
            echo "[INFO]  情資沿用本輪快取（EPSS ${EPSS_DATE}；KEV 本輪未取得）" >&2
        fi
        return 0
    fi

    ## ---- KEV ----
    ## 兩個來源都產出同一份 `kev-ids.txt`（一行一個 CVE ID），下游只認這個檔。
    ## ⚠️ 全部失敗**不 return** —— 降級為 EPSS-only，由 KEV_OK 告訴呼叫端與告警版面
    rm -f "${INTEL_DIR}/kev-ids.txt"
    KEV_SOURCE=""

    ## ① CISA（原始權威，1 MB CSV）
    if curl -sSfL --max-time 60 -o "${INTEL_DIR}/kev.csv" "${KEV_URL}" 2>/dev/null \
       && head -1 "${INTEL_DIR}/kev.csv" | grep -q '^cveID,'; then
        ## 🔴 HTTP 200 不等於拿到 CSV（改版／錯誤頁／攔截頁都是 200）→ 上面驗了表頭
        awk -F, 'NR > 1 && $1 ~ /^CVE-/ { print $1 }' "${INTEL_DIR}/kev.csv" \
            | sort -u > "${INTEL_DIR}/kev-ids.txt"
        KEV_SOURCE="cisa"
    ## ② NVD（NIST 官方鏡像）—— 連不到 cisa.gov 時走這條，分頁取
    else
        _nvd_tmp="${INTEL_DIR}/nvd-ids.tmp"
        : > "${_nvd_tmp}"
        _start=0
        _total=-1
        _nvd_ok=1
        ## 上限 20 頁（10,000 筆）純粹是防呆，避免來源異常時無限迴圈
        while [ "${_start}" -lt 10000 ]; do
            if ! curl -sSfL --max-time 120 -o "${INTEL_DIR}/nvd-page.json" \
                    "${KEV_NVD_URL}&resultsPerPage=${KEV_NVD_PAGE_SIZE}&startIndex=${_start}" 2>/dev/null; then
                echo "[WARN]  NVD 第 ${_start} 頁下載失敗" >&2
                _nvd_ok=0
                break
            fi
            _n=$(jq -r '.vulnerabilities | length' "${INTEL_DIR}/nvd-page.json" 2>/dev/null) || _n=0
            if [ -z "${_n}" ] || [ "${_n}" -eq 0 ]; then
                break
            fi
            if [ "${_total}" -lt 0 ]; then
                _total=$(jq -r '.totalResults // 0' "${INTEL_DIR}/nvd-page.json" 2>/dev/null) || _total=0
            fi
            jq -r '.vulnerabilities[].cve.id' "${INTEL_DIR}/nvd-page.json" >> "${_nvd_tmp}" 2>/dev/null
            _start=$(( _start + KEV_NVD_PAGE_SIZE ))
            if [ "${_total}" -gt 0 ] && [ "${_start}" -ge "${_total}" ]; then
                break
            fi
        done
        rm -f "${INTEL_DIR}/nvd-page.json"

        ## 🔴 分頁最怕「少抓了一頁但看起來成功」→ 與 totalResults 對帳才算數
        _got=$(grep -c '^CVE-' "${_nvd_tmp}" 2>/dev/null) || _got=0
        if [ "${_nvd_ok}" -eq 1 ] && [ "${_total}" -gt 0 ] && [ "${_got}" -ge "${_total}" ]; then
            sort -u "${_nvd_tmp}" > "${INTEL_DIR}/kev-ids.txt"
            KEV_SOURCE="nvd"
        elif [ "${_total}" -gt 0 ]; then
            echo "[WARN]  NVD 分頁不完整：取得 ${_got} / totalResults ${_total}，視為未取得" >&2
        fi
        rm -f "${_nvd_tmp}"
    fi

    ## 下限檢查：KEV 名單長期在 1,300+ 筆。回傳筆數異常少 = 來源改版或被截斷，
    ## 這種「拿到了但不完整」比完全拿不到更危險（會把真的在 KEV 的 CVE 判成不在）。
    if [ -s "${INTEL_DIR}/kev-ids.txt" ] \
       && [ "$(grep -c '^CVE-' "${INTEL_DIR}/kev-ids.txt")" -ge 1000 ]; then
        KEV_OK=1
        KEV_DATE="$(_utc_days_ago 0)"
    else
        if [ -n "${KEV_SOURCE}" ]; then
            echo "[WARN]  KEV 筆數異常（來源 ${KEV_SOURCE}），視為未取得、降級為 EPSS-only" >&2
        else
            echo "[WARN]  KEV 兩個來源都取不到（CISA / NVD），降級為 EPSS-only" >&2
        fi
        ## 沒取得就清空，避免 enrich 讀到殘留而把「未知」誤判成「不在 KEV」
        rm -f "${INTEL_DIR}/kev-ids.txt"
        KEV_SOURCE=""
    fi

    ## ---- EPSS ----
    ## UTC 當日檔常態 403（尚未發佈）→ 階梯往回退。退到第 3 天仍失敗就算情資取得失敗。
    _ok=0
    for _i in 0 1 2; do
        _d="$(_utc_days_ago "${_i}")"
        [ -n "${_d}" ] || continue
        if curl -sSfL --max-time 120 -o "${INTEL_DIR}/epss.csv.gz" \
                "${EPSS_URL_BASE}-${_d}.csv.gz" 2>/dev/null; then
            if gunzip -cf "${INTEL_DIR}/epss.csv.gz" > "${INTEL_DIR}/epss.csv" 2>/dev/null; then
                ## 第 2 行是欄位表頭 `cve,epss,percentile`（第 1 行是 model_version 註解）
                if sed -n '2p' "${INTEL_DIR}/epss.csv" | grep -q '^cve,epss,percentile'; then
                    EPSS_DATE="${_d}"
                    _ok=1
                    break
                fi
            fi
        fi
    done

    if [ "${_ok}" -ne 1 ]; then
        echo "[WARN]  EPSS 下載失敗（已試 3 天：$(_utc_days_ago 0) / $(_utc_days_ago 1) / $(_utc_days_ago 2)）" >&2
        rm -f "${INTEL_DIR}/epss.csv"
        return 1
    fi
    EPSS_OK=1

    ## 寫下本輪的情資狀態，供後續行程沿用（見函式開頭的快取判斷）
    {
        printf 'KEV_DATE=%s\n' "${KEV_DATE}"
        printf 'EPSS_DATE=%s\n' "${EPSS_DATE}"
        printf 'KEV_SOURCE=%s\n' "${KEV_SOURCE}"
    } > "${INTEL_DIR}/meta"

    return 0
}

## ----------------------------------------------------------------------------
## 從 trivy table log 抽出逐筆漏洞
## 輸出 TSV: image \t pkg \t cve \t severity \t installed \t fixed
##
## ⚠️ table 的續行（同一 pkg 的第 2 個 CVE 起）會把 pkg / installed 欄留白 ——
##    必須沿用上一筆，否則 CSV 會缺欄。severity 與上一列相同時也會被合併成空白（trivy 0.76 實測）。
## ⚠️ 空輸入與「真的零 CVE」在這裡都輸出零行，兩者的區分由呼叫端負責（見 kev_epss_enrich）
## ----------------------------------------------------------------------------
kev_epss_extract() {
    _log="$1"
    [ -f "${_log}" ] || return 1

    awk -F'│' '
        ## image 名稱：來自 "namespace: <ns>, <kind>: <name> (<type>)" 這類行
        /^namespace:/ {
            img = $0
            sub(/^namespace: [^,]+, /, "", img)
            next
        }
        /CVE-[0-9]/ {
            pkg = $2; cve = $3; sev = $4; inst = $6; fix = $7
            gsub(/^[ \t]+|[ \t]+$/, "", pkg)
            gsub(/^[ \t]+|[ \t]+$/, "", cve)
            gsub(/^[ \t]+|[ \t]+$/, "", sev)
            gsub(/^[ \t]+|[ \t]+$/, "", inst)
            gsub(/^[ \t]+|[ \t]+$/, "", fix)

            if (cve !~ /^CVE-/) next

            ## 續行：欄位留白時沿用上一筆
            if (pkg  == "") pkg  = last_pkg;  else last_pkg  = pkg
            if (inst == "") inst = last_inst; else last_inst = inst
            if (sev  == "") sev  = last_sev;  else last_sev  = sev

            printf "%s\t%s\t%s\t%s\t%s\t%s\n", img, pkg, cve, sev, inst, fix
        }
    ' "${_log}"
}

## ----------------------------------------------------------------------------
## 富化：對每筆漏洞查 KEV / EPSS，判定 P0
## 輸出 CSV（含表頭）: namespace,resource,image,pkg,cve,severity,installed,fixed,kev,epss,percentile,p0
##
## 🔴 EPSS 查無分數一律寫 `unknown`，**不可寫 0** ——
##    「查無分數」是資料缺口、「分數很低」才是結論，混為一談等於把缺口偽裝成綠燈。
## ----------------------------------------------------------------------------
kev_epss_enrich() {
    _log="$1"
    _ns="${2:-}"

    printf 'namespace,resource,image,pkg,cve,severity,installed,fixed,kev,epss,percentile,p0\n'

    kev_epss_extract "${_log}" | awk -F'\t' \
        -v kevf="${INTEL_DIR}/kev-ids.txt" \
        -v epssf="${INTEL_DIR}/epss.csv" \
        -v th="${EPSS_THRESHOLD}" \
        -v ns="${_ns}" \
        -v kevok="${KEV_OK}" '
    BEGIN {
        FS=","
        ## kev-ids.txt 是一行一個 CVE ID（CISA CSV 與 NVD JSON 都正規化成這個格式）
        while ((getline line < kevf) > 0) { if (line ~ /^CVE-/) kev[line] = 1 }
        while ((getline line < epssf) > 0) {
            split(line, b, ",")
            if (b[1] ~ /^CVE-/) { es[b[1]] = b[2]; pc[b[1]] = b[3] }
        }
        FS="\t"
    }
    {
        img = $1; pkg = $2; cve = $3; sev = $4; inst = $5; fix = $6

        ## 🔴 KEV 沒取得時寫 `unknown`，**不可寫 N** ——
        ##    「查不到」與「確認不在 KEV 裡」是兩件事，混為一談會讓缺口變成綠燈。
        isk = (kevok == 1) ? ((cve in kev) ? "Y" : "N") : "unknown"
        if (cve in es) { sc = es[cve]; pcl = pc[cve]; ise = ((sc + 0) >= (th + 0)) }
        else           { sc = "unknown"; pcl = "unknown"; ise = 0 }

        p0 = (isk == "Y" || ise) ? "Y" : "N"

        ## resource：從 image 欄取 "deployment: xxx" 的名稱部分
        res = img
        sub(/ \(.*\)$/, "", res)

        ## 🔴 每個進 CSV 的文字欄位都要清逗號，一個都不能漏 ——
        ##    trivy 的 fixed version 常是「1.24.13, 1.25.7, 1.26.0-rc.3」這種多版本清單，
        ##    漏清它會讓該列變成 13～14 欄，下游用 $10 讀 epss 時就讀到位移後的欄位。
        ##    症狀極隱蔽：P0 判定（在本 awk 內用變數）仍然正確，只有事後統計會錯。
        gsub(/,/, ";", pkg); gsub(/,/, ";", img); gsub(/,/, ";", res)
        gsub(/,/, ";", inst); gsub(/,/, ";", fix); gsub(/,/, ";", sev)
        printf "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n", ns, res, img, pkg, cve, sev, inst, fix, isk, sc, pcl, p0
    }'
}

## ----------------------------------------------------------------------------
## 統計：讀 enrich 產出的 CSV，設定呼叫端變數
## ⚠️ 不可用 pipe 呼叫（子 shell 會讓變數設不回來）
## ----------------------------------------------------------------------------
kev_epss_summary() {
    _csv="$1"
    CVE_COUNT=0; P0_COUNT=0; KEV_COUNT=0; EPSS_COUNT=0; UNKNOWN_COUNT=0

    eval "$(awk -F, -v th="${EPSS_THRESHOLD}" '
        NR == 1 { next }
        {
            cve = $5
            if (!(cve in seen)) { seen[cve] = 1; n++ }
            if ($9  == "Y")        { if (!(cve in k)) { k[cve] = 1; nk++ } }
            if ($10 == "unknown")  { if (!(cve in u)) { u[cve] = 1; nu++ } }
            else if (($10 + 0) >= (th + 0)) { if (!(cve in e)) { e[cve] = 1; ne++ } }
            if ($12 == "Y")        { if (!(cve in p)) { p[cve] = 1; np++ } }
        }
        END {
            printf "CVE_COUNT=%d; P0_COUNT=%d; KEV_COUNT=%d; EPSS_COUNT=%d; UNKNOWN_COUNT=%d\n",
                   n+0, np+0, nk+0, ne+0, nu+0
        }' "${_csv}")"
}

## ----------------------------------------------------------------------------
## 主程式：被 source 時不執行，直接執行時才跑
## ----------------------------------------------------------------------------
if [ "${0##*/}" = "kev-epss.sh" ]; then
    _target_log="${1:-}"
    _target_ns="${2:-}"

    if [ -z "${_target_log}" ] || [ ! -f "${_target_log}" ]; then
        echo "用法: $0 <trivy.log> [namespace]" >&2
        exit 1
    fi

    if ! kev_epss_fetch; then
        echo "[ERROR] 情資取得失敗，無法進行底線判定" >&2
        exit 1
    fi
    echo "[INFO]  情資: KEV ${KEV_DATE} / EPSS ${EPSS_DATE}" >&2

    _out="${_target_log%.log}.enriched.csv"
    kev_epss_enrich "${_target_log}" "${_target_ns}" > "${_out}"
    kev_epss_summary "${_out}"

    echo "[INFO]  CVE ${CVE_COUNT} 筆 | P0 ${P0_COUNT} | KEV ${KEV_COUNT} | EPSS>=${EPSS_THRESHOLD} ${EPSS_COUNT} | unknown ${UNKNOWN_COUNT}" >&2
    echo "[DONE]  ${_out}" >&2

    if [ "${P0_COUNT}" -gt 0 ]; then
        echo "" >&2
        echo "=== P0（踩到安全底線）===" >&2
        awk -F, 'NR==1 || $12=="Y"' "${_out}" | cut -d, -f5,6,9,10,11 | sort -u >&2
    fi
fi
