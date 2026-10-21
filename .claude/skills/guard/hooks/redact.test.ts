import { describe, expect, test } from 'claude-code/testing'
import { keyedSamples, mark, MARKER, pemHeader, redact, redactContent, samples, selfCheck, walk } from './redact'

// 所有假值、PEM 標頭與遮蔽標記都在執行期拼出（samples() / pemHeader() / mark()）：
// repo 裡不留字面 token、字面標頭、字面標記 —— 否則 guard 生效後，讀這個檔看到的就是標記，
// 連改它都會被自己的「擋寫回」擋下

const B = pemHeader('BEGIN')
const E = pemHeader('END')
const BODY = ['QUJD'.repeat(16), 'REVG'.repeat(16), 'R0hJ'.repeat(8)]

describe('redact()：每條規則都有樣本', () => {
  test('前綴型樣本被遮蔽、不留原值', () => {
    for (const [name, v] of samples()) {
      const out = redact(`x=${v};`)
      expect(out).not.toContain(v)
      expect(out).toContain(mark(name, v.length))
    }
  })

  test('key 名錨定型樣本：只遮值、保留 key', () => {
    for (const [name, k, v, s] of keyedSamples()) {
      expect(redact(k + v + s)).toBe(`${k}${mark(name, v.length)}${s}`)
    }
  })

  test('開 session 的自我檢查回報 0 項（它與上面兩條用同一份樣本 —— 曾經因為各拼各的而每個 session 都報假失敗）', () => {
    expect(selfCheck()).toEqual([])
  })
})

describe('pem', () => {
  test('完整區塊整塊替換', () => {
    const pem = [B, ...BODY, E].join('\n')
    expect(redact(`a\n${pem}\nb`)).toBe(`a\n${mark('pem', pem.length)}\nb`)
  })

  test('沒有 END（head / Read limit / preview 截斷）也遮', () => {
    const cut = [B, BODY[0], BODY[1]].join('\n')
    const out = redact(`${cut}\nSome prose here`)
    expect(out).not.toContain(BODY[0])
    expect(out).not.toContain(BODY[1])
    expect(out).toContain('Some prose here') // 截斷後面的散文不被吃掉
  })

  test('加密金鑰的標頭行（Proc-Type / DEK-Info）', () => {
    const pem = [B, 'Proc-Type: 4,ENCRYPTED', 'DEK-Info: AES-128-CBC,ABCDEF0123456789', '', ...BODY, E].join('\n')
    expect(redact(pem)).toBe(mark('pem', pem.length))
  })

  test('JSON 跳脫的換行（kubectl -o json）', () => {
    const pem = [B, ...BODY, E].join('\\n')
    expect(redact(`{"key": "${pem}"}`)).toBe(`{"key": "${mark('pem', pem.length)}"}`)
  })

  test('PGP 私鑰', () => {
    const b = B.replace('RSA PRIVATE KEY', 'PGP PRIVATE KEY BLOCK')
    const e = E.replace('RSA PRIVATE KEY', 'PGP PRIVATE KEY BLOCK')
    const pem = [b, '', ...BODY, '=AbCd', e].join('\n')
    expect(redact(pem)).toBe(mark('pem', pem.length))
  })

  test('縮排（YAML block scalar）、cat -n、git diff context 行', () => {
    for (const lead of ['  ', '\t', '      ']) {
      const yaml = `tls.key: |\n${lead}${[B, ...BODY, E].join(`\n${lead}`)}\nnext: x`
      const out = redact(yaml)
      for (const l of BODY) expect(out).not.toContain(l)
      expect(out).toContain('next: x')
    }
    const catn = [B, ...BODY, E].map((l, i) => `     ${i + 1}\t${l}`).join('\n')
    for (const l of BODY) expect(redact(catn)).not.toContain(l)
  })

  test('以空白相連（echo $KEY）、標頭黏著本體、整段一行', () => {
    for (const v of [[B, ...BODY, E].join(' '), B + BODY.join(''), B + '\n' + BODY.join('').repeat(8) + '\n' + E]) {
      const out = redact(v)
      for (const l of BODY) expect(out).not.toContain(l)
    }
  })

  test('只提到標頭字串的程式碼不受影響', () => {
    const code = [`awk '/${B}/,/${E}/' key.pem`, 'kubectl --context x get pods', `grep -c "${E}" "$f"`].join('\n')
    expect(redact(code)).toBe(code)
  })
})

describe('pem：沒有 BEGIN（以 END 往回收）與 OpenSSH 本體（實例 ＋ 對抗審查）', () => {
  const OE = pemHeader('END', 'OPENSSH')
  const OB = pemHeader('BEGIN', 'OPENSSH')
  const PE = pemHeader('END', 'PGP') + ''
  // 真實行寬：OpenSSH 每行 70、首行是 `openssh-key-v1` 的 base64；PEM 每行 64；最後一行較短
  const ssh = (tail: string): string[] => ['b3Blbn' + 'NzaC1rZXktdjE' + 'QUJD'.repeat(12) + 'QUJ', 'REVG'.repeat(17) + 'RE', 'R0hJ'.repeat(17) + 'R0', tail]
  const SSH = ssh('SktM'.repeat(8))
  const RSA = ['QUJD'.repeat(16), 'REVG'.repeat(16), 'R0hJ'.repeat(16), 'S0xN']
  const gone = (out: string, lines: string[]) => {
    for (const l of lines) expect(out).not.toContain(l.slice(0, 12))
  }

  test('實際外洩的形狀：逐行以空白相連、前有 key 名、後接引號（BEGIN 行沒印到）', () => {
    for (const tail of ['SktM'.repeat(8), 'SktMSktMSktM']) { // 最後一行短於 16 字元時曾留下一截（接縫）
      const lines = ssh(tail)
      const out = redact(`ctx-a   id_ed25519 ${lines.join(' ')} ${OE} ' 0a1b2c3d4e5f`)
      gone(out, lines)
      expect(out).toContain('id_ed25519 ')
      expect(out).toContain("' 0a1b2c3d4e5f")
    }
  })

  test('換行、CRLF、JSON 跳脫、多重跳脫、行尾 \\、cat -n、Read 的 →、grep -n、YAML 縮排', () => {
    for (const sep of ['\n', '\r\n', '\\n', '\\\\n', '\\\n']) {
      const v = [...RSA, E].join(sep)
      expect(redact(`x ${v} y`)).toBe(`x ${mark('pem', v.length)} y`)
    }
    for (const fmt of [(l: string, i: number) => `     ${i + 2}\t${l}`, (l: string, i: number) => `     ${i + 2}→${l}`,
      (l: string, i: number) => `${i + 2}:${l}`, (l: string) => `    ${l}`]) {
      gone(redact([...RSA, E].map(fmt).join('\n')), RSA.slice(0, 3))
      gone(redact([...SSH, OE].map(fmt).join('\n')), SSH.slice(0, 3))
    }
  })

  test('git diff 的 + / - 前綴（新增 / 刪除私鑰）', () => {
    for (const p of ['+', '-']) {
      gone(redact([...RSA, E].map(l => p + l).join('\n')), RSA.slice(0, 3))
      gone(redact([...SSH, OE].map(l => p + l).join('\n')), SSH.slice(0, 3))
    }
  })

  test('`KEY=` 停在 `=`、整段一行（tr -d）也遮', () => {
    const kv = `PRIVATE_KEY=${RSA.join('')}${E}`
    expect(redact(kv)).toBe(`PRIVATE_KEY=${mark('pem', kv.length - 'PRIVATE_KEY='.length)}`)
  })

  test('沒有 BEGIN 的 PGP 私鑰（checksum 行 `=XXXX`）', () => {
    const pgp = [...RSA.slice(0, 3), '=AbCd', PE.replace('PGP PRIVATE KEY', 'PGP PRIVATE KEY BLOCK')].join('\n')
    gone(redact(pgp), RSA.slice(0, 3))
  })

  test('OpenSSH 本體：BEGIN / END 都沒有也遮（`sed -n 2,5p id_ed25519`）；key=value 也認得', () => {
    for (const v of [SSH.join('\n'), SSH.join(' '), 'id_ed25519=' + SSH.join('\\n')]) gone(redact(v), SSH.slice(0, 3))
  })

  test('完整區塊照舊整塊替換（不會被兩條規則各切一半）', () => {
    const pem = [OB, ...SSH, OE].join('\n')
    expect(redact(`a\n${pem}\nb`)).toBe(`a\n${mark('pem', pem.length)}\nb`)
  })

  test('誤報對照組：只提到 END、前面只有短字或單一 token、分隔線、公鑰', () => {
    const sha = '3f9a2c7e1b'.repeat(4)
    for (const s of [
      `grep -c "${E}" "$f"`,
      `tail -1 key.pem  ## 應印出 ${E}`,
      `echo ${E}`,
      `${E}\n${E}`,
      `cat > k <<EOF\n$SSHPrivateKeyContent\n${E}\nEOF`,
      `INFO loaded runnerIdentityKeyEd25519 trailer=${E}`,
      `Base64EncodedPrivateKeyMaterial ${E}`,
      `${sha}\n${sha}\n${E}`, // commit SHA
      `ghcr.io/x@sha256:${sha}${sha.slice(0, 24)}\n${E}`, // image digest
      '='.repeat(20) + '\n' + E, // 分隔線
      'ssh-ed25519 ' + 'AAAAC3NzaC1lZDI1NTE5AAAAI' + 'QUJD'.repeat(8) + ' user@host', // 公鑰不是機密
    ]) {
      expect(redact(s)).toBe(s)
    }
  })

  test('遮蔽範圍不吞前後的正常內容：SHA、指紋、公鑰、OpenSSH 片段後面的輸出', () => {
    const sha = '3f9a2c7e1b'.repeat(4)
    const fp = 'SHA256:' + 'xQ9z'.repeat(11).slice(0, 43)
    const pub = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI' + 'ZXhh'.repeat(11).slice(0, 43) // 填充字不可與私鑰樣本相同，否則 gone() 會誤判
    for (const pre of [sha, fp, pub]) {
      const out = redact(`${pre}\n${[...RSA, E].join('\n')}`)
      expect(out).toContain(pre)
      gone(out, RSA.slice(0, 3))
    }
    const tailOut = redact([...SSH.slice(0, 3), 'Status: Running', 'Ready: True', 'OK'].join('\n'))
    expect(tailOut).toContain('Status: Running\nReady: True\nOK')
  })
})

describe('url_password', () => {
  test('遮密碼；空 user、密碼含 $ 也遮', () => {
    expect(redact('redis://:' + 'aVeryL0ngPw' + '@cache:6379')).toBe(`redis://:${mark('url_password', 11)}@cache:6379`)
    expect(redact('mysql://u:' + 'pa$$w0rd!x' + '@h/db')).toBe(`mysql://u:${mark('url_password', 10)}@h/db`)
    expect(redact('JDBC:POSTGRESQL://u:' + 's3cr3tPassw0rd' + '@h')).toContain(mark('url_password', 14))
  })

  test('整個值是佔位、或其實是 port 時不遮', () => {
    for (const s of [
      'postgres://u:${DB_PASSWORD}@h/x',
      'postgres://u:$DB_PASSWORD@h/x',
      'mysql://u:<password>@h',
      'redis://u:********@h',
      'http://user:password@example.com',
      'http://host:8080?q=a@b.c',
      'git@github.com:org/repo.git',
    ]) {
      expect(redact(s)).toBe(s)
    }
  })
})

describe('其他格式變體', () => {
  test('Slack webhook：JSON 跳脫與 URL 編碼的斜線', () => {
    const [, url] = samples()[2]
    for (const v of [url.replace(/\//g, '\\/'), encodeURIComponent(url)]) {
      expect(redact(v)).not.toContain(url.split('/').pop() as string)
    }
  })

  test('aliyun_sk：AccessKey Secret 的各種 key 寫法與 STS 長度', () => {
    const sk = 'Ab3'.repeat(10)
    for (const k of ['accessKeySecret: ', 'ALIYUN_ACCESS_KEY_SECRET=', '--access-key-secret ', '\\"AccessKeySecret\\": \\"']) {
      expect(redact(k + sk)).toBe(k + mark('aliyun_sk', 30))
    }
    expect(redact('AccessKeySecret: ' + 'Ab3'.repeat(15))).toBe('AccessKeySecret: ' + mark('aliyun_sk', 45))
  })

  test('JWT：padding 與 eyA（含空白的 JSON）', () => {
    const j = 'ey' + 'A' + 'iYWxnIjoi' + '==' + '.ey' + 'J' + 'zdWIiOiJ4In0' + '.' + 'c2lnbmF0dXJl'
    expect(redact(`Bearer ${j}`)).toBe(`Bearer ${mark('jwt', j.length)}`)
  })
})

describe('一般內容不受影響（誤報對照組）', () => {
  test('常見非機密字串原樣通過', () => {
    const plain = [
      'commit 3f9a2c1d8e7b6a5f4e3d2c1b0a9f8e7d6c5b4a3f', // 40-hex SHA 不是 token
      'LTAI 開頭的 AK ID 刻意不遮', // AK ID 不是機密
      'ghs_APPID_JWT 是 token 格式的文字說明',
      'kubectl --context ctx-prod get pods',
      'svc-1.ns.svc.cluster.local:5432',
      'https://hooks.slack.com/services/YOUR/WEBHOOK/URL', // 文件佔位
      '<redacted:FAMILY len=N> 是 README 的舉例寫法',
    ]
    for (const s of plain) expect(redact(s)).toBe(s)
  })

  test('MARKER 認得遮蔽標記、不認 README 的舉例寫法', () => {
    expect(MARKER.test(redact(samples()[0][1]))).toBe(true)
    expect(MARKER.test('<redacted:FAMILY len=N>')).toBe(false)
  })
})

describe('效能：病態輸入必須線性（超過 heartbeat 5 秒 worker 被砍、guard 整個卸載）', () => {
  const fast = (label: string, s: string) => {
    const t0 = performance.now()
    redact(s)
    const ms = performance.now() - t0
    if (ms > 300) throw new Error(`${label}: ${ms.toFixed(0)}ms`)
  }
  test('長串 a- / a.（url_password）', () => {
    fast('a-', 'a-'.repeat(250_000))
    fast('a.', 'a.'.repeat(250_000))
  })
  test('eyJ 鏈（jwt）', () => fast('eyJ', 'eyJabcdefgh-'.repeat(40_000)))
  test('大量 BEGIN 標頭沒有 END（pem）', () => fast('pem', `${B}\n`.repeat(40_000)))
  test('PEM 標頭後接超長單行（pem）', () => fast('pem-long', `${B}\n` + 'QUJD'.repeat(500_000)))
  test('大量縮排的 BEGIN 標頭（pem）', () => fast('pem-indent', `${B}\n      `.repeat(40_000)))
  test('長 base64 單行', () => fast('b64', 'QUJD'.repeat(500_000)))
  // 以 END 往回收（函式）與 OpenSSH 前綴（regex）
  test('超長 base64 後接一個 END（往回最多走 16 KiB）', () => fast('tail-long', 'QUJD'.repeat(500_000) + '\n' + E))
  test('大量 END、之間只有短字（每個 END 都往回走但很快停）', () => fast('tail-many', `ab ${E}\n`.repeat(40_000)))
  test('大量 16 字元的 base64 段相連、最後才有 END', () => fast('tail-runs', ('QUJD'.repeat(4) + ' ').repeat(100_000) + E))
  test('大量 OpenSSH 前綴（openssh）', () => fast('openssh', ('b3Blbn' + 'NzaC1rZXktdjE' + 'QUJD'.repeat(5) + ' ').repeat(20_000)))
  // 實測：舊 pem 規則尾段 (?:NL|\s) 對換行有兩種切法 → 108 bytes 跑 7.2 秒
  test('BEGIN ＋ 兩段換行 ＋ 只差一個 - 的 END（舊規則尾段指數回溯）', () => {
    const half = `${B}\n` + 'QUJD'.repeat(4) + '\n'.repeat(14) + 'QUJD' + '\n'.repeat(14) + E.slice(0, -1)
    fast('pem-tail-backtrack', half)
    fast('pem-tail-backtrack-crlf', `${B}\n` + 'QUJD'.repeat(4) + '\r\n'.repeat(14) + 'QUJD' + '\r\n'.repeat(14) + E.slice(0, -1))
    fast('pem-tail-backtrack-x', half.repeat(4000))
  })
  test('大量行號前綴 ＋ END（往回收的分隔）', () => fast('lineno', ('     12→' + 'QUJD'.repeat(16) + '\n').repeat(40_000) + E))
})

describe('walk()', () => {
  test('環狀參照與 BigInt 不 throw', () => {
    const o: Record<string, unknown> = { s: samples()[1][1], n: BigInt(1) }
    o.self = o
    const w = walk(o)
    expect(w.changed).toBe(true)
    expect((w.value as Record<string, unknown>).s).toBe(mark('github', 40))
  })
})

describe('tool.call', () => {
  test('Bash 輸出被遮蔽', async ($, on) => {
    const [, v] = samples()[1]
    on('tool.call', () => ({ result: { stdout: `token=${v}`, stderr: '', interrupted: false } }))
    const r = await $.tool.call({ tool: 'Bash', command: 'cat x' })
    expect(JSON.stringify(r.result)).not.toContain(v)
    expect(JSON.stringify(r.result)).toContain(mark('github', 40))
  })

  test('Read 的檔案內容被遮蔽', async ($, on) => {
    const [, v] = samples()[5]
    on('tool.call', () => ({
      result: { type: 'text', file: { filePath: '/x/.env', content: `VAULT_TOKEN=${v}\n`, numLines: 1, startLine: 1, totalLines: 1 } },
    }))
    const r = await $.tool.call({ tool: 'Read', file_path: '/x/.env' })
    expect(JSON.stringify(r.result)).not.toContain(v)
  })

  test('錯誤結果：以 deny 帶回遮蔽後的錯誤文字（字串 result 不符合 output schema）', async ($, on) => {
    const [, v] = samples()[1]
    on('tool.call', () => ({ result: `Exit code 3\ntoken=${v}`, isError: true }) as never)
    const r = await $.tool.call({ tool: 'Bash', command: 'false' })
    expect(r.deny).toContain('Exit code 3')
    expect(r.deny).toContain(mark('github', 40))
    expect(r.deny).not.toContain(v)
  })

  test('無機密的結果原樣通過', async ($, on) => {
    on('tool.call', () => ({ result: { stdout: 'hello', stderr: '', interrupted: false } }))
    const r = await $.tool.call({ tool: 'Bash', command: 'echo hello' })
    expect(r.result).toEqual({ stdout: 'hello', stderr: '', interrupted: false })
  })

  test('擋寫回：Write 內容含遮蔽標記 → deny，且工具不會執行', async ($, on) => {
    let ran = false
    on('tool.call', () => {
      ran = true
      return { result: { type: 'update', filePath: '/x/.env', content: '' } }
    })
    const r = await $.tool.call({ tool: 'Write', file_path: '/x/.env', content: `VAULT_TOKEN=${mark('vault', 28)}\n` })
    expect(r.deny).toContain('guard:')
    expect(ran).toBe(false)
  })

  test('擋寫回：Edit / Bash 同理；不含標記則放行', async ($, on) => {
    on('tool.call', () => ({ result: { stdout: '', stderr: '', interrupted: false } }))
    const e = await $.tool.call({ tool: 'Edit', file_path: '/x', old_string: 'a', new_string: `K=${mark('github', 40)}` })
    expect(e.deny).toContain('guard:')
    const b = await $.tool.call({ tool: 'Bash', command: `echo 'K=${mark('slack', 56)}' > f` })
    expect(b.deny).toContain('guard:')
    const ok = await $.tool.call({ tool: 'Bash', command: 'echo ok' })
    expect(ok.deny).toBe(undefined)
  })
})

describe('redactContent()（session.append 用：不經 tool.call 的列）', () => {
  test('text 與 tool_result 被遮蔽，其他區塊原樣保留', () => {
    const [, v] = samples()[6]
    const thinking = { type: 'thinking', thinking: v }
    const r = redactContent([
      { type: 'text', text: `skill body: ${v}` },
      { type: 'tool_result', content: [{ type: 'text', text: v }] },
      thinking,
    ])
    expect(r.changed).toBe(true)
    expect(JSON.stringify(r.content.slice(0, 2))).not.toContain(v)
    expect(r.content[2]).toBe(thinking) // thinking 由 engine 釘住，不能也不該改
  })

  test('沒有機密時 changed=false、原物件不變', () => {
    const blocks = [{ type: 'text', text: 'hello' }]
    const r = redactContent(blocks)
    expect(r.changed).toBe(false)
    expect(r.content[0]).toBe(blocks[0])
  })
})
