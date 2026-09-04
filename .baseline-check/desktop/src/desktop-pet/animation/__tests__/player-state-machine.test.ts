import { describe, it, expect } from "vitest";
import {
  createInitialState,
  createPlaybackInstanceId,
  isStableAction,
  canBePrevious,
  playerReducer,
  toSnapshot,
} from "../player-state-machine";
import type {
  LoadedAction,
  PackagePlaybackSnapshot,
  PlayActionCommand,
  PlaybackRecoverySnapshot,
} from "../contracts";
import { PlaybackError, PLAYBACK_ERROR_CODES } from "../errors";

function makeLoadedAction(overrides?: Partial<LoadedAction>): LoadedAction {
  return {
    packageId: "test-pkg",
    packageRevision: 1,
    actionKey: "idle",
    displayName: "Idle",
    actionVersion: 1,
    loopType: "loop",
    frames: [],
    baseDurationMs: 100,
    cycleDurationMs: 400,
    anchor: { type: "bottom_center", x: 128, y: 256 },
    interruptible: true,
    interruptAfterMs: 0,
    minimumPlayMs: 0,
    maximumPlayMs: null,
    defaultPriority: 50,
    cooldownMs: 0,
    mutexGroup: null,
    returnTarget: { type: "none" },
    supportsDefaultIdle: true,
    isStableStateCandidate: true,
    isTransitionOnly: false,
    warnings: [],
    ...overrides,
  };
}

function makePackageSnapshot(overrides?: Partial<PackagePlaybackSnapshot>): PackagePlaybackSnapshot {
  return {
    packageId: "test-pkg",
    packageRevision: 1,
    schemaVersion: 2,
    canvas: { width: 256, height: 256 },
    defaultActionKey: "idle",
    actions: [{ actionKey: "idle", configUrl: "file:///idle/config.json" }],
    ...overrides,
  };
}

function makeCommand(overrides?: Partial<PlayActionCommand>): PlayActionCommand {
  return {
    commandId: `cmd_${Math.random().toString(36).slice(2, 8)}`,
    playbackInstanceId: `pbi_${Math.random().toString(36).slice(2, 8)}`,
    idempotencyKey: `idem_${Math.random().toString(36).slice(2, 8)}`,
    installationId: "inst-1",
    petInstanceId: "pet-1",
    packageRevision: 1,
    actionKey: "wave",
    priority: 50,
    queuePolicy: "enqueue",
    interruptPolicy: "respect_action",
    playbackRate: 1,
    issuedAt: new Date().toISOString(),
    ...overrides,
  };
}

function buildPlayingState() {
  let state = createInitialState();
  state = playerReducer(state, {
    type: "INITIALIZE_STARTED",
    snapshot: makePackageSnapshot(),
    generation: 1,
  });
  state = playerReducer(state, {
    type: "DEFAULT_LOADED",
    action: makeLoadedAction(),
    frames: [],
    now: 1000,
    generation: 1,
  });
  return state;
}

describe("createInitialState", () => {
  it("returns phase uninitialized with null fields", () => {
    const state = createInitialState();
    expect(state.phase).toBe("uninitialized");
    expect(state.packageId).toBeNull();
    expect(state.packageRevision).toBe(0);
    expect(state.currentAction).toBeNull();
    expect(state.currentCommandId).toBeNull();
    expect(state.currentPlaybackInstanceId).toBeNull();
    expect(state.frameIndex).toBeNull();
    expect(state.defaultActionKey).toBeNull();
    expect(state.previousStableActionKey).toBeNull();
    expect(state.pauseStartMonotonicMs).toBeNull();
    expect(state.stateVersion).toBe(0);
    expect(state.localElapsedMs).toBe(0);
    expect(state.cycleIndex).toBe(0);
    expect(state.playbackRate).toBe(1);
  });
});

describe("createPlaybackInstanceId", () => {
  it("returns strings with pbi_ prefix", () => {
    const id = createPlaybackInstanceId();
    expect(id.startsWith("pbi_")).toBe(true);
    expect(id.length).toBeGreaterThan("pbi_".length);
  });

  it("returns unique values across calls", () => {
    const a = createPlaybackInstanceId();
    const b = createPlaybackInstanceId();
    const c = createPlaybackInstanceId();
    expect(a).not.toBe(b);
    expect(b).not.toBe(c);
    expect(a).not.toBe(c);
  });
});

describe("isStableAction", () => {
  it("returns true for a stable action", () => {
    expect(isStableAction(makeLoadedAction())).toBe(true);
  });

  it("returns false for transition-only action", () => {
    expect(isStableAction(makeLoadedAction({ isTransitionOnly: true }))).toBe(false);
  });

  it("returns false when not a stable state candidate", () => {
    expect(isStableAction(makeLoadedAction({ isStableStateCandidate: false }))).toBe(false);
  });

  it("returns false for null", () => {
    expect(isStableAction(null)).toBe(false);
  });
});

describe("canBePrevious", () => {
  it("returns true for a non-transition action", () => {
    expect(canBePrevious(makeLoadedAction())).toBe(true);
  });

  it("returns true even when not a stable candidate but not transition-only", () => {
    expect(canBePrevious(makeLoadedAction({ isStableStateCandidate: false }))).toBe(true);
  });

  it("returns false for transition-only action", () => {
    expect(canBePrevious(makeLoadedAction({ isTransitionOnly: true }))).toBe(false);
  });

  it("returns false for null", () => {
    expect(canBePrevious(null)).toBe(false);
  });
});

describe("playerReducer", () => {
  describe("INITIALIZE_STARTED", () => {
    it("transitions from uninitialized to loading_default", () => {
      const state = createInitialState();
      const next = playerReducer(state, {
        type: "INITIALIZE_STARTED",
        snapshot: makePackageSnapshot(),
        generation: 1,
      });
      expect(next.phase).toBe("recovering");
      expect(next.packageId).toBe("test-pkg");
      expect(next.packageRevision).toBe(1);
      expect(next.defaultActionKey).toBe("idle");
      expect(next.currentAction).toBeNull();
      expect(next.currentCommandId).toBeNull();
      expect(next.currentPlaybackInstanceId).toBeNull();
      expect(next.frameIndex).toBeNull();
      expect(next.stateVersion).toBe(state.stateVersion + 1);
    });

    it("is ignored from non-uninitialized state", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, {
        type: "INITIALIZE_STARTED",
        snapshot: makePackageSnapshot({ packageId: "other" }),
        generation: 2,
      });
      expect(next).toBe(playing);
      expect(next.stateVersion).toBe(playing.stateVersion);
      expect(next.phase).toBe("playing");
    });
  });

  describe("DEFAULT_LOADED", () => {
    it("transitions to playing with action set", () => {
      let state = createInitialState();
      state = playerReducer(state, {
        type: "INITIALIZE_STARTED",
        snapshot: makePackageSnapshot(),
        generation: 1,
      });
      const action = makeLoadedAction();
      const next = playerReducer(state, {
        type: "DEFAULT_LOADED",
        action,
        frames: [],
        now: 1000,
        generation: 1,
      });
      expect(next.phase).toBe("playing");
      expect(next.currentAction).toBe(action);
      expect(next.frameIndex).toBe(0);
      expect(next.localElapsedMs).toBe(0);
      expect(next.cycleIndex).toBe(0);
      expect(next.currentCommandId).toBeNull();
      expect(next.currentPlaybackInstanceId).toBeNull();
      expect(next.previousStableActionKey).toBe("idle");
      expect(next.lastError).toBeUndefined();
      expect(next.stateVersion).toBe(state.stateVersion + 1);
    });

    it("is ignored from non-loading_default state", () => {
      const uninitialized = createInitialState();
      expect(
        playerReducer(uninitialized, {
          type: "DEFAULT_LOADED",
          action: makeLoadedAction(),
          frames: [],
          now: 1000,
          generation: 1,
        }),
      ).toBe(uninitialized);

      const playing = buildPlayingState();
      expect(
        playerReducer(playing, {
          type: "DEFAULT_LOADED",
          action: makeLoadedAction({ actionKey: "other" }),
          frames: [],
          now: 1000,
          generation: 1,
        }),
      ).toBe(playing);
    });
  });

  describe("DEFAULT_LOAD_FAILED", () => {
    it("transitions to failed", () => {
      let state = createInitialState();
      state = playerReducer(state, {
        type: "INITIALIZE_STARTED",
        snapshot: makePackageSnapshot(),
        generation: 1,
      });
      const error = new PlaybackError(
        PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID,
        "bad config",
      );
      const next = playerReducer(state, {
        type: "DEFAULT_LOAD_FAILED",
        error,
        generation: 1,
      });
      expect(next.phase).toBe("failed");
      expect(next.lastError?.code).toBe(PLAYBACK_ERROR_CODES.ACTION_CONFIG_INVALID);
      expect(next.lastError?.message).toBe("bad config");
      expect(next.stateVersion).toBe(state.stateVersion + 1);
    });
  });

  describe("PLAY_ACCEPTED", () => {
    it("transitions to loading_action from playing, ready, and holding", () => {
      const playing = buildPlayingState();
      const command = makeCommand();
      const pbi = "pbi_play_1";

      const fromPlaying = playerReducer(playing, {
        type: "PLAY_ACCEPTED",
        command,
        playbackInstanceId: pbi,
        generation: 1,
      });
      expect(fromPlaying.phase).toBe("loading_action");
      expect(fromPlaying.currentCommandId).toBe(command.commandId);
      expect(fromPlaying.currentPlaybackInstanceId).toBe(pbi);
      expect(fromPlaying.playbackRate).toBe(command.playbackRate);
      expect(fromPlaying.lastError).toBeUndefined();

      const ready = playerReducer(playing, {
        type: "ACTION_COMPLETED",
        reason: "natural_end",
        now: 100,
      });
      expect(ready.phase).toBe("ready");
      const fromReady = playerReducer(ready, {
        type: "PLAY_ACCEPTED",
        command,
        playbackInstanceId: pbi,
        generation: 1,
      });
      expect(fromReady.phase).toBe("loading_action");

      const holding = playerReducer(playing, { type: "HOLD_ENTERED" });
      expect(holding.phase).toBe("holding");
      const fromHolding = playerReducer(holding, {
        type: "PLAY_ACCEPTED",
        command,
        playbackInstanceId: pbi,
        generation: 1,
      });
      expect(fromHolding.phase).toBe("loading_action");
    });

    it("is ignored from other phases", () => {
      const uninitialized = createInitialState();
      expect(
        playerReducer(uninitialized, {
          type: "PLAY_ACCEPTED",
          command: makeCommand(),
          playbackInstanceId: "pbi",
          generation: 1,
        }),
      ).toBe(uninitialized);

      const loadingDefault = playerReducer(createInitialState(), {
        type: "INITIALIZE_STARTED",
        snapshot: makePackageSnapshot(),
        generation: 1,
      });
      expect(
        playerReducer(loadingDefault, {
          type: "PLAY_ACCEPTED",
          command: makeCommand(),
          playbackInstanceId: "pbi",
          generation: 1,
        }),
      ).toBe(loadingDefault);

      const playing = buildPlayingState();
      const loadingAction = playerReducer(playing, {
        type: "PLAY_ACCEPTED",
        command: makeCommand(),
        playbackInstanceId: "pbi",
        generation: 1,
      });
      expect(
        playerReducer(loadingAction, {
          type: "PLAY_ACCEPTED",
          command: makeCommand(),
          playbackInstanceId: "pbi2",
          generation: 1,
        }),
      ).toBe(loadingAction);

      const paused = playerReducer(playing, { type: "PAUSE", now: 100 });
      expect(
        playerReducer(paused, {
          type: "PLAY_ACCEPTED",
          command: makeCommand(),
          playbackInstanceId: "pbi",
          generation: 1,
        }),
      ).toBe(paused);
    });
  });

  describe("ACTION_LOADED", () => {
    it("transitions to playing with new action", () => {
      const playing = buildPlayingState();
      const command = makeCommand({ actionKey: "wave", playbackRate: 1.5 });
      const pbi = "pbi_loaded_1";
      const state = playerReducer(playing, {
        type: "PLAY_ACCEPTED",
        command,
        playbackInstanceId: pbi,
        generation: 1,
      });
      const action = makeLoadedAction({ actionKey: "wave" });
      const next = playerReducer(state, {
        type: "ACTION_LOADED",
        action,
        frames: [],
        command,
        playbackInstanceId: pbi,
        now: 1000,
        generation: 1,
      });
      expect(next.phase).toBe("playing");
      expect(next.currentAction).toBe(action);
      expect(next.currentCommandId).toBe(command.commandId);
      expect(next.currentPlaybackInstanceId).toBe(pbi);
      expect(next.frameIndex).toBe(0);
      expect(next.localElapsedMs).toBe(0);
      expect(next.cycleIndex).toBe(0);
      expect(next.playbackRate).toBe(1.5);
      expect(next.lastError).toBeUndefined();
    });
  });

  describe("ACTION_LOAD_FAILED", () => {
    it("returns to playing with error", () => {
      const playing = buildPlayingState();
      const command = makeCommand();
      const pbi = "pbi_fail_1";
      const state = playerReducer(playing, {
        type: "PLAY_ACCEPTED",
        command,
        playbackInstanceId: pbi,
        generation: 1,
      });
      const error = new PlaybackError(
        PLAYBACK_ERROR_CODES.FRAME_DECODE_FAILED,
        "decode failed",
      );
      const next = playerReducer(state, {
        type: "ACTION_LOAD_FAILED",
        error,
        command,
        generation: 1,
      });
      expect(next.phase).toBe("playing");
      expect(next.currentCommandId).toBeNull();
      expect(next.currentPlaybackInstanceId).toBeNull();
      expect(next.lastError?.code).toBe(PLAYBACK_ERROR_CODES.FRAME_DECODE_FAILED);
      expect(next.lastError?.message).toBe("decode failed");
    });
  });

  describe("TICK", () => {
    it("updates frameIndex, cycleIndex, localElapsedMs", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, {
        type: "TICK",
        now: 1000,
        position: { frameIndex: 2, cycleIndex: 1, localMs: 500, completed: false },
      });
      expect(next.frameIndex).toBe(2);
      expect(next.cycleIndex).toBe(1);
      expect(next.localElapsedMs).toBe(500);
      expect(next.presentedFrames).toBe(0);
      expect(next.lastPresentedFrameIndex).toBeNull();
      expect(next.stateVersion).toBe(playing.stateVersion + 1);
    });

    it("advances presentation counters only after FRAME_PRESENTED", () => {
      const playing = buildPlayingState();
      const ticked = playerReducer(playing, {
        type: "TICK",
        now: 1000,
        position: { frameIndex: 2, cycleIndex: 1, localMs: 500, completed: false },
      });
      const presented = playerReducer(ticked, { type: "FRAME_PRESENTED", frameIndex: 2 });
      expect(presented.presentedFrames).toBe(1);
      expect(presented.lastPresentedFrameIndex).toBe(2);
    });

    it("is ignored from non-playing state", () => {
      const ready = playerReducer(buildPlayingState(), {
        type: "ACTION_COMPLETED",
        reason: "natural_end",
        now: 100,
      });
      expect(ready.phase).toBe("ready");
      const next = playerReducer(ready, {
        type: "TICK",
        now: 200,
        position: { frameIndex: 3, cycleIndex: 0, localMs: 10, completed: false },
      });
      expect(next).toBe(ready);
    });
  });

  describe("PAUSE / RESUME", () => {
    it("PAUSE transitions to paused", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, { type: "PAUSE", now: 1000 });
      expect(next.phase).toBe("paused");
      expect(next.pauseStartMonotonicMs).toBe(1000);
      expect(next.lastTransitionAtMonotonicMs).toBe(1000);
    });

    it("RESUME transitions back to playing and accumulates pausedDurationMs", () => {
      const playing = buildPlayingState();
      let state = playerReducer(playing, { type: "PAUSE", now: 1000 });
      state = playerReducer(state, { type: "RESUME", now: 1500 });
      expect(state.phase).toBe("playing");
      expect(state.pauseStartMonotonicMs).toBeNull();
      expect(state.pausedDurationMs).toBe(500);
      expect(state.lastTransitionAtMonotonicMs).toBe(1500);
    });
  });

  describe("HOLD_ENTERED", () => {
    it("transitions to holding", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, { type: "HOLD_ENTERED" });
      expect(next.phase).toBe("holding");
      expect(next.stateVersion).toBe(playing.stateVersion + 1);
    });
  });

  describe("ACTION_COMPLETED", () => {
    it("transitions to ready with cleared action", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, {
        type: "ACTION_COMPLETED",
        reason: "natural_end",
        now: 2000,
      });
      expect(next.phase).toBe("ready");
      expect(next.currentAction).toBeNull();
      expect(next.currentCommandId).toBeNull();
      expect(next.currentPlaybackInstanceId).toBeNull();
      expect(next.frameIndex).toBeNull();
      expect(next.localElapsedMs).toBe(0);
      expect(next.cycleIndex).toBe(0);
      expect(next.previousStableActionKey).toBe("idle");
      expect(next.lastTransitionAtMonotonicMs).toBe(2000);
    });
  });

  describe("ACTION_INTERRUPTED", () => {
    it("transitions to ready", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, {
        type: "ACTION_INTERRUPTED",
        reason: "replaced",
        now: 2000,
      });
      expect(next.phase).toBe("ready");
      expect(next.currentAction).toBeNull();
      expect(next.currentCommandId).toBeNull();
      expect(next.frameIndex).toBeNull();
      expect(next.previousStableActionKey).toBe("idle");
    });
  });

  describe("PACKAGE_SWITCH_STARTED / COMMITTED", () => {
    it("PACKAGE_SWITCH_STARTED transitions to recovering", () => {
      const playing = buildPlayingState();
      const snapshot = makePackageSnapshot({
        packageId: "new-pkg",
        packageRevision: 2,
        defaultActionKey: "sit",
      });
      const next = playerReducer(playing, {
        type: "PACKAGE_SWITCH_STARTED",
        snapshot,
        generation: 2,
      });
      expect(next.phase).toBe("recovering");
      expect(next.packageId).toBe("new-pkg");
      expect(next.packageRevision).toBe(2);
      expect(next.defaultActionKey).toBe("sit");
      expect(next.currentAction).toBeNull();
      expect(next.currentCommandId).toBeNull();
      expect(next.currentPlaybackInstanceId).toBeNull();
      expect(next.pauseStartMonotonicMs).toBeNull();
    });

    it("PACKAGE_SWITCH_COMMITTED transitions to playing from recovering", () => {
      const playing = buildPlayingState();
      const state = playerReducer(playing, {
        type: "PACKAGE_SWITCH_STARTED",
        snapshot: makePackageSnapshot({ packageId: "new-pkg", packageRevision: 2 }),
        generation: 2,
      });
      const action = makeLoadedAction({ actionKey: "idle", packageId: "new-pkg", packageRevision: 2 });
      const next = playerReducer(state, {
        type: "PACKAGE_SWITCH_COMMITTED",
        action,
        frames: [],
        now: 1000,
        generation: 2,
      });
      expect(next.phase).toBe("playing");
      expect(next.currentAction).toBe(action);
      expect(next.frameIndex).toBe(0);
      expect(next.playbackRate).toBe(1);
      expect(next.lastError).toBeUndefined();
    });

    it("PACKAGE_SWITCH_COMMITTED is ignored from non-recovering state", () => {
      const playing = buildPlayingState();
      const action = makeLoadedAction();
      const next = playerReducer(playing, {
        type: "PACKAGE_SWITCH_COMMITTED",
        action,
        frames: [],
        now: 1000,
        generation: 2,
      });
      expect(next).toBe(playing);
    });
  });

  describe("DEFAULT_CHANGED", () => {
    it("updates defaultActionKey", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, {
        type: "DEFAULT_CHANGED",
        newDefaultKey: "sit",
        newDefaultAction: null,
      });
      expect(next.defaultActionKey).toBe("sit");
      expect(next.stateVersion).toBe(playing.stateVersion + 1);
    });
  });

  describe("WINDOW_HIDDEN / WINDOW_SHOWN", () => {
    it("WINDOW_HIDDEN sets pauseStartMonotonicMs", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, { type: "WINDOW_HIDDEN", now: 3000 });
      expect(next.pauseStartMonotonicMs).toBe(3000);
      expect(next.windowHidden).toBe(true);
      expect(next.phase).toBe("playing");
      expect(next.stateVersion).toBe(playing.stateVersion + 1);
    });

    it("WINDOW_HIDDEN is idempotent when already hidden", () => {
      const playing = buildPlayingState();
      const hidden = playerReducer(playing, { type: "WINDOW_HIDDEN", now: 3000 });
      const again = playerReducer(hidden, { type: "WINDOW_HIDDEN", now: 4000 });
      expect(again).toBe(hidden);
      expect(again.pauseStartMonotonicMs).toBe(3000);
    });

    it("WINDOW_SHOWN clears pauseStartMonotonicMs and accumulates paused duration", () => {
      const playing = buildPlayingState();
      let state = playerReducer(playing, { type: "WINDOW_HIDDEN", now: 3000 });
      state = playerReducer(state, { type: "WINDOW_SHOWN", now: 5000 });
      expect(state.pauseStartMonotonicMs).toBeNull();
      expect(state.pausedDurationMs).toBe(2000);
      expect(state.windowHidden).toBe(false);
      expect(state.phase).toBe("playing");
      expect(state.lastTransitionAtMonotonicMs).toBe(5000);
    });

    it("does not auto-resume a manually paused action after hide/show", () => {
      let state = playerReducer(buildPlayingState(), { type: "PAUSE", now: 2000 });
      state = playerReducer(state, { type: "WINDOW_HIDDEN", now: 3000 });
      state = playerReducer(state, { type: "WINDOW_SHOWN", now: 5000 });
      expect(state.phase).toBe("paused");
      expect(state.windowHidden).toBe(false);
      expect(state.pauseStartMonotonicMs).toBe(2000);
    });
  });

  describe("SUSPENDED / RESUMED_SYSTEM", () => {
    it("preserves manual pause across system suspend/resume", () => {
      let state = playerReducer(buildPlayingState(), { type: "PAUSE", now: 2000 });
      state = playerReducer(state, { type: "SUSPENDED", now: 2500 });
      expect(state.systemSuspended).toBe(true);
      state = playerReducer(state, { type: "RESUMED_SYSTEM", now: 4500 });
      expect(state.systemSuspended).toBe(false);
      expect(state.phase).toBe("paused");
      expect(state.pauseStartMonotonicMs).toBe(2000);
    });
  });

  describe("DISPOSE", () => {
    it("transitions to disposed", () => {
      const playing = buildPlayingState();
      const next = playerReducer(playing, { type: "DISPOSE" });
      expect(next.phase).toBe("disposed");
      expect(next.stateVersion).toBe(playing.stateVersion + 1);
      expect(next.currentAction).toBeNull();
      expect(next.packageId).toBeNull();
      expect(next.frameIndex).toBeNull();
    });
  });

  describe("disposed state", () => {
    it("ignores all non-dispose actions", () => {
      const disposed = playerReducer(buildPlayingState(), { type: "DISPOSE" });
      const versionBefore = disposed.stateVersion;

      const ticked = playerReducer(disposed, {
        type: "TICK",
        now: 1,
        position: { frameIndex: 0, cycleIndex: 0, localMs: 0, completed: false },
      });
      expect(ticked).toBe(disposed);

      const played = playerReducer(disposed, {
        type: "PLAY_ACCEPTED",
        command: makeCommand(),
        playbackInstanceId: "pbi",
        generation: 1,
      });
      expect(played).toBe(disposed);

      const initialized = playerReducer(disposed, {
        type: "INITIALIZE_STARTED",
        snapshot: makePackageSnapshot(),
        generation: 1,
      });
      expect(initialized).toBe(disposed);

      const tickedAgain = playerReducer(disposed, { type: "PAUSE", now: 5 });
      expect(tickedAgain).toBe(disposed);

      expect(disposed.phase).toBe("disposed");
      expect(disposed.stateVersion).toBe(versionBefore);
    });
  });

  describe("RECOVER", () => {
    it("transitions to recovering with recovery snapshot", () => {
      const playing = buildPlayingState();
      const recovery: PlaybackRecoverySnapshot = {
        packageId: "test-pkg",
        packageRevision: 1,
        defaultActionKey: "idle",
        lastStableActionKey: "idle",
        lastStableLocalElapsedMs: 120,
        lastStableCycleIndex: 2,
      };
      const next = playerReducer(playing, { type: "RECOVER", snapshot: recovery });
      expect(next.phase).toBe("loading_default");
      expect(next.packageId).toBe("test-pkg");
      expect(next.packageRevision).toBe(1);
      expect(next.defaultActionKey).toBe("idle");
      expect(next.previousStableActionKey).toBe("idle");
      expect(next.localElapsedMs).toBe(120);
      expect(next.cycleIndex).toBe(2);
      expect(next.currentAction).toBeNull();
      expect(next.lastError).toBeUndefined();
    });
  });

  describe("toSnapshot", () => {
    it("correctly maps playing state fields", () => {
      const playing = buildPlayingState();
      const snap = toSnapshot(playing);
      expect(snap.phase).toBe("playing");
      expect(snap.packageId).toBe("test-pkg");
      expect(snap.packageRevision).toBe(1);
      expect(snap.currentActionKey).toBe("idle");
      expect(snap.currentCommandId).toBeNull();
      expect(snap.frameIndex).toBe(0);
      expect(snap.localElapsedMs).toBe(0);
      expect(snap.cycleIndex).toBe(0);
      expect(snap.playbackRate).toBe(1);
      expect(snap.queueLength).toBe(0);
      expect(snap.previousStableActionKey).toBe("idle");
      expect(snap.defaultActionKey).toBe("idle");
      expect(snap.lastError).toBeUndefined();
    });

    it("maps currentActionKey to null when no action is set", () => {
      const ready = playerReducer(buildPlayingState(), {
        type: "ACTION_COMPLETED",
        reason: "natural_end",
        now: 100,
      });
      const snap = toSnapshot(ready);
      expect(snap.phase).toBe("ready");
      expect(snap.currentActionKey).toBeNull();
      expect(snap.previousStableActionKey).toBe("idle");
    });

    it("maps failed state lastError", () => {
      let state = createInitialState();
      state = playerReducer(state, {
        type: "INITIALIZE_STARTED",
        snapshot: makePackageSnapshot(),
        generation: 1,
      });
      state = playerReducer(state, {
        type: "DEFAULT_LOAD_FAILED",
        error: new PlaybackError(PLAYBACK_ERROR_CODES.ACTION_NOT_FOUND, "missing"),
        generation: 1,
      });
      const snap = toSnapshot(state);
      expect(snap.phase).toBe("failed");
      expect(snap.lastError?.code).toBe(PLAYBACK_ERROR_CODES.ACTION_NOT_FOUND);
    });
  });
});
