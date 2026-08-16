import {
  type ClickPayload,
  type DragPayload,
  type PlaybackPayload,
  type RuntimeErrorPayload,
  type RuntimeSessionContext,
  type WindowPayload,
} from "../../shared/runtime-protocol";
import {
  DesktopRuntimeHandlerV2,
  type RuntimeHandlerConfig,
  type RuntimeHandlerHooks,
  type RuntimeHandlerState,
} from "../../desktop-pet/runtime/runtime-handler-v2";
import { randomUUID } from "node:crypto";
import type { RuntimeEnvelope } from "../../desktop-pet/runtime/protocol-v2";
import type { RuntimeCommandExecutionResult } from "./runtime-v2-command-adapter";
import { getRuntimeId, getDeviceId } from "./runtime-identity";

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
  bootstrapTicket: string;
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
  bootstrapTicket: string;
  userId: string;
  runtimeId: string;
  deviceId: string;
  appVersion?: string;
  capabilities?: string[];
}

type SpawnCallback = (msg: RuntimeMessage, payload: SpawnPayload) => Promise<CommandResultPayload>;
type SimpleCommandCallback = (msg: RuntimeMessage, desiredRevision: number) => Promise<CommandResultPayload>;
type PlayActionCallback = (msg: RuntimeMessage, actionKey: string, actionSpecHash: string) => Promise<CommandResultPayload>;
type UpdateSettingsCallback = (msg: RuntimeMessage, settingsRevision: number, settings: SpawnPayload["settings"]) => Promise<CommandResultPayload>;
type RecenterCallback = (msg: RuntimeMessage, settingsRevision: number, screenId: string) => Promise<CommandResultPayload>;
type SyncCallback = (msg: RuntimeMessage, payload: SyncPayload) => Promise<CommandResultPayload>;
type StateProbeCallback = (msg: RuntimeMessage) => PetInstanceSummary[];
type VoidCommandCallback = (msg: RuntimeMessage, desiredRevision: number, reason: string) => Promise<CommandResultPayload>;

export interface DisconnectNotification {
  reason: string;
}

export interface RuntimeBridgeCallbacks {
  onSpawn?: SpawnCallback;
  onDestroy?: VoidCommandCallback;
  onShow?: SimpleCommandCallback;
  onHide?: SimpleCommandCallback;
  onPlayAction?: PlayActionCallback;
  onUpdateSettings?: UpdateSettingsCallback;
  onRecenter?: RecenterCallback;
  onSync?: SyncCallback;
  onStateProbe?: StateProbeCallback;
  onShutdown?: (msg: RuntimeMessage, deadline: string, reason: string) => void;
  onConnected?: (welcome: WelcomePayload) => void;
  onDisconnected?: (notification: DisconnectNotification) => void;
  onError?: (err: Error) => void;
}

const SCHEMA_VERSION = 1;
const PROTOCOL_VERSION = "2.0";

let messageSeq = 0;

function nextMessageId(): string {
  return randomUUID();
}

function buildRuntimeMessage(
  kind: MessageKind,
  name: string,
  payload: unknown = {},
): RuntimeMessage {
  messageSeq += 1;
  return {
    schemaVersion: SCHEMA_VERSION,
    protocolVersion: PROTOCOL_VERSION,
    kind,
    name,
    messageId: nextMessageId(),
    sequence: messageSeq,
    sentAt: new Date().toISOString(),
    payload,
  };
}

export class RuntimeBridgeClient {
  private handler: DesktopRuntimeHandlerV2 | null = null;
  private config: RuntimeBridgeConfig;
  private callbacks: RuntimeBridgeCallbacks;
  private runtimeId: string;
  private deviceId: string;
  private sessionId: string | null = null;
  private connected = false;
  private lastAppliedDesiredRevision = 0;
  private petInstances: PetInstanceSummary[] = [];
  private rendererHealthy = true;
  private sessionContext: RuntimeSessionContext | null = null;
  private lastWindowVisible: boolean | null = null;

  constructor(config: RuntimeBridgeConfig, callbacks: RuntimeBridgeCallbacks = {}) {
    this.config = config;
    this.callbacks = callbacks;
    this.runtimeId = config.runtimeId || getRuntimeId();
    this.deviceId = config.deviceId || getDeviceId();
  }

  connect(): void {
    if (this.handler) {
      this.handler.disconnect();
      this.handler = null;
    }

    const handlerConfig: RuntimeHandlerConfig = {
      url: this.buildRuntimeV2URL(),
      userId: this.config.userId,
      deviceId: this.deviceId,
      runtimeId: this.runtimeId,
      contractVersion: "2.0.0",
      runtimeVersion: "2.0.0",
      autoReconnect: false,
      heartbeatIntervalMs: 15000,
      maxReconnectAttempts: 0,
    };

    const hooks = this.buildHandlerHooks();
    this.handler = new DesktopRuntimeHandlerV2(handlerConfig, hooks);

    void this.handler.connect("initial").catch((err) => {
      this.callbacks.onError?.(err instanceof Error ? err : new Error(String(err)));
    });
  }

  private buildRuntimeV2URL(): string {
    const url = new URL(this.config.endpoint);
    url.searchParams.set("ticket", this.config.bootstrapTicket);
    url.searchParams.set("deviceId", this.config.deviceId);
    url.searchParams.set("runtimeId", this.config.runtimeId);
    return url.toString();
  }

  disconnect(): void {
    if (this.handler) {
      this.handler.disconnect();
      this.handler = null;
    }
    this.connected = false;
    this.sessionId = null;
    this.sessionContext = null;
  }

  isConnected(): boolean {
    return this.handler?.isConnected() ?? false;
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
    if (this.handler) {
      void this.handler.sendRendererHealth(healthy).catch(() => {});
    }
  }

  setLastAppliedDesiredRevision(rev: number): void {
    this.lastAppliedDesiredRevision = rev;
  }

  setSessionContext(ctx: RuntimeSessionContext): void {
    this.sessionContext = ctx;
  }

  reportClick(payload: ClickPayload): void {
    if (!this.handler?.isConnected()) return;
    const clickType = payload.clickCount >= 2 ? "desktop.pet.double_clicked" : "desktop.pet.clicked";
    void this.handler.sendRendererHealth(this.rendererHealthy).catch(() => {});
    void this.sendEvent("interaction.click", {
      button: payload.button,
      clickCount: payload.clickCount,
      canvasX: payload.canvasX,
      canvasY: payload.canvasY,
      screenX: payload.screenX,
      screenY: payload.screenY,
      frameIndex: payload.frameIndex,
      actionKey: payload.actionKey,
      clickType,
      occurredAt: new Date().toISOString(),
    }).catch(() => {});
  }

  reportDrag(payload: DragPayload): void {
    if (!this.handler?.isConnected()) return;
    void this.sendEvent("interaction.drag", {
      dragId: payload.dragId,
      phase: payload.phase,
      startX: payload.startX,
      startY: payload.startY,
      currentX: payload.currentX,
      currentY: payload.currentY,
      deltaX: payload.deltaX,
      deltaY: payload.deltaY,
      displayId: payload.displayId,
      occurredAt: new Date().toISOString(),
    }).catch(() => {});
  }

  reportPlayback(payload: PlaybackPayload): void {
    if (!this.handler?.isConnected()) return;

    if (payload.startedAt) {
      void this.handler.sendPlaybackStarted(
        payload.playbackId,
        payload.commandId ?? "",
      ).catch(() => {});
    } else if (payload.completedAt) {
      void this.handler.sendPlaybackEnded(
        payload.playbackId,
        payload.commandId ?? "",
        payload.actionKey,
        0,
        "natural_end",
      ).catch(() => {});
    } else if (payload.interruptReason) {
      void this.handler.sendPlaybackEnded(
        payload.playbackId,
        payload.commandId ?? "",
        payload.actionKey,
        0,
        "interrupted",
      ).catch(() => {});
    } else if (payload.errorCode) {
      void this.handler.sendPlaybackFailed(
        payload.playbackId,
        payload.commandId ?? "",
        payload.actionKey,
        payload.errorCode,
        "playback error",
      ).catch(() => {});
    }
  }

  reportWindow(payload: WindowPayload): void {
    if (!this.handler?.isConnected()) return;
    void this.sendEvent("window.state", {
      visible: payload.visible,
      x: payload.x,
      y: payload.y,
      displayId: payload.displayId,
      width: payload.width,
      height: payload.height,
      occurredAt: new Date().toISOString(),
    }).catch(() => {});
  }

  reportRuntimeError(payload: RuntimeErrorPayload): void {
    if (!this.handler?.isConnected()) return;
    void this.sendEvent("runtime.error", {
      code: payload.errorCode,
      message: payload.errorMessage,
      commandId: payload.commandId,
      playbackId: payload.playbackId,
      actionKey: payload.actionKey,
      recoverable: payload.recoverable,
      occurredAt: new Date().toISOString(),
    }).catch(() => {});
  }

  private async sendEvent(name: string, payload: unknown): Promise<void> {
    if (!this.handler?.isConnected()) return;
    const evt = {
      type: name,
      occurredAt: new Date().toISOString(),
      ...(payload as Record<string, unknown>),
    };
    await this.handler.sendRuntimeEvent(name, evt);
  }

  private buildHandlerHooks(): RuntimeHandlerHooks {
    return {
      onState: (state: RuntimeHandlerState) => {
        if (state === "connected") {
          this.connected = true;
        } else if (state === "disconnected" || state === "degraded") {
          this.connected = false;
          this.callbacks.onDisconnected?.({
            reason: "runtime disconnected",
          });
        }
      },
      onHelloAck: (ack) => {
        this.sessionId = ack.sessionId ?? "";
        this.lastAppliedDesiredRevision = ack.currentDesiredRevision;
        this.connected = true;
        const welcome: WelcomePayload = {
          sessionId: ack.sessionId ?? "",
          selectedProtocol: "amitia.desktop-pet.runtime",
          backendInstanceId: "",
          heartbeatIntervalMs: 15000,
          heartbeatTimeoutMs: 30000,
          maxMessageBytes: 1024 * 1024,
          fullSyncRequired: !ack.resumeMode || ack.resumeMode === "full_resync",
          serverTime: ack.serverTime,
          acceptedProtocolVersion: "2.0.0",
          currentDesiredRevision: ack.currentDesiredRevision,
          resumeMode: ack.resumeMode as WelcomePayload["resumeMode"],
        };
        this.callbacks.onConnected?.(welcome);
      },
      onEvent: () => {
        void 0;
      },
      onError: (err: Error) => {
        this.callbacks.onError?.(err);
      },
      onDesiredSync: (revision: number) => {
        this.lastAppliedDesiredRevision = revision;
      },
      onCommand: async (command: unknown, envelope: RuntimeEnvelope): Promise<RuntimeCommandExecutionResult> => {
        return this.handleCommand(command, envelope);
      },
    };
  }

  private async handleCommand(command: unknown, envelope: RuntimeEnvelope): Promise<RuntimeCommandExecutionResult> {
    const cmd = command as {
      commandId?: string;
      commandType?: string;
      desiredRevision?: number;
      settingsRevision?: number;
      installationId?: string;
      payload?: {
        installation?: SpawnPayload["installation"];
        settings?: SpawnPayload["settings"];
        actionKey?: string;
        actionSpecHash?: string;
        ensureAbsent?: boolean;
        desiredPet?: SpawnPayload;
        screenId?: string;
        reason?: string;
        deadline?: string;
      };
    };

    if (!cmd?.commandId || !cmd.commandType) {
      return {
        commandId: cmd?.commandId ?? "",
        status: "failed",
        errorCode: "INVALID_COMMAND",
        errorMessage: "missing commandId or commandType",
        appliedRevision: 0,
      };
    }

    const msg = buildRuntimeMessage("command", cmd.commandType, command);
    msg.commandId = cmd.commandId;

    let result: CommandResultPayload = {
      commandId: cmd.commandId,
      status: "failed",
      errorCode: "UNSUPPORTED_COMMAND",
      errorMessage: `unsupported command type: ${cmd.commandType}`,
      appliedRevision: cmd.desiredRevision ?? 0,
    };

    try {
      switch (cmd.commandType) {
        case "spawn":
        case "runtime.command.spawn": {
          const install = cmd.payload?.installation;
          if (install && this.callbacks.onSpawn) {
            const spawnPayload: SpawnPayload = {
              desiredRevision: cmd.desiredRevision ?? 0,
              installation: {
                installationId: install.installationId ?? "",
                characterId: install.characterId ?? "",
                packageId: install.packageId ?? "",
                packageVersion: install.packageVersion ?? "",
                installRoot: install.installRoot ?? "",
                manifestPath: install.manifestPath ?? "",
                packageHash: install.packageHash ?? "",
                defaultActionKey: install.defaultActionKey ?? "",
                canvasWidth: install.canvasWidth ?? 0,
                canvasHeight: install.canvasHeight ?? 0,
              },
              settings: {
                revision: cmd.settingsRevision ?? 0,
                alwaysOnTop: cmd.payload?.settings?.alwaysOnTop ?? true,
                scale: cmd.payload?.settings?.scale ?? 1,
                positionX: cmd.payload?.settings?.positionX ?? 0,
                positionY: cmd.payload?.settings?.positionY ?? 0,
                screenId: cmd.payload?.settings?.screenId ?? "",
                clickThroughMode: cmd.payload?.settings?.clickThroughMode ?? "off",
                soundEnabled: cmd.payload?.settings?.soundEnabled ?? false,
              },
            };
            result = await this.callbacks.onSpawn(msg, spawnPayload);
          }
          break;
        }
        case "destroy":
        case "runtime.command.destroy":
          if (this.callbacks.onDestroy) {
            const reason = cmd.payload?.reason ?? "";
            result = await this.callbacks.onDestroy(msg, cmd.desiredRevision ?? 0, reason);
          }
          break;
        case "show":
        case "runtime.command.show":
          if (this.callbacks.onShow) {
            result = await this.callbacks.onShow(msg, cmd.desiredRevision ?? 0);
          }
          break;
        case "hide":
        case "runtime.command.hide":
          if (this.callbacks.onHide) {
            result = await this.callbacks.onHide(msg, cmd.desiredRevision ?? 0);
          }
          break;
        case "play_action":
        case "runtime.command.play_action": {
          const actionKey = cmd.payload?.actionKey ?? "";
          const actionSpecHash = cmd.payload?.actionSpecHash ?? "";
          if (this.callbacks.onPlayAction) {
            result = await this.callbacks.onPlayAction(msg, actionKey, actionSpecHash);
          }
          break;
        }
        case "update_settings":
        case "runtime.command.update_settings":
          if (this.callbacks.onUpdateSettings && cmd.payload?.settings) {
            result = await this.callbacks.onUpdateSettings(
              msg,
              cmd.settingsRevision ?? 0,
              cmd.payload.settings,
            );
          }
          break;
        case "recenter":
        case "runtime.command.recenter_once":
        case "runtime.command.recenter":
          if (this.callbacks.onRecenter) {
            result = await this.callbacks.onRecenter(
              msg,
              cmd.settingsRevision ?? 0,
              cmd.payload?.screenId ?? "",
            );
          }
          break;
        case "sync":
        case "sync_desired_state":
        case "runtime.command.sync_desired_state": {
          const syncPayload: SyncPayload = {
            desiredRevision: cmd.desiredRevision ?? 0,
            ensureAbsent: cmd.payload?.ensureAbsent ?? false,
            desiredPet: cmd.payload?.desiredPet,
          };
          if (this.callbacks.onSync) {
            result = await this.callbacks.onSync(msg, syncPayload);
          }
          break;
        }
        case "state_probe":
          if (this.callbacks.onStateProbe) {
            this.callbacks.onStateProbe(msg);
            result = {
              commandId: cmd.commandId,
              status: "applied",
              errorCode: "",
              errorMessage: "",
              appliedRevision: cmd.desiredRevision ?? 0,
            };
          }
          break;
        case "shutdown":
          if (this.callbacks.onShutdown) {
            this.callbacks.onShutdown(
              msg,
              cmd.payload?.deadline ?? "",
              cmd.payload?.reason ?? "",
            );
          }
          result = {
            commandId: cmd.commandId,
            status: "applied",
            errorCode: "",
            errorMessage: "",
            appliedRevision: cmd.desiredRevision ?? 0,
          };
          break;
        default:
          break;
      }
    } catch (err) {
      result = {
        commandId: cmd.commandId,
        status: "failed",
        errorCode: "COMMAND_HANDLER_ERROR",
        errorMessage: err instanceof Error ? err.message : String(err),
        appliedRevision: cmd.desiredRevision ?? 0,
      };
    }
    void envelope;
    return result;
  }
}
