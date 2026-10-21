#!/usr/bin/env bash
# install-hooks.sh：啟用本 repo 的 git hook（每個 clone 執行一次，重複執行無害）
#
# 由 repo-template 管理（基礎層）。做三件事：
#   1. core.hooksPath = scripts/githooks（之後範本新增的 hook 自動生效，不必重跑）
#   2. 移除 .git/hooks/ 底下指向 scripts/githooks/ 的舊 symlink（舊的安裝方式；設了 hooksPath 後它們不再生效）
#   3. 列出檢查需要的工具（scripts/lint/lint.sh --doctor）—— clone 當下就知道缺什麼，不必等到 push 被擋
# 退出碼：0 已啟用且工具齊全／1 已啟用但缺工具，或 hooksPath 已被設成別的值（不覆蓋）／
#         其他：lint.sh --doctor 失敗（多半是 lint.sh 還沒遷移成 source lib.sh，見 lib.sh 檔頭）

ROOT="$(git rev-parse --show-toplevel)" || exit 1
cd "${ROOT}" || exit 1

cur="$(git config --local --get core.hooksPath)"
if [ -n "${cur}" ] && [ "${cur}" != scripts/githooks ]; then
  echo "core.hooksPath 已設為 ${cur}（不是 scripts/githooks），不覆蓋。確定要改：git config core.hooksPath scripts/githooks" >&2
  exit 1
fi
git config --local core.hooksPath scripts/githooks || exit 1
echo "[ OK ]  core.hooksPath = scripts/githooks"

# 用 common dir：worktree 的 hook 放在主 repo 的 .git/hooks；--git-path hooks 在設了 hooksPath 後會指回 scripts/githooks
hooks_dir="$(git rev-parse --git-common-dir)/hooks"
for f in "${hooks_dir}"/*; do
  [ -L "${f}" ] || continue
  case "$(readlink "${f}")" in
    *scripts/githooks/*) rm -f "${f}" && echo "[ OK ]  移除舊 symlink：${f}" ;;
  esac
done

echo
echo "檢查需要的工具："
"${ROOT}/scripts/lint/lint.sh" --doctor
