---
name: changelog
description: "重新生成 CHANGELOG.md（git-cliff 從 commit message 全量產出）。觸發：重新生成 CHANGELOG、更新 CHANGELOG、CHANGELOG 沒有我剛剛那筆、CHANGELOG 格式要改、cliff.toml、git-cliff、條目分類不對、要不要手寫 CHANGELOG、什麼時候該生成、要不要每次 commit 都生成、chore(changelog) 噪音、bot 的 commit 沒進 CHANGELOG、skip ci 前綴要不要加、CHANGELOG 長期未提交。⚠️ 月度歸檔走 git:changelog-monthly，不是這支。"
---

# 重新生成 CHANGELOG.md

`CHANGELOG.md` **不手寫**，由 [`git-cliff`](https://git-cliff.org/) 從 commit message 全量生成，
設定在 repo 根的 [`.cliff.toml`](../../../../../.cliff.toml)。

> 🔁 本檔由 repo-template 管理：要改共用流程請改範本、發版後 `copier update`，**不要直接改這裡**。
> 本 repo 的情況（有沒有每日排程代為生成、CI 會不會推進 `main`、有哪些 bot 會 commit）若有記錄，在
> `.claude/custom/git-changelog.md`，**存在就先讀**（不存在＝沒有專屬資訊：沒有排程，使用者要求時才生成）。

## 生成指令

```bash
./scripts/changelog/generate.sh --dry-run   ## 先看差異，結束後還原工作區
./scripts/changelog/generate.sh             ## 生成（只改工作區，**不 commit / 不 push**）
```

🔗 **[`scripts/changelog/generate.sh`](../../../../../scripts/changelog/generate.sh) 是生成範圍的唯一權威實作**
（與 `.cliff.toml` 一樣由範本管理）。🚫 不要在文件或對話裡重寫那段範圍計算 —— 每次照抄都是一次重新實作，
而重新實作會出現「看起來對、實際錯、且不報錯」的變體。

## 🔑 生成範圍：歸檔是**可選的修剪動作**，不是排程義務

**起點 = 歸檔截止月的下一個月 1 號**，從未歸檔就是完整歷史。有遷移前的手寫內容（歸檔檔裡的 slash 月標題）時，
再夾到遷移日以後 —— 遷移日＝`.cliff.toml` 第一次進版控的 commit 日期（腳本自己算）：

| 歸檔狀態 | 主檔起點 |
| --- | --- |
| 從未歸檔、沒有手寫歷史（含 `docs/history/archive/` 不存在） | 完整歷史（第一筆 commit 起） |
| 從未歸檔、只有遷移時搬入的手寫內容 | 遷移日 |
| 10/10 歸檔到 9 月 | `2026-10-01` |
| 11/05 歸檔到 10 月 | `2026-11-01` |

「什麼時候歸檔、要不要歸檔」完全自由。**覺得主檔太長就歸檔一次**（[`/git:changelog-monthly`](../changelog-monthly/SKILL.md)），
主檔下次生成立刻縮短；條目只取 commit 主旨、不含 body，累積幾個月也可以接受。

🔴 **判斷「已歸檔到哪個月」只認 dash（`## 📆 2026-09`），不認 slash（`## 📆 2026/09`）**：
dash ＝ git-cliff 生成的區塊，是「這個月被歸檔過」的唯一證據；slash ＝ 遷移時搬入的手寫內容。
認了 slash → 誤判遷移當月已歸檔 → 起點跳到下個月 → **遷移日之後的條目一筆都不生成，且沒有任何錯誤訊息**。
判斷依據是**歸檔檔的內容**，不是歸檔 commit 的時間（後者會被 rebase / 補提交打亂）。

## ⏱️ 生成時機：**預設不在每次 commit 時生成**

🚫 **`/git:commit` 的收尾不自己跑這支**，push 完直接結束、也不詢問；只有使用者主動要求時才跑。
生成是**全量覆寫且冪等**（來源永遠是 git log），晚生成、換人生成、由排程補齊都不影響正確性；
反之每筆變更都生成，`git log` 會被 `chore(changelog)` 淹沒（`.cliff.toml` 已把它們 `skip`，不會變成條目，只是 log 密度變差）。
本 repo 有沒有每日排程代為生成：見 custom 規則檔。

- **真的要生成時一定是「獨立一筆」**：條目行尾帶該 commit 自己的 hash → 生成時那筆必須已經存在 → 生成物只能落在下一筆；
  `--amend` 併回去也不行（hash 變了，連結指向不存在的 commit）。順序：本次變更一筆 → 生成 → `chore(changelog)` 一筆 → **一起 push 一次**
- **生成前先確認起點沒有落後** —— CI、排程、自動 pull 都可能在作業期間推進 `main` 且不會通知你（本 repo 的來源見 custom 規則檔），
  漏掉的那段 commit 不會有任何缺漏跡象：

  ```bash
  git fetch origin main
  git rev-list --count main..origin/main    ## 非 0 就先 pull --rebase 再生成
  ```

**被否決的替代方案**：commit 前先生成（只收錄到上一筆）、與本次變更同一筆送出 —— 永遠一筆 commit，
但「剛剛那筆不在 CHANGELOG 裡」會**每次都發生**，等於自造混淆。
git-cliff 的主流用法是 release / tag 時才生成；有版本 tag 的 repo 照 release 時生成即可，
沒有 tag 的 repo（月 / 日兩層分段）才改用「由人決定時機的批次生成」。

## 三條踩雷

1. 🔴 **`--before` 一定要帶 `00:00`** —— 純日期會被 git 的 approxidate 解析成「當天當下」，
   `--before=<今天>` 回傳 HEAD 自己 → range 變空 → **生成出只有檔頭的空 CHANGELOG，沒有任何錯誤訊息**
2. **`-o` 是全量覆寫，不是追加** —— 這正是它冪等、不產生併行衝突的原因，但也表示**手動改過的內容一律被沖掉**
3. **不要在編輯器裡開著 `CHANGELOG.md` 編輯** —— 未存檔的 buffer 會與生成結果互相覆蓋

## 🔴 改完 `.cliff.toml` 一定要用「回溯範圍」壓測

當前範圍（當月、可能只有幾個條目）踩不到其他批次才有的主旨形態（例：CI 產的主旨含裸 URL → `MD034`），
要等下次歸檔或跨月時才突然擋 push。

```bash
D=$(mktemp -d) && cp .markdownlint-cli2.jsonc "$D/"
git-cliff "$(git rev-list -1 --before='<兩三個月前>-01 00:00' HEAD)..HEAD" -o "$D/stress.md"
grep -c '^- ' "$D/stress.md"                    ## 條目數，太少代表 range 算錯
(cd "$D" && markdownlint-cli2 stress.md)        ## 要看到 `Linting: 1 file`
```

⚠️ 這裡裸跑 `markdownlint-cli2` 是**刻意的例外**（壓測檔在 repo 外，`lint.sh md` 掃不到）。
前提是用同一支 binary（cli2，不是 cli v1），且把 repo 的 `.markdownlint-cli2.jsonc` 複製過去 —— 缺一就是不同判準。

## 要改什麼，就去改哪裡

| 想改的東西 | 改哪裡 |
| --- | --- |
| 某條目的**文字內容** | 那個 commit 的**主旨**（body 不進 CHANGELOG）。🔴 已 push 的改不了 → 在後續 commit 補說明 |
| **格式 / 分類 / emoji、哪些 commit 要收錄** | `.cliff.toml`（`commit_parsers` 的 `skip = true` 可排除）—— 由 repo-template 管理，**改範本**再 `copier update`，改完重新生成 |
| 上個月以前的紀錄 | `docs/history/archive/<年>/CHANGELOG.md`，走 [`/git:changelog-monthly`](../changelog-monthly/SKILL.md) |

🔴 **改 `.cliff.toml` 前先讀它的檔頭**：列了實測踩雷，每一條的症狀都是「輸出照產、沒有錯誤」。
各 repo 的差異靠 `commit_parsers` 的 no-op 規則吸收 —— 🚫 **不要為了「這裡用不到」而刪掉某條規則**。

前置：`brew install git-cliff`（本機工具，**不進 pre-push 閘門** —— 其他 committer 沒裝也不受影響，
他們的 commit 會在下次有人生成時一併收錄）。

## 症狀：`CHANGELOG.md` 長期處於未提交狀態、但沒有人記得改過它

`generate.sh` 在 lint 失敗時**刻意不還原**生成物 → 排程每天生成完就中止、不 commit，兩側都不報錯。
先跑 `./scripts/changelog/generate.sh` 看它擋在哪，不要直接 `--fix`（CHANGELOG 是生成物，手改下次就沒了）：

- 🔴 **擋下的違規通常在與 CHANGELOG 無關的 md 檔** —— 它跑的是全量 `lint.sh md`，別先往 `.cliff.toml` 或生成範圍查
- 違規真的在 CHANGELOG（例：同一天兩個 `### 📅` 標題 → `MD024`）→ 對照 `.cliff.toml` 檔頭的踩雷

## 誰還會寫 CHANGELOG

**沒有人。** 本 repo 有哪些 bot / CI 會 commit：見 custom 規則檔。

🔴 **不要幫任何人加回手寫**：手寫條目下次生成即消失；而且插進生成檔會把檔頭的 HTML 註解**從中間切斷**，
`-->` 落到後面 → 以下整段被吞進註解，**markdownlint 一個錯都不會報**（內容被當註解就看不到了）。

## 🤖 bot 的 commit 主旨要不要加 `[skip ci]`

`.cliff.toml` 的 `commit_parsers` 會 `skip` 主旨以 `[skip ci]` 開頭的 commit —— 條目「就是沒出現」、沒有錯誤訊息，
而且事後補不回來（改 message ＝ 改寫歷史）。

🔑 **判準是「這個 repo 有沒有 push 觸發的 CI 要 skip」，不是「這是不是 bot commit」**：

- **沒有 push 觸發的 workflow**（全是 `workflow_dispatch` / `schedule`）→ 前綴是純損失，不要加
- **部署 repo 的 CI 版本更新 commit** → 那條規則正是為它們留的：這類紀錄本來就不該進 CHANGELOG
- ⚠️ 反向代價：拿掉前綴後，日後若加了 push 觸發的 workflow，bot 的 commit 也會觸發它

稽核要量輸出（腳本可能用變數組 message，grep 原始碼會漏）：`git log --since=30.days --format='%s' | grep -c '^\[skip ci\]'`

## 🔴 bot / CI 自己跑 git-cliff 之前

**淺 clone 會讓全量覆寫變成災難**：`git clone --depth 1`、`actions/checkout` 的預設 `fetch-depth: 1` 只有一個 commit 的歷史 →
`git-cliff -o CHANGELOG.md` 把整份 CHANGELOG 換成只剩那一筆，**沒有任何錯誤訊息**；bot 直推 `main` 又不經 pre-push。以下缺一不可：

1. 歷史要夠：`--shallow-since="$(date +%Y-%m-01)"` 以上，或 `fetch-depth: 0`；範圍計算還要讀 `docs/history/archive/`
2. 生成用的那筆 commit 要被 `commit_parsers` `skip`（`chore(changelog)` 已經是）
3. 🔴 **生成後不可 `git commit --amend`**（條目帶 commit hash）
4. 直推 `main` 不經 pre-push → 自己跑 `./scripts/lint/lint.sh md`
5. 用 `git diff --quiet` 判斷，無變更就別 commit（否則每次排程都留空 commit）

本機排程（例如 LaunchAgent）會走 pre-push 閘門、有完整 clone，所以是比 GitHub Actions 安全的選擇。
