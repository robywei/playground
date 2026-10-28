---
name: distill
description: "對話收尾萃取：把這段對話裡執行過、驗證過的知識沉澱成 repo 可復用資源（skill / README / docs / CLAUDE.md），去重後提交，讓對話可安心刪、換電腦拉 repo 即一致。觸發：收尾、萃取、沉澱知識、distill、harvest、把學到的存進 repo、對話可以刪了嗎、封存 plan、CLAUDE.md 太長要外放。"
---

# 對話收尾萃取（Archive）

把「這段對話裡實際執行過、且已驗證的知識」沉澱成 **git repo 內的可復用資源**，
之後對話即使被清除、或換電腦，拉最新 repo 就能維持一致狀態、少走重複分析與錯誤執行。

> 核心原則：**只萃取「執行過 + 已驗證 + 未來會再用」的東西。**
> 一次性、推測未驗證、repo / git history / CLAUDE.md 已可得的，一律丟棄。
>
> ⚠️ **跨機必留的知識一律進 git，不要只寫 memory。**
> memory 在 `~/.claude/projects/<hash>/memory/`，是 harness 自動載入的**本機快取，不在 git、不隨 repo 走** ——
> 換電腦就消失。memory 只適合放「純本機脈絡 / 暫時筆記」。

🔁 本檔與同目錄的參考檔由 repo-template 管理：要改共用流程請改範本、發版後 `copier update`，**不要直接改這裡**。
本 repo 的跨 repo 路由、文件落點等專屬資訊若有記錄，在 `.claude/custom/archive-distill.md`，**存在就先讀**（不存在＝沒有專屬資訊）。

## 何時用

- 一段對話告一段落，要收尾並判斷「這次學到什麼值得留」
- 對話前先確認「過去類似事是否已有 skill / docs」（先查再做）
- 想把舊對話刪掉前，先確認知識已落進 git

## 分流：寫到哪裡（皆進 git）

| 內容性質 | 去處 |
| --- | --- |
| 可重複的操作流程 / 指令序列 / 排錯步驟 | **skill**（`.claude/skills/<domain>/skills/<skill>/SKILL.md`） |
| 某個目錄 / 腳本專屬的操作細節、參數語意 | **該目錄的 `README.md`** |
| 跨多個頂層目錄**且**動手前就必須生效的規則 | **`CLAUDE.md`**（哪一份、多長算太長：見 [`claude-md.md`](claude-md.md)） |
| 架構 / 設計敘事 / spec / 決策與被否決的方案 | **`docs/`**，或該決策生效位置旁的 README / 設定檔註解 |
| 一次性、已在 repo / git log、未驗證的推測 | **丟棄**（例：「這次跑了哪些指令」流水帳） |

⚠️ **被 `.gitignore` 忽略的目錄（常見是 `temp/`）寫進去等同跨機消失** —— 要保留的知識禁止放那裡。

- 🔁 **反向：要把 `temp/` 裡的既有檔案「納管」前，先確認它是不是殘留。** 被版控版本取代後舊檔仍留在本機，
  看起來像「一直沒納管」。先 `git log --oneline -S'<檔名>'`、`grep -rn '<檔名>' --include='*.md' .` ——
  查到已被取代 → 任務是**刪殘留**，不是重寫一份重複的
- plugin 自帶忽略規則的工作目錄（例：底下的 `.gitignore` 是 `*`）同理：一個專案跑完，
  那批素材是**唯一**的過程紀錄 → 收尾時整理成一份結案報告放 `docs/history/`；素材與報告不同天時，
  **逐項標註交付後的漂移**，否則封存的是過時資訊

## 步驟

1. **掃過對話**，逐段問：這是上表哪一類？
2. **先全域去重**（寫之前一定先查，**這步不可跳過**）：
   - **skill**：掃過 `.claude/skills/` 下**所有 domain** 的 `SKILL.md`，不只看「明顯對應」那個 ——
     也要 grep skill 的**內文與參考檔**，不只 description（原理常躺在參考檔或服務 README 裡）
     - 完全重複 → 不新增
     - 類似但不完全相同 → **更新既有 skill**，擴充涵蓋範圍與 `description` 觸發詞
     - 相關但屬不同 domain → 加 cross-reference，不要複製貼上
     - 確認不存在 → 先過 [`lifecycle.md`](../../../lifecycle.md) 的 🧹 退場判準（它其實是知識不是流程？只是薄指標？），都不是才新建
   - **docs / CLAUDE.md**：`grep -rn '<關鍵字>' --include='*.md' .`，有 → 更新該段落；沒有才新增。
     ⚠️ repo 可能有多份 `CLAUDE.md`，**全部都要 grep**，不能只看 root
   - 🔴 盤點不用裸 `rg`（跳過 `.claude/` 這類隱藏目錄 → 假的「不存在」→ 寫出第二份重複說明），規則見 `.claude/rules/common/agent-env.md`
   - 原則：**寧可更新一處變厚，不要散落成兩處講同件事**。既有說明「情境正確但缺通則」時，別再寫一份原理，
     只在該寫的地方補「規則本身 ＋ 指向既有說明的指標」
3. **依規範產出**
   - skill：照 [`.claude/skills/README.md`](../../../README.md) 的佈局與「新增 / 修改 skill」
     —— 🔴 含手動更新該 domain `plugin.json` 的 `description`、與新 `SKILL.md` 放同一個 commit（沒有任何檢查會抓它漂移）
   - docs / README / CLAUDE.md：遵守 markdownlint；事實附「為何 / 何時適用」，相對日期轉絕對日期
   - 規則寫在**觸發它的動作**那份文件，不是它描述的對象那份：見 [`claude-md.md`](claude-md.md)
   - **確保可被發現（不可跳過）**：新建的 README / docs 不能是孤兒檔 —— 在該目錄的 `CLAUDE.md`（沒有才退回 root）
     或對應 skill 加一行指標。自問：「下次開新對話、沒有這次脈絡，能靠 CLAUDE.md / skill 清單找到它嗎？」
4. **驗證**（避免寫進過時資訊）
   - 路徑 / 指令 / 旗標要對得上現況（實際 `ls` / 開檔確認）
   - 動到 skill → `claude plugin validate .claude/skills/<domain>`；動到 `.md` → `./scripts/lint/lint.sh md`
   - 🔴 **要寫進文件的「驗證指令」，自己先跑一次並看輸出合不合理**（例：含空白的關鍵字被切碎、碎片命中一切 → 一片「命中」）。
     沒跑就交付，等於把一個永遠全綠的假檢查固化進 repo
5. 🔴 **改到的檔案裡有沒有「repo 不等於線上」的？有就要部署，否則等於沒改**

   distill 的預設心智是「寫文件」，很容易把**設定檔**也當文件改完就算。逐檔問一次「這個檔是不是某個線上物件的來源」：

   - 被打包、掛載或複製到線上的設定檔與腳本（例：部署時才讀進去的設定）→ 要**重新部署 / 覆蓋**
   - 執行時才從 repo 拉取讀取的（例：排程任務每次 clone 讀的設定）、純文件 → push 即生效

   漏掉時**完全無聲**：repo 看起來對、lint 全綠、線上照舊。部署方式去該目錄的 README 查（別自己拼），
   部署後回頭 diff 線上與 repo，確認只差你改的那一段。本 repo 有哪些這類檔：見 custom 規則檔
6. **落點在哪個 repo**
   - 改到的內容屬於 **repo-template 擁有的檔**（看 `.copier-answers.yml` 的範本產出、或檔頭寫著「由 repo-template 管理」）
     → **改範本 repo、發版、各 repo `copier update`**，不要在本地改（本地改會被當成客製靜默保留，從此分岔）；
     只屬於本 repo 的補充寫進 `.claude/custom/<domain>-<skill>.md`（沒有就新建，見 [skills 佈局](../../../README.md)）
   - 知識屬於別的 repo → 寫進那個 repo（對照表見 custom 規則檔）
7. **收尾提交**：走 [`git:commit`](../../../git/skills/commit/SKILL.md)
   - 萃取的內容若只是替這段對話某筆**還沒 push** 的 commit 補完同一件事 → 併回它（見 `git:commit`「一筆還是拆多筆」）；
     跨主題的通則才獨立一筆
   - 推之前跑 `git log --oneline origin/main..HEAD`，把這段對話先前延後未推的 commit 一起推上去
   - 完成且推上後，**這段對話即可安心刪**

## 遇到這些情況，讀對應的參考檔

| 情況 | 參考檔 |
| --- | --- |
| 實作 plan / 分析文件做完了，要封存；或要把 `temp/` 的文件收進 git | [`plan-archive.md`](plan-archive.md) |
| 要寫進 `CLAUDE.md`、不確定哪一份；規則該寫在哪份文件；root `CLAUDE.md` 太長要外放 | [`claude-md.md`](claude-md.md) |
| 寫護欄、否決方案、修漂移的註解、寫實測數字、要沉澱的是一段可執行邏輯、改了跨檔案的機制或退役了某個資源 | [`writing.md`](writing.md) |
| 回答完一輪分析，要確認哪些結論已在 repo、哪些是當輪推導的 | [`writing.md`](writing.md)「讀來的與推來的」 |

## 常見錯誤

- 把流水帳（跑過哪些指令）當知識存 → 只存「下次怎麼做」的可重複步驟
- 把跨機要用的知識只寫進 memory → 換電腦就消失
- 沒去重就新增 → 出現兩份講同件事的 skill / docs；只 grep root `CLAUDE.md` 就斷定「沒寫過」
- 目錄專屬細節塞進 root `CLAUDE.md` → 每次對話全文載入，被稀釋、成本變高；
  反過來，全 repo 通用、動手前就要生效的規則只寫進子目錄 `CLAUDE.md` → lazy-load，等於沒寫
- 新增 skill 沒改 `plugin.json` 的 description
- 建了孤兒文件沒掛入口 → 下次照樣重查（等於白寫）
- 寫了沒驗證 → 把過時路徑 / 旗標固化進 repo，比沒寫更糟
- 改了「repo 不等於線上」的檔沒部署
- 在本地改範本擁有的檔 → 下次 update 不會覆蓋、也不會提示，從此和其他 repo 分岔
- 萃取完忘了 push → 換電腦 / 刪對話後就遺失（commit ≠ push）
