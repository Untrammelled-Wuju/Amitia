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
    deploymentLifecycle.includes("await this.stopLegacyPetIntegrations()") &&
    deploymentLifecycle.includes("await this.startLegacyPetIntegrations()") &&
    deploymentLifecycle.includes("await this.desktopPetManager.initialize()") &&
    deploymentLifecycle.includes("await this.desktopPetManager.shutdown()") &&
    deploymentLifecycle.includes("await this.reconcileChain"),
  "deployment lifecycle must drain reconcile and await desktop-pet start/stop transitions",
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
  "[verify-desktop-pet-finalization] PASSED: desired-state convergence, restore policy, and startup authority are frozen",
);
