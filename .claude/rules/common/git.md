# Git 操作

> 由 repo-template 管理（基礎層），要改請改範本；本 repo 專屬規則寫在 `.claude/custom/` 或 repo 自己的 CLAUDE.md

commit 與 push 的完整流程見 `/git:commit`；以下是沒走那支 skill 時也要守的。

- 第一筆 commit 前確認身分：`git config --show-origin user.email`
- `git add <路徑…>` 只要有一個路徑不存在（含已 `git rm` 的）就 `fatal`、**整批一個都沒加** → stderr 不要遮，失敗時逐一查路徑
- 工作區可能有別的 agent 或人的變更 → 只 add 自己改的路徑；不用 `git add -A`、`commit -a`、`stash`、`clean`、
  `checkout .`、`reset --hard` 這類動到整個工作區的指令
- 已 push 的歷史不改寫（`--amend`、`rebase`、`push --force`）
- 檢查缺工具或跑不起來 → 停下來列出缺什麼與安裝方式；**使用者明確同意**才用 `SKIP_CHECKS=<項目>`，回報寫明哪幾項沒檢查。
  不自己加 `--no-verify`；有發現（不是跑不起來）不能跳過，要修或逐筆寫進例外清單
