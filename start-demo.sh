#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="$ROOT/AmitiaData"
LOG_DIR="$DATA_DIR/logs"
RUN_DIR="$DATA_DIR/run"
DEMO_LOG="$LOG_DIR/demo-start.log"
EMULATOR_LOG="$LOG_DIR/demo-emulator.log"
LOCK_DIR="$RUN_DIR/demo-start.lock"
DESKTOP_PID_FILE="$LOG_DIR/desktop.pid"
ANDROID_SCREENSHOT="$LOG_DIR/demo-android.png"
ANDROID_RUNTIME_LOG="$LOG_DIR/demo-android-runtime.log"

mkdir -p "$LOG_DIR" "$RUN_DIR"
exec > >(tee -a "$DEMO_LOG") 2>&1

report_failure() {
  local status=$?
  printf '未处理错误（第 %s 行，退出码 %s）：%s\n' "${BASH_LINENO[0]}" "$status" "$BASH_COMMAND" >&2
}
trap report_failure ERR

fail() {
  printf '\n演示启动失败：%s\n' "$*" >&2
  exit 1
}

cleanup_lock() {
  if [[ -d "$LOCK_DIR" ]]; then
    local owner=""
    if [[ -f "$LOCK_DIR/pid" ]]; then
      owner="$(tr -d '[:space:]' < "$LOCK_DIR/pid")"
    fi
    if [[ "$owner" == "$$" ]]; then
      rm -f "$LOCK_DIR/pid"
      rmdir "$LOCK_DIR" 2>/dev/null || true
    fi
  fi
}

acquire_lock() {
  if mkdir "$LOCK_DIR" 2>/dev/null; then
    printf '%s\n' "$$" > "$LOCK_DIR/pid"
    trap cleanup_lock EXIT
    return
  fi
  if [[ -f "$LOCK_DIR/pid" ]]; then
    local owner
    owner="$(tr -d '[:space:]' < "$LOCK_DIR/pid")"
    if [[ -n "$owner" ]] && kill -0 "$owner" 2>/dev/null; then
      fail "已有演示启动任务在运行，PID=$owner"
    fi
    rm -f "$LOCK_DIR/pid"
    rmdir "$LOCK_DIR" 2>/dev/null || fail "无法清理失效的演示启动锁"
  fi
  mkdir "$LOCK_DIR" || fail "无法创建演示启动锁"
  printf '%s\n' "$$" > "$LOCK_DIR/pid"
  trap cleanup_lock EXIT
}

terminate_tree() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  local child
  for child in $(pgrep -P "$pid" 2>/dev/null || true); do
    terminate_tree "$child"
  done
  kill "$pid" >/dev/null 2>&1 || true
}

stop_previous_desktop() {
  if [[ -f "$DESKTOP_PID_FILE" ]]; then
    terminate_tree "$(tr -d '[:space:]' < "$DESKTOP_PID_FILE")"
    rm -f "$DESKTOP_PID_FILE"
  fi
  local pattern
  for pattern in \
    "$ROOT/desktop/node_modules/.bin/electron" \
    "$ROOT/desktop/node_modules/electron/dist" \
    "$ROOT/desktop/node_modules/.pnpm/.*/electron/dist" \
    "$ROOT/desktop/node_modules/vite" \
    "$ROOT/scripts/dev-vite.mjs"; do
    local pid
    for pid in $(pgrep -f "$pattern" 2>/dev/null || true); do
      [[ "$pid" == "$$" ]] && continue
      terminate_tree "$pid"
    done
  done
  sleep 1
}

resolve_android_sdk() {
  local candidate
  for candidate in \
    "${ANDROID_SDK_ROOT:-}" \
    "${ANDROID_HOME:-}" \
    "$HOME/Library/Android/sdk" \
    "/usr/local/share/android-sdk"; do
    [[ -n "$candidate" ]] || continue
    if [[ -x "$candidate/platform-tools/adb" && -x "$candidate/emulator/emulator" ]]; then
      ANDROID_SDK="$candidate"
      ADB="$candidate/platform-tools/adb"
      EMULATOR="$candidate/emulator/emulator"
      return
    fi
  done
  fail "未找到同时包含 adb 与 emulator 的 Android SDK"
}

resolve_build_tool() {
  local name="$1"
  local tool
  tool="$(find "$ANDROID_SDK/build-tools" -type f -name "$name" 2>/dev/null | sort | tail -n 1)"
  [[ -n "$tool" && -x "$tool" ]] || fail "Android build-tools 缺少 $name"
  printf '%s\n' "$tool"
}

resolve_apk() {
  if [[ -n "${AMITIA_DEMO_APK:-}" ]]; then
    APK_PATH="$AMITIA_DEMO_APK"
    case "$APK_PATH" in
      /*) ;;
      *) APK_PATH="$ROOT/$APK_PATH" ;;
    esac
  else
    local candidate candidate_mtime latest_mtime latest_path
    latest_mtime=-1
    latest_path=""
    while IFS= read -r -d '' candidate; do
      candidate_mtime="$(stat -f '%m' "$candidate")" || fail "无法读取 APK 时间：$candidate"
      if (( candidate_mtime > latest_mtime )); then
        latest_mtime="$candidate_mtime"
        latest_path="$candidate"
      fi
    done < <(find "$ROOT/artifacts" -type f -name "amitia-$PROJECT_VERSION-*-arm64-v8a.apk" -print0 2>/dev/null)
    APK_PATH="$latest_path"
  fi
  [[ -f "$APK_PATH" ]] || fail "未找到项目版本 $PROJECT_VERSION 的 Release APK，可通过 AMITIA_DEMO_APK 指定"
}

verify_apk() {
  local badging
  badging="$("$AAPT" dump badging "$APK_PATH" 2>&1)" || {
    printf '%s\n' "$badging" >&2
    fail "无法读取 APK 清单"
  }
  PACKAGE_NAME="$(printf '%s\n' "$badging" | sed -n "s/^package: name='\([^']*\)'.*/\1/p" | head -n 1)"
  APK_VERSION_NAME="$(printf '%s\n' "$badging" | sed -n "s/^package: .* versionName='\([^']*\)'.*/\1/p" | head -n 1)"
  APK_VERSION_CODE="$(printf '%s\n' "$badging" | sed -n "s/^package: .* versionCode='\([^']*\)'.*/\1/p" | head -n 1)"
  [[ "$PACKAGE_NAME" == "com.amitia.amitia_app" ]] || fail "APK 包名不正确：$PACKAGE_NAME"
  [[ "$APK_VERSION_NAME" == "$PROJECT_VERSION" ]] || fail "APK 版本 $APK_VERSION_NAME 与项目版本 $PROJECT_VERSION 不一致"
  [[ -n "$APK_VERSION_CODE" ]] || fail "无法读取 APK versionCode"
  if [[ "$badging" == *"application-debuggable"* ]]; then
    fail "禁止使用 Debug APK"
  fi
  if [[ "$badging" != *"native-code: 'arm64-v8a'"* ]]; then
    fail "APK 不包含 arm64-v8a"
  fi
  local signature
  signature="$("$APKSIGNER" verify --verbose "$APK_PATH" 2>&1)" || {
    printf '%s\n' "$signature" >&2
    fail "APK 签名校验失败"
  }
  local alignment
  alignment="$("$ZIPALIGN" -c -P 16 4 "$APK_PATH" 2>&1)" || {
    printf '%s\n' "$alignment" >&2
    fail "APK 未通过 16 KB zipalign 校验"
  }
  APK_SHA256="$(shasum -a 256 "$APK_PATH" | awk '{print $1}')"
}

online_emulator() {
  "$ADB" devices | awk '$2 == "device" && $1 ~ /^emulator-/ { print $1; exit }'
}

wait_for_emulator() {
  local deadline=$((SECONDS + BOOT_TIMEOUT))
  local serial=""
  while (( SECONDS < deadline )); do
    serial="$(online_emulator)"
    if [[ -n "$serial" ]]; then
      EMULATOR_SERIAL="$serial"
      break
    fi
    if [[ -n "${EMULATOR_PID:-}" ]] && ! kill -0 "$EMULATOR_PID" 2>/dev/null; then
      tail -n 80 "$EMULATOR_LOG" >&2 || true
      fail "Android 模拟器进程提前退出"
    fi
    sleep 2
  done
  [[ -n "${EMULATOR_SERIAL:-}" ]] || fail "等待 Android 模拟器连接超时"
}

wait_for_android_boot() {
  local deadline=$((SECONDS + BOOT_TIMEOUT))
  while (( SECONDS < deadline )); do
    local booted
    booted="$("$ADB" -s "$EMULATOR_SERIAL" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r\n')"
    if [[ "$booted" == "1" ]]; then
      return
    fi
    sleep 2
  done
  fail "Android 模拟器启动超时"
}

dismiss_anr_dialogs() {
  local attempt window_dump dialog_dump bounds x1 y1 x2 y2
  for attempt in 1 2 3; do
    if ! window_dump="$("$ADB" -s "$EMULATOR_SERIAL" shell dumpsys window windows 2>&1)"; then
      printf '%s\n' "$window_dump" >&2
      fail "读取 Android 窗口状态失败"
    fi
    [[ "$window_dump" == *"Application Not Responding"* ]] || return 0
    "$ADB" -s "$EMULATOR_SERIAL" shell uiautomator dump /sdcard/amitia-anr.xml >/dev/null
    if ! dialog_dump="$("$ADB" -s "$EMULATOR_SERIAL" exec-out cat /sdcard/amitia-anr.xml 2>&1)"; then
      printf '%s\n' "$dialog_dump" >&2
      fail "读取 Android ANR 对话框失败"
    fi
    bounds="$(printf '%s' "$dialog_dump" | sed -n 's/.*resource-id="android:id\/aerr_wait"[^>]*bounds="\[\([0-9][0-9]*\),\([0-9][0-9]*\)\]\[\([0-9][0-9]*\),\([0-9][0-9]*\)\]".*/\1 \2 \3 \4/p')"
    [[ -n "$bounds" ]] || return 0
    set -- $bounds
    x1="$1"
    y1="$2"
    x2="$3"
    y2="$4"
    "$ADB" -s "$EMULATOR_SERIAL" shell input tap "$(((x1 + x2) / 2))" "$(((y1 + y2) / 2))"
    sleep 2
  done
}

activate_android_app() {
  local start_output
  start_output="$("$ADB" -s "$EMULATOR_SERIAL" shell am start -W -n com.amitia.amitia_app/.MainActivity 2>&1)" || {
    printf '%s\n' "$start_output" >&2
    fail "APK 页面聚焦失败"
  }
  [[ "$start_output" == *"com.amitia.amitia_app/.MainActivity"* ]] || {
    printf '%s\n' "$start_output" >&2
    fail "APK 页面聚焦命令未返回目标 Activity"
  }
}

wait_for_android_ui() {
  local deadline=$((SECONDS + UI_TIMEOUT))
  local ui_dump=""
  while (( SECONDS < deadline )); do
    "$ADB" -s "$EMULATOR_SERIAL" shell uiautomator dump /sdcard/amitia-demo-ui.xml >/dev/null 2>&1 || true
    ui_dump="$("$ADB" -s "$EMULATOR_SERIAL" exec-out cat /sdcard/amitia-demo-ui.xml 2>/dev/null || true)"
    if [[ "$ui_dump" == *'package="com.amitia.amitia_app"'* && "$ui_dump" =~ (text|content-desc)=\"[^\"]+\" ]]; then
      return
    fi
    sleep 2
  done
  fail "APK 已启动但界面在 $UI_TIMEOUT 秒内未渲染"
}

ensure_emulator() {
  local reuse="${AMITIA_DEMO_REUSE_EMULATOR:-1}"
  if [[ "$reuse" != "0" ]]; then
    EMULATOR_SERIAL="$(online_emulator)"
    if [[ -n "$EMULATOR_SERIAL" ]]; then
      printf '复用已连接模拟器：%s\n' "$EMULATOR_SERIAL"
      wait_for_android_boot
      return
    fi
  else
    [[ -z "$(online_emulator)" ]] || fail "已有模拟器运行且已禁用复用"
  fi

  AVD_NAME="${AMITIA_DEMO_AVD:-Pixel_10_Pro}"
  "$EMULATOR" -list-avds | grep -Fxq "$AVD_NAME" || fail "AVD 不存在：$AVD_NAME"
  : > "$EMULATOR_LOG"
  nohup "$EMULATOR" -avd "$AVD_NAME" -no-audio -no-boot-anim -no-snapshot-load -no-snapshot-save -memory 4096 -cores 4 </dev/null > "$EMULATOR_LOG" 2>&1 &
  EMULATOR_PID=$!
  disown >/dev/null 2>&1 || true
  printf '已启动 AVD：%s，PID=%s\n' "$AVD_NAME" "$EMULATOR_PID"
  wait_for_emulator
  wait_for_android_boot
  sleep 5
}

install_and_open_apk() {
  local abi
  abi="$("$ADB" -s "$EMULATOR_SERIAL" shell getprop ro.product.cpu.abi | tr -d '\r\n')"
  [[ "$abi" == "arm64-v8a" ]] || fail "模拟器 ABI 为 $abi，无法安装 arm64-v8a APK"

  local input_output
  if ! input_output="$("$ADB" -s "$EMULATOR_SERIAL" shell input keyevent 82 2>&1)"; then
    printf '%s\n' "$input_output" >&2
    fail "唤醒 Android 模拟器失败"
  fi
  if ! input_output="$("$ADB" -s "$EMULATOR_SERIAL" shell wm dismiss-keyguard 2>&1)"; then
    printf '%s\n' "$input_output" >&2
    fail "关闭 Android 锁屏失败"
  fi

  local debug_packages
  if ! debug_packages="$("$ADB" -s "$EMULATOR_SERIAL" shell pm list packages com.amitia.amitia_app.debug 2>&1 | tr -d '\r')"; then
    printf '%s\n' "$debug_packages" >&2
    fail "读取 Android Debug 包状态失败"
  fi
  if [[ "$debug_packages" == *"package:com.amitia.amitia_app.debug"* ]]; then
    local uninstall_output
    uninstall_output="$("$ADB" -s "$EMULATOR_SERIAL" uninstall com.amitia.amitia_app.debug 2>&1)" || {
      printf '%s\n' "$uninstall_output" >&2
      fail "卸载旧 Debug 包失败"
    }
    [[ "$uninstall_output" == "Success" ]] || fail "卸载旧 Debug 包异常：$uninstall_output"
  fi

  if [[ "${AMITIA_DEMO_SKIP_INSTALL:-0}" == "1" ]]; then
    printf '按配置跳过 APK 覆盖安装\n'
  else
    printf '安装 APK：%s\n' "$APK_PATH"
    local install_output
    install_output="$("$ADB" -s "$EMULATOR_SERIAL" install -r "$APK_PATH" 2>&1)" || {
      printf '%s\n' "$install_output" >&2
      fail "APK 安装失败"
    }
    [[ "$install_output" == *"Success"* ]] || {
      printf '%s\n' "$install_output" >&2
      fail "APK 安装结果不是 Success"
    }
  fi

  local package_dump
  package_dump="$("$ADB" -s "$EMULATOR_SERIAL" shell dumpsys package com.amitia.amitia_app 2>&1)" || {
    printf '%s\n' "$package_dump" >&2
    fail "无法读取已安装应用信息"
  }
  [[ "$package_dump" == *"versionName=$PROJECT_VERSION"* ]] || fail "设备内 versionName 与项目版本不一致"
  [[ "$package_dump" == *"versionCode=$APK_VERSION_CODE"* ]] || fail "设备内 versionCode 与 APK 不一致"
  if ! debug_packages="$("$ADB" -s "$EMULATOR_SERIAL" shell pm list packages com.amitia.amitia_app.debug 2>&1 | tr -d '\r')"; then
    printf '%s\n' "$debug_packages" >&2
    fail "复核 Android Debug 包状态失败"
  fi
  [[ -z "$debug_packages" ]] || fail "设备内仍存在 Debug 包"

  if ! input_output="$("$ADB" -s "$EMULATOR_SERIAL" shell am force-stop com.amitia.amitia_app 2>&1)"; then
    printf '%s\n' "$input_output" >&2
    fail "停止旧 APK 进程失败"
  fi
  if ! input_output="$("$ADB" -s "$EMULATOR_SERIAL" logcat -c 2>&1)"; then
    printf '%s\n' "$input_output" >&2
    fail "清理 Android 日志失败"
  fi
  local start_output
  start_output="$("$ADB" -s "$EMULATOR_SERIAL" shell am start -W -n com.amitia.amitia_app/.MainActivity 2>&1)" || {
    printf '%s\n' "$start_output" >&2
    fail "APK 启动失败"
  }
  printf '%s\n' "$start_output"
  if [[ "$start_output" != *"com.amitia.amitia_app/.MainActivity"* ]]; then
    fail "APK 启动命令未返回目标 Activity"
  fi

  local deadline=$((SECONDS + 30))
  local process_id=""
  while (( SECONDS < deadline )); do
    process_id="$("$ADB" -s "$EMULATOR_SERIAL" shell pidof com.amitia.amitia_app 2>/dev/null | tr -d '\r\n' || true)"
    if [[ -n "$process_id" ]]; then
      break
    fi
    sleep 1
  done
  [[ -n "$process_id" ]] || fail "APK 启动后未发现应用进程"
  ANDROID_PID="$process_id"

  local runtime_deadline=$((SECONDS + RUNTIME_TIMEOUT))
  ANDROID_RUNTIME_STATUS="STARTING"
  local runtime_logs=""
  while (( SECONDS < runtime_deadline )); do
    if ! runtime_logs="$("$ADB" -s "$EMULATOR_SERIAL" logcat -d --pid="$ANDROID_PID" AmitiaRuntime:V AndroidRuntime:E '*:S' 2>&1)"; then
      printf '%s\n' "$runtime_logs" >&2
      fail "读取 Android Runtime 状态失败"
    fi
    if [[ "$runtime_logs" == *"snapshot: state=READY"* ]]; then
      ANDROID_RUNTIME_STATUS="READY"
      break
    fi
    if [[ "$runtime_logs" == *"snapshot: state=FAILED"* || "$runtime_logs" == *"Runtime startup failed"* || "$runtime_logs" == *"recovery budget exhausted"* ]]; then
      ANDROID_RUNTIME_STATUS="FAILED"
      "$ADB" -s "$EMULATOR_SERIAL" logcat -d --pid="$ANDROID_PID" > "$ANDROID_RUNTIME_LOG"
      break
    fi
    process_id="$("$ADB" -s "$EMULATOR_SERIAL" shell pidof com.amitia.amitia_app 2>/dev/null | tr -d '\r\n' || true)"
    [[ -n "$process_id" ]] || fail "Android Runtime 启动期间应用进程退出"
    ANDROID_PID="$process_id"
    sleep 2
  done

  if [[ "$ANDROID_RUNTIME_STATUS" != "READY" ]]; then
    "$ADB" -s "$EMULATOR_SERIAL" logcat -d --pid="$ANDROID_PID" > "$ANDROID_RUNTIME_LOG" 2>&1 || true
    "$ADB" -s "$EMULATOR_SERIAL" exec-out screencap -p > "$ANDROID_SCREENSHOT" || true
    fail "Android Runtime 未就绪，状态=$ANDROID_RUNTIME_STATUS；完整原始日志：$ANDROID_RUNTIME_LOG"
  fi

  activate_android_app
  wait_for_android_ui

  local activity_dump
  activity_dump="$("$ADB" -s "$EMULATOR_SERIAL" shell dumpsys activity activities 2>&1)" || {
    printf '%s\n' "$activity_dump" >&2
    fail "无法读取 Android 前台页面"
  }
  [[ "$activity_dump" == *"com.amitia.amitia_app"* ]] || fail "APK 进程存在但页面未处于活动栈"
  dismiss_anr_dialogs
  "$ADB" -s "$EMULATOR_SERIAL" exec-out screencap -p > "$ANDROID_SCREENSHOT" || fail "Android 截图失败"
}

start_desktop() {
  stop_previous_desktop
  "$ROOT/start-all-services.sh"
  [[ -f "$DESKTOP_PID_FILE" ]] || fail "桌面端启动后未生成 PID 文件"
  DESKTOP_PID="$(tr -d '[:space:]' < "$DESKTOP_PID_FILE")"
  kill -0 "$DESKTOP_PID" 2>/dev/null || fail "桌面端进程已退出"
  CORE_PORT="$("$PROJECT_NODE" -e 'const fs=require("fs");const value=JSON.parse(fs.readFileSync(process.argv[1],"utf8"));process.stdout.write(String(value.port||""))' "$RUN_DIR/endpoint.json")"
  [[ -n "$CORE_PORT" ]] || fail "核心端点未写入端口"
  HEALTH_JSON="$(curl -fsS "http://127.0.0.1:$CORE_PORT/api/health")" || fail "核心健康接口不可用"
  HEALTH_VERSION="$(printf '%s' "$HEALTH_JSON" | "$PROJECT_NODE" -e 'const fs=require("fs");const value=JSON.parse(fs.readFileSync(0,"utf8"));const health=value.data||value;process.stdout.write(String(health.version||""))')"
  [[ "$HEALTH_VERSION" == "$PROJECT_VERSION" ]] || fail "核心版本 $HEALTH_VERSION 与项目版本 $PROJECT_VERSION 不一致"
  HEALTH_READY="$(printf '%s' "$HEALTH_JSON" | "$PROJECT_NODE" -e 'const fs=require("fs");const value=JSON.parse(fs.readFileSync(0,"utf8"));const health=value.data||value;process.stdout.write(String(health.ready===true))')"
  [[ "$HEALTH_READY" == "true" ]] || fail "核心健康接口未返回 ready=true"
  local deadline=$((SECONDS + 30))
  ELECTRON_PID=""
  while (( SECONDS < deadline )); do
    ELECTRON_PID="$(pgrep -f "$ROOT/desktop/node_modules/.pnpm/.*/electron/dist/Electron.app/Contents/MacOS/Electron" 2>/dev/null | head -n 1 || true)"
    if [[ -n "$ELECTRON_PID" ]]; then
      break
    fi
    sleep 1
  done
  [[ -n "$ELECTRON_PID" ]] || fail "桌面端 Core 已启动，但 Electron 窗口进程未运行"
  READY_JSON="$(curl -fsS "http://127.0.0.1:$CORE_PORT/readyz")" || fail "核心就绪接口不可用"
}

usage() {
  printf '%s\n' \
    "用法：./start-demo.sh" \
    "" \
    "可选环境变量：" \
    "  AMITIA_DEMO_AVD=名称          指定 AVD，默认 Pixel_10_Pro" \
    "  AMITIA_DEMO_APK=路径          指定当前版本 Release APK" \
    "  AMITIA_DEMO_REUSE_EMULATOR=0  不复用已运行模拟器" \
    "  AMITIA_DEMO_SKIP_INSTALL=1    跳过 APK 覆盖安装，仅启动已安装版本" \
    "  AMITIA_DEMO_BOOT_TIMEOUT=240  模拟器启动超时秒数" \
    "  AMITIA_DEMO_RUNTIME_TIMEOUT=600  Android Runtime 状态等待秒数" \
    "  AMITIA_DEMO_UI_TIMEOUT=60        Android 首屏渲染等待秒数"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

[[ "$(uname -s)" == "Darwin" ]] || fail "当前演示脚本仅支持 macOS 桌面端与 Android 模拟器"
[[ "$(uname -m)" == "arm64" ]] || fail "当前 Release APK 仅支持 arm64-v8a，请使用 Apple Silicon Mac"
[[ $# -eq 0 ]] || fail "不支持参数：$*；使用 --help 查看环境变量"

acquire_lock
resolve_android_sdk
export ANDROID_HOME="$ANDROID_SDK"
export ANDROID_SDK_ROOT="$ANDROID_SDK"
AAPT="$(resolve_build_tool aapt)"
APKSIGNER="$(resolve_build_tool apksigner)"
ZIPALIGN="$(resolve_build_tool zipalign)"

PROJECT_NODE="$DATA_DIR/node/bin/node"
[[ -x "$PROJECT_NODE" ]] || fail "缺少项目 Node：$PROJECT_NODE"
export PATH="$DATA_DIR/node/bin:$ANDROID_SDK/platform-tools:$ANDROID_SDK/emulator:$PATH"
command -v pnpm >/dev/null 2>&1 || fail "未找到 pnpm"

PROJECT_VERSION="$("$PROJECT_NODE" -p 'require(process.argv[1]).version' "$ROOT/desktop/package.json")"
resolve_apk
verify_apk
BOOT_TIMEOUT="${AMITIA_DEMO_BOOT_TIMEOUT:-240}"
RUNTIME_TIMEOUT="${AMITIA_DEMO_RUNTIME_TIMEOUT:-600}"
UI_TIMEOUT="${AMITIA_DEMO_UI_TIMEOUT:-60}"

printf '\n=== Amitia 一键演示启动 ===\n'
printf '项目版本：%s\n' "$PROJECT_VERSION"
printf 'APK：%s\n' "$APK_PATH"
printf 'APK SHA-256：%s\n' "$APK_SHA256"
printf '日志：%s\n' "$DEMO_LOG"

printf '\n[1/4] 准备 Android 模拟器\n'
ensure_emulator
printf '模拟器就绪：%s\n' "$EMULATOR_SERIAL"

printf '\n[2/4] 安装并打开 Android APK\n'
install_and_open_apk

printf '\n[3/4] 启动完整桌面项目\n'
start_desktop

printf '\n[4/4] 验证演示状态\n'
activate_android_app
wait_for_android_ui
"$ADB" -s "$EMULATOR_SERIAL" exec-out screencap -p > "$ANDROID_SCREENSHOT" || fail "Android 截图失败"
printf 'Android：%s，包名=%s，versionCode=%s，PID=%s\n' "$EMULATOR_SERIAL" "$PACKAGE_NAME" "$APK_VERSION_CODE" "$ANDROID_PID"
printf 'Android Runtime：%s\n' "$ANDROID_RUNTIME_STATUS"
printf '桌面端 PID：%s，Electron PID：%s\n' "$DESKTOP_PID" "$ELECTRON_PID"
printf '核心健康接口：http://127.0.0.1:%s/api/health\n' "$CORE_PORT"
printf '核心就绪接口：http://127.0.0.1:%s/readyz\n' "$CORE_PORT"
printf 'Android 截图：%s\n' "$ANDROID_SCREENSHOT"
printf '桌面日志：%s\n' "$LOG_DIR/desktop.log"
printf '模拟器日志：%s\n' "$EMULATOR_LOG"
printf '\n演示环境已就绪。\n'
