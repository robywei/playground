#!/usr/bin/env python3
##
## replace.py — 多檔、多錨點的「全部對了才寫」字串替換（取代臨時 `sed -i` 與手寫 python 改檔）
##
## 🔁 由 repo-template 管理（基礎層）：要改請改範本、發版後 `copier update`，不要直接改這裡。
##    規則面（什麼時候該用它）見 `.claude/rules/common/agent-env.md`。
##
## 把臨時改檔時「每次照抄重新實作」的紀律做成一支工具：
##   ① 每個錨點斷言出現次數（預設恰好 1 處；`replace(old, new, 1)` 不是保護，它會改錯的那一處）
##   ② 所有替換先做在記憶體，**任何一條不對就一個檔都不寫**（跨檔也是 —— 拆成兩個呼叫才會留半套）
##   ③ `keep`：替換後仍必須存在的錨點（擋「用兩個 index 切區塊時多切」）
##   ④ 錨點對不上時，印出檔案裡最接近的幾行的 repr()（中文 markdown 手動折行，記憶中的一句話常斷成兩行）
##
## 用法：
##   python3 scripts/tools/replace.py <plan.json>             ## 套用
##   python3 scripts/tools/replace.py --dry-run <plan.json>   ## 只印 unified diff，不寫檔
##
## plan.json（用 Write 工具寫，**不要用 heredoc 餵** —— 內容裡的標記會提早關閉 heredoc）：
##   {
##     "edits": [
##       {"file": "docs/x.md", "old": "舊字串", "new": "新字串"},
##       {"file": "docs/y.md", "old": "出現三次的字", "new": "新字", "count": 3}
##     ],
##     "keep": [ {"file": "docs/x.md", "text": "替換後仍必須存在的字串"} ]
##   }
##   - 同一個檔的多條 edit 依序套在「前一條改完的內容」上
##   - 路徑相對於執行時的目錄 → 每次呼叫自帶 `cd <repo 根的絕對路徑> &&`，或在 plan 裡寫絕對路徑
##   - plan 檔放 scratchpad 等 repo 外的位置，不要留在工作區
##
## 退出碼：0 全部套用（或 dry-run 通過）/ 1 有錨點不對，**什麼都沒寫** / 2 用法或 plan 格式錯
##

import difflib
import json
import os
import shutil
import sys
import tempfile

RED, GREEN, YELLOW, CYAN, RESET = "\033[31m", "\033[32m", "\033[33m", "\033[36m", "\033[0m"


def msg(tag: str, color: str, text: str) -> None:
    print(f"{color}{tag:<8} {text}{RESET}")


def near(content: str, old: str, limit: int = 3) -> list[str]:
    """錨點找不到時：用錨點第一行的前綴（20 → 12 → 6 字）找候選行，印 repr 讓人看出折行 / 縮排差在哪"""
    first = old.strip().split("\n")[0]
    lines = content.split("\n")
    for size in (20, 12, 6):
        head = first[:size]
        if not head:
            return []
        out = [f"{n}: {line!r}" for n, line in enumerate(lines, 1) if head in line][:limit]
        if out:
            return out
    return []


def load_plan(path: str) -> dict:
    try:
        with open(path, encoding="utf-8") as f:
            plan = json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        msg("[ERROR]", RED, f"讀不到 plan：{e}")
        sys.exit(2)
    edits = plan.get("edits")
    if not isinstance(edits, list) or not edits:
        msg("[ERROR]", RED, "plan 必須有非空的 edits 陣列")
        sys.exit(2)
    for i, e in enumerate(edits, 1):
        if not all(isinstance(e.get(k), str) for k in ("file", "old", "new")) or e["old"] == "":
            msg("[ERROR]", RED, f"edits[{i}] 要有字串 file / old（非空）/ new")
            sys.exit(2)
        if not isinstance(e.get("count", 1), int) or e.get("count", 1) < 1:
            msg("[ERROR]", RED, f"edits[{i}].count 必須是 ≥ 1 的整數")
            sys.exit(2)
    for i, k in enumerate(plan.get("keep", []), 1):
        if not (isinstance(k.get("file"), str) and isinstance(k.get("text"), str) and k["text"]):
            msg("[ERROR]", RED, f"keep[{i}] 要有字串 file / text（非空）")
            sys.exit(2)
    return plan


def main() -> int:
    args = sys.argv[1:]
    if not args or args[0] in ("-h", "--help"):
        with open(__file__, encoding="utf-8") as f:
            for line in f.read().split("\n")[1:]:
                if not line.startswith("##"):
                    break
                print(line[3:] if line.startswith("## ") else line[2:])
        return 0 if args else 2
    dry = "--dry-run" in args
    rest = [a for a in args if a != "--dry-run"]
    if len(rest) != 1:
        msg("[ERROR]", RED, "用法：replace.py [--dry-run] <plan.json>")
        return 2
    plan = load_plan(rest[0])

    original: dict[str, str] = {}
    current: dict[str, str] = {}
    failures = 0
    for i, e in enumerate(plan["edits"], 1):
        f = e["file"]
        if f not in current:
            try:
                with open(f, encoding="utf-8", newline="") as fh:
                    original[f] = current[f] = fh.read()
            except OSError as err:
                msg("[ERROR]", RED, f"edits[{i}] {f}：{err}")
                failures += 1
                continue
        want = e.get("count", 1)
        got = current[f].count(e["old"])
        if got != want:
            failures += 1
            msg("[ERROR]", RED, f"edits[{i}] {f}：錨點出現 {got} 次，預期 {want} 次")
            for line in near(current[f], e["old"]):
                print(f"           {line}")
            continue
        current[f] = current[f].replace(e["old"], e["new"])
        msg("[DONE]", GREEN, f"edits[{i}] {f}：{got} 處")

    for i, k in enumerate(plan.get("keep", []), 1):
        f = k["file"]
        body = current.get(f)
        if body is None:
            try:
                with open(f, encoding="utf-8", newline="") as fh:
                    body = fh.read()
            except OSError as err:
                msg("[ERROR]", RED, f"keep[{i}] {f}：{err}")
                failures += 1
                continue
        if k["text"] not in body:
            failures += 1
            msg("[ERROR]", RED, f"keep[{i}] {f}：替換後找不到應保留的錨點（多切了？）")

    if failures > 0:
        msg("[ERROR]", RED, f"{failures} 個問題 —— **沒有寫入任何檔案**")
        return 1

    changed = [f for f in current if current[f] != original[f]]
    if dry:
        for f in changed:
            sys.stdout.writelines(
                difflib.unified_diff(
                    original[f].splitlines(keepends=True), current[f].splitlines(keepends=True), f"a/{f}", f"b/{f}"
                )
            )
        msg("[INFO]", CYAN, f"--dry-run：{len(changed)} 個檔會被改，沒有寫入")
        return 0

    # 全部驗過才寫：同目錄暫存檔 → rename（不會留下寫一半的檔），保留原本的權限
    for f in changed:
        d = os.path.dirname(os.path.abspath(f))
        fd, tmp = tempfile.mkstemp(dir=d, prefix=".replace-", suffix=".tmp")
        try:
            with os.fdopen(fd, "w", encoding="utf-8", newline="") as fh:
                fh.write(current[f])
            shutil.copymode(f, tmp)
            os.replace(tmp, f)
        except BaseException:
            if os.path.exists(tmp):
                os.unlink(tmp)
            raise
    msg("[DONE]", GREEN, f"寫入 {len(changed)} 個檔")
    return 0


if __name__ == "__main__":
    sys.exit(main())
