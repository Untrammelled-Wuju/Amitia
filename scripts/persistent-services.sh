#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT_DIR="$HOME/Library/LaunchAgents"
LOG_DIR="$ROOT/AmitiaData/logs"
DESKTOP_LABEL="com.amitia.desktop.dev"
EMULATOR_LABEL="com.amitia.android.emulator"
APP_LABEL="com.amitia.android.app"
DOMAIN="gui/$(id -u)"

mkdir -p "$AGENT_DIR" "$LOG_DIR"

desktop_plist="$AGENT_DIR/$DESKTOP_LABEL.plist"
emulator_plist="$AGENT_DIR/$EMULATOR_LABEL.plist"
app_plist="$AGENT_DIR/$APP_LABEL.plist"

write_agent() {
  local path="$1"
  local label="$2"
  local program="$3"
  local working_directory="$4"
  local process_type="$5"
  local log_path="$6"
  local plist_tmp="${path}.tmp"

  cat > "$plist_tmp" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$label</string>
  <key>ProgramArguments</key>
  <array>
    <string>$program</string>
  </array>
  <key>WorkingDirectory</key>
  <string>$working_directory</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>ThrottleInterval</key>
  <integer>10</integer>
  <key>ProcessType</key>
  <string>$process_type</string>
  <key>LimitLoadToSessionType</key>
  <string>Aqua</string>
  <key>StandardOutPath</key>
  <string>$log_path</string>
  <key>StandardErrorPath</key>
  <string>$log_path</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>$ROOT/AmitiaData/node/bin:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    <key>AMITIA_NODE_BIN</key>
    <string>$ROOT/AmitiaData/node/bin/node</string>
    <key>AMITIA_RUN_MODE</key>
    <string>--foreground</string>
  </dict>
</dict>
</plist>
EOF
  plutil -lint "$plist_tmp" >/dev/null
  mv "$plist_tmp" "$path"
}

install_agents() {
  chmod +x \
    "$ROOT/start-all-services.sh" \
    "$ROOT/scripts/persistent-android-emulator.sh" \
    "$ROOT/scripts/persistent-android-app.sh"

  write_agent \
    "$desktop_plist" \
    "$DESKTOP_LABEL" \
    "$ROOT/start-all-services.sh" \
    "$ROOT" \
    "Interactive" \
    "$LOG_DIR/persistent-desktop.log"
  write_agent \
    "$emulator_plist" \
    "$EMULATOR_LABEL" \
    "$ROOT/scripts/persistent-android-emulator.sh" \
    "$ROOT" \
    "Interactive" \
    "$LOG_DIR/persistent-emulator.log"
  write_agent \
    "$app_plist" \
    "$APP_LABEL" \
    "$ROOT/scripts/persistent-android-app.sh" \
    "$ROOT" \
    "Interactive" \
    "$LOG_DIR/persistent-android-app.log"
}

bootout_agent() {
  local label="$1"
  launchctl bootout "$DOMAIN/$label" >/dev/null 2>&1 || true
  local attempt
  for attempt in $(seq 1 50); do
    launchctl print "$DOMAIN/$label" >/dev/null 2>&1 || return 0
    sleep 0.2
  done
  return 1
}

bootstrap_agent() {
  local plist="$1"
  local attempt
  for attempt in $(seq 1 10); do
    if launchctl bootstrap "$DOMAIN" "$plist" 2>/dev/null; then
      return 0
    fi
    sleep 0.5
  done
  launchctl bootstrap "$DOMAIN" "$plist"
}

start_agents() {
  local spec
  local label
  for spec in \
    "$DESKTOP_LABEL:$desktop_plist" \
    "$EMULATOR_LABEL:$emulator_plist" \
    "$APP_LABEL:$app_plist"; do
    label="${spec%%:*}"
    plist="${spec#*:}"
    bootout_agent "$label"
    bootstrap_agent "$plist"
  done
}

stop_agents() {
  bootout_agent "$APP_LABEL"
  bootout_agent "$EMULATOR_LABEL"
  bootout_agent "$DESKTOP_LABEL"
}

show_status() {
  local label
  for label in "$DESKTOP_LABEL" "$EMULATOR_LABEL" "$APP_LABEL"; do
    echo "=== $label ==="
    launchctl print "$DOMAIN/$label" 2>/dev/null |
      sed -n '1,28p' || echo "not loaded"
  done
}

case "${1:-install}" in
  install)
    install_agents
    start_agents
    ;;
  start)
    install_agents
    start_agents
    ;;
  stop)
    stop_agents
    ;;
  restart)
    stop_agents
    install_agents
    start_agents
    ;;
  status)
    show_status
    ;;
  uninstall)
    stop_agents
    rm -f "$desktop_plist" "$emulator_plist" "$app_plist"
    ;;
  *)
    echo "用法: $0 [install|start|stop|restart|status|uninstall]" >&2
    exit 2
    ;;
esac
