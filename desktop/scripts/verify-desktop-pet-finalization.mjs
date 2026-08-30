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
const viteConfig = await read("desktop/vite.config.ts");
const copyStaticAssets = await read("desktop/scripts/copy-static-assets.mjs");
const rendererBuildVerifier = await read("desktop/scripts/verify-renderer-build.mjs");
const packagedPetVerifier = await read("desktop/scripts/verify-packaged-desktop-pet.mjs");
const packageSchema = await read("desktop/src/shared/package-schema.ts");
const resourceLoader = await read("desktop/src/main/pet/resource-loader.ts");
const visualSurface = await read("desktop/src/desktop-pet/animation/surface/canvas-pet-visual-surface.ts");
const backendPackageValidator = await read("backend/internal/desktoppet/packageformat/validator.go");
const backendSchemaRegistry = await read("backend/internal/desktoppet/packageformat/schema_registry.go");
const backendPackageManifest = await read("backend/internal/desktoppet/packageformat/manifest.go");
const backendReleaseService = await read("backend/internal/desktoppet/release/service.go");
const backendModel = await read("backend/internal/desktoppet/installation/model.go");
const backendDto = await read("backend/internal/desktoppet/installation/dto.go");
const backendInstallationCoordinator = await read("backend/internal/desktoppet/installation/coordinator/coordinator.go");
const backendInstallationHandler = await read("backend/internal/desktoppet/installation/handler.go");
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
const runtimeBehaviorFinalizationMigration = await read("backend/internal/migration/desktop_pet_runtime_behavior_finalization.go");
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
const runtimeProtocolV2 = await read("desktop/src/desktop-pet/runtime/protocol-v2.ts");
const runtimeProtocolV2Tests = await read("desktop/src/desktop-pet/runtime/__tests__/runtime-handler-v2.test.ts");
const appProtocol = await read("desktop/src/main/app-protocol.ts");
const desktopWindow = await read("desktop/src/main/window.ts");
const desktopConfigTemplate = await read("desktop/resources/config-template/config.yaml");
const mobileBackendRouting = await read("mobile_app/lib/core/backend_transport/routed_backend_service_api.dart");
const mobileBackendProviders = await read("mobile_app/lib/core/backend_transport/providers/backend_transport_providers.dart");
const mobileDesktopPetRuntime = await read("mobile_app/lib/features/desktop_pet/runtime/desktop_pet_mobile_runtime.dart");
const mobileDesktopPetPage = await read("mobile_app/lib/features/desktop_pet/presentation/pages/desktop_pet_page.dart");
const mobileAppRoot = await read("mobile_app/lib/app/app.dart");
const androidManifest = await read("mobile_app/android/app/src/main/AndroidManifest.xml");
const androidOverlay = await read("mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider/overlay/OverlayNativeHandler.kt");
const androidPetRenderer = await read("mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider/desktoppet/DesktopPetRendererNativeHandler.kt");
const androidCompositionRoot = await read("mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider/AndroidNativeCompositionRoot.kt");
const desktopPetWorkflow = await read(".github/workflows/desktop-pet.yml");
const sourceGitignore = await read(".gitignore");

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
  runtimeBehaviorFinalizationMigration.includes('Version: "202608300002"') &&
    runtimeBehaviorFinalizationMigration.includes('"desktop_pet_runtime_actual_states_v2", "position_x"') &&
    runtimeBehaviorFinalizationMigration.includes("desktop_pet_behavior_mesh_outbox") &&
    runtimeBehaviorFinalizationMigration.includes("payload_hash TEXT NOT NULL DEFAULT ''") &&
    runtimeBehaviorFinalizationMigration.includes("target_installation_id TEXT NOT NULL DEFAULT ''") &&
    migrations.includes("DesktopPetRuntimeBehaviorFinalizationMigration()") &&
    migrationBaseline.includes("ALTER TABLE desktop_pet_runtime_actual_states_v2 ADD COLUMN position_x") &&
    migrationBaseline.includes("ALTER TABLE desktop_pet_runtime_actual_states_v2 ADD COLUMN scale") &&
    migrationBaseline.includes("CREATE TABLE IF NOT EXISTS desktop_pet_behavior_mesh_affinities") &&
    migrationBaseline.includes("CREATE TABLE IF NOT EXISTS desktop_pet_behavior_mesh_outbox") &&
    migrationBaseline.includes("PRIMARY KEY(cloud_user_id, event_id)") &&
    migrationBaseline.includes("payload_hash TEXT NOT NULL DEFAULT ''") &&
    migrationBaseline.includes("target_installation_id TEXT NOT NULL DEFAULT ''") &&
    migrationBaseline.includes("idx_dpbmo_claim") &&
    migrationBaseline.includes("idx_dpbmo_expiry"),
  "Runtime V2 geometry and behavior-mesh durability schema must be present in both upgrade migration and new-database baseline",
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
    manager.includes("await this.runLifecycleMutation(() => this.recoverRuntimeInternal(reason))") &&
    manager.includes("this.initializeInternal(options.restoreActiveInstallation ?? true)") &&
    manager.includes("async handleCharacterSwitched(characterId: string | null)") &&
    manager.includes("await this.handleCharacterSwitchedInternal(normalized)"),
  "initialize, character reconciliation, shutdown and runtime recovery must share the serialized lifecycle mutation queue",
);
assert(
  viteConfig.includes('"pet-main": resolve(__dirname, "src/renderer/pet-main.ts")') &&
    viteConfig.includes('chunkInfo.name === "pet-main"') &&
    !copyStaticAssets.includes('dist-types/src/renderer/pet-main.js'),
  "production pet renderer must be a real Vite/Rollup entry, never a copied standalone tsc module",
);
assert(
  rendererBuildVerifier.includes("checkRendererModuleGraph") &&
    rendererBuildVerifier.includes("pet-main.js still contains source-tree relative imports") &&
    packagedPetVerifier.includes('"pet-main.js"') &&
    packagedPetVerifier.includes("collectRendererBundleFiles"),
  "release verification must prove pet-main and its bundled dependency graph are present before and after ASAR packaging",
);
assert(
  packageSchema.includes("Number.isInteger(numberValue)") &&
    packageSchema.includes("requireV2PlaybackMode") &&
    packageSchema.includes("maximumPlayMs: number | null") &&
    resourceLoader.includes("maximumPlayMs: action.maximumPlayMs ?? null") &&
    resourceLoader.includes("interruptAfterMs: action.interruptAfterMs") &&
    backendPackageValidator.includes("DecodeStrictTopLevelJSON(data, &cfg, actionConfigAllowedFields)") &&
    backendPackageValidator.includes("validateV2ActionNestedRequiredFields") &&
    backendPackageValidator.includes("ErrCodePackageResourceHashMismatch") &&
    backendPackageValidator.includes("validateLegacyActionConfigLayer") &&
    backendSchemaRegistry.includes("v2ManifestRequiredFields") &&
    backendSchemaRegistry.includes("isJSONNull"),
  "Package V2 readers must preserve null/zero semantics, enforce nested/hash integrity, and retain real V1 compatibility",
);
assert(
  backendPackageValidator.includes("expectedIntegrityAlgorithm := IntegrityAlgorithmV2") &&
    backendPackageValidator.includes("isLegacyV1 && action.QualityVerdict == QualityVerdictSkipped") &&
    backendPackageManifest.includes('Builder:    "amitia-packageformat-v2"') &&
    backendReleaseService.includes('"maximumPlayMs":          nil') &&
    backendReleaseService.includes('"mutexGroup":             nil') &&
    backendReleaseService.includes('"supportsDefaultIdle":    supportsDefaultIdle') &&
    backendReleaseService.includes('"isStableStateCandidate": isStableStateCandidate') &&
    backendReleaseService.includes('"isTransitionOnly":       isTransitionOnly'),
  "V2 package producers and validators must emit and enforce the same canonical semantic fields",
);
assert(
  visualSurface.includes('anchorType === "normalized_canvas"') &&
    visualSurface.includes("anchor.x * canvasWidth") &&
    visualSurface.includes("anchor.y * canvasHeight"),
  "normalized_canvas anchors must be resolved against canvas dimensions instead of treated as pixel coordinates",
);

assert(
  deploymentLifecycle.includes("private shuttingDown = false") &&
    deploymentLifecycle.includes("private shutdownPromise: Promise<void> | null = null") &&
    deploymentLifecycle.includes("await this.stopLocalPetIntegrations()") &&
    deploymentLifecycle.includes("await this.startLocalPetIntegrations()") &&
    deploymentLifecycle.includes("if (localRuntimeAvailable)") &&
    deploymentLifecycle.includes("coreBaseURL: this.topology.businessCore.baseURL") &&
    deploymentLifecycle.includes("getBackendSessionClient().getMainProcessAuthHeaders()") &&
    deploymentLifecycle.includes("await this.desktopPetManager.initialize({ restoreActiveInstallation: false })") &&
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
    securityCors.includes('"Idempotency-Key"') &&
    securityCorsTests.includes("AllowsDesktopDeviceHeadersForLoopbackPetRequests") &&
    securityCorsTests.includes('"idempotency-key"') &&
    securityCorsTests.includes("AllowsPackagedAppOrigin"),
  "cloud-to-loopback desktop-pet requests must pass CORS preflight with device and idempotency headers",
);

assert(
  appProtocol.includes("protocol.registerSchemesAsPrivileged") &&
    appProtocol.includes('standard: true') &&
    appProtocol.includes('secure: true') &&
    appProtocol.includes('supportFetchAPI: true') &&
    desktopIndex.includes("registerAppProtocol(") &&
    desktopWindow.includes('win.loadURL(`${APP_PROTOCOL_ORIGIN}/#/`)') &&
    !desktopWindow.includes("win.loadFile(") &&
    desktopConfigTemplate.includes('- "app://amitia"'),
  "packaged Electron renderer must use the privileged app://amitia origin instead of file:// opaque origin",
);

assert(
  mobileBackendRouting.includes("'/api/desktop-pets'") &&
    mobileBackendRouting.includes("'/api/desktop-pet'") &&
    mobileBackendRouting.includes("Idempotency-Key") &&
    mobileBackendRouting.includes("X-Amitia-Client-Type") &&
    mobileBackendProviders.includes("deviceLocalBackendConnectionProvider") &&
    mobileBackendProviders.includes("rawDeviceLocalBackendServiceApiProvider") &&
    mobileBackendProviders.includes("RoutedBackendServiceApiProxy"),
  "mobile desktop-pet API traffic must route to the embedded Device Agent and use fresh mutation idempotency keys",
);

assert(
  androidManifest.includes("android.permission.SYSTEM_ALERT_WINDOW") &&
    androidOverlay.includes("WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY") &&
    androidOverlay.includes("windowManager.addView") &&
    androidOverlay.includes("Settings.ACTION_MANAGE_OVERLAY_PERMISSION") &&
    androidPetRenderer.includes("PACKAGE_SCHEMA_VERSION = 2") &&
    androidPetRenderer.includes('PACKAGE_MANIFEST_FORMAT = "amitia-desktop-pet"') &&
    androidPetRenderer.includes("windowManager.addView") &&
    androidPetRenderer.includes("lastCompletedPlaybackId") &&
    androidPetRenderer.includes("actualFiles == seen") &&
    androidPetRenderer.includes("canonicalManifestData(manifest)") &&
    androidPetRenderer.includes("computeContentRootHash(") &&
    androidCompositionRoot.includes("DesktopPetRendererNativeHandler(context)"),
  "Android desktop-pet host must provide a real WindowManager overlay and Package V2 renderer",
);

assert(
  mobileDesktopPetRuntime.includes("_runtimeWsSubprotocol = 'amitia.runtime.v2'") &&
    mobileDesktopPetRuntime.includes("desktopPetMobileRuntimeBootstrapProvider") &&
    mobileDesktopPetRuntime.includes("CHARACTER_MISMATCH") &&
    mobileDesktopPetRuntime.includes("lastCompletedPlaybackId") &&
    mobileDesktopPetRuntime.includes("Runtime identity and cursor are incarnation-scoped") &&
    mobileDesktopPetRuntime.includes("runtime envelope sequence is stale or duplicated") &&
    mobileDesktopPetRuntime.includes("final playedMs = _nonNegativeInt(native['lastCompletedPlayedMs'])") &&
    mobileDesktopPetRuntime.includes("final playedMs = _nonNegativeInt(rendererStop['stoppedPlayedMs'])") &&
    mobileDesktopPetRuntime.includes("preservePositionMode: true") &&
    mobileDesktopPetRuntime.includes("_persistRuntimePosition(") &&
    mobileDesktopPetRuntime.includes("final settings = _map(updated['settings'])") &&
    mobileDesktopPetRuntime.includes("桌宠居中位置保存失败") &&
    !mobileDesktopPetRuntime.includes("difference(tracked.startedAt)") &&
    !mobileDesktopPetRuntime.includes("_prefsRuntimeId") &&
    mobileAppRoot.includes("ref.watch(desktopPetMobileRuntimeBootstrapProvider)") &&
    mobileDesktopPetPage.includes("desktopPetMobileRuntimeProvider") &&
    !mobileDesktopPetPage.includes("companionStateProvider") &&
    !mobileDesktopPetPage.includes("心情很好"),
  "mobile desktop-pet center and Runtime V2 transport must use real renderer facts with strict per-connection sequencing",
);

assert(
  runtimeProtocolV2.includes("function backendCanonicalJSON(value: unknown): string") &&
    runtimeProtocolV2.includes("Object.is(value, -0)") &&
    runtimeProtocolV2.includes('case "<": return "\\\\u003c"') &&
    runtimeProtocolV2Tests.includes("b7e059caca7f51b074a58adda59aaa8943450b4d78e80b4cb5aed992a5fe6e0a"),
  "Electron Runtime V2 payload hashing must match the Go canonical JSON golden vector",
);

assert(
  mobileDesktopPetRuntime.includes("String _backendCanonicalJson(Object? value)") &&
    mobileDesktopPetRuntime.includes("value == 0 && value.isNegative") &&
    mobileDesktopPetRuntime.includes("replaceAll('<', r'\\u003c')") &&
    mobileDesktopPetRuntime.includes("replaceAll('&', r'\\u0026')"),
  "mobile Runtime V2 payload hashing must reproduce the Go server canonical JSON escaping and number semantics",
);

assert(
  desktopPetWorkflow.includes("- 'mobile_app/**'") &&
    desktopPetWorkflow.includes("Mobile Desktop Pet Gate") &&
    desktopPetWorkflow.includes("flutter analyze") &&
    desktopPetWorkflow.includes("flutter build apk --debug") &&
    desktopPetWorkflow.includes("- 'backend/**'") &&
    desktopPetWorkflow.includes("Run desktop-pet CORS integration tests") &&
    desktopPetWorkflow.includes("verify-pet-player-singleton.mjs") &&
    desktopPetWorkflow.includes("verify-desktop-pet-runtime-singletrack.mjs") &&
    desktopPetWorkflow.includes("verify-desktop-pet-runtime-lifecycle-semantics.mjs") &&
    desktopPetWorkflow.includes("verify-desktop-pet-finalization.mjs") &&
    desktopPetWorkflow.includes("verify-cloud-device-execution-finalization.mjs") &&
    desktopPetWorkflow.includes("verify-desktop-pet-device-agent-freeze-gate.mjs"),
  "desktop-pet CI must cover mobile/Android compilation and every production freeze gate",
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


const updateSettingsStart = manager.indexOf("async updateSettings(");
const applyRuntimeSettingsStart = manager.indexOf("private async applyRuntimeSettingsLocal(", updateSettingsStart);
const publicUpdateSettings = manager.slice(updateSettingsStart, applyRuntimeSettingsStart);
assert(
  updateSettingsStart >= 0 &&
    applyRuntimeSettingsStart > updateSettingsStart &&
    publicUpdateSettings.includes("return this.callUpdateSettingsApi") &&
    !publicUpdateSettings.includes("applyRuntimeSettingsLocal") &&
    !publicUpdateSettings.includes("markSettingsRevisionApplied"),
  "public settings mutations must submit desired state only; Runtime V2 is the sole runtime/apply authority",
);

const updateDefaultStart = manager.indexOf("async updateDefaultAction(");
const applyDefaultStart = manager.indexOf("private async applyDefaultActionLocal(", updateDefaultStart);
const publicUpdateDefault = manager.slice(updateDefaultStart, applyDefaultStart);
assert(
  updateDefaultStart >= 0 &&
    applyDefaultStart > updateDefaultStart &&
    publicUpdateDefault.includes("return this.callUpdateDefaultActionApi") &&
    !publicUpdateDefault.includes("applyDefaultActionLocal"),
  "public default-action mutations must not bypass Runtime V2 desired-state convergence",
);

const persistPositionStart = manager.indexOf("private async persistRuntimePosition()");
const persistPositionEnd = manager.indexOf("private registerChatStateIpc()", persistPositionStart);
const persistPositionBody = manager.slice(persistPositionStart, persistPositionEnd);
assert(
  persistPositionStart >= 0 &&
    persistPositionEnd > persistPositionStart &&
    persistPositionBody.includes("await this.callUpdateSettingsApi") &&
    !persistPositionBody.includes("markSettingsRevisionApplied") &&
    !persistPositionBody.includes("this.activeSettings ="),
  "physical drag persistence must not claim a backend settings revision is locally applied before Runtime V2 ACK",
);

assert(
  manager.includes("PET_MUTATION_RESPONSE_INVALID") &&
    manager.includes("PET_MUTATION_NOT_ACCEPTED") &&
    manager.includes("desiredRevision") &&
    manager.includes("operationId"),
  "desktop mutation APIs must preserve operation metadata and fail closed on invalid/failed coordinator responses",
);

assert(
  backendInstallationCoordinator.includes("Stage: operation.OpStageWaitingRuntimeACK") &&
    backendInstallationCoordinator.includes("Stage: existing.Stage") &&
    backendInstallationHandler.includes('"desiredRevision": result.DesiredRevision') &&
    backendInstallationHandler.includes('"settingsRevision": updatedSettings.SettingsRevision'),
  "coordinator mutation responses must preserve waiting-runtime-ACK stage and desired/settings revision metadata",
);

assert(
  desktopPetWorkflow.includes("- 'backend/**'") &&
    desktopPetWorkflow.includes("name: Backend Full Test Gate") &&
    desktopPetWorkflow.includes("go test ./... -count=1") &&
    desktopPetWorkflow.includes("sha256sum -c DESKTOP_PET_FINALIZATION_20260830_SHA256SUMS.txt") &&
    desktopPetWorkflow.split("- name: Verify Dart SDK floor").length - 1 === 1 &&
    desktopPetWorkflow.includes("run: dart --version"),
  "Desktop Pet CI must trigger on all backend dependencies, run the complete backend suite, verify the frozen hash baseline, and contain no empty Flutter steps",
);

assert(
  sourceGitignore.includes("temp_apk_extract/") &&
    sourceGitignore.includes(".gradle-proot-build/") &&
    sourceGitignore.includes("trace.atrace") &&
    sourceGitignore.includes(".meituan-catpaw/"),
  "final source-package hygiene must ignore local APK extraction, Gradle audit, trace, and AI workspace artifacts",
);

for (const file of await collectFiles("backend/internal/desktoppet", [".go"])) {
  const source = await fs.readFile(file, "utf8");
  assert(
    !source.includes("internal/gamehost"),
    `desktop-pet backend must not depend on GameHost internals: ${path.relative(repoRoot, file)}`,
  );
}

console.log(
  "[verify-desktop-pet-finalization] PASSED: Runtime V2 behavior, cloud-local pet authority, character reconciliation, rollback, and startup authority are frozen",
);
