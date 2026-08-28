import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, "../..");

async function read(relativePath) {
  return fs.readFile(path.join(repoRoot, relativePath), "utf8");
}

async function collectFiles(rootRelative, extensions) {
  const root = path.join(repoRoot, rootRelative);
  const result = [];
  const visit = async (directory) => {
    const entries = await fs.readdir(directory, { withFileTypes: true });
    for (const entry of entries) {
      if (entry.name === "node_modules" || entry.name === "dist" || entry.name === "__tests__") continue;
      const full = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        await visit(full);
      } else if (entry.isFile() && extensions.some((ext) => entry.name.endsWith(ext))) {
        result.push(full);
      }
    }
  };
  await visit(root);
  return result;
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(`[verify-desktop-pet-finalization] ${message}`);
  }
}

const manager = await read("desktop/src/main/pet/manager.ts");
const deploymentLifecycle = await read("desktop/src/main/deployment-lifecycle.ts");
const backendModel = await read("backend/internal/desktoppet/installation/model.go");
const backendDto = await read("backend/internal/desktoppet/installation/dto.go");
const configStore = await read("desktop/src/main/config-store.ts");
const desktopIndex = await read("desktop/src/main/index.ts");
const ipcHandlers = await read("desktop/src/main/ipc-handlers.ts");
const tray = await read("desktop/src/main/tray.ts");
const runtimeCommandService = await read("backend/internal/desktoppet/runtime/protocol/v2/command_service.go");
const runtimeDispatcher = await read("backend/internal/desktoppet/runtime/protocol/v2/command_dispatcher.go");
const runtimeRouter = await read("backend/internal/desktoppet/runtime/protocol/v2/router.go");
const runtimeReconciler = await read("backend/internal/desktoppet/runtime/protocol/v2/reconciler.go");
const runtimeHandler = await read("desktop/src/desktop-pet/runtime/runtime-handler-v2.ts");
const desiredStateForwardFix = await read("backend/internal/migration/desktop_pet_desired_state_schema_forward_fix.go");
const migrations = await read("backend/internal/migration/migrations.go");
const migrationBaseline = await read("backend/internal/migration/baseline.sql");
const behaviorReducer = await read("backend/internal/desktoppet/behavior/reducer.go");
const behaviorSchema = await read("backend/internal/desktoppet/behavior/schema.go");
const behaviorPlaybackAdapter = await read("backend/internal/desktoppet/behavior/adapters/playback.go");
const behaviorEnvelope = await read("backend/internal/desktoppet/behavior/events/envelope.go");
const serverServices = await read("backend/cmd/server/services.go");
const runtimeComponents = await read("backend/cmd/server/runtime_components.go");
const runtimePolicy = await read("backend/internal/runtimeprofile/policy.go");
const deviceAgentRouter = await read("backend/cmd/server/device_agent_router.go");
const securityCors = await read("backend/internal/middleware/security/cors.go");
const securityCorsTests = await read("backend/internal/middleware/security/cors_test.go");
const characterWatcher = await read("desktop/src/main/pet/character-watcher.ts");
const managerTests = await read("desktop/src/main/pet/__tests__/manager.test.ts");
const characterWatcherTests = await read("desktop/src/main/pet/__tests__/character-watcher.test.ts");
const behaviorBindingValidator = await read("backend/internal/desktoppet/behavior/bindings/validator.go");
const frontendRuntimeAdapter = await read("front/src/runtime/runtime-adapter.ts");
const frontendApi = await read("front/src/composables/useApi.ts");
const frontendRequestAuth = await read("front/src/runtime/request-auth.ts");
const authenticatedSSE = await read("front/src/runtime/authenticated-sse.ts");
const generationTask = await read("front/src/composables/useGenerationTask.ts");
const processingTask = await read("front/src/composables/useProcessingTask.ts");
const realtimeCallWidget = await read("front/src/components/RealtimeCallWidget.vue");
const frontendPetChatState = await read("front/src/runtime/desktop-pet-chat-state.ts");
const desktopPreload = await read("desktop/src/preload/index.ts");

assert(
  runtimeDispatcher.includes("ListCommandsToDispatchForConnection(") &&
    !runtimeDispatcher.includes("ListCommandsToDispatch(100)"),
  "Runtime V2 dispatch must be scoped per connected user/device/runtime",
);
assert(
  runtimeCommandService.includes("MarkFailedRetryable") &&
    runtimeCommandService.includes("CommandStatusFailedRetryable") &&
    runtimeReconciler.includes('"ACK_TIMEOUT"') &&
    runtimeReconciler.includes("if cmd.IsDurable() {"),
  "Runtime V2 durable commands must have retryable delivery semantics",
);
assert(
  runtimeCommandService.includes("ReconcileDesiredStateOnHello(") &&
    runtimeCommandService.includes('const desiredTable = "desktop_pet_runtime_desired_states"') &&
    runtimeCommandService.includes("RequeueDurableCommand(existing.ID"),
  "reconnect must reconcile against authoritative persisted desired state",
);
assert(
  runtimeRouter.includes("type outboundFrame struct") &&
    runtimeRouter.includes("frame.result <- err") &&
    runtimeRouter.includes("SetWriteDeadline") &&
    runtimeRouter.includes("ctx.conn.MarkHandshakeAcked()") &&
    runtimeDispatcher.includes("!conn.IsHandshakeAcked()"),
  "transport-dispatched and handshake ordering must be fenced by real websocket writes",
);
assert(
  runtimeHandler.includes("commandReplayCache") &&
    runtimeHandler.includes("replayEntries") &&
    runtimeHandler.includes("getReplayEntries()") &&
    runtimeHandler.includes("isRevisionedDurableCommand") &&
    runtimeHandler.includes("replayCachedCommand") &&
    runtimeHandler.includes("this.lastProcessedCommandSequence = Math.max(this.lastProcessedCommandSequence, sequence)") &&
    !runtimeHandler.includes("commandSequence <= this.lastProcessedCommandSequence"),
  "Runtime V2 client must replay by command identity/revision, never assume a sequence high-water mark proves lower commands executed",
);
assert(
  manager.includes("runtimeCommandReplayEntries") &&
    manager.includes("replayEntries: this.runtimeCommandReplayEntries") &&
    manager.includes("handler.getReplayEntries()"),
  "Electron manager must preserve terminal command replay results across runtime handler replacement",
);
assert(
  runtimeCommandService.includes("desired_revision > ?") &&
    runtimeCommandService.includes("CommandStatusSuperseded") &&
    runtimeCommandService.includes('updates["runtime_id"] = runtimeID') &&
    runtimeCommandService.includes("existing.RuntimeID != runtimeID"),
  "durable recovery must not revive stale desired revisions and must retarget commands to the current runtime incarnation",
);
assert(
  manager.includes("ack.currentDesiredRevision") &&
    !manager.includes("void ack.currentDesiredRevision"),
  "Electron runtime must consume the authoritative desired revision advertised by hello_ack",
);
assert(
  desiredStateForwardFix.includes("desktop_pet_device_desired_revision_counters") &&
    desiredStateForwardFix.includes("MAX(desired_revision)") &&
    desiredStateForwardFix.includes("superseded_by_desired_state_schema_forward_fix") &&
    migrations.includes("DesktopPetDesiredStateSchemaForwardFixMigration()") &&
    migrationBaseline.includes("settings_snapshot_json TEXT NOT NULL DEFAULT ''") &&
    migrationBaseline.includes("uq_dprds_user_device"),
  "desired-state schema migration must preserve revision monotonicity, suppress stale outbox rows, and match the canonical baseline",
);

assert(
  manager.includes("restoreOnAppStart: boolean"),
  "RuntimeSettingsInfo must expose restoreOnAppStart",
);
assert(
  manager.includes("private async applyDesiredStateCommand("),
  "Runtime V2 desired-state convergence helper is missing",
);
assert(
  manager.includes("payload.settingsSnapshot"),
  "sync_desired_state must consume the authoritative settings snapshot",
);
assert(
  manager.includes("this.applyDefaultActionLocal(desiredDefaultActionKey)"),
  "sync_desired_state must apply defaultActionKey locally",
);
assert(
  manager.includes("const forceReload =") && manager.includes("DESIRED_RELEASE_NOT_APPLIED"),
  "sync_desired_state must force/verify release convergence",
);
assert(
  manager.includes("desiredRevision < this.lastAppliedDesiredRevision"),
  "durable desired-state commands must reject stale revision rollback",
);
assert(
  manager.includes('case "runtime.command.sync_desired_state"') &&
    !manager.includes('case "spawn"') &&
    !manager.includes('case "destroy"') &&
    !manager.includes('case "play_action"') &&
    !manager.includes('case "update_settings"') &&
    !manager.includes('case "recenter"') &&
    !manager.includes('case "sync"'),
  "Runtime V2 handler must not retain compatibility command aliases",
);
assert(
  !manager.includes("this.markSettingsRevisionApplied(command?.settingsRevision") &&
    manager.includes("Settings revision is advanced only by applyRuntimeSettingsLocal()"),
  "settings revision must be advanced only after real runtime side effects",
);
assert(
  manager.includes('normalized === CLICK_THROUGH_MODE_FULL') &&
    manager.includes('if (mode === "full") return CLICK_THROUGH_MODE_FULL'),
  "full click-through must round-trip between backend and Electron runtime",
);
assert(
  manager.includes('positionMode: "absolute" | "relative" | "recenter"') &&
    manager.includes("private async applyRuntimePositionModeLocal("),
  "desired settings position modes must have runtime semantics",
);
assert(
  manager.includes("await this.callDisableApi(installation.id)") &&
    manager.includes('"启动恢复已关闭"'),
  "restoreOnAppStart=false must converge backend desired state before bridge startup",
);
assert(
  manager.includes("await this.runLifecycleMutation(() => this.shutdownInternal())") &&
    manager.includes("await this.runLifecycleMutation(() => this.recoverRuntimeInternal(reason))"),
  "shutdown and runtime recovery must share the serialized lifecycle mutation queue",
);
assert(
  deploymentLifecycle.includes("private shuttingDown = false") &&
    deploymentLifecycle.includes("private shutdownPromise: Promise<void> | null = null") &&
    deploymentLifecycle.includes("await this.stopLocalPetIntegrations()") &&
    deploymentLifecycle.includes("await this.startLocalPetIntegrations()") &&
    deploymentLifecycle.includes("if (localRuntimeAvailable)") &&
    deploymentLifecycle.includes("coreBaseURL: this.topology.businessCore.baseURL") &&
    deploymentLifecycle.includes("getBackendSessionClient().getMainProcessAuthHeaders()") &&
    deploymentLifecycle.includes("await this.desktopPetManager.initialize()") &&
    deploymentLifecycle.includes("await this.desktopPetManager.shutdown()") &&
    deploymentLifecycle.includes("await this.reconcileChain"),
  "deployment lifecycle must keep the desktop-pet body local in local/cloud modes and reconcile its character authority correctly",
);

assert(
  runtimeComponents.includes("profilesDeviceLocal") &&
    runtimeComponents.includes("runtimeprofile.ProfileDeviceAgent") &&
    runtimeComponents.includes("Profiles: profilesDeviceLocal"),
  "device-agent profile must host the local desktop-pet runtime in cloud deployments",
);

const deviceAgentPolicyStart = runtimePolicy.indexOf("case ProfileDeviceAgent:");
const deviceAgentPolicyEnd = runtimePolicy.indexOf("default:", deviceAgentPolicyStart);
const deviceAgentPolicy = runtimePolicy.slice(deviceAgentPolicyStart, deviceAgentPolicyEnd);
assert(
  deviceAgentPolicyStart >= 0 &&
    deviceAgentPolicy.includes("DesktopPet: true") &&
    deviceAgentPolicy.includes("FullHTTPAPI:          false") &&
    deviceAgentPolicy.includes("CoreBusinessServices: false"),
  "device-agent policy must enable only the device-local desktop-pet capability, not cloud business authority",
);

assert(
  deviceAgentRouter.includes("middleware.TraceMiddleware()") &&
    deviceAgentRouter.includes("security.CorsMiddleware") &&
    deviceAgentRouter.includes('localSession.POST("/sessions", sessionSvc.CreateSession)') &&
    deviceAgentRouter.includes('localDesktop.POST("/devices/register"') &&
    deviceAgentRouter.includes('runtime-bootstrap-tickets') &&
    deviceAgentRouter.includes('localAdmin.POST("/shutdown"') &&
    deviceAgentRouter.includes('security.RequirePermission("system.shutdown")') &&
    deviceAgentRouter.includes("desktoppet.RegisterDesktopPetRouter") &&
    deviceAgentRouter.includes("desktoppet.RegisterDesktopPetWriteRouter") &&
    deviceAgentRouter.includes("runtimev2.RegisterUserRoutes") &&
    deviceAgentRouter.includes("runtimev2.RegisterInternalRoutes") &&
    !deviceAgentRouter.includes("setupRouter(ctx"),
  "device-agent must expose authenticated local pet/session/Runtime-v2 plus graceful local-admin shutdown without opening the full business router",
);

assert(
  securityCors.includes('"X-Amitia-Device-ID"') &&
    securityCors.includes('"X-Amitia-Client-Type"') &&
    securityCorsTests.includes("AllowsDesktopDeviceHeadersForLoopbackPetRequests"),
  "cloud-to-loopback desktop-pet requests must pass CORS preflight with desktop device headers",
);

assert(
  frontendRuntimeAdapter.includes("isDeviceLocalApiPath") &&
    frontendRuntimeAdapter.includes('LOCAL_DEVICE_RUNTIME_BASE_URL = "http://127.0.0.1:18899"') &&
    frontendApi.includes("deviceLocal ? LOCAL_DEVICE_RUNTIME_BASE_URL : runtime.apiBaseURL") &&
    frontendApi.includes("delete config.headers.Authorization") &&
    frontendApi.includes("__amitiaDeviceLocal"),
  "desktop-pet API traffic must stay on the local device agent in cloud mode without leaking cloud bearer credentials",
);

assert(
  characterWatcher.includes("authHeadersProvider") &&
    characterWatcher.includes("await this.onActiveCharacterChanged?.(characterId)") &&
    characterWatcher.includes("this.lastCharacterId = characterId") &&
    characterWatcher.includes("首次角色同步失败，将继续重试") &&
    characterWatcher.includes("this.timer = setInterval") &&
    !characterWatcher.includes("if (!previous) return"),
  "character watcher must authenticate, reconcile the initial character, and keep retrying after transient startup failures",
);

assert(
  manager.includes("const previousInstallationId") &&
    manager.includes("switchInstallation.restorePrevious") &&
    manager.includes("PET_SWITCH_FAILED_AND_ROLLBACK_FAILED") &&
    manager.includes("selectInstallationForCharacter") &&
    manager.includes("INSTALLATION_STATUS_INSTALLED") &&
    manager.includes("this.parseTimestamp(right.lastEnabledAt)") &&
    manager.includes("角色切换时查询安装列表失败:") &&
    manager.includes("角色切换后切换桌宠失败:") &&
    managerTests.includes("propagates installation lookup failures so CharacterWatcher can retry") &&
    managerTests.includes("selects the most recently enabled usable pet for a newly active character") &&
    managerTests.includes("propagates switch failures so CharacterWatcher does not commit the new character") &&
    characterWatcherTests.includes("keeps retrying when initial reconciliation fails and only commits after success"),
  "desktop-pet switching must roll back failed targets and propagate character reconciliation failures for retry",
);

assert(
  behaviorReducer.includes('case "runtime.pointer.clicked":') &&
    behaviorReducer.includes('case "runtime.drag.completed", "runtime.drag.cancelled":') &&
    behaviorReducer.includes('case "runtime.playback.action_started":') &&
    !behaviorReducer.includes('case "desktop.pet.clicked":') &&
    !behaviorReducer.includes('case "playback.action.started":') &&
    behaviorSchema.includes('{"runtime.drag.cancelled"') &&
    behaviorSchema.includes('{"runtime.pet.interacted"') &&
    behaviorSchema.includes('"sequence": "int64", "interactionType": "string"') &&
    behaviorReducer.includes('case "runtime.pet.interacted":') &&
    behaviorReducer.includes("reduceDesktopInteracted") &&
    behaviorBindingValidator.includes('"runtime.drag.cancelled"') &&
    behaviorPlaybackAdapter.includes('eventType := "runtime.playback.action_" + string(feedback.Phase)') &&
    behaviorEnvelope.includes('return "runtime.pointer.clicked"') &&
    behaviorEnvelope.includes('return "runtime.drag.completed"') &&
    behaviorEnvelope.includes('builder.PayloadField("interactionType", evt.GestureType)') &&
    serverServices.includes('"runtime.drag.cancelled"'),
  "Runtime V2, behavior adapters, schema, bindings, sink, and reducer must share one canonical event vocabulary",
);

assert(
  frontendRequestAuth.includes('deployment.mode === "local" || deviceLocal') &&
    frontendRequestAuth.includes('headers.delete("Authorization")') &&
    frontendRequestAuth.includes("ensureValidToken()") &&
    authenticatedSSE.includes("createAuthenticatedFetchInit") &&
    authenticatedSSE.includes('Accept: "text/event-stream"') &&
    generationTask.includes("consumeAuthenticatedSSE") &&
    processingTask.includes("consumeAuthenticatedSSE") &&
    !generationTask.includes("new EventSource") &&
    !processingTask.includes("new EventSource"),
  "fetch/SSE authentication must use cloud bearer for business traffic and Desktop Session for local pet streams",
);

assert(
  desktopPreload.includes("notifyDesktopPetChatState") &&
    frontendPetChatState.includes("window.amitiaDesktop?.notifyDesktopPetChatState?.") &&
    (await read("front/src/composables/useWebChatSend.ts")).includes('notifyDesktopPetChatState("assistant_thinking"') &&
    (await read("front/src/composables/useWebChatSend.ts")).includes("let completed = false") &&
    (await read("front/src/composables/useWebChatSend.ts")).includes("if (completed)") &&
    (await read("front/src/composables/useWebChatSSE.ts")).includes('notifyDesktopPetChatState("assistant_speaking"') &&
    realtimeCallWidget.includes('"assistant_speaking"') &&
    realtimeCallWidget.includes('"assistant_listening"'),
  "cloud/local text and realtime voice lifecycles must drive the local pet speaking/thinking/listening state through Electron IPC",
);

const activeClientFiles = [
  ...(await collectFiles("desktop/src", [".ts", ".tsx", ".vue"])),
  ...(await collectFiles("front/src", [".ts", ".tsx", ".vue"])),
];
for (const file of activeClientFiles) {
  const source = await fs.readFile(file, "utf8");
  assert(
    !source.includes("launchOnStartup"),
    `legacy launchOnStartup leaked into active client source: ${path.relative(repoRoot, file)}`,
  );
}

assert(
  backendModel.includes('LaunchOnStartup') && backendModel.includes('json:"-"'),
  "legacy launch_on_startup column must be persistence-only",
);
assert(
  !backendDto.includes('json:"launchOnStartup"'),
  "desktop-pet API must not expose launchOnStartup as a runtime setting mutation",
);
assert(
  backendDto.includes('case "boundingbox":') &&
    backendDto.includes("v = clickThroughModeBoundingBox"),
  "backend must canonicalize boundingBox instead of lower-casing it into an invalid value",
);

assert(
  configStore.includes("getAutoLaunch") && configStore.includes("setAutoLaunch"),
  "Electron ConfigStore must remain the OS auto-launch authority",
);
assert(
  desktopIndex.includes("app.setLoginItemSettings") &&
    ipcHandlers.includes("app.setLoginItemSettings") &&
    tray.includes("app.setLoginItemSettings"),
  "OS auto-launch must be applied through the canonical Electron paths",
);
const allowedLoginItemFiles = new Set([
  "desktop/src/main/index.ts",
  "desktop/src/main/ipc-handlers.ts",
  "desktop/src/main/tray.ts",
]);
for (const file of await collectFiles("desktop/src", [".ts", ".tsx"])) {
  const source = await fs.readFile(file, "utf8");
  if (!source.includes("setLoginItemSettings")) continue;
  const relative = path.relative(repoRoot, file).replaceAll("\\", "/");
  assert(
    allowedLoginItemFiles.has(relative),
    `unexpected OS auto-launch authority: ${relative}`,
  );
}

console.log(
  "[verify-desktop-pet-finalization] PASSED: Runtime V2 behavior, cloud-local pet authority, character reconciliation, rollback, and startup authority are frozen",
);
