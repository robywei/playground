# 併行 agent 共用工作區：只 commit 自己那份

同機常有多個 Claude session／agent、常駐 bot 或人同時改同一個 repo。本手冊處理「**工作區裡混著別人的變更，
我只要提交自己那幾個檔案**」，以及併發造成的幾種「看起來成功／失敗、其實相反」。

> 📎 本檔是 [`git:commit`](SKILL.md) 的參考檔：commit 流程中遇到併發狀況時才讀，處理完回到 `git:commit`。
> 由 repo-template 管理，**不要直接改這裡**；本 repo 踩過的實例記在
> [`.claude/custom/git-commit.md`](../../../../custom/git-commit.md)。
> 判讀原則（差異指令、截斷不是盤點）在每個 session 都載入的 `.claude/rules/common/evidence.md`，這裡只寫 git 上的形狀。

## 1. HEAD 會在回合之間前進 —— 先確認，不要憑記憶

對方在**同一個工作區** commit（甚至 push）後，你的 HEAD 跟著前進，而 `git status` **一片乾淨**，完全看不出來。
用差異指令定案：

```bash
git reflog show main --date=iso | head -5   ## 這輪有沒有別人的 commit 插進來（預覽用）
git log --oneline HEAD..origin/main         ## 回空＝沒落後（權威答案）
git log --oneline origin/main..HEAD         ## 回空＝沒領先
```

- ⚠️ 別用 `git log -3 origin/main` 反推落差（它永遠有輸出）；別用 `git rev-parse --short HEAD origin/main`
  （`--short` 只吃單一 revision → `fatal` + exit 128，看起來像沒差異，其實是沒結論）
- 💡 對方的 commit 落在你的變更之前是好事：照常 commit 自己的檔案即可，不需要 rebase、`pull`、`stash`。
  只有**改到同一個檔案**時，先 `git show <對方的 commit> -- <該檔>`，避免內容重複或矛盾

### 我自己的 commit「不見了」

session 跨越較久（隔天續談、對方累積幾十個 commit）時，自己那筆會被推出 `reflog -10`、`git log --all -3` 的視窗，
最容易誤判成「有人 force push 洗掉了」。🔴 `git cat-file -t <sha>` 只證明物件存在（被 reset 丟棄的 commit 在 gc 前一樣回 `commit`），
**「還在不在歷史上」只能用祖先判定**：

```bash
git merge-base --is-ancestor <sha> HEAD && echo '✅ 在歷史上' || echo '❌ 不在'
git branch --contains <sha>              ## 哪些分支含它
git log --oneline -1 -- <我改過的檔案>    ## 該檔最後被哪個 commit 動過
```

## 2. 先分類，再用 pathspec 只提交自己的

先跑 `git status --porcelain` + `git diff --cached --stat`，逐檔歸類：

| 狀態 | 意思 | 處理 |
| --- | --- | --- |
| `_M` 且不是我改的 | 別人只改了工作區 | **不碰**，pathspec 排除即可 |
| `M_` / `D_` 且不是我改的 | 別人**已經 staged** | **不碰**，而且 commit 一定要帶 pathspec（見下） |
| `MM` **且雙方都改** | 同一檔兩人各有 hunk | ⚠️ 最麻煩，見 §3 |

每筆 commit 都要動的手寫共用檔（例如手寫的 `CHANGELOG.md`）必然落在第三類；改成生成物後兩邊產出相同，就不再撞車。

⛔ `git add -- <我的檔案>` 之後裸跑 `git commit`，提交的是**整個 index** —— 別人 staged 的刪除／修改會被一起帶走。
**把 pathspec 帶到 `commit`**：

```bash
git commit -m "..." -- <我的檔案 1> <我的檔案 2> ...
```

`git commit -- <paths>` 只從**工作區**取這些路徑組 commit，index 其餘項目原封不動。**前提是這些檔案沒有別人的 hunk**（`MM` 不適用）。

## 3. 同一檔雙方都改（`MM`）：暫存 index 手術

`git commit -- <該檔>` 取的是工作區版本，會把對方的 hunk 一起帶走（產出「描述的變更不在這個 commit 裡」的不一致 commit）；
`git stash push --staged` 會破壞對方的 staging。正解：用 `GIT_INDEX_FILE` 開一份獨立 index，手工組 tree 再 `commit-tree`，
**真 index 與工作區全程不動**。以下用 `CHANGELOG.md` 當範例，換成實際撞到的檔即可：

```bash
set -e
D=<scratchpad>

## ① 組出「只含我的段落」的版本（HEAD 版本 + 只插入我那段）
git show HEAD:CHANGELOG.md > "$D/head.md"
head -<對方段落結束行> "$D/head.md"  > "$D/new.md"
sed -n '<我的起>,<我的迄>p' CHANGELOG.md >> "$D/new.md"      ## 從工作區抽我的段落
tail -n +<接續行> "$D/head.md"       >> "$D/new.md"
wc -l "$D/head.md" "$D/new.md" CHANGELOG.md                  ## 行數必須對得上

## ② 用獨立 index 組 tree
export GIT_INDEX_FILE="$D/tmp-index"; rm -f "$GIT_INDEX_FILE"
git read-tree HEAD
git add -- <我的其餘檔案...>
BLOB=$(git hash-object -w "$D/new.md")
git update-index --cacheinfo 100644,"$BLOB",CHANGELOG.md
TREE=$(git write-tree)
unset GIT_INDEX_FILE

## ③ 建 commit 並推進 branch（帶舊值防競態）
COMMIT=$(git commit-tree "$TREE" -p HEAD -F "$D/msg.txt")
git show --stat --format='%s' "$COMMIT"                      ## 驗證：檔案數與行數
git update-ref refs/heads/main "$COMMIT" "$(git rev-parse HEAD)"

## ④ 收尾：把我那些路徑的真 index 對齊工作區（不可省）
git add -- <我的全部檔案，含 CHANGELOG.md>
git status --porcelain && git diff --cached --stat            ## 對方的 staged 應原樣還在
```

🔴 **③ 與 ④ 必須在同一個 shell 區塊跑完，中間不可有任何等待**：`update-ref` 只動 branch ref，真 index 裡我那些路徑
仍停在舊 HEAD → 相對新 HEAD 成了「staged 的反向修改」。對方若在空檔跑了不帶 pathspec 的 `git commit`，
**我剛推進的 commit 就被整批 revert，兩邊都成功、沒有衝突、沒有警告**。④ 之後這些路徑 index == 工作區 == 新 HEAD，
`CHANGELOG.md` 則只剩對方那段 staged —— 正是對方要的狀態。

- `commit-tree` **完全跳過 hooks** → 交付前自己先跑 `./scripts/lint/lint.sh`
- 手術前看 `git log -3 --format='%h %ci %an %s'`：最後一個 commit 是幾分鐘前 → 對方**還活著**，`update-ref` 的舊值參數必帶
- 別人的檔案**一個都不要 lint --fix / 格式化**，那會製造出「我改的」假象

## 4. 我的變更被別人收走或 revert 了

症狀相同：**我剛編輯過的檔案，`git status` 卻乾淨**。三種解釋要用內容特徵字串分辨，不能看 status：

| 解釋 | 怎麼分辨 |
| --- | --- |
| 我的編輯根本沒生效 | 工作區內容**沒有**我加的特徵字串 |
| 生效了但被對方 `checkout --` / `restore` 還原 | 工作區沒有，HEAD 也沒有 |
| **生效了，已被對方不帶 pathspec 的 commit 收走** | 工作區**有**，`git show HEAD:<檔案>` 也有 |

```bash
grep -c '<我加的特徵字串>' <檔案>                      ## 先看內容在不在
git log --oneline -S'<我加的特徵字串>' -- <檔案>       ## 在哪個 commit 進去的
```

🔴 **最貴的誤判是「以為改檔腳本寫失敗了」再跑一次** —— 錨點還在，於是插入**第二份**，而只驗錨點次數的保護抓不到重複插入。

- **已被收走**：不要「重新提交」已在 HEAD 裡的那段 —— 它掛在不相關的 commit message 下且無法補救，要留痕就寫進自己下一筆的 body；
  剩下的檔案照常用 pathspec commit。對方還活著時，後續 commit 一律帶 pathspec，並在同一次工具呼叫內 commit + push
- **被 revert 後要復原**：`git commit -- <paths>` 的來源是**工作區**，而工作區可能已被對方還原成舊版 → 再 commit 一次只會二度 revert 自己。
  **指名來源 commit**，同一個區塊內驗證完再提交：

  ```bash
  set -e
  git checkout <我原本那個 sha> -- <我的檔案...>       ## 指名來源，不依賴工作區
  grep -c '<我加的特徵字串>' <我的檔案>                 ## 驗證真的取回來了，沒中就 exit
  git commit -F "<訊息檔>" -- <我的檔案...>
  git show HEAD:<我的檔案> | grep -c '<特徵字串>'      ## 驗證 HEAD 真的有
  git push                                          ## 立刻推走，內容不再受本機工作區影響
  ```

## 5. push 的結果與閘門：訊息常和事實相反

同一條分支只能整條推，所以**對方的 push 會連你已建好的 commit 一起推上去**。push 自己的訊息因此兩個方向都會騙人：

| 訊息 | 可能的真相 |
| --- | --- |
| `[remote rejected] … cannot lock ref … is at <X> but expected <Y>` | `X` 就是我的 commit → **已經上去了**，是競態不是失敗；`X` 是別人的、我的不在其中才要處理 |
| `Everything up-to-date` | commit 根本沒建起來（`git add` fatal，見 `git:commit` 注意事項）→ ❌ 什麼都沒進去；**或** commit 已建、被對方的 push 帶走 → ✅ 已經上去了 |

🚫 不要照直覺重跑 push、更不要 `--force`。一律用差異指令＋內容特徵定案：

```bash
git fetch origin main -q
git log --oneline HEAD..origin/main        ## 回空＝沒落後
git log --oneline origin/main..HEAD        ## 回空＝沒領先（兩個都空就是一致）
git branch -r --contains <我的 SHA>         ## 應列出 origin/main
git show origin/main:<我的檔案> | grep -c '<我加的特徵字串>'   ## 遠端內容確認
```

### pre-push 擋下，而失敗的檢查不是我改的

pre-push 跑的通常是**全 repo 的檢查**，不是只掃你的 diff → 別人未完成的工作（例：新增了互相依賴的檔案、只建了一半）
會擋住你的 push，訊息指向你沒碰過的檔案。另一種與檔案無關：對方正在跑**同一個獨佔 cache 的工具**，
錯誤長得像掃描失敗，上面幾行卻是 `lock` / `cache` / `timeout` —— 它一個問題都沒掃到。

1. 🚫 **不要為了解鎖去改對方的檔案**（會把對方領域的變更掛到你的 commit 上），也**不要 `--no-verify`**
   （這次沒驗到的東西，之後也不會有人補驗）
2. ✅ **先 commit 自己那幾個檔案**（pathspec）—— commit 不跑 pre-push，變更不會因為等待而遺失
3. ✅ 等閘門自己通（有子檢查就只重跑失敗的那個），通了再 push：

   ```bash
   until ./scripts/lint/lint.sh <失敗的那個子檢查> > /dev/null 2>&1; do sleep 30; done
   ```

- ⚠️ 等待期間對方的 commit 會疊在你上面，你的 push 會連它一起推 —— **不要為此 rebase / reorder**；
  推之前確認對方那筆是完成品（message 完整、閘門全綠），回報時說明它一起上去了。
  反過來，你等待時對方的 push 也可能已帶走你的 commit → 重試看到 `Everything up-to-date` 時用上面的差異指令定案
- 💡 反過來也成立：你自己未提交的半成品也會擋住對方 → 互相依賴的檔案在同一筆提交，不要讓它停在工作區

## 6. 絕不做的事

`.claude/rules/common/git.md` 已禁的（`git add -A` / `.`、`commit -a`、`stash`、`reset`、`clean`、`checkout .`）照守，另外：

- 🔴 **「先跑 `git status` 確認都是我的，再批次 add」不是許可證** —— 確認與提交之間必有時間差（例如中間跑了數分鐘的 lint），
  對方在空窗寫入的變更會被你提交並 push。**唯一有效的是把 pathspec 帶到 `commit`**（§2），中間隔多久都不影響
- 幫對方一起 commit —— 對方的變更常只 staged 一半（例如刪除已 staged、修改還在工作區），連帶提交會產生半套的不一致 commit
