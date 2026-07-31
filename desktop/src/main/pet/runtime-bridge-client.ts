import { app } from "electron";
import { createHash, randomUUID } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import {
  CapPetClickThrough,
  CapPetInteractionEvents,
  CapPetRecenter,
  CapPetSettings,
  CapPetWindow,
  CapPetAnimationFrame,
  CapRuntimeCommandAck,
  CapRuntimeSnapshotV2,
  EvtRuntimeCommandAcknowledged,
  EvtRuntimeCommandRejected,
  type ClickPayload,
  type CommandAckPayload,
  type DragPayload,
  type OutboxPriority,
  type PlaybackPayload,
  type RuntimeErrorPayload,
  type RuntimeEventType,
  type RuntimeSessionContext,
  type WindowPayload,
  OutboxPriorityDroppable,
  OutboxPriorityMergeable,
  OutboxPriorityMustRetain,
  outboxPriorityForEvent,
  inferClickEventType,
  inferDragEventType,
  inferPlaybackEventType,
} from "../../shared/runtime-protocol";

const SCHEMA_VERSION = 1;
const PROTOCOL_VERSION = "1.0";
const SUBPROTOCOL = "amitia-desktop-pet.v1";
const HEARTBEAT_INTERVAL_MS = 10000;
const RECONNECT_BASE_DELAY_MS = 1000;
const RECONNECT_MAX_DELAY_MS = 30000;
const MESSAGE_SEQ_KEY = "_runtimeBridgeMsgSeq";
const MAX_MESSAGE_BYTES_DEFAULT = 1024 * 1024;
const MAX_EVENTS_PER_SECOND = 50;
const OUTBOX_MAX_SIZE = 500;
const INBOX_MAX_SIZE = 1000;
const INBOX_TTL_MS = 24 * 60 * 60 * 1000;

export type MessageKind = "control" | "command" | "result" | "event";

export interface RuntimeMessage {
  schemaVersion: number;
  protocolVersion: string;
  kind: MessageKind;
  name: string;
  messageId: string;
  requestId?: string;
  commandId?: string;
  correlationId?: string;
  causationId?: string;
  idempotencyKey?: string;
  runtimeId?: string;
  sessionId?: string;
  userId?: string;
  deviceId?: string;
  installationId?: string;
  petInstanceId?: string;
  petId?: string;
  releaseId?: string;
  runtimeInstanceId?: string;
  sequence?: number;
  sentAt: string;
  deadlineAt?: string;
  payload?: unknown;
}

export interface PetInstanceSummary {
  petInstanceId: string;
  installationId: string;
  visible: boolean;
  currentActionKey: string;
  positionX: number;
  positionY: number;
  screenId: string;
  scale: number;
}

export interface HeartbeatPayload {
  rendererHealthy: boolean;
  petInstances: PetInstanceSummary[];
  lastAppliedDesiredRevision: number;
  queueDepth: number;
  memoryUsageMB: number;
  errorSummary: string;
}

export interface RegisterPayload {
  runtimeId: string;
  deviceId: string;
  processInstanceId: string;
  appVersion: string;
  platform: string;
  arch: string;
  protocolMin: string;
  protocolMax: string;
  capabilities: string[];
  resumeSessionId: string;
  lastAppliedDesiredRevision: number;
  lastProcessedCommandSequence: number;
  lastSentEventSequence: number;
  pendingCommandIds: string[];
  challengeResponse: string;
}

export interface WelcomePayload {
  sessionId: string;
  selectedProtocol: string;
  backendInstanceId: string;
  heartbeatIntervalMs: number;
  heartbeatTimeoutMs: number;
  maxMessageBytes: number;
  fullSyncRequired: boolean;
  serverTime: string;
  runtimeInstanceId?: string;
  acceptedProtocolVersion?: string;
  currentDesiredRevision?: number;
  resumeMode?: string;
}

export interface SpawnPayload {
  desiredRevision: number;
  installation: {
    installationId: string;
    characterId: string;
    packageId: string;
    packageVersion: string;
    installRoot: string;
    manifestPath: string;
    packageHash: string;
    defaultActionKey: string;
    canvasWidth: number;
    canvasHeight: number;
  };
  settings: {
    revision: number;
    alwaysOnTop: boolean;
    scale: number;
    positionX: number;
    positionY: number;
    screenId: string;
    clickThroughMode: string;
    soundEnabled: boolean;
  };
}

export interface SyncPayload {
  desiredRevision: number;
  ensureAbsent: boolean;
  desiredPet?: SpawnPayload;
}

export interface CommandResultPayload {
  commandId: string;
  status: "accepted" | "applied" | "rejected" | "failed" | "duplicate" | "expired" | "cancelled";
  errorCode: string;
  errorMessage: string;
  appliedRevision: number;
  actualState?: PetInstanceSummary;
  acceptedAction?: string;
  playbackRequestId?: string;
}

export interface RuntimeBridgeConfig {
  endpoint: string;
  token: string;
  runtimeId?: string;
  deviceId?: string;
  appVersion?: string;
  capabilities?: string[];
}

export interface RuntimeBridgeCallbacks {
  onSpawn?: (msg: RuntimeMessage, payload: SpawnPayload) => Promise<CommandResultPayload>;
  onDestroy?: (msg: RuntimeMessage, desiredRevision: number, reason: string) => Promise<CommandResultPayload>;
  onShow?: (msg: RuntimeMessage, desiredRevision: number) => Promise<CommandResultPayload>;
  onHide?: (msg: RuntimeMessage, desiredRevision: number) => Promise<CommandResultPayload>;
  onPlayAction?: (msg: RuntimeMessage, actionKey: string, actionSpecHash: string) => Promise<CommandResultPayload>;
  onUpdateSettings?: (msg: RuntimeMessage, settingsRevision: number, settings: SpawnPayload["settings"]) => Promise<CommandResultPayload>;
  onRecenter?: (msg: RuntimeMessage, settingsRevision: number, screenId: string) => Promise<CommandResultPayload>;
  onSync?: (msg: RuntimeMessage, payload: SyncPayload) => Promise<CommandResultPayload>;
  onStateProbe?: (msg: RuntimeMessage) => PetInstanceSummary[];
  onShutdown?: (msg: RuntimeMessage, deadline: string, reason: string) => void;
  onConnected?: (welcome: WelcomePayload) => void;
  onDisconnected?: (reason: string) => void;
  onError?: (err: Error) => void;
}

interface InboxEntry {
  commandId: string;
  commandSequence: number;
  status: string;
  resultHash: string;
  result: CommandResultPayload | null;
  receivedAt: number;
}

interface OutboxEntry {
  message: RuntimeMessage;
  eventType: RuntimeEventType;
  priority: OutboxPriority;
  enqueuedAt: number;
}

let messageSeq = 0;

function nextMessageId(): string {
  return randomUUID();
}

function nextSequence(): number {
  messageSeq += 1;
  return messageSeq;
}

function getRuntimeId(): string {
  const dataDir = app.getPath("userData");
  const idFile = join(dataDir, "runtime-id.txt");
  try {
    if (existsSync(idFile)) {
      const id = readFileSync(idFile, "utf8").trim();
      if (id) return id;
    }
    if (!existsSync(dataDir)) {
      mkdirSync(dataDir, { recursive: true });
    }
    const newId = "rt_" + randomUUID();
    writeFileSync(idFile, newId, "utf8");
    return newId;
  } catch {
    return "rt_" + randomUUID();
  }
}

function getDeviceId(): string {
  const dataDir = app.getPath("userData");
  const idFile = join(dataDir, "device-id.txt");
  try {
    if (existsSync(idFile)) {
      const id = readFileSync(idFile, "utf8").trim();
      if (id) return id;
    }
    if (!existsSync(dataDir)) {
      mkdirSync(dataDir, { recursive: true });
    }
    const newId = "dev_" + randomUUID();
    writeFileSync(idFile, newId, "utf8");
    return newId;
  } catch {
    return "dev_" + randomUUID();
  }
}

export class RuntimeBridgeClient {
  private ws: WebSocket | null = null;
  private config: RuntimeBridgeConfig;
  private callbacks: RuntimeBridgeCallbacks;
  private runtimeId: string;
  private deviceId: string;
  private sessionId: string | null = null;
  private connected = false;
  private intentionallyClosed = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectAttempts = 0;
  private lastAppliedDesiredRevision = 0;
  private lastProcessedCommandSequence = 0;
  private lastSentEventSequence = 0;
  private petInstances: PetInstanceSummary[] = [];
  private rendererHealthy = true;
  private maxMessageBytes = MAX_MESSAGE_BYTES_DEFAULT;

  private sessionContext: RuntimeSessionContext | null = null;
  private commandInbox: Map<string, InboxEntry> = new Map();
  private eventOutbox: OutboxEntry[] = [];
  private eventTimestamps: number[] = [];
  private lastWindowVisible: boolean | null = null;

  constructor(config: RuntimeBridgeConfig, callbacks: RuntimeBridgeCallbacks = {}) {
    this.config = config;
    this.callbacks = callbacks;
    this.runtimeId = config.runtimeId || getRuntimeId();
    this.deviceId = config.deviceId || getDeviceId();
  }

  connect(): void {
    this.intentionallyClosed = false;
    this.doConnect();
  }

  disconnect(): void {
    this.intentionallyClosed = true;
    this.cleanupTimers();
    if (this.ws) {
      try {
        this.ws.close(1000, "client disconnect");
      } catch {
        void 0;
      }
      this.ws = null;
    }
    this.connected = false;
    this.sessionId = null;
    this.sessionContext = null;
  }

  isConnected(): boolean {
    return this.connected;
  }

  getSessionId(): string | null {
    return this.sessionId;
  }

  getRuntimeId(): string {
    return this.runtimeId;
  }

  getDeviceId(): string {
    return this.deviceId;
  }

  updatePetInstances(instances: PetInstanceSummary[]): void {
    this.petInstances = instances;
  }

  setRendererHealthy(healthy: boolean): void {
    this.rendererHealthy = healthy;
  }

  setLastAppliedDesiredRevision(rev: number): void {
    this.lastAppliedDesiredRevision = rev;
  }

  setSessionContext(ctx: RuntimeSessionContext): void {
    this.sessionContext = ctx;
  }

  reportClick(payload: ClickPayload): void {
    const eventType = inferClickEventType(payload);
    this.sendTypedEvent(eventType, payload);
  }

  reportDrag(payload: DragPayload): void {
    const eventType = inferDragEventType(payload);
    this.sendTypedEvent(eventType, payload);
  }

  reportPlayback(payload: PlaybackPayload): void {
    const eventType = inferPlaybackEventType(payload);
    this.sendTypedEvent(eventType, payload);
  }

  reportWindow(payload: WindowPayload): void {
    let eventType: RuntimeEventType;
    if (this.lastWindowVisible !== null) {
      if (payload.visible && !this.lastWindowVisible) {
        eventType = "window.shown";
      } else if (!payload.visible && this.lastWindowVisible) {
        eventType = "window.hidden";
      } else {
        eventType = "window.moved";
      }
    } else {
      eventType = payload.visible ? "window.shown" : "window.hidden";
    }
    this.lastWindowVisible = payload.visible;
    this.sendTypedEvent(eventType, payload);
  }

  reportRuntimeError(payload: RuntimeErrorPayload): void {
    this.sendTypedEvent("runtime.disconnected", payload);
  }

  sendCommandResult(result: CommandResultPayload, originalMsg?: RuntimeMessage): void {
    if (!this.connected || !this.ws) {
      const msg = this.buildMessage("result", "runtime.result", result);
      if (originalMsg) {
        msg.commandId = originalMsg.commandId;
        msg.correlationId = originalMsg.messageId;
        msg.runtimeId = originalMsg.runtimeId || this.runtimeId;
        msg.sessionId = originalMsg.sessionId || this.sessionId || undefined;
      }
      this.enqueueOutbox(msg, EvtRuntimeCommandAcknowledged, OutboxPriorityMustRetain);
      return;
    }
    const msg = this.buildMessage("result", "runtime.result", result);
    if (originalMsg) {
      msg.commandId = originalMsg.commandId;
      msg.correlationId = originalMsg.messageId;
      msg.runtimeId = originalMsg.runtimeId || this.runtimeId;
      msg.sessionId = originalMsg.sessionId || this.sessionId || undefined;
    }
    if (originalMsg?.commandId) {
      this.updateInboxStatus(originalMsg.commandId, result.status, result);
    }
    this.sendMessage(msg);
  }

  private doConnect(): void {
    if (this.intentionallyClosed) return;

    const subprotocols = [SUBPROTOCOL, `amitia-runtime.${this.config.token}`];
    try {
      this.ws = new WebSocket(this.config.endpoint, subprotocols);
    } catch (err) {
      this.handleError(err as Error);
      this.scheduleReconnect();
      return;
    }

    this.ws.addEventListener("open", () => {
      this.reconnectAttempts = 0;
      this.sendRegister();
    });

    this.ws.addEventListener("message", (event: MessageEvent) => {
      this.handleMessage(event.data);
    });

    this.ws.addEventListener("close", (event: CloseEvent) => {
      this.handleDisconnect(event.reason || `code=${event.code}`);
    });

    this.ws.addEventListener("error", () => {
      this.handleError(new Error("WebSocket error"));
    });
  }

  private sendRegister(): void {
    const pendingCommandIds: string[] = [];
    for (const [cmdId, entry] of this.commandInbox) {
      if (entry.status === "accepted" || entry.status === "started" || entry.status === "received") {
        pendingCommandIds.push(cmdId);
      }
    }

    const payload: RegisterPayload = {
      runtimeId: this.runtimeId,
      deviceId: this.deviceId,
      processInstanceId: process.pid.toString(),
      appVersion: this.config.appVersion || app.getVersion(),
      platform: process.platform,
      arch: process.arch,
      protocolMin: PROTOCOL_VERSION,
      protocolMax: PROTOCOL_VERSION,
      capabilities: this.config.capabilities || [
        CapPetWindow,
        CapPetAnimationFrame,
        CapPetSettings,
        CapPetRecenter,
        CapPetClickThrough,
        CapPetInteractionEvents,
        CapRuntimeCommandAck,
        CapRuntimeSnapshotV2,
      ],
      resumeSessionId: this.sessionId || "",
      lastAppliedDesiredRevision: this.lastAppliedDesiredRevision,
      lastProcessedCommandSequence: this.lastProcessedCommandSequence,
      lastSentEventSequence: this.lastSentEventSequence,
      pendingCommandIds,
      challengeResponse: "",
    };
    const msg = this.buildMessage("control", "runtime.register", payload);
    this.sendMessage(msg);
  }

  private handleMessage(raw: unknown): void {
    let msg: RuntimeMessage;
    try {
      const text = typeof raw === "string" ? raw : JSON.stringify(raw);
      msg = JSON.parse(text) as RuntimeMessage;
    } catch {
      return;
    }

    switch (msg.kind) {
      case "control":
        this.handleControl(msg);
        break;
      case "command":
        void this.handleCommand(msg);
        break;
      default:
        break;
    }
  }

  private handleControl(msg: RuntimeMessage): void {
    switch (msg.name) {
      case "runtime.welcome":
        this.handleWelcome(msg);
        break;
      case "runtime.sync":
        void this.handleSync(msg);
        break;
      case "runtime.state_probe":
        this.handleStateProbe(msg);
        break;
      case "control.shutdown":
        this.handleShutdown(msg);
        break;
      case "control.superseded":
        this.handleSuperseded(msg);
        break;
      default:
        break;
    }
  }

  private handleWelcome(msg: RuntimeMessage): void {
    const payload = msg.payload as WelcomePayload;
    if (!payload) return;

    const previousSessionId = this.sessionId;
    this.sessionId = payload.sessionId;
    this.connected = true;

    if (typeof payload.maxMessageBytes === "number" && payload.maxMessageBytes > 0) {
      this.maxMessageBytes = payload.maxMessageBytes;
    }

    this.startHeartbeat(payload.heartbeatIntervalMs || HEARTBEAT_INTERVAL_MS);

    const resumeMode = payload.resumeMode || "";
    if (resumeMode === "session_reset") {
      this.commandInbox.clear();
      this.eventOutbox = [];
      this.lastSentEventSequence = 0;
      this.lastProcessedCommandSequence = 0;
    } else if (resumeMode === "full_resync") {
      this.eventOutbox = [];
    } else if (resumeMode === "resume" && previousSessionId === payload.sessionId) {
      void this.flushOutbox();
    }

    if (this.callbacks.onConnected) {
      this.callbacks.onConnected(payload);
    }

    if (resumeMode === "resume" || resumeMode === "full_resync") {
      void this.flushOutbox();
    }
  }

  private async handleSync(msg: RuntimeMessage): Promise<void> {
    const payload = msg.payload as SyncPayload;
    if (!payload || !this.callbacks.onSync) {
      this.sendCommandResult(
        {
          commandId: "sync_" + (this.sessionId || ""),
          status: "rejected",
          errorCode: "SYNC_HANDLER_MISSING",
          errorMessage: "sync handler not registered",
          appliedRevision: 0,
        },
        msg,
      );
      return;
    }
    try {
      const result = await this.callbacks.onSync(msg, payload);
      this.lastAppliedDesiredRevision = result.appliedRevision || payload.desiredRevision;
      this.sendCommandResult(result, msg);
    } catch (err) {
      this.sendCommandResult(
        {
          commandId: "sync_" + (this.sessionId || ""),
          status: "failed",
          errorCode: "SYNC_FAILED",
          errorMessage: (err as Error).message,
          appliedRevision: 0,
        },
        msg,
      );
    }
  }

  private handleStateProbe(msg: RuntimeMessage): void {
    if (this.callbacks.onStateProbe) {
      const instances = this.callbacks.onStateProbe(msg);
      this.petInstances = instances;
    }
    const result: CommandResultPayload = {
      commandId: "state_probe_" + (this.sessionId || ""),
      status: "applied",
      errorCode: "",
      errorMessage: "",
      appliedRevision: this.lastAppliedDesiredRevision,
      actualState: this.petInstances[0],
    };
    this.sendCommandResult(result, msg);
  }

  private handleShutdown(msg: RuntimeMessage): void {
    const payload = msg.payload as { deadline: string; reason: string } | undefined;
    if (this.callbacks.onShutdown) {
      this.callbacks.onShutdown(msg, payload?.deadline || "", payload?.reason || "");
    }
    this.disconnect();
  }

  private handleSuperseded(msg: RuntimeMessage): void {
    this.connected = false;
    this.cleanupTimers();
    this.sessionId = null;
    this.sessionContext = null;
    this.scheduleReconnect();
  }

  private async handleCommand(msg: RuntimeMessage): Promise<void> {
    const commandId = msg.commandId || "";
    const commandSequence = msg.sequence || 0;

    if (commandSequence) {
      this.lastProcessedCommandSequence = commandSequence;
    }

    if (commandId) {
      const existing = this.commandInbox.get(commandId);
      if (existing) {
        const payloadHash = this.hashPayload(msg.payload);
        if (existing.resultHash !== payloadHash) {
          this.sendCommandAck(commandId, commandSequence, "rejected", "command_id_conflict");
          return;
        }
        if (
          existing.status === "completed" ||
          existing.status === "rejected" ||
          existing.status === "failed"
        ) {
          if (existing.result) {
            this.sendCommandResult(existing.result, msg);
          }
          this.sendCommandAck(commandId, commandSequence, existing.status);
          return;
        }
        this.sendCommandAck(commandId, commandSequence, existing.status);
        return;
      }

      const payloadHash = this.hashPayload(msg.payload);
      this.commandInbox.set(commandId, {
        commandId,
        commandSequence,
        status: "received",
        resultHash: payloadHash,
        result: null,
        receivedAt: Date.now(),
      });
      this.pruneInbox();
    }

    this.sendCommandAck(commandId, commandSequence, "received");

    let result: CommandResultPayload;
    try {
      this.updateInboxStatus(commandId, "accepted", null);
      this.sendCommandAck(commandId, commandSequence, "accepted");
      result = await this.dispatchCommand(msg);
    } catch (err) {
      result = {
        commandId: commandId,
        status: "failed",
        errorCode: "COMMAND_FAILED",
        errorMessage: (err as Error).message,
        appliedRevision: this.lastAppliedDesiredRevision,
      };
    }

    this.updateInboxStatus(commandId, result.status, result);
    this.sendCommandResult(result, msg);
  }

  private async dispatchCommand(msg: RuntimeMessage): Promise<CommandResultPayload> {
    const payload = msg.payload as Record<string, unknown> | undefined;

    switch (msg.name) {
      case "pet.spawn": {
        if (this.callbacks.onSpawn) {
          return await this.callbacks.onSpawn(msg, payload as unknown as SpawnPayload);
        }
        break;
      }
      case "pet.destroy": {
        if (this.callbacks.onDestroy) {
          const rev = Number(payload?.desiredRevision || 0);
          const reason = String(payload?.reason || "");
          return await this.callbacks.onDestroy(msg, rev, reason);
        }
        break;
      }
      case "pet.show": {
        if (this.callbacks.onShow) {
          const rev = Number(payload?.desiredRevision || 0);
          return await this.callbacks.onShow(msg, rev);
        }
        break;
      }
      case "pet.hide": {
        if (this.callbacks.onHide) {
          const rev = Number(payload?.desiredRevision || 0);
          return await this.callbacks.onHide(msg, rev);
        }
        break;
      }
      case "pet.play_action": {
        if (this.callbacks.onPlayAction) {
          const actionKey = String(payload?.actionKey || "");
          const actionSpecHash = String(payload?.actionSpecHash || "");
          return await this.callbacks.onPlayAction(msg, actionKey, actionSpecHash);
        }
        break;
      }
      case "pet.update_settings": {
        if (this.callbacks.onUpdateSettings) {
          const settingsRevision = Number(payload?.settingsRevision || 0);
          const settings = payload?.settings as SpawnPayload["settings"];
          return await this.callbacks.onUpdateSettings(msg, settingsRevision, settings);
        }
        break;
      }
      case "pet.recenter": {
        if (this.callbacks.onRecenter) {
          const settingsRevision = Number(payload?.settingsRevision || 0);
          const screenId = String(payload?.screenId || "");
          return await this.callbacks.onRecenter(msg, settingsRevision, screenId);
        }
        break;
      }
      default:
        break;
    }

    return {
      commandId: msg.commandId || "",
      status: "rejected",
      errorCode: "UNKNOWN_COMMAND",
      errorMessage: `unknown command: ${msg.name}`,
      appliedRevision: this.lastAppliedDesiredRevision,
    };
  }

  private startHeartbeat(intervalMs: number): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
    }
    this.heartbeatTimer = setInterval(() => {
      this.sendHeartbeat();
    }, intervalMs);
    if (typeof (this.heartbeatTimer as NodeJS.Timeout).unref === "function") {
      (this.heartbeatTimer as NodeJS.Timeout).unref();
    }
  }

  private sendHeartbeat(): void {
    if (!this.connected || !this.ws) return;
    const payload: HeartbeatPayload = {
      rendererHealthy: this.rendererHealthy,
      petInstances: this.petInstances,
      lastAppliedDesiredRevision: this.lastAppliedDesiredRevision,
      queueDepth: this.eventOutbox.length,
      memoryUsageMB: Math.round(process.memoryUsage().rss / 1024 / 1024),
      errorSummary: "",
    };
    const msg = this.buildMessage("control", "runtime.heartbeat", payload);
    this.sendMessage(msg);
  }

  private buildMessage(kind: MessageKind, name: string, payload: unknown): RuntimeMessage {
    const msg: RuntimeMessage = {
      schemaVersion: SCHEMA_VERSION,
      protocolVersion: PROTOCOL_VERSION,
      kind,
      name,
      messageId: nextMessageId(),
      runtimeId: this.runtimeId,
      sessionId: this.sessionId || undefined,
      sequence: nextSequence(),
      sentAt: new Date().toISOString(),
      payload,
    };
    if (this.sessionContext) {
      msg.userId = this.sessionContext.userId;
      msg.deviceId = this.sessionContext.deviceId;
      msg.installationId = this.sessionContext.installationId;
      msg.petInstanceId = this.sessionContext.petId;
      msg.petId = this.sessionContext.petId;
      msg.releaseId = this.sessionContext.releaseId;
      msg.runtimeInstanceId = this.sessionContext.runtimeInstanceId;
    }
    return msg;
  }

  private sendMessage(msg: RuntimeMessage): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    try {
      const data = JSON.stringify(msg);
      if (data.length > this.maxMessageBytes) {
        return;
      }
      this.ws.send(data);
    } catch {
      void 0;
    }
  }

  private sendTypedEvent(eventType: RuntimeEventType, payload: unknown): void {
    const priority = outboxPriorityForEvent(eventType);

    if (!this.canSendEvent(priority)) {
      return;
    }

    const eventSequence = this.nextEventSequence();
    const now = new Date().toISOString();
    const ctx = this.sessionContext;
    const envelopePayload = {
      protocolVersion: PROTOCOL_VERSION,
      eventId: nextMessageId(),
      eventType,
      eventSequence,
      userId: ctx?.userId ?? "",
      deviceId: ctx?.deviceId ?? "",
      installationId: ctx?.installationId ?? "",
      petId: ctx?.petId ?? "",
      releaseId: ctx?.releaseId ?? "",
      runtimeInstanceId: ctx?.runtimeInstanceId ?? this.runtimeId,
      occurredAt: now,
      sentAt: now,
      payload,
    };

    const msg = this.buildMessage("event", "runtime.event", envelopePayload);

    if (!this.connected || !this.ws || this.ws.readyState !== WebSocket.OPEN) {
      this.enqueueOutbox(msg, eventType, priority);
      return;
    }

    this.sendMessage(msg);
  }

  private sendCommandAck(
    commandId: string,
    commandSequence: number,
    status: string,
    rejectReason?: string,
  ): void {
    const ackPayload: CommandAckPayload = {
      commandId,
      commandSequence,
      status,
      runtimeInstanceId: this.sessionContext?.runtimeInstanceId ?? this.runtimeId,
      receivedAt: new Date().toISOString(),
      rejectReason,
    };
    const eventType =
      status === "rejected"
        ? EvtRuntimeCommandRejected
        : EvtRuntimeCommandAcknowledged;
    this.sendTypedEvent(eventType, ackPayload);
  }

  private nextEventSequence(): number {
    this.lastSentEventSequence += 1;
    return this.lastSentEventSequence;
  }

  private canSendEvent(priority: OutboxPriority): boolean {
    const now = Date.now();
    const oneSecondAgo = now - 1000;
    this.eventTimestamps = this.eventTimestamps.filter((t) => t > oneSecondAgo);

    if (this.eventTimestamps.length >= MAX_EVENTS_PER_SECOND) {
      if (priority === OutboxPriorityDroppable) {
        return false;
      }
    }

    this.eventTimestamps.push(now);
    return true;
  }

  private hashPayload(payload: unknown): string {
    try {
      const str = JSON.stringify(payload ?? {});
      return createHash("sha256").update(str).digest("hex").slice(0, 16);
    } catch {
      return "";
    }
  }

  private updateInboxStatus(
    commandId: string,
    status: string,
    result: CommandResultPayload | null,
  ): void {
    const entry = this.commandInbox.get(commandId);
    if (!entry) return;
    entry.status = status;
    if (result) {
      entry.result = result;
    }
  }

  private pruneInbox(): void {
    if (this.commandInbox.size <= INBOX_MAX_SIZE) return;
    const now = Date.now();
    const expired: string[] = [];
    for (const [cmdId, entry] of this.commandInbox) {
      if (now - entry.receivedAt > INBOX_TTL_MS) {
        expired.push(cmdId);
      }
    }
    for (const cmdId of expired) {
      this.commandInbox.delete(cmdId);
    }
    if (this.commandInbox.size <= INBOX_MAX_SIZE) return;
    const sorted = Array.from(this.commandInbox.entries()).sort(
      (a, b) => a[1].receivedAt - b[1].receivedAt,
    );
    const toRemove = sorted.length - INBOX_MAX_SIZE;
    for (let i = 0; i < toRemove; i++) {
      this.commandInbox.delete(sorted[i][0]);
    }
  }

  private enqueueOutbox(
    message: RuntimeMessage,
    eventType: RuntimeEventType,
    priority: OutboxPriority,
  ): void {
    if (
      priority === OutboxPriorityMustRetain &&
      this.eventOutbox.length >= OUTBOX_MAX_SIZE
    ) {
      this.evictDroppableAndMergeable();
    }

    if (priority === OutboxPriorityMergeable) {
      const lastIdx = this.eventOutbox.length - 1;
      if (lastIdx >= 0) {
        const last = this.eventOutbox[lastIdx];
        if (last.eventType === eventType && last.priority === OutboxPriorityMergeable) {
          last.message = message;
          last.enqueuedAt = Date.now();
          return;
        }
      }
    }

    if (this.eventOutbox.length >= OUTBOX_MAX_SIZE) {
      if (priority === OutboxPriorityDroppable) {
        return;
      }
      this.evictDroppableAndMergeable();
      if (this.eventOutbox.length >= OUTBOX_MAX_SIZE) {
        return;
      }
    }

    this.eventOutbox.push({
      message,
      eventType,
      priority,
      enqueuedAt: Date.now(),
    });
  }

  private evictDroppableAndMergeable(): void {
    for (let i = this.eventOutbox.length - 1; i >= 0; i--) {
      const entry = this.eventOutbox[i];
      if (
        entry.priority === OutboxPriorityDroppable ||
        entry.priority === OutboxPriorityMergeable
      ) {
        this.eventOutbox.splice(i, 1);
        if (this.eventOutbox.length < OUTBOX_MAX_SIZE) {
          break;
        }
      }
    }
  }

  private async flushOutbox(): Promise<void> {
    if (this.eventOutbox.length === 0) return;
    if (!this.connected || !this.ws || this.ws.readyState !== WebSocket.OPEN) return;

    const ordered = this.eventOutbox.sort((a, b) => {
      if (a.priority !== b.priority) {
        return a.priority - b.priority;
      }
      return a.enqueuedAt - b.enqueuedAt;
    });

    const toSend = ordered.splice(0);
    this.eventOutbox = [];

    for (const entry of toSend) {
      if (
        !this.connected ||
        !this.ws ||
        this.ws.readyState !== WebSocket.OPEN
      ) {
        this.eventOutbox.push(entry);
        continue;
      }
      this.sendMessage(entry.message);
    }
  }

  private handleDisconnect(reason: string): void {
    this.connected = false;
    this.cleanupTimers();
    if (this.callbacks.onDisconnected) {
      this.callbacks.onDisconnected(reason);
    }
    if (!this.intentionallyClosed) {
      this.scheduleReconnect();
    }
  }

  private handleError(err: Error): void {
    if (this.callbacks.onError) {
      this.callbacks.onError(err);
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
    }
    this.reconnectAttempts += 1;
    const delay = Math.min(
      RECONNECT_BASE_DELAY_MS * Math.pow(2, this.reconnectAttempts - 1),
      RECONNECT_MAX_DELAY_MS,
    );
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.doConnect();
    }, delay);
    if (typeof (this.reconnectTimer as NodeJS.Timeout).unref === "function") {
      (this.reconnectTimer as NodeJS.Timeout).unref();
    }
  }

  private cleanupTimers(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}

export { getRuntimeId, getDeviceId };
