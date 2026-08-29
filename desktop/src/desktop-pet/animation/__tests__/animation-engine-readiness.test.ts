import { afterEach, describe, expect, it, vi } from "vitest";
import { DesktopPetAnimationEngine } from "../animation-engine";
import { FakePlaybackClock } from "../playback-clock";
import { PlaybackError, PLAYBACK_ERROR_CODES } from "../errors";
import type {
  ActionAssetRepository,
  DecodedFrame,
  LoadedAction,
  LoadedActionAssets,
  PackagePlaybackSnapshot,
  PetVisualSurface,
  PlaybackClock,
  PlayActionCommand,
  PresentedFrameInfo,
} from "../contracts";

function makeSnapshot(
  overrides?: Partial<PackagePlaybackSnapshot>,
): PackagePlaybackSnapshot {
  return {
    packageId: "test-pkg",
    packageRevision: 1,
    schemaVersion: 2,
    canvas: { width: 256, height: 256 },
    defaultActionKey: "idle",
    actions: [{ actionKey: "idle", configUrl: "amitia-pet://idle/action.json" }],
    ...overrides,
  };
}

function makeAssets(
  actionKey = "idle",
  overrides?: Partial<LoadedAction>,
): LoadedActionAssets {
  const normalizedFrame = {
    index: 0,
    resourceUrl: `amitia-pet://${actionKey}/frame.png`,
    durationMs: 100,
    cumulativeStartMs: 0,
    cumulativeEndMs: 100,
    frameId: "frame-0",
    assetId: "asset-0",
    contentHash: "hash-0",
  };
  const action: LoadedAction = {
    packageId: "test-pkg",
    packageRevision: 1,
    actionKey,
    displayName: actionKey,
    actionVersion: 1,
    loopType: "loop",
    frames: [normalizedFrame],
    baseDurationMs: 100,
    cycleDurationMs: 100,
    anchor: { type: "bottom_center", x: 0, y: 0 },
    interruptible: true,
    interruptAfterMs: 0,
    minimumPlayMs: 0,
    maximumPlayMs: null,
    defaultPriority: 50,
    cooldownMs: 0,
    mutexGroup: null,
    returnTarget: { type: "default" },
    supportsDefaultIdle: true,
    isStableStateCandidate: true,
    isTransitionOnly: false,
    warnings: [],
    ...overrides,
  };
  const decoded: DecodedFrame = {
    frameIndex: 0,
    bitmap: {} as HTMLImageElement,
    width: 128,
    height: 128,
    estimatedBytes: 128 * 128 * 4,
    sourceUrl: normalizedFrame.resourceUrl,
    decoderName: "test",
    contentHash: normalizedFrame.contentHash,
  };
  return {
    action,
    decodedFrames: [decoded],
    totalEstimatedBytes: decoded.estimatedBytes,
  };
}



function makePlayCommand(
  overrides?: Partial<PlayActionCommand>,
): PlayActionCommand {
  return {
    commandId: "cmd-wave",
    playbackInstanceId: "pbi-wave",
    idempotencyKey: "idem-wave",
    installationId: "install-1",
    petInstanceId: "pet-1",
    packageRevision: 1,
    actionKey: "wave",
    priority: 70,
    queuePolicy: "replace_current",
    interruptPolicy: "force_system",
    playbackRate: 1,
    issuedAt: new Date().toISOString(),
    ...overrides,
  };
}

class RacingPlaybackClock implements PlaybackClock {
  private nowMs = 0;
  private nextHandle = 0;
  private callbacks = new Map<number, (now: number) => void>();

  now(): number {
    return this.nowMs;
  }

  requestTick(callback: (now: number) => void): number {
    this.nextHandle += 1;
    this.callbacks.set(this.nextHandle, callback);
    return this.nextHandle;
  }

  // Deliberately leave the callback queued to model a platform callback that
  // has already crossed the cancellation boundary.
  cancelTick(_handle: number): void {}

  fire(handle: number, advanceMs = 16): void {
    const callback = this.callbacks.get(handle);
    if (!callback) return;
    this.callbacks.delete(handle);
    this.nowMs += advanceMs;
    callback(this.nowMs);
  }

  handles(): number[] {
    return [...this.callbacks.keys()].sort((a, b) => a - b);
  }
}

function makeSurface(
  presentResult: PresentedFrameInfo = {
    presented: true,
    frameIndex: 0,
    timestamp: 1,
  },
): PetVisualSurface & { present: ReturnType<typeof vi.fn> } {
  const present = vi.fn(async () => presentResult);
  return {
    configureCanvas: vi.fn(),
    present,
    retainLastFrame: vi.fn(),
    clear: vi.fn(),
    captureHitMask: vi.fn(() => ({
      width: 0,
      height: 0,
      data: new Uint8Array(0),
      threshold: 128,
    })),
    dispose: vi.fn(),
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("DesktopPetAnimationEngine startup readiness", () => {
  it("does not commit playing state when the default first frame cannot be presented", async () => {
    const assets = makeAssets();
    const repository: ActionAssetRepository = {
      loadAction: vi.fn(async () => assets),
    };
    const surface = makeSurface({
      presented: false,
      frameIndex: 0,
      timestamp: 1,
      error: "draw failed",
    });
    const engine = new DesktopPetAnimationEngine({
      surface,
      assetRepository: repository,
      clock: new FakePlaybackClock(),
    });

    await expect(engine.initialize(makeSnapshot())).rejects.toMatchObject({
      code: PLAYBACK_ERROR_CODES.SURFACE_FAILED,
    });
    expect(engine.getPhase()).toBe("failed");
    engine.dispose();
  });

  it("commits playing state only after the first frame was really presented", async () => {
    const assets = makeAssets();
    const repository: ActionAssetRepository = {
      loadAction: vi.fn(async () => assets),
    };
    const surface = makeSurface();
    const engine = new DesktopPetAnimationEngine({
      surface,
      assetRepository: repository,
      clock: new FakePlaybackClock(),
    });

    await engine.initialize(makeSnapshot());

    expect(surface.present).toHaveBeenCalledTimes(1);
    expect(engine.getPhase()).toBe("playing");
    engine.dispose();
  });

  it("actually presents preview fallback but still rejects failed default initialization", async () => {
    const repositoryFailure = new PlaybackError(
      PLAYBACK_ERROR_CODES.FRAME_FETCH_FAILED,
      "default action failed",
    );
    const repository: ActionAssetRepository = {
      loadAction: vi.fn(async () => {
        throw repositoryFailure;
      }),
    };
    const surface = makeSurface();
    const close = vi.fn();
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(new Blob(["preview"], { type: "image/png" }), { status: 200 }),
      ),
    );
    vi.stubGlobal(
      "createImageBitmap",
      vi.fn(async () => ({ width: 64, height: 64, close } as unknown as ImageBitmap)),
    );
    const engine = new DesktopPetAnimationEngine({
      surface,
      assetRepository: repository,
      clock: new FakePlaybackClock(),
    });
    const events: string[] = [];
    engine.onEvent((event) => {
      events.push(`${event.type}:${event.actionKey ?? ""}`);
    });

    await expect(
      engine.initialize(
        makeSnapshot({ previewUrl: "amitia-pet://installation/install-1/preview.png" }),
      ),
    ).rejects.toBe(repositoryFailure);

    expect(surface.present).toHaveBeenCalledTimes(1);
    expect(events).toContain("playback.fallback_started:");
    expect(events).toContain("playback.frame_presented:__fallback_preview__");
    expect(engine.getPhase()).toBe("failed");
    engine.dispose();
    expect(close).toHaveBeenCalledTimes(1);
  });

  it("fences a stale tick callback from an older playback generation", async () => {
    const clock = new RacingPlaybackClock();
    const repository: ActionAssetRepository = {
      loadAction: vi.fn(async ({ actionKey }) => makeAssets(actionKey)),
    };
    const engine = new DesktopPetAnimationEngine({
      surface: makeSurface(),
      assetRepository: repository,
      clock,
    });
    await engine.initialize(makeSnapshot({
      actions: [
        { actionKey: "idle", configUrl: "amitia-pet://idle/action.json" },
        { actionKey: "wave", configUrl: "amitia-pet://wave/action.json" },
      ],
    }));

    expect(clock.handles()).toEqual([1]);
    await engine.playAction(makePlayCommand());
    expect(clock.handles()).toEqual([1, 2]);

    clock.fire(1);
    // A stale callback must not clear handle 2 or create a third concurrent loop.
    expect(clock.handles()).toEqual([2]);

    engine.dispose();
  });

  it("advances a playback whose monotonic start time is exactly zero and emits max-duration terminal once", async () => {
    const clock = new FakePlaybackClock();
    const repository: ActionAssetRepository = {
      loadAction: vi.fn(async ({ actionKey }) =>
        actionKey === "wave"
          ? makeAssets("wave", { maximumPlayMs: 50, returnTarget: { type: "none" } })
          : makeAssets("idle"),
      ),
    };
    const surface = makeSurface();
    const engine = new DesktopPetAnimationEngine({ surface, assetRepository: repository, clock });
    const terminal: Array<{ type: string; playbackInstanceId?: string; reason?: string; playedDurationMs?: number }> = [];
    engine.onEvent((event) => {
      if (event.type === "playback.action_interrupted" || event.type === "playback.action_completed" || event.type === "playback.action_failed") {
        terminal.push(event);
      }
    });

    await engine.initialize(
      makeSnapshot({
        actions: [
          { actionKey: "idle", configUrl: "amitia-pet://idle/action.json" },
          { actionKey: "wave", configUrl: "amitia-pet://wave/action.json" },
        ],
      }),
    );
    await engine.playAction(makePlayCommand());

    clock.advance(60);

    const waveTerminal = terminal.filter((event) => event.playbackInstanceId === "pbi-wave");
    expect(waveTerminal).toHaveLength(1);
    expect(waveTerminal[0]).toMatchObject({
      type: "playback.action_interrupted",
      reason: "max_duration_reached",
      playedDurationMs: 60,
    });
    engine.dispose();
  });


  it("keeps the active playback running when a replacement first frame cannot be presented", async () => {
    const clock = new FakePlaybackClock();
    const repository: ActionAssetRepository = {
      loadAction: vi.fn(async ({ actionKey }) => makeAssets(actionKey)),
    };
    const surface = makeSurface();
    const engine = new DesktopPetAnimationEngine({ surface, assetRepository: repository, clock });
    const terminal: Array<{ type: string; actionKey?: string; playbackInstanceId?: string; reason?: string }> = [];
    engine.onEvent((event) => {
      if (event.type === "playback.action_interrupted" || event.type === "playback.action_failed") {
        terminal.push(event);
      }
    });

    await engine.initialize(
      makeSnapshot({
        actions: [
          { actionKey: "idle", configUrl: "amitia-pet://idle/action.json" },
          { actionKey: "wave", configUrl: "amitia-pet://wave/action.json" },
        ],
      }),
    );

    surface.present.mockResolvedValueOnce({
      presented: false,
      frameIndex: 0,
      timestamp: 2,
      error: "replacement draw failed",
    });
    await engine.playAction(makePlayCommand());

    expect(engine.getSnapshot()).toMatchObject({
      phase: "playing",
      currentActionKey: "idle",
    });
    expect(terminal.filter((event) => event.actionKey === "idle")).toHaveLength(0);
    expect(terminal.filter((event) => event.playbackInstanceId === "pbi-wave")).toEqual([
      expect.objectContaining({ type: "playback.action_failed" }),
    ]);
    engine.dispose();
  });

  it("terminalizes the active runtime playback before an atomic package switch clears identity", async () => {
    const clock = new FakePlaybackClock();
    const repository: ActionAssetRepository = {
      loadAction: vi.fn(async ({ actionKey, packageSnapshot }) =>
        makeAssets(actionKey, {
          packageId: packageSnapshot.packageId,
          packageRevision: packageSnapshot.packageRevision,
          loopType: "loop",
        }),
      ),
    };
    const surface = makeSurface();
    const engine = new DesktopPetAnimationEngine({ surface, assetRepository: repository, clock });
    const events: Array<{ type: string; playbackInstanceId?: string; reason?: string }> = [];
    engine.onEvent((event) => {
      if (event.type === "playback.action_interrupted") events.push(event);
    });

    await engine.initialize(
      makeSnapshot({
        actions: [
          { actionKey: "idle", configUrl: "amitia-pet://idle/action.json" },
          { actionKey: "wave", configUrl: "amitia-pet://wave/action.json" },
        ],
      }),
    );
    await engine.playAction(makePlayCommand());
    await engine.switchPackage(
      makeSnapshot({ packageId: "test-pkg-2", packageRevision: 2 }),
    );

    expect(events.filter((event) => event.playbackInstanceId === "pbi-wave")).toEqual([
      expect.objectContaining({ reason: "package_switch" }),
    ]);
    engine.dispose();
  });

});
