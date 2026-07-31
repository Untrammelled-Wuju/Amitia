import { describe, it, expect } from "vitest";
import {
  Schema2PackageReader,
  RuntimePackageNormalizer,
  compareVersions,
  CURRENT_RUNTIME_VERSION,
} from "../package-schema";

const goldenManifest = {
  schemaVersion: 2,
  petId: "golden-pet",
  name: "Golden Pet",
  defaultAction: "idle",
  canvas: { width: 256, height: 256 },
  actions: [
    { key: "idle", name: "Idle", config: "actions/idle/action.json" },
    { key: "wave", name: "Wave", config: "actions/wave/action.json" },
  ],
  compatibility: { minimumRuntimeVersion: "1.0.0" },
  integrity: { contentRootHash: "golden-hash" },
};

const goldenIdleAction = {
  actionKey: "idle",
  displayName: "Idle",
  fps: 8,
  playbackMode: "loop",
  frames: [
    { file: "frame0.png", index: 0, durationMs: 125, contentHash: "h0", frameId: "idle_0" },
    { file: "frame1.png", index: 1, durationMs: 125, contentHash: "h1", frameId: "idle_1" },
    { file: "frame2.png", index: 2, durationMs: 125, contentHash: "h2", frameId: "idle_2" },
    { file: "frame3.png", index: 3, durationMs: 125, contentHash: "h3", frameId: "idle_3" },
  ],
};

const goldenWaveAction = {
  actionKey: "wave",
  displayName: "Wave",
  fps: 10,
  playbackMode: "once",
  returnTo: { type: "action", actionKey: "idle" },
  frames: [
    { file: "wave0.png", index: 0, durationMs: 100, contentHash: "w0", frameId: "wave_0" },
    { file: "wave1.png", index: 1, durationMs: 100, contentHash: "w1", frameId: "wave_1" },
    { file: "wave2.png", index: 2, durationMs: 100, contentHash: "w2", frameId: "wave_2" },
  ],
};

describe("Golden Package", () => {
  const reader = new Schema2PackageReader();
  const normalizer = new RuntimePackageNormalizer();

  it("golden manifest 被正确解析", () => {
    const manifest = reader.readManifest(goldenManifest);
    expect(manifest.schemaVersion).toBe(2);
    expect(manifest.petId).toBe("golden-pet");
    expect(manifest.displayName).toBe("Golden Pet");
    expect(manifest.defaultActionKey).toBe("idle");
    expect(manifest.canvas).toEqual({ width: 256, height: 256 });
    expect(manifest.actionEntries).toEqual([
      { key: "idle", name: "Idle", config: "actions/idle/action.json" },
      { key: "wave", name: "Wave", config: "actions/wave/action.json" },
    ]);
    expect(manifest.minimumRuntimeVersion).toBe("1.0.0");
    expect(manifest.contentRootHash).toBe("golden-hash");
  });

  it("golden idle action 被正确规范化", () => {
    const action = reader.readAction(
      goldenIdleAction,
      "idle",
      "actions/idle/action.json",
    );
    expect(action.actionKey).toBe("idle");
    expect(action.displayName).toBe("Idle");
    expect(action.playbackMode).toBe("loop");
    expect(action.fps).toBe(8);
    expect(action.configPath).toBe("actions/idle/action.json");
    expect(action.frames).toHaveLength(4);
    expect(action.frames[0]).toEqual({
      frameId: "idle_0",
      index: 0,
      file: "frame0.png",
      durationMs: 125,
      contentHash: "h0",
    });
    expect(action.frames[3]).toEqual({
      frameId: "idle_3",
      index: 3,
      file: "frame3.png",
      durationMs: 125,
      contentHash: "h3",
    });
  });

  it("golden wave action 被正确规范化", () => {
    const action = reader.readAction(
      goldenWaveAction,
      "wave",
      "actions/wave/action.json",
    );
    expect(action.actionKey).toBe("wave");
    expect(action.displayName).toBe("Wave");
    expect(action.playbackMode).toBe("once");
    expect(action.fps).toBe(10);
    expect(action.returnTo).toEqual({ type: "action", actionKey: "idle" });
    expect(action.configPath).toBe("actions/wave/action.json");
    expect(action.frames).toHaveLength(3);
    expect(action.frames[0].file).toBe("wave0.png");
    expect(action.frames[0].index).toBe(0);
    expect(action.frames[2].file).toBe("wave2.png");
    expect(action.frames[2].index).toBe(2);
  });

  it("playbackMode ping_pong 在 golden 数据中被正确处理", () => {
    const goldenPingPongAction = {
      actionKey: "bounce",
      displayName: "Bounce",
      playbackMode: "ping-pong",
      frames: [
        { file: "b0.png", index: 0, durationMs: 80, contentHash: "b0", frameId: "bounce_0" },
      ],
    };
    const action = reader.readAction(
      goldenPingPongAction,
      "bounce",
      "actions/bounce/action.json",
    );
    expect(action.playbackMode).toBe("ping_pong");
  });

  it("returnTo 规则一致性：通过 normalizer 验证", () => {
    const actions = new Map<string, unknown>([
      ["idle", goldenIdleAction],
      ["wave", goldenWaveAction],
    ]);
    const pkg = normalizer.normalize(goldenManifest, actions, "/golden-pkg");
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
    const pkg = normalizer.normalize(goldenManifest, actions, "/golden-pkg");
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
    const minVersion = goldenManifest.compatibility.minimumRuntimeVersion;
    expect(compareVersions(minVersion, CURRENT_RUNTIME_VERSION)).toBe(true);
    expect(compareVersions(minVersion, "0.9.0")).toBe(false);
    expect(compareVersions(minVersion, "1.0.0")).toBe(true);
    expect(compareVersions(minVersion, "2.0.0")).toBe(true);
  });

  it("golden package 通过 normalizer 完整规范化后所有字段一致", () => {
    const actions = new Map<string, unknown>([
      ["idle", goldenIdleAction],
      ["wave", goldenWaveAction],
    ]);
    const pkg = normalizer.normalize(goldenManifest, actions, "/golden-pkg");
    expect(pkg.schemaVersion).toBe(2);
    expect(pkg.sourceSchemaVersion).toBe(2);
    expect(pkg.petId).toBe("golden-pet");
    expect(pkg.displayName).toBe("Golden Pet");
    expect(pkg.defaultActionKey).toBe("idle");
    expect(pkg.compatibility.minimumRuntimeVersion).toBe("1.0.0");
    expect(pkg.compatibility.renderMode).toBe("sprite");
    expect(pkg.integrity.contentRootHash).toBe("golden-hash");
    expect(pkg.packageRoot).toBe("/golden-pkg");
    expect(pkg.actions.size).toBe(2);
    expect(pkg.actions.has("idle")).toBe(true);
    expect(pkg.actions.has("wave")).toBe(true);
  });
});
