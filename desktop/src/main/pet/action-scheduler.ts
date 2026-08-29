import type { LoadedInstallation, RuntimeAction } from "./resource-loader";
import { ResourceLoader } from "./resource-loader";
import type {
  DesktopPetPlayerPort,
  PlayerLifecyclePort,
  PlayerSwitchContext,
} from "./player-port";

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
  | "voice_listening"
  | "voice_speaking"
  | "tool_working"
  | "emotion"
  | "proactive"
  | "autonomous"
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
  | "action-interrupted"
  | "action-cancelled";

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
  VOICE_LISTENING: "voice_listening",
  VOICE_SPEAKING: "voice_speaking",
  TOOL_WORKING: "tool_working",
  EMOTION: "emotion",
  PROACTIVE: "proactive",
  AUTONOMOUS: "autonomous",
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
  listening: ["dialogue_listening", "idle_look_around", "idle_normal"],
  working: ["work", "thinking", "study", "idle_normal"],
  researching: ["read", "study", "thinking", "idle_normal"],
  organizing: ["write", "work", "thinking", "idle_normal"],
  happy: ["excited", "wave", "idle_normal"],
  tired: ["sleep", "sit", "idle_breathing", "idle_normal"],
  greeting: ["wave", "bow", "idle_normal"],
  walk_left: ["walk", "move", "idle_normal"],
  walk_right: ["walk", "move", "idle_normal"],
  fall: ["dropped", "dragged", "idle_normal"],
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
  voice_listening: 0,
  voice_speaking: 0,
  tool_working: 0,
  emotion: 1000,
  proactive: 3000,
  autonomous: 5000,
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

export class DesktopPetActionScheduler {
  private loaded: LoadedInstallation | null = null;
  private player: DesktopPetPlayerPort & PlayerLifecyclePort;
  private callbacks: SchedulerCallbacks;
  private queue: DesktopPetActionRequest[] = [];
  private current: DesktopPetActionRequest | null = null;
  private currentActionStartedAt = 0;
  private currentPlaybackInstanceId: string | null = null;
  private currentCommandId: string | null = null;
  private lastTriggeredAt: Map<string, number> = new Map();
  private sustainedState: string | null = null;
  private idleRepeatCount: Map<string, number> = new Map();
  private resourceLoader: ResourceLoader;

  constructor(player: DesktopPetPlayerPort & PlayerLifecyclePort, callbacks?: SchedulerCallbacks) {
    this.player = player;
    this.callbacks = callbacks ?? {};
    this.resourceLoader = new ResourceLoader();
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

  forceInterrupt(reason: "user_drag" | "app_exit" | "resource_invalid" | "runtime_stop"): void {
    const now = nowTimestamp();
    const previous = this.current;
    this.current = null;
    this.currentActionStartedAt = 0;
    this.currentPlaybackInstanceId = null;
    this.currentCommandId = null;
    this.cancelQueuedRequests(`force_interrupt:${reason}`);

    if (previous) {
      this.emit("action-interrupted", previous, this.player.getCurrentAction());
    }

    const stopReason = reason === "app_exit"
      ? "window_destroyed"
      : reason === "resource_invalid"
        ? "resource_failure"
        : reason === "runtime_stop"
          ? "runtime_stop"
          : "system_force";
    this.player.stop(stopReason);

    if (reason === "app_exit" || reason === "runtime_stop" || !this.loaded) {
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
    this.cancelQueuedRequests("queue_cleared");
  }

  dispose(): void {
    this.resetRuntimeState();
  }

  notifyActionStarted(actionKey: string, playbackInstanceId: string, commandId?: string): void {
    const current = this.current;
    if (!current || current.actionKey !== actionKey) return;
    const expectedCommandId = current.metadata?.runtimeCommandId ?? this.currentCommandId ?? "";
    if (commandId && expectedCommandId && commandId !== expectedCommandId) return;
    this.currentPlaybackInstanceId = playbackInstanceId || this.currentPlaybackInstanceId;
    this.currentCommandId = commandId || expectedCommandId || this.currentCommandId;
  }

  notifyActionCompleted(actionKey: string, playbackInstanceId?: string, commandId?: string): void {
    this.handleActionComplete(actionKey, playbackInstanceId, commandId);
  }

  notifyActionInterrupted(actionKey: string, playbackInstanceId?: string, commandId?: string): void {
    if (!this.loaded || !this.matchesCurrent(actionKey, playbackInstanceId, commandId)) return;
    const interruptedRequest = this.current;
    this.current = null;
    this.currentActionStartedAt = 0;
    this.currentPlaybackInstanceId = null;
    this.currentCommandId = null;
    if (interruptedRequest) {
      this.emit("action-interrupted", interruptedRequest, this.player.getCurrentAction());
    }
  }

  private handleActionComplete(actionKey: string, playbackInstanceId?: string, commandId?: string): void {
    if (!this.loaded || !this.matchesCurrent(actionKey, playbackInstanceId, commandId)) return;
    const completedRequest = this.current;
    this.current = null;
    this.currentActionStartedAt = 0;
    this.currentPlaybackInstanceId = null;
    this.currentCommandId = null;

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

    const minDuration = this.getMinPlayDuration(currentAction, this.current);
    if (now - this.currentActionStartedAt < minDuration) {
      return false;
    }

    return true;
  }

  private getMinPlayDuration(action: RuntimeAction, request: DesktopPetActionRequest | null): number {
    const requestMinimum = this.readMetadataNumber(request, "minimumPlayMs");
    const requestInterruptAfter = this.readMetadataNumber(request, "interruptAfterMs");
    const hasExplicitTiming =
      requestMinimum !== undefined ||
      requestInterruptAfter !== undefined ||
      action.minimumPlayMs !== undefined ||
      action.interruptAfterMs !== undefined;
    if (!hasExplicitTiming) return DEFAULT_MIN_PLAY_DURATION_MS;

    const minimumPlayMs = requestMinimum ?? action.minimumPlayMs ?? 0;
    const interruptAfterMs = requestInterruptAfter ?? action.interruptAfterMs ?? 0;
    return Math.max(0, minimumPlayMs, interruptAfterMs);
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
    this.currentPlaybackInstanceId = null;
    this.currentCommandId = request.metadata?.runtimeCommandId ?? null;
    this.bumpIdleRepeat(request);
    const submission = this.player.switchAction(action, this.buildPlayerSwitchContext(request, action));
    if (submission) {
      this.currentPlaybackInstanceId = submission.playbackInstanceId;
      this.currentCommandId = submission.commandId;
    }
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
      const [evicted] = this.queue.splice(lowestIndex, 1);
      if (evicted) this.emitCancelled(evicted, "queue_evicted");
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
        const existing = this.queue[existingIndex];
        if (existing.metadata?.runtimeCommandId !== request.metadata?.runtimeCommandId) {
          this.emitCancelled(existing, "queue_coalesced");
        }
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
    this.cancelQueuedRequests("scheduler_reset");
    this.current = null;
    this.currentActionStartedAt = 0;
    this.currentPlaybackInstanceId = null;
    this.currentCommandId = null;
    this.lastTriggeredAt.clear();
    this.idleRepeatCount.clear();
    this.sustainedState = null;
  }

  private matchesCurrent(actionKey: string, playbackInstanceId?: string, commandId?: string): boolean {
    const current = this.current;
    if (!current || current.actionKey !== actionKey) return false;
    const expectedCommandId = current.metadata?.runtimeCommandId ?? this.currentCommandId ?? "";
    if (commandId && expectedCommandId && commandId !== expectedCommandId) return false;
    if (playbackInstanceId && this.currentPlaybackInstanceId && playbackInstanceId !== this.currentPlaybackInstanceId) return false;
    return true;
  }

  private buildPlayerSwitchContext(
    request: DesktopPetActionRequest,
    action: RuntimeAction,
  ): PlayerSwitchContext {
    const returnTo = request.metadata?.returnTo ?? "";
    const returnOverride = returnTo === "default"
      ? { type: "default" as const }
      : returnTo === "previous"
        ? { type: "previous" as const }
        : returnTo === "current_activity"
          ? { type: "current_activity" as const }
          : returnTo === "none"
            ? { type: "none" as const }
            : returnTo
              ? { type: "action" as const, actionKey: returnTo }
              : undefined;
    return {
      commandId: request.metadata?.runtimeCommandId || undefined,
      idempotencyKey: request.metadata?.runtimeDecisionId || request.dedupeKey,
      priority: request.priority,
      queuePolicy: request.interrupt ? "replace_current" : "enqueue",
      interruptPolicy: request.interrupt ? "respect_action" : "never_interrupt",
      returnOverride,
      minimumPlayMs: this.readMetadataNumber(request, "minimumPlayMs") ?? action.minimumPlayMs,
      maximumPlayMs: this.readMetadataNumber(request, "maximumPlayMs") ?? action.maximumPlayMs,
      interruptAfterMs: this.readMetadataNumber(request, "interruptAfterMs") ?? action.interruptAfterMs,
      completionPolicy: request.metadata?.completionPolicy || undefined,
      source: request.source,
      traceId: request.metadata?.traceId || undefined,
    };
  }

  private readMetadataNumber(request: DesktopPetActionRequest | null, key: string): number | undefined {
    const raw = request?.metadata?.[key];
    if (raw === undefined || raw === "") return undefined;
    const value = Number(raw);
    if (!Number.isFinite(value) || value < 0) return undefined;
    return value;
  }

  private emitCancelled(request: DesktopPetActionRequest, reason: string): void {
    this.emit(
      "action-cancelled",
      { ...request, metadata: { ...(request.metadata ?? {}), cancellationReason: reason } },
      this.resolveAction(request),
    );
  }

  private cancelQueuedRequests(reason: string): void {
    if (this.queue.length === 0) return;
    const queued = this.queue.splice(0, this.queue.length);
    for (const request of queued) this.emitCancelled(request, reason);
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
