#!/usr/bin/env python3
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
FAILURES: list[str] = []


def require(path: str, *needles: str) -> None:
    target = ROOT / path
    if not target.is_file():
        FAILURES.append(f"missing required file: {path}")
        return
    text = target.read_text(encoding="utf-8", errors="replace")
    for needle in needles:
        if needle not in text:
            FAILURES.append(f"{path}: missing required marker {needle!r}")


def forbid(path: str, *patterns: str) -> None:
    target = ROOT / path
    if not target.is_file():
        return
    text = target.read_text(encoding="utf-8", errors="replace")
    for pattern in patterns:
        if re.search(pattern, text, flags=re.MULTILINE):
            FAILURES.append(f"{path}: forbidden production pattern matched {pattern!r}")


def main() -> int:
    require(
        "mobile_app/android/amitia-runtime/src/main/kotlin/com/amitia/amitia_app/runtime/recovery/RuntimeDesiredStateStore.kt",
        "recoveryAttempt", "nextRecoveryAt", "recoveryToken", "recoveryExhausted", "markRecoveryExhausted", "bootGeneration",
    )
    require(
        "mobile_app/android/amitia-runtime/src/test/kotlin/com/amitia/amitia_app/runtime/recovery/RuntimeCrashRecoveryPolicyTest.kt",
        "processRecreation_restoresPersistentBudgetAndFingerprint", "markRecoveryExhausted", "recoveryExhausted",
    )
    require(
        "mobile_app/android/amitia-runtime/src/main/kotlin/com/amitia/amitia_app/runtime/recovery/RuntimeRecoveryJobService.kt",
        "JobService", "recoveryToken", "failedGeneration",
    )
    require(
        "mobile_app/android/amitia-runtime/src/main/kotlin/com/amitia/amitia_app/runtime/recovery/RuntimeRecoveryScheduler.kt",
        "PersistentRuntimeRecoveryScheduler", "setPersisted(true)", "ensureScheduledFromStore",
    )
    require(
        "mobile_app/android/amitia-runtime/src/main/kotlin/com/amitia/amitia_app/runtime/AndroidRuntimeModule.kt",
        "AndroidRuntimeDesiredStateStore", "PersistentRuntimeRecoveryScheduler",
    )
    require(
        "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider/accessibility/AccessibilityHealthMonitor.kt",
        "lastConnectedAt", "lastDisconnectAt", "generation",
    )
    require(
        "mobile_app/android/amitia-runtime/src/main/kotlin/com/amitia/amitia_app/runtime/workflow/WorkflowDeviceEventJournal.kt",
        "fingerprint", "ack", "sourceSequence",
    )
    require(
        "backend/internal/extension/workflow_local_kws_backend.go",
        "workflow_local_kws", "LocalOnly", "keywords-file", "16000",
    )
    require(
        "backend/internal/extension/workflow_android_runtime_health.go",
        "runtimeReady", "nativeBridgeReady", "accessibilityReady", "uiAgentReady",
        "backgroundRestricted", "InteractionState", "workflowAndroidHealthStaleAfter",
        "RecoveryExhausted", "MetricRuntimeCrashTotal", "MetricRuntimeRecoveryTotal", "MetricAndroidAccessibilityDisconnect",
    )
    require(
        "backend/internal/extension/workflow_android_runtime_health_test.go",
        "EmitsRecoveryAndAccessibilityMetrics", "MetricRuntimeRecoveryExhaustedTotal",
    )
    require(
        "backend/internal/extension/workflow_preflight.go",
        "WorkflowMeshAndroidRuntimeHealth", "workflowPreflightDeviceIsAndroid",
        "WAITING_UNLOCK", "WAITING_SCREEN", "device health heartbeat is stale",
        "automatic recovery is exhausted", "workflowNodeExplicitlyRequiresAndroid",
    )
    require(
        "backend/internal/extension/workflow_device_mesh.go",
        "WorkflowMeshAndroidRuntimeHealth", "api.meshAndroidRuntimeHealth",
    )
    require(
        "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/workflow/WorkflowTriggerCapabilityReporter.kt",
        "HEALTH_HEARTBEAT_MS", "DeviceInteractionStateReader", "uiAgentReady",
        "reportAndroidRuntimeHealth", "AndroidRuntimeDesiredStateStore", "recoveryExhausted",
    )
    require(
        "mobile_app/android/amitia-runtime/src/main/kotlin/com/amitia/amitia_app/runtime/workflow/WorkflowWakeAudioMonitor.kt",
        "wake_active", "wake_suspended", "wake_blocked_by_android", "wake_permission_missing",
    )
    require(
        "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider/AndroidNativeHost.kt",
        "handlers.keys.associateWith { true }", "capabilityCache.get()",
    )
    require(
        "backend/internal/agent/android_ui_reliability.go",
        "LOW_INFORMATION", "semanticRematchTarget", "visualFallbackAction",
    )
    require(
        "front/src/views/creative-workshop/workflows/WorkflowBuilderView.vue",
        "简单模式", "高级模式", "每天执行时间", "simple-expression-builder", "预检",
        "默认使用本地 KWS",
    )
    require(
        "mobile_app/lib/features/workshop/presentation/pages/workflow_editor_page.dart",
        "每天执行时间", "_toolSchemaFields", "用简单条件替换", "工作流预检",
    )
    require(
        "backend/internal/extension/kernel/workflow/reliability_metrics.go",
        "workflow_trigger_total", "runtime_crash_total", "ui_agent_loop_total", "wake_detection_total", "P95", "P99",
    )
    require("scripts/test/workflow-reliability-matrix.json", "androidApiLevels", "runtimeChaos", "releaseSeconds")

    forbid(
        "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/workflow/WorkflowManifestEventReceiver.kt",
        r"getSharedPreferences\(\s*\"amitia_runtime_desired_state\"",
    )
    forbid(
        "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/workflow/WorkflowAutomationHealthJobService.kt",
        r"getSharedPreferences\(\s*\"amitia_runtime_desired_state\"",
    )
    forbid(
        "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider/AndroidNativeHost.kt",
        r"capabilityCache\.get\(\)\?\.let",
    )

    generated_artifacts = [
        path.relative_to(ROOT)
        for path in ROOT.rglob("*")
        if path.is_file() and ("__pycache__" in path.parts or path.suffix == ".pyc")
    ]
    if generated_artifacts:
        FAILURES.append("generated Python cache artifacts must not be packaged: " + ", ".join(map(str, generated_artifacts)))

    matrix_path = ROOT / "scripts/test/workflow-reliability-matrix.json"
    if matrix_path.is_file():
        try:
            matrix = json.loads(matrix_path.read_text(encoding="utf-8"))
            if matrix.get("androidApiLevels") != [26, 29, 31, 33, 35, 36]:
                FAILURES.append("reliability matrix must cover API 26/29/31/33/35/36")
            if int(matrix.get("soak", {}).get("qualificationSeconds", 0)) < 72 * 3600:
                FAILURES.append("qualification soak must be at least 72 hours")
            if int(matrix.get("soak", {}).get("releaseSeconds", 0)) < 7 * 24 * 3600:
                FAILURES.append("release soak must be at least 7 days")
        except Exception as exc:
            FAILURES.append(f"invalid reliability matrix: {exc}")

    if FAILURES:
        print("workflow hardening static gate FAILED", file=sys.stderr)
        for failure in FAILURES:
            print(f" - {failure}", file=sys.stderr)
        return 1
    print("workflow hardening static gate PASSED")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
