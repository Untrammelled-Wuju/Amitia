import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, "../..");

async function read(relativePath) {
  return fs.readFile(path.join(repoRoot, relativePath), "utf8");
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(`[verify-desktop-pet-runtime-lifecycle-semantics] ${message}`);
  }
}

const commandModel = await read("backend/internal/desktoppet/runtime/protocol/v2/command.go");
const commandService = await read("backend/internal/desktoppet/runtime/protocol/v2/command_service.go");
const dispatcher = await read("backend/internal/desktoppet/runtime/protocol/v2/command_dispatcher.go");
const handler = await read("backend/internal/desktoppet/runtime/protocol/v2/handler.go");
const envelope = await read("backend/internal/desktoppet/runtime/protocol/v2/envelope.go");
const runtimeHandler = await read("desktop/src/desktop-pet/runtime/runtime-handler-v2.ts");
const manager = await read("desktop/src/main/pet/manager.ts");
const mainIndex = await read("desktop/src/main/index.ts");
const scheduler = await read("desktop/src/main/pet/action-scheduler.ts");
const playerBridge = await read("desktop/src/main/pet/animation-player-bridge.ts");
const animationIpc = await read("desktop/src/main/pet/animation-ipc.ts");
const renderer = await read("desktop/src/renderer/pet-main.ts");
const engine = await read("desktop/src/desktop-pet/animation/animation-engine.ts");
const gateway = await read("desktop/src/desktop-pet/animation/command-gateway.ts");
const queue = await read("desktop/src/desktop-pet/animation/action-queue.ts");
const characterWatcher = await read("desktop/src/main/pet/character-watcher.ts");
const eventBridge = await read("desktop/src/main/pet/event-bridge.ts");
const dragController = await read("desktop/src/main/pet/drag-controller.ts");
const worldController = await read("desktop/src/main/pet/world-controller.ts");

assert(
  commandModel.includes("func (c CommandType) IsEphemeral() bool") &&
    commandModel.includes("CommandTypePlayAction, CommandTypeStopAction, CommandTypePauseAction") &&
    commandModel.includes("CommandTypeResumeAction, CommandTypeRecenterOnce") &&
    commandModel.includes("func (c CommandType) IsKnown() bool") &&
    commandModel.includes("func (c *RuntimeCommand) HasValidClassification() bool") &&
    commandService.includes("unsupported durable runtime command type") &&
    commandService.includes("unsupported ephemeral runtime command type"),
  "unknown or mismatched command types must not be classified or persisted as Runtime-v2 work",
);
assert(
  commandService.includes("unbound ephemeral command creation is disabled") &&
    commandService.includes("CreateEphemeralCommandForSession") &&
    commandService.includes("ExpiresAt:") &&
    commandService.includes("invalid play_action expiresAt") &&
    commandService.includes("CommandStatusPlaybackStarted") &&
    commandService.includes("playbackLivenessDeadline") &&
    commandService.includes("payload.MaximumPlayMs") &&
    commandService.includes("defaultPlaybackLivenessCeiling") &&
    commandService.includes("ephemeralExpiryReconcileGrace") &&
    commandService.includes("parsed.UTC().Before(expiresAt)") &&
    commandService.includes("expires_at <= ?"),
  "ephemeral commands must be session-bound, carry server-capped authoritative expiry, fail closed on invalid expiry, and use per-command playback liveness plus delivery grace instead of a fixed transport timeout",
);
assert(
  commandService.includes("runtime_playback_id") &&
    commandService.includes("MarkRendererAccepted") &&
    commandService.includes("MarkPlaybackStarted") &&
    commandService.includes("PlaybackRequestID") &&
    commandService.includes('strings.TrimSpace(cmd.PlaybackRequestID) != playbackID'),
  "backend command progress must bind and enforce renderer playback identity",
);
assert(
  dispatcher.includes("cmd.HasValidClassification()") &&
    dispatcher.includes("COMMAND_CLASSIFICATION_INVALID") &&
    dispatcher.includes("cmd.RuntimeSessionID != sessionID") &&
    dispatcher.includes("MarkSuperseded") &&
    dispatcher.includes("ExpiresAt:        cmd.ExpiresAt") &&
    dispatcher.includes("CommandAttempt") &&
    dispatcher.includes("connectionSupportsCommand") &&
    dispatcher.includes("return false"),
  "dispatcher must reject invalid stored classifications, fence stale sessions, propagate expiry, persist attempts, and enforce capabilities",
);
assert(
  envelope.includes("CurrentRuntimeVersion = contracts.RuntimeVersion") &&
    envelope.includes("CapabilitySyncDesiredV2") &&
    envelope.includes("CapabilityPlayActionV2") &&
    envelope.includes("CapabilityRendererAckV2") &&
    envelope.includes("CapabilityExpiryRFC3339"),
  "runtime version and mandatory capability IDs must have one backend authority",
);
assert(
  handler.includes("stored runtime command classification is invalid") &&
    handler.includes("payload.RuntimeVersion) != CurrentRuntimeVersion") &&
    handler.includes("mandatoryRuntimeCapabilities()") &&
    handler.includes("EventPlaybackCommandAccepted") &&
    handler.includes("MarkRendererAccepted(meta.CommandID") &&
    handler.includes("completed command_ack is forbidden for renderer/durable desired commands") &&
    handler.includes("desired-state event hash mismatch"),
  "backend must gate hello and derive renderer/durable terminal progress only from canonical runtime events",
);
assert(
  handler.includes("state snapshot appliedDesiredHash is required when appliedDesiredRevision > 0"),
  "runtime actual-state projection must fail closed when a claimed applied desired revision has no authoritative hash",
);
assert(
  handler.includes("SupersedeEphemeralCommands") &&
    handler.includes("CloseTransport()"),
  "reconnect must supersede old-session ephemeral work and proactively close the old transport",
);

const runtimeCommandAckWindow = runtimeHandler.slice(
  runtimeHandler.indexOf("private async handleCommand"),
  runtimeHandler.indexOf("private async replayCachedCommand"),
);
assert(
  runtimeHandler.includes("function isEphemeralRuntimeCommand") &&
    runtimeHandler.includes("function validateAuthoritativeExpiry") &&
    runtimeHandler.includes('errorCode: "COMMAND_EXPIRY_REQUIRED"') &&
    runtimeHandler.includes('errorCode: "COMMAND_EXPIRY_INVALID"') &&
    runtimeHandler.includes('errorCode: "COMMAND_EXPIRED"') &&
    runtimeCommandAckWindow.includes("validateAuthoritativeExpiry(command.expiresAt)"),
  "all Runtime-v2 Ephemeral commands must validate authoritative expiresAt before local execution",
);
assert(
  runtimeHandler.includes('"runtime.sync_desired_v2"') &&
    runtimeHandler.includes('"runtime.play_action_v2"') &&
    runtimeHandler.includes('"runtime.renderer_ack_v2"') &&
    runtimeHandler.includes('"runtime.expiry_rfc3339_v1"'),
  "desktop hello must advertise all mandatory Runtime-v2 capabilities",
);
assert(
  runtimeCommandAckWindow.includes('"runtime_received"') &&
    runtimeCommandAckWindow.includes('"runtime_accepted"') &&
    !runtimeCommandAckWindow.includes('sendCommandAck(envelope, "renderer_accepted"') &&
    runtimeHandler.includes('"runtime.state.desired_applied"') &&
    runtimeHandler.includes('"runtime.state.desired_rejected"'),
  "RuntimeHandler must not synthesize renderer ACK/terminal truth and must use canonical desired-state events",
);
assert(
  runtimeHandler.includes("drainInFlightCommands") &&
    manager.includes("await previousHandler.drainInFlightCommands()") &&
    manager.includes('this.scheduler?.forceInterrupt("runtime_stop")'),
  "desktop reconnect must drain admitted work and stop old-session physical playback",
);
assert(
  !runtimeCommandAckWindow.includes("desiredRevision <= this.lastAppliedDesiredRevision") &&
    manager.includes("validateDesiredCommandRevision") &&
    manager.includes('errorCode: "STALE_DESIRED_REVISION"') &&
    manager.includes('errorCode: "DESIRED_HASH_MISMATCH"'),
  "durable desired commands must reject stale revisions and same-revision hash conflicts instead of revision-only duplicate shortcuts",
);

assert(
  renderer.includes("crypto.randomUUID()") &&
    engine.includes('type: "playback.command_accepted"') &&
    playerBridge.includes("pendingPlaybackId = null") &&
    playerBridge.includes('case "playback.command_accepted"'),
  "playback identity must be renderer-generated and bound only after command_accepted",
);
assert(
  scheduler.includes("return Date.now();") &&
    scheduler.includes('this.emit("action-submitted"') &&
    scheduler.includes("notifyActionStarted(") &&
    scheduler.includes("ttl_expired_in_queue") &&
    scheduler.includes("previousRequest && this.replacementPrevious") &&
    !scheduler.includes('this.emit("action-started", effectiveRequest, action);'),
  "scheduler must use epoch time, recheck queued TTL, keep submission separate from renderer-started truth, and preserve the last real playback across pending replacement chains",
);
assert(
  animationIpc.includes("requiresAuthoritativeExpiry") &&
    animationIpc.includes("Date.parse(command.expiresAt as string)") &&
    animationIpc.includes('status: "rejected", reason: "ttl_expired"') &&
    animationIpc.includes('status: "rejected", reason: "command_invalid"') &&
    !animationIpc.includes("COMMAND_TTL_MS"),
  "Main IPC must preserve Runtime authoritative expiry without inventing a local TTL or rejecting local/default actions",
);
assert(
  gateway.includes("Date.parse") && queue.includes("Date.parse") &&
    gateway.includes("requiresAuthoritativeExpiry === true") &&
    queue.includes("requiresAuthoritativeExpiry === true"),
  "renderer gateway/queue must fail closed on missing Runtime expiry while allowing local/default commands without a Runtime TTL",
);
assert(
  engine.includes("command.expiresAt || command.requiresAuthoritativeExpiry === true") &&
    engine.includes("command.expiresAt ? Date.parse(command.expiresAt) : Number.NaN"),
  "renderer post-load expiry recheck must fail closed for Runtime commands without breaking local/default actions that intentionally have no TTL",
);
assert(
  manager.includes("const playerCommandId = event.commandId") &&
    manager.includes("const runtimeCommandId = mappedRuntimeCommandId") &&
    manager.includes("submittedRuntimeCommands") &&
    manager.includes("currentRequestRuntimeCommandId") &&
    manager.includes("MISSING_OR_INVALID_EXPIRY") &&
    manager.includes("INSTALLATION_MISMATCH") &&
    manager.includes("MISSING_CHARACTER_ID") &&
    manager.includes("CHARACTER_MISMATCH") &&
    manager.includes("MISSING_PET_INSTANCE_ID") &&
    manager.includes("PET_INSTANCE_MISMATCH") &&
    manager.includes("lastAppliedDesiredHash") &&
    manager.includes("petInstanceId: getRuntimeId()"),
  "Manager must correlate pending renderer submissions without guessing from Scheduler.current, separate local player IDs from Runtime IDs, and fail closed on missing/mismatched target identity, expiry, and desired hash",
);
assert(
  manager.includes("Do not infer Runtime identity from Scheduler.current here") &&
    !manager.slice(
      manager.indexOf("private handleActionSwitch"),
      manager.indexOf("private handleSchedulerEvent"),
    ).includes("playbackCommandIds.set"),
  "playback identity must never be guessed from the scheduler current request during command_accepted",
);
assert(
  manager.includes("suppressRuntimeLifecycleReporting") &&
    manager.includes("this.playbackCommandIds.clear();") &&
    manager.indexOf("this.playbackCommandIds.clear();", manager.indexOf("onHelloAck")) <
      manager.indexOf('this.scheduler?.forceInterrupt("runtime_stop")', manager.indexOf("onHelloAck")),
  "runtime reconnect must clear old playback identity before stopping old-session work and suppress synchronous old-session reports",
);
assert(
  characterWatcher.includes("hasObservedCharacter") &&
    characterWatcher.includes("string | null") &&
    manager.includes("handleCharacterSwitched(characterId: string | null)"),
  "active-character authority must represent and apply an explicit no-character state",
);
assert(
  dragController.includes('this.onEvent("drag-cancel"') &&
    eventBridge.includes('event === "drag-cancel"') &&
    !eventBridge.includes("pendingClickTimer") &&
    worldController.includes("isCurrentPlaybackStarted"),
  "interaction semantics must separate drag cancellation, avoid Main click-delay timers, and bind roaming to real playback",
);
assert(
  manager.includes("getChatStateWindow") &&
    manager.includes("senderWindow === trustedWindow") &&
    mainIndex.includes("getChatStateWindow: () => mainWindow"),
  "chat-state IPC must be authorized to the current main chat window rather than a broad URL origin",
);

console.log("[verify-desktop-pet-runtime-lifecycle-semantics] PASS");
