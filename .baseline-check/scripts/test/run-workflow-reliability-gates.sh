#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="${1:-quick}"
cd "$ROOT"

fail() { echo "workflow reliability gate FAILED: $*" >&2; exit 1; }
require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }

run_static() {
  require_cmd python3
  python3 scripts/test/verify-workflow-hardening.py
  test -f backend/go.mod || fail "backend/go.mod missing"
  local go_files=(
    internal/agent/android_ui_agent.go
    internal/agent/android_ui_reliability.go
    internal/agent/android_ui_reliability_test.go
    internal/androiduiagent/bridge.go
    internal/extension/runtime.go
    internal/extension/workflow_android_runtime_health.go
    internal/extension/workflow_android_runtime_health_test.go
    internal/extension/workflow_api.go
    internal/extension/workflow_device_control.go
    internal/extension/workflow_device_mesh.go
    internal/extension/workflow_error_taxonomy.go
    internal/extension/workflow_local_kws_backend.go
    internal/extension/workflow_local_kws_backend_test.go
    internal/extension/workflow_preflight.go
    internal/extension/workflow_wake_runtime.go
    internal/extension/kernel/android_ui_agent_tool.go
    internal/extension/kernel/workflow/executor.go
    internal/extension/kernel/workflow/reliability_metrics.go
    internal/extension/kernel/workflow/reliability_metrics_test.go
    internal/extension/kernel/workflow/trigger.go
  )
  (cd backend && GOTOOLCHAIN=local gofmt -l "${go_files[@]}" | tee /tmp/amitia-workflow-gofmt.txt)
  test ! -s /tmp/amitia-workflow-gofmt.txt || fail "modified Go files are not gofmt-clean"
}

run_go() {
  require_cmd go
  local version
  version="$(go env GOVERSION 2>/dev/null || true)"
  if [[ ! "$version" =~ ^go1\.([0-9]+)(\.|$) ]] || (( 10#${BASH_REMATCH[1]} < 26 )); then
    fail "Go 1.26.x or newer required by backend/go.mod; found ${version:-unknown}"
  fi
  (cd backend && GOTOOLCHAIN=local go test ./internal/extension/kernel/workflow ./internal/agent ./internal/extension -run 'Test(WorkflowReliabilityMetricsSnapshot|AndroidUI|WorkflowLocalKWS|WorkflowAndroidRuntimeHealth|SetWorkflowAndroidRuntimeHealth|UpdateWorkflowWakeDeviceStatus)' -count=1)
}

run_android() {
  local wrapper="mobile_app/android/gradle/wrapper/gradle-wrapper.jar"
  [[ -f "$wrapper" ]] || fail "$wrapper missing; this source package cannot execute Gradle Android gates"
  [[ -x mobile_app/android/gradlew ]] || chmod +x mobile_app/android/gradlew
  (cd mobile_app/android && ./gradlew :amitia-runtime:testDebugUnitTest :app:testDebugUnitTest --stacktrace)
}

run_physical() {
  require_cmd adb
  local devices
  devices="$(adb devices | awk 'NR>1 && $2=="device" {count++} END {print count+0}')"
  [[ "$devices" -gt 0 ]] || fail "no authorized physical Android device attached"
  [[ -n "${AMITIA_WORKFLOW_PHYSICAL_E2E_CMD:-}" ]] || fail "set AMITIA_WORKFLOW_PHYSICAL_E2E_CMD to the lab physical-device E2E runner; gate will not fake a pass"
  bash -lc "$AMITIA_WORKFLOW_PHYSICAL_E2E_CMD"
}

run_soak() {
  local required_seconds="$1"
  [[ -n "${AMITIA_WORKFLOW_SOAK_PROBE_CMD:-}" ]] || fail "set AMITIA_WORKFLOW_SOAK_PROBE_CMD"
  [[ -n "${AMITIA_WORKFLOW_METRICS_URL:-}" ]] || fail "set AMITIA_WORKFLOW_METRICS_URL to /workflows/reliability-metrics"
  require_cmd curl
  local seconds="$required_seconds"
  if [[ "${AMITIA_GATE_TEST_OVERRIDE:-0}" == "1" ]]; then
    seconds="${AMITIA_WORKFLOW_SOAK_DURATION_SECONDS:-$required_seconds}"
  fi
  local started now next_chaos
  started="$(date +%s)"; next_chaos="$started"
  while true; do
    now="$(date +%s)"
    (( now - started >= seconds )) && break
    bash -lc "$AMITIA_WORKFLOW_SOAK_PROBE_CMD"
    curl --fail --silent --show-error "$AMITIA_WORKFLOW_METRICS_URL" >/tmp/amitia-workflow-reliability-metrics.json
    python3 - <<'PY'
import json
p='/tmp/amitia-workflow-reliability-metrics.json'
data=json.load(open(p,encoding='utf-8'))
required=json.load(open('scripts/test/workflow-reliability-matrix.json',encoding='utf-8'))['requiredMetrics']
counters=data.get('counters',{})
missing=[name for name in required if name not in counters]
if missing: raise SystemExit('missing reliability metrics: '+', '.join(missing))
PY
    if [[ -n "${AMITIA_WORKFLOW_CHAOS_CMD:-}" && "$now" -ge "$next_chaos" ]]; then
      bash -lc "$AMITIA_WORKFLOW_CHAOS_CMD"
      next_chaos=$((now + 300))
    fi
    sleep "${AMITIA_WORKFLOW_SOAK_POLL_SECONDS:-60}"
  done
}

case "$MODE" in
  static) run_static ;;
  quick) run_static; run_go ;;
  android) run_static; run_go; run_android ;;
  physical) run_static; run_go; run_android; run_physical ;;
  soak72h) run_static; run_go; run_android; run_physical; run_soak 259200 ;;
  release7d) run_static; run_go; run_android; run_physical; run_soak 604800 ;;
  *) fail "unknown mode '$MODE' (static|quick|android|physical|soak72h|release7d)" ;;
esac

echo "workflow reliability gate PASSED: $MODE"
