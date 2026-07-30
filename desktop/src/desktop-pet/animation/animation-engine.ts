import type {
  ActionAssetRepository,
  AnimationDiagnostics,
  CommandAck,
  DecodedFrame,
  FrameTimeline,
  GenerationToken,
  InterruptionReason,
  LoadedAction,
  LoadedActionAssets,
  PackagePlaybackSnapshot,
  PlaybackClock,
  PlaybackErrorView,
  PlaybackEvent,
  PlaybackRecoverySnapshot,
  PlaybackSnapshot,
  PetVisualSurface,
  PlayActionCommand,
  PlayerPhase,
  ReturnTarget,
  TimelinePosition,
} from "./contracts";
import {
  CLOCK_LARGE_GAP_THRESHOLD_MS,
  DEFAULT_CACHE_BUDGET_BYTES,
  DEFAULT_QUEUE_LIMITS,
  FINALIZED_SET_MAX_ENTRIES,
  FINALIZED_SET_TTL_MS,
} from "./contracts";
import { PlaybackError, PLAYBACK_ERROR_CODES } from "./errors";
import {
  createFrameTimeline,
} from "./frame-timeline";
import {
  GenerationManagerImpl,
} from "./generation-manager";
import {
  MonotonicPlaybackClock,
} from "./playback-clock";
import {
  ActionQueue,
} from "./action-queue";
import {
  CommandGateway,
} from "./command-gateway";
import {
  InterruptArbiter,
} from "./interrupt-arbiter";
import {
  ReturnTargetResolver,
} from "./return-target-resolver";
import {
  createInitialState,
  isStableAction,
  canBePrevious,
  createPlaybackInstanceId,
  playerReducer,
  PlayerState,
  StateAction,
  toSnapshot,
} from "./player-state-machine";
import { PlaybackTelemetry } from "./telemetry";
import { PlaybackRecoveryController } from "./playback-recovery";

export type EventListener = (event: PlaybackEvent) => void;

interface FinalizedEntry {
  timestamp: number;
}

interface LoadedActionEntry {
  action: LoadedAction;
  frames: DecodedFrame[];
  timeline: FrameTimeline;
  totalBytes: number;
}

export interface AnimationEngineDeps {
  surface: PetVisualSurface;
  assetRepository: ActionAssetRepository;
  clock?: PlaybackClock;
  cacheBudgetBytes?: number;
}

export class DesktopPetAnimationEngine {
  private state: PlayerState;
  private clock: PlaybackClock;
  private surface: PetVisualSurface;
  private assetRepository: ActionAssetRepository;
  private generationManager: GenerationManagerImpl;
  private queue: ActionQueue;
  private arbiter: InterruptArbiter;
  private gateway: CommandGateway;
  private returnResolver: ReturnTargetResolver;
  private telemetry: PlaybackTelemetry;
  private recovery: PlaybackRecoveryController;

  private packageSnapshot: PackagePlaybackSnapshot | null = null;
  private loadedActions: Map<string, LoadedActionEntry> = new Map();
  private currentLoadedEntry: LoadedActionEntry | null = null;
  private currentTimeline: FrameTimeline | null = null;
  private currentDecodedFrames: DecodedFrame[] = [];
  private tickHandle: number | null = null;
  private lastTickTime = 0;
  private lastGapMs = 0;
  private isFrozen = false;
  private isAtomicPackageCommit = false;
  private disposed = false;
  private listeners: Set<EventListener> = new Set();
  private finalizedIds: Map<string, FinalizedEntry> = new Map();
  private loadedActionCache: Map<string, LoadedAction> = new Map();
  private lastCleanupTime = 0;

  constructor(deps: AnimationEngineDeps) {
    this.state = createInitialState();
    this.surface = deps.surface;
    this.assetRepository = deps.assetRepository;
    this.clock = deps.clock ?? new MonotonicPlaybackClock();
    this.generationManager = new GenerationManagerImpl();
    this.queue = new ActionQueue({
      limits: { ...DEFAULT_QUEUE_LIMITS },
      resolveMutexGroup: (actionKey) => {
        const action = this.loadedActionCache.get(actionKey);
        return action?.mutexGroup ?? null;
      },
    });
    this.arbiter = new InterruptArbiter();
    this.gateway = new CommandGateway({
      queue: this.queue,
      arbiter: this.arbiter,
      getPackageRevision: () => this.state.packageRevision,
      getCurrentAction: () => this.state.currentAction,
      getCurrentLocalElapsedMs: () => this.state.localElapsedMs,
      isAtomicPackageCommit: () => this.isAtomicPackageCommit,
      hasAction: (actionKey) => {
        if (!this.packageSnapshot) return false;
        return this.packageSnapshot.actions.some((a) => a.actionKey === actionKey);
      },
    });
    this.returnResolver = new ReturnTargetResolver({
      getLoadedAction: (actionKey) => this.loadedActionCache.get(actionKey) ?? null,
      getDefaultActionKey: () => this.state.defaultActionKey,
      getPreviousStableActionKey: () => this.state.previousStableActionKey,
    });
    this.telemetry = new PlaybackTelemetry();
    this.recovery = new PlaybackRecoveryController();
  }

  onEvent(listener: EventListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  private emit(event: PlaybackEvent): void {
    if (this.disposed) return;
    for (const listener of this.listeners) {
      try {
        listener(event);
      } catch {
        void 0;
      }
    }
  }

  async initialize(snapshot: PackagePlaybackSnapshot): Promise<void> {
    this.assertNotDisposed();
    if (this.state.phase !== "uninitialized" && this.state.phase !== "recovering") {
      return;
    }

    this.packageSnapshot = snapshot;
    const generation = this.generationManager.next(snapshot.packageRevision);
    this.dispatch({ type: "INITIALIZE_STARTED", snapshot, generation: generation.generation });

    try {
      const loaded = await this.assetRepository.loadAction({
        packageSnapshot: snapshot,
        actionKey: snapshot.defaultActionKey,
        signal: generation.signal,
        priority: "critical",
      });

      if (!generation.isCurrent()) {
        this.releaseLoadedAssets(loaded);
        return;
      }

      const entry = this.createLoadedEntry(loaded);
      this.loadedActions.set(loaded.action.actionKey, entry);
      this.loadedActionCache.set(loaded.action.actionKey, loaded.action);
      this.currentLoadedEntry = entry;
      this.currentTimeline = entry.timeline;
      this.currentDecodedFrames = entry.frames;

      this.dispatch({ type: "DEFAULT_LOADED", action: loaded.action, frames: entry.frames, generation: generation.generation });

      this.surface.configureCanvas({
        width: snapshot.canvas.width,
        height: snapshot.canvas.height,
        scale: 1,
        interpolationMode: snapshot.interpolationMode ?? "smooth",
      });

      const firstFrame = entry.frames[0];
      if (firstFrame) {
        this.surface.present(firstFrame, {
          anchor: loaded.action.anchor,
          frameIndex: 0,
          actionKey: loaded.action.actionKey,
        });
      }

      this.startTickLoop();
      const loadTime = this.clock.now();
      this.telemetry.recordDefaultActionFirstFrame(loadTime);

      this.emit({
        type: "playback.action_started",
        actionKey: loaded.action.actionKey,
        frameIndex: 0,
        timestamp: Date.now(),
        packageId: snapshot.packageId,
        packageRevision: snapshot.packageRevision,
      });
    } catch (error) {
      if (PlaybackError.isAbort(error)) return;
      const pbError = PlaybackError.fromUnknown(error);
      this.dispatch({ type: "DEFAULT_LOAD_FAILED", error: pbError, generation: generation.generation });
      this.telemetry.recordError(pbError.toView());
      this.emit({
        type: "playback.action_failed",
        error: pbError.toView(),
        timestamp: Date.now(),
      });
      this.tryFallback();
    }
  }

  async playAction(command: PlayActionCommand): Promise<CommandAck> {
    this.assertNotDisposed();
    const now = this.clock.now();

    this.cleanupExpired(now);

    const result = this.gateway.processCommand(command, now);
    const ack = result.ack;

    switch (result.decision) {
      case "accept_and_load":
        await this.prepareAndSwitch(command, ack);
        break;
      case "queue":
        this.emit({
          type: "playback.command_queued",
          commandId: command.commandId,
          actionKey: command.actionKey,
          timestamp: Date.now(),
        });
        break;
      case "reject":
        this.telemetry.recordCommandReject();
        this.emit({
          type: "playback.command_rejected",
          commandId: command.commandId,
          actionKey: command.actionKey,
          reason: ack.reason ?? "rejected",
          timestamp: Date.now(),
        });
        break;
      case "expired":
        this.emit({
          type: "playback.action_expired",
          commandId: command.commandId,
          actionKey: command.actionKey,
          timestamp: Date.now(),
        });
        break;
      case "satisfied":
      case "duplicate":
        break;
    }

    return ack;
  }

  private async prepareAndSwitch(command: PlayActionCommand, ack: CommandAck): Promise<void> {
    const playbackInstanceId = createPlaybackInstanceId();
    const generation = this.generationManager.next(this.state.packageRevision);

    this.dispatch({
      type: "PLAY_ACCEPTED",
      command,
      playbackInstanceId,
      generation: generation.generation,
    });

    this.emit({
      type: "playback.action_loading",
      playbackInstanceId,
      commandId: command.commandId,
      actionKey: command.actionKey,
      timestamp: Date.now(),
    });

    const loadStart = this.clock.now();

    try {
      const loaded = await this.assetRepository.loadAction({
        packageSnapshot: this.packageSnapshot!,
        actionKey: command.actionKey,
        signal: generation.signal,
        priority: "critical",
      });

      if (!generation.isCurrent() || command.packageRevision !== this.state.packageRevision) {
        this.releaseLoadedAssets(loaded);
        return;
      }

      const entry = this.createLoadedEntry(loaded);
      this.loadedActions.set(loaded.action.actionKey, entry);
      this.loadedActionCache.set(loaded.action.actionKey, loaded.action);

      const oldActionKey = this.state.currentAction?.actionKey ?? null;
      if (oldActionKey && this.state.currentPlaybackInstanceId) {
        this.finalizePlayback(this.state.currentPlaybackInstanceId, "interrupted", "replaced", this.clock.now());
      }

      this.currentLoadedEntry = entry;
      this.currentTimeline = entry.timeline;
      this.currentDecodedFrames = entry.frames;

      this.dispatch({
        type: "ACTION_LOADED",
        action: loaded.action,
        frames: entry.frames,
        command,
        playbackInstanceId,
        generation: generation.generation,
      });

      const firstFrame = entry.frames[0];
      if (firstFrame) {
        this.surface.present(firstFrame, {
          anchor: loaded.action.anchor,
          frameIndex: 0,
          actionKey: loaded.action.actionKey,
        });
      }

      this.startTickLoop();

      const loadMs = this.clock.now() - loadStart;
      this.telemetry.recordActionLoad(command.actionKey, loadMs);
      this.telemetry.recordActionSwitchFirstFrame(command.actionKey, loadMs);
      this.telemetry.recordTransition(oldActionKey ?? "", command.actionKey, "replace");

      this.emit({
        type: "playback.action_started",
        playbackInstanceId,
        commandId: command.commandId,
        actionKey: command.actionKey,
        frameIndex: 0,
        timestamp: Date.now(),
        packageId: this.state.packageId ?? undefined,
        packageRevision: this.state.packageRevision,
      });
    } catch (error) {
      if (PlaybackError.isAbort(error)) return;
      const pbError = PlaybackError.fromUnknown(error);
      this.dispatch({ type: "ACTION_LOAD_FAILED", error: pbError, command, generation: generation.generation });
      this.telemetry.recordActionLoadFailure();
      this.telemetry.recordError(pbError.toView());
      this.emit({
        type: "playback.action_failed",
        playbackInstanceId,
        commandId: command.commandId,
        actionKey: command.actionKey,
        error: pbError.toView(),
        timestamp: Date.now(),
      });
    }
  }

  pause(): void {
    this.assertNotDisposed();
    const now = this.clock.now();
    this.dispatch({ type: "PAUSE", now });
    this.stopTickLoop();
  }

  resume(): void {
    this.assertNotDisposed();
    const now = this.clock.now();
    this.dispatch({ type: "RESUME", now });
    this.startTickLoop();
  }

  stop(): void {
    this.assertNotDisposed();
    const now = this.clock.now();
    if (this.state.currentPlaybackInstanceId) {
      this.finalizePlayback(this.state.currentPlaybackInstanceId, "interrupted", "user_disabled", now);
    }
    this.dispatch({ type: "ACTION_INTERRUPTED", reason: "user_disabled", now });
    this.stopTickLoop();
    this.processQueueOrReturn();
  }

  async switchPackage(snapshot: PackagePlaybackSnapshot): Promise<void> {
    this.assertNotDisposed();
    this.isAtomicPackageCommit = true;

    this.recovery.captureSnapshot(this.createRecoveryContext());

    const generation = this.generationManager.next(snapshot.packageRevision);
    this.dispatch({ type: "PACKAGE_SWITCH_STARTED", snapshot, generation: generation.generation });

    this.packageSnapshot = snapshot;
    this.stopTickLoop();

    try {
      const loaded = await this.assetRepository.loadAction({
        packageSnapshot: snapshot,
        actionKey: snapshot.defaultActionKey,
        signal: generation.signal,
        priority: "critical",
      });

      if (!generation.isCurrent()) {
        this.releaseLoadedAssets(loaded);
        return;
      }

      const entry = this.createLoadedEntry(loaded);
      this.loadedActions.clear();
      this.loadedActionCache.clear();
      this.loadedActions.set(loaded.action.actionKey, entry);
      this.loadedActionCache.set(loaded.action.actionKey, loaded.action);
      this.currentLoadedEntry = entry;
      this.currentTimeline = entry.timeline;
      this.currentDecodedFrames = entry.frames;

      this.dispatch({ type: "PACKAGE_SWITCH_COMMITTED", action: loaded.action, frames: entry.frames, generation: generation.generation });

      this.surface.configureCanvas({
        width: snapshot.canvas.width,
        height: snapshot.canvas.height,
        scale: 1,
        interpolationMode: snapshot.interpolationMode ?? "smooth",
      });

      const firstFrame = entry.frames[0];
      if (firstFrame) {
        this.surface.present(firstFrame, {
          anchor: loaded.action.anchor,
          frameIndex: 0,
          actionKey: loaded.action.actionKey,
        });
      }

      this.startTickLoop();

      this.emit({
        type: "playback.package_switched",
        packageId: snapshot.packageId,
        packageRevision: snapshot.packageRevision,
        actionKey: loaded.action.actionKey,
        timestamp: Date.now(),
      });
    } catch (error) {
      if (PlaybackError.isAbort(error)) return;
      const pbError = PlaybackError.fromUnknown(error);
      this.telemetry.recordError(pbError.toView());
      this.emit({
        type: "playback.action_failed",
        error: pbError.toView(),
        timestamp: Date.now(),
      });
      this.tryFallback();
    } finally {
      this.isAtomicPackageCommit = false;
    }
  }

  updateDefaultAction(actionKey: string, action: LoadedAction | null): void {
    this.assertNotDisposed();
    if (!this.packageSnapshot) return;

    this.dispatch({ type: "DEFAULT_CHANGED", newDefaultKey: actionKey, newDefaultAction: action });

    if (action) {
      this.loadedActionCache.set(actionKey, action);
    }

    this.emit({
      type: "playback.default_changed",
      actionKey,
      timestamp: Date.now(),
    });
  }

  onWindowHidden(): void {
    if (this.disposed) return;
    const now = this.clock.now();
    this.isFrozen = true;
    this.dispatch({ type: "WINDOW_HIDDEN", now });
    this.stopTickLoop();
  }

  onWindowShown(): void {
    if (this.disposed) return;
    const now = this.clock.now();
    this.isFrozen = false;
    this.dispatch({ type: "WINDOW_SHOWN", now });
    if (this.state.phase === "playing") {
      this.startTickLoop();
    }
  }

  onSystemSuspend(): void {
    if (this.disposed) return;
    const now = this.clock.now();
    this.dispatch({ type: "SUSPENDED", now });
    this.stopTickLoop();
  }

  onSystemResume(): void {
    if (this.disposed) return;
    const now = this.clock.now();
    this.dispatch({ type: "RESUMED_SYSTEM", now });
    if (this.state.phase === "playing" && !this.isFrozen) {
      this.startTickLoop();
    }
  }

  onRendererRecover(snapshot: PlaybackRecoverySnapshot): void {
    if (this.disposed) return;
    this.telemetry.recordRendererRecovery();
    this.dispatch({ type: "RECOVER", snapshot });
    if (this.packageSnapshot) {
      this.initialize(this.packageSnapshot);
    }
  }

  getSnapshot(): PlaybackSnapshot {
    return toSnapshot(this.state);
  }

  getDiagnostics(): AnimationDiagnostics {
    const cacheStats = {
      budgetBytes: DEFAULT_CACHE_BUDGET_BYTES,
      usedBytes: 0,
      entries: this.loadedActions.size,
    };
    return this.telemetry.getDiagnostics(
      toSnapshot(this.state),
      this.currentLoadedEntry
        ? {
            key: this.currentLoadedEntry.action.actionKey,
            loopType: this.currentLoadedEntry.action.loopType,
            frameCount: this.currentLoadedEntry.frames.length,
            cycleDurationMs: this.currentLoadedEntry.action.cycleDurationMs,
            loadedBytes: this.currentLoadedEntry.totalBytes,
          }
        : null,
      this.queue.toArray().map((q) => ({
        actionKey: q.command.actionKey,
        priority: q.command.priority,
        expiresAt: q.command.expiresAt,
      })),
      cacheStats,
      {
        visible: !this.isFrozen,
        suspended: this.state.phase === "paused",
        lastGapMs: this.lastGapMs,
      },
    );
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    this.stopTickLoop();
    this.dispatch({ type: "DISPOSE" });
    this.surface.dispose();
    this.queue.clear();
    this.loadedActions.clear();
    this.loadedActionCache.clear();
    this.finalizedIds.clear();
    this.listeners.clear();
    this.generationManager.markCurrentStale();
  }

  isDisposed(): boolean {
    return this.disposed;
  }

  getPhase(): PlayerPhase {
    return this.state.phase;
  }

  private startTickLoop(): void {
    if (this.disposed) return;
    if (this.state.phase !== "playing") return;
    if (this.tickHandle !== null) return;
    this.lastTickTime = this.clock.now();
    this.tickHandle = this.clock.requestTick((now) => this.onTick(now));
  }

  private stopTickLoop(): void {
    if (this.tickHandle !== null) {
      this.clock.cancelTick(this.tickHandle);
      this.tickHandle = null;
    }
  }

  private onTick(now: number): void {
    if (this.disposed) return;
    if (this.state.phase !== "playing") return;
    if (!this.currentTimeline || !this.state.currentAction) {
      this.tickHandle = null;
      return;
    }

    const delta = now - this.lastTickTime;
    this.lastTickTime = now;

    if (delta > CLOCK_LARGE_GAP_THRESHOLD_MS) {
      this.lastGapMs = delta;
      this.telemetry.recordClockLargeGap();
    }

    const localElapsed = this.computeLocalElapsed(now);
    const position = this.currentTimeline.locate(localElapsed, this.state.currentAction.loopType);

    this.dispatch({ type: "TICK", now, position });

    if (position.completed) {
      this.handleActionComplete(now);
      return;
    }

    if (
      this.state.currentAction.maximumPlayMs !== null &&
      localElapsed >= this.state.currentAction.maximumPlayMs
    ) {
      this.handleMaxDurationReached(now);
      return;
    }

    const frameIndex = position.frameIndex;
    if (frameIndex >= 0 && frameIndex < this.currentDecodedFrames.length) {
      const frame = this.currentDecodedFrames[frameIndex];
      if (frame && this.state.lastPresentedFrameIndex !== frameIndex) {
        const presentStart = this.clock.now();
        this.surface.present(frame, {
          anchor: this.state.currentAction.anchor,
          frameIndex,
          actionKey: this.state.currentAction.actionKey,
        });
        this.telemetry.recordFramePresent(this.clock.now() - presentStart);
      }
    }

    this.tickHandle = this.clock.requestTick((t) => this.onTick(t));
  }

  private computeLocalElapsed(now: number): number {
    const state = this.state;
    if (state.startMonotonicMs === 0) return 0;
    let elapsed = now - state.startMonotonicMs - state.pausedDurationMs;
    if (state.pauseStartMonotonicMs !== null) {
      elapsed -= Math.max(0, now - state.pauseStartMonotonicMs);
    }
    elapsed *= state.playbackRate;
    return Math.max(0, elapsed);
  }

  private handleActionComplete(now: number): void {
    const instanceId = this.state.currentPlaybackInstanceId;
    const action = this.state.currentAction;
    if (!action) return;

    if (action.loopType === "hold") {
      this.dispatch({ type: "HOLD_ENTERED" });
      this.stopTickLoop();
      if (instanceId) {
        this.finalizePlayback(instanceId, "completed", "natural_end", now);
      }
      this.emit({
        type: "playback.action_holding",
        playbackInstanceId: instanceId ?? undefined,
        actionKey: action.actionKey,
        frameIndex: this.state.frameIndex ?? undefined,
        timestamp: Date.now(),
      });
      return;
    }

    if (instanceId) {
      this.finalizePlayback(instanceId, "completed", "natural_end", now);
    }

    this.dispatch({ type: "ACTION_COMPLETED", reason: "natural_end", now });

    this.emit({
      type: "playback.action_completed",
      playbackInstanceId: instanceId ?? undefined,
      actionKey: action.actionKey,
      reason: "natural_end",
      playedDurationMs: this.state.localElapsedMs,
      presentedFrames: this.state.presentedFrames,
      droppedFramesEstimate: this.state.droppedFramesEstimate,
      timestamp: Date.now(),
    });

    this.telemetry.recordTransition(action.actionKey, "", "completed");
    this.processQueueOrReturn();
  }

  private handleMaxDurationReached(now: number): void {
    const instanceId = this.state.currentPlaybackInstanceId;
    const action = this.state.currentAction;
    if (!action) return;

    if (instanceId) {
      this.finalizePlayback(instanceId, "interrupted", "max_duration_reached", now);
    }

    this.dispatch({ type: "ACTION_INTERRUPTED", reason: "max_duration_reached", now });

    this.emit({
      type: "playback.action_interrupted",
      playbackInstanceId: instanceId ?? undefined,
      actionKey: action.actionKey,
      reason: "max_duration_reached",
      playedDurationMs: this.state.localElapsedMs,
      timestamp: Date.now(),
    });

    this.processQueueOrReturn();
  }

  private processQueueOrReturn(): void {
    const next = this.queue.dequeue();
    if (next) {
      this.playAction(next.command);
      return;
    }

    const action = this.state.currentAction;
    if (!action) {
      this.returnToDefault();
      return;
    }

    const resolveResult = this.returnResolver.resolve({
      actionReturnTarget: action.returnTarget,
      queueHasItems: false,
    });

    if (resolveResult.targetActionKey) {
      this.loadAndPlayReturn(resolveResult.targetActionKey);
    } else if (resolveResult.target.type === "none") {
      this.stopTickLoop();
    } else {
      this.returnToDefault();
    }
  }

  private async loadAndPlayReturn(actionKey: string): Promise<void> {
    if (!this.packageSnapshot) return;
    if (actionKey === this.state.currentAction?.actionKey) {
      this.returnToDefault();
      return;
    }

    const generation = this.generationManager.next(this.state.packageRevision);

    try {
      const loaded = await this.assetRepository.loadAction({
        packageSnapshot: this.packageSnapshot,
        actionKey,
        signal: generation.signal,
        priority: "high",
      });

      if (!generation.isCurrent()) {
        this.releaseLoadedAssets(loaded);
        return;
      }

      const entry = this.createLoadedEntry(loaded);
      this.loadedActions.set(actionKey, entry);
      this.loadedActionCache.set(actionKey, loaded.action);
      this.currentLoadedEntry = entry;
      this.currentTimeline = entry.timeline;
      this.currentDecodedFrames = entry.frames;

      this.dispatch({
        type: "ACTION_LOADED",
        action: loaded.action,
        frames: entry.frames,
        command: {
          commandId: `return_${Date.now()}`,
          idempotencyKey: `return:${actionKey}:${Date.now()}`,
          installationId: "",
          petInstanceId: "",
          packageRevision: this.state.packageRevision,
          actionKey,
          priority: loaded.action.defaultPriority,
          queuePolicy: "replace_current",
          interruptPolicy: "force_system",
          playbackRate: 1,
          issuedAt: new Date().toISOString(),
        },
        playbackInstanceId: createPlaybackInstanceId(),
        generation: generation.generation,
      });

      const firstFrame = entry.frames[0];
      if (firstFrame) {
        this.surface.present(firstFrame, {
          anchor: loaded.action.anchor,
          frameIndex: 0,
          actionKey: loaded.action.actionKey,
        });
      }

      this.startTickLoop();

      this.emit({
        type: "playback.action_started",
        actionKey: loaded.action.actionKey,
        frameIndex: 0,
        timestamp: Date.now(),
        packageId: this.state.packageId ?? undefined,
        packageRevision: this.state.packageRevision,
      });
    } catch (error) {
      if (PlaybackError.isAbort(error)) return;
      this.returnToDefault();
    }
  }

  private async returnToDefault(): Promise<void> {
    if (!this.packageSnapshot || !this.state.defaultActionKey) {
      this.stopTickLoop();
      return;
    }

    const defaultKey = this.state.defaultActionKey;
    const cached = this.loadedActions.get(defaultKey);
    if (cached) {
      this.currentLoadedEntry = cached;
      this.currentTimeline = cached.timeline;
      this.currentDecodedFrames = cached.frames;

      this.dispatch({
        type: "ACTION_LOADED",
        action: cached.action,
        frames: cached.frames,
        command: {
          commandId: `default_${Date.now()}`,
          idempotencyKey: `default:${defaultKey}:${Date.now()}`,
          installationId: "",
          petInstanceId: "",
          packageRevision: this.state.packageRevision,
          actionKey: defaultKey,
          priority: cached.action.defaultPriority,
          queuePolicy: "replace_current",
          interruptPolicy: "force_system",
          playbackRate: 1,
          issuedAt: new Date().toISOString(),
        },
        playbackInstanceId: createPlaybackInstanceId(),
        generation: this.generationManager.current()?.generation ?? 0,
      });

      const firstFrame = cached.frames[0];
      if (firstFrame) {
        this.surface.present(firstFrame, {
          anchor: cached.action.anchor,
          frameIndex: 0,
          actionKey: cached.action.actionKey,
        });
      }

      this.startTickLoop();
      return;
    }

    await this.loadAndPlayReturn(defaultKey);
  }

  private tryFallback(): void {
    this.telemetry.recordFallback();
    if (!this.packageSnapshot) return;

    this.emit({
      type: "playback.fallback_started",
      timestamp: Date.now(),
    });

    if (this.packageSnapshot.previewUrl) {
      this.emit({
        type: "playback.fallback_failed",
        error: {
          code: PLAYBACK_ERROR_CODES.FALLBACK_FAILED,
          message: "default action load failed",
        },
        timestamp: Date.now(),
      });
    }
  }

  private finalizePlayback(
    instanceId: string,
    type: "completed" | "interrupted",
    reason: string,
    now: number,
  ): void {
    const existing = this.finalizedIds.get(instanceId);
    if (existing) return;

    this.finalizedIds.set(instanceId, { timestamp: now });
    this.cleanupExpired(now);

    if (type === "interrupted") {
      this.emit({
        type: "playback.action_interrupted",
        playbackInstanceId: instanceId,
        reason,
        playedDurationMs: this.state.localElapsedMs,
        timestamp: Date.now(),
      });
    }
  }

  private cleanupExpired(now: number): void {
    if (now - this.lastCleanupTime < FINALIZED_SET_TTL_MS / 2) return;
    this.lastCleanupTime = now;

    const cutoff = now - FINALIZED_SET_TTL_MS;
    for (const [id, entry] of this.finalizedIds) {
      if (entry.timestamp < cutoff) {
        this.finalizedIds.delete(id);
      }
    }

    if (this.finalizedIds.size > FINALIZED_SET_MAX_ENTRIES) {
      const sorted = [...this.finalizedIds.entries()].sort(
        (a, b) => a[1].timestamp - b[1].timestamp,
      );
      const toRemove = sorted.length - FINALIZED_SET_MAX_ENTRIES;
      for (let i = 0; i < toRemove; i++) {
        this.finalizedIds.delete(sorted[i][0]);
      }
    }
  }

  private createLoadedEntry(loaded: LoadedActionAssets): LoadedActionEntry {
    const timeline = createFrameTimeline(loaded.action.frames);
    const totalBytes = loaded.totalEstimatedBytes;
    return {
      action: loaded.action,
      frames: [...loaded.decodedFrames],
      timeline,
      totalBytes,
    };
  }

  private releaseLoadedAssets(loaded: LoadedActionAssets): void {
    for (const frame of loaded.decodedFrames) {
      if (frame.bitmap && typeof (frame.bitmap as ImageBitmap).close === "function") {
        try {
          (frame.bitmap as ImageBitmap).close();
        } catch {
          void 0;
        }
      }
    }
  }

  private createRecoveryContext() {
    return {
      getCurrentPhase: () => this.state.phase,
      getPackageSnapshot: () => this.packageSnapshot,
      getCurrentAction: () => this.state.currentAction,
      getPreviousStableActionKey: () => this.state.previousStableActionKey,
      getLocalElapsedMs: () => this.state.localElapsedMs,
      getCycleIndex: () => this.state.cycleIndex,
    };
  }

  private dispatch(action: StateAction): void {
    this.state = playerReducer(this.state, action);
  }

  private assertNotDisposed(): void {
    if (this.disposed) {
      throw new PlaybackError(
        PLAYBACK_ERROR_CODES.ENGINE_DISPOSED,
        "animation engine is disposed",
      );
    }
  }
}
