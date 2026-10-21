#!/usr/bin/env python3
"""把 MD013（行長）違規的 markdown 行折成多行，續行自帶正確前綴。

本檔由 repo-template 管理（基礎層）：要改請改範本、發版後 `copier update`，不要直接改這裡。

存在的理由：MD013 是 `.markdownlint-cli2.jsonc` 裡唯一**不可自動修**的規則
（`markdownlint-cli2 --fix` 不處理它），而手動折行有四個會靜默改壞語意的陷阱，
每次靠人記等於每次重新實作一遍。這支腳本把那四條規則和事後驗證都固化下來。

用法（預設 dry-run，只列出會怎麼折）：

    ./scripts/lint/md-wrap.py                # 預覽
    ./scripts/lint/md-wrap.py --apply        # 實際寫檔
    ./scripts/lint/md-wrap.py --limit 100    # 臨時換上限（預設讀設定檔）
    ./scripts/lint/md-wrap.py --apply docs/draft.md    # 只處理指定檔（被 gitignore 的草稿要這樣給）

檔案清單與 `lint.sh` 同源（`git ls-files`），違規清單直接問 `markdownlint-cli2`
—— 不自己算行長，否則 `code_blocks: false` / `tables: false` / `strict` 這些
豁免條件會與閘門判準漂移。

四條斷點規則（前三條錯了會改壞語意，且 markdownlint 一律不報）：

  1. code span（`…`）與連結 `[…](…)` 的**內部**不可斷 —— 前者斷行會多一個空格
     （內容當場變了）、後者會直接壞掉。但切在它們的**邊界**是安全的（整個元素
     移到下一行），把邊界也算成禁區會讓「span 之間有標點」的行完全找不到斷點。
  2. 粗體 / 斜體**可以**斷 —— `**text**` 的強調跨越 soft line break 仍有效。
     把它列為禁區會讓「整句被粗體包住」的長行無解。
  3. 續行開頭不可是 `-` `+` `*` `#` `>` `|` 或數字加點 —— 會被解析成清單項、
     標題、blockquote 或表格列。實測踩過：一行「`FROM alpine:3` + 相同 apk 套件
     + 解開的 CLI 安裝區塊」斷在 `+` 前，續行變成清單項，同時觸發
     MD004 與 MD032。
  4. 續行前綴要跟著原行的結構：清單項對齊內容縮排、blockquote 補 `>`。

套用前後一律跑內建的字元級等價驗證（扣掉換行、縮排、blockquote 前綴後，
實質字元必須完全一致），不通過就中止、不寫檔。
"""

import argparse
import json
import pathlib
import re
import subprocess
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
CONFIG = REPO_ROOT / ".markdownlint-cli2.jsonc"
CJK_PUNCT = "。，、；：？！）」』】》"
CJK_RANGE = r'[一-鿿]'


def mdlen(s):
    """markdownlint 算的長度 —— 它是 JS 寫的，用 UTF-16 code unit 而非字元數。

    差別只在 BMP 外的字元（emoji、部分罕用字）每個多算 1。用 python 的 `len()`
    會低估含 emoji 的行：實測某行 python 算 120（等於上限、
    看起來合規），markdownlint 報 `Actual: 121`。
    """
    return len(s.encode("utf-16-le")) // 2


def max_index_within(s, budget):
    """回傳最大的字元索引 i，使 mdlen(s[:i]) <= budget。"""
    total = 0
    for i, ch in enumerate(s):
        w = 2 if ord(ch) > 0xFFFF else 1
        if total + w > budget:
            return i
        total += w
    return len(s)


def config_limit():
    """從設定檔讀 MD013 的 line_length，讀不到就用 markdownlint 的預設 80。"""
    try:
        text = CONFIG.read_text(encoding="utf-8")
    except OSError:
        return 80
    # 只需要 line_length 這個值，不必完整解析 jsonc（它有行內註解）
    m = re.search(r'"MD013"\s*:\s*\{[^}]*"line_length"\s*:\s*(\d+)', text)
    return int(m.group(1)) if m else 80


def md013_disabled():
    """設定檔明寫 `"MD013": false` 時回 True —— 此時 markdownlint 不會回報任何行長違規，
    印「沒有違規（上限 80）」會誤導成「有在檢查、而且都合規」。"""
    try:
        text = CONFIG.read_text(encoding="utf-8")
    except OSError:
        return False
    return re.search(r'"MD013"\s*:\s*false', text) is not None


def md_files():
    """`lint.sh` 的 `list_files` 同款取法 —— 兩個旗標都是必要的，缺了會靜默漏檔。

    🔴 `-c core.quotepath=false`：預設 `git ls-files` 把非 ASCII 檔名輸出成**帶引號的
    octal 轉義**（`"docs/…\347\231\275…md"`）。那個字串當參數傳給 markdownlint 的結果是
    `Linting: 0 files` ＋ `Summary: 0 issues in 0 files` ＋ **exit 0** —— 假綠燈，
    症狀看起來像「這個檔沒有違規」。實測：一個 repo 的 2 個中文檔名 `.md`
    （合計 108 筆違規）因此全部被跳過。
    🔴 `-z`：檔名以 NUL 分隔，才不會被 `split()` 依空白切開（原本的寫法遇到含空格的
    檔名會把一個檔切成兩個不存在的路徑）。
    """
    out = subprocess.run(
        ["git", "-c", "core.quotepath=false", "ls-files", "-z",
         "--cached", "--others", "--exclude-standard", "--", "*.md"],
        cwd=REPO_ROOT, capture_output=True, text=True, check=True,
    )
    return [f for f in out.stdout.split("\0") if f]


def violations(files=None):
    """問 markdownlint 要 MD013 的 file:line，判準與閘門完全一致。"""
    files = files if files else md_files()
    if not files:
        return {}
    proc = subprocess.run(
        ["markdownlint-cli2", *files],
        cwd=REPO_ROOT, capture_output=True, text=True,
    )
    combined = proc.stdout + proc.stderr
    m = re.search(r'^Linting: (\d+) files?$', combined, re.M)
    if not m:
        sys.exit("✗ markdownlint-cli2 沒有輸出 'Linting: N files' —— 無法確認它真的跑過")
    ## 「它有輸出」不等於「它掃了我給的每一個檔」。少掃的那幾個會表現成「沒有違規」，
    ## 與真的乾淨無法區分（成因見 md_files 的兩個旗標）。所以對帳筆數，不只看有沒有跑。
    if int(m.group(1)) != len(files):
        sys.exit(f"✗ 給了 {len(files)} 個檔，markdownlint 只掃了 {m.group(1)} 個 "
                 "—— 有檔名沒被它接受（先查非 ASCII / 含空格的檔名）")
    hits = {}
    for m in re.finditer(r'^(.+?\.md):(\d+)(?::\d+)? error MD013/', combined, re.M):
        hits.setdefault(m.group(1), set()).add(int(m.group(2)))
    return hits


def prefix_for(line):
    """回傳 (內容起點, 續行前綴)。"""
    for pattern in (r'^(\s*)([-*+]) ', r'^(\s*)(\d+)[.)] '):
        m = re.match(pattern, line)
        if m:
            return len(m.group(0)), " " * len(m.group(0))
    m = re.match(r'^((?:\s*>)+\s*)', line)
    if m and m.group(1).strip():
        return len(m.group(1)), m.group(1)
    m = re.match(r'^(\s+)', line)
    if m:
        return len(m.group(1)), m.group(1)
    return 0, ""


def inner_of_atomic(s):
    """標記「不可切進去」的位置 —— 只含元素內部，不含邊界（見檔頭規則 1）。"""
    bad = [False] * (len(s) + 1)
    for pattern in (r'`+[^`]*`+', r'!?\[[^\]]*\]\([^)]*\)'):
        for m in re.finditer(pattern, s):
            for i in range(m.start() + 1, m.end()):
                bad[i] = True
    return bad


def bad_continuation(s, i):
    """切在 i 之後，續行開頭會不會被當成 markdown 標記（見檔頭規則 3）。"""
    return bool(re.match(r'([-+*#>|]|\d+[.)])(\s|$)', s[i:].lstrip()))


def find_break(s, limit):
    bad = inner_of_atomic(s)

    def ok(i):
        return not bad[i] and not bad_continuation(s, i)

    upper = min(limit, len(s)) - 1
    for i in range(upper, 0, -1):                       # 1) 中文標點之後
        if s[i - 1] in CJK_PUNCT and ok(i):
            return i
    for i in range(upper, 0, -1):                       # 2) 空格處
        if s[i] == " " and ok(i):
            return i
    for i in range(upper, 0, -1):                       # 3) 中文字之間
        if ok(i) and re.match(CJK_RANGE, s[i]) and re.match(CJK_RANGE, s[i - 1]):
            return i
    return -1


def wrap_line(line, limit):
    if mdlen(line) <= limit:
        return [line]
    start, cont = prefix_for(line)
    head, body, out = line[:start], line[start:], []
    cur = head
    while True:
        avail = limit - mdlen(cur)
        if mdlen(body) <= avail:
            out.append(cur + body)
            break
        cut = find_break(body, max_index_within(body, avail) + 1)
        if cut <= 0:
            out.append(cur + body)      # 找不到安全斷點
            break
        seg, body = body[:cut].rstrip(), body[cut:].lstrip()
        out.append(cur + seg)
        cur = cont
        if not body:
            break
    # 折完仍有片段超標就整行放棄 —— 「折了但沒折乾淨」比不折更糟：
    # 排版被改動、違規還在，而且下一輪又會被挑出來重折一次。
    if any(mdlen(p) > limit for p in out):
        return [line]
    return out


def essence(text):
    """扣掉換行、縮排與 blockquote 前綴後的實質字元。"""
    stripped = (re.sub(r'^\s*>+\s*', '', ln) for ln in text.split("\n"))
    return re.sub(r'\s+', '', "".join(stripped))


def main():
    ap = argparse.ArgumentParser(description="折 MD013 過長行（預設 dry-run）")
    ap.add_argument("--apply", action="store_true", help="實際寫檔（預設只預覽）")
    ap.add_argument("--dry-run", action="store_true", help="明示預覽（預設行為）")
    ap.add_argument("--limit", type=int, default=None, help="行長上限（預設讀設定檔）")
    ## 預設檔案清單同 lint.sh（git ls-files，含未追蹤但未 ignore 的）—— 被 gitignore 的草稿不在裡面，
    ## 要折它們就明確給路徑
    ap.add_argument("paths", nargs="*", help="只處理這些 .md（預設：與 lint.sh 相同的清單）")
    args = ap.parse_args()

    ## 違規清單是問 markdownlint 拿的：規則停用時它永遠回 0 筆，帶 --limit 也偵測不到（--limit 只決定折行寬度）
    if md013_disabled():
        print(f"✓ MD013 已在 {CONFIG.name} 停用 —— 沒有行長上限，不需要折行")
        return 0

    limit = args.limit or config_limit()
    hits = violations([str(pathlib.Path(p).resolve()) for p in args.paths])
    if not hits:
        print(f"✓ 沒有 MD013 違規（上限 {limit}）")
        return 0

    wrapped = unresolved = 0
    for rel in sorted(hits):
        path = REPO_ROOT / rel
        original = path.read_text(encoding="utf-8")
        lines, out = original.split("\n"), []
        for idx, line in enumerate(lines, 1):
            if idx in hits[rel] and mdlen(line) > limit:
                pieces = wrap_line(line, limit)
                if len(pieces) > 1:
                    wrapped += 1
                    print(f"  {rel}:{idx}  {mdlen(line)} → {[mdlen(p) for p in pieces]}")
                else:
                    unresolved += 1
                    print(f"  ⚠️ {rel}:{idx}  {mdlen(line)} 找不到安全斷點，需人工處理")
                out.extend(pieces)
            else:
                out.append(line)
        new = "\n".join(out)

        if essence(new) != essence(original):
            sys.exit(f"✗ {rel}：折行後實質字元改變了，已中止且未寫檔（這是 bug，請回報）")
        if args.apply and new != original:
            path.write_text(new, encoding="utf-8")

    mode = "已套用" if args.apply else "dry-run（加 --apply 才寫檔）"
    print(f"\n折行 {wrapped} 行 / {len(hits)} 檔；需人工處理 {unresolved} 行  [{mode}]")
    if args.apply:
        print("⚠️ 跑完務必 `./scripts/lint/lint.sh md` 確認 0 筆，並 `git diff` 抽看幾處")
    return 0


if __name__ == "__main__":
    sys.exit(main())
