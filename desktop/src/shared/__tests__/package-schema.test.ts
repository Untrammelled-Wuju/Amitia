import { describe, it, expect } from "vitest";
import {
  Schema1PackageReader,
  Schema2PackageReader,
  RuntimePackageNormalizer,
  compareVersions,
} from "../package-schema";

describe("compareVersions", () => {
  it("1.0.0 vs 1.0.0 返回 true", () => {
    expect(compareVersions("1.0.0", "1.0.0")).toBe(true);
  });

  it("2.0.0 vs 1.0.0 返回 true", () => {
    expect(compareVersions("1.0.0", "2.0.0")).toBe(true);
  });

  it("0.9.0 vs 1.0.0 返回 false", () => {
    expect(compareVersions("1.0.0", "0.9.0")).toBe(false);
  });

  it("1.0.0 vs 2.0.0 返回 false", () => {
    expect(compareVersions("2.0.0", "1.0.0")).toBe(false);
  });

  it("1.0 vs 1.0.0 返回 true（缺少的部分补 0）", () => {
    expect(compareVersions("1.0.0", "1.0")).toBe(true);
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
          { key: "idle", name: "Idle" },
          { key: "wave" },
        ],
        compatibility: { minRuntimeVersion: "1.0.0" },
        integrity: { contentRootHash: "hash-1" },
      };
      const manifest = reader.readManifest(raw);
      expect(manifest.schemaVersion).toBe(1);
      expect(manifest.petId).toBe("test-pet-1");
      expect(manifest.displayName).toBe("Test Pet");
      expect(manifest.defaultActionKey).toBe("idle");
      expect(manifest.canvas).toEqual({ width: 128, height: 128 });
      expect(manifest.actionEntries).toEqual([
        { key: "idle", name: "Idle", config: "actions/idle/action.json" },
        { key: "wave", name: "wave", config: "actions/wave/action.json" },
      ]);
      expect(manifest.minimumRuntimeVersion).toBe("1.0.0");
      expect(manifest.contentRootHash).toBe("hash-1");
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
      const action = reader.readAction(raw, "idle", "actions/idle/action.json");
      expect(action.actionKey).toBe("idle");
      expect(action.displayName).toBe("Idle Action");
      expect(action.playbackMode).toBe("once");
      expect(action.priority).toBe(50);
      expect(action.fps).toBe(10);
      expect(action.configPath).toBe("actions/idle/action.json");
      expect(action.frames).toHaveLength(2);
    });
  });
});

describe("Schema2PackageReader", () => {
  const reader = new Schema2PackageReader();

  describe("readManifest", () => {
    it("正确解析 schemaVersion=2 的 manifest，minRuntimeVersion 从 compatibility.minimumRuntimeVersion 读取", () => {
      const raw = {
        schemaVersion: 2,
        petId: "test-pet-2",
        name: "Test Pet 2",
        defaultAction: "idle",
        canvas: { width: 256, height: 256 },
        actions: [{ key: "idle" }],
        compatibility: { minimumRuntimeVersion: "2.0.0" },
        integrity: { contentRootHash: "hash-2" },
      };
      const manifest = reader.readManifest(raw);
      expect(manifest.schemaVersion).toBe(2);
      expect(manifest.petId).toBe("test-pet-2");
      expect(manifest.displayName).toBe("Test Pet 2");
      expect(manifest.defaultActionKey).toBe("idle");
      expect(manifest.minimumRuntimeVersion).toBe("2.0.0");
    });
  });

  describe("readAction", () => {
    it("playbackMode 直接读取，fps 不回退到 defaultFps", () => {
      const raw = {
        playbackMode: "ping_pong",
        defaultFps: 30,
        frames: [{ file: "f0.png" }],
      };
      const action = reader.readAction(raw, "wave", "actions/wave/action.json");
      expect(action.playbackMode).toBe("ping_pong");
      expect(action.fps).toBe(0);
    });
  });
});

describe("RuntimePackageNormalizer", () => {
  const normalizer = new RuntimePackageNormalizer();

  it("Schema 1 输入 → sourceSchemaVersion=1, schemaVersion=2", () => {
    const manifest = {
      schemaVersion: 1,
      packageId: "pet-s1",
      name: "Pet S1",
      defaultAction: "idle",
      actions: [{ key: "idle" }],
      compatibility: { minRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    expect(result.sourceSchemaVersion).toBe(1);
    expect(result.schemaVersion).toBe(2);
  });

  it("Schema 2 输入 → sourceSchemaVersion=2, schemaVersion=2", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-s2",
      name: "Pet S2",
      defaultAction: "idle",
      actions: [{ key: "idle" }],
      compatibility: { minimumRuntimeVersion: "1.0.0" },
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 8, frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    expect(result.sourceSchemaVersion).toBe(2);
    expect(result.schemaVersion).toBe(2);
  });

  it("ping-pong playbackMode 被转换为 ping_pong", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-pp",
      name: "Pet PP",
      actions: [{ key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "ping-pong", frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    expect(result.actions.get("idle")?.playbackMode).toBe("ping_pong");
  });

  it("pingpong playbackMode 被转换为 ping_pong", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-pp2",
      name: "Pet PP2",
      actions: [{ key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "pingpong", frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    expect(result.actions.get("idle")?.playbackMode).toBe("ping_pong");
  });

  it("未知 playbackMode 抛出 UNKNOWN_PLAYBACK_MODE 错误", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-unk",
      name: "Pet Unk",
      actions: [{ key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "invalid-mode", frames: ["f0.png"] }],
    ]);
    expect(() => normalizer.normalize(manifest, actions, "/pkg")).toThrow(
      "UNKNOWN_PLAYBACK_MODE",
    );
  });

  it("returnTo { type: action, actionKey: wave } 被正确解析", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-rt",
      name: "Pet RT",
      actions: [{ key: "wave" }, { key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", frames: ["f0.png"] }],
      [
        "wave",
        {
          playbackMode: "once",
          returnTo: { type: "action", actionKey: "idle" },
          frames: ["w0.png"],
        },
      ],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    expect(result.actions.get("wave")?.returnTo).toEqual({
      type: "action",
      actionKey: "idle",
    });
  });

  it("returnAction 字符串被转换为 { type: action, actionKey: ... }", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-ra",
      name: "Pet RA",
      actions: [{ key: "wave" }, { key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", frames: ["f0.png"] }],
      [
        "wave",
        { playbackMode: "once", returnAction: "idle", frames: ["w0.png"] },
      ],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    expect(result.actions.get("wave")?.returnTo).toEqual({
      type: "action",
      actionKey: "idle",
    });
  });

  it("无 returnTo 和 returnAction 时默认为 { type: default }", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-def",
      name: "Pet Def",
      actions: [{ key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    expect(result.actions.get("idle")?.returnTo).toEqual({ type: "default" });
  });

  it("anchor 默认为 { x: 0.5, y: 1.0, coordinateSpace: normalized_canvas }", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-anchor",
      name: "Pet Anchor",
      actions: [{ key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", frames: ["f0.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    expect(result.actions.get("idle")?.anchor).toEqual({
      x: 0.5,
      y: 1.0,
      coordinateSpace: "normalized_canvas",
    });
  });

  it("字符串帧被正确转换为 RuntimeFrame 对象", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-str",
      name: "Pet Str",
      actions: [{ key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      ["idle", { playbackMode: "loop", fps: 10, frames: ["f0.png", "f1.png"] }],
    ]);
    const result = normalizer.normalize(manifest, actions, "/pkg");
    const frames = result.actions.get("idle")?.frames;
    expect(frames).toHaveLength(2);
    expect(frames?.[0]).toEqual({
      frameId: "idle_frame_0",
      index: 0,
      file: "f0.png",
      durationMs: 100,
      contentHash: "",
    });
    expect(frames?.[1]).toEqual({
      frameId: "idle_frame_1",
      index: 1,
      file: "f1.png",
      durationMs: 100,
      contentHash: "",
    });
  });

  it("对象帧使用显式 index 和 file", () => {
    const manifest = {
      schemaVersion: 2,
      petId: "pet-obj",
      name: "Pet Obj",
      actions: [{ key: "idle" }],
    };
    const actions = new Map<string, unknown>([
      [
        "idle",
        {
          playbackMode: "loop",
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
    const result = normalizer.normalize(manifest, actions, "/pkg");
    const frames = result.actions.get("idle")?.frames;
    expect(frames).toHaveLength(1);
    expect(frames?.[0]).toEqual({
      frameId: "custom-id",
      index: 5,
      file: "custom.png",
      durationMs: 200,
      contentHash: "ch-1",
    });
  });
});
