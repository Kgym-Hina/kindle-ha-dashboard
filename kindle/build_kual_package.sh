#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
PLUGIN_NAME="kindle-ha-dashboard"
DIST_DIR="$ROOT/dist"
PKG_ROOT="$DIST_DIR/$PLUGIN_NAME"
BIN_DIR="$PKG_ROOT/bin"
FONT_DIR="$PKG_ROOT/fonts"
ZIP_PATH="$DIST_DIR/${PLUGIN_NAME}.zip"

rm -rf "$PKG_ROOT"
mkdir -p "$BIN_DIR" "$FONT_DIR"

echo "Building Kindle ARMv7 executable..."
(
  cd "$ROOT"
  GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN_DIR/kindle-dashboard" ./cmd/kindle-dashboard
)

cp "$ROOT/extensions/config.xml" "$PKG_ROOT/config.xml"
cp "$ROOT/extensions/menu.json" "$PKG_ROOT/menu.json"
cp "$ROOT/extensions/bin/start.sh" "$BIN_DIR/start.sh"
cp "$ROOT/extensions/bin/stop.sh" "$BIN_DIR/stop.sh"
cp "$ROOT/config.example" "$PKG_ROOT/config"
cp "$ROOT/config.example" "$PKG_ROOT/config.example"
cp "$ROOT/fonts/NotoSansCJKsc-Regular.otf" "$FONT_DIR/NotoSansCJKsc-Regular.otf"
cp "$ROOT/fonts/NotoSansCJKsc-Bold.otf" "$FONT_DIR/NotoSansCJKsc-Bold.otf"
cp "$ROOT/fonts/OFL.txt" "$FONT_DIR/OFL.txt"

chmod 755 "$BIN_DIR/kindle-dashboard" "$BIN_DIR/start.sh" "$BIN_DIR/stop.sh"
echo "Creating KUAL package..."
(
  cd "$DIST_DIR"
  zip -r "$ZIP_PATH" "$PLUGIN_NAME"
)
echo "Package ready: $ZIP_PATH"
