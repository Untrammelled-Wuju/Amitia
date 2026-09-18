import { describe, expect, it } from "vitest";
import { createActionNormalizer, normalizeActionConfig } from "../loaders/action-config-normalizer";
import type { PackagePlaybackSnapshot, RawActionConfig } from "../contracts";
import { PlaybackError } from "../errors";

function makeFrame(index: number, durationMs = 100) {
  return {
    index,
    file: `frame_${index}.png`,
    durationMs,
    frameId: `idle-frame-${index}`,
    assetId: `asset-${index}`,
    contentHash: `sha256:frame-${index}`,
  };
}

function makePackageSnapshot(overrides?: Partial<PackagePlaybackSnapshot>): PackagePlaybackSnapshot {
  return {
    packageId: "test-pkg",
    packageRevision: 1,
    schemaVersion: 1,
    canvas: { width: 256, height: 256 },
    defaultActionKey: "idle",
    actions: [
      { actionKey: "idle", configUrl: "file:///idle/config.json" },
      { actionKey: "wave", configUrl: "file:///wave/config.json" },
    ],
    ...overrides,
  };
}

function makeRawConfig(overrides?: Partial<RawActionConfig>): RawActionConfig {
  return {
    actionKey: "idle",
    displayName: "Idle",
    version: 1,
    loopType: "loop",
    playbackMode: "loop",
    fps: 10,
    frameDurationMs: 100,
    frameCount: 4,
    frames: [0, 1, 2, 3].map((index) => makeFrame(index)),
    anchor: { type: "bottom_center", x: 0.5, y: 1, coordinateSpace: "normalized_canvas" },
    returnTo: { type: "none" },
    ...overrides,
  };
}

describe("normalizeActionConfig", () => {
  it("normalizes canonical frames and cumulative timing", () => {
    const result = normalizeActionConfig({
      raw: makeRawConfig(),
      packageSnapshot: makePackageSnapshot(),
    });

    expect(result.frames).toHaveLength(4);
    expect(result.frames[0]).toMatchObject({
      index: 0,
      resourceUrl: "frame_0.png",
      durationMs: 100,
      cumulativeStartMs: 0,
      cumulativeEndMs: 100,
      frameId: "idle-frame-0",
      assetId: "asset-0",
      contentHash: "sha256:frame-0",
    });
    expect(result.frames[3]).toMatchObject({
      index: 3,
      cumulativeStartMs: 300,
      cumulativeEndMs: 400,
    });
    expect(result.cycleDurationMs).toBe(400);
    expect(result.baseDurationMs).toBe(100);
  });

  it("sorts explicitly indexed frames without changing identity", () => {
    const result = normalizeActionConfig({
      raw: makeRawConfig({
        frames: [makeFrame(3), makeFrame(1), makeFrame(0), makeFrame(2)],
      }),
      packageSnapshot: makePackageSnapshot(),
    });

    expect(result.frames.map((frame) => frame.index)).toEqual([0, 1, 2, 3]);
    expect(result.frames.map((frame) => frame.resourceUrl)).toEqual([
      "frame_0.png",
      "frame_1.png",
      "frame_2.png",
      "frame_3.png",
    ]);
  });

  it("accepts every canonical playback mode", () => {
    for (const playbackMode of ["loop", "once", "hold", "ping_pong"] as const) {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ playbackMode, returnTo: { type: "none" } }),
        packageSnapshot: makePackageSnapshot(),
      });
      expect(result.loopType).toBe(playbackMode);
    }
  });

  it("rejects unknown playback modes", () => {
    expect(() => normalizeActionConfig({
      raw: makeRawConfig({ playbackMode: "unknown" }),
      packageSnapshot: makePackageSnapshot(),
    })).toThrow(PlaybackError);
  });

  it("resolves canonical return targets", () => {
    expect(normalizeActionConfig({
      raw: makeRawConfig({ returnTo: { type: "default" } }),
      packageSnapshot: makePackageSnapshot(),
    }).returnTarget).toEqual({ type: "default" });
    expect(normalizeActionConfig({
      raw: makeRawConfig({ returnTo: { type: "previous" } }),
      packageSnapshot: makePackageSnapshot(),
    }).returnTarget).toEqual({ type: "previous" });
    expect(normalizeActionConfig({
      raw: makeRawConfig({ returnTo: { type: "current_activity" } }),
      packageSnapshot: makePackageSnapshot(),
    }).returnTarget).toEqual({ type: "current_activity" });
    expect(normalizeActionConfig({
      raw: makeRawConfig({ returnTo: { type: "action", actionKey: "wave" } }),
      packageSnapshot: makePackageSnapshot(),
    }).returnTarget).toEqual({ type: "action", actionKey: "wave" });
  });

  it("rejects invalid return targets", () => {
    for (const returnTo of [
      undefined,
      { type: "action" },
      { type: "action", actionKey: "idle" },
      { type: "action", actionKey: "missing" },
      { type: "unknown" },
    ]) {
      expect(() => normalizeActionConfig({
        raw: makeRawConfig({ returnTo }),
        packageSnapshot: makePackageSnapshot(),
      })).toThrow(PlaybackError);
    }
  });

  it("requires a normalized canonical anchor", () => {
    const result = normalizeActionConfig({
      raw: makeRawConfig({
        anchor: { type: "bottom_center", x: 0.25, y: 0.75, coordinateSpace: "normalized_canvas" },
      }),
      packageSnapshot: makePackageSnapshot(),
    });
    expect(result.anchor).toEqual({ type: "bottom_center", x: 0.25, y: 0.75 });

    for (const anchor of [
      undefined,
      { x: 0.5, y: 1 },
      { x: -0.1, y: 1, coordinateSpace: "normalized_canvas" },
      { x: 0.5, y: 1.1, coordinateSpace: "normalized_canvas" },
      { x: 0.5, y: 1, coordinateSpace: "canvas" },
    ]) {
      expect(() => normalizeActionConfig({
        raw: makeRawConfig({ anchor }),
        packageSnapshot: makePackageSnapshot(),
      })).toThrow(PlaybackError);
    }
  });

  it("rejects empty frames and frame count mismatches", () => {
    expect(() => normalizeActionConfig({
      raw: makeRawConfig({ frames: [], frameCount: 0 }),
      packageSnapshot: makePackageSnapshot(),
    })).toThrow(PlaybackError);
    expect(() => normalizeActionConfig({
      raw: makeRawConfig({ frameCount: 5 }),
      packageSnapshot: makePackageSnapshot(),
    })).toThrow(PlaybackError);
  });

  it("rejects duplicate indices and incomplete frame identity", () => {
    expect(() => normalizeActionConfig({
      raw: makeRawConfig({
        frames: [makeFrame(0), makeFrame(0), makeFrame(2), makeFrame(3)],
      }),
      packageSnapshot: makePackageSnapshot(),
    })).toThrow(PlaybackError);
    expect(() => normalizeActionConfig({
      raw: makeRawConfig({
        frames: [
          { ...makeFrame(0), frameId: "" },
          makeFrame(1),
          makeFrame(2),
          makeFrame(3),
        ],
      }),
      packageSnapshot: makePackageSnapshot(),
    })).toThrow(PlaybackError);
  });

  it("rejects invalid frame durations", () => {
    expect(() => normalizeActionConfig({
      raw: makeRawConfig({
        frames: [
          { ...makeFrame(0), durationMs: 0 },
          makeFrame(1),
          makeFrame(2),
          makeFrame(3),
        ],
      }),
      packageSnapshot: makePackageSnapshot(),
    })).toThrow(PlaybackError);
  });

  it("rejects unsupported package schema versions", () => {
    expect(() => normalizeActionConfig({
      raw: makeRawConfig(),
      packageSnapshot: makePackageSnapshot({ schemaVersion: 2 }),
    })).toThrow(PlaybackError);
  });

  it("computes variable frame durations", () => {
    const result = normalizeActionConfig({
      raw: makeRawConfig({
        frames: [makeFrame(0, 50), makeFrame(1, 100), makeFrame(2, 150), makeFrame(3, 200)],
      }),
      packageSnapshot: makePackageSnapshot(),
    });

    expect(result.cycleDurationMs).toBe(500);
    expect(result.baseDurationMs).toBe(125);
  });

  it("applies canonical defaults", () => {
    const result = normalizeActionConfig({
      raw: makeRawConfig(),
      packageSnapshot: makePackageSnapshot(),
    });

    expect(result.packageId).toBe("test-pkg");
    expect(result.packageRevision).toBe(1);
    expect(result.actionKey).toBe("idle");
    expect(result.displayName).toBe("Idle");
    expect(result.actionVersion).toBe(1);
    expect(result.interruptible).toBe(true);
    expect(result.interruptAfterMs).toBe(0);
    expect(result.minimumPlayMs).toBe(0);
    expect(result.maximumPlayMs).toBeNull();
    expect(result.defaultPriority).toBe(50);
    expect(result.cooldownMs).toBe(0);
    expect(result.mutexGroup).toBeNull();
    expect(result.returnTarget).toEqual({ type: "none" });
    expect(result.supportsDefaultIdle).toBe(true);
    expect(result.isStableStateCandidate).toBe(true);
    expect(result.isTransitionOnly).toBe(false);
    expect(result.warnings).toEqual([]);
  });
});

describe("createActionNormalizer", () => {
  it("returns an independent canonical normalizer function", () => {
    const normalizer = createActionNormalizer();
    const first = normalizer({
      raw: makeRawConfig({ actionKey: "idle", playbackMode: "loop" }),
      packageSnapshot: makePackageSnapshot(),
    });
    const second = normalizer({
      raw: makeRawConfig({
        actionKey: "wave",
        displayName: "Wave",
        playbackMode: "once",
        returnTo: { type: "default" },
      }),
      packageSnapshot: makePackageSnapshot(),
    });

    expect(first.actionKey).toBe("idle");
    expect(first.loopType).toBe("loop");
    expect(second.actionKey).toBe("wave");
    expect(second.displayName).toBe("Wave");
    expect(second.loopType).toBe("once");
  });
});
