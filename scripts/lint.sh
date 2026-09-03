#!/usr/bin/env bash
#
# 統一的檢查入口。存在的理由是兩個實測過的靜默失敗：
#
#   1. 從子目錄呼叫 markdownlint / markdownlint-cli2 時，設定檔按 cwd 往上找，
#      找不到就靜默改用預設值 —— 症狀是 MD013 等已停用的規則突然一片紅。
#   2. 兩支工具拿不到檔案時都 exit 0：cli v1 印 usage、cli2 印 `Linting: 0 files`。
#      於是 `markdownlint <檔> && echo OK` 會印出 OK。
#
# 本腳本一律 cd 到 repo 根再跑，並檢查「真的有檔案被檢查」。

set -euo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

usage() {
  cat <<'USAGE'
用法: ./scripts/lint.sh [md|all]

  md    檢查所有 markdown（預設）
  all   同 md（目前只有 markdown 檢查項）
USAGE
}

lint_md() {
  command -v markdownlint-cli2 >/dev/null 2>&1 || {
    echo "✗ 找不到 markdownlint-cli2" >&2
    echo "  安裝: npm i -g markdownlint-cli2" >&2
    return 127
  }

  local out status
  # 不帶參數 —— 檢查範圍由 .markdownlint-cli2.jsonc 的 globs 決定
  set +e
  out="$(markdownlint-cli2 2>&1)"
  status=$?
  set -e
  printf '%s\n' "$out"

  # 假綠燈防線：exit 0 但一個檔案都沒檢查到
  local n
  n="$(printf '%s\n' "$out" | sed -n 's/^Linting: \([0-9]*\) files\?$/\1/p' | tail -1)"
  if [[ -z "$n" ]]; then
    echo "✗ 輸出中找不到 'Linting: N files' —— 無法確認檢查真的執行過" >&2
    return 1
  fi
  if [[ "$n" -eq 0 ]]; then
    echo "✗ Linting: 0 files —— glob 沒命中任何檔案，這不是通過" >&2
    return 1
  fi

  if [[ $status -eq 0 ]]; then
    echo "✓ markdown 檢查通過（$n 個檔案）"
  fi
  return $status
}

case "${1:-md}" in
  md | all) lint_md ;;
  -h | --help | help) usage ;;
  *)
    echo "✗ 未知的檢查項: $1" >&2
    usage >&2
    exit 2
    ;;
esac
