---
name: commit
description: "Git 提交流程（**git domain 的預設情境**）：分析變更 → Code Review → 更新 README / CLAUDE → commit → push（不生成 CHANGELOG、也不詢問）。觸發：commit、提交、push、幫我送出變更、一筆還是拆多筆、distill 產出要不要併回、同一件事拆成好幾筆、`git status` 出現不是我這輪改的檔案、併行 agent 提交衝突、HEAD 跟這輪開頭記的不一樣、`[remote rejected]` cannot lock ref、pre-push 擋下但失敗的檢查不是我改的；沒有其他明確情境時一律走這支。"
---

# Git 提交流程

執行完整的 Git 提交流程，包含文檔更新。**這是 `git` domain 的預設情境**（`/git:commit`）。
**不要詢問用戶要做哪件事，直接執行提交流程。**

> 🔁 本檔由 repo-template 管理：要改共用流程請改範本、發版後 `copier update`，**不要直接改這裡**
> （直接改會被當成本地客製保留下來，從此和其他 repo 分岔）。只屬於本 repo 的規則寫在
> [`.claude/custom/git-commit.md`](../../../../custom/git-commit.md)。

## 檔案說明

- **README.md**：Repo 說明文件（人類優先閱讀）
- **CLAUDE.md**：AI 運作時參考的說明（每次對話全文載入）

## 步驟 0：讀本 repo 專屬規則

先讀 [`.claude/custom/git-commit.md`](../../../../custom/git-commit.md)，其中的規則與下列共用步驟**一起**適用。
檔案不存在時**明確回報「找不到本 repo 的 custom 規則檔」**，不要靜默略過。

## 執行步驟

1. **分析變更並確認範圍**（`git status`、`git diff`）
   - 本機常有多個 agent 或人同時改同一個 repo → **commit 前一定要跑**：

     ```bash
     git status --porcelain          ## 每一項都是自己這輪改的嗎？
     git diff --cached --stat        ## 已 staged 的也要看（別人可能先 add 了）
     ```

   - 出現「你不記得改過」的項目就**停下來問**：不要默默包進 commit（別人的變更會記在你的 commit message 下），
     也不要逕自還原或 `git clean`（可能刪掉對方正在做的事）。併發來源不限 Claude session，同機的人、部署 bot 都算
   - ⚠️ 併發時不要用 `git stash`：會破壞對方的 staging；完整處理流程見 [`concurrent.md`](concurrent.md)
2. **Code Review**（根據變更規模調整深度）
   - **檢查項目**：
     - 🔒 安全：硬編碼密碼/token、SQL injection、command injection、敏感資訊外洩
     - 🐛 邏輯：空值處理、邊界條件、錯誤處理、race condition
     - ⚡ 效能：N+1 查詢、無限迴圈風險、記憶體洩漏
     - 📝 風格：命名一致性、死碼、過度複雜
     - 📌 本 repo 特有的高頻缺陷：見 custom 規則檔（步驟 0）
   - **處理方式**：小型（< 50 行）直接檢查；中型（50–200 行）逐檔案檢查；大型（> 200 行）啟動獨立的 review 子 agent
   - **阻擋條件**（發現以下問題必須修復後才能提交）：
     - 硬編碼的密碼、API key、token（guard 生效時 diff 裡的真機密顯示成遮蔽標記 `<redacted:FAMILY len=N>` ——
       **看到標記＝命中**，不是「已處理」：檔案裡是真值）
     - 明顯的安全漏洞
     - 會導致服務中斷的邏輯錯誤
3. **根據變更內容更新相關文檔**：
   - 新功能或重大變更 → 更新 `README.md`
   - 影響 AI 操作的變更 → 更新 `CLAUDE.md`
     - **自動精簡分析**（root `CLAUDE.md` **每次對話全文載入**，長度＝所有對話的固定成本）：
       - 移除過時範例、把重複的配置範例改為簡短參考
       - 移除已退役環境的詳細說明（保留列表即可）
       - 砍掉踩雷的編年史敘事（哪天、哪支腳本），**保留規則與失敗症狀**
       - **root 目標 ≤ 250 行**（以 `wc -l CLAUDE.md` 實查）。超過就把細節外放到該目錄 `README.md` 或對應 skill，
         **不要兩邊各寫一份**（必然漂移）
       - 落點判準見 [`/archive:distill`](../../../archive/skills/distill/SKILL.md) 的 `claude-md.md`
       - ⚠️ 外放前先確認落點真的存在，否則是「刪掉」不是「外放」
   - 本 repo 的文件落點（哪些目錄有 `README.md`、`docs/` 怎麼分）：見 custom 規則檔
4. **建立 Git commit**：
   - 主旨用 Conventional Commits（`type(scope): 描述`），**主旨必須能獨立讀懂**
   - **body 寫清單**：細節留在 git，`git log --grep=<關鍵字>` / `-S` 搜得到
   - 🔴 **寫錯無法事後修**（push 後改 message ＝ 改寫歷史，不做，理由見「注意事項」的機密那條），
     只能在後續 commit 補說明
   - 🔴 **主旨與 body 之間必須空一行** —— 少了它 git 把**整段**（含 body 清單）當成主旨。
     `git commit -F - <<'EOF'` 時最容易漏 → commit 後先 `git log -1 --format=%s` 確認只有一行，
     **push 前**發現還能 `--amend`
5. **推送到遠端**。`pre-push` hook 會跑 `scripts/lint/lint.sh`（本 repo 的檢查項見該腳本檔頭）
   - 🚫 **被擋就修到過再推，不用跳過旗標**（含擋下原因與本次無關時）
   - 格式問題可用 `./scripts/lint/lint.sh md --fix` 自動修，但 ⚠️ **`--fix` 會靜默改壞內容，跑完必須 `git diff`**
   - push 後用 `git ls-remote origin refs/heads/main` 核對 SHA，**不要只信 `Everything up-to-date`**
   - 非 `/git` 觸發的事件中途 commit 可延後到收尾再推，見下方「一筆還是拆多筆」
6. **宣告完成前，三件事都要查**（少查任何一件，「已完成」就可能是假的）：

   ```bash
   git status --porcelain                 ## 必須是空的
   git log --oneline origin/main..HEAD    ## 必須是空的（沒有未推的 commit）
   git stash list                         ## 必須是空的
   ```

   - 🔴 **只核對 SHA 不夠**：SHA 一致只證明 commit 推上去了，不證明工作區沒有殘留
   - ⚠️ **`git stash` 不是「丟棄」、是「暫存待取回」**：要丟掉改壞的檔案用 `git restore <file>`；
     真的用了 stash 就要收乾淨（`git stash pop` 或 `git stash drop`）

## 一筆還是拆多筆

判準不是檔案數、也不是涉及幾個目錄，而是依序問兩題：

1. **原子性 —— 缺任何一部分，剩下的會不會壞？** 會壞 → **一筆**
2. **同一件事 —— 在 CHANGELOG 上是不是同一條？** 是 → **一筆**，事後補的文件 / skill / distill 產出
   **併回它補充的那筆**，不另開。不是 → 拆，讓每筆各自能 revert、各自有正確的 type 分類
   （動到產線的變更通常值得獨立一筆）

文件永遠「不會壞」，只問第 1 題就會一律拆 —— 第 2 題是為它補的。
⚠️ 「同一件事」**不是**「同一個來源」：同一份報告的兩個發現仍是兩筆（一個已處置、一個待決時，併在一起會讓
CHANGELOG 看不到後者，revert 前者也會連帶刪掉後者）。

### 併回去的做法（只限**還沒 push** 的那筆）

```bash
git log --oneline origin/main..HEAD                ## 先確認目標在這個範圍內、且範圍內每一筆都是自己這輪的
git commit --amend --no-edit                        ## 目標就是 HEAD
git commit --fixup=<sha> && git rebase --autosquash --autostash origin/main  ## 目標不是 HEAD
```

`--autosquash` 不帶 `-i` 需 git ≥ 2.44。

- 🔴 **已 push 就不併** —— 那是改寫已推送的歷史；
  只能獨立一筆，主旨寫明它補的是哪件事
- 🔴 **範圍內夾著別的 agent 的 commit 就不 rebase** —— 會改掉它的 hash，對方下一步會以為 HEAD 被動過
  （見 [`concurrent.md`](concurrent.md)）→ 改為獨立一筆
- 併完 body 要補上新增的那幾項（`--amend` 時去掉 `--no-edit`；fixup 會被 squash 掉 message，事後 `--amend` 補）

### 為了能併回去：非 `/git` 觸發的 commit 可以先不 push

事件處置中途自發的 commit（例：套用線上變更後立即入庫）**先只 commit**，push 留到對話收尾 ——
收尾若跑 [`/archive:distill`](../../../archive/skills/distill/SKILL.md)，它的產出才併得回去。

- 使用者叫 `/git` → 照常 commit + push（明確要求，不延後）
- 常駐 agent 依它自己的 playbook，通常一律 commit + push，不適用延後
- 🔴 延後的代價是「忘了推就遺失」→ 收尾前**必跑** `git log --oneline origin/main..HEAD`，非空就推

## 注意事項

- Commit message 使用**繁體中文**
- 確保不提交任何機密資訊（密碼、金鑰等）
  - 🔴 **機密已經 commit 進去（尤其已 push）→ 立刻輪換那組憑證，不要靠改寫歷史清除**：既有的 clone、fork、CI 快取都還留著，
    GitHub 上的舊 commit 在 GC 前仍可用網址直接打開 —— 改寫擋不住已外洩的副本，只會讓 CHANGELOG、文件裡引用的 SHA
    全部指向不存在的 commit，而且不報錯。服務已退場時，改為確認那組密碼沒有被複用到別處
  - 處置紀錄寫進 custom 規則檔時，**不要寫是哪個服務、殘留在哪幾個 commit**（等於做出一份「哪裡有舊憑證」的索引）
- 🔴 **`git add` / `git commit` / `git push` 的 stderr 一律不遮**（要靜音只能靜 stdout 的 `-q`）
  - `git add -- <路徑A> <路徑B>` 只要**任一路徑在本 repo 不存在**、或該路徑**已經 staged 過且沒有新變更**，
    就回 `fatal: pathspec … did not match any files` 而**整個中止** —— 連存在的路徑也沒 stage。
    接著 `git commit` 印 `no changes added to commit`、`git push` 印 `Everything up-to-date`，**整串看起來像提交成功**
  - 跨 repo 批次提交時，pathspec 逐 repo 給（只列該 repo 真的有的路徑），commit 後**逐 repo 核對**
    `git log --oneline -1` 與 `git ls-remote origin refs/heads/main`
- **刪檔 / 搬檔一律 `git rm` / `git mv`**：閘門以 index ∪ 工作目錄列舉受檢檔，用 `rm` / `mv` 會讓 pre-push 以
  `[Errno 2] No such file or directory` 擋下，且訊息看不出真因
  - ⚠️ 剛新建、還沒 `git add` 的檔案 `git mv` 會失敗（不在 index）→ 先 `git add` 再搬
  - 閘門突然對「我沒改過的檔案」報錯時，先找 index 裡已不存在的檔：
    `git ls-files --cached -- <路徑> | while read -r f; do [ -e "$f" ] || echo "$f"; done`
  - **補救**：`git add -A -- <路徑>`（帶 pathspec，不是裸 `git add -A`），git 會把內容相同的識別為 rename
- 🔀 **搬檔 / 改名後批次替換路徑一定會漏三類**，改完逐類掃一次
  （替換用 [`scripts/tools/replace.py`](../../../../../scripts/tools/replace.py)，不要 `sed -i`；count 對了不代表那幾處都該改）：

  | 漏改類型 | 為什麼字串替換抓不到 | 怎麼找 |
  | --- | --- | --- |
  | **泛稱路徑** | 寫成 `<舊目錄>/<主題>.md` 這種佔位形式，不含被替換的完整檔名 | `grep -rn '<舊目錄>'` 逐筆判斷語意（有些是**正確**指向舊目錄） |
  | **相對連結** | `[文字](../conventions/)` 不含完整路徑字串 | `lint.sh md` 的斷鏈檢查會擋 |
  | **文字引用段落標題** | 「見某檔的『某段落』」：連結沒壞，但**標題已改名** | 拿舊標題關鍵字 `grep -rn` 掃一遍 |

  ⚠️ 第三類最陰：**連結有效、lint 全綠、內容卻指向不存在的段落**。所以**改動段落標題**（不只搬檔）之後也要掃。
