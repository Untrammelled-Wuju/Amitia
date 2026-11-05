#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="$ROOT/AmitiaData"
LOG_DIR="$DATA_DIR/logs"

mkdir -p "$LOG_DIR"

resolve_android_sdk() {
  local candidate
  for candidate in \
    "${ANDROID_SDK_ROOT:-}" \
    "${ANDROID_HOME:-}" \
    "$HOME/Library/Android/sdk" \
    "/usr/local/share/android-sdk"; do
    [[ -n "$candidate" ]] || continue
    if [[ -x "$candidate/platform-tools/adb" && -x "$candidate/emulator/emulator" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

ANDROID_SDK="$(resolve_android_sdk)" || {
  echo "未找到同时包含 adb 与 emulator 的 Android SDK" >&2
  exit 1
}
ADB="$ANDROID_SDK/platform-tools/adb"
EMULATOR="$ANDROID_SDK/emulator/emulator"
AVD_NAME="${AMITIA_PERSISTENT_AVD:-Pixel_10_Pro}"

export ANDROID_HOME="$ANDROID_SDK"
export ANDROID_SDK_ROOT="$ANDROID_SDK"
export PATH="$DATA_DIR/node/bin:$ANDROID_SDK/platform-tools:$ANDROID_SDK/emulator:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"

"$ADB" start-server

serial="$("$ADB" devices | awk '$2 == "device" && $1 ~ /^emulator-/ { print $1; exit }')"
if [[ -n "$serial" ]]; then
  echo "复用已运行模拟器：$serial"
  while "$ADB" devices | awk '$2 == "device" && $1 == "'"$serial"'" { found=1 } END { exit found ? 0 : 1 }'; do
    sleep 30
  done
  echo "模拟器连接已断开：$serial" >&2
  exit 1
fi

"$EMULATOR" -list-avds | grep -Fxq "$AVD_NAME" || {
  echo "AVD 不存在：$AVD_NAME" >&2
  exit 1
}

echo "启动 Android 模拟器：$AVD_NAME"
exec "$EMULATOR" \
  -avd "$AVD_NAME" \
  -no-audio \
  -no-boot-anim \
  -no-snapshot-load \
  -no-snapshot-save \
  -memory 4096 \
  -cores 4
