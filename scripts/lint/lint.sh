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

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

usage() {
  cat <<'USAGE'
用法: ./scripts/lint/lint.sh [all|md|shell|trivy]

  all    全部檢查（預設；pre-push 跑這個）
  md     檢查所有 markdown
  shell  shellcheck（error 級）：進版控的 *.sh 與 githooks
  trivy  vuln / misconfig / secret（範本的 scripts/trivy/trivy-cli.sh repo）
USAGE
}

lint_md() {
  command -v markdownlint-cli2 >/dev/null 2>&1 || {
    echo "✗ 找不到 markdownlint-cli2" >&2
    echo "  安裝: npm i -g markdownlint-cli2" >&2
    return 127
  }

  local out status
  # 檢查範圍由本腳本給：進版控（含未追蹤但未 ignore）的 .md。設定檔刻意不設 globs，
  # 才不會掃到 temp/ 等被 gitignore 的草稿。xargs 可能拆成多次呼叫 → 下面把每段的 Linting 數加總
  set +e
  out="$(git -c core.quotepath=false ls-files -z --cached --others --exclude-standard -- '*.md' \
    | xargs -0 markdownlint-cli2 2>&1)"
  status=$?
  set -e
  printf '%s\n' "$out"

  # 假綠燈防線：exit 0 但一個檔案都沒檢查到
  local n
  n="$(printf '%s\n' "$out" | sed -n 's/^Linting: \([0-9]*\) files\?$/\1/p' | awk '{s+=$1} END{if (NR) print s}')"
  if [[ -z "$n" ]]; then
    echo "✗ 輸出中找不到 'Linting: N files' —— 無法確認檢查真的執行過" >&2
    return 1
  fi
  if [[ "$n" -eq 0 ]]; then
    echo "✗ Linting: 0 files —— 沒有任何 .md 被檢查，這不是通過" >&2
    return 1
  fi

  if [[ $status -eq 0 ]]; then
    echo "✓ markdown 檢查通過（$n 個檔案）"
  fi
  return $status
}

lint_shell() {
  command -v shellcheck >/dev/null 2>&1 || {
    echo "✗ 找不到 shellcheck" >&2
    echo "  安裝: brew install shellcheck" >&2
    return 127
  }

  local -a files=()
  local f
  while IFS= read -r -d '' f; do files+=("$f"); done < <(
    git -c core.quotepath=false ls-files -z --cached --others --exclude-standard -- '*.sh' 'scripts/githooks/*'
  )
  # 假綠燈防線：一支腳本都沒找到不是通過
  if [[ ${#files[@]} -eq 0 ]]; then
    echo "✗ 沒有找到任何 shell 腳本 —— 這不是通過" >&2
    return 1
  fi
  shellcheck -S error "${files[@]}" || return 1
  echo "✓ shellcheck（error 級）通過（${#files[@]} 個腳本）"
}

lint_trivy() {
  # QUIET=true：有發現或 trivy 執行失敗都非 0（找不到 trivy 也擋，不是通過）
  QUIET=true "$REPO_ROOT/scripts/trivy/trivy-cli.sh" repo "$REPO_ROOT"
}

case "${1:-all}" in
  md) lint_md ;;
  shell) lint_shell ;;
  trivy) lint_trivy ;;
  all)
    rc=0
    lint_md || rc=1
    lint_shell || rc=1
    lint_trivy || rc=1
    exit "$rc"
    ;;
  -h | --help | help) usage ;;
  *)
    echo "✗ 未知的檢查項: $1" >&2
    usage >&2
    exit 2
    ;;
esac
