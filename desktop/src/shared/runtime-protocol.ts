export const RUNTIME_PROTOCOL_VERSION_1_0 = "1.0";
export const RUNTIME_CURRENT_PROTOCOL_VERSION = RUNTIME_PROTOCOL_VERSION_1_0;
export const RUNTIME_SNAPSHOT_VERSION_V2 = 2;
export const RUNTIME_CURRENT_SNAPSHOT_VERSION = RUNTIME_SNAPSHOT_VERSION_V2;

export type RuntimeEventType =
  | "runtime.connected"
  | "runtime.disconnected"
  | "runtime.heartbeat"
  | "desktop.pet.clicked"
  | "desktop.pet.double_clicked"
  | "desktop.pet.hover.entered"
  | "desktop.pet.hover.moved"
  | "desktop.pet.hover.left"
  | "desktop.pet.drag.started"
  | "desktop.pet.drag.moved"
  | "desktop.pet.drag.completed"
  | "desktop.pet.drag.cancelled"
  | "playback.action.requested"
  | "playback.action.started"
  | "playback.action.completed"
  | "playback.action.interrupted"
  | "playback.action.failed"
  | "window.shown"
  | "window.hidden"
  | "window.moved"
  | "window.display_changed"
  | "runtime.state.snapshot"
  | "runtime.command.acknowledged"
  | "runtime.command.rejected";

export const EvtRuntimeConnected: RuntimeEventType = "runtime.connected";
export const EvtRuntimeDisconnected: RuntimeEventType = "runtime.disconnected";
export const EvtRuntimeHeartbeat: RuntimeEventType = "runtime.heartbeat";
export const EvtDesktopPetClicked: RuntimeEventType = "desktop.pet.clicked";
export const EvtDesktopPetDoubleClicked: RuntimeEventType = "desktop.pet.double_clicked";
export const EvtDesktopPetHoverEntered: RuntimeEventType = "desktop.pet.hover.entered";
export const EvtDesktopPetHoverMoved: RuntimeEventType = "desktop.pet.hover.moved";
export const EvtDesktopPetHoverLeft: RuntimeEventType = "desktop.pet.hover.left";
export const EvtDesktopPetDragStarted: RuntimeEventType = "desktop.pet.drag.started";
export const EvtDesktopPetDragMoved: RuntimeEventType = "desktop.pet.drag.moved";
export const EvtDesktopPetDragCompleted: RuntimeEventType = "desktop.pet.drag.completed";
export const EvtDesktopPetDragCancelled: RuntimeEventType = "desktop.pet.drag.cancelled";
export const EvtPlaybackActionRequested: RuntimeEventType = "playback.action.requested";
export const EvtPlaybackActionStarted: RuntimeEventType = "playback.action.started";
export const EvtPlaybackActionCompleted: RuntimeEventType = "playback.action.completed";
export const EvtPlaybackActionInterrupted: RuntimeEventType = "playback.action.interrupted";
export const EvtPlaybackActionFailed: RuntimeEventType = "playback.action.failed";
export const EvtWindowShown: RuntimeEventType = "window.shown";
export const EvtWindowHidden: RuntimeEventType = "window.hidden";
export const EvtWindowMoved: RuntimeEventType = "window.moved";
export const EvtWindowDisplayChanged: RuntimeEventType = "window.display_changed";
export const EvtRuntimeStateSnapshot: RuntimeEventType = "runtime.state.snapshot";
export const EvtRuntimeCommandAcknowledged: RuntimeEventType = "runtime.command.acknowledged";
export const EvtRuntimeCommandRejected: RuntimeEventType = "runtime.command.rejected";

const STANDARD_EVENT_TYPES: Set<RuntimeEventType> = new Set<RuntimeEventType>([
  EvtRuntimeConnected,
  EvtRuntimeDisconnected,
  EvtRuntimeHeartbeat,
  EvtDesktopPetClicked,
  EvtDesktopPetDoubleClicked,
  EvtDesktopPetHoverEntered,
  EvtDesktopPetHoverMoved,
  EvtDesktopPetHoverLeft,
  EvtDesktopPetDragStarted,
  EvtDesktopPetDragMoved,
  EvtDesktopPetDragCompleted,
  EvtDesktopPetDragCancelled,
  EvtPlaybackActionRequested,
  EvtPlaybackActionStarted,
  EvtPlaybackActionCompleted,
  EvtPlaybackActionInterrupted,
  EvtPlaybackActionFailed,
  EvtWindowShown,
  EvtWindowHidden,
  EvtWindowMoved,
  EvtWindowDisplayChanged,
  EvtRuntimeStateSnapshot,
  EvtRuntimeCommandAcknowledged,
  EvtRuntimeCommandRejected,
]);

export function isValidEventType(t: string): boolean {
  return STANDARD_EVENT_TYPES.has(t as RuntimeEventType);
}

export type CommandPhase =
  | "created"
  | "queued"
  | "sent"
  | "received"
  | "accepted"
  | "started"
  | "completed"
  | "rejected"
  | "failed"
  | "expired"
  | "superseded";

export const PhaseCreated: CommandPhase = "created";
export const PhaseQueued: CommandPhase = "queued";
export const PhaseSent: CommandPhase = "sent";
export const PhaseReceived: CommandPhase = "received";
export const PhaseAccepted: CommandPhase = "accepted";
export const PhaseStarted: CommandPhase = "started";
export const PhaseCompleted: CommandPhase = "completed";
export const PhaseRejected: CommandPhase = "rejected";
export const PhaseFailed: CommandPhase = "failed";
export const PhaseExpired: CommandPhase = "expired";
export const PhaseSuperseded: CommandPhase = "superseded";

export type IdempotencyMode = "stateful_replace" | "once" | "query";

export const IdempotencyStatefulReplace: IdempotencyMode = "stateful_replace";
export const IdempotencyOnce: IdempotencyMode = "once";
export const IdempotencyQuery: IdempotencyMode = "query";

export type RejectReason =
  | "runtime_not_ready"
  | "installation_mismatch"
  | "release_mismatch"
  | "action_not_found"
  | "command_expired"
  | "command_out_of_order"
  | "unsupported_command"
  | "invalid_payload"
  | "window_unavailable"
  | "renderer_unavailable"
  | "command_id_conflict"
  | "stale_desired_revision";

export const RejectRuntimeNotReady: RejectReason = "runtime_not_ready";
export const RejectInstallationMismatch: RejectReason = "installation_mismatch";
export const RejectReleaseMismatch: RejectReason = "release_mismatch";
export const RejectActionNotFound: RejectReason = "action_not_found";
export const RejectCommandExpired: RejectReason = "command_expired";
export const RejectCommandOutOfOrder: RejectReason = "command_out_of_order";
export const RejectUnsupportedCommand: RejectReason = "unsupported_command";
export const RejectInvalidPayload: RejectReason = "invalid_payload";
export const RejectWindowUnavailable: RejectReason = "window_unavailable";
export const RejectRendererUnavailable: RejectReason = "renderer_unavailable";
export const RejectCommandIDConflict: RejectReason = "command_id_conflict";
export const RejectStaleDesiredRevision: RejectReason = "stale_desired_revision";

export type ResumeMode = "resume" | "full_resync" | "session_reset";

export const ResumeModeResume: ResumeMode = "resume";
export const ResumeModeFullResync: ResumeMode = "full_resync";
export const ResumeModeSessionReset: ResumeMode = "session_reset";

export interface RuntimeEventEnvelope {
  protocolVersion: string;
  eventId: string;
  eventType: RuntimeEventType;
  eventSequence: number;
  userId: string;
  deviceId: string;
  installationId: string;
  petId: string;
  releaseId: string;
  runtimeInstanceId: string;
  commandId?: string;
  desiredRevision?: number;
  occurredAt: string;
  sentAt: string;
  payload: unknown;
}

export interface RuntimeCommandEnvelope {
  protocolVersion: string;
  commandId: string;
  commandType: string;
  commandSequence: number;
  userId: string;
  deviceId: string;
  installationId: string;
  petId: string;
  releaseId: string;
  runtimeInstanceId: string;
  desiredRevision: number;
  issuedAt: string;
  expiresAt: string;
  idempotencyMode: IdempotencyMode;
  operationId?: string;
  payload: unknown;
}

export interface RuntimeSessionContext {
  userId: string;
  deviceId: string;
  installationId: string;
  petId: string;
  releaseId: string;
  runtimeInstanceId: string;
}

export interface ClickPayload {
  button: string;
  clickCount: number;
  canvasX: number;
  canvasY: number;
  screenX: number;
  screenY: number;
  frameIndex: number;
  actionKey: string;
}

export interface DragPayload {
  dragId: string;
  phase: "started" | "moved" | "completed" | "cancelled";
  startX: number;
  startY: number;
  currentX: number;
  currentY: number;
  deltaX: number;
  deltaY: number;
  displayId: string;
}

export interface PlaybackPayload {
  actionKey: string;
  playbackId: string;
  commandId?: string;
  frameIndex: number;
  cycleIndex: number;
  startedAt?: string;
  completedAt?: string;
  interruptReason?: string;
  errorCode?: string;
}

export interface WindowPayload {
  visible: boolean;
  x: number;
  y: number;
  displayId: string;
  width: number;
  height: number;
}

export interface RuntimeErrorPayload {
  errorCode: string;
  errorMessage: string;
  component: string;
  recoverable: boolean;
  commandId?: string;
  playbackId?: string;
  actionKey?: string;
}

export interface CommandAckPayload {
  commandId: string;
  commandSequence: number;
  status: string;
  runtimeInstanceId: string;
  receivedAt: string;
  rejectReason?: string;
}

export interface RuntimeHelloPayload {
  protocolVersion: string;
  deviceId: string;
  clientVersion: string;
  runtimeCapabilities: string[];
  lastReceivedCommandSequence: number;
  lastSentEventSequence: number;
  lastAppliedDesiredRevision: number;
  pendingCommandIds?: string[];
}

export interface RuntimeWelcomePayload {
  runtimeInstanceId: string;
  serverTime: string;
  acceptedProtocolVersion: string;
  currentDesiredRevision: number;
  resumeMode: ResumeMode;
  sessionId: string;
  heartbeatIntervalMs: number;
  heartbeatTimeoutMs: number;
  maxMessageBytes: number;
}

export interface ActualStateMeta {
  runtimeInstanceId: string;
  lastEventSequence: number;
  lastCommandSequence: number;
  lastAppliedDesiredRevision: number;
  lastEventId: string;
}

export interface RuntimeStateSnapshotV2 {
  snapshotVersion: number;
  runtimeInstanceId: string;
  installationId: string;
  petId: string;
  releaseId: string;
  connected: boolean;
  windowVisible: boolean;
  windowX: number;
  windowY: number;
  displayId: string;
  currentActionKey: string;
  playbackId: string;
  playbackPhase: string;
  frameIndex: number;
  cycleIndex: number;
  lastAppliedDesiredRevision: number;
  lastReceivedCommandSequence: number;
  lastSentEventSequence: number;
  capturedAt: string;
}

export type OutboxPriority = 0 | 1 | 2;

export const OutboxPriorityMustRetain: OutboxPriority = 0;
export const OutboxPriorityMergeable: OutboxPriority = 1;
export const OutboxPriorityDroppable: OutboxPriority = 2;

export function outboxPriorityForEvent(eventType: RuntimeEventType): OutboxPriority {
  switch (eventType) {
    case EvtRuntimeCommandAcknowledged:
    case EvtRuntimeCommandRejected:
      return OutboxPriorityMustRetain;
    case EvtPlaybackActionCompleted:
    case EvtPlaybackActionInterrupted:
    case EvtPlaybackActionFailed:
      return OutboxPriorityMustRetain;
    case EvtRuntimeStateSnapshot:
      return OutboxPriorityMergeable;
    case EvtWindowMoved:
    case EvtDesktopPetHoverMoved:
      return OutboxPriorityMergeable;
    case EvtRuntimeHeartbeat:
      return OutboxPriorityMergeable;
    default:
      return OutboxPriorityDroppable;
  }
}

export const InterruptReasonHigherPriorityAction = "higher_priority_action";
export const InterruptReasonReleaseSwitch = "release_switch";
export const InterruptReasonRuntimeDisable = "runtime_disable";
export const InterruptReasonWindowDestroyed = "window_destroyed";
export const InterruptReasonUserDrag = "user_drag";
export const InterruptReasonCommandCancelled = "command_cancelled";

export const ErrorCodeRendererNotReady = "renderer_not_ready";
export const ErrorCodeWindowCreateFailed = "window_create_failed";
export const ErrorCodeReleaseLoadFailed = "release_load_failed";
export const ErrorCodeActionNotFound = "action_not_found";
export const ErrorCodeFrameMissing = "frame_missing";
export const ErrorCodeFrameDecodeFailed = "frame_decode_failed";
export const ErrorCodeAnimationTimeout = "animation_timeout";
export const ErrorCodeIpcDeliveryFailed = "ipc_delivery_failed";
export const ErrorCodeRuntimeSessionSuperseded = "runtime_session_superseded";
export const ErrorCodeProtocolVersionUnsupported = "protocol_version_unsupported";

export type RuntimeCapability =
  | "animation.v2"
  | "playback.hold"
  | "playback.ping_pong"
  | "window.click-through.alpha"
  | "window.multi-display"
  | "runtime.command-ack"
  | "runtime.snapshot.v2"
  | "pet.window"
  | "pet.animation.frame_sequence"
  | "pet.settings"
  | "pet.recenter"
  | "pet.click_through"
  | "pet.interaction_events";

export const CapAnimationV2: RuntimeCapability = "animation.v2";
export const CapPlaybackHold: RuntimeCapability = "playback.hold";
export const CapPlaybackPingPong: RuntimeCapability = "playback.ping_pong";
export const CapWindowClickThrough: RuntimeCapability = "window.click-through.alpha";
export const CapWindowMultiDisplay: RuntimeCapability = "window.multi-display";
export const CapRuntimeCommandAck: RuntimeCapability = "runtime.command-ack";
export const CapRuntimeSnapshotV2: RuntimeCapability = "runtime.snapshot.v2";
export const CapPetWindow: RuntimeCapability = "pet.window";
export const CapPetAnimationFrame: RuntimeCapability = "pet.animation.frame_sequence";
export const CapPetSettings: RuntimeCapability = "pet.settings";
export const CapPetRecenter: RuntimeCapability = "pet.recenter";
export const CapPetClickThrough: RuntimeCapability = "pet.click_through";
export const CapPetInteractionEvents: RuntimeCapability = "pet.interaction_events";

export function inferClickEventType(payload: ClickPayload): RuntimeEventType {
  return payload.clickCount >= 2
    ? EvtDesktopPetDoubleClicked
    : EvtDesktopPetClicked;
}

export function inferDragEventType(payload: DragPayload): RuntimeEventType {
  switch (payload.phase) {
    case "started":
      return EvtDesktopPetDragStarted;
    case "moved":
      return EvtDesktopPetDragMoved;
    case "completed":
      return EvtDesktopPetDragCompleted;
    case "cancelled":
      return EvtDesktopPetDragCancelled;
    default:
      return EvtDesktopPetDragMoved;
  }
}

export function inferPlaybackEventType(payload: PlaybackPayload): RuntimeEventType {
  if (payload.errorCode) return EvtPlaybackActionFailed;
  if (payload.interruptReason) return EvtPlaybackActionInterrupted;
  if (payload.completedAt) return EvtPlaybackActionCompleted;
  if (payload.startedAt) return EvtPlaybackActionStarted;
  return EvtPlaybackActionRequested;
}
