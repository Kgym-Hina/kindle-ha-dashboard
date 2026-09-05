#!/bin/sh

BASE=/mnt/us/extensions/kindle-ha-dashboard
PIDFILE="$BASE/dashboard.pid"
LOG="$BASE/logs/dashboard.log"
BIN="$BASE/bin/kindle-dashboard"
CONFIG="$BASE/config"
PATH="/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/sbin:/usr/local/bin"

export PATH
mkdir -p "$BASE/logs"

if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "Kindle HA Dashboard already running"
  exit 0
fi
if [ ! -x "$BIN" ]; then
  echo "Missing executable: $BIN"
  exit 1
fi
if [ ! -f "$CONFIG" ]; then
  echo "Missing config: $CONFIG"
  exit 1
fi

cd "$BASE" || exit 1
echo "[$(date)] starting Kindle HA Dashboard" >> "$LOG"
KINDLE_DASHBOARD_CONFIG="$CONFIG" \
  nohup "$BIN" >> "$LOG" 2>&1 </dev/null &
echo $! > "$PIDFILE"
echo "Kindle HA Dashboard started"

