import type { Register } from 'claude-code'
import { families, MARKER, redact, redactContent, selfCheck, walk } from './redact'

// 會把內容寫進檔案或交給 shell 的工具 → 檢查的欄位
// MultiEdit 目前已不是工具，保留只為了它若回來仍受保護
const WRITE_FIELDS: Readonly<Record<string, readonly string[]>> = {
  Write: ['content'],
  Edit: ['new_string'],
  MultiEdit: ['edits'],
  NotebookEdit: ['new_source'],
  Bash: ['command'],
  Monitor: ['command'],
}

// 擋寫回時告訴模型「那現在該怎麼做」—— 只說「不行」會讓它停下來問人、或換個寫法後被 classifier 擋
const NEXT_STEP: Readonly<Record<string, string>> = {
  Write: '要複製檔案用 `cp`；要改其他內容用 Edit，只改不含標記的那幾行。',
  Edit: 'old_string / new_string 都只放不含標記的片段（例如只改 `KEY=` 那行的 key 名）。',
  MultiEdit: '每一段都只放不含標記的片段。',
  NotebookEdit: '只改不含標記的 cell 片段。',
  Bash:
    '要用真值就在同一次呼叫裡從來源展開成環境變數（`set -a; . ./<檔>; set +a; <指令>`，或本機的載入函式），不要貼標記；' +
    'grep 標記必然零命中（真檔案裡是真值）。',
  Monitor: '要用真值就在指令裡從來源展開成環境變數，不要貼標記。',
}

// 遮蔽後的下一步：每個 loop（主迴圈 / 各子 agent）第一次給完整版，之後一行
const HINT_FULL = (f: string): string =>
  `guard: 這個結果裡有機密被遮蔽（${f}）。標記不是真值、也不是檔案內容，真檔案沒被改動。下一步依目的：` +
  '只要確認有值 → 印長度（`printf %s "$VAR" | wc -c`）；' +
  '要把值交給指令 → 在同一次 Bash 呼叫裡從來源展開成環境變數讓指令自己讀（`set -a; . ./<檔>; set +a; <指令>`，或本機的載入函式），' +
  '不要用 `$(grep …)` 把值撈進指令字串、不要印出來；要複製 / 改名 → `cp` / `mv`；要改同檔其他行 → Edit 只挑不含標記的片段；' +
  '要改的就是機密值本身 → 交給使用者。被 classifier 擋下就停下來交給使用者，不要再換寫法重試。'
const HINT_SHORT = (f: string): string => `guard: 又遮蔽了機密（${f}）；下一步同這個 loop 第一次的提示。`

// 大輸出在 hook 拿到結果前就已落檔；只改寫 tool-results 目錄下的檔，別碰任意路徑
const PERSISTED_KEYS = ['persistedOutputPath', 'rawOutputPath'] as const
const isToolResultsFile = (p: unknown): p is string => typeof p === 'string' && p.includes('/tool-results/')

// 遮蔽失敗時寧可藏起輸出（fail closed）：hook 一 throw 就被跳過、原文入檔
const WITHHELD = 'guard: 遮蔽過程發生錯誤，為避免機密外洩已隱藏此輸出；請縮小輸出範圍後重跑'
const UNREADABLE =
  'guard: 此輸出無法讀入遮蔽（常見原因：超過 $.fs 的 4 MiB 上限），原檔已以本訊息取代；請加過濾（grep / head）後重跑\n'
// 錯誤結果只能以 deny 帶回（見 tool.call 第 3 步）→ 明講這不是權限拒絕，免得模型改寫指令重試
const ERROR_PREFIX = 'guard（遮蔽後的錯誤輸出）：工具已執行、exit 非 0 —— 這不是權限拒絕。\n'

export const register: Register = on => {
  const hinted = new Set<string>() // agentId（主迴圈用 ''）

  const hint = (agent: string | undefined, v: unknown): string => {
    let f = ''
    try {
      f = Object.keys(families(v)).join(', ')
    } catch {
      // 只是少了家族名，提示照給
    }
    const k = agent ?? ''
    if (hinted.has(k)) return HINT_SHORT(f)
    hinted.add(k)
    return HINT_FULL(f)
  }

  // 自我檢查：規則若被改壞（regex 失效），開 session 時就跳警告，而不是安靜地放行
  // ⚠️ toast 只在有通知列的介面看得到；headless（`claude -p`）只寫 debug log
  on('session.start', async ($, e, next) => {
    try {
      const broken = selfCheck()
      if (broken.length > 0) $.ui.toast(`guard: 遮蔽自我檢查失敗（${broken.join(', ')}）—— 這些格式的機密不會被遮蔽`)
    } catch {
      // 檢查或通知本身失敗不能擋住 session
    }
    return next(e)
  })

  // 第二道網：不經過 tool.call 的列（skill 內文、Monitor 事件、子 agent 交付、hook context…）
  // 使用者自己打的 prompt 不改：它在 queue-operation 另有原樣紀錄，改了也遮不住，只會讓模型讀不到使用者給的東西
  on('session.append', async ($, e, next) => {
    if (e.door === 'prompt') return next(e)
    const { content, changed } = redactContent(e.message.content)
    return changed ? next({ ...e, message: { ...e.message, content } }) : next(e)
  })

  on('tool.call', async ($, e, next) => {
    const agent = (e as { agentId?: string }).agentId
    const input = e as Record<string, unknown>

    // 1. 擋寫回：Read 的結果被遮蔽後，模型可能把標記當成檔案內容寫回去，覆蓋掉真的機密
    const fields = WRITE_FIELDS[e.tool] ?? []
    const hit = fields.find(f => MARKER.test(JSON.stringify(input[f] ?? '')))
    if (hit !== undefined) {
      return {
        deny:
          `guard: ${e.tool}.${hit} 含有遮蔽標記 <redacted:…>（guard 遮掉的機密，不是檔案的真實內容），寫回會用標記覆蓋真值。` +
          (NEXT_STEP[e.tool] ?? '只編輯不含標記的片段。') +
          '要改的就是機密值本身才交給使用者。',
      }
    }

    const r = await next(e)
    if (r.deny !== undefined) return r
    try {
      // 2. 錯誤結果：result 是字串、不符合工具的 output schema，不能以 { result } 回傳
      //    （schema 驗證失敗 → 模型只看到 shape 錯誤）；改以 deny 帶回遮蔽後的錯誤文字
      if (r.isError) {
        const t = r.text ?? (typeof r.result === 'string' ? r.result : '')
        const c = redact(t)
        if (c === t) return r
        return { deny: [`${ERROR_PREFIX}${c}`, hint(agent, c)].join('\n\n') }
      }
      if (r.result === undefined) return r

      // 3. 大輸出落檔：就地改寫；讀不進來（多半是超過 4 MiB）就以提示取代原檔（fail closed）
      const resultRecord = r.result as Record<string, unknown>
      for (const k of PERSISTED_KEYS) {
        const p = resultRecord?.[k]
        if (!isToolResultsFile(p)) continue
        try {
          const raw = await $.fs.read(p)
          const clean = redact(raw)
          if (clean !== raw) await $.fs.write(p, clean)
        } catch {
          try {
            await $.fs.write(p, UNREADABLE)
          } catch {
            $.ui.toast(`guard: 無法改寫大輸出落檔 ${p}，該檔可能留有未遮蔽內容`)
          }
        }
      }

      // 4. 遮蔽結果：回傳不帶 ref 的新 result，core 以它重新 map 並寫進 transcript（含 toolUseResult）
      //    並附下一步提示（model-only 的 context，使用者看不到）
      const w = walk(r.result)
      if (!w.changed) return r
      return { result: w.value as typeof r.result, context: [...(r.context ?? []), hint(agent, w.value)] }
    } catch {
      return { deny: WITHHELD }
    }
  })
}
