# trivy 掃描

> 由 repo-template 管理（基礎層），要改請改範本；本 repo 的掃描資料（下表「歸本 repo」）直接改

本機與 pre-push 共用同一個入口 `trivy-cli.sh`：閘門跑的是 `QUIET=true ./trivy-cli.sh repo`，掃描行為與手動執行完全相同。
需要 `trivy`；KEV／EPSS 判讀另需 `curl`、`jq`、`gunzip`。

| 檔案 | 歸屬 | 內容 |
| --- | --- | --- |
| `trivy-cli.sh` | 範本 | 掃描入口 |
| `kev-epss.sh` | 範本 | KEV／EPSS 判讀函式庫（也可獨立執行：`./kev-epss.sh <trivy.log>`） |
| `.trivyignore.yaml` | 歸本 repo（範本只產生一次） | 忽略清單；加之前的三個條件寫在檔頭 |
| `config/secret-rules.yaml` | 歸本 repo（範本只產生一次） | 自訂 secret 規則與依路徑的豁免 |
| `config/trivy.yaml` | 歸本 repo（選用，存在才載入） | trivy 全域設定 |

## 用法

```bash
./scripts/trivy/trivy-cli.sh repo                      # 掃本 repo（git 忽略的檔不掃）
./scripts/trivy/trivy-cli.sh repo ../other-repo        # 路徑相對於呼叫時的目錄
./scripts/trivy/trivy-cli.sh image nginx:1.27 redis:7
REGISTRY=nginx TAG_LIST="1.26 1.27" ./scripts/trivy/trivy-cli.sh image
./scripts/trivy/trivy-cli.sh conf ./deploy
./scripts/trivy/trivy-cli.sh k8s <context|環境名> [trivy 旗標…]
QUIET=true ./scripts/trivy/trivy-cli.sh repo           # 閘門模式
```

- **退出碼**：`QUIET=true` 時乾淨＝0、有發現＝1、無法執行＝2（找不到 trivy、漏洞庫下載失敗、cache lock…；輸出會印出來）。
  `lint.sh` 靠這個區分：2 屬於「跑不起來」，使用者可用 `SKIP_CHECKS=trivy` 明確跳過；1 不行，要逐筆寫進 `.trivyignore.yaml`。
  一般模式是報表用途，**有發現也是 0**，只反映 trivy 有沒有執行成功 → 要當檢查用一律帶 `QUIET=true`
- **log**：一般模式每次寫一份 `$TMPDIR/trivy/trivy_<模式>_<時間>.log`（沒有 `TMPDIR` 時是 `/tmp`；`TRIVY_LOG_DIR` 可改）
- **k8s**：參數是 kube context 就直接交給 trivy，**不會**切換你目前的 context；
  不是 context 時才交給本 repo 的 `common/env.sh` 或 `scripts/lib/env.sh` 對照環境名（那支通常會順手切換 context）。
  預設只掃 Deployment／StatefulSet／DaemonSet 的 CRITICAL、不起 node-collector；後面接的 trivy 旗標可覆寫

## KEV／EPSS：決定先修哪個

一般模式掃完會對 CVE 補上兩個外部訊號（`KEV_EPSS=false` 關閉；QUIET 不跑）：

| 訊號 | 來源 | 意義 |
| --- | --- | --- |
| **KEV** | CISA Known Exploited Vulnerabilities（連不到時改用 NVD） | 已被實際利用 —— 命中就是最高優先，不論 CVSS |
| **EPSS** | FIRST EPSS 全量 CSV | 未來 30 天被利用的機率（0～1） |

**P0 ＝ KEV 命中，或 EPSS ≥ 0.5**（`EPSS_THRESHOLD` 可改）。優先序：KEV > 高 EPSS > 高 CVSS ——
CVSS 9.8 但 EPSS 0.001 且不在 KEV 的，排在 CVSS 7.5 但在 KEV 裡的之後。

- 查不到的一律寫 `unknown`、不寫 0 或 N：「查不到」與「確認不在」是兩件事
- 情資快取在 `$TMPDIR/trivy-intel/`（必須是自己擁有的目錄，否則不使用），超過有效期才重抓（`INTEL_MAX_AGE_MIN`，預設值見 `kev-epss.sh`）；要立刻更新就刪掉該目錄

## ⚠️ 綠燈 ≠ 沒有機密

- 內建 secret 規則只認**有固定前綴**的憑證；沒有前綴的密碼靠 `config/secret-rules.yaml` 的自訂規則
- **AWS 官方範例 key `AKIAIOSFODNN7EXAMPLE` 內建規則不報**（被放行）—— 拿它當探針會得到「trivy 沒在掃」的錯誤結論。
  探針要用非範例的假值，並先確認它在基準組會紅
- git 忽略的檔不掃（不會進版控，掃到也無從修）；已追蹤的檔即使符合 `.gitignore` 照樣掃
