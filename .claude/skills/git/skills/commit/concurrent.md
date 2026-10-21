# 併行 agent 共用工作區：只 commit 自己那份

本機常有多個 Claude session／agent 同時改同一個 repo（tmux 裡的多個 session、常駐 bot、使用者手動開的 `/git` agent，
也包括同機的人）。本手冊處理「**工作區裡混著別人的變更，我只要提交自己那 N 個檔案**」。

> 📎 本檔是 [`git:commit`](SKILL.md) 的參考檔：commit 流程中遇到併發狀況時才讀，處理完回到 `git:commit`。
> 由 repo-template 管理，**不要直接改這裡**；本 repo 踩過的實例記在
> [`.claude/custom/git-commit.md`](../../../../custom/git-commit.md)。

## 0. 先看「對方是不是已經 commit 了」—— 你的 HEAD 會在回合之間前進

比工作區混雜更容易誤判的一種：**對方那輪已經 commit + push 完了**。
此時 `git status` **一片乾淨**（只有你自己的變更），完全看不出 HEAD 動過 ——
但你這輪開頭記得的那個 commit 已經不是 HEAD 了。

實例：session 開頭記下 HEAD，中途另一個 session 在**同一個工作區** commit 並 push，
於是我的 HEAD 也跟著前進。我卻據 `git log -3 origin/main` 的輸出判定「本機落後遠端一個 commit」，
白跑了一輪 `stash` + `pull --rebase` + `stash pop`。

```bash
git reflog show main --date=iso | head -5   ## 這輪有沒有別人的 commit 插進來
git log --oneline HEAD..origin/main         ## 回空＝沒落後（權威答案）
git log --oneline origin/main..HEAD         ## 回空＝沒領先
```

⚠️ **別用 `git log -3 origin/main` 反推落差**（它永遠有輸出，只是列歷史），
也**別用 `git rev-parse --short HEAD origin/main`**（`--short` 只吃單一 revision，
會 `fatal: Needed a single revision` + exit 128 → 看起來像沒差異，其實是沒結論）。

💡 **對方的 commit 落在你這輪的變更之前，是好事**：你只要正常 commit 自己那幾個檔案，
不需要 rebase、也不需要處理 index。真正要小心的是**你們改到同一個檔案** ——
此時先 `git show <對方的 commit> -- <該檔>` 看它寫了什麼，避免你的內容與它重複或矛盾。

## 0.1 我自己的 commit 被時間推出 reflog 視窗，看起來像被洗掉

§0 的指令用 `head -5` 是因為它問的是「**這輪**有沒有人插進來」。但 session 若跨越
較長時間（隔天續談、或對方累積了幾十個 commit），**你自己那筆會被推出前 N 名** ——
於是 `git reflog -10` 與 `git log --all -3` 都看不到它。

這時最容易得出的錯誤結論是「**有人 force push 把我的成果洗掉了**」，而且下一步
（`git cat-file -t <sha>` 回 `commit`）會讓人更確信：**物件明明還在、列表卻沒有
→ 一定是變成懸空 commit 了**。

🔴 **`git cat-file -t` 只證明物件存在，不證明它在任何歷史上** ——
被 reset 丟棄的 commit 在 gc 之前一樣回 `commit`。它分不出「還在 main 上」與「已被丟棄」。

**判準：「這個 commit 還在不在某個 ref 的歷史上」只能用祖先判定，不能看列表。**

```bash
git merge-base --is-ancestor <sha> HEAD && echo '✅ 在歷史上' || echo '❌ 不在'
git branch --contains <sha>              ## 哪些分支含它
git log --oneline -1 -- <我改過的檔案>    ## 該檔最後被哪個 commit 動過
```

實例：距離自己前一輪 commit 已過**兩天**、期間累積 20+ 個 commit，`reflog -10` 與 `git log --all -3`
都看不到那兩筆，於是**連續兩次**判定成「歷史被 reset、成果消失」。`merge-base --is-ancestor` 一問，
**全都在 main 上**，什麼都沒丟。

⚠️ 這是「拿截斷輸出當不存在」與「有沒有差異只能用差異指令回答」在 git 上的形狀
—— `-N` / `head -N` 用在**盤點**而不是預覽時，就是在製造假陰性。

## 0.2 `[remote rejected]` 不等於沒推上去 —— 先讀錯誤訊息裡的 SHA

共用工作區時，對方的 `git push` 會**連你剛做好的 commit 一起推上去**（同一條分支只能整條推）。
於是你自己那次 push 撞上 ref 已被更新，git 回：

```text
! [remote rejected] main -> main (cannot lock ref 'refs/heads/main':
  is at <X>... but expected <Y>...)
```

🔑 **判準：`is at <X>` 的那個 X 是誰的 commit？**

- **X 就是我這次的 commit** → **已經上去了**，這是競態不是失敗，什麼都不用做
- X 是別人的、我的不在其中 → 才是真的要處理（`fetch` 後看落差）

⚠️ **不要照直覺重跑 `push`、更不要 `--force`** —— 前者多半回 `Everything up-to-date`
（對，但會讓人以為前一次真的失敗過），後者在「其實已經成功」的情況下純粹是拿別人的
commit 冒險。一律用權威指令確認：

```bash
git fetch origin main -q
git log --oneline HEAD..origin/main        ## 回空＝沒落後
git log --oneline origin/main..HEAD        ## 回空＝沒領先（兩個都空就是完全一致）
git branch -r --contains <我的 SHA>         ## 應該列出 origin/main
```

實測：push 印 `[remote rejected]` ＋ `error: failed to push some refs`，**兩個非零訊號**，
但 `is at` 的 SHA 正是我 3 秒前建的那個 commit，遠端與本機完全一致。
—— 這與 [`git:commit`](SKILL.md) 記的 `Everything up-to-date` 是**同一枚硬幣的兩面**：
那邊是「看起來成功、其實沒進去」，這邊是「看起來失敗、其實已進去」。
**兩種都只能靠 `HEAD..origin/main` 這類差異指令定案，不能靠 push 自己的訊息。**

## 0.3 反向的那一種：對方的 commit 把我還沒 commit 的變更收走了

§0 講的是「對方的 commit 落在我的變更之前」。還有一種方向相反、症狀更容易誤判：
對方那輪的 `git commit` **沒有帶 pathspec**（或 `-a`），把我還在工作區的 hunk
**一起提交並 push 掉了**。

症狀：**我剛編輯過的檔案，`git status` 卻乾淨**。三種解釋長得一模一樣：

| 解釋 | 怎麼分辨 |
| --- | --- |
| 我的編輯根本沒生效 | 檔案內容**沒有**我加的特徵字串 |
| 生效了但被對方 `checkout --` 還原（§2.5） | 內容沒有特徵字串，且 HEAD 也沒有 |
| **生效了，而且已被對方 commit 走** | 內容**有**特徵字串，`git show HEAD:<檔案>` 也有 |

```bash
grep -c '<我加的特徵字串>' <檔案>                      ## 先看內容在不在
git log --oneline -S'<我加的特徵字串>' -- <檔案>       ## 在哪個 commit 進去的（權威答案）
```

🔴 **最貴的誤判是「以為改檔腳本寫失敗了」再跑一次** —— 錨點還在，
於是插入**第二份**內容，而 `assert count == 1` 那種保護只驗錨點、驗不到重複插入。

處置（**不要試圖「重新提交」已經在 HEAD 裡的那段**）：

1. 內容已經在 origin/main 上，且掛在**不相關的 commit message** 底下 —— 這無法補救，
   `git log -S` 的結果就是唯一的歸屬紀錄。要留痕就寫進自己下一個 commit 的 message
2. **剩下的檔案照常 commit**（用 pathspec）
3. 若對方 agent 還活著（`git log -1 --format=%ci` 是幾分鐘內），
   後續 commit 一律帶 pathspec，並在同一次工具呼叫內 commit + push

⚠️ 這是「被中斷／被拒絕之後先 `git status`」的**例外**：在共用工作區裡，
`git status` 乾淨**不能**當成「沒寫進去」的證據。

## 先分類：三種混雜程度

先跑 `git status --porcelain` + `git diff --cached --stat`，逐檔歸類：

| 狀態 | 意思 | 處理 |
| --- | --- | --- |
| `_M` 且不是我改的 | 別人只改了工作區 | **不碰**，用 pathspec 排除即可 |
| `M_` / `D_` 且不是我改的 | 別人**已經 staged** | **不碰**，見下方「不可污染 index」 |
| `MM` **且雙方都改** | 同一檔兩人各有 hunk | ⚠️ 最麻煩，見 §2 |

> 💡 手寫的共用檔（例如手寫的 `CHANGELOG.md`）幾乎必然落在第三類 ——
> 每筆 commit 都要動它。改成生成物（git-cliff）之後，兩個 agent 跑出來的內容位元組相同，就不再撞車。

## 1. 單純情況：pathspec 就夠

```bash
git add -- <我的檔案 1> <我的檔案 2> ...
git commit -m "..."          ## commit 會帶上 index 裡「別人 staged 的東西」！
```

⛔ **上面那行是錯的**。`git commit`（不帶 pathspec）提交的是**整個 index**，
別人 staged 的刪除／修改會被一起帶走。正確寫法是**把 pathspec 帶到 `commit`**：

```bash
git commit -m "..." -- <我的檔案 1> <我的檔案 2> ...
```

`git commit -- <paths>` 只從**工作區**取這些路徑組 commit，index 其餘項目原封不動。
**前提是這些檔案沒有別人的 hunk**（第三類不適用）。

## 2. 同一檔雙方都改（`MM`）：暫存 index 手術

> ℹ️ 下面用 `CHANGELOG.md` 當範例，因為手寫時代它是最常撞車的檔。
> **手術方法對任何雙方都改的檔案都適用** —— 把 `CHANGELOG.md` 換成你實際撞到的那個檔即可。

`git commit -- CHANGELOG.md` 取的是**工作區版本**，會把對方的條目一起帶走 →
產出「條目描述的程式碼變更不在這個 commit 裡」的不一致 commit。
`git stash push --staged` 則會**破壞對方正在進行的 staging**，同樣不可用。

正解：用 `GIT_INDEX_FILE` 開一份獨立 index，手工組出 tree 再 `commit-tree`，
**真 index 與工作區全程不動**。

```bash
D=<scratchpad>

## ① 先組出「只含我的條目」的 CHANGELOG（HEAD 版本 + 只插入我那段）
git show HEAD:CHANGELOG.md > "$D/head.md"
head -<對方條目結束行> "$D/head.md"  > "$D/new.md"
sed -n '<我的起>,<我的迄>p' CHANGELOG.md >> "$D/new.md"      ## 從工作區抽我的條目
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

## ④ ⚠️ 收尾：把我那些路徑的真 index 對齊工作區（不可省，見下）
git add -- <我的全部檔案，含 CHANGELOG.md>
git status --porcelain && git diff --cached --stat            ## 對方的 staged 應原樣還在
```

### 🔴 ③ 與 ④ 必須在**同一次操作內**跑完，中間不可有任何等待

實證：③ 與 ④ 拆成兩次工具呼叫，中間隔了幾十秒 —— 對方 agent 剛好在那個空檔跑了不帶 pathspec 的 `git commit`，
把「index 裡仍停在舊 HEAD 的我的路徑」當成修改寫進它的 tree，**我剛推進的 commit 被整批 revert**。
兩邊的 commit 都成功、沒有衝突、沒有警告。

把 ③④ 併成一個 shell 區塊（`set -e` + 驗證），不要拆開。

`update-ref` 只動 branch ref。真 index 裡我那些路徑**仍停在舊 HEAD 的內容** →
相對新 HEAD 就成了「**staged 的反向修改**」，對方下一次 `git commit` 直接把我剛推的
變更整批 revert 回去，**沒有衝突、沒有警告**。

`git add` 之後這些路徑的 index == 工作區 == 新 HEAD → status 乾淨。
`CHANGELOG.md` 則變成「index 相對新 HEAD 只多對方那段」，正是對方要的狀態。

### 其他注意

- `commit-tree` **完全跳過 hooks** —— push 時 pre-push 照常執行，但交付前仍要自己先跑
  `./scripts/lint/lint.sh`（本 repo 的檢查項見該腳本檔頭）
- 手術前先看 `git log -3 --format='%h %ci %an %s'` 對比現在時間 ——
  若最後一個 commit 是幾分鐘前，代表對方 agent **還活著**，`update-ref` 的舊值參數必帶
- 別人的檔案**一個都不要 lint --fix / 格式化**，那會製造出「我改的」假象

## 2.5 復原自己被 revert 的內容：來源必須指名 commit

被 §2 那個時間差咬到之後，直覺是「工作區還是我的新版，再 commit 一次就好」——**錯**。

`git commit -- <paths>` 的內容來源是**工作區**（不是 index、也不是任何 commit），
而併行 agent 隨時可能 `git checkout -- .` / `git restore` 把工作區還原成 HEAD。
實測就這樣**二度 revert 了自己**：第一次補救的 commit 從工作區取內容，
而工作區在那之前已被對方還原 → 提交上去的又是舊版，diff 是減行而不是加行。

正解是**指名來源 commit**，並在同一個區塊內驗證完再提交：

```bash
set -e
git checkout <我原本那個 sha> -- <我的檔案...>       ## 指名來源，不依賴工作區
grep -c '<我加的特徵字串>' <我的檔案>                 ## 驗證真的取回來了，沒中就 exit
git commit -F - -- <我的檔案...> <<'EOF'
...
EOF
git show HEAD:<我的檔案> | grep -c '<特徵字串>'      ## 驗證 HEAD 真的有
git push                                          ## 立刻推走，內容不再受本機工作區影響
```

⚠️ **驗證一律用「內容特徵字串」，不要看 `git status` 乾不乾淨** ——
被 revert 之後工作區與 HEAD 是一致的，status 完全乾淨，看起來像成功。

## 2.9 push 被擋，而失敗的檢查不是我改的東西

pre-push 跑的通常是 **`lint.sh` 的全 repo 閘門，不是只掃你的 diff**。
所以**別人未完成的工作會擋住你的 push**，而訊息指向一個你根本沒碰過的檔案。

症狀：自己的檔案都已 commit、工作區乾淨（或只剩對方的），但 `git push` 被 pre-push 擋下，
且失敗的那一項與你這輪無關。

實例：對方 agent 三分鐘前才新增一支 skill（`SKILL.md` 還是 untracked），而檢查項比對的是
`plugin.json` 的 commit 時間 —— 未提交的新 skill 一律當「現在」→ 在對方把兩者一起提交之前，這項必然失敗。

處置：

1. 🚫 **不要為了解鎖去改對方的檔案**（連 `plugin.json` 補一下都不要）——
   那會把對方領域的變更掛到你的 commit 上，也違反本手冊「別人的檔案一個都不要碰」
2. ✅ **先 commit 自己那幾個檔案**（`git commit -- <paths>`）。commit **不跑**
   pre-push 閘門，所以這步不會被擋，變更也就不會因為等待而遺失
3. ✅ 背景等閘門自己通，通了再 push（用**出問題的那個子檢查**，不是整份 lint）：

   ```bash
   until ./scripts/lint/lint.sh <失敗的那個子檢查> > /dev/null 2>&1; do sleep 30; done
   ```

⚠️ **預期對方的 commit 會疊在你的上面** —— 於是你的 `git push` 會**連他們那個一起推**。
共用分支只能整條推，**不要為此去 rebase / reorder**（那才是真的動到對方的東西）。
推之前確認對方那個是**完成品**（commit message 完整、閘門全綠），並在回報時說明它一起上去了。

💡 反過來也成立：**你自己未提交的新 skill 會擋住對方的 push。**
新增 skill 時把 `SKILL.md` 與該 domain 的 `plugin.json` **在同一個 commit 裡**提交，別讓它在工作區停留。

### 等閘門的那段時間裡，對方的 push 會把你的 commit 一起帶走

上面第 2 步（先 commit、再等閘門）有個必然的副作用：**你的 commit 已經在共用分支上了**。
於是對方在你等待期間 push，會連你那筆一起推上去 —— 等你重試時，`git push` 回 **`Everything up-to-date`**。

🔴 **這句話在本手冊裡有兩種相反的含義，不能靠它自己判斷**：

| 情境 | `Everything up-to-date` 的意思 |
| --- | --- |
| [`git:commit`](SKILL.md) 記的那種（commit 根本沒建起來） | ❌ 一個字都沒進去 |
| **本節這種**（commit 已建、被對方的 push 帶走） | ✅ 已經上去了 |

**一律用差異指令 ＋ 內容特徵定案**，不要重跑 push、更不要 `--force`：

```bash
git fetch origin main -q
git log --oneline HEAD..origin/main        ## 回空＝沒落後
git log --oneline origin/main..HEAD        ## 回空＝沒領先（兩個都空就是一致）
git branch -r --contains <我的 SHA>         ## 應列出 origin/main
git show origin/main:<我的檔案> | grep -c '<我加的特徵字串>'   ## 遠端內容確認
```

實測：pre-push 被擋 → 等閘門 → 重試回 `Everything up-to-date`。查證後對方的 commit 比我的**早 6 秒**，
它那次 push 把我的一起帶了上去，遠端內容完整。**這是成功，不需要任何補救動作。**

### 閘門失敗的第二種「不是我的問題」：併行 agent 佔住工具的 cache lock

上面講的是**別人的檔案**讓閘門失敗。還有一種與檔案無關：
**對方 agent 正在跑同一個工具**，而該工具的 cache 是獨佔的（例如 trivy）：

```text
ERROR  Failed to acquire cache or database lock
FATAL  unable to initialize fs cache: cache may be in use by another process: timeout
```

⚠️ **閘門的訊息長得像「掃描發現問題」，但它一個問題都沒掃到** —— 是連 cache 都沒初始化成功。
判準：看錯誤上面那幾行是 **scan 結果**還是 **lock / cache / timeout**。

處置與上面第 3 步相同（等，別 `--no-verify`）—— 對方跑完就好，通常一兩分鐘內：

```bash
for i in $(seq 1 20); do
  ./scripts/lint/lint.sh <該工具的子檢查> >/dev/null 2>&1 && { echo "✅ 第 $i 次通過"; break; }
  sleep 20
done
```

🚫 **不要因為「反正不是我的問題」就 `git push --no-verify`** —— 閘門這次確實沒驗到東西，
下次也就不會有人補驗。等待成本是幾十秒，跳過的代價是一次真的沒被掃過的 push。

## 3. 絕不做的事

- `git add -A` / `git add .` / `git commit -a`
  - 🔴 **「我先跑過 `git status --short`、確認都是我的檔案」不是許可證**。
    實例的順序是：`git status` 確認乾淨 → 跑 `lint.sh`（**數分鐘**）→ `git add -A` → commit。
    對方 agent 在 lint 那段空窗寫入，與我的 commit **只相隔 16 秒**，於是它的變更被我提交並 push。
    ⚠️ 護欄的主詞是**結果**不是動作：任何「先確認、後批次 add」的形式都擋不住，
    因為確認與提交之間必然有時間差。**唯一有效的是把 pathspec 帶到 `commit`**（§1），
    它取的是那幾條路徑、與工作區其餘變動無關，中間隔多久都不影響
- `git stash`（任何形式）—— 會捲走對方的工作區變更
- `git reset` —— 會清掉對方的 staging
- 幫對方一起 commit —— 對方的變更常只 staged 一半（例如刪除已 staged、修改還在工作區），
  連帶提交會產生半套的不一致 commit
