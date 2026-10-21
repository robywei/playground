// 遮蔽規則：只收「有明確前綴或 key 名錨定」的格式 —— 誤報低，漏報見 README「盲區」
// ⚠️ 不收雲端 AccessKey ID（例：`LTAI…`）：它不是機密，遮掉只會妨礙正常工作；AccessKey Secret 靠 key 名錨定
//
// 🔴 每條規則都必須是線性時間：hook 同步跑在 worker 裡，單次超過 heartbeat（5 秒）worker 會被砍、
//    guard 在該 session 剩下的時間**整個卸載**，之後的結果全部原文入檔（實測：`a-` 重複 250k 次即觸發）。
//    寫法：起點用 lookbehind 錨在「一段字元的開頭」（不用 \b，否則每個字界都重掃一次 → 平方），
//    每個重複都設上限。新增規則一定要在測試的效能組補一筆病態輸入。

type Rule = {
  readonly name: string
  readonly rx: RegExp
  // 有 keep 群組時只遮蔽值、保留 key 名（讓模型看得出是哪個欄位）
  readonly keep?: number
}

// 換行的長相：真換行、JSON 字串裡的 `\n` / `\r\n`（kubectl -o json / 日誌常見），含 JSON 包 JSON 的多重跳脫 `\\n`
const NL = String.raw`(?:\r?\n|(?:\\{1,4}r)?\\{1,4}n)`
// 行首可能有：git diff 刪除行的 `-`、縮排（YAML block scalar、diff context 行）、
// `cat -n`（tab）/ Read 工具（`→`）/ `grep -n`（`:`）的行號。`+` 不放這裡：它也是 base64 字元，放了會讓同一行有兩種切法
const LEAD = String.raw`-?[ \t]{0,16}(?:\d{1,7}[\t→:])?`
const PEM_HEAD = String.raw`-----BEGIN [A-Z0-9 ]{0,40}PRIVATE KEY(?: BLOCK)?-----`
const PEM_TAIL = String.raw`-----END [A-Z0-9 ]{0,40}PRIVATE KEY(?: BLOCK)?-----`
// base64 行必須在行尾結束 —— 否則金鑰被截斷時，後面接的散文單字也會被當成 base64 吃掉
const EOL = String.raw`(?=\r?\n|\\|$|["',])`
// 本體一行：換行（+ 行首）後接「加密金鑰的標頭行」或「base64 行」；
// 或以空白相連的長 base64 段（`echo $KEY` 沒加引號）—— 只收 40 字元以上，免得吃進散文
const PEM_LINE =
  String.raw`(?:${NL}{1,2}${LEAD}(?:[A-Za-z-]{1,20}: [^\r\n\\]{0,200}|[A-Za-z0-9+/=]{1,8192}${EOL})` +
  String.raw`|[ \t][A-Za-z0-9+/=]{40,8192}(?=[ \t]|${EOL}))`

export const RULES: readonly Rule[] = [
  // 至少一段 16 字元以上的 base64 才算（只提到標頭字串的程式碼 / 文件不算）；
  // END 可有可無 —— head / Read limit / 大輸出 preview 都會把金鑰從中間切斷。
  // 標頭與本體之間可以是換行、空白，或直接黏著（`tr -d '\n'`）
  {
    name: 'pem',
    rx: new RegExp(
      `${PEM_HEAD}(?:${NL}${LEAD}[A-Za-z-]{1,20}: [^\\r\\n\\\\]{0,200}){0,4}(?:${NL}${LEAD}|[ \\t]){0,4}[A-Za-z0-9+/=]{16,8192}` +
        // ⚠️ 尾段的分隔不可寫成 (?:${NL}|\s)：真換行兩邊都吃 → 每個換行兩種切法、2^k 回溯
        //    （實測：END 只差一個 `-` 的 108 bytes 輸入跑 7.2 秒 > heartbeat，guard 整個卸載）
        `${PEM_LINE}{0,4000}(?:(?:\\s|\\\\{1,4}[rn]){0,20}(?:[A-Za-z0-9+/=]{1,200}(?:\\s|\\\\{1,4}[rn]){1,20})?${PEM_TAIL})?`,
      'g',
    ),
  },
  // OpenSSH 私鑰本體的固定開頭（`openssh-key-v1\0` 的 base64）—— 連 BEGIN / END 都沒有的片段也認得
  // （`sed -n 2,5p id_ed25519`、逐行處理時把標頭行丟掉）。
  // 續行只收 OpenSSH 固定的 70 字元寬（`+` 前綴是 git diff）＋ 最多一行短尾 ＋ 可選 END：
  // 不沿用 PEM_LINE —— 它收 `Header: value` 行與任意長度的行，沒有 END 收尾時會吃掉後面的 `Status: Running` 之類正常輸出
  {
    name: 'pem',
    // lookbehind 不排除 `=`：`id_ed25519=b3Bl…` 這種 key=value 寫法也要認得
    rx: new RegExp(
      String.raw`(?<![A-Za-z0-9+/])b3Blbn` + String.raw`NzaC1rZXktdjE[A-Za-z0-9+/=]{20,8192}` +
        String.raw`(?:(?:${NL}${LEAD}\+?|[ \t]+)[A-Za-z0-9+/]{70}(?=[ \t]|${EOL})){0,4000}` +
        String.raw`(?:(?:${NL}${LEAD}\+?|[ \t]+)[A-Za-z0-9+/=]{1,70}(?=[ \t]|${EOL}))?` +
        String.raw`(?:(?:${NL}${LEAD}|[ \t]){1,4}${PEM_TAIL})?`,
      'g',
    ),
  },
  { name: 'github_pat', rx: /(?<![A-Za-z0-9_])github_pat_[A-Za-z0-9_]{60,255}/g },
  { name: 'github', rx: /(?<![A-Za-z0-9_])gh[pousr]_[A-Za-z0-9_.-]{36,1024}/g },
  // 斜線有三種寫法：`/`、JSON 跳脫的 `\/`、URL 編碼的 `%2F`；最後一段（secret）至少 16 字元，
  // 文件裡的 `services/YOUR/WEBHOOK/URL` 這類佔位不算
  {
    name: 'slack_webhook',
    rx: /https?(?::\/\/|:\\\/\\\/|%3A%2F%2F)hooks\.slack\.com(?:\/|\\\/|%2F)(?:services|workflows|triggers)(?:(?:\/|\\\/|%2F)[A-Za-z0-9_-]{1,64}){1,5}(?:\/|\\\/|%2F)[A-Za-z0-9_-]{16,64}(?![A-Za-z0-9_-])/gi,
  },
  { name: 'slack', rx: /(?<![A-Za-z0-9_-])xox[abeoprs]-[0-9A-Za-z-]{10,255}/g },
  { name: 'slack_app', rx: /(?<![A-Za-z0-9_-])xapp-[0-9]-[A-Za-z0-9-]{10,255}/g },
  { name: 'cloudflare', rx: /(?<![A-Za-z0-9_])cf(?:at|ut)_[A-Za-z0-9_-]{40,255}/g },
  { name: 'vault', rx: /(?<![A-Za-z0-9_.])hv[sbr]\.[A-Za-z0-9_-]{24,1024}/g },
  // eyJ = `{"`、eyA = `{ "`、ew = `{\n`；允許 padding 與標準 base64（+/）
  {
    name: 'jwt',
    rx: /(?<![A-Za-z0-9_+/-])e[wy][A-Za-z0-9_+/-]{8,4096}={0,2}\.e[wy][A-Za-z0-9_+/-]{8,8192}={0,2}\.[A-Za-z0-9_+/-]{8,4096}={0,2}/g,
  },
  { name: 'anthropic', rx: /(?<![A-Za-z0-9_-])sk-ant-[A-Za-z0-9_-]{20,255}/g },
  { name: 'openai', rx: /(?<![A-Za-z0-9_-])sk-(?:proj|svcacct|admin)-[A-Za-z0-9_-]{20,255}/g },
  { name: 'google', rx: /(?<![A-Za-z0-9_-])AIza[0-9A-Za-z_-]{35}(?![A-Za-z0-9_-])/g },
  { name: 'onepassword', rx: /(?<![A-Z0-9-])A3-[A-Z0-9]{6}-[A-Z0-9]{6}-[A-Z0-9]{5}-[A-Z0-9]{5}-[A-Z0-9]{5}-[A-Z0-9]{5}(?![A-Z0-9])/g },
  // key 名錨定（大小寫不拘、含 JSON 跳脫的引號與 CLI 旗標）；STS 臨時憑證的 secret 比 30 字元長
  {
    name: 'aliyun_sk',
    rx: /((?:access[_-]?key[_-]?secret|secret[_-]?access[_-]?key|ALI(?:BABA_CLOUD|YUN|CLOUD)_[A-Z_]{0,40}SECRET[A-Z_]{0,40}|ALI_KEY_[A-Z0-9_]{0,40})\\?["']?\s{0,4}[:=]\s{0,4}\\?["']?|--access-key-secret[ =])([A-Za-z0-9]{30,128})(?![A-Za-z0-9])/gi,
    keep: 1,
  },
  {
    name: 'aliyun_sts',
    rx: /((?:security[_-]?token|sts[_-]?token)\\?["']?\s{0,4}[:=]\s{0,4}\\?["']?|--sts-token[ =])([A-Za-z0-9+/=]{100,4096})/gi,
    keep: 1,
  },
  // URL 內嵌密碼：只遮密碼段；**整個值**是佔位（$VAR / ${X} / <x> / *** / xxx / password）才放過；
  // `host:8080?x=a@b` 這種「其實是 port」的不算
  {
    name: 'url_password',
    rx: /((?<![A-Za-z0-9+.-])[A-Za-z][A-Za-z0-9+.-]{0,31}:\/\/[^/\s:@]{0,256}:)(?!\d{1,5}[/?#])(?!(?:\$\{?[A-Za-z_][A-Za-z0-9_]{0,64}\}?|<[^>@\s]{1,64}>|\*{3,64}|[xX]{3,64}|pass(?:word)?|secret|changeme)@)([^@\s/]{8,256})(?=@)/g,
    keep: 1,
  },
]

// 遮蔽標記。README 舉例時寫成 `len=N`（非數字），不會被下方 MARKER 誤判
export const mark = (name: string, len: number): string => `<redacted:${name} len=${len}>`
export const MARKER = /<redacted:[a-z0-9_]+ len=\d+>/

// 結果裡各 family 的遮蔽次數（提示文案用）
export const families = (v: unknown): Record<string, number> => {
  const out: Record<string, number> = {}
  for (const m of JSON.stringify(v ?? '').matchAll(/<redacted:([a-z0-9_]+) len=\d+>/g)) {
    const k = m[1] ?? ''
    out[k] = (out[k] ?? 0) + 1
  }
  return out
}

// 只有 END、沒有 BEGIN 的 PEM（`tail key.pem`、逐行處理時把標頭行丟掉）：以 END 行為錨點**往回**收 base64 本體。
// 用函式不用 regex —— 「一串 base64 行 + END」寫成 regex 時，沒有 END 的長 base64 會在每段開頭重掃一次（平方）。
// 線性的理由：END 一定以 `-` 結尾，而往回走遇到它一定停 → 各個 END 走過的區段互不重疊（PEM_TAIL_BACK 只是本體上限）。
// 收進來的判準（對抗審查後定案，見 redact.test.ts 的實例）：
//   - 最靠近 END 的那段是金鑰最後一行，長度不限；更前面的行**寬度一致**且是 64（PEM）/ 70（OpenSSH）/ 76（MIME）
//     → 前面剛好有 SHA、指紋、公鑰、長 token 時不會被一起吞掉
//   - 純 hex 段不收（commit SHA、image digest）；只有 `=` 的段不收（分隔線）
//   - 只有一段時要 64 字元以上 —— 除非它緊接在 pem 遮蔽標記之後（OpenSSH 前綴規則吃掉前面幾行、只剩短尾 + END）
//   - 行首前綴當分隔：`cat -n`（tab）/ Read 工具（`→`）/ `grep -n`（`:`）的行號、git diff 的 `-`、反斜線（多重跳脫、行尾 `\`）；
//     行首的 `+`（git diff 新增行）以寬度判斷是前綴還是 base64
// 實例：比對 SSH key 指紋時把解碼值逐行印出、BEGIN 行沒印到 → 只認 BEGIN 的規則整段放行
const PEM_TAIL_RX = new RegExp(PEM_TAIL, 'g')
const PEM_TAIL_BACK = 65_536
const PEM_MARK_TAIL = /<redacted:pem len=\d+>$/
const PEM_WIDTHS: ReadonlySet<number> = new Set([64, 70, 76])
const HEX_ONLY = /^[0-9a-fA-F]+$/
const isB64 = (c: string | undefined): boolean => c !== undefined && /[A-Za-z0-9+/=]/.test(c)
const isWs = (c: string | undefined): boolean => c === ' ' || c === '\t' || c === '\n' || c === '\r'
const isDigit = (c: string | undefined): boolean => c !== undefined && c >= '0' && c <= '9'
// s[i-2..i-1] 是 JSON 跳脫的 `\n` / `\r`
const isEsc = (s: string, i: number): boolean => s[i - 2] === '\\' && (s[i - 1] === 'n' || s[i - 1] === 'r')
// 位置 i 是不是一行的開頭（前一格是換行 / 空白 / 反斜線 / 跳脫換行，或已到邊界）
const isLineStart = (s: string, i: number, floor: number): boolean =>
  i <= floor || isWs(s[i - 1]) || s[i - 1] === '\\' || isEsc(s, i)
// 行號前綴（1–7 位數 + tab / → / :）：回傳跳過它之後的位置；不是就回傳 k 本身
const skipLineNo = (s: string, k: number, floor: number): number => {
  const c = s[k - 1]
  if (c !== '\t' && c !== '→' && c !== ':') return k
  let p = k - 1
  while (p > floor && isDigit(s[p - 1])) p--
  const n = k - 1 - p
  if (n === 0 || n > 7 || !isLineStart(s, p, floor)) return k
  return p
}
// 回傳要遮蔽的 [start, end) 範圍（遞增、不重疊）；scripts/fp-scan.mts 也用它，掃描與 mod 共用同一份實作
export const findHeadlessTails = (s: string): Array<readonly [number, number]> => {
  const spans: Array<readonly [number, number]> = []
  if (!s.includes('-----END ')) return spans
  let done = 0 // 上一個範圍的結尾；往回走不越過它
  for (const m of s.matchAll(PEM_TAIL_RX)) {
    const at = m.index ?? 0
    const floor = Math.max(done, at - PEM_TAIL_BACK)
    let j = at
    let stop = at // 往回走停下的位置（s[stop - 1] 是擋住它的字元）
    let start = -1
    let body = 0
    let runs = 0
    let last = 0 // 最靠近 END 那段（最後一行）的長度
    let width = 0
    while (j > floor) {
      let k = j // 先吃分隔
      while (k > floor) {
        const q = skipLineNo(s, k, floor)
        if (q < k) k = q
        else if (isWs(s[k - 1]) || s[k - 1] === '\\') k--
        else if (k - 1 > floor && isEsc(s, k)) k -= 2
        else if (s[k - 1] === '-' && isLineStart(s, k - 1, floor)) k-- // git diff 刪除行
        else break
      }
      stop = k
      let r = k // 再吃一段 base64（遇到跳脫換行的 n / r 就停；`=` 只能是段尾的 padding —— `KEY=MIIE…` 停在 `=`）
      let data = false
      while (r > floor && isB64(s[r - 1]) && !isEsc(s, r)) {
        if (s[r - 1] !== '=') data = true
        else if (data) {
          if (isLineStart(s, r - 1, floor)) r-- // PGP armor 的 checksum 行 `=XXXX`
          break
        }
        r--
      }
      const len = k - r
      if (len === 0 || !data) break
      // 行首單獨一個 `+`（git diff 新增行，含 END 那行自己的 `+`）只是前綴 → 當分隔
      if (len === 1 && s[r] === '+' && isLineStart(s, r, floor)) {
        j = r
        continue
      }
      if (len >= 16 && HEX_ONLY.test(s.slice(r, k))) break
      const plus = s[r] === '+' && isLineStart(s, r, floor) // 可能是 git diff 的 `+` 前綴
      if (runs === 0) last = len
      else if (runs === 1) {
        width = PEM_WIDTHS.has(len) ? len : plus && PEM_WIDTHS.has(len - 1) ? len - 1 : 0
        if (width === 0 || last > width + 1) break
      } else if (len !== width && !(plus && len - 1 === width)) break
      start = r
      body += len
      runs++
      j = r
    }
    if (start < 0) continue
    // 緊接在 pem 標記之後 = 前面幾行已被別條規則遮掉，剩下的短尾一定是同一把金鑰 → 不套長度下限
    const afterMark = s[stop - 1] === '>' && PEM_MARK_TAIL.test(s.slice(Math.max(0, stop - 32), stop))
    if (!afterMark && (body < 16 || (runs === 1 && body < 64))) continue
    const end = at + m[0].length
    spans.push([start, end])
    done = end
  }
  return spans
}
const redactHeadlessTail = (s: string): string => {
  const spans = findHeadlessTails(s)
  if (spans.length === 0) return s
  let out = ''
  let done = 0
  for (const [a, b] of spans) {
    out += s.slice(done, a) + mark('pem', b - a)
    done = b
  }
  return out + s.slice(done)
}

export const redact = (s: string): string =>
  redactHeadlessTail(
    RULES.reduce(
      (acc, r) =>
        acc.replace(r.rx, (m: string, ...g: unknown[]) => {
          if (r.keep === undefined) return mark(r.name, m.length)
          const kept = g[r.keep - 1] as string
          return `${kept}${mark(r.name, m.length - kept.length)}`
        }),
      s,
    ),
  )

// 走訪整個 result，所有字串欄位都遮蔽；結構與型別不變，才能通過工具的 output schema
// - `base64` 欄位（Read 圖片 / PDF）不碰：不會含文字機密，且動了會毀掉資料
// - 必須是全函數（不 throw）：hook 一 throw 就被跳過、原文入檔 → 環狀參照與 BigInt 都要能處理
export const walk = (v: unknown): { value: unknown; changed: boolean } => {
  const seen = new WeakSet<object>()
  let changed = false
  const go = (x: unknown, key: string): unknown => {
    if (typeof x === 'string') {
      if (key === 'base64') return x
      const r = redact(x)
      if (r !== x) changed = true
      return r
    }
    if (typeof x === 'bigint') return x
    if (x === null || typeof x !== 'object') return x
    if (seen.has(x)) return x
    seen.add(x)
    if (Array.isArray(x)) return x.map(y => go(y, ''))
    return Object.fromEntries(Object.entries(x).map(([k, y]) => [k, go(y, k)]))
  }
  const value = go(v, '')
  return { value, changed }
}

// session.append 的列內容：text 區塊與 tool_result 的內容遮蔽，其他區塊（thinking、tool_use、圖片）原樣保留
type Block = { readonly type: string; readonly text?: string; readonly content?: unknown }
export const redactContent = <T extends Block>(blocks: readonly T[]): { content: T[]; changed: boolean } => {
  let changed = false
  const content = blocks.map(b => {
    if (b.type === 'text' && typeof b.text === 'string') {
      const t = redact(b.text)
      if (t === b.text) return b
      changed = true
      return { ...b, text: t }
    }
    if (b.type === 'tool_result') {
      const w = walk(b.content)
      if (!w.changed) return b
      changed = true
      return { ...b, content: w.value }
    }
    return b
  })
  return { content, changed }
}

// 自我檢查與測試用的樣本：執行期拼出、形狀正確的假值 —— repo 裡不留任何字面 token 或 PEM 標頭
// （GitHub push protection 會掃 webhook 等格式；guard 生效後字面值會讓這個檔自己讀回來就是標記）
// ⚠️ 只能**往後加**：測試用索引取值
const A = (n: number): string => 'A1b2C3d4E5'.repeat(Math.ceil(n / 10)).slice(0, n)
export const pemHeader = (w: 'BEGIN' | 'END', kind = 'RSA'): string => `${'-'.repeat(5)}${w} ${kind} PRIV${'ATE KEY'}${'-'.repeat(5)}`
export const samples = (): ReadonlyArray<readonly [string, string]> => [
  ['github_pat', 'github' + '_pat_' + A(82)],
  ['github', 'gh' + 'p_' + A(36)],
  ['slack_webhook', 'https://hooks.slack.com/' + 'services/T' + A(8) + '/B' + A(10) + '/' + A(24)],
  ['slack', 'xo' + 'xb-' + '1234567890123-1234567890123-' + A(24)],
  ['cloudflare', 'cf' + 'at_' + A(48)],
  ['vault', 'hv' + 's.' + A(24)],
  ['jwt', 'ey' + 'J' + A(20) + '.ey' + 'J' + A(20) + '.' + A(40)],
  ['pem', pemHeader('BEGIN') + '\n' + 'QUJD'.repeat(16) + '\n' + 'QUJD'.repeat(4) + '\n' + pemHeader('END')],
  ['slack_app', 'xa' + 'pp-1-' + A(20)],
  ['anthropic', 'sk-' + 'ant-' + A(30)],
  ['openai', 'sk-' + 'proj-' + A(30)],
  ['google', 'AI' + 'za' + A(35)],
  ['onepassword', 'A' + '3-ABCDEF-GHIJKL-MNOPQ-RSTUV-WXYZ1-23456'],
  // 沒有 BEGIN 的 PEM（以 END 往回收）、OpenSSH 本體固定開頭（BEGIN / END 都沒有）
  ['pem', 'QUJD'.repeat(16) + '\n' + 'REVG'.repeat(3) + '\n' + pemHeader('END', 'OPENSSH')],
  ['pem', 'b3Blbn' + 'NzaC1rZXktdjE' + A(120)],
]
// key 名錨定的規則：樣本是「key + 值 + 後綴」，遮蔽後只剩 key 名與後綴
// 後綴是規則成立的必要上下文（url_password 要看到 `@host` 才認得出是密碼）—— 自我檢查與測試都用同一份
export const keyedSamples = (): ReadonlyArray<readonly [string, string, string, string]> => [
  ['aliyun_sk', 'AccessKey' + 'Secret: ', A(30), ''],
  ['aliyun_sts', 'Security' + 'Token=', A(120), ''],
  ['url_password', 'postgres://app:', 's3cr3t' + 'Passw0rd', '@db'],
]

// 開 session 時的自我檢查：回傳「樣本沒被遮蔽」的規則名（應為空）
// ⚠️ 不要在 register.ts 另拼樣本：曾經漏了 url_password 的 `@db` → 每個 session 都報假失敗（單元測試有補所以一直是綠的）
export const selfCheck = (): string[] => [
  ...samples().filter(([, v]) => redact(v) === v).map(([n]) => n),
  ...keyedSamples().filter(([, k, v, s]) => redact(k + v + s) === k + v + s).map(([n]) => n),
]
