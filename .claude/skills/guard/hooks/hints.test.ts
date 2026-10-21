import { describe, expect, test } from 'claude-code/testing'
import { families, mark, samples } from './redact'

// 遮蔽之後教下一步、擋寫回給下一步
// 假值一律用 samples() / mark() 在執行期拼出（理由見 redact.test.ts 檔頭）

const bash = (stdout: string) => () => ({ result: { stdout, stderr: '', interrupted: false } })
const LEN = 'wc -c' // 提示裡「印長度」的寫法（bash / zsh 都能跑）

describe('遮蔽後的下一步提示', () => {
  test('遮蔽結果附 context 提示：家族名 ＋ 下一步；第二次只給短版', async ($, on) => {
    const [, v] = samples()[1]
    on('tool.call', bash(`token=${v}`))
    const a = await $.tool.call({ tool: 'Bash', command: 'cat a' })
    const first = (a.context ?? []).join('\n')
    expect(first).toContain('github')
    expect(first).toContain(LEN)
    expect(first).toContain('set -a')
    expect(first).toContain('cp')
    expect(first).not.toContain(v)
    const b = await $.tool.call({ tool: 'Bash', command: 'cat b' })
    const second = (b.context ?? []).join('\n')
    expect(second).toContain('第一次的提示')
    expect(second.length).toBeLessThan(first.length)
  })

  test('子 agent 各自拿到一次完整版（不被主迴圈用掉）', async ($, on) => {
    const [, v] = samples()[1]
    on('tool.call', bash(`token=${v}`))
    await $.tool.call({ tool: 'Bash', command: 'cat a' })
    const sub = await $.tool.call({ tool: 'Bash', command: 'cat a', agentId: 'sub-1' } as never)
    expect((sub.context ?? []).join('\n')).toContain(LEN)
  })

  test('沒有遮蔽就沒有提示（成功時零 token）', async ($, on) => {
    on('tool.call', bash('hello'))
    const r = await $.tool.call({ tool: 'Bash', command: 'echo hello' })
    expect(r.context ?? []).toEqual([])
  })

  test('錯誤結果：deny 文字說明「工具已執行、不是權限拒絕」並附提示', async ($, on) => {
    const [, v] = samples()[1]
    on('tool.call', () => ({ result: `Exit code 3\ntoken=${v}`, isError: true }) as never)
    const r = await $.tool.call({ tool: 'Bash', command: 'false' })
    expect(r.deny).toContain('不是權限拒絕')
    expect(r.deny).toContain('Exit code 3')
    expect(r.deny).toContain('github')
    expect(r.deny).not.toContain(v)
  })

  test('families：各 family 的筆數', () => {
    expect(families({ a: `${mark('github', 40)} ${mark('github', 40)} ${mark('jwt', 99)}` })).toEqual({ github: 2, jwt: 1 })
    expect(families('plain')).toEqual({})
  })
})

describe('擋寫回的文案給下一步', () => {
  test('Write → 指向 cp / Edit；Bash → 從來源展開；都不再把「交給使用者」當預設', async ($, on) => {
    on('tool.call', bash(''))
    const w = await $.tool.call({ tool: 'Write', file_path: '/x/.env', content: `K=${mark('vault', 28)}\n` })
    expect(w.deny).toContain('`cp`')
    expect(w.deny).toContain('要改的就是機密值本身才交給使用者')
    const b = await $.tool.call({ tool: 'Bash', command: `echo 'K=${mark('slack', 56)}' > f` })
    expect(b.deny).toContain('從來源展開')
  })

  test('NotebookEdit / Monitor 也擋', async ($, on) => {
    let ran = false
    on('tool.call', () => ((ran = true), { result: { stdout: '', stderr: '', interrupted: false } }))
    const n = await $.tool.call({ tool: 'NotebookEdit', notebook_path: '/x.ipynb', new_source: `k = '${mark('jwt', 99)}'` } as never)
    expect(n.deny).toContain('guard:')
    const m = await $.tool.call({ tool: 'Monitor', command: `grep ${mark('github', 40)} app.log` } as never)
    expect(m.deny).toContain('guard:')
    expect(ran).toBe(false)
  })
})
