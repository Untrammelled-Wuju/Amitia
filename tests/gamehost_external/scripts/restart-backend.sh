#!/usr/bin/env bash
set -euo pipefail

: "${GAMEHOST_BACKEND_BIN:?GAMEHOST_BACKEND_BIN is required}"
: "${GAMEHOST_BACKEND_PID_FILE:?GAMEHOST_BACKEND_PID_FILE is required}"
: "${GAMEHOST_BACKEND_LOG:?GAMEHOST_BACKEND_LOG is required}"

action="${1:-restart}"
server_root="${GAMEHOST_SERVER_ROOT:-http://127.0.0.1:18899}"
health_url="${server_root%/}/api/public/health"
backend_cwd="${GAMEHOST_BACKEND_CWD:-$(pwd)}"

is_running() {
  [[ -f "$GAMEHOST_BACKEND_PID_FILE" ]] || return 1
  local pid
  pid="$(cat "$GAMEHOST_BACKEND_PID_FILE" 2>/dev/null || true)"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  kill -0 "$pid" 2>/dev/null
}

stop_backend() {
  if ! is_running; then
    rm -f "$GAMEHOST_BACKEND_PID_FILE"
    return 0
  fi
  local pid
  pid="$(cat "$GAMEHOST_BACKEND_PID_FILE")"
  # The E2E restart leg deliberately models abrupt host loss. Persisted desired
  # runtime state must recover and orphaned plugin processes must be eliminated.
  kill -KILL "$pid" 2>/dev/null || true
  for _ in $(seq 1 100); do
    if ! kill -0 "$pid" 2>/dev/null; then
      break
    fi
    sleep 0.1
  done
  rm -f "$GAMEHOST_BACKEND_PID_FILE"
}

start_backend() {
  if [[ ! -x "$GAMEHOST_BACKEND_BIN" ]]; then
    echo "backend binary is not executable: $GAMEHOST_BACKEND_BIN" >&2
    exit 2
  fi
  mkdir -p "$(dirname "$GAMEHOST_BACKEND_PID_FILE")" "$(dirname "$GAMEHOST_BACKEND_LOG")"
  (
    cd "$backend_cwd"
    nohup "$GAMEHOST_BACKEND_BIN" --runtime-profile=local >>"$GAMEHOST_BACKEND_LOG" 2>&1 &
    echo $! >"$GAMEHOST_BACKEND_PID_FILE"
  )
  local new_pid
  new_pid="$(cat "$GAMEHOST_BACKEND_PID_FILE")"
  for _ in $(seq 1 180); do
    if ! kill -0 "$new_pid" 2>/dev/null; then
      echo "GameHost backend exited before becoming healthy" >&2
      tail -n 200 "$GAMEHOST_BACKEND_LOG" >&2 || true
      exit 1
    fi
    if curl --fail --silent --show-error --max-time 2 "$health_url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "GameHost backend did not become healthy at $health_url" >&2
  tail -n 200 "$GAMEHOST_BACKEND_LOG" >&2 || true
  exit 1
}

case "$action" in
  start)
    if ! is_running; then start_backend; fi
    ;;
  restart)
    stop_backend
    start_backend
    ;;
  stop)
    stop_backend
    ;;
  status)
    if is_running; then
      echo "running $(cat "$GAMEHOST_BACKEND_PID_FILE")"
    else
      echo "stopped"
      exit 1
    fi
    ;;
  *)
    echo "usage: $0 {start|restart|stop|status}" >&2
    exit 2
    ;;
esac
