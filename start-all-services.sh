#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="$ROOT/AmitiaData"
LOG_DIR="$DATA_DIR/logs"

mkdir -p "$DATA_DIR/qdrant" "$DATA_DIR/surrealdb" "$LOG_DIR"

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
  local roots=(
    "$ROOT/desktop/resources/databases/$TARGET"
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

stop_project_processes() {
  pkill -f "$ROOT/desktop/resources/core/AmitiaCore" >/dev/null 2>&1 || true
  pkill -f "$ROOT/AmitiaData/qdrant/qdrant" >/dev/null 2>&1 || true
  pkill -f "$ROOT/AmitiaData/surrealdb/surreal" >/dev/null 2>&1 || true
}

command -v node >/dev/null 2>&1 || { echo "未找到 node，请先安装 Node.js" >&2; exit 1; }
command -v pnpm >/dev/null 2>&1 || { echo "未找到 pnpm，请先启用 corepack pnpm" >&2; exit 1; }

echo "[1/5] 清理旧进程"
stop_project_processes
sleep 1

echo "[2/5] 准备核心 ($TARGET)"
if [ ! -f "$ROOT/desktop/resources/core/AmitiaCore" ] && [ ! -f "$ROOT/desktop/resources/core/$TARGET/AmitiaCore" ]; then
  command -v go >/dev/null 2>&1 || { echo "未找到 go，无法构建 AmitiaCore" >&2; exit 1; }
  (cd "$ROOT/desktop" && node scripts/build-amitiacore.mjs)
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

echo "[4/5] 检查依赖"
for package in front desktop; do
  if [ ! -d "$ROOT/$package/node_modules" ]; then
    pnpm --dir "$ROOT/$package" install
  fi
done

echo "[5/5] 启动桌面端与核心"
export AMITIA_RUNTIME_ROOT="$DATA_DIR"
export AMITIA_CONFIG_DIR="$DATA_DIR/config"
export AMITIA_WORKSPACE_DIR="$ROOT"
export AMITIA_DATA_DIR="$DATA_DIR"
export AMITIA_RUN_MODE=desktop
export AMITIA_EXTENSION_DEV_MODE=true

nohup pnpm --dir "$ROOT/desktop" dev > "$LOG_DIR/desktop.log" 2>&1 &
echo "$!" > "$LOG_DIR/desktop.pid"
disown >/dev/null 2>&1 || true

for _ in $(seq 1 90); do
  if curl -fsS http://127.0.0.1:18899/api/health >/dev/null 2>&1; then
    echo "服务已就绪: http://127.0.0.1:18899"
    echo "日志: $LOG_DIR/desktop.log"
    exit 0
  fi
  sleep 1
done

echo "启动超时，最近日志:" >&2
tail -n 40 "$LOG_DIR/desktop.log" >&2 || true
exit 1
