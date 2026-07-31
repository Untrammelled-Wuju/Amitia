import { describe, it, expect } from "vitest";
import {
  Schema1PackageReader,
  Schema2PackageReader,
  RuntimePackageNormalizer,
  compareVersions,
  isValidSemVer,
  INTEGRITY_ALGORITHM_V2,
  MANIFEST_FORMAT_CANONICAL,
} from "../package-schema";

describe("isValidSemVer", () => {
  it("1.0.0 是有效的 SemVer", () => {
    expect(isValidSemVer("1.0.0")).toBe(true);
  });

  it("1.0 不是有效的 SemVer（缺少 patch 段）", () => {
    expect(isValidSemVer("1.0")).toBe(false);
  });

  it("1.0.0-alpha 是有效的 SemVer", () => {
    expect(isValidSemVer("1.0.0-alpha")).toBe(true);
  });

  it("空字符串不是有效的 SemVer", () => {
    expect(isValidSemVer("")).toBe(false);
  });
});

describe("compareVersions", () => {
  it("1.0.0 vs 1.0.0 返回 true", () => {
    expect(compareVersions("1.0.0", "1.0.0")).toBe(true);
  });

  it("1.0.0 vs 2.0.0 返回 true（当前版本更高）", () => {
    expect(compareVersions("1.0.0", "2.0.0")).toBe(true);
  });

  it("1.0.0 vs 0.9.0 返回 false（当前版本更低）", () => {
    expect(compareVersions("1.0.0", "0.9.0")).toBe(false);
  });

  it("2.0.0 vs 1.0.0 返回 false（当前版本更低）", () => {
    expect(compareVersions("2.0.0", "1.0.0")).toBe(false);
  });

  it("1.0 不是有效的 SemVer，抛出错误", () => {
    expect(() => compareVersions("1.0.0", "1.0")).toThrow();
  });

  it("prerelease 版本比较：1.0.0-alpha < 1.0.0", () => {
    expect(compareVersions("1.0.0", "1.0.0-alpha")).toBe(true);
  });
});

describe("Schema1PackageReader", () => {
  const reader = new Schema1PackageReader();

  describe("readManifest", () => {
    it("正确解析 schemaVersion=1 的 manifest，petId 从 packageId 回退，actionEntries 的 config 默认为 actions/{key}/action.json", () => {
      const raw = {
        schemaVersion: 1,
        packageId: "test-pet-1",
        name: "Test Pet",
        defaultAction: "idle",
        canvas: { width: 128, height: 128 },
        actions: [
          { key: "idle", name: "Idle", loopType: "loop" },
          { key: "wave", loopType: "loop" },
        ],
        compatibility: { minRuntimeVersion: "1.0.0" },
        integrity: { contentRootHash: "hash-1" },
      };
      const result = reader.readManifest(raw);
      expect(result.data.schemaVersion).toBe(1);
      expect(result.data.petId).toBe("test-pet-1");
      expect(result.data.displayName).toBe("Test Pet");
      expect(result.data.defaultActionKey).toBe("idle");
      expect(result.data.canvas).toEqual({
        width: 128,
        height: 128,
        coordinateSystem: "top-left",
      });
      expect(result.data.actionEntries).toHaveLength(2);
      expect(result.data.actionEntries[0]).toMatchObject({
        key: "idle",
        name: "Idle",
        config: "actions/idle/action.json",
        playbackMode: "loop",
      });
      expect(result.data.actionEntries[1]).toMatchObject({
        key: "wave",
        name: "wave",
        config: "actions/wave/action.json",
        playbackMode: "loop",
      });
      expect(result.data.compatibility.minRuntimeVersion).toBe("1.0.0");
      expect(result.data.integrity.contentRootHash).toBe("hash-1");
      expect(result.warnings.some((w) => w.code === "LEGACY_PET_ID_FALLBACK")).toBe(true);
    });
  });

  describe("readAction", () => {
    it("playbackMode 从 loopType 回退，displayName 从 name 回退，默认 priority=50", () => {
      const raw = {
        name: "Idle Action",
        loopType: "once",
        fps: 10,
        frames: ["f0.png", "f1.png"],
      };
      const result = reader.readAction(raw, "idle", "actions/idle/action.json");
      expect(result.action.actionKey).toBe("idle");
      expect(result.action.displayName).toBe("Idle Action");
      expect(result.action.playbackMode).toBe("once");
      expect(result.action.priority).toBe(50);
      expect(result.action.fps).toBe(10);
      expect(result.action.configPath).toBe("actions/idle/action.json");
      expect(result.action.frames).toHaveLength(2);
    });
  });
});

describe("Schema2PackageReader", () => {
  const reader = new Schema2PackageReader();

  function buildValidManifest(overrides: Record<string, unknown> = {}): unknown {
    return {
      schemaVersion: 2,
      manifestFormat: MANIFEST_FORMAT_CANONICAL,
      petId: "test-pet-2",
      releaseId: "release-1",
      version: "1.0.0",
      name: "Test Pet 2",
      defaultAction: "idle",
      canvas: { width: 256, height: 256, coordinateSystem: "top-left" },
      actions: [
        {
          key: "idle",
          name: "Idle",
          config: "actions/idle/action.json",
          playbackMode: "loop",
          fps: 10,
          frameCount: 1,
          supportsDefaultIdle: true,
          isStableStateCandidate: true,
          isTransitionOnly: false,
        },
      ],
      compatibility: { minRuntimeVersion: "2.0.0", renderMode: "sprite" },
      binding: { policy: "bound" },
      integrity: {
        algorithm: INTEGRITY_ALGORITHM_V2,
        manifestHash: "0".repeat(64),
        contentRootHash: "1".repeat(64),
        fileCount: 1,
        totalBytes: 100,
        files: [
          {
            path: "actions/idle/action.json",
            sha256: "a".repeat(64),
            bytes: 100,
            mediaType: "application/json",
            role: "action_config",
          },
        ],
      },
      ...overrides,
    };
  }

  function buildValidAction(overrides: Record<string, unknown> = {}): unknown {
    return {
      schemaVersion: 2,
      actionKey: "wave",
      displayName: "Wave",
      version: 1,
      playbackMode: "ping_pong",
      fps: 10,
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
        {
          frameId: "wave_frame_0",
          index: 0,
          file: "f0.png",
          durationMs: 100,
          assetId: "asset-1",
          contentHash: "b".repeat(64),
        },
      ],
      ...overrides,
    };
  }

  describe("readManifest", () => {
    it("正确解析 schemaVersion=2 的 manifest", () => {
      const raw = buildValidManifest();
      const result = reader.readManifest(raw);
      expect(result.data.schemaVersion).toBe(2);
      expect(result.data.petId).toBe("test-pet-2");
      expect(result.data.displayName).toBe("Test Pet 2");
      expect(result.data.defaultActionKey).toBe("idle");
      expect(result.data.compatibility.minRuntimeVersion).toBe("2.0.0");
      expect(result.data.integrity.algorithm).toBe(INTEGRITY_ALGORITHM_V2);
      expect(result.data.integrity.manifestHash).toBe("0".repeat(64));
      expect(result.warnings).toHaveLength(0);
    });

    it("缺少 schemaVersion 时抛出错误", () => {
      const raw = buildValidManifest({ schemaVersion: undefined });
      expect(() => reader.readManifest(raw)).toThrow();
    });

    it("schemaVersion 不为 2 时抛出错误", () => {
      const raw = buildValidManifest({ schemaVersion: 1 });
      expect(() => reader.readManifest(raw)).toThrow();
    });

    it("缺少 manifestFormat 时抛出错误", () => {
      const raw = buildValidManifest({ manifestFormat: "invalid" });
      expect(() => reader.readManifest(raw)).toThrow();
    });

    it("缺少 petId 时抛出错误", () => {
      const raw = buildValidManifest({ petId: undefined });
      expect(() => reader.readManifest(raw)).toThrow();
    });

    it("integrity.algorithm 不正确时抛出错误", () => {
      const raw = buildValidManifest({
        integrity: {
          algorithm: "amitia-tree-sha256-v1",
          manifestHash: "0".repeat(64),
          contentRootHash: "1".repeat(64),
          fileCount: 1,
          totalBytes: 100,
          files: [],
        },
      });
      expect(() => reader.readManifest(raw)).toThrow();
    });

    it("integrity.files 使用 hash 字段兼容旧写法", () => {
      const raw = buildValidManifest({
        integrity: {
          algorithm: INTEGRITY_ALGORITHM_V2,
          manifestHash: "0".repeat(64),
          contentRootHash: "1".repeat(64),
          fileCount: 1,
          totalBytes: 100,
          files: [
            {
              path: "actions/idle/action.json",
              hash: "a".repeat(64),
              bytes: 100,
              mediaType: "application/json",
              role: "action_config",
            },
          ],
        },
      });
      const result = reader.readManifest(raw);
      expect(result.data.integrity.files[0].sha256).toBe("a".repeat(64));
    });
  });

  describe("readAction", () => {
    it("playbackMode 直接读取，fps 不回退到 defaultFps", () => {
      const raw = buildValidAction({
        fps: 15,
        defaultFps: 30,
      });
      const result = reader.readAction(raw, "wave", "actions/wave/action.json");
      expect(result.action.playbackMode).toBe("ping_pong");
      expect(result.action.fps).toBe(15);
    });

    it("缺少 schemaVersion 时抛出错误", () => {
      const raw = buildValidAction({ schemaVersion: undefined });
      expect(() => reader.readAction(raw, "wave", "actions/wave/action.json")).toThrow();
    });

    it("actionKey 不匹配时抛出错误", () => {
      const raw = buildValidAction({ actionKey: "different" });
      expect(() => reader.readAction(raw, "wave", "actions/wave/action.json")).toThrow();
    });

    it("缺少 frames 时抛出错误", () => {
      const raw = buildValidAction({ frames: undefined });
      expect(() => reader.readAction(raw, "wave", "actions/wave/action.json")).toThrow();
    });

    it("frame 缺少 assetId 时抛出 PACKAGE_FRAME_ASSET_ID_MISSING", () => {
      const raw = buildValidAction({
        frames: [
          {
            frameId: "wave_frame_0",
            index: 0,
            file: "f0.png",
            durationMs: 100,
            contentHash: "b".repeat(64),
          },
        ],
      });
      expect(() => reader.readAction(raw, "wave", "actions/wave/action.json")).toThrow(
        "PACKAGE_FRAME_ASSET_ID_MISSING",
      );
    });
  });
});

describe("RuntimePackageNormalizer", () => {
  const normalizer = new RuntimePackageNormalizer();
  const RUNTIME_VERSION = "2.0.0";

  it("Schema 1 输入 → sourceSchemaVersion=1, schemaVersion=2", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-s1",
      name: "Pet S1",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.sourceSchemaVersion).toBe(1);
    expect(result.schemaVersion).toBe(2);
  });

  it("Schema 2 输入 → sourceSchemaVersion=2, schemaVersion=2", () => {
    const manifest = {
      schemaVersion: 2,
      manifestFormat: MANIFEST_FORMAT_CANONICAL,
      petId: "pet-s2",
      releaseId: "rel-1",
      version: "1.0.0",
      name: "Pet S2",
      defaultAction: "idle",
      canvas: { width: 128, height: 128, coordinateSystem: "top-left" },
      actions: [
        {
          key: "idle",
          name: "Idle",
          config: "actions/idle/action.json",
          playbackMode: "loop",
          fps: 8,
          frameCount: 1,
          supportsDefaultIdle: true,
          isStableStateCandidate: true,
          isTransitionOnly: false,
        },
      ],
      compatibility: { minRuntimeVersion: "1.0.0", renderMode: "sprite" },
      binding: { policy: "bound" },
      integrity: {
        algorithm: INTEGRITY_ALGORITHM_V2,
        manifestHash: "0".repeat(64),
        contentRootHash: "1".repeat(64),
        fileCount: 1,
        totalBytes: 100,
        files: [
          {
            path: "actions/idle/action.json",
            sha256: "a".repeat(64),
            bytes: 100,
            mediaType: "application/json",
            role: "action_config",
          },
        ],
      },
    };
    const actions = new Map<string, unknown>([
      [
        "idle",
        {
          schemaVersion: 2,
          actionKey: "idle",
          displayName: "Idle",
          version: 1,
          playbackMode: "loop",
          fps: 8,
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
            {
              frameId: "idle_frame_0",
              index: 0,
              file: "f0.png",
              durationMs: 125,
              assetId: "asset-0",
              contentHash: "b".repeat(64),
            },
          ],
        },
      ],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.sourceSchemaVersion).toBe(2);
    expect(result.schemaVersion).toBe(2);
  });

  it("不支持的 schemaVersion 抛出错误", () => {
    const manifest = { schemaVersion: 99, packageId: "pet-x", name: "X" };
    const actions = new Map<string, unknown>();
    expect(() => normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION)).toThrow();
  });

  it("ping-pong playbackMode 被转换为 ping_pong", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-pp",
      name: "Pet PP",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "ping-pong", fps: 8, frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("idle")?.playbackMode).toBe("ping_pong");
  });

  it("pingpong playbackMode 被转换为 ping_pong", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-pp2",
      name: "Pet PP2",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "pingpong", fps: 8, frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("idle")?.playbackMode).toBe("ping_pong");
  });

  it("未知 playbackMode 抛出 UNKNOWN_PLAYBACK_MODE 错误", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-unk",
      name: "Pet Unk",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "invalid-mode", fps: 8, frames: ["f0.png"] }],
    ]);
    expect(() => normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION)).toThrow(
      "UNKNOWN_PLAYBACK_MODE",
    );
  });

  it("returnTo { type: action, actionKey: wave } 被正确解析", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-rt",
      name: "Pet RT",
      defaultAction: "idle",
      actions: [{ key: "wave", loopType: "once" }, { key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
      [
        "wave",
        {
          playbackMode: "once",
          fps: 8,
          returnTo: { type: "action", actionKey: "idle" },
          frames: ["w0.png"],
        },
      ],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("wave")?.returnTo).toEqual({
      type: "action",
      actionKey: "idle",
    });
  });

  it("returnAction 字符串被转换为 { type: action, actionKey: ... }", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-ra",
      name: "Pet RA",
      defaultAction: "idle",
      actions: [{ key: "wave", loopType: "once" }, { key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
      [
        "wave",
        { playbackMode: "once", fps: 8, returnAction: "idle", frames: ["w0.png"] },
      ],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("wave")?.returnTo).toEqual({
      type: "action",
      actionKey: "idle",
    });
  });

  it("无 returnTo 和 returnAction 时默认为 { type: default }", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-def",
      name: "Pet Def",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("idle")?.returnTo).toEqual({ type: "default" });
  });

  it("anchor 默认为 { x: 0.5, y: 1.0, coordinateSpace: normalized_canvas }", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-anchor",
      name: "Pet Anchor",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("idle")?.anchor).toEqual({
      x: 0.5,
      y: 1.0,
      coordinateSpace: "normalized_canvas",
    });
  });

  it("字符串帧被正确转换为 RuntimeFrame 对象（含 assetId）", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-str",
      name: "Pet Str",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 10, frames: ["f0.png", "f1.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    const frames = result.actions.get("idle")?.frames;
    expect(frames).toHaveLength(2);
    expect(frames?.[0]).toEqual({
      frameId: "idle_frame_0",
      index: 0,
      file: "f0.png",
      durationMs: 100,
      assetId: "idle_asset_0",
      contentHash: "",
    });
    expect(frames?.[1]).toEqual({
      frameId: "idle_frame_1",
      index: 1,
      file: "f1.png",
      durationMs: 100,
      assetId: "idle_asset_1",
      contentHash: "",
    });
  });

  it("对象帧使用显式 index 和 file，缺少 assetId 时自动生成", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-obj",
      name: "Pet Obj",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      [
        "idle",
        {
          playbackMode: "loop",
          fps: 10,
          frames: [
            {
              file: "custom.png",
              index: 5,
              durationMs: 200,
              contentHash: "ch-1",
              frameId: "custom-id",
            },
          ],
        },
      ],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    const frames = result.actions.get("idle")?.frames;
    expect(frames).toHaveLength(1);
    expect(frames?.[0]).toEqual({
      frameId: "custom-id",
      index: 5,
      file: "custom.png",
      durationMs: 200,
      assetId: "idle_asset_0",
      contentHash: "ch-1",
    });
  });

  it("runtime version 低于 minRuntimeVersion 时抛出错误", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-ver",
      name: "Pet Ver",
      defaultAction: "idle",
      actions: [{ key: "idle", loopType: "loop" }],
      compatibility: { minRuntimeVersion: "3.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
    ]);
    expect(() => normalizer.normalize(manifest, actions, "/pkg", "1.0.0")).toThrow();
  });
});
