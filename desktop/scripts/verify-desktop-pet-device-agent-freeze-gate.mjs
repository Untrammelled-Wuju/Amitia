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
  runtimeHandler,
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
  read("backend/internal/desktoppet/runtime/protocol/v2/handler.go"),
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
  behaviorMesh.includes('desktopPetBehaviorMeshHandler = "desktop_pet.behavior.submit_event"') &&
    behaviorMesh.includes("desktopPetOwnerMapper") &&
    behaviorMesh.includes("Resolve(") &&
    behaviorMesh.includes("InvokeDeviceHandlerWithRuntimeType") &&
    services.includes("newDesktopPetBehaviorMeshPublisher") &&
    services.includes("localRuntimeDispatcher.RegisterCancellable(desktopPetBehaviorMeshHandler"),
  "CloudCore -> DeviceMesh -> DeviceAgent behavior bridge with owner mapping is incomplete",
);
assert(
  meshRuntime.includes("InvokeDeviceHandlerWithRuntimeType") &&
    meshHub.includes("GetPreferredByUser") &&
    localHandler.includes("SetCredentialObserver"),
  "Device Mesh behavior routing must use deterministic device selection and credential-bound owner mapping",
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
    manager.includes("this.handlePlaybackEvent({") &&
    manager.includes("renderer_delivery_${reason}"),
  "main-process renderer delivery failures must terminalize accepted Runtime v2 commands",
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
  "[verify-desktop-pet-device-agent-freeze-gate] PASSED: device-agent topology, cloud behavior mesh, owner/auth authority, durable release events, and playback terminal invariants are frozen",
);
