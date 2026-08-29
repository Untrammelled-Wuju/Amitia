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
  // Fence callbacks from an older playback loop. Platform cancellation can race
  // with an already queued callback, so callback identity must be explicit.
  private tickLoopEpoch = 0;
  private lastTickTime = 0;
  private lastGapMs = 0;
  private isFrozen = false;
  private isAtomicPackageCommit = false;
  private disposed = false;
  private listeners: Set<EventListener> = new Set();
  private finalizedIds: Map<string, FinalizedEntry> = new Map();
  private firstCycleReportedIds: Set<string> = new Set();
  private acceptedCommands: Map<string, PlayActionCommand> = new Map();
  private loadedActionCache: Map<string, LoadedAction> = new Map();
  private fallbackFrame: DecodedFrame | null = null;
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
      const firstFrame = entry.frames[0];
      if (!firstFrame) {
        this.releaseLoadedAssets(loaded);
        throw new PlaybackError(
          PLAYBACK_ERROR_CODES.SURFACE_FAILED,
          `default action has no decoded frame: ${loaded.action.actionKey}`,
          { actionKey: loaded.action.actionKey },
        );
      }

      this.surface.configureCanvas({
        width: snapshot.canvas.width,
        height: snapshot.canvas.height,
        scale: 1,
        interpolationMode: snapshot.interpolationMode ?? "smooth",
      });

      const presentResult = await this.surface.present(firstFrame, {
        anchor: loaded.action.anchor,
        frameIndex: 0,
        actionKey: loaded.action.actionKey,
      });
      if (!presentResult.presented) {
        this.releaseLoadedAssets(loaded);
        throw new PlaybackError(
          PLAYBACK_ERROR_CODES.SURFACE_FAILED,
          `default first frame was not presented: ${presentResult.error ?? "unknown surface error"}`,
          {
            actionKey: loaded.action.actionKey,
            frameIndex: 0,
            resourceUrl: firstFrame.sourceUrl,
          },
        );
      }

      this.releaseFallbackFrame();
      this.loadedActions.set(loaded.action.actionKey, entry);
      this.loadedActionCache.set(loaded.action.actionKey, loaded.action);
      this.currentLoadedEntry = entry;
      this.currentTimeline = entry.timeline;
      this.currentDecodedFrames = entry.frames;

      // Commit the state only after a real frame has been presented. RuntimeReady
      // is emitted by the renderer after initialize() resolves, so this ordering
      // prevents an invisible/failed package from being reported as ready.
      this.dispatch({
        type: "DEFAULT_LOADED",
        action: loaded.action,
        frames: entry.frames,
        generation: generation.generation,
        now: this.clock.now(),
      });

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
      await this.tryFallback(pbError, generation);
      // A preview fallback keeps an already-visible runtime from becoming blank,
      // but it is not a successfully initialized default action. Propagate the
      // failure so the startup handshake cannot emit a false RuntimeReady.
      throw pbError;
    }
  }

  async playAction(command: PlayActionCommand): Promise<CommandAck> {
    this.assertNotDisposed();
    const now = this.clock.now();
    this.cleanupExpired(now);

    const result = this.gateway.processCommand(command, now);
    const ack = result.ack;

    if (result.decision === "accept_and_load" || result.decision === "queue" || result.decision === "satisfied") {
      this.acceptedCommands.set(command.playbackInstanceId, command);
      this.emit({
        type: "playback.command_accepted",
        playbackInstanceId: command.playbackInstanceId,
        commandId: command.commandId,
        actionKey: command.actionKey,
        timestamp: Date.now(),
      });
    }
    this.drainQueueRemovals();

    switch (result.decision) {
      case "accept_and_load":
        await this.prepareAndSwitch(command, ack);
        break;
      case "queue":
        this.emit({
          type: "playback.command_queued",
          playbackInstanceId: command.playbackInstanceId,
          commandId: command.commandId,
          actionKey: command.actionKey,
          timestamp: Date.now(),
        });
        break;
      case "reject":
        this.telemetry.recordCommandReject();
        this.emit({
          type: "playback.command_rejected",
          playbackInstanceId: command.playbackInstanceId,
          commandId: command.commandId,
          actionKey: command.actionKey,
          reason: ack.reason ?? "rejected",
          timestamp: Date.now(),
        });
        this.emitCommandFailure(command, "PLAYBACK_COMMAND_REJECTED", ack.reason ?? "renderer rejected command");
        break;
      case "expired":
        this.emit({
          type: "playback.action_expired",
          playbackInstanceId: command.playbackInstanceId,
          commandId: command.commandId,
          actionKey: command.actionKey,
          timestamp: Date.now(),
        });
        this.emitCommandFailure(command, "PLAYBACK_COMMAND_EXPIRED", "renderer command expired before execution");
        break;
      case "satisfied":
        this.acceptedCommands.set(command.playbackInstanceId, command);
        this.finalizePlayback(
          command.playbackInstanceId,
          "completed",
          ack.reason ?? "already_satisfied",
          now,
          { commandId: command.commandId, actionKey: command.actionKey, playedDurationMs: 0 },
        );
        break;
      case "duplicate":
        // The original command owns lifecycle completion. Replays do not emit a
        // second terminal event.
        break;
    }

    return ack;
  }

  private async prepareAndSwitch(command: PlayActionCommand, _ack: CommandAck): Promise<void> {
    const playbackInstanceId = command.playbackInstanceId;
    this.acceptedCommands.set(playbackInstanceId, command);

    // A new generation cancels any previous in-flight preparation. The active
    // playback identity remains untouched until the replacement is presented.
    const generation = this.generationManager.next(this.state.packageRevision);
    this.stopTickLoop();
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
      if (!this.packageSnapshot) {
        throw new PlaybackError(
          PLAYBACK_ERROR_CODES.INTERNAL_STATE_INVALID,
          "package snapshot unavailable during action preparation",
          { playbackInstanceId, commandId: command.commandId, actionKey: command.actionKey },
        );
      }
      const loaded = await this.assetRepository.loadAction({
        packageSnapshot: this.packageSnapshot,
        actionKey: command.actionKey,
        signal: generation.signal,
        priority: "critical",
      });

      if (!generation.isCurrent() || command.packageRevision !== this.state.packageRevision) {
        this.releaseLoadedAssets(loaded);
        this.emitCommandFailure(command, "PLAYBACK_COMMAND_CANCELLED", "playback preparation superseded by a newer generation");
        return;
      }

      const effectiveAction = this.applyCommandOverrides(loaded.action, command);
      const entry = this.createLoadedEntry({ ...loaded, action: effectiveAction });
      if (command.expiresAt || command.requiresAuthoritativeExpiry === true) {
        const expiresAtMs = command.expiresAt ? Date.parse(command.expiresAt) : Number.NaN;
        if (!Number.isFinite(expiresAtMs) || expiresAtMs <= Date.now()) {
          this.releaseLoadedAssets(loaded);
          this.emitCommandFailure(
            command,
            "PLAYBACK_COMMAND_EXPIRED",
            "playback command expired while loading assets",
          );
          return;
        }
      }
      const firstFrame = entry.frames[0];
      if (!firstFrame) {
        this.releaseLoadedAssets(loaded);
        throw new PlaybackError(
          PLAYBACK_ERROR_CODES.SURFACE_FAILED,
          `action has no decoded frame: ${command.actionKey}`,
          { actionKey: command.actionKey, playbackInstanceId, commandId: command.commandId },
        );
      }

      const oldAction = this.state.currentAction;
      const oldPlaybackId = this.state.currentPlaybackInstanceId;
      const oldCommandId = this.state.currentCommandId;
      const oldDuration = this.state.localElapsedMs;

      // Present the replacement first frame before terminalizing the active
      // playback. A failed replacement must leave the previous action active
      // and resumable rather than creating a blank/ready gap.
      const presentResult = await this.surface.present(firstFrame, {
        anchor: effectiveAction.anchor,
        frameIndex: 0,
        actionKey: effectiveAction.actionKey,
      });
      if (!presentResult.presented) {
        this.releaseLoadedAssets(loaded);
        throw new PlaybackError(
          PLAYBACK_ERROR_CODES.SURFACE_FAILED,
          `action first frame was not presented: ${presentResult.error ?? "unknown surface error"}`,
          {
            actionKey: command.actionKey,
            frameIndex: 0,
            resourceUrl: firstFrame.sourceUrl,
            playbackInstanceId,
            commandId: command.commandId,
          },
        );
      }
      if (!generation.isCurrent() || command.packageRevision !== this.state.packageRevision) {
        this.releaseLoadedAssets(loaded);
        this.emitCommandFailure(command, "PLAYBACK_COMMAND_CANCELLED", "playback superseded after first-frame presentation");
        return;
      }

      // The replacement is now visibly present. Only at this point may the old
      // active identity be terminalized and cleared. Keep the pending identity
      // by re-accepting after ACTION_INTERRUPTED resets the active reducer state.
      if (oldAction) {
        const interruptedAt = this.clock.now();
        if (oldPlaybackId) {
          this.finalizePlayback(oldPlaybackId, "interrupted", "replaced", interruptedAt, {
            commandId: oldCommandId ?? undefined,
            actionKey: oldAction.actionKey,
            playedDurationMs: oldDuration,
          });
        } else {
          this.emit({
            type: "playback.action_interrupted",
            actionKey: oldAction.actionKey,
            reason: "replaced",
            playedDurationMs: oldDuration,
            timestamp: Date.now(),
          });
        }
        this.dispatch({ type: "ACTION_INTERRUPTED", reason: "replaced", now: interruptedAt });
        this.dispatch({
          type: "PLAY_ACCEPTED",
          command,
          playbackInstanceId,
          generation: generation.generation,
        });
      }

      this.loadedActions.set(loaded.action.actionKey, entry);
      this.loadedActionCache.set(loaded.action.actionKey, loaded.action);
      this.currentLoadedEntry = entry;
      this.currentTimeline = entry.timeline;
      this.currentDecodedFrames = entry.frames;

      const startedAt = this.clock.now();
      this.dispatch({
        type: "ACTION_LOADED",
        action: effectiveAction,
        frames: entry.frames,
        command,
        playbackInstanceId,
        generation: generation.generation,
        now: startedAt,
      });
      this.startTickLoop();

      const loadMs = startedAt - loadStart;
      this.telemetry.recordActionLoad(command.actionKey, loadMs);
      this.telemetry.recordActionSwitchFirstFrame(command.actionKey, loadMs);
      this.telemetry.recordTransition(oldAction?.actionKey ?? "", command.actionKey, "replace");

      this.emit({
        type: "playback.action_started",
        playbackInstanceId,
        commandId: command.commandId,
        actionKey: command.actionKey,
        frameIndex: 0,
        timestamp: Date.now(),
        packageId: this.state.packageId ?? undefined,
        packageRevision: this.state.packageRevision,
        traceId: command.traceId,
      });
    } catch (error) {
      const isAbort = PlaybackError.isAbort(error);
      const pbError = isAbort
        ? new PlaybackError(
            PLAYBACK_ERROR_CODES.COMMAND_INVALID,
            "playback preparation cancelled",
            { playbackInstanceId, commandId: command.commandId, actionKey: command.actionKey },
          )
        : PlaybackError.fromUnknown(error);
      if (this.state.pendingPlaybackInstanceId === playbackInstanceId) {
        this.dispatch({ type: "ACTION_LOAD_FAILED", error: pbError, command, generation: generation.generation });
      }
      if (this.state.currentAction) this.startTickLoop();
      this.telemetry.recordActionLoadFailure();
      this.telemetry.recordError(pbError.toView());
      this.emitCommandFailureView(command, pbError.toView());
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

  stop(reason: InterruptionReason = "user_disabled"): void {
    this.assertNotDisposed();
    const now = this.clock.now();
    this.stopTickLoop();

    const activeAction = this.state.currentAction;
    const activePlaybackId = this.state.currentPlaybackInstanceId;
    const activeCommandId = this.state.currentCommandId;
    const activeDuration = this.state.localElapsedMs;
    if (activeAction) {
      if (activePlaybackId) {
        this.finalizePlayback(activePlaybackId, "interrupted", reason, now, {
          commandId: activeCommandId ?? undefined,
          actionKey: activeAction.actionKey,
          playedDurationMs: activeDuration,
        });
      } else {
        this.emit({
          type: "playback.action_interrupted",
          actionKey: activeAction.actionKey,
          reason,
          playedDurationMs: activeDuration,
          timestamp: Date.now(),
        });
      }
    }

    const pendingPlaybackId = this.state.pendingPlaybackInstanceId;
    if (pendingPlaybackId && pendingPlaybackId !== activePlaybackId) {
      const pending = this.acceptedCommands.get(pendingPlaybackId);
      if (pending) this.emitCommandFailure(pending, "PLAYBACK_COMMAND_CANCELLED", `playback stopped: ${reason}`);
    }

    this.queue.clear();
    this.drainQueueRemovals(`playback stopped: ${reason}`);
    this.dispatch({ type: "ACTION_INTERRUPTED", reason, now });
  }

  async switchPackage(snapshot: PackagePlaybackSnapshot): Promise<void> {
    this.assertNotDisposed();
    this.isAtomicPackageCommit = true;
    this.recovery.captureSnapshot(this.createRecoveryContext());
    this.stopTickLoop();

    // Terminalize the old generation before clearing/replacing its identity.
    const now = this.clock.now();
    const activeAction = this.state.currentAction;
    const activePlaybackId = this.state.currentPlaybackInstanceId;
    if (activeAction && activePlaybackId) {
      this.finalizePlayback(activePlaybackId, "interrupted", "package_switch", now, {
        commandId: this.state.currentCommandId ?? undefined,
        actionKey: activeAction.actionKey,
        playedDurationMs: this.state.localElapsedMs,
      });
    }
    const pendingPlaybackId = this.state.pendingPlaybackInstanceId;
    if (pendingPlaybackId && pendingPlaybackId !== activePlaybackId) {
      const pending = this.acceptedCommands.get(pendingPlaybackId);
      if (pending) this.emitCommandFailure(pending, "PLAYBACK_COMMAND_CANCELLED", "package switch cancelled playback preparation");
    }
    this.queue.clear();
    this.drainQueueRemovals("package switch evicted queued playback");

    const generation = this.generationManager.next(snapshot.packageRevision);
    this.dispatch({ type: "PACKAGE_SWITCH_STARTED", snapshot, generation: generation.generation });
    this.packageSnapshot = snapshot;

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
      const firstFrame = entry.frames[0];
      if (!firstFrame) {
        this.releaseLoadedAssets(loaded);
        throw new PlaybackError(
          PLAYBACK_ERROR_CODES.SURFACE_FAILED,
          `package default action has no decoded frame: ${loaded.action.actionKey}`,
          { actionKey: loaded.action.actionKey },
        );
      }

      this.surface.configureCanvas({
        width: snapshot.canvas.width,
        height: snapshot.canvas.height,
        scale: 1,
        interpolationMode: snapshot.interpolationMode ?? "smooth",
      });

      const presentResult = await this.surface.present(firstFrame, {
        anchor: loaded.action.anchor,
        frameIndex: 0,
        actionKey: loaded.action.actionKey,
      });
      if (!presentResult.presented) {
        this.releaseLoadedAssets(loaded);
        throw new PlaybackError(
          PLAYBACK_ERROR_CODES.SURFACE_FAILED,
          `package first frame was not presented: ${presentResult.error ?? "unknown surface error"}`,
          { actionKey: loaded.action.actionKey, frameIndex: 0, resourceUrl: firstFrame.sourceUrl },
        );
      }

      this.releaseFallbackFrame();
      this.loadedActions.clear();
      this.loadedActionCache.clear();
      this.loadedActions.set(loaded.action.actionKey, entry);
      this.loadedActionCache.set(loaded.action.actionKey, loaded.action);
      this.currentLoadedEntry = entry;
      this.currentTimeline = entry.timeline;
      this.currentDecodedFrames = entry.frames;

      this.dispatch({
        type: "PACKAGE_SWITCH_COMMITTED",
        action: loaded.action,
        frames: entry.frames,
        generation: generation.generation,
        now: this.clock.now(),
      });
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
      this.emit({ type: "playback.action_failed", error: pbError.toView(), timestamp: Date.now() });
      await this.tryFallback(pbError, generation);
      throw pbError;
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

  async onRendererRecover(snapshot: PlaybackRecoverySnapshot): Promise<void> {
    if (this.disposed) return;
    this.telemetry.recordRendererRecovery();
    this.dispatch({ type: "RECOVER", snapshot });
    if (!this.packageSnapshot) return;
    try {
      await this.initialize(this.packageSnapshot);
    } catch (error) {
      const pbError = PlaybackError.fromUnknown(error);
      this.telemetry.recordError(pbError.toView());
      console.error("[AnimationEngine] renderer recovery failed", pbError);
      throw pbError;
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
    this.stopTickLoop();

    const now = this.clock.now();
    const active = this.state.currentAction;
    const activeId = this.state.currentPlaybackInstanceId;
    if (active && activeId) {
      this.finalizePlayback(activeId, "interrupted", "window_destroyed", now, {
        commandId: this.state.currentCommandId ?? undefined,
        actionKey: active.actionKey,
        playedDurationMs: this.state.localElapsedMs,
      });
    }
    const pendingId = this.state.pendingPlaybackInstanceId;
    if (pendingId && pendingId !== activeId) {
      const pending = this.acceptedCommands.get(pendingId);
      if (pending) this.emitCommandFailure(pending, "PLAYBACK_COMMAND_CANCELLED", "renderer disposed during preparation");
    }
    this.queue.clear();
    this.drainQueueRemovals("renderer disposed with queued command");

    this.disposed = true;
    this.dispatch({ type: "DISPOSE" });
    this.releaseFallbackFrame();
    this.surface.dispose();
    this.loadedActions.clear();
    this.loadedActionCache.clear();
    this.finalizedIds.clear();
    this.firstCycleReportedIds.clear();
    this.acceptedCommands.clear();
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
    const epoch = ++this.tickLoopEpoch;
    this.tickHandle = this.clock.requestTick((now) => this.onTick(now, epoch));
  }

  private stopTickLoop(): void {
    // Invalidate the closure even when cancelTick races with a callback that is
    // already queued for execution.
    this.tickLoopEpoch += 1;
    if (this.tickHandle !== null) {
      this.clock.cancelTick(this.tickHandle);
      this.tickHandle = null;
    }
  }

  private onTick(now: number, epoch: number): void {
    // A stale callback belongs to an older playback loop. It must not clear or
    // reschedule the handle owned by the current generation.
    if (epoch !== this.tickLoopEpoch) return;

    // The current scheduled handle has fired; clear it before early returns so
    // this same generation may establish its next frame.
    this.tickHandle = null;
    if (this.disposed) return;
    if (this.state.phase !== "playing") return;
    if (!this.currentTimeline || !this.state.currentAction) return;

    const delta = now - this.lastTickTime;
    this.lastTickTime = now;

    if (delta > CLOCK_LARGE_GAP_THRESHOLD_MS) {
      this.lastGapMs = delta;
      this.telemetry.recordClockLargeGap();
    }

    const localElapsed = this.computeLocalElapsed(now);
    const position = this.currentTimeline.locate(localElapsed, this.state.currentAction.loopType);
    this.dispatch({ type: "TICK", now, position });

    const activePlaybackId = this.state.currentPlaybackInstanceId;
    if (activePlaybackId && position.cycleIndex >= 1 && !this.firstCycleReportedIds.has(activePlaybackId)) {
      this.firstCycleReportedIds.add(activePlaybackId);
      this.emit({
        type: "playback.action_first_cycle",
        playbackInstanceId: activePlaybackId,
        commandId: this.state.currentCommandId ?? undefined,
        actionKey: this.state.currentAction.actionKey,
        timestamp: Date.now(),
      });
    }

    if (position.completed) {
      this.handleActionComplete(now);
      return;
    }

    if (this.state.currentAction.maximumPlayMs !== null && localElapsed >= this.state.currentAction.maximumPlayMs) {
      this.handleMaxDurationReached(now);
      return;
    }

    const frameIndex = position.frameIndex;
    if (frameIndex >= 0 && frameIndex < this.currentDecodedFrames.length) {
      const frame = this.currentDecodedFrames[frameIndex];
      if (frame && this.state.lastPresentedFrameIndex !== frameIndex) {
        const presentStart = this.clock.now();
        void Promise.resolve(this.surface.present(frame, {
          anchor: this.state.currentAction.anchor,
          frameIndex,
          actionKey: this.state.currentAction.actionKey,
        })).catch(() => undefined);
        this.telemetry.recordFramePresent(this.clock.now() - presentStart);
      }
    }

    if (epoch !== this.tickLoopEpoch || this.state.phase !== "playing") return;
    this.tickHandle = this.clock.requestTick((t) => this.onTick(t, epoch));
  }

  private computeLocalElapsed(now: number): number {
    const state = this.state;
    if (state.startMonotonicMs < 0) return 0;
    let elapsed = now - state.startMonotonicMs - state.pausedDurationMs;
    if (state.pauseStartMonotonicMs !== null) {
      elapsed -= Math.max(0, now - state.pauseStartMonotonicMs);
    }
    elapsed *= state.playbackRate;
    return Math.max(0, elapsed);
  }

  private handleActionComplete(now: number): void {
    const instanceId = this.state.currentPlaybackInstanceId;
    const commandId = this.state.currentCommandId;
    const action = this.state.currentAction;
    if (!action) return;
    const playedDurationMs = this.state.localElapsedMs;
    const presentedFrames = this.state.presentedFrames;
    const droppedFramesEstimate = this.state.droppedFramesEstimate;

    if (action.loopType === "hold") {
      this.dispatch({ type: "HOLD_ENTERED" });
      this.stopTickLoop();
      if (instanceId) {
        this.finalizePlayback(instanceId, "completed", "natural_end", now, {
          commandId: commandId ?? undefined,
          actionKey: action.actionKey,
          playedDurationMs,
          presentedFrames,
          droppedFramesEstimate,
        });
      }
      this.emit({
        type: "playback.action_holding",
        playbackInstanceId: instanceId ?? undefined,
        commandId: commandId ?? undefined,
        actionKey: action.actionKey,
        frameIndex: this.state.frameIndex ?? undefined,
        timestamp: Date.now(),
      });
      return;
    }

    if (instanceId) {
      this.finalizePlayback(instanceId, "completed", "natural_end", now, {
        commandId: commandId ?? undefined,
        actionKey: action.actionKey,
        playedDurationMs,
        presentedFrames,
        droppedFramesEstimate,
      });
    } else {
      this.emit({
        type: "playback.action_completed",
        actionKey: action.actionKey,
        reason: "natural_end",
        playedDurationMs,
        presentedFrames,
        droppedFramesEstimate,
        timestamp: Date.now(),
      });
    }

    this.dispatch({ type: "ACTION_COMPLETED", reason: "natural_end", now });
    this.telemetry.recordTransition(action.actionKey, "", "completed");
    this.processQueueOrReturn(action.returnTarget);
  }

  private handleMaxDurationReached(now: number): void {
    const instanceId = this.state.currentPlaybackInstanceId;
    const commandId = this.state.currentCommandId;
    const action = this.state.currentAction;
    if (!action) return;
    const playedDurationMs = this.state.localElapsedMs;

    if (instanceId) {
      this.finalizePlayback(instanceId, "interrupted", "max_duration_reached", now, {
        commandId: commandId ?? undefined,
        actionKey: action.actionKey,
        playedDurationMs,
      });
    } else {
      this.emit({
        type: "playback.action_interrupted",
        actionKey: action.actionKey,
        reason: "max_duration_reached",
        playedDurationMs,
        timestamp: Date.now(),
      });
    }

    this.dispatch({ type: "ACTION_INTERRUPTED", reason: "max_duration_reached", now });
    this.processQueueOrReturn(action.returnTarget);
  }

  private processQueueOrReturn(returnTarget?: ReturnTarget): void {
    this.drainQueueRemovals();
    while (true) {
      const next = this.queue.dequeue();
      if (!next) break;
      const promoted = this.gateway.promoteQueuedCommand(next.command, this.clock.now());
      if (promoted.decision === "accept_and_load") {
        void this.prepareAndSwitch(next.command, promoted.ack);
        return;
      }
      if (promoted.decision === "expired") {
        this.emitCommandFailure(next.command, "PLAYBACK_COMMAND_EXPIRED", "queued playback expired before execution");
        continue;
      }
      this.emitCommandFailure(next.command, "PLAYBACK_COMMAND_CANCELLED", promoted.ack.reason ?? "queued playback could not be promoted");
    }

    if (!returnTarget) {
      void this.returnToDefault();
      return;
    }
    const resolveResult = this.returnResolver.resolve({ actionReturnTarget: returnTarget, queueHasItems: false });
    if (resolveResult.targetActionKey) {
      void this.loadAndPlayReturn(resolveResult.targetActionKey);
    } else if (resolveResult.target.type === "none") {
      this.stopTickLoop();
    } else {
      void this.returnToDefault();
    }
  }

  private async loadAndPlayReturn(actionKey: string): Promise<void> {
    if (!this.packageSnapshot) return;

    const generation = this.generationManager.next(this.state.packageRevision);
    const playbackInstanceId = createPlaybackInstanceId();
    this.stopTickLoop();
    this.dispatch({ type: "INTERNAL_ACTION_REQUESTED", playbackInstanceId, generation: generation.generation });

    try {
      let entry = this.loadedActions.get(actionKey) ?? null;
      if (!entry) {
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
        entry = this.createLoadedEntry(loaded);
        this.loadedActions.set(actionKey, entry);
        this.loadedActionCache.set(actionKey, loaded.action);
      }

      const firstFrame = entry.frames[0];
      if (!firstFrame) throw new PlaybackError(PLAYBACK_ERROR_CODES.SURFACE_FAILED, `return action has no frame: ${actionKey}`);
      const presentResult = await this.surface.present(firstFrame, {
        anchor: entry.action.anchor,
        frameIndex: 0,
        actionKey: entry.action.actionKey,
      });
      if (!presentResult.presented) {
        throw new PlaybackError(
          PLAYBACK_ERROR_CODES.SURFACE_FAILED,
          `return action first frame was not presented: ${presentResult.error ?? "unknown surface error"}`,
          { actionKey, frameIndex: 0, resourceUrl: firstFrame.sourceUrl, playbackInstanceId },
        );
      }
      if (!generation.isCurrent()) return;

      this.currentLoadedEntry = entry;
      this.currentTimeline = entry.timeline;
      this.currentDecodedFrames = entry.frames;
      this.dispatch({
        type: "INTERNAL_ACTION_LOADED",
        action: entry.action,
        frames: entry.frames,
        playbackInstanceId,
        generation: generation.generation,
        now: this.clock.now(),
      });
      this.startTickLoop();
      this.emit({
        type: "playback.action_started",
        playbackInstanceId,
        actionKey: entry.action.actionKey,
        frameIndex: 0,
        timestamp: Date.now(),
        packageId: this.state.packageId ?? undefined,
        packageRevision: this.state.packageRevision,
      });
    } catch (error) {
      if (PlaybackError.isAbort(error)) return;
      const pbError = PlaybackError.fromUnknown(error);
      this.dispatch({ type: "ACTION_FAILED", error: pbError });
      this.telemetry.recordError(pbError.toView());
      this.emit({
        type: "playback.action_failed",
        playbackInstanceId,
        actionKey,
        error: pbError.toView(),
        timestamp: Date.now(),
      });
    }
  }

  private async returnToDefault(): Promise<void> {
    if (!this.packageSnapshot || !this.state.defaultActionKey) {
      this.stopTickLoop();
      return;
    }
    await this.loadAndPlayReturn(this.state.defaultActionKey);
  }

  private async tryFallback(
    cause: PlaybackError,
    generation?: GenerationToken,
  ): Promise<boolean> {
    this.telemetry.recordFallback();
    const snapshot = this.packageSnapshot;
    this.emit({
      type: "playback.fallback_started",
      timestamp: Date.now(),
      packageId: snapshot?.packageId,
      packageRevision: snapshot?.packageRevision,
    });

    const previewUrl = snapshot?.previewUrl;
    if (!snapshot || !previewUrl) {
      this.emit({
        type: "playback.fallback_failed",
        error: {
          code: PLAYBACK_ERROR_CODES.FALLBACK_FAILED,
          message: `preview fallback unavailable after: ${cause.message}`,
        },
        timestamp: Date.now(),
      });
      return false;
    }

    try {
      const frame = await this.loadPreviewFrame(previewUrl, generation?.signal);
      if (generation && !generation.isCurrent()) {
        this.closeFrameBitmap(frame);
        return false;
      }

      this.surface.configureCanvas({
        width: snapshot.canvas.width,
        height: snapshot.canvas.height,
        scale: 1,
        interpolationMode: snapshot.interpolationMode ?? "smooth",
      });
      const result = await this.surface.present(frame, {
        anchor: { type: "center", x: 0, y: 0 },
        frameIndex: 0,
        actionKey: "__fallback_preview__",
      });
      if (!result.presented) {
        this.closeFrameBitmap(frame);
        throw new Error(result.error ?? "preview surface present failed");
      }

      this.releaseFallbackFrame();
      this.fallbackFrame = frame;
      this.emit({
        type: "playback.frame_presented",
        actionKey: "__fallback_preview__",
        frameIndex: 0,
        timestamp: Date.now(),
        packageId: snapshot.packageId,
        packageRevision: snapshot.packageRevision,
      });
      return true;
    } catch (error) {
      if (PlaybackError.isAbort(error)) return false;
      const message = error instanceof Error ? error.message : String(error);
      this.emit({
        type: "playback.fallback_failed",
        error: {
          code: PLAYBACK_ERROR_CODES.FALLBACK_FAILED,
          message: `preview fallback failed: ${message}`,
          resourceUrl: previewUrl,
        },
        timestamp: Date.now(),
        packageId: snapshot.packageId,
        packageRevision: snapshot.packageRevision,
      });
      return false;
    }
  }

  private async loadPreviewFrame(
    previewUrl: string,
    signal?: AbortSignal,
  ): Promise<DecodedFrame> {
    if (typeof createImageBitmap === "function") {
      const response = await fetch(previewUrl, signal ? { signal } : undefined);
      if (!response.ok) {
        throw new Error(`preview fetch failed: ${response.status} ${response.statusText}`);
      }
      const blob = await response.blob();
      const bitmap = await createImageBitmap(blob);
      return {
        frameIndex: 0,
        bitmap,
        width: bitmap.width,
        height: bitmap.height,
        estimatedBytes: bitmap.width * bitmap.height * 4,
        sourceUrl: previewUrl,
        decoderName: "fallback-image-bitmap",
        contentHash: "",
      };
    }

    if (typeof Image === "undefined") {
      throw new Error("no image decoder available for preview fallback");
    }

    const image = new Image();
    await new Promise<void>((resolve, reject) => {
      let settled = false;
      const cleanup = () => {
        image.onload = null;
        image.onerror = null;
        signal?.removeEventListener("abort", onAbort);
      };
      const finish = (error?: Error) => {
        if (settled) return;
        settled = true;
        cleanup();
        if (error) reject(error);
        else resolve();
      };
      const onAbort = () => {
        image.src = "";
        const error = new Error("preview fallback aborted");
        error.name = "AbortError";
        finish(error);
      };
      image.onload = () => finish();
      image.onerror = () => finish(new Error(`preview image load failed: ${previewUrl}`));
      if (signal?.aborted) {
        onAbort();
        return;
      }
      signal?.addEventListener("abort", onAbort, { once: true });
      image.src = previewUrl;
    });

    return {
      frameIndex: 0,
      bitmap: image,
      width: image.naturalWidth || image.width,
      height: image.naturalHeight || image.height,
      estimatedBytes: (image.naturalWidth || image.width) * (image.naturalHeight || image.height) * 4,
      sourceUrl: previewUrl,
      decoderName: "fallback-html-image",
      contentHash: "",
    };
  }

  private closeFrameBitmap(frame: DecodedFrame): void {
    if (frame.bitmap && typeof (frame.bitmap as ImageBitmap).close === "function") {
      try {
        (frame.bitmap as ImageBitmap).close();
      } catch {
        void 0;
      }
    }
  }

  private releaseFallbackFrame(): void {
    if (!this.fallbackFrame) return;
    this.closeFrameBitmap(this.fallbackFrame);
    this.fallbackFrame = null;
  }

  private finalizePlayback(
    instanceId: string,
    type: "completed" | "interrupted",
    reason: string,
    now: number,
    context?: {
      commandId?: string;
      actionKey?: string;
      playedDurationMs?: number;
      presentedFrames?: number;
      droppedFramesEstimate?: number;
    },
  ): boolean {
    if (!instanceId || this.finalizedIds.has(instanceId)) return false;

    this.finalizedIds.set(instanceId, { timestamp: now });
    this.firstCycleReportedIds.delete(instanceId);
    const command = this.acceptedCommands.get(instanceId);
    this.acceptedCommands.delete(instanceId);
    this.cleanupExpired(now);

    const commandId = context?.commandId ?? command?.commandId;
    const actionKey = context?.actionKey ?? command?.actionKey;
    const playedDurationMs = context?.playedDurationMs ?? 0;
    if (type === "completed") {
      this.emit({
        type: "playback.action_completed",
        playbackInstanceId: instanceId,
        commandId,
        actionKey,
        reason,
        playedDurationMs,
        presentedFrames: context?.presentedFrames,
        droppedFramesEstimate: context?.droppedFramesEstimate,
        timestamp: Date.now(),
        traceId: command?.traceId,
      });
    } else {
      this.emit({
        type: "playback.action_interrupted",
        playbackInstanceId: instanceId,
        commandId,
        actionKey,
        reason,
        playedDurationMs,
        timestamp: Date.now(),
        traceId: command?.traceId,
      });
    }
    return true;
  }

  private emitCommandFailure(command: PlayActionCommand, code: string, message: string): void {
    this.emitCommandFailureView(command, {
      code,
      message,
      actionKey: command.actionKey,
      playbackInstanceId: command.playbackInstanceId,
      commandId: command.commandId,
      traceId: command.traceId,
    });
  }

  private emitCommandFailureView(command: PlayActionCommand, error: PlaybackErrorView): void {
    const instanceId = command.playbackInstanceId;
    if (!instanceId || this.finalizedIds.has(instanceId)) return;
    const now = this.clock.now();
    this.finalizedIds.set(instanceId, { timestamp: now });
    this.firstCycleReportedIds.delete(instanceId);
    this.acceptedCommands.delete(instanceId);
    this.cleanupExpired(now);
    this.emit({
      type: "playback.action_failed",
      playbackInstanceId: instanceId,
      commandId: command.commandId,
      actionKey: command.actionKey,
      error,
      reason: error.code,
      timestamp: Date.now(),
      traceId: command.traceId,
    });
  }

  private drainQueueRemovals(messagePrefix = "queued playback removed before execution"): void {
    for (const removal of this.queue.drainRemovals()) {
      this.emitCommandFailure(
        removal.item.command,
        "PLAYBACK_COMMAND_CANCELLED",
        `${messagePrefix}: ${removal.reason}`,
      );
    }
  }

  private applyCommandOverrides(action: LoadedAction, command: PlayActionCommand): LoadedAction {
    return {
      ...action,
      defaultPriority: command.priority,
      minimumPlayMs: command.minimumPlayMs ?? action.minimumPlayMs,
      maximumPlayMs: command.maximumPlayMs ?? action.maximumPlayMs,
      interruptAfterMs: command.interruptAfterMs ?? action.interruptAfterMs,
      returnTarget: command.returnOverride ?? action.returnTarget,
    };
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
