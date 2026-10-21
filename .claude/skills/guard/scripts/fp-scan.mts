// fp-scan.mts — 用 guard 的真實規則（hooks/redact.ts）掃一個目錄下的文字檔
//
// 兩個用途，結果都是「這些檔被 Read 時會變成遮蔽標記」：
//   1. 誤報：文件裡的佔位範例被當成機密 → 那些檔讀回來是標記、難以編輯
//   2. 真機密被 commit：規則命中真值
//
// 用法（node 22.6+ 直接跑 TypeScript）：
//   node .claude/skills/guard/scripts/fp-scan.mts [目錄]     ## 預設目前目錄；不需要是 git repo
// 跳過 .git/、node_modules/、引擎產生的 .claude-plugin/types/、符號連結、二進位與 3 MB 以上的檔。
// 只印 檔名:行號 與規則名，**不印命中的值**（命中真機密時也不會外洩）。有命中時 exit 1。
// ⚠️ `claude plugin test` 的環境是沙箱、讀不到檔案，所以這支要用 node 跑，不能寫成 *.test.ts

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { findHeadlessTails, RULES } from '../hooks/redact.ts'

const SKIP_DIRS = new Set(['.git', 'node_modules'])
const root = resolve(process.argv[2] ?? '.')

const files: string[] = []
const walk = (dir: string): void => {
  for (const d of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, d.name)
    if (d.isDirectory()) {
      if (SKIP_DIRS.has(d.name) || p.endsWith('/.claude-plugin/types')) continue
      walk(p)
    } else if (d.isFile()) files.push(p)
  }
}
walk(root)

const hits: string[] = []
let scanned = 0
for (const p of files) {
  let t: string
  try {
    if (statSync(p).size > 3_000_000) continue
    t = readFileSync(p, 'utf8')
  } catch {
    continue
  }
  if (t.includes('\0')) continue // 二進位
  scanned++
  const f = relative(root, p)
  for (const r of RULES) {
    for (const m of t.matchAll(new RegExp(r.rx.source, r.rx.flags))) {
      hits.push(`${f}:${t.slice(0, m.index).split('\n').length}  ${r.name}`)
    }
  }
  // RULES 以外的函式型規則（沒有 BEGIN 的 PEM）—— 只掃 RULES 會漏掉它
  for (const [a] of findHeadlessTails(t)) hits.push(`${f}:${t.slice(0, a).split('\n').length}  pem(headless)`)
}

console.log(`掃描 ${root} 下 ${scanned} 個文字檔，命中 ${hits.length} 處`)
for (const h of hits) console.log(`  ${h}`)
process.exit(hits.length > 0 ? 1 : 0)
