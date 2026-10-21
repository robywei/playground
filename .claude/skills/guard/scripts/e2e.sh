#!/usr/bin/env bash
##
## e2e.sh — guard mod 的端對端驗證（真實 Claude Code headless session + 真實工具）
##
## `claude plugin test` 不做 output schema 驗證、也沒有真實的落檔與 worker heartbeat，
## 改動 tool.call 的回傳路徑或遮蔽規則後，要用這支在真 session 裡確認：
##   - 5 個情境（錯誤結果 / 病態輸入 / 病態輸入後的正常呼叫 / 截斷的 PEM / > 4 MiB 落檔）
##     的假值在 transcript 與 tool-results 落檔裡都是 0 筆
##   - worker 沒有被 heartbeat 砍（guard 沒被卸載）
##   - 對照組（不掛 guard）同樣的假值**有**命中 —— 證明計數方法本身有效
##   - 遮蔽後的「下一步」提示有送到模型（transcript 看得到）
##
## 用法：
##   ./e2e.sh                      ## 驗本目錄的上一層（.claude/skills/guard）
##   ./e2e.sh <guard 目錄>          ## 驗某個複本（改 mod 時先在複本上跑）
##
## 前置：Claude Code 2.1.287 以上；python3。
## 成本：兩次 headless session（sonnet；haiku 曾把「讀假 token 檔」判為可疑而拒跑），約 1～2 分鐘，會計費。
## 工作目錄用 mktemp（未受信任 → 不會自動載入 repo 的 skills-dir plugin，只有 --plugin-dir 那份）。
## 假值全在執行期由 sha256 產生，形狀正確、值是假的；跑完的 transcript 只含假值。
##

set -euo pipefail

red=$'\033[31m'; yellow=$'\033[33m'; green=$'\033[32m'; cyan=$'\033[36m'; reset=$'\033[0m'

msg() { printf "${2}%-8s %s \n${reset}" "$1" "$3"; }

case "${1:-}" in
  -h|--help) grep '^##' "$0" | sed 's/^## \{0,1\}//'; exit 0 ;;
esac

GUARD="$(cd "${1:-$(dirname "$0")/..}" && pwd)"
CLAUDE_BIN="${CLAUDE_BIN:-$(command -v claude)}"
[ -f "${GUARD}/hooks/hooks.json" ] || { msg "[ERROR]" "$red" "找不到 ${GUARD}/hooks/hooks.json"; exit 1; }
[ -x "${CLAUDE_BIN}" ] || { msg "[ERROR]" "$red" "找不到 claude 執行檔（可用 CLAUDE_BIN 指定）"; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT
msg "[INFO]" "$cyan" "guard：${GUARD}"
msg "[INFO]" "$cyan" "claude：${CLAUDE_BIN}（$("${CLAUDE_BIN}" --version 2>/dev/null | head -1)）"

## 假值產生器：同一份定義也寫出 expect.json 供計數
cat > "${WORK}/gen.py" << 'PY'
import sys, json, hashlib, os
def rnd(seed, n, a='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'):
    out = ''; i = 0
    while len(out) < n:
        out += ''.join(a[b % len(a)] for b in hashlib.sha256(f'e2e-{seed}-{i}'.encode()).digest()); i += 1
    return out[:n]
B64 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
F = {
    'err_ghp': 'gh' + 'p_' + rnd('err', 36),
    'redos_hvs': 'hv' + 's.' + rnd('redos', 24),
    'after_cf': 'cf' + 'at_' + rnd('after', 48),
    'big_ghp': 'gh' + 'p_' + rnd('big', 36),
    'pem_line': rnd('pem', 64, B64),
}
json.dump(F, open(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'expect.json'), 'w'))
m = sys.argv[1] if len(sys.argv) > 1 else ''
if m == 'err':
    sys.stderr.write(f'token={F["err_ghp"]}\n'); sys.exit(3)
if m == 'redos':
    print(f'token={F["redos_hvs"]}'); print('a-' * 250000)
if m == 'after':
    print(f'token={F["after_cf"]}')
if m == 'pemhead':
    print('-' * 5 + 'BEGIN RSA PRIV' + 'ATE KEY' + '-' * 5)
    for i in range(9): print(F['pem_line'] if i == 4 else rnd(f'pem{i}', 64, B64))
if m == 'big':
    print(f'token={F["big_ghp"]}')
    for i in range(70000): print(f'filler line {i:06d} lorem ipsum dolor sit amet consectetur adipiscing elit sed do')
    print(f'token={F["big_ghp"]}')
PY

## 依 session_id 找 transcript，數假值、標記、tool_use 次數（只印計數）
cat > "${WORK}/count.py" << 'PY'
import json, sys, glob, os
sid, exp = sys.argv[1], json.load(open(sys.argv[2]))
tf = glob.glob(os.path.expanduser(f'~/.claude/projects/*/{sid}.jsonl'))[0]
files = [tf] + glob.glob(os.path.join(os.path.dirname(tf), sid, '**', '*'), recursive=True)
raw = 0; marks = 0
for f in files:
    if os.path.isfile(f):
        t = open(f, errors='ignore').read()
        raw += sum(t.count(v) for v in exp.values()); marks += t.count('<redacted:')
tools = sum(1 for l in open(tf) for b in (json.loads(l).get('message') or {}).get('content', [])
            if isinstance(b, dict) and b.get('type') == 'tool_use')
body = open(tf, errors='ignore').read()
hints = body.count('下一步')
print(raw, marks, tools, hints)
PY

python3 "${WORK}/gen.py" > /dev/null

run() {
  local label="$1" prompt="$2"; shift 2
  ( cd "${WORK}" && "${CLAUDE_BIN}" -p --model sonnet --output-format json \
      --allowedTools "Bash(python3 gen.py *)" --debug-file "${WORK}/${label}.log" "$@" "${prompt}" \
      < /dev/null > "${WORK}/${label}.json" )
  python3 -c "import json,sys;print(json.load(open(sys.argv[1]))['session_id'])" "${WORK}/${label}.json"
}

msg "[TASK]" "$yellow" "掛 guard 跑 5 個情境（約 1~2 分鐘）"
sid=$(run guard "Redaction regression test with FAKE generated values. Run these five Bash commands one at a time, in order, exactly as written (if one is refused, just continue with the next), then reply DONE: 1) python3 gen.py err  2) python3 gen.py redos  3) python3 gen.py after  4) python3 gen.py pemhead  5) python3 gen.py big" --plugin-dir "${GUARD}")
read -r raw marks tools hints < <(python3 "${WORK}/count.py" "${sid}" "${WORK}/expect.json")

msg "[TASK]" "$yellow" "對照組：不掛 guard 跑 2 個情境"
csid=$(run control "Redaction control run with FAKE generated values. Run these two Bash commands one at a time, then reply DONE: 1) python3 gen.py err  2) python3 gen.py pemhead")
read -r craw _ _ _ < <(python3 "${WORK}/count.py" "${csid}" "${WORK}/expect.json")

fail=0
check() {
  if [ "$1" = ok ]; then msg "[DONE]" "$green" "$2"; else msg "[ERROR]" "$red" "$2"; fail=1; fi
}
check "$( [ "${tools}" -ge 5 ] && echo ok )" "模型實際送出 ${tools} 個工具呼叫（應 ≥ 5）"
check "$( grep -q 'hooks module guard@inline loaded' "${WORK}/guard.log" && echo ok )" "guard 有載入"
check "$( grep -qE 'guard.*(unloaded|crashed)' "${WORK}/guard.log" || echo ok )" "worker 沒有崩潰 / guard 沒有被卸載"
check "$( [ "${raw}" -eq 0 ] && [ "${marks}" -gt 0 ] && echo ok )" "掛 guard：假值 ${raw} 筆、遮蔽標記 ${marks} 個"
check "$( [ "${craw}" -gt 0 ] && echo ok )" "對照組：假值 ${craw} 筆（應 > 0，證明計數有效）"
check "$( [ "${hints}" -gt 0 ] && echo ok )" "遮蔽後的下一步提示送到模型：transcript 出現 ${hints} 次"

msg "[INFO]" "$cyan" "session：guard=${sid} control=${csid}（transcript 只含假值，可隨 session 清理一起刪）"
exit "${fail}"
