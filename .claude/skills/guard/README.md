# `guard`：機密遮蔽 mod

hooks-only plugin（Claude Code mod，沒有 skill）。做四件事：

1. **遮蔽**：工具結果與其他對話列寫進 transcript **之前**，把形狀像機密的字串換成 `<redacted:FAMILY len=N>`
2. **擋寫回**：`Write` / `Edit` / `NotebookEdit` / `Bash` / `Monitor` 的內容含遮蔽標記時直接 deny，訊息附該工具的正確做法
   —— 否則模型可能把讀到的標記當成檔案內容寫回去，**用標記覆蓋掉真的機密**
3. **遮蔽後的提示**：附一段只給模型看的「下一步怎麼做」（見〈遮蔽之後怎麼做〉）；每個 loop（主迴圈、各子 agent）
   第一次給完整版、之後一行 —— 只遮不教，模型會卡住或換寫法撞牆
4. **自我檢查**：開 session 時用內建樣本跑一次全部規則，有規則失效就跳通知

出錯時的方向：遮蔽與大輸出落檔出錯一律 **fail closed**（藏起輸出）。

刻意**不做**的：不在執行前攔指令（例如 `bash -x`）。xtrace 印出的 token 一樣會被遮蔽、不進 transcript；
要擋指令得帶一個 shell lexer 才分得出「執行」與「字串裡出現」，成本與範本「盡可能精簡」不符。
需要攔指令的 repo 另建自己的 mod（獨立 plugin）。

## 前置

Claude Code `2.1.287` 起 mod 預設開啟。停用：`/plugin` 停用這個 plugin，或用 `claude --safe-mode` 開單一 session。

## 確認它真的在運作

在 session 裡讓 agent 跑這行（執行期才拼出假的 Vault token，指令本身不含 token 字面值）：

```bash
printf 'hv%s.%s\n' s A1b2C3d4E5A1b2C3d4E5A1b2C3d4
```

| 模型看到的輸出 | 判讀 |
| --- | --- |
| `vault` 的遮蔽標記（長度 32） | ✅ 運作中 |
| 原樣的 `hvs.A1b2…` | ❌ 沒載入 → 開新 session 再探測；仍失敗就在**不含 mod 的目錄**跑 `claude plugin test`（`no hooks module to load` ＝ mod 可以載入；`turned off` ＝ 被設定或遠端關閉），再用 `claude --debug-file <檔>` 找 `hooks module guard` |

探測失敗期間照散文規則自己防：不開 xtrace（`bash -x`、`set -x`）、不把機密印出來。

## 遮蔽規則

權威是 [`hooks/redact.ts`](hooks/redact.ts) 的 `RULES`。原則：**只收有明確前綴、或 key 名錨定的格式**（誤報低）。

| family | 涵蓋 |
| --- | --- |
| `pem` | `BEGIN … PRIVATE KEY` 區塊（含 PGP 的 `PRIVATE KEY BLOCK`、加密金鑰的 `Proc-Type` 標頭行、JSON 跳脫的 `\n`）；**沒有 END 也遮**（`head`、Read limit、大輸出 preview 截斷）；行首縮排（YAML `tls.key: \|`）、`cat -n` 行號、`git diff` 前綴、空白相連（`echo $KEY`）、標頭黏著本體、整段一行都涵蓋；只提到標頭字串、後面沒有 base64 的程式碼不遮。**沒有 BEGIN 也遮**：以 END 行為錨點往回收本體 —— 行寬須一致且是 64 / 70 / 76、純 hex 不收、只有一段時要 64 字元以上（擋掉前面的 SHA、指紋、公鑰、長 token）。**OpenSSH 本體固定開頭**（`openssh-key-v1` 的 base64）連 BEGIN / END 都沒有也遮 |
| `github_pat` / `github` | fine-grained PAT、`ghp_` / `gho_` / `ghu_` / `ghs_` / `ghr_` |
| `slack` / `slack_app` / `slack_webhook` | `xox[abeoprs]-`、`xapp-`、`hooks.slack.com/{services,workflows,triggers}/…` 整條 URL（斜線含 `\/` 與 `%2F` 寫法） |
| `cloudflare` | `cfat_` / `cfut_` |
| `vault` | `hvs.` / `hvb.` / `hvr.` |
| `jwt` | 三段式 `eyJ…` / `eyA…` / `ew…`，含 padding `=` 與標準 base64 `+/` |
| `anthropic` / `openai` / `google` / `onepassword` | `sk-ant-`、`sk-proj-` 等、`AIza…`、1Password Secret Key `A3-…` |
| `aliyun_sk` | AccessKey Secret（30 字元以上）：key 名 `access_key_secret` / `secret_access_key` 各種大小寫與分隔、雲端廠商前綴且含 `SECRET` 的環境變數、`--access-key-secret`；**只遮值、保留 key 名** |
| `aliyun_sts` | STS 臨時憑證（`security_token` / `sts_token` / `--sts-token`）；只遮值 |
| `url_password` | `scheme://user:密碼@`（user 可空、scheme 不分大小寫），密碼 8 字元以上；**整個值**是 `$VAR` / `<x>` / `***` / `xxx` / `password` 這類佔位才放過；`host:8080?x=a@b` 這種其實是 port 的不算 |

刻意**不遮**：AccessKey ID（`LTAI…`，不是機密，遮了只會妨礙工作）、40-hex（與 git SHA 無法區分）。

## 盲區（沒遮到 ≠ 沒外洩）

- **無前綴的機密**：各種密碼、Vault unseal key、舊式 `s.` Vault token、舊式 40-hex classic PAT —— 只有部分靠 key 名錨定
- **base64 包裝**：Kubernetes Secret 的 `.data`、`.dockerconfigjson` 的 `auth`
- **PEM 片段的剩餘形狀**：只有中段、沒有 BEGIN 也沒有 END 的 RSA / EC / PKCS#8（OpenSSH 有固定開頭所以認得）；
  只印最後一行 + END（`tail -2`）；行首有 `#` / `>` / `//`、ANSI 色碼包住、小寫 END、JSON 陣列 `"…","…"`、`grep` 的檔名前綴
- 密碼裡含原樣 `@` 或 `/` 的 URL
- **使用者自己輸入的 prompt 一律不改**（改了也遮不住 queue 那份原樣紀錄，只會讓模型讀不到使用者給的東西）
- **`Monitor` 與背景任務通知**：模型讀到的那份已遮蔽，但 queue 的原樣紀錄任何 hook 都改不到 →
  用 `Monitor` 盯會吐機密的輸出（`kubectl logs -f` 之類）仍不安全
- **不在 hook 範圍**：`~/.claude/file-history` 的編輯前備份、附件 payload；模型自己產生的 `tool_use` 參數與 thinking 區塊
  —— 使用者貼進來的機密被模型寫進指令時，那份指令原樣保存
- **命令本身已經印出來**：只保護 transcript，不保護終端畫面、CI log、聊天室等外部系統
- **大輸出落檔**（`tool-results/`）：4 MiB 以內就地改寫；讀不進來就**以一行提示取代原檔**（fail closed，要看完整輸出請加過濾後重跑）
- **遮蔽過程本身出錯**時同樣 fail closed：該次輸出以 deny 訊息取代
- **`bash -x`／`set -x` 的輸出**：transcript 裡已遮蔽，但終端畫面與外部 log 照樣印出真值 → 不開 xtrace

## 對工作流的影響

- 模型讀到的是 `<redacted:FAMILY len=N>`，**拿不到真值** —— 需要用機密的指令，在同一次呼叫裡從來源展開成環境變數，
  不要先印出來再用
- 編輯含機密的檔案：Edit 的 `old_string` 只挑不含標記的片段；整檔 Write 會被擋
- `grep` 遮蔽標記、commit message 帶標記也會被擋（Bash 指令含標記即 deny）—— 刻意的：真檔案裡是真值，
  grep 標記必然零命中，是誤導性的假陰性；要在文字裡提到標記格式就寫 `len=N`
- **工具回錯誤**（exit code 非 0）且輸出含機密時，結果以 deny 形式帶回遮蔽後的錯誤文字 ——
  文字開頭寫明「工具已執行、這不是權限拒絕」，免得模型以為被擋而改寫指令重試

## 遮蔽之後怎麼做

看到遮蔽標記後，**不要想辦法把真值弄回來**，依目的選下一步。guard 會把下表濃縮成提示附給模型：

| 目的 | 做法 |
| --- | --- |
| 只想確認機密有沒有載入 | 印長度：`printf %s "$VAR" \| wc -c`（bash / zsh 都能跑） |
| 把值交給某個指令 | 在**同一個 Bash 呼叫**從來源展開成環境變數，讓指令自己讀：`set -a; . ./app.env; set +a; <指令>`，或本機的載入函式 |
| 複製含機密的檔 | `cp`（讀了再 Write 會被擋寫回） |
| 改同一檔的其他行 | Edit 的 `old_string` / `new_string` 只放不含標記的片段（例如只動 `KEY=` 那行的 key） |
| 要改的就是機密值本身 | 交給使用者 |

⚠️ **不要用 `$(grep KEY= app.env | cut -d= -f2-)` 把值撈進指令字串**：auto mode 的 classifier 常把它判成
憑證外流而擋下。上表的環境變數寫法也只是**比較常過**、classifier 的判定不是決定性的；被擋就停下來交給使用者，
不要再換寫法重試。

## 修改這個 mod

🔴 互動 session 會「**存檔即熱重載**」`.claude/skills/guard/` —— 改到一半的檔案會立刻生效。所以**先在複本上改**，測過再覆蓋：

```bash
SP=<暫存目錄>; cp -R .claude/skills/guard "$SP/guard"       ## 在複本上改
claude plugin validate "$SP/guard"                           ## manifest 與 API 用法
claude plugin test "$SP/guard"                               ## 行為（遮蔽規則、擋寫回、效能），約 1 秒
node "$SP/guard/scripts/fp-scan.mts" .                       ## 新規則對 repo 檔有沒有誤報（node 22.6+）
cp "$SP"/guard/hooks/*.ts .claude/skills/guard/hooks/        ## 都過了才一次覆蓋
```

- 改到 `tool.call` 的回傳路徑或遮蔽規則時，另跑 [`scripts/e2e.sh`](scripts/e2e.sh) `"$SP/guard"`：
  兩次 headless session（約 1～2 分鐘、**會計費**），驗 output schema、真實落檔與 worker heartbeat ——
  `claude plugin test` 不驗這些，hook 回傳形狀錯誤的 `{ result }` 時測試照樣通過、實際 session 卻會被拒絕
- 🔴 **每條 regex 都必須是線性時間**：hook 同步跑在 worker 裡，單次超過 heartbeat（5 秒）worker 會被砍，
  **guard 在該 session 剩下的時間整個卸載**、之後所有結果原文入檔，`-p` 模式沒有任何提示。寫法見 `redact.ts` 檔頭；
  新增規則一定要在測試的效能組補一筆病態輸入
- 🔴 **`tool.call` 裡新加的任何程式碼都要包 try/catch** —— hook 一 throw 就整個被跳過，**遮蔽也跟著消失**。
  提示這類觀察層寫壞了只能少一句，不能拿掉遮蔽
- **`$` 不能交給 helper 函式**（validator 擋）：在 handler 內取值，把結果傳進純函式
- **測試檔與文件裡不要寫字面 token、字面 PEM 標頭或字面標記**：假值用 `samples()` / `keyedSamples()`、
  標頭用 `pemHeader()`、標記用 `mark()`，都在執行期拼出；文件舉例寫 `len=N`（非數字，`MARKER` 不認）。
  否則 guard 生效後，讀這些檔看到的就是標記，**連改它們都會被自己擋下**，而字面 token 會被 GitHub push protection 攔
- `samples()` / `keyedSamples()` **每條規則都要有一筆**，而且**自我檢查與測試共用同一份**（含規則需要的上下文後綴，
  例如 `url_password` 要的 `@host`）—— 各拼各的會讓自我檢查每個 session 都報假失敗、而單元測試一直是綠的
- **發現漏網的格式**：`samples()` 或 `keyedSamples()` 加一筆 → 「一般內容不受影響」對照組補一筆相似但不該命中的反例 →
  效能組補一筆病態輸入 → `plugin test` → `fp-scan.mts`
- 擋指令的規則改動後，對照組（不擋）要涵蓋「字串裡出現同樣的字」—— 誤擋會讓正常工作卡住
- `tsconfig.json` 只給編輯器用（引擎不讀），它 `extends` 的 `.claude-plugin/types/` 是引擎在 session 載入 mod 時
  依當地 CLI 版本產生、自帶 `.gitignore` 的目錄；新 clone 還沒載入過時編輯器會報找不到型別，開一次 session 即可
