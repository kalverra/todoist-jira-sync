#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="todoist-jira-sync"
INSTALL_DIR="$HOME/bin"
BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
PLIST_NAME="com.kalverra.$BINARY_NAME"
PLIST_PATH="$HOME/Library/LaunchAgents/$PLIST_NAME.plist"
LOG_DIR="$HOME/Library/Logs"

echo "==> Building $BINARY_NAME..."
mkdir -p "$INSTALL_DIR"
go build -o "$BINARY_PATH" "$PROJECT_DIR"
echo "    Installed to $BINARY_PATH"

echo "==> Unloading existing LaunchAgent (if any)..."
launchctl unload "$PLIST_PATH" 2>/dev/null || true

echo "==> Writing LaunchAgent plist to $PLIST_PATH..."
cat > "$PLIST_PATH" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$PLIST_NAME</string>

  <key>ProgramArguments</key>
  <array>
    <string>$BINARY_PATH</string>
    <string>watch</string>
  </array>

  <key>WorkingDirectory</key>
  <string>$PROJECT_DIR</string>

  <key>RunAtLoad</key>
  <true/>

  <key>KeepAlive</key>
  <true/>

  <key>StandardOutPath</key>
  <string>$LOG_DIR/$BINARY_NAME.stdout.log</string>

  <key>StandardErrorPath</key>
  <string>$LOG_DIR/$BINARY_NAME.stderr.log</string>
</dict>
</plist>
PLIST

echo "==> Loading LaunchAgent..."
launchctl load "$PLIST_PATH"

sleep 2
if launchctl list | grep -q "$PLIST_NAME"; then
    echo "==> $BINARY_NAME is running."
else
    echo "==> WARNING: $BINARY_NAME does not appear to be running."
fi

echo ""
echo "Logs:"
echo "  Human-readable : tail -f $LOG_DIR/$BINARY_NAME.stderr.log"
echo "  Structured JSON: tail -f $LOG_DIR/$BINARY_NAME.log.jsonl"
echo ""
echo "Management:"
echo "  Stop   : launchctl unload $PLIST_PATH"
echo "  Start  : launchctl load $PLIST_PATH"
echo "  Status : launchctl list | grep $BINARY_NAME"
echo "  Rebuild: $0"
