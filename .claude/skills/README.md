# `.claude/skills/` 佈局慣例（專案範圍 plugin）

本目錄的每個 domain 都是一個**專案範圍的 Claude Code plugin**（`<name>@skills-dir`），
一個情境就是一個 skill，叫用方式是 **`/<domain>:<skill>`**（例：`/git:commit`）。

> 為什麼不用「一個 dispatcher `SKILL.md` + 各情境檔」：那樣 `/<domain>` 只會出現**一列**，
> 情境全藏在 dispatcher 的表格裡，既叫不出來、也無法各自被自動觸發。

🔁 本檔與 [`lifecycle.md`](lifecycle.md)（命名、改名、退場、`plugin.json` 對帳）由 repo-template 管理：
要改請改範本、發版後 `copier update`，**不要直接改這裡**。
本 repo 的 domain 用途、寫 skill 的本 repo 慣例與待補情境記在
[`.claude/custom/skills-readme.md`](../custom/skills-readme.md)，**先讀**。

## 目錄形狀

```text
.claude/
  skills/
    <domain>/
      .claude-plugin/plugin.json   ## 有這個檔才會被當成 plugin 載入
      references/<共用前置>.md      ## 選用：跨 skill 共用、但不該各自複述的內容
      skills/
        <skill>/SKILL.md           ## 一個情境 = 一個 skill（frontmatter 必填 name / description）
        <skill>/<參考檔>.md         ## 選用：只有這個 skill 用得到的細節，從 SKILL.md 連過去
        <skill>/<附帶腳本>.sh       ## 選用：與該 skill 同目錄
  custom/<對應檔>.md               ## repo 專屬規則（範本只產生一次，之後歸 repo）
```

⚠️ **`plugin.json` 沒有 `skills` 欄位時，預設掃 `skills/` 子目錄**，不必逐一列出。

### 有哪些 skill：用查的，不要手寫清單

```bash
for d in .claude/skills/*/skills; do echo "$(basename "$(dirname "$d")"): $(ls "$d" | tr '\n' ' ')"; done
```

本檔與 root `CLAUDE.md` 都**不列 skill 清單**：session 啟動時 harness 已注入每支 skill 的完整 `description`，
再手寫一份等於同一份知識存兩份。實證：手寫清單的 README 在後續新增 skill 時全部沒跟上，漏列的 skill 在清單上「不存在」。

### 範本擁有的 domain：`git`、`archive`、`guard`

這幾個 domain 的 `SKILL.md`、參考檔、hooks 程式與 `.claude-plugin/plugin.json` 由範本產生（檔頭寫著「由 repo-template 管理」）。

- 🔴 **`plugin.json` 不可手改** —— `copier update` 會靜默保留本地修改（不提示），從此與範本分岔；答案或範本一變就衝突
- **在 `git` 加本 repo 自己的 skill**：skill 目錄照常建（歸本 repo），`plugin.json` 的描述改走答案 `git_extra_skills`，
  `--data` 給的是**整份清單**（含既有項目）：

  ```bash
  copier update --defaults --data 'git_extra_skills=[{name: release-notes, desc: "發版說明"}]'
  ```

- **`archive` 目前沒有對應機制** → 本 repo 要的萃取類 skill 先建在本 repo 自己的新 domain
- **`guard` 是 hooks-only plugin**：本 repo 需要額外的遮蔽或攔截規則時，另建自己的 mod（獨立 plugin），不要改範本的 guard
- **不可在本 repo 改名**：改名要在範本發版，所有使用端一起跟

### 例外形態：hooks-only plugin（mod）

有些 domain 沒有 `skills/`，只有 `hooks/`（`hooks.json` + TypeScript 模組 + `*.test.ts`）——
它是 Claude Code mod，改寫的是工具行為，不是可叫用的情境（例：遮蔽輸出中機密的 `guard`）。

- 🔴 **一個 plugin 只能有一個 hooks 模組** → 失敗領域不同的 hooks **分成不同 plugin**。
  一個模組寫壞（語法錯、import 失敗）時才不會連帶卸載另一個；**不要為了整齊把它們合併**
- **同一個 domain 不要混放 skill 與 hooks**，讓「這個 plugin 會改寫工具行為」一眼可辨；
  下方 frontmatter 與 `plugin.json` 對帳的規則不適用（它沒有 skill）
- ⚠️ 常駐的互動 session 可能在存檔時就熱重載 hooks，改到一半就生效 → 先在複本上改，改好再放回來
- 驗證：`claude plugin validate .claude/skills/<domain>`；有 `hooks/*.test.ts` 時再跑 `claude plugin test .claude/skills/<domain>`
- 要停用 mod：`/plugin` 裡停用該 plugin，或以 `claude --safe-mode` 啟動（停用所有自訂）

## 新增 / 修改 skill

1. 建 `<domain>/skills/<skill>/SKILL.md`，frontmatter 至少要有：

   ```yaml
   ---
   name: <與目錄同名的 kebab-case>
   description: "做什麼 + 觸發：<關鍵字>。這段是 always-on，是模型自動觸發與 /<domain>: 補全的唯一依據"
   ---
   ```

2. 🔴 **手動更新該 domain 的 `.claude-plugin/plugin.json` 的 `description`**（列出這個 domain 涵蓋哪些情境）——
   `plugin validate` 不驗它、按路徑 grep 也抓不到，增刪 skill 後它會**安靜地說謊**。
   範本擁有的 domain 改走答案（見上節）；對帳迴圈見 [`lifecycle.md`](lifecycle.md)
3. `claude plugin details <domain>` 看 token 成本（`description` 全部常駐，別寫成關鍵字傾倒場）
4. 驗證：

   ```bash
   ## 1) frontmatter / manifest —— 全 domain 一次驗
   for D in .claude/skills/*/.claude-plugin; do claude plugin validate "$(dirname "$D")"; done
   ## 2) 本 repo 的 lint 入口（檢查項由各 repo 自己維護）
   ./scripts/lint/lint.sh
   ```

   - ⚠️ **`plugin validate` 的參數是 plugin 目錄路徑，不是 domain 名** —— `claude plugin validate git` 會報
     `No manifest found in directory`，因為它找的是 **CWD 下**的 `<參數>/.claude-plugin/plugin.json`
   - ⚠️ `plugin validate` 只看 frontmatter 與 manifest、markdownlint 只看格式，**兩者都不驗連結目標存不存在** ——
     搬檔、改名後要另外確認相對連結（以檔案所在目錄解析），而且要掃全 repo，不能只掃 `.claude/skills/`
5. 🔴 **新 `SKILL.md` 與該 domain 的 `plugin.json` 在同一個 commit 提交**，別讓 plugin 的自述與實際內容在歷史裡錯開；
   提交走 [`/git:commit`](git/skills/commit/SKILL.md)
6. 改動要確認生效，見下節

## 讓改動生效

改完 skill / plugin 之後**什麼時候生效**，各版本、各介面的行為不同，文件記載也互相矛盾 ——
**不要照任何一份記載推論，當場確認**。沒生效時的排除順序：

1. **確認磁碟內容**：直接 `Read` 那份 `SKILL.md`，以磁碟上的檔案為準，不要照注入的文字做
2. **終端機 CLI**：`/reload-plugins`（可帶 `--force`）
3. **開新對話 / 重啟 session**

- 🚫 **`/reload-skills` 不能代替** —— 它只重掃 `SKILL.md`、不碰 plugin 子系統，對新增的 `plugin.json` 視而不見（回 `no changes`）
- 🚫 **IDE 的「Open in Terminal」也不能代替** —— 那開的是**另一個獨立 session**，在裡面重載不影響原本的 session
- 改 skill 名、plugin 名（目錄名於 session 啟動時註冊）至少要重載才完全生效

## 硬性規則

| 規則 | 為什麼 |
| --- | --- |
| **`description` 內不可出現 `---`** | frontmatter 解析器對 `---` 做**非行錨定**切分 → 描述中間的 `---` 會提早關掉 frontmatter（`YAML Parse error: Unexpected EOF`），該 skill **以空 metadata 靜默載入**。`plugin validate` 抓得到，人眼看不出來；不要自己寫行錨定的 parser 來驗 |
| **`description` 要有 `觸發：<關鍵字>`** | 那串關鍵字是模型自動觸發的依據。寫成散文（「只有提到 X 時才走」）語意雖對，卻少了可匹配的標記 |
| **`plugin.json` 的 `name` 必須等於目錄名** | `plugin validate` 會擋 |
| **plugin 名不可撞內建 slash command** | 撞名時手動 `/<name>` 叫不出來、自動觸發照樣中，看起來像時好時壞。保留字見 [`lifecycle.md`](lifecycle.md) |
| **範本擁有的 domain 不手改 `plugin.json`、不在 repo 端改名** | 由範本產生；本地修改會被 update 靜默保留成分岔（見「範本擁有的 domain」） |
| **相對連結深度** | `skills/<x>/SKILL.md` 到 repo 根是 `../../../../../`；`references/` 檔是 `../../../../`；到 `.claude/custom/` 是 `../../../../custom/`；跨 domain 是 `../../../<domain>/skills/<skill>/SKILL.md`；同 domain 手足是 `../<skill>/SKILL.md` |
| **裸檔名連結是搬檔最易漏的一類** | `](foo.md)` 沒有 `../` 前綴 → `grep '](\.\.'` 抓不到。要抓所有 `](...)` 才看得到 |
| **共用前置放 `references/`，不要各 skill 複述** | 例：`<domain>/references/auth.md`（憑證載入、參數合法值）。複述必漂移 |
| **本檔與 root `CLAUDE.md` 不列 skill 清單** | 見「有哪些 skill」；root 只留叫用慣例 `/<domain>:<skill>` 與本檔指標 |
| **新 clone 的機器要先信任工作區** | 未信任時 `plugin list` 會印「... was not loaded because this workspace was not trusted」，**skill 靜默不存在**。接受 trust dialog 後重載 |
| **`SKILL.md` 內文避免裸 `$0`～`$9`** | 見下節 |

### 帶 args 叫用時，內文的 `$N` 可能被替換

帶 args 叫用 skill 時，內文的 `$0`～`$9` 會被替換成 args 依空白分詞後的第 N 個 token —— **原檔不變、只有載入後的呈現被改、不報錯**。
`awk '{print $2}'` 可能變成 `awk '{print <某個 args 字詞>}'`。替換規則隨版本可能不同，以當場確認為準。

- **判準**：呈現的 code block 出現不該有的字（中文、與情境無關的 token）時，**先 `grep` 原檔對照**，原檔永遠是權威
- 🔴 **反向更貴**：誤判成「skill 檔有 bug」而去修，是把正確的檔案改壞
- **寫 skill 時規避**：`awk '{print $NF}'`、`awk -v c=2 '{print $c}'`、`cut -d' ' -f2`。盤點現有的：
  `grep -rnE '\$[0-9]' --include='*.md' .claude/skills/`
