#!/usr/bin/env bash
##
## lint.sh — 本 repo 的檢查入口（pre-push 跑不帶參數的完整檢查）
##
## 由 repo-template 產生一次，之後歸本 repo 維護，範本 update 不會覆蓋這個檔案。
## 基礎檢查（md、shell、trivy、conflict）與共同介面在同目錄的 lib.sh（範本擁有，不要改）；用法見 lib.sh 檔頭。
## 本 repo 專屬的檢查：定義 check_<名>()，並把名字加進 REPO_CHECKS。
##

# shellcheck source-path=SCRIPTDIR source=lib.sh
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/lib.sh" || exit 1

REPO_CHECKS=""

lint_main "$@"
