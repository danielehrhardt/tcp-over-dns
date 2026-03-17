#!/bin/bash
#
# Build the macOS .app bundle with the tcpdns binary embedded.
# Output: dist/tcpdns.app
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DIST_DIR="$PROJECT_DIR/dist"
APP_DIR="$DIST_DIR/tcpdns.app"

echo "Building tcpdns macOS app..."

# Build the binary
echo "  Compiling tcpdns..."
cd "$PROJECT_DIR"
CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o "$PROJECT_DIR/bin/tcpdns" ./cmd/tcpdns

# Create the .app structure
echo "  Creating app bundle..."
rm -rf "$APP_DIR"
mkdir -p "$APP_DIR/Contents/MacOS"
mkdir -p "$APP_DIR/Contents/Resources"

# Copy the launcher
cp "$PROJECT_DIR/desktop/macos/tcpdns.app/Contents/MacOS/launch" "$APP_DIR/Contents/MacOS/launch"
chmod +x "$APP_DIR/Contents/MacOS/launch"

# Copy Info.plist
cp "$PROJECT_DIR/desktop/macos/tcpdns.app/Contents/Info.plist" "$APP_DIR/Contents/Info.plist"

# Embed the binary
cp "$PROJECT_DIR/bin/tcpdns" "$APP_DIR/Contents/Resources/tcpdns"
chmod +x "$APP_DIR/Contents/Resources/tcpdns"

echo ""
echo "Done! App bundle created at:"
echo "  $APP_DIR"
echo ""
echo "To install, drag tcpdns.app to /Applications/"
echo "Or run: cp -r $APP_DIR /Applications/"
