import type { LoadedInstallation, RuntimeAction } from "./resource-loader";
import { ResourceLoader } from "./resource-loader";
import type { ActionPlayer, PlayerCallbacks, PlayerLike } from "./action-player";

export type EventSource =
  | "system"
  | "idle"
  | "user_click"
  | "user_double_click"
  | "user_drag"
  | "user_hover"
  | "chat_listening"
  | "chat_thinking"
  | "chat_speaking"
  | "manual"
  | "recovery";

export interface DesktopPetActionRequest {
  actionKey: string;
  source: EventSource;
  priority: number;
  interrupt: boolean;
  dedupeKey?: string;
  expiresAt?: number;
  metadata?: Record<string, string>;
}

export type SchedulerEvent =
  | "action-started"
  | "action-completed"
  | "action-fallback"
  | "action-queued"
  | "action-rejected"
  | "action-interrupted";

export interface SchedulerCallbacks {
  onEvent?: (
    event: SchedulerEvent,
    request: DesktopPetActionRequest,
    action: RuntimeAction | null,
  ) => void;
}

export const EventSources = {
  SYSTEM: "system",
  IDLE: "idle",
  USER_CLICK: "user_click",
  USER_DOUBLE_CLICK: "user_double_click",
  USER_DRAG: "user_drag",
  USER_HOVER: "user_hover",
  CHAT_LISTENING: "chat_listening",
  CHAT_THINKING: "chat_thinking",
  CHAT_SPEAKING: "chat_speaking",
  MANUAL: "manual",
  RECOVERY: "recovery",
} as const;

export const ActionPriorities = {
  DRAG: 100,
  FALL: 90,
  CLICK: 80,
  SPEAKING: 70,
  THINKING: 60,
  EMOTION: 50,
  GREETING: 40,
  LIFE: 30,
  RANDOM_IDLE: 20,
  DEFAULT_IDLE: 10,
} as const;

export const FallbackMappings: Record<string, string[]> = {
  clicked: ["happy", "wave", "idle", "idle_normal", "idle_breathing"],
  double_clicked: ["clicked", "happy", "wave", "idle", "idle_normal", "idle_breathing"],
  hovered: ["happy", "wave", "idle", "idle_normal", "idle_breathing"],
  speaking: ["listening", "idle", "idle_normal", "idle_breathing"],
  thinking: ["waiting", "idle", "idle_normal", "idle_breathing"],
  dragged: ["picked_up", "idle", "idle_normal", "idle_breathing"],
  land: ["idle", "idle_normal", "idle_breathing"],
  dropped: ["fall", "land", "idle", "idle_normal", "idle_breathing"],
};

const MAX_QUEUE_SIZE = 5;
const DEFAULT_MIN_PLAY_DURATION_MS = 300;
const DEFAULT_HOVER_COOLDOWN_MS = 3000;
const DEFAULT_CLICK_COOLDOWN_MS = 500;
const MAX_IDLE_REPEAT_COUNT = 2;
const IDLE_PRIORITY_THRESHOLD = ActionPriorities.RANDOM_IDLE;
const SUSTAINED_SPEAKING_KEY = "chat_speaking";

const SOURCE_COOLDOWN_MS: Record<EventSource, number> = {
  system: 0,
  idle: 0,
  user_click: DEFAULT_CLICK_COOLDOWN_MS,
  user_double_click: DEFAULT_CLICK_COOLDOWN_MS * 2,
  user_drag: 0,
  user_hover: DEFAULT_HOVER_COOLDOWN_MS,
  chat_listening: 0,
  chat_thinking: 0,
  chat_speaking: 0,
  manual: 0,
  recovery: 0,
};

function nowTimestamp(): number {
  if (
    typeof performance !== "undefined" &&
    typeof performance.now === "function"
  ) {
    return performance.now();
  }
  return Date.now();
}

type PlayerInternal = {
  callbacks: PlayerCallbacks;
};

export class DesktopPetActionScheduler {
  private loaded: LoadedInstallation | null = null;
  private player: PlayerLike;
  private callbacks: SchedulerCallbacks;
  private queue: DesktopPetActionRequest[] = [];
  private current: DesktopPetActionRequest | null = null;
  private currentActionStartedAt = 0;
  private lastTriggeredAt: Map<string, number> = new Map();
  private sustainedState: string | null = null;
  private idleRepeatCount: Map<string, number> = new Map();
  private resourceLoader: ResourceLoader;
  private originalPlayerCallbacks: PlayerCallbacks;

  constructor(player: PlayerLike, callbacks?: SchedulerCallbacks) {
    this.player = player;
    this.callbacks = callbacks ?? {};
    this.resourceLoader = new ResourceLoader();
    const playerRef = this.player as unknown as PlayerInternal;
    this.originalPlayerCallbacks = playerRef.callbacks;
    this.installPlayerHooks();
  }

  attachLoaded(loaded: LoadedInstallation): void {
    this.loaded = loaded;
    this.player.attachLoaded(loaded);
    this.resetRuntimeState();
    this.tryPlayDefaultIdle();
  }

  detachLoaded(): void {
    this.loaded = null;
    this.player.detachLoaded();
    this.resetRuntimeState();
  }

  submit(request: DesktopPetActionRequest): "played" | "queued" | "rejected" | "fallback" {
    if (!this.loaded) {
      this.emit("action-rejected", request, null);
      return "rejected";
    }

    const now = nowTimestamp();

    if (request.expiresAt !== undefined && request.expiresAt <= now) {
      this.emit("action-rejected", request, null);
      return "rejected";
    }

    if (this.isSustainedBlocked(request)) {
      this.emit("action-rejected", request, null);
      return "rejected";
    }

    const cooldownMs = this.getCooldownForRequest(request);
    const cooldownKey = request.dedupeKey ?? request.actionKey;
    if (cooldownMs > 0) {
      const last = this.lastTriggeredAt.get(cooldownKey);
      if (last !== undefined && now - last < cooldownMs) {
        this.emit("action-rejected", request, null);
        return "rejected";
      }
    }

    if (!this.canIdleRepeat(request)) {
      this.emit("action-rejected", request, null);
      return "rejected";
    }

    const targetAction = this.resolveAction(request);
    if (!targetAction) {
      this.emit("action-rejected", request, null);
      return "rejected";
    }

    const isFallback = targetAction.key !== request.actionKey;
    if (isFallback) {
      this.emit("action-fallback", request, targetAction);
    }

    if (this.canPlayImmediately(request, now)) {
      this.playNow(request, targetAction, now);
      this.lastTriggeredAt.set(cooldownKey, now);
      return isFallback ? "fallback" : "played";
    }

    if (this.tryEnqueue(request)) {
      this.emit("action-queued", request, targetAction);
      return "queued";
    }

    this.emit("action-rejected", request, targetAction);
    return "rejected";
  }

  forceInterrupt(reason: "user_drag" | "app_exit" | "resource_invalid"): void {
    const now = nowTimestamp();
    const previous = this.current;
    this.current = null;
    this.currentActionStartedAt = 0;
    this.queue.length = 0;

    if (previous) {
      this.emit("action-interrupted", previous, this.player.getCurrentAction());
    }

    this.player.stop();

    if (reason === "app_exit" || !this.loaded) {
      return;
    }

    let fallbackAction: RuntimeAction | null = null;
    if (reason === "user_drag") {
      const chain = FallbackMappings["dragged"] ?? [];
      fallbackAction = this.resourceLoader.findFirstAvailableAction(this.loaded, chain);
    }
    if (!fallbackAction) {
      fallbackAction = this.loaded.defaultAction;
    }

    if (fallbackAction) {
      const recoveryRequest: DesktopPetActionRequest = {
        actionKey: fallbackAction.key,
        source: "recovery",
        priority: ActionPriorities.FALL,
        interrupt: true,
        dedupeKey: `recovery_${fallbackAction.key}_${Math.floor(now)}`,
      };
      this.startAction(recoveryRequest, fallbackAction, now);
    }
  }

  getCurrent(): DesktopPetActionRequest | null {
    return this.current;
  }

  getQueue(): DesktopPetActionRequest[] {
    return this.queue.slice();
  }

  setSustainedState(state: string | null): void {
    const previous = this.sustainedState;
    this.sustainedState = state;
    if (previous && previous !== state && previous === "speaking") {
      this.lastTriggeredAt.delete(SUSTAINED_SPEAKING_KEY);
    }
  }

  clearQueue(): void {
    this.queue.length = 0;
  }

  dispose(): void {
    const playerRef = this.player as unknown as PlayerInternal;
    playerRef.callbacks = this.originalPlayerCallbacks;
    this.resetRuntimeState();
  }

  private installPlayerHooks(): void {
    const playerRef = this.player as unknown as PlayerInternal;
    const original = this.originalPlayerCallbacks;
    playerRef.callbacks = {
      onFrameChange: original.onFrameChange
        ? (key, frame) => original.onFrameChange!(key, frame)
        : undefined,
      onActionComplete: (key, loop) => {
        try {
          original.onActionComplete?.(key, loop);
        } catch {
          void 0;
        }
        this.handleActionComplete(key);
      },
      onActionSwitch: (newKey, oldKey) => {
        try {
          original.onActionSwitch?.(newKey, oldKey);
        } catch {
          void 0;
        }
        this.handleActionSwitch(newKey, oldKey);
      },
      onError: original.onError,
    };
  }

  private handleActionComplete(_actionKey: string): void {
    if (!this.loaded) return;
    const completedRequest = this.current;
    this.current = null;
    this.currentActionStartedAt = 0;

    if (completedRequest) {
      this.emit("action-completed", completedRequest, this.player.getCurrentAction());
    }

    while (this.queue.length > 0) {
      const next = this.queue.shift() as DesktopPetActionRequest;
      const nextAction = this.resolveAction(next);
      if (!nextAction) {
        continue;
      }
      const now = nowTimestamp();
      this.startAction(next, nextAction, now);
      const cooldownKey = next.dedupeKey ?? next.actionKey;
      this.lastTriggeredAt.set(cooldownKey, now);
      return;
    }

    this.tryPlayDefaultIdle();
  }

  private handleActionSwitch(_newKey: string, _oldKey: string | null): void {
  }

  private isSustainedBlocked(request: DesktopPetActionRequest): boolean {
    if (!this.sustainedState) return false;
    if (
      request.source === "chat_speaking" &&
      this.sustainedState === "speaking"
    ) {
      const key = request.dedupeKey ?? SUSTAINED_SPEAKING_KEY;
      if (this.lastTriggeredAt.has(key)) {
        return true;
      }
    }
    return false;
  }

  private getCooldownForRequest(request: DesktopPetActionRequest): number {
    const fromMeta = request.metadata?.cooldownMs;
    if (fromMeta !== undefined) {
      const parsed = Number(fromMeta);
      if (Number.isFinite(parsed) && parsed >= 0) {
        return parsed;
      }
    }
    return SOURCE_COOLDOWN_MS[request.source] ?? 0;
  }

  private canIdleRepeat(request: DesktopPetActionRequest): boolean {
    if (request.priority > IDLE_PRIORITY_THRESHOLD) return true;
    if (request.source !== "idle") return true;
    const key = request.dedupeKey ?? request.actionKey;
    const count = this.idleRepeatCount.get(key) ?? 0;
    if (count >= MAX_IDLE_REPEAT_COUNT) return false;
    return true;
  }

  private bumpIdleRepeat(request: DesktopPetActionRequest): void {
    if (request.priority > IDLE_PRIORITY_THRESHOLD) return;
    if (request.source !== "idle") return;
    const key = request.dedupeKey ?? request.actionKey;
    const count = this.idleRepeatCount.get(key) ?? 0;
    this.idleRepeatCount.set(key, count + 1);
  }

  private resolveAction(request: DesktopPetActionRequest): RuntimeAction | null {
    if (!this.loaded) return null;

    const direct = this.loaded.actions.get(request.actionKey);
    if (direct && direct.available) {
      return direct;
    }

    const fallbackChain = FallbackMappings[request.actionKey];
    if (fallbackChain && fallbackChain.length > 0) {
      const fallback = this.resourceLoader.findFirstAvailableAction(
        this.loaded,
        fallbackChain,
      );
      if (fallback) {
        return fallback;
      }
    }

    if (direct) {
      const playerChain = this.player.getFallbackChain(direct, this.loaded);
      if (playerChain.length > 0) {
        return playerChain[0];
      }
    }

    return this.loaded.defaultAction ?? null;
  }

  private canPlayImmediately(
    request: DesktopPetActionRequest,
    now: number,
  ): boolean {
    if (!this.current) return true;
    const currentAction = this.player.getCurrentAction();
    if (!currentAction) return true;

    if (request.priority <= this.current.priority) {
      return false;
    }

    if (!request.interrupt) {
      return false;
    }

    if (currentAction.interruptible !== true) {
      return false;
    }

    const minDuration = this.getMinPlayDuration(currentAction);
    if (now - this.currentActionStartedAt < minDuration) {
      return false;
    }

    return true;
  }

  private getMinPlayDuration(_action: RuntimeAction): number {
    return DEFAULT_MIN_PLAY_DURATION_MS;
  }

  private playNow(
    request: DesktopPetActionRequest,
    action: RuntimeAction,
    now: number,
  ): void {
    const previous = this.current;
    if (previous) {
      this.emit("action-interrupted", previous, this.player.getCurrentAction());
    }
    this.startAction(request, action, now);
  }

  private startAction(
    request: DesktopPetActionRequest,
    action: RuntimeAction,
    now: number,
  ): void {
    const previousRequest = this.current;
    if (
      previousRequest &&
      previousRequest.actionKey !== request.actionKey
    ) {
      const prevIdleKey = previousRequest.dedupeKey ?? previousRequest.actionKey;
      this.idleRepeatCount.delete(prevIdleKey);
    }

    this.current = request;
    this.currentActionStartedAt = now;
    this.bumpIdleRepeat(request);
    this.player.switchAction(action);
    this.emit("action-started", request, action);
  }

  private tryEnqueue(request: DesktopPetActionRequest): boolean {
    if (this.queue.length >= MAX_QUEUE_SIZE) {
      if (request.priority <= IDLE_PRIORITY_THRESHOLD) {
        return false;
      }
      let lowestIndex = 0;
      for (let i = 1; i < this.queue.length; i++) {
        if (this.queue[i].priority < this.queue[lowestIndex].priority) {
          lowestIndex = i;
        }
      }
      if (this.queue[lowestIndex].priority >= request.priority) {
        return false;
      }
      this.queue.splice(lowestIndex, 1);
    } else if (
      request.priority <= IDLE_PRIORITY_THRESHOLD &&
      this.queue.length > 0
    ) {
      return false;
    }

    if (request.dedupeKey) {
      const existingIndex = this.queue.findIndex(
        (item) =>
          item.dedupeKey === request.dedupeKey &&
          item.actionKey === request.actionKey,
      );
      if (existingIndex !== -1) {
        this.queue[existingIndex] = request;
        this.sortQueue();
        return true;
      }
    }

    this.queue.push(request);
    this.sortQueue();
    return true;
  }

  private sortQueue(): void {
    this.queue.sort((a, b) => b.priority - a.priority);
  }

  private tryPlayDefaultIdle(): void {
    if (!this.loaded) return;
    const defaultAction = this.loaded.defaultAction;
    if (!defaultAction) return;
    const now = nowTimestamp();
    const request: DesktopPetActionRequest = {
      actionKey: defaultAction.key,
      source: "system",
      priority: ActionPriorities.DEFAULT_IDLE,
      interrupt: false,
    };
    this.startAction(request, defaultAction, now);
  }

  private resetRuntimeState(): void {
    this.queue.length = 0;
    this.current = null;
    this.currentActionStartedAt = 0;
    this.lastTriggeredAt.clear();
    this.idleRepeatCount.clear();
    this.sustainedState = null;
  }

  private emit(
    event: SchedulerEvent,
    request: DesktopPetActionRequest,
    action: RuntimeAction | null,
  ): void {
    if (!this.callbacks.onEvent) return;
    try {
      this.callbacks.onEvent(event, request, action);
    } catch {
      void 0;
    }
  }
}
