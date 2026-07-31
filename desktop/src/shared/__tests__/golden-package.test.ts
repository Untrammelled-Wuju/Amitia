import { describe, it, expect } from "vitest";
import {
  Schema2PackageReader,
  RuntimePackageNormalizer,
  compareVersions,
  INTEGRITY_ALGORITHM_V2,
  MANIFEST_FORMAT_CANONICAL,
} from "../package-schema";

const RUNTIME_VERSION = "2.0.0";

const goldenManifest = {
  schemaVersion: 2,
  manifestFormat: MANIFEST_FORMAT_CANONICAL,
  petId: "golden-pet",
  releaseId: "golden-release-1",
  version: "1.0.0",
  name: "Golden Pet",
  defaultAction: "idle",
  canvas: { width: 256, height: 256, coordinateSystem: "top-left" },
  actions: [
    {
      key: "idle",
      name: "Idle",
      config: "actions/idle/action.json",
      playbackMode: "loop",
      fps: 8,
      frameCount: 4,
      supportsDefaultIdle: true,
      isStableStateCandidate: true,
      isTransitionOnly: false,
    },
    {
      key: "wave",
      name: "Wave",
      config: "actions/wave/action.json",
      playbackMode: "once",
      fps: 10,
      frameCount: 3,
      supportsDefaultIdle: true,
      isStableStateCandidate: false,
      isTransitionOnly: true,
    },
  ],
  compatibility: { minRuntimeVersion: "1.0.0", renderMode: "sprite" },
  binding: { policy: "bound" },
  integrity: {
    algorithm: INTEGRITY_ALGORITHM_V2,
    manifestHash: "0".repeat(64),
    contentRootHash: "5".repeat(64),
    fileCount: 2,
    totalBytes: 500,
    files: [
      {
        path: "actions/idle/action.json",
        sha256: "a".repeat(64),
        bytes: 250,
        mediaType: "application/json",
        role: "action_config",
        actionKey: "idle",
      },
      {
        path: "actions/wave/action.json",
        sha256: "b".repeat(64),
        bytes: 250,
        mediaType: "application/json",
        role: "action_config",
        actionKey: "wave",
      },
    ],
  },
};

const goldenIdleAction = {
  schemaVersion: 2,
  actionKey: "idle",
  displayName: "Idle",
  version: 1,
  fps: 8,
  playbackMode: "loop",
  interruptible: true,
  priority: 50,
  cooldownMs: 0,
  minimumPlayMs: 0,
  maximumPlayMs: null,
  mutexGroup: null,
  supportsDefaultIdle: true,
  isStableStateCandidate: true,
  isTransitionOnly: false,
  returnTo: { type: "default" },
  anchor: { x: 0.5, y: 1.0, coordinateSpace: "normalized_canvas" },
  frames: [
    { file: "frame0.png", index: 0, durationMs: 125, contentHash: "c".repeat(64), frameId: "idle_0", assetId: "idle_asset_0" },
    { file: "frame1.png", index: 1, durationMs: 125, contentHash: "d".repeat(64), frameId: "idle_1", assetId: "idle_asset_1" },
    { file: "frame2.png", index: 2, durationMs: 125, contentHash: "e".repeat(64), frameId: "idle_2", assetId: "idle_asset_2" },
    { file: "frame3.png", index: 3, durationMs: 125, contentHash: "f".repeat(64), frameId: "idle_3", assetId: "idle_asset_3" },
  ],
};

const goldenWaveAction = {
  schemaVersion: 2,
  actionKey: "wave",
  displayName: "Wave",
  version: 1,
  fps: 10,
  playbackMode: "once",
  interruptible: true,
  priority: 50,
  cooldownMs: 0,
  minimumPlayMs: 0,
  maximumPlayMs: null,
  mutexGroup: null,
  supportsDefaultIdle: true,
  isStableStateCandidate: false,
  isTransitionOnly: true,
  returnTo: { type: "action", actionKey: "idle" },
  anchor: { x: 0.5, y: 1.0, coordinateSpace: "normalized_canvas" },
  frames: [
    { file: "wave0.png", index: 0, durationMs: 100, contentHash: "1".repeat(64), frameId: "wave_0", assetId: "wave_asset_0" },
    { file: "wave1.png", index: 1, durationMs: 100, contentHash: "2".repeat(64), frameId: "wave_1", assetId: "wave_asset_1" },
    { file: "wave2.png", index: 2, durationMs: 100, contentHash: "3".repeat(64), frameId: "wave_2", assetId: "wave_asset_2" },
  ],
};

describe("Golden Package", () => {
  const reader = new Schema2PackageReader();
  const normalizer = new RuntimePackageNormalizer();

  it("golden manifest 被正确解析", () => {
    const result = reader.readManifest(goldenManifest);
    expect(result.data.schemaVersion).toBe(2);
    expect(result.data.petId).toBe("golden-pet");
    expect(result.data.displayName).toBe("Golden Pet");
    expect(result.data.defaultActionKey).toBe("idle");
    expect(result.data.canvas).toEqual({
      width: 256,
      height: 256,
      coordinateSystem: "top-left",
    });
    expect(result.data.actionEntries).toHaveLength(2);
    expect(result.data.actionEntries[0]).toMatchObject({
      key: "idle",
      name: "Idle",
      config: "actions/idle/action.json",
      playbackMode: "loop",
      fps: 8,
      frameCount: 4,
    });
    expect(result.data.actionEntries[1]).toMatchObject({
      key: "wave",
      name: "Wave",
      config: "actions/wave/action.json",
      playbackMode: "once",
      fps: 10,
      frameCount: 3,
    });
    expect(result.data.compatibility.minRuntimeVersion).toBe("1.0.0");
    expect(result.data.integrity.contentRootHash).toBe("5".repeat(64));
    expect(result.data.integrity.algorithm).toBe(INTEGRITY_ALGORITHM_V2);
  });

  it("golden idle action 被正确规范化", () => {
    const result = reader.readAction(
      goldenIdleAction,
      "idle",
      "actions/idle/action.json",
    );
    expect(result.action.actionKey).toBe("idle");
    expect(result.action.displayName).toBe("Idle");
    expect(result.action.playbackMode).toBe("loop");
    expect(result.action.fps).toBe(8);
    expect(result.action.configPath).toBe("actions/idle/action.json");
    expect(result.action.frames).toHaveLength(4);
    expect(result.action.frames[0]).toEqual({
      frameId: "idle_0",
      index: 0,
      file: "frame0.png",
      durationMs: 125,
      contentHash: "c".repeat(64),
      assetId: "idle_asset_0",
    });
    expect(result.action.frames[3]).toEqual({
      frameId: "idle_3",
      index: 3,
      file: "frame3.png",
      durationMs: 125,
      contentHash: "f".repeat(64),
      assetId: "idle_asset_3",
    });
  });

  it("golden wave action 被正确规范化", () => {
    const result = reader.readAction(
      goldenWaveAction,
      "wave",
      "actions/wave/action.json",
    );
    expect(result.action.actionKey).toBe("wave");
    expect(result.action.displayName).toBe("Wave");
    expect(result.action.playbackMode).toBe("once");
    expect(result.action.fps).toBe(10);
    expect(result.action.returnTo).toEqual({ type: "action", actionKey: "idle" });
    expect(result.action.configPath).toBe("actions/wave/action.json");
    expect(result.action.frames).toHaveLength(3);
    expect(result.action.frames[0].file).toBe("wave0.png");
    expect(result.action.frames[0].index).toBe(0);
    expect(result.action.frames[2].file).toBe("wave2.png");
    expect(result.action.frames[2].index).toBe(2);
  });

  it("playbackMode ping_pong 在 golden 数据中被正确处理", () => {
    const goldenPingPongAction = {
      schemaVersion: 2,
      actionKey: "bounce",
      displayName: "Bounce",
      version: 1,
      fps: 10,
      playbackMode: "ping-pong",
      interruptible: true,
      priority: 50,
      cooldownMs: 0,
      minimumPlayMs: 0,
      maximumPlayMs: null,
      mutexGroup: null,
      supportsDefaultIdle: true,
      isStableStateCandidate: false,
      isTransitionOnly: true,
      returnTo: { type: "default" },
      anchor: { x: 0.5, y: 1.0, coordinateSpace: "normalized_canvas" },
      frames: [
        { file: "b0.png", index: 0, durationMs: 80, contentHash: "4".repeat(64), frameId: "bounce_0", assetId: "bounce_asset_0" },
      ],
    };
    const result = reader.readAction(
      goldenPingPongAction,
      "bounce",
      "actions/bounce/action.json",
    );
    expect(result.action.playbackMode).toBe("ping_pong");
  });

  it("returnTo 规则一致性：通过 normalizer 验证", () => {
    const actions = new Map<string, unknown>([
      ["idle", goldenIdleAction],
      ["wave", goldenWaveAction],
    ]);
    const pkg = normalizer.normalize(goldenManifest, actions, "/golden-pkg", RUNTIME_VERSION);
    expect(pkg.actions.get("wave")?.returnTo).toEqual({
      type: "action",
      actionKey: "idle",
    });
    expect(pkg.actions.get("idle")?.returnTo).toEqual({ type: "default" });
  });

  it("帧路径和索引一致性：通过 normalizer 验证", () => {
    const actions = new Map<string, unknown>([
      ["idle", goldenIdleAction],
      ["wave", goldenWaveAction],
    ]);
    const pkg = normalizer.normalize(goldenManifest, actions, "/golden-pkg", RUNTIME_VERSION);
    const idleFrames = pkg.actions.get("idle")?.frames;
    expect(idleFrames?.map((f) => f.file)).toEqual([
      "frame0.png",
      "frame1.png",
      "frame2.png",
      "frame3.png",
    ]);
    expect(idleFrames?.map((f) => f.index)).toEqual([0, 1, 2, 3]);
    const waveFrames = pkg.actions.get("wave")?.frames;
    expect(waveFrames?.map((f) => f.file)).toEqual([
      "wave0.png",
      "wave1.png",
      "wave2.png",
    ]);
    expect(waveFrames?.map((f) => f.index)).toEqual([0, 1, 2]);
  });

  it("compareVersions 对 golden minRuntimeVersion 的判断", () => {
    const minVersion = goldenManifest.compatibility.minRuntimeVersion;
    expect(compareVersions(minVersion, RUNTIME_VERSION)).toBe(true);
    expect(compareVersions(minVersion, "0.9.0")).toBe(false);
    expect(compareVersions(minVersion, "1.0.0")).toBe(true);
    expect(compareVersions(minVersion, "2.0.0")).toBe(true);
  });

  it("golden package 通过 normalizer 完整规范化后所有字段一致", () => {
    const actions = new Map<string, unknown>([
      ["idle", goldenIdleAction],
      ["wave", goldenWaveAction],
    ]);
    const pkg = normalizer.normalize(goldenManifest, actions, "/golden-pkg", RUNTIME_VERSION);
    expect(pkg.schemaVersion).toBe(2);
    expect(pkg.sourceSchemaVersion).toBe(2);
    expect(pkg.petId).toBe("golden-pet");
    expect(pkg.displayName).toBe("Golden Pet");
    expect(pkg.defaultActionKey).toBe("idle");
    expect(pkg.compatibility.minRuntimeVersion).toBe("1.0.0");
    expect(pkg.compatibility.renderMode).toBe("sprite");
    expect(pkg.integrity.contentRootHash).toBe("5".repeat(64));
    expect(pkg.integrity.algorithm).toBe(INTEGRITY_ALGORITHM_V2);
    expect(pkg.integrity.manifestHash).toBe("0".repeat(64));
    expect(pkg.packageRoot).toBe("/golden-pkg");
    expect(pkg.actions.size).toBe(2);
    expect(pkg.actions.has("idle")).toBe(true);
    expect(pkg.actions.has("wave")).toBe(true);
  });
});
