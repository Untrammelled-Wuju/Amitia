import type {
  RuntimeEnvelope,
  HelloPayload,
  HelloAckPayload,
  CommandAckPayload,
  PlaybackEventPayload,
  StateSnapshotPayload,
  CommandStatus,
  SessionStatus,
  RuntimeMessageType,
} from "./protocol-v2";
import {
  buildHelloPayload,
  buildEnvelope,
  isCommandTerminal,
  computePayloadHash,
} from "./protocol-v2";
import type { RuntimeCommandExecutionResult } from "../../main/pet/runtime-v2-command-adapter";

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

export interface RuntimeHandlerConfig {
  url: string;
  userId: string;
  deviceId: string;
  runtimeId: string;
  contractVersion?: string;
  runtimeVersion?: string;
  capabilities?: RuntimeCapabilities;
  autoReconnect?: boolean;
  connectTimeoutMs?: number;
  heartbeatIntervalMs?: number;
  maxReconnectAttempts?: number;
  reconnectBaseDelayMs?: number;
  reconnectMaxDelayMs?: number;
}

export interface RuntimeHandlerHooks {
  onState: (state: RuntimeHandlerState) => void;
  onHelloAck: (ack: HelloAckPayload) => void;
  onEvent: (envelope: RuntimeEnvelope) => void;
  onError: (err: Error) => void;
  onDesiredSync: (revision: number) => void;
  onCommand: (command: unknown, envelope: RuntimeEnvelope) => Promise<RuntimeCommandExecutionResult>;
}

export interface RuntimeCommandAttempt {
  commandId: string;
  idempotencyKey: string;
  status: CommandStatus;
  attemptedAt: number;
}

const DEFAULT_HEARTBEAT_MS = 15000;
const DEFAULT_CONNECT_TIMEOUT_MS = 10000;
const DEFAULT_MAX_RECONNECT = 5;
const DEFAULT_RECONNECT_BASE_MS = 1000;
const DEFAULT_RECONNECT_MAX_MS = 30000;

export class DesktopRuntimeHandlerV2 {
  private readonly config: Required<RuntimeHandlerConfig>;
  private readonly hooks: RuntimeHandlerHooks;

  private ws: WebSocket | null = null;
  private state: RuntimeHandlerState = "disconnected";
  private sessionId = "";
  private connectionGeneration = 0;
  private eventSequence = 0;
  private commandSequence = 0;
  private lastAppliedDesiredRevision = 0;
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private idleHeartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private lastServerTime = "";

  private readonly pendingCommands = new Map<string, RuntimeCommandAttempt>();
  private reconnectReason: ReconnectReason = "initial";

  constructor(config: RuntimeHandlerConfig, hooks: RuntimeHandlerHooks) {
    this.config = {
      url: config.url,
      userId: config.userId,
      deviceId: config.deviceId,
      runtimeId: config.runtimeId,
      contractVersion: config.contractVersion ?? "2.0.0",
      runtimeVersion: config.runtimeVersion ?? "2.0.0",
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
      maxReconnectAttempts: config.maxReconnectAttempts ?? DEFAULT_MAX_RECONNECT,
      reconnectBaseDelayMs: config.reconnectBaseDelayMs ?? DEFAULT_RECONNECT_BASE_MS,
      reconnectMaxDelayMs: config.reconnectMaxDelayMs ?? DEFAULT_RECONNECT_MAX_MS,
    };
    this.hooks = hooks;
  }

  getState(): RuntimeHandlerState {
    return this.state;
  }

  getSessionId(): string {
    return this.sessionId;
  }

  getEventSequence(): number {
    return this.eventSequence;
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

        const ws = new WebSocket(this.config.url);
        this.ws = ws;

        const timeoutId = setTimeout(() => {
          ws.close(4000, "connect_timeout");
          reject(new Error("runtime connect timeout"));
        }, this.config.connectTimeoutMs);

        ws.onopen = () => {
          clearTimeout(timeoutId);
          void this.sendHello();
        };

        ws.onmessage = (event: MessageEvent) => {
          this.handleMessage(event.data)
            .catch((err) => this.hooks.onError(err instanceof Error ? err : new Error(String(err))));
        };

        ws.onerror = () => {
          clearTimeout(timeoutId);
          if (this.state === "handshaking") {
            reject(new Error("runtime socket error"));
          }
        };

        ws.onclose = (event) => {
          clearTimeout(timeoutId);
          this.handleClose(event.code, event.reason);
          resolve();
        };

        this.startHeartbeat();
      } catch (err) {
        this.setState("degraded");
        reject(err);
      }
    });
  }

  disconnect(): void {
    this.stopHeartbeat();
    this.cleanupSocket();
    this.setState("disconnected");
  }

  async sendPlaybackStarted(playbackId: string, commandId: string): Promise<void> {
    const payload: PlaybackEventPayload = {
      type: "playback.started",
      playbackInstanceId: playbackId,
      commandId,
      startedAt: new Date().toISOString(),
      occurredAt: new Date().toISOString(),
    };
    await this.sendRuntimeEvent("playback.started", payload);
  }

  async sendPlaybackEnded(
    playbackId: string,
    commandId: string,
    actionKey: string,
    playedMs: number,
    completionReason: string,
  ): Promise<void> {
    const payload: PlaybackEventPayload = {
      type: "playback.ended",
      playbackInstanceId: playbackId,
      commandId,
      actionKey,
      playedMs,
      completionReason,
      completedAt: new Date().toISOString(),
      occurredAt: new Date().toISOString(),
    };
    await this.sendRuntimeEvent("playback.ended", payload);
  }

  async sendPlaybackFailed(
    playbackId: string,
    commandId: string,
    actionKey: string,
    errorCode: string,
    errorMessage: string,
  ): Promise<void> {
    const payload: PlaybackEventPayload = {
      type: "playback.failed",
      playbackInstanceId: playbackId,
      commandId,
      actionKey,
      errorCode,
      errorMessage,
      failedAt: new Date().toISOString(),
      occurredAt: new Date().toISOString(),
    };
    await this.sendRuntimeEvent("playback.failed", payload);
  }

  async sendRendererState(snapshot: StateSnapshotPayload): Promise<void> {
    await this.sendRuntimeEvent("render.state", snapshot);
  }

  async sendRendererHealth(healthy: boolean, errorCode?: string): Promise<void> {
    await this.sendRuntimeEvent("render.health", {
      healthy,
      errorCode,
      occurredAt: new Date().toISOString(),
    });
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
    this.commandSequence += 1;
    const envelope = buildEnvelope(
      type,
      name,
      this.config.userId,
      this.config.deviceId,
      this.config.runtimeId,
      this.sessionId,
      Math.max(1, this.connectionGeneration),
      this.commandSequence,
      payload,
    );
    ws.send(JSON.stringify(envelope));
  }

  async sendRuntimeEvent(name: string, payload: unknown): Promise<void> {
    await this.sendEnvelope("runtime_event", name, payload);
  }

  private async handleMessage(raw: unknown): Promise<void> {
    if (typeof raw !== "string") {
      return;
    }
    const envelope = JSON.parse(raw) as RuntimeEnvelope;
    if (envelope.protocol !== "amitia.desktop-pet.runtime") {
      return;
    }

    switch (envelope.messageType) {
      case "hello_ack":
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

  private async handleHelloAck(ack: HelloAckPayload): Promise<void> {
    if (!ack.accepted) {
      this.setState("degraded");
      this.hooks.onError(new Error(`hello rejected: ${ack.errorCode} ${ack.errorMessage}`));
      return;
    }

    this.sessionId = ack.sessionId ?? "";
    this.lastAppliedDesiredRevision = ack.currentDesiredRevision ?? 0;
    this.lastServerTime = ack.serverTime ?? "";
    this.reconnectAttempts = 0;
    this.setState("connected");

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

    await this.sendCommandAck(command.commandId, command.commandSequence ?? 0, "runtime_received");

    try {
      const result = await this.hooks.onCommand(envelope.payload, envelope);
      const ackStatus = this.mapCommandExecutionStatus(result.status);
      if (result.status === "failed" || result.status === "rejected") {
        await this.sendCommandAck(
          command.commandId,
          command.commandSequence ?? 0,
          ackStatus,
          result.errorCode || "RENDERER_REJECTED",
          result.errorMessage || "command execution failed",
        );
      } else {
        await this.sendCommandAck(command.commandId, command.commandSequence ?? 0, ackStatus);
      }
    } catch (err) {
      await this.sendCommandAck(
        command.commandId,
        command.commandSequence ?? 0,
        "failed_terminal",
        "RENDERER_REJECTED",
        err instanceof Error ? err.message : String(err),
      );
    }
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
    if (snapshot.eventSequence > this.eventSequence) {
      this.eventSequence = snapshot.eventSequence;
    }
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
      sequence: this.eventSequence,
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
        return "renderer_accepted";
      case "accepted":
        return "runtime_accepted";
      case "failed":
        return "failed_terminal";
      case "rejected":
        return "failed_terminal";
      case "duplicate":
        return "superseded";
      case "expired":
        return "expired";
      case "cancelled":
        return "cancelled";
      default:
        return "renderer_accepted";
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
      lastProcessedCommandSequence: this.commandSequence,
      lastEventSequence: this.eventSequence,
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
        void this.sendEnvelope("ping", "ping", { t: Date.now() }).catch(() => {});
      }
    }, this.config.heartbeatIntervalMs);
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
