import type {
  RuntimeEnvelope,
  HelloAckPayload,
  CommandAckPayload,
  PlaybackEventPayload,
  StateSnapshotPayload,
  CommandStatus,
  RuntimeMessageType,
} from "./protocol-v2";
import {
  buildHelloPayload,
  buildEnvelope,
  isCommandTerminal,
  computePayloadHash,
} from "./protocol-v2";
import type { RuntimeCommandExecutionResult } from "../../main/pet/runtime-v2-command-adapter";
import { DESKTOP_PET_RUNTIME_VERSION } from "../../shared/desktop-pet-runtime-version";

export type RuntimeHandlerState =
  | "disconnected"
  | "handshaking"
  | "connected"
  | "degraded"
  | "reconnecting";

export type ReconnectReason =
  | "initial"
  | "transport_lost"
  | "generation_changed"
  | "server_request";

export interface RuntimeCapabilities {
  devicePixelRatio: number;
  supportsHighDpi: boolean;
  supportsHitTest: boolean;
  supportsShadow: boolean;
  platform: string;
  arch: string;
}

export interface RuntimeEventContext {
  installationId?: string;
  characterId?: string;
  petInstanceId?: string;
  decisionId?: string;
}

export interface RuntimeResumeCursor {
  lastAppliedDesiredRevision: number;
  lastProcessedCommandSequence: number;
  lastEventSequence: number;
  actualStateHash?: string;
}

export interface RuntimeCommandReplayEntry {
  commandId: string;
  commandSequence: number;
  desiredRevision: number;
  ackStatus: "completed" | "failed_terminal";
  errorCode?: string;
  errorMessage?: string;
}

export interface RuntimeHandlerConfig {
  url: string;
  bootstrapTicket: string;
  userId: string;
  deviceId: string;
  runtimeId: string;
  runtimeVersion?: string;
  capabilities?: RuntimeCapabilities;
  autoReconnect?: boolean;
  connectTimeoutMs?: number;
  heartbeatIntervalMs?: number;
  heartbeatTimeoutMs?: number;
  maxMessageBytes?: number;
  maxReconnectAttempts?: number;
  reconnectBaseDelayMs?: number;
  reconnectMaxDelayMs?: number;
  resumeCursor?: Partial<RuntimeResumeCursor>;
  replayEntries?: readonly RuntimeCommandReplayEntry[];
}

export interface RuntimeHandlerHooks {
  onState: (state: RuntimeHandlerState) => void;
  onHelloAck: (ack: HelloAckPayload) => void;
  onEvent: (envelope: RuntimeEnvelope) => void;
  onError: (err: Error) => void;
  onDesiredSync: (revision: number) => void;
  onCommand: (command: unknown, envelope: RuntimeEnvelope) => Promise<RuntimeCommandExecutionResult>;
  onCommandSettled?: (result: RuntimeCommandExecutionResult, envelope: RuntimeEnvelope) => void | Promise<void>;
}

export interface RuntimeCommandAttempt {
  commandId: string;
  idempotencyKey: string;
  status: CommandStatus;
  attemptedAt: number;
}

interface CachedCommandExecution {
  commandSequence: number;
  desiredRevision: number;
  result: RuntimeCommandExecutionResult;
  ackStatus: CommandStatus;
}

const MAX_COMMAND_REPLAY_CACHE = 256;

const DEFAULT_HEARTBEAT_MS = 15000;
const DEFAULT_HEARTBEAT_TIMEOUT_MS = 30000;
const DEFAULT_MAX_MESSAGE_BYTES = 1048576;
const DEFAULT_CONNECT_TIMEOUT_MS = 10000;
const DEFAULT_MAX_RECONNECT = 5;
const DEFAULT_RECONNECT_BASE_MS = 1000;
const DEFAULT_RECONNECT_MAX_MS = 30000;

export const RUNTIME_V2_WEBSOCKET_SUBPROTOCOL = "amitia.runtime.v2";
export const RUNTIME_V2_BOOTSTRAP_SUBPROTOCOL_PREFIX = "amitia.runtime.bootstrap.";

function buildRuntimeWebSocketProtocols(ticket: string): string[] {
  const normalized = ticket.trim();
  if (normalized === "") {
    throw new Error("runtime bootstrap ticket is required");
  }
  if (!/^[A-Za-z0-9._~-]+$/.test(normalized)) {
    throw new Error("runtime bootstrap ticket contains invalid websocket protocol characters");
  }
  return [
    RUNTIME_V2_WEBSOCKET_SUBPROTOCOL,
    `${RUNTIME_V2_BOOTSTRAP_SUBPROTOCOL_PREFIX}${normalized}`,
  ];
}

function isRevisionedDurableCommand(commandType: string | undefined): boolean {
  return commandType === "runtime.command.sync_desired_state" ||
    commandType === "runtime.command.ensure_absent" ||
    commandType === "runtime.command.reload_release";
}

function sanitizeCursor(value: number | undefined): number {
  return typeof value === "number" && Number.isFinite(value) && value > 0
    ? Math.floor(value)
    : 0;
}

export class DesktopRuntimeHandlerV2 {
  private readonly config: Required<Omit<RuntimeHandlerConfig, "resumeCursor" | "replayEntries">>;
  private readonly hooks: RuntimeHandlerHooks;

  private ws: WebSocket | null = null;
  private state: RuntimeHandlerState = "disconnected";
  private sessionId = "";
  private connectionGeneration = 0;
  private outboundSequence = 0;
  private lastEventSequence = 0;
  private lastProcessedCommandSequence = 0;
  private lastAppliedDesiredRevision = 0;
  private actualStateHash = "";
  private lastReportedHealthStatus = "unknown";
  private readonly trackedCommandSequences = new Map<string, number>();
  private readonly commandReplayCache = new Map<string, CachedCommandExecution>();
  private readonly inFlightCommands = new Map<string, Promise<void>>();
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private idleHeartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private lastServerTime = "";
  private lastServerMessageAt = 0;
  private pendingConnectResolve: (() => void) | null = null;
  private pendingConnectReject: ((err: Error) => void) | null = null;

  private readonly pendingCommands = new Map<string, RuntimeCommandAttempt>();
  private reconnectReason: ReconnectReason = "initial";

  constructor(config: RuntimeHandlerConfig, hooks: RuntimeHandlerHooks) {
    this.config = {
      url: config.url,
      bootstrapTicket: config.bootstrapTicket,
      userId: config.userId,
      deviceId: config.deviceId,
      runtimeId: config.runtimeId,
      runtimeVersion: config.runtimeVersion ?? DESKTOP_PET_RUNTIME_VERSION,
      capabilities: config.capabilities ?? {
        devicePixelRatio: 1,
        supportsHighDpi: false,
        supportsHitTest: true,
        supportsShadow: false,
        platform: typeof navigator !== "undefined" ? (navigator.platform ?? "unknown") : "unknown",
        arch: "unknown",
      },
      autoReconnect: config.autoReconnect ?? false,
      connectTimeoutMs: config.connectTimeoutMs ?? DEFAULT_CONNECT_TIMEOUT_MS,
      heartbeatIntervalMs: config.heartbeatIntervalMs ?? DEFAULT_HEARTBEAT_MS,
      heartbeatTimeoutMs: config.heartbeatTimeoutMs ?? DEFAULT_HEARTBEAT_TIMEOUT_MS,
      maxMessageBytes: config.maxMessageBytes ?? DEFAULT_MAX_MESSAGE_BYTES,
      maxReconnectAttempts: config.maxReconnectAttempts ?? DEFAULT_MAX_RECONNECT,
      reconnectBaseDelayMs: config.reconnectBaseDelayMs ?? DEFAULT_RECONNECT_BASE_MS,
      reconnectMaxDelayMs: config.reconnectMaxDelayMs ?? DEFAULT_RECONNECT_MAX_MS,
    };
    const resume = config.resumeCursor ?? {};
    this.lastAppliedDesiredRevision = sanitizeCursor(resume.lastAppliedDesiredRevision);
    this.lastProcessedCommandSequence = sanitizeCursor(resume.lastProcessedCommandSequence);
    this.lastEventSequence = sanitizeCursor(resume.lastEventSequence);
    this.outboundSequence = this.lastEventSequence;
    this.actualStateHash = typeof resume.actualStateHash === "string" ? resume.actualStateHash : "";
    for (const entry of config.replayEntries ?? []) {
      if (!entry?.commandId || (entry.ackStatus !== "completed" && entry.ackStatus !== "failed_terminal")) {
        continue;
      }
      const commandSequence = sanitizeCursor(entry.commandSequence);
      const desiredRevision = sanitizeCursor(entry.desiredRevision);
      this.cacheCommandExecution(entry.commandId, {
        commandSequence,
        desiredRevision,
        ackStatus: entry.ackStatus,
        result: {
          commandId: entry.commandId,
          status: entry.ackStatus === "completed" ? "duplicate" : "failed",
          errorCode: entry.errorCode ?? "",
          errorMessage: entry.errorMessage ?? "",
          appliedRevision: desiredRevision,
        },
      });
    }
    this.hooks = hooks;
  }

  getState(): RuntimeHandlerState {
    return this.state;
  }

  getSessionId(): string {
    return this.sessionId;
  }

  getEventSequence(): number {
    return this.lastEventSequence;
  }

  getLastProcessedCommandSequence(): number {
    return this.lastProcessedCommandSequence;
  }

  getLastAppliedDesiredRevision(): number {
    return this.lastAppliedDesiredRevision;
  }

  getActualStateHash(): string {
    return this.actualStateHash;
  }

  getResumeCursor(): RuntimeResumeCursor {
    return {
      lastAppliedDesiredRevision: this.lastAppliedDesiredRevision,
      lastProcessedCommandSequence: this.lastProcessedCommandSequence,
      lastEventSequence: this.lastEventSequence,
      actualStateHash: this.actualStateHash || undefined,
    };
  }

  getReplayEntries(): RuntimeCommandReplayEntry[] {
    const entries: RuntimeCommandReplayEntry[] = [];
    for (const [commandId, cached] of this.commandReplayCache) {
      if (!isCommandTerminal(cached.ackStatus)) continue;
      entries.push({
        commandId,
        commandSequence: cached.commandSequence,
        desiredRevision: cached.desiredRevision,
        ackStatus: cached.ackStatus === "failed_terminal" ? "failed_terminal" : "completed",
        errorCode: cached.result.errorCode || undefined,
        errorMessage: cached.result.errorMessage || undefined,
      });
    }
    return entries.slice(-MAX_COMMAND_REPLAY_CACHE);
  }

  getConnectionGeneration(): number {
    return this.connectionGeneration;
  }

  async connect(reconnectReason: ReconnectReason = "initial"): Promise<void> {
    this.reconnectReason = reconnectReason;
    this.setState("handshaking");

    return new Promise<void>((resolve, reject) => {
      try {
        this.cleanupSocket();
        this.rejectPendingConnect(new Error("runtime connection superseded"));
        this.pendingConnectResolve = resolve;
        this.pendingConnectReject = reject;

        const ws = new WebSocket(
          this.config.url,
          buildRuntimeWebSocketProtocols(this.config.bootstrapTicket),
        );
        this.ws = ws;

        const timeoutId = setTimeout(() => {
          if (this.state !== "handshaking") return;
          const err = new Error("runtime connect timeout");
          this.rejectPendingConnect(err);
          ws.close(4000, "connect_timeout");
        }, this.config.connectTimeoutMs);

        ws.onopen = () => {
          clearTimeout(timeoutId);
          this.sendHello().catch((err) => {
            this.rejectPendingConnect(err instanceof Error ? err : new Error(String(err)));
            ws.close(4001, "hello_send_failed");
          });
        };

        ws.onmessage = (event: MessageEvent) => {
          this.handleMessage(event.data).catch((err) => {
            const error = err instanceof Error ? err : new Error(String(err));
            this.hooks.onError(error);
            if (this.ws === ws && ws.readyState === WebSocket.OPEN) {
              ws.close(4003, "protocol_violation");
            }
          });
        };

        ws.onerror = () => {
          clearTimeout(timeoutId);
          if (this.state === "handshaking") {
            this.rejectPendingConnect(new Error("runtime socket error"));
          }
        };

        ws.onclose = (event) => {
          clearTimeout(timeoutId);
          if (this.state === "handshaking") {
            this.rejectPendingConnect(new Error(`runtime socket closed during handshake: ${event.code} ${event.reason}`));
          }
          this.handleClose(event.code, event.reason);
        };

        this.startHeartbeat();
      } catch (err) {
        const error = err instanceof Error ? err : new Error(String(err));
        this.setState("degraded");
        this.rejectPendingConnect(error);
      }
    });
  }

  private resolvePendingConnect(): void {
    const resolve = this.pendingConnectResolve;
    this.pendingConnectResolve = null;
    this.pendingConnectReject = null;
    resolve?.();
  }

  private rejectPendingConnect(err: Error): void {
    const reject = this.pendingConnectReject;
    this.pendingConnectResolve = null;
    this.pendingConnectReject = null;
    reject?.(err);
  }

  disconnect(): void {
    this.rejectPendingConnect(new Error("runtime disconnected"));
    this.stopHeartbeat();
    this.cleanupSocket();
    this.setState("disconnected");
  }

  async sendPlaybackStarted(
    playbackId: string,
    commandId: string,
    actionKey: string,
    context: RuntimeEventContext = {},
  ): Promise<void> {
    const payload: PlaybackEventPayload = {
      type: "runtime.playback.action_started",
      playbackInstanceId: playbackId,
      commandId,
      actionKey,
      ...context,
      startedAt: new Date().toISOString(),
      occurredAt: new Date().toISOString(),
    };
    await this.sendRuntimeEvent("runtime.playback.action_started", payload);
  }

  async sendPlaybackFirstCycle(
    playbackId: string,
    commandId: string,
    actionKey: string,
    context: RuntimeEventContext = {},
  ): Promise<void> {
    const payload: PlaybackEventPayload = {
      type: "runtime.playback.action_first_cycle",
      playbackInstanceId: playbackId,
      commandId,
      actionKey,
      ...context,
      cycleIndex: 1,
      occurredAt: new Date().toISOString(),
    };
    await this.sendRuntimeEvent("runtime.playback.action_first_cycle", payload);
  }

  async sendPlaybackEnded(
    playbackId: string,
    commandId: string,
    actionKey: string,
    playedMs: number,
    completionReason: string,
    context: RuntimeEventContext = {},
  ): Promise<void> {
    const payload: PlaybackEventPayload = {
      type: "runtime.playback.action_completed",
      playbackInstanceId: playbackId,
      commandId,
      actionKey,
      ...context,
      playedMs,
      completionReason,
      completedAt: new Date().toISOString(),
      occurredAt: new Date().toISOString(),
    };
    await this.sendRuntimeEvent("runtime.playback.action_completed", payload);
    this.markTrackedCommandProcessed(commandId);
  }

  async sendPlaybackInterrupted(
    playbackId: string,
    commandId: string,
    actionKey: string,
    playedMs: number,
    interruptReason: string,
    context: RuntimeEventContext = {},
  ): Promise<void> {
    const payload: PlaybackEventPayload = {
      type: "runtime.playback.action_interrupted",
      playbackInstanceId: playbackId,
      commandId,
      actionKey,
      ...context,
      playedMs,
      interruptReason,
      interruptedAt: new Date().toISOString(),
      occurredAt: new Date().toISOString(),
    };
    await this.sendRuntimeEvent("runtime.playback.action_interrupted", payload);
    this.markTrackedCommandProcessed(commandId);
  }

  async sendPlaybackFailed(
    playbackId: string,
    commandId: string,
    actionKey: string,
    errorCode: string,
    errorMessage: string,
    context: RuntimeEventContext = {},
  ): Promise<void> {
    const payload: PlaybackEventPayload = {
      type: "runtime.playback.action_failed",
      playbackInstanceId: playbackId,
      commandId,
      actionKey,
      ...context,
      errorCode,
      errorMessage,
      failedAt: new Date().toISOString(),
      occurredAt: new Date().toISOString(),
    };
    await this.sendRuntimeEvent("runtime.playback.action_failed", payload);
    this.markTrackedCommandProcessed(commandId, "failed_terminal", errorCode, errorMessage);
  }

  async sendRendererState(snapshot: StateSnapshotPayload): Promise<void> {
    const normalized: StateSnapshotPayload = {
      ...snapshot,
      connectionGeneration: Math.max(1, this.connectionGeneration),
      eventSequence: this.outboundSequence + 1,
      appliedDesiredRevision: Math.max(snapshot.appliedDesiredRevision, this.lastAppliedDesiredRevision),
      lastProcessedCommandSequence: Math.max(
        snapshot.lastProcessedCommandSequence,
        this.lastProcessedCommandSequence,
      ),
      actualStateHash: snapshot.actualStateHash || this.actualStateHash,
      capturedAt: snapshot.capturedAt || new Date().toISOString(),
    };
    await this.sendRuntimeEvent("runtime.state.snapshot", normalized);
    this.lastAppliedDesiredRevision = Math.max(
      this.lastAppliedDesiredRevision,
      normalized.appliedDesiredRevision,
    );
    this.lastProcessedCommandSequence = Math.max(
      this.lastProcessedCommandSequence,
      normalized.lastProcessedCommandSequence,
    );
    if (normalized.actualStateHash) {
      this.actualStateHash = normalized.actualStateHash;
    }
  }

  async sendRendererHealth(healthy: boolean, errorCode?: string): Promise<void> {
    const currentStatus = healthy ? "healthy" : "failed";
    if (currentStatus === this.lastReportedHealthStatus && !errorCode) {
      return;
    }
    const previousStatus = this.lastReportedHealthStatus;
    await this.sendRuntimeEvent("runtime.health.changed", {
      previousStatus,
      currentStatus,
      reason: errorCode || undefined,
      changedAt: new Date().toISOString(),
    });
    this.lastReportedHealthStatus = currentStatus;
  }

  isConnected(): boolean {
    return this.state === "connected" && this.ws?.readyState === WebSocket.OPEN;
  }

  private async sendEnvelope(
    type: RuntimeMessageType,
    name: string,
    payload: unknown,
    allowHandshake = false,
  ): Promise<void> {
    const ws = this.ws;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      throw new Error("runtime socket not open");
    }
    if (!allowHandshake && this.state !== "connected") {
      throw new Error("runtime session not ready");
    }
    this.outboundSequence += 1;
    const envelope = buildEnvelope(
      type,
      name,
      this.config.userId,
      this.config.deviceId,
      this.config.runtimeId,
      this.sessionId,
      Math.max(1, this.connectionGeneration),
      this.outboundSequence,
      payload,
    );
    const serialized = JSON.stringify(envelope);
    const messageBytes = new TextEncoder().encode(serialized).byteLength;
    if (this.config.maxMessageBytes > 0 && messageBytes > this.config.maxMessageBytes) {
      throw new Error(
        `runtime envelope exceeds maxMessageBytes: ${messageBytes} > ${this.config.maxMessageBytes}`,
      );
    }
    ws.send(serialized);
    if (type === "runtime_event" || type === "command_ack") {
      this.lastEventSequence = this.outboundSequence;
    }
  }

  async sendRuntimeEvent(name: string, payload: unknown): Promise<void> {
    await this.sendEnvelope("runtime_event", name, payload);
  }

  private async handleMessage(raw: unknown): Promise<void> {
    if (typeof raw !== "string") {
      throw new Error("runtime server envelope must be text");
    }
    if (this.config.maxMessageBytes > 0 && new TextEncoder().encode(raw).byteLength > this.config.maxMessageBytes) {
      throw new Error("runtime server envelope exceeds maxMessageBytes");
    }

    const envelope = JSON.parse(raw) as RuntimeEnvelope;
    this.validateServerEnvelope(envelope);
    this.lastServerMessageAt = Date.now();

    switch (envelope.messageType) {
      case "hello_ack":
        this.connectionGeneration = Math.max(
          this.connectionGeneration,
          envelope.connectionGeneration,
        );
        await this.handleHelloAck(envelope.payload as HelloAckPayload);
        break;
      case "command":
        await this.handleCommand(envelope);
        break;
      case "command_ack":
        await this.handleCommandAck(envelope.payload as CommandAckPayload);
        break;
      case "state_snapshot":
        this.handleStateSnapshot(envelope.payload as StateSnapshotPayload);
        break;
      case "error":
        this.handleServerEnvelopeError(envelope);
        break;
      default:
        this.hooks.onEvent(envelope);
        break;
    }
  }

  private validateServerEnvelope(envelope: RuntimeEnvelope): void {
    if (!envelope || envelope.envelopeVersion !== 2 || envelope.protocol !== "amitia.desktop-pet.runtime") {
      throw new Error("invalid runtime server envelope protocol");
    }
    if (envelope.userId !== this.config.userId ||
        envelope.deviceId !== this.config.deviceId ||
        envelope.runtimeId !== this.config.runtimeId) {
      throw new Error("runtime server envelope identity mismatch");
    }
    if (!Number.isInteger(envelope.connectionGeneration) || envelope.connectionGeneration <= 0) {
      throw new Error("runtime server envelope generation is invalid");
    }
    if (!Number.isInteger(envelope.sequence) || envelope.sequence <= 0) {
      throw new Error("runtime server envelope sequence is invalid");
    }
    if (envelope.payloadHash !== computePayloadHash(envelope.payload)) {
      throw new Error("runtime server envelope payload hash mismatch");
    }

    if (envelope.messageType === "hello_ack") {
      const ack = envelope.payload as HelloAckPayload | undefined;
      if (!ack) {
        throw new Error("runtime hello_ack payload is missing");
      }
      if (ack.accepted && (!ack.sessionId || envelope.runtimeSessionId !== ack.sessionId)) {
        throw new Error("runtime hello_ack session mismatch");
      }
      if (this.connectionGeneration > 0 && envelope.connectionGeneration < this.connectionGeneration) {
        throw new Error("runtime hello_ack generation regressed");
      }
      return;
    }

    if (!this.sessionId || envelope.runtimeSessionId !== this.sessionId) {
      throw new Error("runtime server envelope session mismatch");
    }
    if (envelope.connectionGeneration !== this.connectionGeneration) {
      throw new Error("runtime server envelope generation mismatch");
    }
  }

  private async handleHelloAck(ack: HelloAckPayload): Promise<void> {
    if (!ack.accepted) {
      const err = new Error(`hello rejected: ${ack.errorCode} ${ack.errorMessage}`);
      this.setState("degraded");
      this.rejectPendingConnect(err);
      this.hooks.onError(err);
      return;
    }

    this.sessionId = ack.sessionId ?? "";
    this.lastServerTime = ack.serverTime ?? "";
    if (typeof ack.heartbeatIntervalMs === "number" && Number.isFinite(ack.heartbeatIntervalMs) && ack.heartbeatIntervalMs > 0) {
      this.config.heartbeatIntervalMs = Math.floor(ack.heartbeatIntervalMs);
    }
    if (typeof ack.heartbeatTimeoutMs === "number" && Number.isFinite(ack.heartbeatTimeoutMs) &&
        ack.heartbeatTimeoutMs > this.config.heartbeatIntervalMs) {
      this.config.heartbeatTimeoutMs = Math.floor(ack.heartbeatTimeoutMs);
    }
    if (typeof ack.maxMessageBytes === "number" && Number.isFinite(ack.maxMessageBytes) && ack.maxMessageBytes > 0) {
      this.config.maxMessageBytes = Math.floor(ack.maxMessageBytes);
    }
    this.lastServerMessageAt = Date.now();
    this.reconnectAttempts = 0;
    this.setState("connected");
    this.startHeartbeat();
    this.resolvePendingConnect();

    this.hooks.onHelloAck(ack);
  }

  private async handleCommand(envelope: RuntimeEnvelope): Promise<void> {
    const command = envelope.payload as {
      commandId?: string;
      commandType?: string;
      commandSequence?: number;
      desiredRevision?: number;
      settingsRevision?: number;
      installationId?: string;
      petId?: string;
      releaseId?: string;
      payload?: unknown;
    } | undefined;

    if (!command?.commandId || !command.commandType) {
      throw new Error("invalid runtime command");
    }

    const commandId = command.commandId;
    const commandSequence = sanitizeCursor(command.commandSequence);
    const desiredRevision = sanitizeCursor(command.desiredRevision);

    const cached = this.commandReplayCache.get(commandId);
    if (cached) {
      await this.replayCachedCommand(commandId, commandSequence, cached);
      return;
    }

    const inFlight = this.inFlightCommands.get(commandId);
    if (inFlight) {
      await inFlight;
      const settled = this.commandReplayCache.get(commandId);
      if (settled) {
        await this.replayCachedCommand(commandId, commandSequence, settled);
      }
      return;
    }

    // lastProcessedCommandSequence is a high-water mark, not proof that every
    // lower sequence has completed: independent commands may settle out of
    // order. Only revisioned durable desired-state commands can be suppressed
    // without a commandId cache entry, because an already-applied revision is
    // itself the idempotency fence. Ephemeral commands must never be discarded
    // solely because their sequence is below the high-water mark.
    if (
      isRevisionedDurableCommand(command.commandType) &&
      desiredRevision > 0 &&
      desiredRevision <= this.lastAppliedDesiredRevision
    ) {
      const duplicateResult: RuntimeCommandExecutionResult = {
        commandId,
        status: "duplicate",
        errorCode: "",
        errorMessage: "",
        appliedRevision: this.lastAppliedDesiredRevision,
      };
      this.lastProcessedCommandSequence = Math.max(this.lastProcessedCommandSequence, commandSequence);
      this.cacheCommandExecution(commandId, {
        commandSequence,
        desiredRevision,
        result: duplicateResult,
        ackStatus: "completed",
      });
      await this.sendCommandAck(commandId, commandSequence, "completed");
      return;
    }

    const execution = this.executeCommand(
      envelope,
      {
        commandId,
        commandType: command.commandType,
        commandSequence,
        desiredRevision,
      },
      commandSequence,
      desiredRevision,
    );
    this.inFlightCommands.set(commandId, execution);
    try {
      await execution;
    } finally {
      if (this.inFlightCommands.get(commandId) === execution) {
        this.inFlightCommands.delete(commandId);
      }
    }
  }

  private async executeCommand(
    envelope: RuntimeEnvelope,
    command: {
      commandId: string;
      commandType?: string;
      commandSequence?: number;
      desiredRevision?: number;
    },
    commandSequence: number,
    desiredRevision: number,
  ): Promise<void> {
    await this.sendCommandAck(command.commandId, commandSequence, "runtime_received");
    await this.sendCommandAck(command.commandId, commandSequence, "runtime_accepted");

    let result: RuntimeCommandExecutionResult;
    try {
      result = await this.hooks.onCommand(envelope.payload, envelope);
    } catch (err) {
      result = {
        commandId: command.commandId,
        status: "failed",
        errorCode: "RENDERER_REJECTED",
        errorMessage: err instanceof Error ? err.message : String(err),
        appliedRevision: 0,
      };
    }

    const ackStatus = this.mapCommandExecutionStatus(result.status);

    // The local side effect/result is authoritative once onCommand returns. It
    // must be recorded before attempting the terminal ACK; otherwise a socket
    // loss between execution and ACK can cause a durable redelivery to execute
    // the same side effect again and can misclassify transport failure as a
    // renderer failure.
    this.recordCommandResult(
      command.commandId,
      commandSequence,
      desiredRevision,
      result,
      ackStatus,
    );
    this.cacheCommandExecution(command.commandId, {
      commandSequence,
      desiredRevision,
      result,
      ackStatus,
    });

    let ackError: unknown;
    try {
      if (result.status === "failed" || result.status === "rejected" ||
          result.status === "expired" || result.status === "cancelled") {
        await this.sendCommandAck(
          command.commandId,
          commandSequence,
          ackStatus,
          result.errorCode || "RENDERER_REJECTED",
          result.errorMessage || "command execution failed",
        );
      } else {
        await this.sendCommandAck(command.commandId, commandSequence, "renderer_accepted");
        if (ackStatus !== "renderer_accepted") {
          await this.sendCommandAck(command.commandId, commandSequence, ackStatus);
        }
      }
    } catch (err) {
      ackError = err;
    }

    try {
      await this.hooks.onCommandSettled?.(result, envelope);
    } catch (err) {
      this.hooks.onError(err instanceof Error ? err : new Error(String(err)));
    }

    if (ackError) {
      throw ackError;
    }
  }

  private cacheCommandExecution(commandId: string, cached: CachedCommandExecution): void {
    this.commandReplayCache.delete(commandId);
    this.commandReplayCache.set(commandId, cached);
    while (this.commandReplayCache.size > MAX_COMMAND_REPLAY_CACHE) {
      const oldest = this.commandReplayCache.keys().next().value as string | undefined;
      if (!oldest) break;
      this.commandReplayCache.delete(oldest);
    }
  }

  private async replayCachedCommand(
    commandId: string,
    commandSequence: number,
    cached: CachedCommandExecution,
  ): Promise<void> {
    const sequence = commandSequence > 0 ? commandSequence : cached.commandSequence;
    if (isCommandTerminal(cached.ackStatus)) {
      this.lastProcessedCommandSequence = Math.max(this.lastProcessedCommandSequence, sequence);
    }
    if (cached.ackStatus === "failed_terminal") {
      await this.sendCommandAck(
        commandId,
        sequence,
        "failed_terminal",
        cached.result.errorCode || "RENDERER_REJECTED",
        cached.result.errorMessage || "command execution failed",
      );
      return;
    }
    await this.sendCommandAck(commandId, sequence, cached.ackStatus);
  }

  private async handleCommandAck(ack: CommandAckPayload): Promise<void> {
    const pending = this.pendingCommands.get(ack.commandId);
    if (pending) {
      pending.status = ack.status as CommandStatus;
      if (isCommandTerminal(ack.status as CommandStatus)) {
        this.pendingCommands.delete(ack.commandId);
      }
    }
  }

  private handleStateSnapshot(snapshot: StateSnapshotPayload): void {
    this.hooks.onEvent({
      envelopeVersion: 2,
      protocol: "amitia.desktop-pet.runtime",
      messageType: "state_snapshot",
      messageName: "state_snapshot",
      messageId: `snap_${Date.now()}`,
      userId: this.config.userId,
      deviceId: this.config.deviceId,
      runtimeId: this.config.runtimeId,
      runtimeSessionId: this.sessionId,
      connectionGeneration: snapshot.connectionGeneration,
      sequence: snapshot.eventSequence,
      payloadSchemaVersion: 1,
      payloadHash: computePayloadHash(snapshot),
      sentAt: new Date().toISOString(),
      payload: snapshot,
    });
  }

  private handleServerEnvelopeError(envelope: RuntimeEnvelope): void {
    const payload = envelope.payload as { code?: string; message?: string } | undefined;
    const err = new Error(`server error: ${payload?.code} ${payload?.message}`);
    this.hooks.onError(err);
  }

  private async sendCommandAck(
    commandId: string,
    commandSequence: number,
    status: CommandStatus,
    rejectErrorCode = "",
    rejectReason = "",
  ): Promise<void> {
    const payload: CommandAckPayload = {
      commandId,
      commandSequence,
      status,
      runtimeSessionId: this.sessionId,
      receivedAt: new Date().toISOString(),
      rejectErrorCode: rejectErrorCode || undefined,
      rejectReason: rejectReason || undefined,
    };
    await this.sendEnvelope("command_ack", "command_ack", payload);
  }

  private mapCommandExecutionStatus(status: string): CommandStatus {
    switch (status) {
      case "applied":
      case "duplicate":
        return "completed";
      case "accepted":
        return "renderer_accepted";
      case "failed":
      case "rejected":
      case "expired":
      case "cancelled":
      default:
        return "failed_terminal";
    }
  }

  private recordCommandResult(
    commandId: string,
    commandSequence: number,
    desiredRevision: number,
    result: RuntimeCommandExecutionResult,
    ackStatus: CommandStatus,
  ): void {
    if (desiredRevision > 0 && (result.status === "applied" || result.status === "duplicate")) {
      this.lastAppliedDesiredRevision = Math.max(
        this.lastAppliedDesiredRevision,
        desiredRevision,
      );
      this.hooks.onDesiredSync(this.lastAppliedDesiredRevision);
    }

    if (isCommandTerminal(ackStatus)) {
      this.lastProcessedCommandSequence = Math.max(
        this.lastProcessedCommandSequence,
        commandSequence,
      );
      this.trackedCommandSequences.delete(commandId);
      return;
    }

    if (commandId && commandSequence > 0) {
      this.trackedCommandSequences.set(commandId, commandSequence);
    }
  }

  private markTrackedCommandProcessed(
    commandId: string,
    terminalStatus: "completed" | "failed_terminal" = "completed",
    errorCode = "",
    errorMessage = "",
  ): void {
    if (!commandId) return;
    const seq = this.trackedCommandSequences.get(commandId);
    if (typeof seq === "number") {
      this.lastProcessedCommandSequence = Math.max(this.lastProcessedCommandSequence, seq);
      this.trackedCommandSequences.delete(commandId);
    }
    const cached = this.commandReplayCache.get(commandId);
    if (cached) {
      cached.ackStatus = terminalStatus;
      cached.result = {
        ...cached.result,
        status: terminalStatus === "completed" ? "applied" : "failed",
        errorCode: errorCode || cached.result.errorCode,
        errorMessage: errorMessage || cached.result.errorMessage,
      };
      this.cacheCommandExecution(commandId, cached);
    }
  }

  private handleClose(code: number, reason: string): void {
    void code;
    void reason;
    this.stopHeartbeat();
    this.cleanupSocket();
    this.setState("disconnected");

    if (this.config.autoReconnect) {
      void this.attemptReconnect().catch((err) => {
        this.hooks.onError(err instanceof Error ? err : new Error(String(err)));
      });
    }
  }

  private async attemptReconnect(): Promise<void> {
    if (this.reconnectAttempts >= this.config.maxReconnectAttempts) {
      this.setState("disconnected");
      this.hooks.onError(new Error("max reconnect attempts exceeded"));
      return;
    }

    this.setState("reconnecting");
    this.reconnectAttempts += 1;

    const delay = Math.min(
      this.config.reconnectBaseDelayMs * Math.pow(2, this.reconnectAttempts - 1),
      this.config.reconnectMaxDelayMs,
    );

    await new Promise<void>((resolve) => {
      this.reconnectTimer = setTimeout(() => {
        this.reconnectTimer = null;
        resolve();
      }, delay);
    });

    try {
      await this.connect("transport_lost");
    } catch (err) {
      this.hooks.onError(err instanceof Error ? err : new Error(String(err)));
    }
  }

  private async sendHello(): Promise<void> {
    const payload = buildHelloPayload({
      deviceId: this.config.deviceId,
      runtimeId: this.config.runtimeId,
      capabilities: this.runtimeCapabilitiesList(),
      lastAppliedDesiredRevision: this.lastAppliedDesiredRevision,
      lastProcessedCommandSequence: this.lastProcessedCommandSequence,
      lastEventSequence: this.lastEventSequence,
      actualStateHash: this.actualStateHash || undefined,
      runtimeVersion: this.config.runtimeVersion,
    });
    await this.sendEnvelope("hello", "hello", payload, true);
  }

  private runtimeCapabilitiesList(): string[] {
    const caps: string[] = [];
    const c = this.config.capabilities;
    if (c.supportsHighDpi) caps.push("high_dpi");
    if (c.supportsHitTest) caps.push("hit_test");
    if (c.supportsShadow) caps.push("shadow");
    caps.push(`platform:${c.platform}`);
    return caps;
  }

  private setState(state: RuntimeHandlerState): void {
    if (this.state === state) return;
    this.state = state;
    this.hooks.onState(state);
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      if (this.isConnected()) {
        void this.sendEnvelope("ping", "ping", { t: Date.now() }).catch((err) => {
          this.hooks.onError(err instanceof Error ? err : new Error(String(err)));
        });
      }
    }, this.config.heartbeatIntervalMs);

    const watchdogIntervalMs = Math.max(1000, Math.min(
      this.config.heartbeatIntervalMs,
      Math.floor(this.config.heartbeatTimeoutMs / 3),
    ));
    this.idleHeartbeatTimer = setInterval(() => {
      if (!this.isConnected() || this.lastServerMessageAt <= 0) return;
      if (Date.now() - this.lastServerMessageAt <= this.config.heartbeatTimeoutMs) return;

      const err = new Error("runtime server heartbeat timeout");
      this.hooks.onError(err);
      this.stopHeartbeat();
      const ws = this.ws;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.close(4002, "heartbeat_timeout");
      }
    }, watchdogIntervalMs);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer !== null) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    if (this.idleHeartbeatTimer !== null) {
      clearInterval(this.idleHeartbeatTimer);
      this.idleHeartbeatTimer = null;
    }
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private cleanupSocket(): void {
    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.onmessage = null;
      if (this.ws.readyState === WebSocket.OPEN) {
        this.ws.close(1000, "cleanup");
      }
      this.ws = null;
    }
  }
}
