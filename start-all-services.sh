#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="$ROOT/AmitiaData"
LOG_DIR="$DATA_DIR/logs"
RUN_DIR="$DATA_DIR/run"
START_LOCK_DIR="$RUN_DIR/start.lock"
START_LOCK_HELD=0
RUN_MODE="${1:-${AMITIA_RUN_MODE:-detached}}"

case "$RUN_MODE" in
  detached|--foreground) ;;
  *)
    echo "不支持的模式: $RUN_MODE" >&2
    exit 2
    ;;
esac

mkdir -p "$DATA_DIR/qdrant" "$DATA_DIR/surrealdb" "$DATA_DIR/node" "$LOG_DIR" "$RUN_DIR"

release_start_lock() {
  if [ "$START_LOCK_HELD" = "1" ]; then
    rm -f "$START_LOCK_DIR/pid"
    rmdir "$START_LOCK_DIR" >/dev/null 2>&1 || true
    START_LOCK_HELD=0
  fi
}

acquire_start_lock() {
  local owner=""
  if mkdir "$START_LOCK_DIR" 2>/dev/null; then
    printf '%s\n' "$$" > "$START_LOCK_DIR/pid"
    START_LOCK_HELD=1
    return
  fi
  if [ -f "$START_LOCK_DIR/pid" ]; then
    owner="$(tr -d '[:space:]' < "$START_LOCK_DIR/pid")"
  fi
  if [ -n "$owner" ] && kill -0 "$owner" 2>/dev/null; then
    owner_command="$(ps -p "$owner" -o command= 2>/dev/null || true)"
    case "$owner_command" in
      *start-all-services.sh*)
        echo "项目启动任务正在运行，PID=$owner" >&2
        exit 1
        ;;
    esac
  fi
  rm -f "$START_LOCK_DIR/pid"
  rmdir "$START_LOCK_DIR" >/dev/null 2>&1 || {
    echo "无法清理失效的项目启动锁: $START_LOCK_DIR" >&2
    exit 1
  }
  mkdir "$START_LOCK_DIR"
  printf '%s\n' "$$" > "$START_LOCK_DIR/pid"
  START_LOCK_HELD=1
}

trap release_start_lock EXIT
acquire_start_lock

case "$(uname -s)" in
  Darwin) HOST_OS=macos ;;
  Linux) HOST_OS=linux ;;
  *)
    echo "不支持的系统: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  arm64|aarch64) HOST_ARCH=arm64 ;;
  x86_64|amd64) HOST_ARCH=x64 ;;
  *)
    echo "不支持的架构: $(uname -m)" >&2
    exit 1
    ;;
esac

TARGET="$HOST_OS-$HOST_ARCH"

find_database_archive() {
  local kind="$1"
  local legacy="$2"
  local names=("$legacy" "${kind}-${TARGET}.zip")
  if [ "$kind" = "surrealdb" ]; then
    names+=("surreal-${TARGET}.zip")
  fi
  local roots=(
    "$ROOT/desktop/resources/databases/$TARGET/$kind"
    "$ROOT/desktop/resources/databases/$TARGET"
    "$ROOT/desktop/resources/release-runtime/$kind"
    "$ROOT/desktop/resources/$kind"
    "$ROOT/backend/$kind"
  )
  local root name
  for root in "${roots[@]}"; do
    for name in "${names[@]}"; do
      if [ -f "$root/$name" ]; then
        echo "$root/$name"
        return 0
      fi
    done
  done
  return 1
}

terminate_tree() {
  local pid="$1"
  [ -n "$pid" ] || return 0
  local child
  for child in $(pgrep -P "$pid" 2>/dev/null || true); do
    terminate_tree "$child"
  done
  kill "$pid" >/dev/null 2>&1 || true
  local waited=0
  while kill -0 "$pid" 2>/dev/null && [ "$waited" -lt 20 ]; do
    sleep 0.1
    waited=$((waited + 1))
  done
  kill -9 "$pid" >/dev/null 2>&1 || true
}

project_process_patterns() {
  printf '%s\n' \
    "$ROOT/desktop/node_modules/.*/electron/dist/Electron.app/Contents/MacOS/Electron" \
    "$ROOT/desktop/node_modules/.*/vite/bin/vite.js" \
    "node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5178" \
    "node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5178 .*" \
    "scripts/dev-vite.mjs" \
    "../scripts/dev-vite.mjs node_modules/vite/bin/vite.js" \
    "$ROOT/desktop/resources/core/AmitiaCore" \
    "$ROOT/desktop/resources/core/$TARGET/AmitiaCore" \
    "$ROOT/AmitiaData/plugin-host" \
    "$ROOT/AmitiaData/extensions-v2/installations" \
    "$ROOT/AmitiaData/qdrant/qdrant" \
    "$ROOT/AmitiaData/surrealdb/surreal"
}

project_processes_alive() {
  local pattern pids pid stat
  while IFS= read -r pattern; do
    pids="$(pgrep -f "$pattern" 2>/dev/null || true)"
    for pid in $pids; do
      stat="$(ps -o stat= -p "$pid" 2>/dev/null | tr -d '[:space:]')"
      case "$stat" in
        *U*|*Z*) continue ;;
      esac
      return 0
    done
  done < <(project_process_patterns)
  if lsof -tiTCP:5178 -sTCP:LISTEN >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

force_stop_project_processes() {
  local pattern
  while IFS= read -r pattern; do
    pkill -KILL -f "$pattern" >/dev/null 2>&1 || true
  done < <(project_process_patterns)
}

wait_project_processes_exit() {
  local attempt
  for attempt in $(seq 1 100); do
    if ! project_processes_alive; then
      return 0
    fi
    sleep 0.1
  done
  echo "旧项目进程未在 10 秒内退出，执行强制终止" >&2
  force_stop_project_processes
  for attempt in $(seq 1 50); do
    if ! project_processes_alive; then
      return 0
    fi
    sleep 0.1
  done
  echo "旧项目进程强制终止后仍未退出" >&2
  project_process_patterns | while IFS= read -r pattern; do
    pgrep -fl "$pattern" 2>/dev/null || true
  done >&2
  exit 1
}

stop_project_processes() {
  if [ -f "$LOG_DIR/desktop.pid" ]; then
    terminate_tree "$(tr -d '[:space:]' < "$LOG_DIR/desktop.pid")"
  fi
  pkill -f "$ROOT/desktop/node_modules/.*/electron/dist/Electron.app/Contents/MacOS/Electron" >/dev/null 2>&1 || true
  pkill -f "$ROOT/desktop/node_modules/.*/vite/bin/vite.js" >/dev/null 2>&1 || true
  pkill -f "node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5178" >/dev/null 2>&1 || true
  pkill -f "node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5178 .*" >/dev/null 2>&1 || true
  pkill -f "scripts/dev-vite.mjs" >/dev/null 2>&1 || true
  pkill -f "../scripts/dev-vite.mjs node_modules/vite/bin/vite.js" >/dev/null 2>&1 || true
  project_vite_pid="$(lsof -tiTCP:5178 -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$project_vite_pid" ]; then
    project_vite_command="$(ps -p "$project_vite_pid" -o command= 2>/dev/null || true)"
    case "$project_vite_command" in
      *node_modules/vite/bin/vite.js*)
        project_vite_parent="$(ps -p "$project_vite_pid" -o ppid= 2>/dev/null | tr -d '[:space:]' || true)"
        if [ -n "$project_vite_parent" ]; then terminate_tree "$project_vite_parent"; fi
        terminate_tree "$project_vite_pid"
        ;;
      *)
        echo "端口 5178 被非项目进程占用: $project_vite_command" >&2
        exit 1
        ;;
    esac
  fi
  pkill -f "$ROOT/desktop/resources/core/AmitiaCore" >/dev/null 2>&1 || true
  pkill -f "$ROOT/desktop/resources/core/$TARGET/AmitiaCore" >/dev/null 2>&1 || true
  pkill -f "$ROOT/AmitiaData/plugin-host" >/dev/null 2>&1 || true
  pkill -f "$ROOT/AmitiaData/extensions-v2/installations" >/dev/null 2>&1 || true
  pkill -f "$ROOT/AmitiaData/qdrant/qdrant" >/dev/null 2>&1 || true
  pkill -f "$ROOT/AmitiaData/surrealdb/surreal" >/dev/null 2>&1 || true
  wait_project_processes_exit
}

if [ ! -x "$DATA_DIR/node/bin/node" ]; then
  unzip -oq "$ROOT/desktop/resources/core/node/node.zip" -d "$DATA_DIR/node"
  chmod +x "$DATA_DIR/node/bin/node"
fi
export PATH="$DATA_DIR/node/bin:/usr/local/bin:/usr/bin:/bin"
command -v node >/dev/null 2>&1 || { echo "项目 Node 运行时不可用" >&2; exit 1; }
command -v pnpm >/dev/null 2>&1 || { echo "未找到 pnpm，请先启用 corepack pnpm" >&2; exit 1; }

echo "[1/5] 清理旧进程"
stop_project_processes
rm -f "$RUN_DIR/endpoint.json"

echo "[2/5] 准备核心 ($TARGET)"
if [ ! -f "$ROOT/desktop/resources/core/AmitiaCore" ] && [ ! -f "$ROOT/desktop/resources/core/$TARGET/AmitiaCore" ]; then
  command -v go >/dev/null 2>&1 || { echo "未找到 go，无法构建 AmitiaCore" >&2; exit 1; }
  (cd "$ROOT/desktop" && node scripts/build-amitiacore.mjs)
fi

core_root="$ROOT/desktop/resources/core/AmitiaCore"
core_target="$ROOT/desktop/resources/core/$TARGET/AmitiaCore"
if [ -f "$core_root" ]; then
  mkdir -p "$(dirname "$core_target")"
  if [ ! -f "$core_target" ] || ! cmp -s "$core_root" "$core_target"; then
    cp "$core_root" "$core_target"
    chmod +x "$core_target"
  fi
fi

echo "[3/5] 准备数据库归档"
if [ ! -f "$DATA_DIR/qdrant/qdrant.zip" ]; then
  qdrant_archive="$(find_database_archive qdrant qdrant.zip)" || {
    echo "缺少 Qdrant 归档，请放置 desktop/resources/databases/$TARGET/qdrant.zip" >&2
    exit 1
  }
  cp "$qdrant_archive" "$DATA_DIR/qdrant/qdrant.zip"
fi
if [ ! -f "$DATA_DIR/surrealdb/surreal.zip" ]; then
  surreal_archive="$(find_database_archive surrealdb surreal.zip)" || {
    echo "缺少 SurrealDB 归档，请放置 desktop/resources/databases/$TARGET/surreal.zip" >&2
    exit 1
  }
  cp "$surreal_archive" "$DATA_DIR/surrealdb/surreal.zip"
fi
if [ ! -x "$DATA_DIR/qdrant/qdrant" ]; then
  unzip -oq "$DATA_DIR/qdrant/qdrant.zip" -d "$DATA_DIR/qdrant"
  chmod +x "$DATA_DIR/qdrant/qdrant"
fi
if [ ! -x "$DATA_DIR/surrealdb/surreal" ]; then
  unzip -oq "$DATA_DIR/surrealdb/surreal.zip" -d "$DATA_DIR/surrealdb"
  chmod +x "$DATA_DIR/surrealdb/surreal"
fi

echo "[4/5] 检查依赖"
for package in front desktop; do
  if [ ! -d "$ROOT/$package/node_modules" ]; then
    pnpm --dir "$ROOT/$package" install
  fi
done

ensure_web_port_free() {
  local attempt pid command
  for attempt in $(seq 1 150); do
    pid="$(lsof -tiTCP:5178 -sTCP:LISTEN 2>/dev/null | head -n 1 || true)"
    if [ -z "$pid" ]; then
      return 0
    fi
    command="$(ps -p "$pid" -o command= 2>/dev/null || true)"
    case "$command" in
      *node_modules/vite/bin/vite.js*|*scripts/dev-vite.mjs*)
        terminate_tree "$pid"
        kill -9 "$pid" >/dev/null 2>&1 || true
        ;;
      *)
        echo "端口 5178 被非项目进程占用: $command" >&2
        exit 1
        ;;
    esac
    sleep 0.2
  done
  echo "端口 5178 未在 30 秒内释放" >&2
  exit 1
}

echo "[5/5] 启动桌面端与核心"
ensure_web_port_free
export AMITIA_RUNTIME_ROOT="$DATA_DIR"
export AMITIA_CONFIG_DIR="$DATA_DIR/config"
export AMITIA_WORKSPACE_DIR="$ROOT"
export AMITIA_DATA_DIR="$DATA_DIR"
export AMITIA_EXTENSION_DEV_MODE=true
export AMITIA_NODE_BIN="$DATA_DIR/node/bin/node"

foreground_mode=0
if [ "$RUN_MODE" = "--foreground" ]; then
  foreground_mode=1
  echo "$$" > "$LOG_DIR/desktop.pid"
  pnpm --dir "$ROOT/desktop" dev &
  desktop_pid="$!"
else
  nohup pnpm --dir "$ROOT/desktop" dev > "$LOG_DIR/desktop.log" 2>&1 &
  echo "$!" > "$LOG_DIR/desktop.pid"
  desktop_pid="$!"
  disown >/dev/null 2>&1 || true
fi

for _ in $(seq 1 90); do
  if ! kill -0 "$desktop_pid" 2>/dev/null; then
    echo "桌面端进程已退出:" >&2
    tail -n 40 "$LOG_DIR/desktop.log" >&2 || true
    exit 1
  fi
  core_port=""
  core_pid=""
  if [ -f "$DATA_DIR/run/endpoint.json" ]; then
    core_port="$(node -e 'const fs=require("fs");const value=JSON.parse(fs.readFileSync(process.argv[1],"utf8"));process.stdout.write(String(value.port||""))' "$DATA_DIR/run/endpoint.json" 2>/dev/null || true)"
    core_pid="$(node -e 'const fs=require("fs");const value=JSON.parse(fs.readFileSync(process.argv[1],"utf8"));process.stdout.write(String(value.pid||""))' "$DATA_DIR/run/endpoint.json" 2>/dev/null || true)"
  fi
  if [ -n "$core_port" ] && [ -n "$core_pid" ] && kill -0 "$core_pid" 2>/dev/null && curl -fsS "http://127.0.0.1:${core_port}/api/health" >/dev/null 2>&1; then
    release_start_lock
    echo "服务已就绪: http://127.0.0.1:${core_port}"
    echo "日志: $LOG_DIR/desktop.log"
    if [ "$foreground_mode" = "1" ]; then
      wait "$desktop_pid"
      exit $?
    fi
    exit 0
  fi
  sleep 1
done

echo "启动超时，最近日志:" >&2
tail -n 40 "$LOG_DIR/desktop.log" >&2 || true
terminate_tree "$desktop_pid"
exit 1
