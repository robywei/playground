# 🤖 AI Agents 操作指引

個人實驗與側案儲存庫，一個目錄一個專案。

- **時區**：Asia/Taipei (UTC+8)
- **目前的專案**：[`babywei-bakery/`](babywei-bakery/) —— 麵包店成本與庫存管理
  （Vue 3 + Vite 前端 embed 進 Go binary，設計文件在該目錄 `docs/superpowers/specs/`）

## 📖 動手前先讀

| 要做的事 | 先讀 |
| --- | --- |
| 寫或改任何 `.md` | [`docs/rules/markdown-style.md`](docs/rules/markdown-style.md)（**兩種假綠燈**、設定檔按 cwd 找、`--fix` 會靜默改壞） |
| 建 commit | 下方 📋 —— **主旨即 CHANGELOG 的唯一內容**，且事後無法修 |
| 新增目錄 | 下方 📂 —— 全小寫 kebab-case |

## 📋 CHANGELOG 與 Git Commit

🚫 **`CHANGELOG.md` 不手寫** —— 由 `git-cliff` 從 commit message 全量生成
（設定 [`.cliff.toml`](.cliff.toml)，跨 repo 標準版，改動前先讀該檔檔頭）。
手寫的條目下次生成就消失。

```sh
git-cliff --config .cliff.toml -o CHANGELOG.md
```

生成後以 `chore(changelog):` 提交 —— `.cliff.toml` 會把這類 commit 從下一次生成
中 skip 掉，否則每次「生成 → commit」都會多一條噪音條目。

於是 **commit message 就是 CHANGELOG 的唯一內容來源，而且寫錯無法事後修**
（改 message ＝ 改寫歷史）：

- **主旨**：Conventional Commits `type(scope): 主旨`，中文撰寫，
  路徑與關鍵字用 `` ` `` 包住 —— 例 ``docs(lint): 📏 `README.md` 補上檢查指令``
- ⚠️ **CHANGELOG 只取主旨**（body 不輸出）→ **主旨必須能獨立讀懂**，
  不能依賴 body 補完
- **body 仍要寫**（markdown 清單，單檔用完整路徑）：細節留在 git，
  用 `git log --grep=<關鍵字>` / `git log -S` 搜得到，CHANGELOG 不重複一份

`type` 決定 CHANGELOG 分類（權威映射在 `.cliff.toml` 的 `commit_parsers`）：

| type | CHANGELOG 分類 |
| --- | --- |
| `feat` | ✨ Added |
| `fix` | 🐛 Fixed |
| `refactor` `perf` `style` | ✏️ Changed |
| `cleanup` `revert` | 🗑️ Removed |
| `docs` | 📚 Docs |
| `ops` | 🔧 Ops |
| `chore` `ci` `build` `test` | ⚙️ Chore |

⚠️ **上表的 emoji 只出現在 CHANGELOG 的分類標題，不要寫進 commit 主旨。**
主旨裡的 emoji 是另一套：緊接冒號後、**自由選用的語意標記**，描述「這筆做了什麼」
而非它屬於哪一類（🔄 同步、📇 索引、🔍 調查、🧹 清理、📏 規範、✂️ 精簡…）。
可以不放；放就要對得上內容。

## 📂 目錄命名

原始碼目錄採**全小寫 kebab-case**。三個理由：`package.json` 的 `name` 欄位不允許
大寫（npm 對新套件名一律拒絕）；Go module path 中的大寫字母會在 module cache 被
轉義為 `!` + 小寫；混用大小寫的路徑在不分大小寫的 macOS 與分大小寫的 CI 之間會
造成 git 偵測不到變更。

此規則只適用於原始碼。**交付產物保留其顯示名稱** —— 例如 `babywei-bakery/`
打包後是 `BabyWei Bakery.app`。

## 🧩 Agent skills

⚠️ **優先序：本檔 > skills。**

`superpowers` 的 `brainstorming` 在此 repo 適用（新專案一律走完整流程：問題 →
方案比較 → spec → `writing-plans`）。`verification-before-completion` 同樣適用，
且與 `docs/rules/markdown-style.md` 的「假綠燈」同源：**exit 0 不等於檢查過**。
