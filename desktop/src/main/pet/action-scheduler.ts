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
  | "action-submitted"
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
  // Runtime-v2 expiry is an absolute wall-clock timestamp. Scheduler cooldowns
  // and minimum-play windows use the same epoch domain so values can never be
  // compared against performance.now() by accident.
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
  private replacementPrevious: {
    request: DesktopPetActionRequest;
    startedAt: number;
    playbackInstanceId: string | null;
    commandId: string | null;
  } | null = null;
  private readonly submittedRequests = new Map<string, {
    request: DesktopPetActionRequest;
    action: RuntimeAction;
  }>();
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
      if (!this.playNow(request, targetAction, now)) {
        return "rejected";
      }
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
    this.replacementPrevious = null;
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
      if (!this.startAction(recoveryRequest, fallbackAction, now)) {
        this.drainQueueOrIdle();
      }
    }
  }

  getCurrent(): DesktopPetActionRequest | null {
    return this.current;
  }

  getQueue(): DesktopPetActionRequest[] {
    return this.queue.slice();
  }

  isCurrentPlaybackStarted(dedupeKey?: string): boolean {
    return this.currentActionStartedAt > 0 && this.current !== null &&
      (!dedupeKey || this.current.dedupeKey === dedupeKey);
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
    if (!playbackInstanceId || !commandId) return;
    const current = this.current;
    const submitted = this.submittedRequests.get(commandId);

    if (!current || current.actionKey !== actionKey) {
      // Cross-process ordering can expose a Renderer start for B after Main has
      // already submitted replacement C. If C has not started yet, B is now the
      // real physical rollback anchor and must replace the older A mirror.
      if (
        current &&
        this.currentActionStartedAt === 0 &&
        submitted &&
        submitted.request.actionKey === actionKey
      ) {
        this.replacementPrevious = {
          request: submitted.request,
          startedAt: nowTimestamp(),
          playbackInstanceId,
          commandId,
        };
        this.submittedRequests.delete(commandId);
        this.emit("action-started", submitted.request, submitted.action);
      }
      return;
    }

    const expectedCommandId = current.metadata?.runtimeCommandId ?? this.currentCommandId ?? "";
    if (expectedCommandId && commandId !== expectedCommandId) return;
    this.currentPlaybackInstanceId = playbackInstanceId;
    this.currentCommandId = commandId;
    this.currentActionStartedAt = nowTimestamp();
    this.submittedRequests.delete(commandId);
    // Renderer has committed the replacement first frame. The old renderer
    // playback will report its own interrupted event; it is no longer a restore
    // candidate from this point onward.
    this.replacementPrevious = null;
    this.emit("action-started", current, this.player.getCurrentAction());
  }

  notifyActionCompleted(actionKey: string, playbackInstanceId?: string, commandId?: string): void {
    if (commandId) this.submittedRequests.delete(commandId);
    const previous = this.replacementPrevious;
    if (previous && this.matchesIdentity(previous, actionKey, playbackInstanceId, commandId)) {
      this.replacementPrevious = null;
      this.emit("action-completed", previous.request, this.player.getCurrentAction());
      return;
    }
    if (this.restorePreviousAfterPendingTerminal("action-completed", actionKey, playbackInstanceId, commandId)) {
      return;
    }
    this.handleActionComplete(actionKey, playbackInstanceId, commandId);
  }

  notifyActionInterrupted(actionKey: string, playbackInstanceId?: string, commandId?: string): void {
    if (commandId) this.submittedRequests.delete(commandId);
    const previous = this.replacementPrevious;
    if (previous && this.matchesIdentity(previous, actionKey, playbackInstanceId, commandId)) {
      this.replacementPrevious = null;
      this.emit("action-interrupted", previous.request, this.player.getCurrentAction());
      return;
    }
    if (this.restorePreviousAfterPendingTerminal("action-interrupted", actionKey, playbackInstanceId, commandId)) {
      return;
    }
    this.finishCurrent("action-interrupted", actionKey, playbackInstanceId, commandId);
  }

  notifyActionFailed(actionKey: string, playbackInstanceId?: string, commandId?: string): void {
    if (commandId) this.submittedRequests.delete(commandId);
    const previous = this.replacementPrevious;
    if (previous && this.matchesIdentity(previous, actionKey, playbackInstanceId, commandId)) {
      this.replacementPrevious = null;
      this.emit("action-interrupted", previous.request, this.player.getCurrentAction());
      return;
    }
    if (!this.matchesCurrent(actionKey, playbackInstanceId, commandId)) return;
    const failed = this.current;
    // A replacement can be accepted by the command gateway and still fail while
    // loading assets, before its first frame becomes visible. In that case the
    // renderer resumes the old playback; restore the Scheduler mirror as well.
    if (failed && this.currentActionStartedAt === 0 && this.replacementPrevious) {
      const previous = this.replacementPrevious;
      this.replacementPrevious = null;
      this.current = previous.request;
      this.currentActionStartedAt = previous.startedAt;
      this.currentPlaybackInstanceId = previous.playbackInstanceId;
      this.currentCommandId = previous.commandId;
      this.emit("action-rejected", failed, this.resolveAction(failed));
      return;
    }
    this.finishCurrent("action-interrupted", actionKey, playbackInstanceId, commandId);
  }

  private restorePreviousAfterPendingTerminal(
    terminalEvent: "action-completed" | "action-interrupted",
    actionKey: string,
    playbackInstanceId?: string,
    commandId?: string,
  ): boolean {
    if (this.currentActionStartedAt !== 0 || !this.replacementPrevious) return false;
    if (!this.matchesCurrent(actionKey, playbackInstanceId, commandId)) return false;
    const terminalRequest = this.current;
    const previous = this.replacementPrevious;
    this.replacementPrevious = null;
    this.current = previous.request;
    this.currentActionStartedAt = previous.startedAt;
    this.currentPlaybackInstanceId = previous.playbackInstanceId;
    this.currentCommandId = previous.commandId;
    if (terminalRequest) {
      // Covers Renderer `already_satisfied` completion and pre-first-frame
      // interruption. The command is terminal, but the previous physical
      // playback is still what the user sees.
      this.emit(terminalEvent, terminalRequest, this.player.getCurrentAction());
    }
    return true;
  }

  private handleActionComplete(actionKey: string, playbackInstanceId?: string, commandId?: string): void {
    this.finishCurrent("action-completed", actionKey, playbackInstanceId, commandId);
  }

  private finishCurrent(
    terminalEvent: "action-completed" | "action-interrupted",
    actionKey: string,
    playbackInstanceId?: string,
    commandId?: string,
  ): void {
    if (!this.loaded || !this.matchesCurrent(actionKey, playbackInstanceId, commandId)) return;
    const finishedRequest = this.current;
    this.current = null;
    this.currentActionStartedAt = 0;
    this.currentPlaybackInstanceId = null;
    this.currentCommandId = null;

    if (finishedRequest) {
      this.emit(terminalEvent, finishedRequest, this.player.getCurrentAction());
    }
    this.drainQueueOrIdle();
  }

  private drainQueueOrIdle(): void {
    while (this.queue.length > 0) {
      const next = this.queue.shift() as DesktopPetActionRequest;
      const now = nowTimestamp();
      if (next.expiresAt !== undefined && next.expiresAt <= now) {
        this.emitCancelled(next, "ttl_expired_in_queue");
        continue;
      }
      const nextAction = this.resolveAction(next);
      if (!nextAction) {
        this.emitCancelled(next, "action_unavailable_at_dequeue");
        continue;
      }
      if (this.startAction(next, nextAction, now)) {
        const cooldownKey = next.dedupeKey ?? next.actionKey;
        this.lastTriggeredAt.set(cooldownKey, now);
        return;
      }
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
    if (this.currentActionStartedAt > 0 && now - this.currentActionStartedAt < minDuration) {
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
  ): boolean {
    // Renderer owns physical replacement. The old request is not terminal until
    // the renderer presents the replacement first frame and emits interrupted.
    return this.startAction(request, action, now);
  }

  private startAction(
    request: DesktopPetActionRequest,
    action: RuntimeAction,
    now: number,
  ): boolean {
    void now;
    const effectiveRequest: DesktopPetActionRequest = action.key === request.actionKey
      ? request
      : {
          ...request,
          actionKey: action.key,
          metadata: {
            ...(request.metadata ?? {}),
            requestedActionKey: request.actionKey,
          },
        };
    const previousRequest = this.current;
    if (
      previousRequest &&
      previousRequest.actionKey !== effectiveRequest.actionKey
    ) {
      const prevIdleKey = previousRequest.dedupeKey ?? previousRequest.actionKey;
      this.idleRepeatCount.delete(prevIdleKey);
    }

    if (previousRequest && this.currentActionStartedAt > 0) {
      this.replacementPrevious = {
        request: previousRequest,
        startedAt: this.currentActionStartedAt,
        playbackInstanceId: this.currentPlaybackInstanceId,
        commandId: this.currentCommandId,
      };
    } else if (previousRequest && this.replacementPrevious) {
      // A replacement can itself be replaced while it is still loading. Keep
      // the last Renderer-confirmed playback as the rollback anchor across the
      // whole pending chain (A playing -> B loading -> C loading). If C fails,
      // Renderer can still resume A and Scheduler must restore the same mirror.
      // B's later cancellation/failure event is intentionally ignored because
      // it never became the active physical playback.
    } else {
      this.replacementPrevious = null;
    }

    this.current = effectiveRequest;
    // Submission is not playback. Do not start the minimum-play clock and do not
    // publish a playback identity until Renderer emits command_accepted/started.
    this.currentActionStartedAt = 0;
    this.currentPlaybackInstanceId = null;
    this.currentCommandId = effectiveRequest.metadata?.runtimeCommandId ?? null;
    this.bumpIdleRepeat(effectiveRequest);
    const submission = this.player.switchAction(
      action,
      this.buildPlayerSwitchContext(effectiveRequest, action),
    );
    if (!submission) {
      const failed = this.current;
      const previous = this.replacementPrevious;
      this.replacementPrevious = null;
      if (previous) {
        this.current = previous.request;
        this.currentActionStartedAt = previous.startedAt;
        this.currentPlaybackInstanceId = previous.playbackInstanceId;
        this.currentCommandId = previous.commandId;
      } else {
        this.current = null;
        this.currentActionStartedAt = 0;
        this.currentPlaybackInstanceId = null;
        this.currentCommandId = null;
      }
      if (failed) this.emit("action-rejected", failed, action);
      return false;
    }
    this.currentCommandId = submission.commandId;
    this.submittedRequests.set(submission.commandId, {
      request: effectiveRequest,
      action,
    });
    this.emit("action-submitted", effectiveRequest, action);
    return true;
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
    void this.startAction(request, defaultAction, now);
  }

  private resetRuntimeState(): void {
    this.cancelQueuedRequests("scheduler_reset");
    this.current = null;
    this.currentActionStartedAt = 0;
    this.currentPlaybackInstanceId = null;
    this.currentCommandId = null;
    this.replacementPrevious = null;
    this.submittedRequests.clear();
    this.lastTriggeredAt.clear();
    this.idleRepeatCount.clear();
    this.sustainedState = null;
  }

  private matchesIdentity(
    identity: {
      request: DesktopPetActionRequest;
      playbackInstanceId: string | null;
      commandId: string | null;
    },
    actionKey: string,
    playbackInstanceId?: string,
    commandId?: string,
  ): boolean {
    if (identity.request.actionKey !== actionKey) return false;
    const expectedCommandId = identity.request.metadata?.runtimeCommandId ?? identity.commandId ?? "";
    if (expectedCommandId && commandId !== expectedCommandId) return false;
    if (!expectedCommandId && commandId) return false;
    if (identity.playbackInstanceId && playbackInstanceId !== identity.playbackInstanceId) return false;
    return true;
  }

  private matchesCurrent(actionKey: string, playbackInstanceId?: string, commandId?: string): boolean {
    const current = this.current;
    if (!current || current.actionKey !== actionKey) return false;
    const expectedCommandId = current.metadata?.runtimeCommandId ?? this.currentCommandId ?? "";
    if (expectedCommandId) {
      if (!commandId || commandId !== expectedCommandId) return false;
      if (this.currentPlaybackInstanceId && (!playbackInstanceId || playbackInstanceId !== this.currentPlaybackInstanceId)) return false;
      return true;
    }
    if (commandId) return false;
    if (this.currentPlaybackInstanceId && playbackInstanceId && playbackInstanceId !== this.currentPlaybackInstanceId) return false;
    return current.actionKey === actionKey;
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
      requiresAuthoritativeExpiry: Boolean(request.metadata?.runtimeCommandId),
      expiresAt: request.expiresAt !== undefined ? new Date(request.expiresAt).toISOString() : undefined,
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
    } catch (err) {
      console.error(`[DesktopPetActionScheduler] callback failed for ${event}`, err);
    }
  }
}
