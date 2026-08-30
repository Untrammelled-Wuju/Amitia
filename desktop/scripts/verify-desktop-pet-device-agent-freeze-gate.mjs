import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, "../..");
const read = (relativePath) => fs.readFile(path.join(repoRoot, relativePath), "utf8");

function assert(condition, message) {
  if (!condition) {
    throw new Error(`[verify-desktop-pet-device-agent-freeze-gate] ${message}`);
  }
}

const [
  runtimeComponents,
  main,
  deviceAgentRouter,
  services,
  behaviorMesh,
  meshRuntime,
  meshHub,
  localHandler,
  router,
  frontendRuntimeAdapter,
  releasePublisher,
  releaseOutboxDispatcher,
  animationEngine,
  scheduler,
  playerBridge,
  manager,
  chatStateBridge,
  playerStateMachine,
  doctor,
  legacyPlaybackBridge,
  runtimeActionPort,
  runtimeHandler,
  runtimeProtocolEvent,
  runtimeActualState,
  mobileRuntime,
  androidRenderer,
  runtimeBehaviorMigration,
  migrationBaseline,
] = await Promise.all([
  read("backend/cmd/server/runtime_components.go"),
  read("backend/cmd/server/main.go"),
  read("backend/cmd/server/device_agent_router.go"),
  read("backend/cmd/server/services.go"),
  read("backend/cmd/server/desktop_pet_behavior_mesh.go"),
  read("backend/internal/devicemesh/runtime.go"),
  read("backend/internal/devicemesh/server/connection_hub.go"),
  read("backend/internal/devicemesh/agent/local_handler.go"),
  read("backend/cmd/server/router.go"),
  read("front/src/runtime/runtime-adapter.ts"),
  read("backend/internal/desktoppet/release/recovery.go"),
  read("backend/internal/desktoppet/release/worker/outbox_dispatcher.go"),
  read("desktop/src/desktop-pet/animation/animation-engine.ts"),
  read("desktop/src/main/pet/action-scheduler.ts"),
  read("desktop/src/main/pet/animation-player-bridge.ts"),
  read("desktop/src/main/pet/manager.ts"),
  read("desktop/src/main/pet/chat-state-bridge.ts"),
  read("desktop/src/desktop-pet/animation/player-state-machine.ts"),
  read("backend/internal/desktoppet/doctor/doctor.go"),
  read("desktop/src/desktop-pet/runtime/playback-event-bridge.ts"),
  read("backend/internal/desktoppet/behavior/wiring/runtime_v2_action_port.go"),
  read("backend/internal/desktoppet/runtime/protocol/v2/handler.go"),
  read("backend/internal/desktoppet/runtime/protocol/v2/event.go"),
  read("backend/internal/desktoppet/runtime/protocol/v2/actual_state.go"),
  read("mobile_app/lib/features/desktop_pet/runtime/desktop_pet_mobile_runtime.dart"),
  read("mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider/desktoppet/DesktopPetRendererNativeHandler.kt"),
  read("backend/internal/migration/desktop_pet_runtime_behavior_finalization.go"),
  read("backend/internal/migration/baseline.sql"),
]);

// Device Agent must host the complete desktop-pet runtime body and fail closed.
assert(
  runtimeComponents.includes("profilesDeviceLocal") &&
    runtimeComponents.includes("Required: config.AppCfg.DesktopPetRuntime.Enabled"),
  "DesktopPet component must be required when enabled and available in local/device-agent profiles",
);
for (const required of [
  "InstallationProjectionBridge.Start(ctx)",
  "InstallationDesiredOutbox.Start(ctx)",
  "InstallationRecoveryWorker.Start(ctx)",
  "ReleaseRecoveryWorker.Start(ctx)",
  "ReleaseEventOutboxDispatcher.Start(ctx)",
  "BehaviorService.Start(ctx)",
  "DesktopPetRuntimeV2.Start(ctx)",
  "RuntimeDomainEventConsumer.Start(ctx)",
]) {
  assert(runtimeComponents.includes(required), `DesktopPet component start topology missing ${required}`);
}
for (const readiness of [
  "generation worker not running",
  "processing worker not running",
  "quality worker not running",
  "regeneration worker not running",
  "revision bridge recovery worker not running",
  "installation projection bridge not running",
  "installation desired outbox worker not running",
  "installation recovery worker not running",
  "release recovery worker not running",
  "release event outbox dispatcher not running",
  "behavior service not running",
  "runtime v2 not running",
  "runtime domain event consumer not running",
]) {
  assert(
    runtimeComponents.includes(readiness) && deviceAgentRouter.includes(readiness),
    `device-agent /readyz and component Ready() must both enforce ${readiness}`,
  );
}
assert(
  !main.includes("InstallationProjectionBridge.Start(ctx)") &&
    !main.includes("InstallationDesiredOutbox.Start(ctx)") &&
    !main.includes("InstallationRecoveryWorker.Start(ctx)"),
  "installation reliability workers must not also be started by core workers",
);

// Cloud lifecycle events must cross Device Mesh and resolve canonical local ownership.
assert(
  behaviorMesh.includes('desktopPetBehaviorMeshHandler') &&
    behaviorMesh.includes('"desktop_pet.behavior.submit_event"') &&
    behaviorMesh.includes("desktopPetOwnerMapper") &&
    behaviorMesh.includes("Resolve(") &&
    behaviorMesh.includes("InvokeDeviceHandlerWithRuntimeType") &&
    services.includes("newDesktopPetBehaviorMeshPublisher") &&
    services.includes("localRuntimeDispatcher.RegisterCancellable(desktopPetBehaviorMeshHandler"),
  "CloudCore -> DeviceMesh -> DeviceAgent behavior bridge with owner mapping is incomplete",
);
assert(
  meshRuntime.includes("InvokeDeviceHandlerWithRuntimeType") &&
    meshHub.includes("ListByUser") &&
    localHandler.includes("SetCredentialObserver") &&
    !behaviorMesh.includes("GetPreferredByUser"),
  "Desktop Pet behavior routing must enumerate authenticated user devices and must never select by freshest heartbeat",
);
assert(
  runtimeBehaviorMigration.includes('"target_installation_id"') ||
    runtimeBehaviorMigration.includes("target_installation_id TEXT NOT NULL DEFAULT ''"),
  "Cloud behavior outbox migration must persist target_installation_id execution fence",
);
assert(
  migrationBaseline.includes("target_installation_id TEXT NOT NULL DEFAULT ''") &&
    migrationBaseline.includes("position_x INTEGER NOT NULL DEFAULT 0") &&
    migrationBaseline.includes("window_width INTEGER NOT NULL DEFAULT 0") &&
    migrationBaseline.includes("scale REAL NOT NULL DEFAULT 0"),
  "baseline schema must include behavior target fencing and Runtime V2 physical-state columns",
);

assert(
  behaviorMesh.includes("desktopPetBehaviorMeshAffinity") &&
    behaviorMesh.includes("desktopPetBehaviorMeshOutbox") &&
    behaviorMesh.includes("processOutboxRow") &&
    behaviorMesh.includes("claim_expires_at") &&
    !behaviorMesh.includes('claim_expires_at;index') &&
    !behaviorMesh.includes('available_at;not null;index') &&
    behaviorMesh.includes('"processing"') &&
    behaviorMesh.includes("RowsAffected == 0") &&
    behaviorMesh.includes("pin.RowsAffected != 1") &&
    behaviorMesh.includes('"target_installation_id"') &&
    behaviorMesh.includes("outbox claim lost before target fence persistence") &&
    behaviorMesh.includes("resolveOnDevice") &&
    behaviorMesh.includes("desktopPetBehaviorMeshResolveHandler") &&
    behaviorMesh.includes("GetActiveBindingForUserDeviceTx") &&
    behaviorMesh.includes("target device %s is offline") &&
    behaviorMesh.includes("concrete outbox target is an execution fence") &&
    behaviorMesh.includes("pinned target device %s no longer owns the active installation"),
  "Cloud behavior delivery must persist recoverable/durable events, preserve verified character/device affinity, and never reroute an already-targeted event",
);
assert(
  services.includes("DesktopPetBehaviorMesh") &&
    services.includes("localRuntimeDispatcher.RegisterCancellable(desktopPetBehaviorMeshResolveHandler") &&
    runtimeComponents.includes("desktopPetBehaviorMeshComponent") &&
    runtimeComponents.includes("DesktopPetBehaviorMesh.Start(ctx)"),
  "Cloud behavior outbox worker and device affinity resolver must be part of managed runtime topology",
);

// Device-local authority and authentication must be explicit.
assert(
  deviceAgentRouter.includes("RequireAuthMethod(security.AuthMethodDesktopSession)") &&
    frontendRuntimeAdapter.includes("DESKTOP_PET_DEVICE_LOCAL_ROUTE_PREFIXES") &&
    frontendRuntimeAdapter.includes('"/api/desktop-pet"') &&
    frontendRuntimeAdapter.includes('"/api/desktop-pets"'),
  "desktop-pet device-local routes must use Desktop Session auth and include singular/plural namespaces",
);
assert(
  router.includes("if services.RuntimePolicy.DesktopPet") &&
    router.includes("behavior.RegisterRoutes(desktopPetWriteGroup"),
  "Cloud Core must not mount Device-local desktop-pet mutation routes",
);

// Runtime V2 desired state must fail closed on Android when the canonical
// window cannot be represented. Silent platform clamping creates false
// desiredHash convergence and is forbidden.
assert(
  mobileRuntime.includes("DESIRED_SETTINGS_UNSUPPORTED") &&
    mobileRuntime.includes("DESIRED_SETTINGS_NOT_APPLIED") &&
    mobileRuntime.includes("appliedWidth != width ||") &&
    mobileRuntime.includes("appliedX != x ||") &&
    mobileRuntime.includes("alwaysOnTop != 1") &&
    mobileRuntime.includes("clickThroughMode != 'off'") &&
    mobileRuntime.includes("soundEnabled != 0") &&
    mobileRuntime.includes("positionMode != 'absolute'") &&
    !mobileRuntime.includes("_double(settings['scale'], 1.0).clamp(0.25, 4.0)") &&
    !mobileRuntime.includes("(canvasWidth * scale).round().clamp(64, 420)"),
  "Android Runtime V2 must reject unsupported canonical desired settings instead of silently projecting/clamping and ACKing them",
);
assert(
  androidRenderer.includes("DESKTOP_PET_SIZE_UNSUPPORTED") &&
    androidRenderer.includes("requestedWidth !in MIN_PET_DP..MAX_PET_DP") &&
    androidRenderer.includes("requestedHeight !in MIN_PET_DP..MAX_PET_DP") &&
    !androidRenderer.includes('intOrNull("width")?.let { params.width = dp(it.coerceIn(MIN_PET_DP, MAX_PET_DP)) }'),
  "Android native renderer must fail closed for unsupported Runtime V2 window dimensions",
);
assert(
  runtimeProtocolEvent.includes('json:"positionX"') &&
    runtimeProtocolEvent.includes('json:"windowWidth"') &&
    runtimeProtocolEvent.includes('json:"scale"') &&
    runtimeActualState.includes('gorm:"column:position_x') &&
    runtimeActualState.includes('gorm:"column:window_width') &&
    runtimeHandler.includes("snapshot.Visible != (windowStatus == WindowStatusVisible)"),
  "Runtime V2 actual-state persistence must include physical window geometry and reject contradictory visibility facts",
);
assert(
  manager.includes("incomingRuntimeId") &&
    manager.includes('errorCode: "MISSING_RUNTIME_ID"') &&
    manager.includes('errorCode: "RUNTIME_MISMATCH"') &&
    runtimeActionPort.includes("targetRuntimeID := string(targetConn.RuntimeID)") &&
    runtimeActionPort.includes("payload.RuntimeID = targetRuntimeID") &&
    runtimeActionPort.includes("payload.PetInstanceID = targetRuntimeID") &&
    runtimeActionPort.indexOf("payload.RuntimeID = targetRuntimeID") < runtimeActionPort.indexOf("json.Marshal(payload)") &&
    scheduler.includes("runtimeInterruptible") &&
    scheduler.includes("effectiveAction") &&
    mobileRuntime.includes("commandInterruptible") &&
    androidRenderer.includes("requestedInterruptible"),
  "play_action runtime identity and command-level interruptibility must be enforced consistently from backend routing through Electron and Android",
);

// Local mode has exactly one production behavior authority. The chat-state
// bridge may mirror presentation state but must not schedule actions itself.
assert(
  !chatStateBridge.includes("DesktopPetActionScheduler") &&
    !chatStateBridge.includes("scheduler.submit(") &&
    !chatStateBridge.includes("setSustainedState(") &&
    chatStateBridge.includes("Backend Behavior owns action scheduling"),
  "frontend chat-state IPC must not become a second desktop-pet action authority",
);

// Doctor must report only checks that actually ran. Historical zero-valued
// freeze counters without producers are forbidden because they create false green.
for (const removedField of [
  "AuthFailOpenCount",
  "UnscopedHandlerCount",
  "UnsafePathWriteCount",
  "UnsafeDeleteCount",
  "LegacyWriterCount",
  "UnresolvedConflictCount",
  "BlockingJournalCount",
  "RequiredWorkerDownCount",
  "ContractMismatchCount",
  "GofmtViolationCount",
]) {
  assert(!doctor.includes(removedField), `Doctor must not expose unproduced freeze field ${removedField}`);
}
assert(
  doctor.includes("ExecutedCheckers") &&
    doctor.includes("report.BlockingFindings++") &&
    doctor.includes('report.Status = "blocked"'),
  "Doctor must expose executed checkers and fail closed when a checker errors",
);

// Release events have one durable production chain, with retry rather than in-memory buffering.
assert(
  releasePublisher.includes("CreateEventOutbox(outbox)") &&
    !releasePublisher.includes("events []ReleaseEvent") &&
    !releasePublisher.includes("func (o *ReleaseEventPublisher) Flush"),
  "release events must be persisted through the durable outbox only",
);
assert(
  releaseOutboxDispatcher.includes('event.Status = "pending"') &&
    releaseOutboxDispatcher.includes("exponentialBackoff") &&
    releaseOutboxDispatcher.includes("IsRunning() bool"),
  "release outbox dispatcher must retry failed delivery and expose readiness",
);

// Playback identity and exactly-one terminal semantics must remain frozen.
for (const invariant of [
  "pendingPlaybackInstanceId",
  "acceptedCommands",
  "finalizedIds",
  "promoteQueuedCommand",
  "surface.present(firstFrame",
  "startTickLoop()",
]) {
  assert(animationEngine.includes(invariant), `animation engine invariant missing: ${invariant}`);
}
assert(
  playerStateMachine.includes("startMonotonicMs: -1") &&
    playerStateMachine.includes("startedAtMonotonicMs: action.now") &&
    animationEngine.includes("private tickLoopEpoch = 0") &&
    animationEngine.includes("epoch !== this.tickLoopEpoch") &&
    animationEngine.includes('type: "INTERNAL_ACTION_REQUESTED"') &&
    animationEngine.includes('type: "INTERNAL_ACTION_LOADED"'),
  "playback timebase, tick generation reset, and return/default state transitions must remain explicit",
);
assert(
  scheduler.includes('returnTo === "default"') &&
    scheduler.includes('{ type: "default" as const }') &&
    playerBridge.includes("context?.returnOverride ?? buildReturnTarget(action)"),
  "explicit Runtime returnTo=default must override package return policy",
);
assert(
  manager.includes('this.scheduler.forceInterrupt("app_exit")') &&
    manager.includes('this.actionPlayer?.stop("window_destroyed")') &&
    !manager.includes("this.actionPlayer?.stop();"),
  "shutdown must preserve window_destroyed rather than emitting a second user_disabled stop",
);

assert(
  scheduler.includes('readMetadataNumber(request, "minimumPlayMs")') &&
    scheduler.includes('readMetadataNumber(request, "interruptAfterMs")') &&
    scheduler.includes("if (!hasExplicitTiming) return DEFAULT_MIN_PLAY_DURATION_MS") &&
    !scheduler.includes("Math.max(DEFAULT_MIN_PLAY_DURATION_MS"),
  "explicit timing contracts, including 0ms, must not be overwritten by the 300ms fallback",
);

assert(
  scheduler.includes("matchesCurrent(actionKey, playbackInstanceId, commandId)") &&
    playerBridge.includes("pendingPlaybackId") &&
    playerBridge.includes("playbackInstanceId") &&
    manager.includes("event.commandId") &&
    manager.includes("playedDurationMs"),
  "scheduler/player/manager playback identity correlation is incomplete",
);
assert(
  manager.includes("RENDERER_DELIVERY_FAILED") &&
    manager.includes("this.actionPlayer.handleSubmissionFailure(command.commandId, reason)") &&
    manager.includes("this.runtimeHandler?.sendRuntimeCommandFailed(") &&
    manager.includes("before command_accepted") &&
    !manager.includes("renderer_delivery_${reason}"),
  "pre-renderer delivery failures must terminalize through Runtime ACK without fabricating playback events",
);
assert(
  playerBridge.includes("STOP_DELIVERY_FAILED") &&
    manager.includes("handlePlayerBridgeFailure") &&
    manager.includes("PLAYER_BRIDGE_DELIVERY_FAILED"),
  "stop delivery failure must terminalize the active accepted Runtime v2 play command",
);
assert(
  scheduler.includes('"action-cancelled"') &&
    manager.includes('event === "action-cancelled"') &&
    manager.includes('"PLAYBACK_COMMAND_CANCELLED"'),
  "scheduler queue eviction/coalesce must terminalize accepted Runtime v2 commands before playback identity exists",
);
assert(
  legacyPlaybackBridge.includes("test-only") &&
    legacyPlaybackBridge.includes('process.env.NODE_ENV !== "test"'),
  "second playback reporting bridge must fail closed outside tests",
);
assert(
  runtimeHandler.includes("PlayActionCompletionOnStarted") &&
    runtimeHandler.includes("PlayActionCompletionOnFirstCycle") &&
    runtimeHandler.includes("PlayActionCompletionOnInterrupted") &&
    runtimeHandler.includes("PlayActionCompletionManualStop") &&
    runtimeHandler.includes("InterruptReason") &&
    runtimeHandler.includes("meta.Reason = meta.InterruptReason") &&
    runtimeHandler.includes("MarkCancelled(meta.CommandID"),
  "Runtime v2 completionPolicy and interrupt reason parsing must execute in the command state machine",
);

console.log(
  "[verify-desktop-pet-device-agent-freeze-gate] PASSED: device-agent topology, affinity-routed durable cloud behavior delivery, Android desired-state truth, owner/auth authority, durable release events, and playback terminal invariants are frozen",
);
