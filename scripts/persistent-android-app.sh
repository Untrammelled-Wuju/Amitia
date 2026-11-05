#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="$ROOT/AmitiaData"
LOG_DIR="$DATA_DIR/logs"
PACKAGE_NAME="com.amitia.amitia_app"
ACTIVITY_NAME="$PACKAGE_NAME/.MainActivity"

mkdir -p "$LOG_DIR"

resolve_android_sdk() {
  local candidate
  for candidate in \
    "${ANDROID_SDK_ROOT:-}" \
    "${ANDROID_HOME:-}" \
    "$HOME/Library/Android/sdk" \
    "/usr/local/share/android-sdk"; do
    [[ -n "$candidate" ]] || continue
    if [[ -x "$candidate/platform-tools/adb" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

ANDROID_SDK="$(resolve_android_sdk)" || {
  echo "未找到 Android SDK adb" >&2
  exit 1
}
ADB="$ANDROID_SDK/platform-tools/adb"
PROJECT_NODE="$DATA_DIR/node/bin/node"

export ANDROID_HOME="$ANDROID_SDK"
export ANDROID_SDK_ROOT="$ANDROID_SDK"
export PATH="$DATA_DIR/node/bin:$ANDROID_SDK/platform-tools:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"

"$ADB" start-server

wait_for_device() {
  local deadline=$((SECONDS + 300))
  local serial=""
  while (( SECONDS < deadline )); do
    serial="$("$ADB" devices | awk '$2 == "device" && $1 ~ /^emulator-/ { print $1; exit }')"
    if [[ -n "$serial" ]]; then
      printf '%s\n' "$serial"
      return 0
    fi
    sleep 2
  done
  return 1
}

SERIAL="$(wait_for_device)" || {
  echo "等待 Android 模拟器连接超时" >&2
  exit 1
}

boot_deadline=$((SECONDS + 240))
while (( SECONDS < boot_deadline )); do
  if [[ "$("$ADB" -s "$SERIAL" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r\n')" == "1" ]]; then
    break
  fi
  sleep 2
done
[[ "$("$ADB" -s "$SERIAL" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r\n')" == "1" ]] || {
  echo "Android 模拟器启动超时" >&2
  exit 1
}

find_latest_apk() {
  local version
  version="$("$PROJECT_NODE" -p 'require(process.argv[1]).version' "$ROOT/desktop/package.json")"
  find "$ROOT/artifacts" -type f -name "amitia-$version-*-arm64-v8a.apk" -print0 2>/dev/null |
    while IFS= read -r -d '' candidate; do
      printf '%s\t%s\n' "$(stat -f '%m' "$candidate")" "$candidate"
    done |
    sort -nr |
    head -n 1 |
    cut -f 2-
}

installed_version="$("$ADB" -s "$SERIAL" shell dumpsys package "$PACKAGE_NAME" 2>/dev/null | sed -n 's/.*versionName=//p' | head -n 1 | tr -d '\r\n' || true)"
if [[ -z "$installed_version" ]]; then
  APK_PATH="${AMITIA_PERSISTENT_APK:-$(find_latest_apk)}"
  [[ -n "$APK_PATH" && -f "$APK_PATH" ]] || {
    echo "未找到可安装的 Amitia APK" >&2
    exit 1
  }
  echo "安装 Amitia APK：$APK_PATH"
  "$ADB" -s "$SERIAL" install -r "$APK_PATH"
fi

start_app() {
  "$ADB" -s "$SERIAL" shell input keyevent 82 >/dev/null 2>&1 || true
  "$ADB" -s "$SERIAL" shell wm dismiss-keyguard >/dev/null 2>&1 || true
  "$ADB" -s "$SERIAL" shell am start -W -n "$ACTIVITY_NAME"
}

start_app

while true; do
  current_serial="$("$ADB" devices | awk '$2 == "device" && $1 ~ /^emulator-/ { print $1; exit }')"
  if [[ -z "$current_serial" ]]; then
    echo "Android 模拟器连接丢失" >&2
    exit 1
  fi
  SERIAL="$current_serial"
  if [[ -z "$("$ADB" -s "$SERIAL" shell pidof "$PACKAGE_NAME" 2>/dev/null | tr -d '\r\n' || true)" ]]; then
    start_app
  fi
  sleep 15
done
