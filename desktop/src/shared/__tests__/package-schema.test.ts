import { describe, it, expect } from "vitest";
import {
  CanonicalPackageReader,
  RuntimePackageNormalizer,
  compareVersions,
  isValidSemVer,
  INTEGRITY_ALGORITHM_V1,
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
    expect(compareVersions("1.0.0", "1.0.0-alpha")).toBe(false);
  });
});

describe("CanonicalPackageReader", () => {
  const reader = new CanonicalPackageReader();

  function buildValidManifest(overrides: Record<string, unknown> = {}): unknown {
    return {
      schemaVersion: 1,
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
      compatibility: { minRuntimeVersion: "1.0.0", renderMode: "sprite" },
      binding: { policy: "bound" },
      capabilities: {
        transparentBackground: true,
        frameSequence: true,
        perFrameDuration: true,
        audio: false,
      },
      provenance: { builder: "package-schema-test", sourceType: "generated" },
      integrity: {
        algorithm: INTEGRITY_ALGORITHM_V1,
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
      schemaVersion: 1,
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
    it("正确解析 schemaVersion=1 的 manifest", () => {
      const raw = buildValidManifest();
      const result = reader.readManifest(raw);
      expect(result.data.schemaVersion).toBe(1);
      expect(result.data.petId).toBe("test-pet-2");
      expect(result.data.displayName).toBe("Test Pet 2");
      expect(result.data.defaultActionKey).toBe("idle");
      expect(result.data.compatibility.minRuntimeVersion).toBe("1.0.0");
      expect(result.data.integrity.algorithm).toBe(INTEGRITY_ALGORITHM_V1);
      expect(result.data.integrity.manifestHash).toBe("0".repeat(64));
      expect(result.warnings).toHaveLength(0);
    });

    it("缺少 schemaVersion 时抛出错误", () => {
      const raw = buildValidManifest({ schemaVersion: undefined });
      expect(() => reader.readManifest(raw)).toThrow();
    });

    it("schemaVersion 不为 1 时抛出错误", () => {
      const raw = buildValidManifest({ schemaVersion: 2 });
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

    it("V1 integrity.files 不接受 legacy hash 字段", () => {
      const raw = buildValidManifest({
        integrity: {
          algorithm: INTEGRITY_ALGORITHM_V1,
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
      expect(() => reader.readManifest(raw)).toThrow();
    });
  });

  describe("readAction", () => {
    it("playbackMode 直接读取且显式 fps 被保留", () => {
      const raw = buildValidAction({ fps: 15 });
      const result = reader.readAction(raw, "wave", "actions/wave/action.json");
      expect(result.action.playbackMode).toBe("ping_pong");
      expect(result.action.fps).toBe(15);
    });

    it("V1 action 拒绝 defaultFps 等未知顶层字段", () => {
      const raw = buildValidAction({ defaultFps: 30 });
      expect(() => reader.readAction(raw, "wave", "actions/wave/action.json")).toThrow();
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
        "assetId is required",
      );
    });
  });
});

describe("RuntimePackageNormalizer", () => {
  const normalizer = new RuntimePackageNormalizer();
  const RUNTIME_VERSION = "1.0.0";

  function canonicalManifest(overrides: Record<string, unknown> = {}) {
    return {
      schemaVersion: 1,
      manifestFormat: MANIFEST_FORMAT_CANONICAL,
      petId: "pet-1",
      releaseId: "release-1",
      version: "1.0.0",
      name: "Pet",
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
      capabilities: {
        transparentBackground: true,
        frameSequence: true,
        perFrameDuration: true,
        audio: false,
      },
      provenance: { builder: "normalizer-test", sourceType: "generated" },
      integrity: {
        algorithm: INTEGRITY_ALGORITHM_V1,
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

  function canonicalAction(actionKey: string, overrides: Record<string, unknown> = {}) {
    return {
      schemaVersion: 1,
      actionKey,
      displayName: actionKey,
      version: 1,
      playbackMode: "loop",
      fps: 8,
      interruptible: true,
      priority: 50,
      cooldownMs: 0,
      minimumPlayMs: 0,
      maximumPlayMs: null,
      mutexGroup: null,
      supportsDefaultIdle: actionKey === "idle",
      isStableStateCandidate: actionKey === "idle",
      isTransitionOnly: actionKey !== "idle",
      returnTo: { type: "default" },
      anchor: { x: 0.5, y: 1.0, coordinateSpace: "normalized_canvas" },
      frames: [
        {
          frameId: `${actionKey}_frame_0`,
          index: 0,
          file: `${actionKey}0.png`,
          durationMs: 100,
          assetId: `${actionKey}_asset_0`,
          contentHash: "c".repeat(64),
        },
      ],
      ...overrides,
    };
  }

  it("新 v1 输入 → schemaVersion=1", () => {
    const manifest = {
      schemaVersion: 1,
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
      capabilities: {
        transparentBackground: true,
        frameSequence: true,
        perFrameDuration: true,
        audio: false,
      },
      provenance: { builder: "normalizer-test", sourceType: "generated" },
      integrity: {
        algorithm: INTEGRITY_ALGORITHM_V1,
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
          schemaVersion: 1,
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
    expect(result.schemaVersion).toBe(1);
  });

  it("不支持的 schemaVersion 抛出错误", () => {
    const manifest = { schemaVersion: 99, packageId: "pet-x", name: "X" };
    const actions = new Map<string, unknown>();
    expect(() => normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION)).toThrow();
  });

  it("未知 playbackMode 抛出 UNKNOWN_PLAYBACK_MODE 错误", () => {
    const manifest = canonicalManifest();
    const actions = new Map<string, unknown>([
      ["idle", canonicalAction("idle", { playbackMode: "invalid-mode" })],
    ]);
    expect(() => normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION)).toThrow(
      "playbackMode must be one of",
    );
  });

  it("returnTo { type: action, actionKey: wave } 被正确解析", () => {
    const manifest = canonicalManifest({
      actions: [
        {
          key: "wave",
          name: "Wave",
          config: "actions/wave/action.json",
          playbackMode: "once",
          fps: 8,
          frameCount: 1,
          supportsDefaultIdle: false,
          isStableStateCandidate: false,
          isTransitionOnly: true,
        },
        ...canonicalManifest().actions,
      ],
    });
    const actions = new Map<string, unknown>([
      ["idle", canonicalAction("idle")],
      ["wave", canonicalAction("wave", { returnTo: { type: "action", actionKey: "idle" } })],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("wave")?.returnTo).toEqual({
      type: "action",
      actionKey: "idle",
    });
  });

  it("无 returnTo 和 returnAction 时默认为 { type: default }", () => {
    const manifest = canonicalManifest();
    const actions = new Map<string, unknown>([
      ["idle", canonicalAction("idle")],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("idle")?.returnTo).toEqual({ type: "default" });
  });

  it("anchor 默认为 { x: 0.5, y: 1.0, coordinateSpace: normalized_canvas }", () => {
    const manifest = canonicalManifest();
    const actions = new Map<string, unknown>([
      ["idle", canonicalAction("idle")],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    expect(result.actions.get("idle")?.anchor).toEqual({
      x: 0.5,
      y: 1.0,
      coordinateSpace: "normalized_canvas",
    });
  });

  it("对象帧使用显式 index、file 和 assetId", () => {
    const manifest = canonicalManifest();
    const actions = new Map<string, unknown>([
      ["idle", canonicalAction("idle", {
        frames: [
          {
            file: "custom.png",
            index: 5,
            durationMs: 200,
            contentHash: "d".repeat(64),
            frameId: "custom-id",
            assetId: "custom-asset",
          },
        ],
      })],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg", RUNTIME_VERSION);
    const frames = result.actions.get("idle")?.frames;
    expect(frames).toHaveLength(1);
    expect(frames?.[0]).toEqual({
      frameId: "custom-id",
      index: 5,
      file: "custom.png",
      durationMs: 200,
      assetId: "custom-asset",
      contentHash: "d".repeat(64),
    });
  });

  it("runtime version 低于 minRuntimeVersion 时抛出错误", () => {
    const manifest = canonicalManifest({
      compatibility: { minRuntimeVersion: "3.0.0", renderMode: "sprite" },
    });
    const actions = new Map<string, unknown>([
      ["idle", { schemaVersion: 1, playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
    ]);
    expect(() => normalizer.normalize(manifest, actions, "/pkg", "1.0.0")).toThrow();
  });
});
