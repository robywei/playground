---
name: changelog-monthly
description: "CHANGELOG.md 月度歸檔：把已結束月份的紀錄補進 docs/history/archive/<年>/。觸發：歸檔、archive、月度歸檔、搬舊紀錄、備份舊 CHANGELOG。⚠️ **只有用戶明確提到上述關鍵字時才走這支**，其餘一律走 git:commit；重新生成當月走 git:changelog。"
---

# CHANGELOG 月度歸檔

主檔需要修剪時（建議在月初），把**已結束月份**的紀錄補進 `docs/history/archive/<年>/CHANGELOG.md`。

> 🔁 本檔由 repo-template 管理：要改共用流程請改範本、發版後 `copier update`，**不要直接改這裡**。
> 本 repo 歸檔檔的歷史（有哪些年度檔、導入 git-cliff 時搬了什麼）若有記錄，在
> `.claude/custom/git-changelog-monthly.md`（存在才讀；需要記錄時新建）。

## 🔑 歸檔決定了主檔的生成範圍

歸檔到 X 月之後，主檔下次生成就自動從 X+1 月 1 號起（見 [`/git:changelog`](../changelog/SKILL.md)「生成範圍」）。
所以本流程**不需要「從主檔剪下」**，只需要**補進歸檔檔**，主檔會自己縮短。
歸檔不是必要行為、也不必按月做：不做只是主檔變長（內容永遠在 git log 裡），生成邏輯完全正常。

## 歸檔目錄結構

按**年份分目錄**，每年一個累積檔：

```text
docs/history/archive/
├── <前一年>/CHANGELOG.md   # 該年各月（時間倒序）
└── <今年>/CHANGELOG.md     # 今年各月（時間倒序）
```

- 每個年度檔內部時間倒序（最近的在最上面），標題為 `# 📋 CHANGELOG.md（<年> 年歸檔）`
- 年度檔之間以頂部 `>` 引言互相 cross-reference；跨年第一次歸檔時新建該年目錄與檔案
- 本 repo 實際有哪些年度檔：`ls docs/history/archive/`（custom 規則檔若有記錄搬入的經過，一併讀）

## 執行步驟

### 步驟 1：生成上個月的區塊

```bash
export TZ="${TZ:-Asia/Taipei}"                       ## 與 generate.sh、.cliff.toml 同一個時區，月份邊界才一致
LM_START=$(date -v-1m +%Y-%m-01 2>/dev/null || date -d "$(date +%Y-%m-01) -1 month" +%Y-%m-01)   ## BSD / GNU 兩種 date 都支援
LM_END=$(date +%Y-%m-01)
A=$(git rev-list -1 --before="${LM_START} 00:00" HEAD)
B=$(git rev-list -1 --before="${LM_END} 00:00" HEAD)
git rev-list --count "$A..$B"                        ## 先看筆數，0 筆就不必往下做
BLOCK=$(mktemp) && git-cliff "$A..$B" --strip all > "$BLOCK"
```

- 🔴 **`--before` 一定要帶 `00:00`**：純日期會被解析成「當天當下」，range 會錯
- **`--strip all`** 去掉 header / footer，只留條目 —— 不加會把檔頭一起搬進歸檔檔

### 步驟 2：確認目標年度檔

1. `docs/history/archive/<年>/CHANGELOG.md` 存在 → 讀開頭確認最新月份，避免重複
2. 不存在 → 以 `# 📋 CHANGELOG.md（<年> 年歸檔）` 新建，並補指向相鄰年度檔的 `>` 引言

### 步驟 3：插入

把 `$BLOCK` 的內容插在年度檔的**標題/引言之後、既有內容之前**（時間倒序）。

### 步驟 4：重新生成主檔並驗證

```bash
./scripts/changelog/generate.sh         ## 範圍自動跳過已歸檔的月份
./scripts/lint/lint.sh md               ## 權威閘門
```

驗證項：

1. 歸檔檔的月份排列是時間倒序
2. 主檔已不含上月的 `## 📆 <上月>` 區塊
3. 條目數對得上步驟 1 的 `git rev-list --count`

## ⚠️ 導入 git-cliff 前的手寫紀錄，格式和生成的不同

若導入時把手寫紀錄整批搬進了歸檔檔，同一個年度檔會混排兩種格式：

```text
## 📆 2026-08          ← git-cliff 生成：月 ＋ 日兩層
### 📅 2026-08-31
## 📆 2026/08          ← 手寫：日期在 `* YYYY/MM/DD` 那層
```

月份標題寫法不同（dash vs slash），不會互相覆蓋，生成範圍也只認 dash。
**這是刻意的，不要「統一格式」** —— 舊條目是歷史快照，不完全等於當時的 commit message，重寫沒有來源可依據。

## 注意事項

- **本流程只增不刪**：歸檔檔永遠不刪除舊月份 —— 它沒有其他備份來源，刪掉只能從 git log 重建
- 歸檔完成後走 [`/git:commit`](../commit/SKILL.md) 提交
