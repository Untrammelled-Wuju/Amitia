import { createHash } from "node:crypto";
import {
  DESKTOP_PET_RUNTIME_CONTRACT_VERSION,
  DESKTOP_PET_RUNTIME_VERSION,
} from "../../shared/desktop-pet-runtime-version";

export const RUNTIME_PROTOCOL_VERSION = "amitia.desktop-pet.runtime";
export const RUNTIME_ENVELOPE_VERSION = 2;
export const RUNTIME_CONTRACT_VERSION = DESKTOP_PET_RUNTIME_CONTRACT_VERSION;

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
  heartbeatIntervalMs?: number;
  heartbeatTimeoutMs?: number;
  maxMessageBytes?: number;
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
  decisionId?: string;
  actionKey?: string;
  triggerSource?: string;
  installationId?: string;
  characterId?: string;
  petInstanceId?: string;
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

function goJSONScalar(value: unknown): string {
  if (typeof value === "number" && Object.is(value, -0)) {
    return "-0";
  }
  const encoded = JSON.stringify(value);
  if (encoded === undefined) {
    throw new TypeError("runtime payload contains a non-JSON value");
  }
  if (typeof value !== "string") {
    return encoded;
  }
  // Go encoding/json uses HTML escaping by default and always escapes the two
  // JavaScript line-separator code points. Runtime V2 payload hashes are
  // authored by the Go server, so Electron must reproduce those bytes exactly.
  return encoded.replace(/[<>&\u2028\u2029]/g, (char) => {
    switch (char) {
      case "<": return "\\u003c";
      case ">": return "\\u003e";
      case "&": return "\\u0026";
      case "\u2028": return "\\u2028";
      case "\u2029": return "\\u2029";
      default: return char;
    }
  });
}

function backendCanonicalJSON(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(backendCanonicalJSON).join(",")}]`;
  }
  if (value !== null && typeof value === "object") {
    const input = value as Record<string, unknown>;
    return `{${Object.keys(input).sort().map((key) =>
      // deviceruntime/protocol.marshalCanonical writes protocol field names
      // directly. Runtime payload keys are schema-owned ASCII identifiers.
      `"${key}":${backendCanonicalJSON(input[key])}`
    ).join(",")}}`;
  }
  return goJSONScalar(value);
}

export function computePayloadHash(payload: unknown): string {
  const canonical = backendCanonicalJSON(payload);
  return "sha256:" + createHash("sha256").update(canonical, "utf8").digest("hex");
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
    runtimeVersion: input.runtimeVersion ?? DESKTOP_PET_RUNTIME_VERSION,
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
    payloadHash: computePayloadHash(payload),
    sentAt,
    payload,
  };
}
