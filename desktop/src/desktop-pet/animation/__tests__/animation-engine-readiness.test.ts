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

function makeAssets(): LoadedActionAssets {
  const normalizedFrame = {
    index: 0,
    resourceUrl: "amitia-pet://idle/frame.png",
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
    actionKey: "idle",
    displayName: "Idle",
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
});
