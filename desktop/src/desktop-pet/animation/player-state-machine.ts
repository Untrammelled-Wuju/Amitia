import type {
  PlayerPhase,
  LoadedAction,
  DecodedFrame,
  PlayActionCommand,
  PlaybackSnapshot,
  PlaybackErrorView,
  PackagePlaybackSnapshot,
  TimelinePosition,
  PlaybackRecoverySnapshot,
} from "./contracts";
import { PlaybackError } from "./errors";

export interface PlayerState {
  phase: PlayerPhase;
  packageId: string | null;
  packageRevision: number;
  currentAction: LoadedAction | null;
  currentCommandId: string | null;
  currentPlaybackInstanceId: string | null;
  frameIndex: number | null;
  localElapsedMs: number;
  cycleIndex: number;
  playbackRate: number;
  previousStableActionKey: string | null;
  defaultActionKey: string | null;
  lastTransitionAtMonotonicMs: number;
  lastError?: PlaybackErrorView;
  stateVersion: number;
  startMonotonicMs: number;
  pausedDurationMs: number;
  pauseStartMonotonicMs: number | null;
  lastPresentedFrameIndex: number | null;
  presentedFrames: number;
  droppedFramesEstimate: number;
  startedAtMonotonicMs: number;
}

export type StateAction =
  | { type: "INITIALIZE_STARTED"; snapshot: PackagePlaybackSnapshot; generation: number }
  | { type: "DEFAULT_LOADED"; action: LoadedAction; frames: DecodedFrame[]; generation: number }
  | { type: "DEFAULT_LOAD_FAILED"; error: PlaybackError; generation: number }
  | { type: "PLAY_ACCEPTED"; command: PlayActionCommand; playbackInstanceId: string; generation: number }
  | {
      type: "ACTION_LOADED";
      action: LoadedAction;
      frames: DecodedFrame[];
      command: PlayActionCommand;
      playbackInstanceId: string;
      generation: number;
    }
  | { type: "ACTION_LOAD_FAILED"; error: PlaybackError; command: PlayActionCommand; generation: number }
  | { type: "TICK"; now: number; position: TimelinePosition }
  | { type: "PAUSE"; now: number }
  | { type: "RESUME"; now: number }
  | { type: "HOLD_ENTERED" }
  | { type: "ACTION_COMPLETED"; reason: string; now: number }
  | { type: "ACTION_INTERRUPTED"; reason: string; now: number }
  | { type: "ACTION_FAILED"; error: PlaybackError }
  | { type: "PACKAGE_SWITCH_STARTED"; snapshot: PackagePlaybackSnapshot; generation: number }
  | { type: "PACKAGE_SWITCH_COMMITTED"; action: LoadedAction; frames: DecodedFrame[]; generation: number }
  | { type: "DEFAULT_CHANGED"; newDefaultKey: string; newDefaultAction: LoadedAction | null }
  | { type: "WINDOW_HIDDEN"; now: number }
  | { type: "WINDOW_SHOWN"; now: number }
  | { type: "SUSPENDED"; now: number }
  | { type: "RESUMED_SYSTEM"; now: number }
  | { type: "RECOVER"; snapshot: PlaybackRecoverySnapshot }
  | { type: "DISPOSE" }
  | { type: "FATAL_ERROR"; error: PlaybackError };

export function createInitialState(): PlayerState {
  return {
    phase: "uninitialized",
    packageId: null,
    packageRevision: 0,
    currentAction: null,
    currentCommandId: null,
    currentPlaybackInstanceId: null,
    frameIndex: null,
    localElapsedMs: 0,
    cycleIndex: 0,
    playbackRate: 1,
    previousStableActionKey: null,
    defaultActionKey: null,
    lastTransitionAtMonotonicMs: 0,
    stateVersion: 0,
    startMonotonicMs: 0,
    pausedDurationMs: 0,
    pauseStartMonotonicMs: null,
    lastPresentedFrameIndex: null,
    presentedFrames: 0,
    droppedFramesEstimate: 0,
    startedAtMonotonicMs: 0,
  };
}

export function createPlaybackInstanceId(): string {
  const timestamp = Date.now().toString(36);
  const random = Math.random().toString(36).slice(2, 10);
  return `pbi_${timestamp}_${random}`;
}

export function isStableAction(action: LoadedAction | null): boolean {
  if (!action) {
    return false;
  }
  return action.isStableStateCandidate && !action.isTransitionOnly;
}

export function canBePrevious(action: LoadedAction | null): boolean {
  if (!action) {
    return false;
  }
  return !action.isTransitionOnly;
}

export function playerReducer(state: PlayerState, action: StateAction): PlayerState {
  if (action.type === "DISPOSE") {
    return {
      ...createInitialState(),
      phase: "disposed",
      stateVersion: state.stateVersion + 1,
    };
  }

  if (state.phase === "disposed") {
    return state;
  }

  switch (action.type) {
    case "INITIALIZE_STARTED": {
      if (state.phase !== "uninitialized") {
        return state;
      }
      return {
        ...state,
        phase: "loading_default",
        packageId: action.snapshot.packageId,
        packageRevision: action.snapshot.packageRevision,
        defaultActionKey: action.snapshot.defaultActionKey,
        currentAction: null,
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        frameIndex: null,
        localElapsedMs: 0,
        cycleIndex: 0,
        lastTransitionAtMonotonicMs: 0,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "DEFAULT_LOADED": {
      if (state.phase !== "loading_default") {
        return state;
      }
      const next: PlayerState = {
        ...state,
        phase: "playing",
        currentAction: action.action,
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        frameIndex: 0,
        localElapsedMs: 0,
        cycleIndex: 0,
        playbackRate: 1,
        lastPresentedFrameIndex: null,
        lastError: undefined,
        stateVersion: state.stateVersion + 1,
      };
      if (isStableAction(action.action)) {
        next.previousStableActionKey = action.action.actionKey;
      }
      return next;
    }

    case "DEFAULT_LOAD_FAILED": {
      if (state.phase !== "loading_default") {
        return state;
      }
      return {
        ...state,
        phase: "failed",
        lastError: action.error.toView(),
        stateVersion: state.stateVersion + 1,
      };
    }

    case "PLAY_ACCEPTED": {
      if (state.phase !== "playing" && state.phase !== "ready" && state.phase !== "holding") {
        return state;
      }
      return {
        ...state,
        phase: "loading_action",
        currentCommandId: action.command.commandId,
        currentPlaybackInstanceId: action.playbackInstanceId,
        playbackRate: action.command.playbackRate,
        lastError: undefined,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "ACTION_LOADED": {
      if (state.phase !== "loading_action") {
        return state;
      }
      return {
        ...state,
        phase: "playing",
        currentAction: action.action,
        currentCommandId: action.command.commandId,
        currentPlaybackInstanceId: action.playbackInstanceId,
        playbackRate: action.command.playbackRate,
        frameIndex: 0,
        localElapsedMs: 0,
        cycleIndex: 0,
        lastPresentedFrameIndex: null,
        lastError: undefined,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "ACTION_LOAD_FAILED": {
      if (state.phase !== "loading_action") {
        return state;
      }
      return {
        ...state,
        phase: "playing",
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        lastError: action.error.toView(),
        stateVersion: state.stateVersion + 1,
      };
    }

    case "TICK": {
      if (state.phase !== "playing") {
        return state;
      }
      const position = action.position;
      let presentedFrames = state.presentedFrames;
      let droppedFramesEstimate = state.droppedFramesEstimate;
      let lastPresentedFrameIndex = state.lastPresentedFrameIndex;
      if (lastPresentedFrameIndex !== null && position.frameIndex !== lastPresentedFrameIndex) {
        const expected = lastPresentedFrameIndex + 1;
        if (position.frameIndex > expected) {
          droppedFramesEstimate += position.frameIndex - expected;
        }
        presentedFrames += 1;
      } else if (lastPresentedFrameIndex === null) {
        presentedFrames += 1;
      }
      lastPresentedFrameIndex = position.frameIndex;
      return {
        ...state,
        frameIndex: position.frameIndex,
        cycleIndex: position.cycleIndex,
        localElapsedMs: position.localMs,
        presentedFrames,
        droppedFramesEstimate,
        lastPresentedFrameIndex,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "PAUSE": {
      if (state.phase !== "playing") {
        return state;
      }
      return {
        ...state,
        phase: "paused",
        pauseStartMonotonicMs: action.now,
        lastTransitionAtMonotonicMs: action.now,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "RESUME": {
      if (state.phase !== "paused") {
        return state;
      }
      let pausedDurationMs = state.pausedDurationMs;
      if (state.pauseStartMonotonicMs !== null) {
        pausedDurationMs += Math.max(0, action.now - state.pauseStartMonotonicMs);
      }
      return {
        ...state,
        phase: "playing",
        pauseStartMonotonicMs: null,
        pausedDurationMs,
        lastTransitionAtMonotonicMs: action.now,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "HOLD_ENTERED": {
      if (state.phase !== "playing") {
        return state;
      }
      return {
        ...state,
        phase: "holding",
        stateVersion: state.stateVersion + 1,
      };
    }

    case "ACTION_COMPLETED": {
      const stableKey = state.currentAction && isStableAction(state.currentAction)
        ? state.currentAction.actionKey
        : state.previousStableActionKey;
      return {
        ...state,
        phase: "ready",
        currentAction: null,
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        frameIndex: null,
        localElapsedMs: 0,
        cycleIndex: 0,
        previousStableActionKey: stableKey,
        lastTransitionAtMonotonicMs: action.now,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "ACTION_INTERRUPTED": {
      const stableKey = state.currentAction && isStableAction(state.currentAction)
        ? state.currentAction.actionKey
        : state.previousStableActionKey;
      return {
        ...state,
        phase: "ready",
        currentAction: null,
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        frameIndex: null,
        localElapsedMs: 0,
        cycleIndex: 0,
        previousStableActionKey: stableKey,
        lastTransitionAtMonotonicMs: action.now,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "ACTION_FAILED": {
      const stableKey = state.currentAction && isStableAction(state.currentAction)
        ? state.currentAction.actionKey
        : state.previousStableActionKey;
      const recoverable = action.error.isRecoverable();
      return {
        ...state,
        phase: recoverable ? "ready" : "failed",
        currentAction: null,
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        frameIndex: null,
        localElapsedMs: 0,
        cycleIndex: 0,
        previousStableActionKey: stableKey,
        lastError: action.error.toView(),
        stateVersion: state.stateVersion + 1,
      };
    }

    case "PACKAGE_SWITCH_STARTED": {
      return {
        ...state,
        phase: "recovering",
        packageId: action.snapshot.packageId,
        packageRevision: action.snapshot.packageRevision,
        defaultActionKey: action.snapshot.defaultActionKey,
        currentAction: null,
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        frameIndex: null,
        localElapsedMs: 0,
        cycleIndex: 0,
        pauseStartMonotonicMs: null,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "PACKAGE_SWITCH_COMMITTED": {
      if (state.phase !== "recovering") {
        return state;
      }
      return {
        ...state,
        phase: "playing",
        currentAction: action.action,
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        frameIndex: 0,
        localElapsedMs: 0,
        cycleIndex: 0,
        playbackRate: 1,
        lastPresentedFrameIndex: null,
        lastError: undefined,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "DEFAULT_CHANGED": {
      return {
        ...state,
        defaultActionKey: action.newDefaultKey,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "WINDOW_HIDDEN": {
      if (state.phase === "failed") {
        return state;
      }
      if (state.pauseStartMonotonicMs !== null) {
        return state;
      }
      return {
        ...state,
        pauseStartMonotonicMs: action.now,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "WINDOW_SHOWN": {
      if (state.phase === "failed") {
        return state;
      }
      let pausedDurationMs = state.pausedDurationMs;
      if (state.pauseStartMonotonicMs !== null) {
        pausedDurationMs += Math.max(0, action.now - state.pauseStartMonotonicMs);
      }
      const nextPhase: PlayerPhase =
        state.phase === "paused" || state.phase === "holding" ? "playing" : state.phase;
      return {
        ...state,
        phase: nextPhase,
        pauseStartMonotonicMs: null,
        pausedDurationMs,
        lastTransitionAtMonotonicMs: action.now,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "SUSPENDED": {
      if (state.phase === "failed") {
        return state;
      }
      if (state.pauseStartMonotonicMs !== null) {
        return state;
      }
      return {
        ...state,
        pauseStartMonotonicMs: action.now,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "RESUMED_SYSTEM": {
      if (state.phase === "failed") {
        return state;
      }
      let pausedDurationMs = state.pausedDurationMs;
      if (state.pauseStartMonotonicMs !== null) {
        pausedDurationMs += Math.max(0, action.now - state.pauseStartMonotonicMs);
      }
      const nextPhase: PlayerPhase =
        state.phase === "paused" || state.phase === "holding" ? "playing" : state.phase;
      return {
        ...state,
        phase: nextPhase,
        pauseStartMonotonicMs: null,
        pausedDurationMs,
        lastTransitionAtMonotonicMs: action.now,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "RECOVER": {
      return {
        ...state,
        phase: "loading_default",
        packageId: action.snapshot.packageId,
        packageRevision: action.snapshot.packageRevision,
        defaultActionKey: action.snapshot.defaultActionKey,
        currentAction: null,
        currentCommandId: null,
        currentPlaybackInstanceId: null,
        frameIndex: null,
        localElapsedMs: action.snapshot.lastStableLocalElapsedMs,
        cycleIndex: action.snapshot.lastStableCycleIndex,
        previousStableActionKey: action.snapshot.lastStableActionKey,
        pauseStartMonotonicMs: null,
        lastError: undefined,
        stateVersion: state.stateVersion + 1,
      };
    }

    case "FATAL_ERROR": {
      return {
        ...state,
        phase: "failed",
        lastError: action.error.toView(),
        stateVersion: state.stateVersion + 1,
      };
    }

    default: {
      return state;
    }
  }
}

export function toSnapshot(state: PlayerState): PlaybackSnapshot {
  return {
    phase: state.phase,
    packageId: state.packageId,
    packageRevision: state.packageRevision,
    currentActionKey: state.currentAction?.actionKey ?? null,
    currentCommandId: state.currentCommandId,
    frameIndex: state.frameIndex,
    localElapsedMs: state.localElapsedMs,
    cycleIndex: state.cycleIndex,
    playbackRate: state.playbackRate,
    queueLength: 0,
    previousStableActionKey: state.previousStableActionKey,
    defaultActionKey: state.defaultActionKey,
    lastTransitionAtMonotonicMs: state.lastTransitionAtMonotonicMs,
    lastError: state.lastError,
  };
}
