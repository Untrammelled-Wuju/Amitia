export const RUNTIME_PROTOCOL_VERSION = "amitia.desktop-pet.runtime";
export const RUNTIME_ENVELOPE_VERSION = 2;
export const RUNTIME_CONTRACT_VERSION = "2.0.0";

export type RuntimeMessageType =
  | "hello"
  | "hello_ack"
  | "command"
  | "command_ack"
  | "runtime_event"
  | "state_snapshot"
  | "error"
  | "ping"
  | "pong";

export type SessionStatus =
  | "registering"
  | "syncing"
  | "ready"
  | "degraded"
  | "closing"
  | "closed"
  | "superseded";

export function isSessionActive(status: SessionStatus): boolean {
  return status === "registering" || status === "syncing" || status === "ready" || status === "degraded";
}

export function isSessionTerminal(status: SessionStatus): boolean {
  return status === "closed" || status === "superseded";
}

export type CommandType =
  | "runtime.command.sync_desired_state"
  | "runtime.command.ensure_absent"
  | "runtime.command.reload_release"
  | "runtime.command.play_action"
  | "runtime.command.stop_action"
  | "runtime.command.pause_action"
  | "runtime.command.resume_action"
  | "runtime.command.recenter_once";

export type CommandStatus =
  | "created"
  | "queued"
  | "dispatching"
  | "transport_dispatched"
  | "runtime_received"
  | "runtime_accepted"
  | "renderer_accepted"
  | "playback_started"
  | "completed"
  | "failed_retryable"
  | "failed_terminal"
  | "expired"
  | "cancel_requested"
  | "cancelled"
  | "superseded";

export function isCommandTerminal(status: CommandStatus): boolean {
  return status === "completed" || status === "failed_terminal" || status === "expired" ||
    status === "cancelled" || status === "superseded";
}

export function isCommandRunning(status: CommandStatus): boolean {
  return status === "dispatching" || status === "transport_dispatched" ||
    status === "runtime_received" || status === "runtime_accepted" ||
    status === "renderer_accepted" || status === "playback_started";
}

export type TriggerSource =
  | "runtime_command"
  | "behavior"
  | "local_idle"
  | "local_interaction"
  | "system_recovery";

export interface RuntimeEnvelope {
  envelopeVersion: number;
  protocol: string;
  messageType: RuntimeMessageType;
  messageName: string;
  messageId: string;
  correlationId?: string;
  causationId?: string;
  userId: string;
  deviceId: string;
  runtimeId: string;
  runtimeSessionId: string;
  connectionGeneration: number;
  sequence: number;
  payloadSchemaVersion: number;
  payloadHash: string;
  sentAt: string;
  occurredAt?: string;
  payload?: unknown;
}

export interface HelloPayload {
  runtimeVersion: string;
  runtimeContractVersion: string;
  deviceId: string;
  runtimeId: string;
  runtimeCapabilities: string[];
  lastAppliedDesiredRevision: number;
  lastProcessedCommandSequence: number;
  lastEventSequence: number;
  actualStateHash?: string;
}

export interface HelloAckPayload {
  accepted: boolean;
  sessionId?: string;
  serverTime: string;
  currentDesiredRevision: number;
  resumeMode?: string;
  errorCode?: string;
  errorMessage?: string;
}

export interface CommandAckPayload {
  commandId: string;
  commandSequence: number;
  status: string;
  payloadHash?: string;
  rejectReason?: string;
  rejectErrorCode?: string;
  estimatedStartMs?: number;
  runtimeSessionId: string;
  receivedAt: string;
}

export interface PlaybackEventPayload {
  type: string;
  playbackInstanceId?: string;
  commandId?: string;
  actionKey?: string;
  triggerSource?: string;
  frameIndex?: number;
  cycleIndex?: number;
  startedAt?: string;
  completedAt?: string;
  holdingAt?: string;
  interruptedAt?: string;
  failedAt?: string;
  interruptReason?: string;
  replacedByCommandId?: string;
  replacedByPlaybackInstanceId?: string;
  completionReason?: string;
  cycleCount?: number;
  returnTarget?: string;
  errorCode?: string;
  errorMessage?: string;
  recoverable?: boolean;
  playedMs?: number;
  occurredAt: string;
}

export interface StateSnapshotPayload {
  connectionGeneration: number;
  eventSequence: number;
  actualStateHash: string;
  instanceStatus: string;
  windowStatus: string;
  rendererStatus: string;
  playbackStatus: string;
  appliedDesiredRevision: number;
  appliedDesiredHash?: string;
  appliedSettingsRevision: number;
  installationId: string;
  petId: string;
  releaseId: string;
  stableActionKey: string;
  currentActionKey: string;
  playbackInstanceId?: string;
  currentCommandId?: string;
  lastProcessedCommandSequence: number;
  capturedAt: string;
}

export interface RuntimeErrorPayload {
  code: string;
  message: string;
  commandId?: string;
}

export function buildHelloPayload(input: {
  deviceId: string;
  runtimeId: string;
  capabilities: string[];
  lastAppliedDesiredRevision?: number;
  lastProcessedCommandSequence?: number;
  lastEventSequence?: number;
  actualStateHash?: string;
  runtimeVersion?: string;
}): HelloPayload {
  return {
    runtimeVersion: input.runtimeVersion ?? "2.0.0",
    runtimeContractVersion: RUNTIME_CONTRACT_VERSION,
    deviceId: input.deviceId,
    runtimeId: input.runtimeId,
    runtimeCapabilities: input.capabilities ?? [],
    lastAppliedDesiredRevision: input.lastAppliedDesiredRevision ?? 0,
    lastProcessedCommandSequence: input.lastProcessedCommandSequence ?? 0,
    lastEventSequence: input.lastEventSequence ?? 0,
    actualStateHash: input.actualStateHash,
  };
}

export function buildEnvelope<T>(
  msgType: RuntimeMessageType,
  msgName: string,
  userId: string,
  deviceId: string,
  runtimeId: string,
  sessionId: string,
  connectionGen: number,
  sequence: number,
  payload: T,
): RuntimeEnvelope {
  const sentAt = new Date().toISOString();
  return {
    envelopeVersion: RUNTIME_ENVELOPE_VERSION,
    protocol: RUNTIME_PROTOCOL_VERSION,
    messageType: msgType,
    messageName: msgName,
    messageId: `msg_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`,
    correlationId: undefined,
    causationId: undefined,
    userId,
    deviceId,
    runtimeId,
    runtimeSessionId: sessionId,
    connectionGeneration: connectionGen,
    sequence,
    payloadSchemaVersion: 1,
    payloadHash: "",
    sentAt,
    payload,
  };
}
