# skill 的生命週期：命名、改名、退場、`plugin.json` 對帳

> 📎 [`.claude/skills/README.md`](README.md) 的參考檔。由 repo-template 管理，**不要直接改這裡**。
> 新增 / 修改 skill 的步驟與硬性規則在 README；這裡是少用但不能做錯的維護操作。

## 🏷️ 命名守則：兩層命名，撞名風險不同

| 層級 | 解析依據 | 撞內建 slash command 會怎樣 |
| --- | --- | --- |
| **plugin 名** = `.claude/skills/<dir>/` 目錄名 | 目錄名（`plugin.json` 的 `name` 必須一致，`plugin validate` 會擋） | ⚠️ **仍有風險**：打 `/<name>` 會先命中內建 |
| **skill 名** = `skills/<dir>/` 目錄名 | 目錄名（frontmatter `name:` 要一致） | ✅ **不受影響**：永遠帶 `<plugin>:` 前綴，`/git:commit` 不會撞 `/commit` |

plugin 化把撞名風險從「每個情境」收斂到「每個 domain」。被 shadow 的症狀最隱蔽：
**模型自動觸發（靠 `description` 命中）照樣叫得到，只有手動 `/<name>` 進不去** —— 看起來像時好時壞，其實是手動入口壞了。

常見內建保留字（別拿來當 plugin 名）：`mcp`、`help`、`clear`、`compact`、`config`、`cost`、`doctor`、`init`、
`memory`、`model`、`agents`、`review`、`status`、`login`、`logout`、`permissions`、`resume`、`add-dir`、`bug`、
`hooks`、`export`、`vim`、`terminal-setup`、`pr-comments` …

- 內建命令會隨版本增減，**完整清單以當前 `/help` 為準**；取名前先在互動視窗打 `/` 看補全
- `inline`、`builtin`、`skills-dir` 是 **marketplace 保留字**，不能拿來當 marketplace 名（plugin 名不受此限）

## 改名 SOP

🔴 **範本擁有的 domain（`git`、`archive`、`guard`）與其中由範本提供的 skill，不可在本 repo 改名** ——
改名要在範本發版，所有使用端 `copier update` 一起跟。下面只適用本 repo 自己的 domain / skill。

**改 skill 名**（`<plugin>:<old>` → `<plugin>:<new>`）：

1. `git mv .claude/skills/<domain>/skills/<old> .claude/skills/<domain>/skills/<new>`
2. 改該 `SKILL.md` 的 `name:` frontmatter 與 H1 標題
3. 全掃引用並更新：`grep -rn 'skills/<old>' .`（同 domain 手足 `../<old>/SKILL.md`、跨 domain
   `../../../<domain>/skills/<old>/SKILL.md`）；裸檔名連結與純文字提到的舊名 grep 路徑抓不到，另用 `grep -rn '<old>'` 掃
4. 該 domain 的 `plugin.json` 的 `description` 若提到舊名一起改；跑 README 的驗證

**改 plugin 名**（`<old>:*` → `<new>:*`）：

1. `git mv .claude/skills/<old> .claude/skills/<new>`
2. 改 `.claude-plugin/plugin.json` 的 `"name"`（**必須等於目錄名**）
3. 改所有交叉引用（`grep -rn 'skills/<old>' .`、`grep -rn '/<old>:' .`）
4. 檢查 `.claude/settings.local.json` 的 `Edit(...)` 白名單是否寫著舊路徑
5. 驗證後走 [`/git:commit`](git/skills/commit/SKILL.md) 提交

⚠️ 兩種改名都要重載才完全生效（目錄名於 session 啟動時註冊），方式見 README「讓改動生效」。

## 🧹 什麼時候該收掉一個 skill（退場判準）

skill 只會長不會縮的話，**常駐成本會被一批沒人叫的 `description` 吃掉**（每支的 `description` 全文常駐）。四條判準：

| 判準 | 症狀 | 處置 |
| --- | --- | --- |
| **① 它其實是知識，不是流程** | 通篇沒有可執行步驟，只有「為什麼選 A 不選 B」 | 寫到**該決策生效的位置旁**（該目錄 README / 設定檔註解） |
| **② 薄指標，零新增資訊** | 全文就是「去讀 `<某>/README.md` §N」 | 直接刪。中間再加一層只是多一份會漂移的東西 |
| **③ 與現有 README 重複** | 指令兩邊各一份，skill 自己還寫著「以該 README 為準」 | 刪 skill，把 README 的入口指標補強 |
| **④ domain 錯置** | skill 講的是**現役**的工具或環境，卻掛在備案、已停用或已退役的 domain 下 | `git mv` 到正確 domain，別跟著陪葬 |

- ⚠️ **判準①的變形最貴：一個備案不該佔 N 支 skill。** 先問「這個 domain 現在是不是現役」，再問它值幾支
- 🚫 **「主題與本 repo 領域不合」不是退場理由。** 判準永遠是**「這件事還會不會再做」**，不是「它純不純」 ——
  會反覆用、已有可運作腳本的工具被收掉後，換來的只是「下次找不到」
- 🚫 **排錯型 skill 看起來像一次性事件紀錄，但若屬於會再發生的類別就不收。** 判準是「這類事會不會再來」，
  不是「這件事發生過幾次」

### 收掉的動作（順序不能顛倒）

1. **先搬內容**，再刪 skill —— 反過來就會在 git 歷史裡追內容
2. **掃引用再刪**：各層 `CLAUDE.md`、各層 `README.md`、其他 skill、`.claude/settings.local.json` 的 `Edit(...)` 白名單

   ```bash
   LC_ALL=C grep -rn 'skills/<name>' --include='*.md' --include='*.json' . \
     | LC_ALL=C grep -v -e '^\./CHANGELOG' -e '^\./docs/history/'
   ```

   ⚠️ `CHANGELOG.md` 與 `docs/history/` 的引用**不要改**，那是歷史事實
3. 🔴 **改該 domain 的 `plugin.json` 的 `description`**（範本擁有的 domain 改答案，見 README）—— 見下節
4. **不要為了「以後可能用到」留副本**：暫緩理由日後可能不成立的，留在原地並在 `SKILL.md` 標註停用理由；
   內容已完整併入他處的直接刪（考古走 `git log --diff-filter=D --oneline -- <舊路徑>`，再 `git show <sha>^:<路徑>`）
5. 跑 README 的驗證 + **重開 session**

## 🔴 `plugin.json` 的 `description`：沒有自動檢查會抓到它漂移

| 機制 | 為什麼抓不到 |
| --- | --- |
| `claude plugin validate` | 只驗 **schema**（欄位在不在、`name` 等不等於目錄名），**不驗語意** —— description 寫什麼都過 |
| 按路徑 `grep 'skills/<name>'` | `plugin.json` 寫的是**人話、不含路徑** → 必然零命中 |

**增刪任何 skill 後都要跑這段對帳**：

```bash
## 左邊是 plugin.json 的自述，右邊是該 domain 實際有哪些 skill —— 人眼比對（hooks-only plugin 沒有 skills/，印「（無）」）
for D in .claude/skills/*/.claude-plugin/plugin.json; do
  p=$(dirname "$(dirname "$D")")
  echo "── $(basename "$p")"
  python3 -c "import json;print('  說:',json.load(open('$D'))['description'])"
  if [ -d "$p/skills" ]; then echo "  有: $(ls "$p/skills" | tr '\n' ' ')"; else echo "  有: （無）"; fi
done
```

**「增刪時記得改」不夠**，有兩條路徑會繞過它：萃取對話時只寫了 `SKILL.md` 就提交；或建 domain 時手寫描述就漏了幾支
（生下來就不完整，任何「比對前後差異」的方法都抓不到）。
**判準：domain 越大，越不能相信手寫的描述涵蓋完整** → 做全 repo 文件稽核時把上面的迴圈當固定項跑一次。

### 「注入清單只印名字」不等於該 skill 壞了

session 啟動注入的 skill 清單裡，有些條目可能只有名字、沒有 description —— 和「`---` 讓 frontmatter 提早關閉、
以空 metadata 靜默載入」症狀幾乎一樣，極易誤判。用 `claude plugin details <domain>` 的 always-on token 分辨：

| 該 skill 的 always-on | 結論 |
| --- | --- |
| **有數值** | description 已被解析並注入，清單只印名字是呈現層省略 → **檔案無事可做，不要去亂改** |
| **接近 0** | 才是真的空 metadata → 去查 frontmatter 是否被 `---` 提早關閉（`claude plugin validate`） |
