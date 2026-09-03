#!/usr/bin/env bash
#
# 建置單一執行檔：前端 vite build → 產物由 go:embed 編進 binary。

set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "==> 建置前端"
(cd web && npm ci --silent && npm run build)

echo "==> 建置 binary"
go build -ldflags="-s -w" -o bakery .

size_mb=$(echo "scale=1; $(stat -f %z bakery) / 1048576" | bc)
echo "==> 完成: $ROOT/bakery (${size_mb} MB)"
