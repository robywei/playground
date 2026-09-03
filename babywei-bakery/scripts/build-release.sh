#!/usr/bin/env bash
#
# 打包交付版：單一 binary 包成 .app，連同空的 data/ 與安裝說明壓成 zip。
#
# 產出結構（解壓後）：
#
#   BabyWei Bakery/
#   ├── BabyWei Bakery.app/Contents/MacOS/bakery
#   ├── data/            ← 使用者資料，升級時不要動它
#   └── 安裝說明.txt
#
# data/ 與 .app 分開，後續版本只需替換 .app。

set -euo pipefail

# 用法: ./scripts/build-release.sh [版本] [universal|arm64|amd64]

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

APP_NAME="BabyWei Bakery"
BUNDLE_ID="tech.axiomgaming.babywei-bakery"
VERSION="${1:-0.1.0}"

# ARCH=universal 同時支援 Apple Silicon 與 Intel Mac（binary 大一倍）。
# ARCH=arm64 只給 Apple Silicon。不確定對方是哪一種就用 universal ——
# 架構不符時 macOS 只會說「應用程式無法在此 Mac 上使用」，
# 收到的人無從判斷原因。
ARCH="${2:-universal}"

RELEASE_DIR="$ROOT/release"
STAGE="$RELEASE_DIR/$APP_NAME"
APP="$STAGE/$APP_NAME.app"

echo "==> 清理"
rm -rf "$RELEASE_DIR"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources" "$STAGE/data"

echo "==> 建置前端"
(cd web && npm ci --silent && npm run build)

echo "==> 建置 binary ($ARCH)"
LDFLAGS="-s -w -X main.version=$VERSION"
case "$ARCH" in
  universal)
    GOARCH=arm64 go -C server build -ldflags="$LDFLAGS" -o "$RELEASE_DIR/bakery-arm64" .
    GOARCH=amd64 go -C server build -ldflags="$LDFLAGS" -o "$RELEASE_DIR/bakery-amd64" .
    lipo -create -output "$APP/Contents/MacOS/bakery" \
      "$RELEASE_DIR/bakery-arm64" "$RELEASE_DIR/bakery-amd64"
    rm -f "$RELEASE_DIR/bakery-arm64" "$RELEASE_DIR/bakery-amd64"
    ;;
  arm64 | amd64)
    GOARCH="$ARCH" go -C server build -ldflags="$LDFLAGS" -o "$APP/Contents/MacOS/bakery" .
    ;;
  *)
    echo "✗ 未知的架構: ${ARCH}（可用 universal / arm64 / amd64）" >&2
    exit 2
    ;;
esac
echo "    架構: $(lipo -archs "$APP/Contents/MacOS/bakery")"

echo "==> 產生圖示"
# 單一來源是 web/public/icon.png —— 同一張圖同時是 .app 圖示與瀏覽器 favicon。
ICONSET="$RELEASE_DIR/AppIcon.iconset"
mkdir -p "$ICONSET"
for spec in "16 icon_16x16" "32 icon_16x16@2x" "32 icon_32x32" "64 icon_32x32@2x" \
            "128 icon_128x128" "256 icon_128x128@2x" "256 icon_256x256" \
            "512 icon_256x256@2x" "512 icon_512x512" "1024 icon_512x512@2x"; do
  px="${spec%% *}"
  name="${spec#* }"
  sips -s format png -z "$px" "$px" web/public/icon.png --out "$ICONSET/$name.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"
rm -rf "$ICONSET"

echo "==> 組裝 .app"
cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>$APP_NAME</string>
  <key>CFBundleDisplayName</key><string>$APP_NAME</string>
  <key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleExecutable</key><string>bakery</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleIconFile</key><string>AppIcon</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

cat > "$STAGE/安裝說明.txt" <<'TXT'
BabyWei Bakery — 安裝說明
========================

第一次使用（只需要做一次）
--------------------------

macOS 會擋下從網路或 AirDrop 傳來、未經 Apple 公證的程式，
畫面只會說「無法打開，因為無法驗證開發者」。

請「安裝的人」先在「終端機」執行下面這一行（把路徑換成這個資料夾的實際位置）：

    xattr -dr com.apple.quarantine "/path/to/BabyWei Bakery"

做過一次之後就不會再被擋了。


日常使用
--------

雙擊「BabyWei Bakery.app」，瀏覽器會自動打開。
重複雙擊不會開出第二個，它會直接把你帶回同一個畫面。

要關閉：在 Dock 上對圖示按右鍵 →「結束」。


資料放在哪
----------

全部在這個資料夾裡的 data/：

    data/babywei.db          你的資料
    data/backups/            每次啟動自動存的快照（保留最近 30 份）

備份有兩種做法，挑一種就好：

  1. 複製整個「BabyWei Bakery」資料夾 —— 最省事
  2. 只複製 data/ 資料夾 —— 檔案小很多，資料一樣完整

程式本身不會連網，資料不會傳到任何地方。


更新版本
--------

只替換「BabyWei Bakery.app」，data/ 資料夾原封不動。
你的資料不會受影響。
TXT

echo "==> 重新簽章"
# Go 連結器已做過 ad-hoc 簽章（Apple Silicon 執行的必要條件），但識別碼是
# 預設的 "a.out"。以 bundle id 重簽整個 .app，讓系統看到的身分與 Info.plist 一致。
codesign --force --deep --sign - --identifier "$BUNDLE_ID" "$APP"
codesign --verify --deep --strict "$APP"

echo "==> 壓縮"
cd "$RELEASE_DIR"
ZIP="$APP_NAME-$VERSION-$ARCH.zip"
zip -q -9 -r "$ZIP" "$APP_NAME"

size_mb=$(echo "scale=1; $(stat -f %z "$ZIP") / 1048576" | bc)
echo
echo "==> 完成"
echo "    $RELEASE_DIR/$ZIP (${size_mb} MB)"
echo "    SHA-256: $(shasum -a 256 "$ZIP" | cut -d' ' -f1)"
