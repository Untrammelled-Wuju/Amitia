import { describe, it, expect } from "vitest";
import { normalizeActionConfig, createActionNormalizer } from "../loaders/action-config-normalizer";
import type { RawActionConfig, PackagePlaybackSnapshot } from "../contracts";
import { PlaybackError } from "../errors";

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
    fps: 10,
    frameDurationMs: 100,
    frameCount: 4,
    frames: ["frame_0.png", "frame_1.png", "frame_2.png", "frame_3.png"],
    ...overrides,
  };
}

describe("normalizeActionConfig", () => {
  describe("帧规范化", () => {
    it("字符串帧基础规范化 - 帧索引 0-3, 时长正确", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig(),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.frames).toHaveLength(4);
      expect(result.frames[0].index).toBe(0);
      expect(result.frames[1].index).toBe(1);
      expect(result.frames[2].index).toBe(2);
      expect(result.frames[3].index).toBe(3);
      expect(result.frames[0].resourceUrl).toBe("frame_0.png");
      expect(result.frames[1].resourceUrl).toBe("frame_1.png");
      expect(result.frames[2].resourceUrl).toBe("frame_2.png");
      expect(result.frames[3].resourceUrl).toBe("frame_3.png");
      expect(result.frames[0].durationMs).toBe(100);
      expect(result.frames[1].durationMs).toBe(100);
      expect(result.frames[2].durationMs).toBe(100);
      expect(result.frames[3].durationMs).toBe(100);
      expect(result.frames[0].cumulativeStartMs).toBe(0);
      expect(result.frames[0].cumulativeEndMs).toBe(100);
      expect(result.frames[1].cumulativeStartMs).toBe(100);
      expect(result.frames[1].cumulativeEndMs).toBe(200);
      expect(result.frames[2].cumulativeStartMs).toBe(200);
      expect(result.frames[2].cumulativeEndMs).toBe(300);
      expect(result.frames[3].cumulativeStartMs).toBe(300);
      expect(result.frames[3].cumulativeEndMs).toBe(400);
    });

    it("对象帧使用显式索引进行规范化并排序", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({
          frames: [
            { index: 3, file: "frame_3.png" },
            { index: 1, file: "frame_1.png" },
            { index: 0, file: "frame_0.png" },
            { index: 2, file: "frame_2.png" },
          ],
        }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.frames).toHaveLength(4);
      expect(result.frames[0].index).toBe(0);
      expect(result.frames[0].resourceUrl).toBe("frame_0.png");
      expect(result.frames[1].index).toBe(1);
      expect(result.frames[1].resourceUrl).toBe("frame_1.png");
      expect(result.frames[2].index).toBe(2);
      expect(result.frames[2].resourceUrl).toBe("frame_2.png");
      expect(result.frames[3].index).toBe(3);
      expect(result.frames[3].resourceUrl).toBe("frame_3.png");
    });

    it("schema v2 accepts fully identified frames and normalized anchor", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({
          anchor: { type: "bottom_center", x: 0.5, y: 1, coordinateSpace: "normalized_canvas" },
          frames: [0, 1, 2, 3].map((index) => ({
            index,
            file: `frame_${index}.png`,
            frameId: `idle-frame-${index}`,
            assetId: `asset-${index}`,
            contentHash: `sha256:frame-${index}`,
          })),
        }),
        packageSnapshot: makePackageSnapshot({ schemaVersion: 2 }),
      });

      expect(result.frames).toHaveLength(4);
      expect(result.frames[0]).toMatchObject({
        frameId: "idle-frame-0",
        assetId: "asset-0",
        contentHash: "sha256:frame-0",
      });
      expect(result.anchor).toEqual({ type: "bottom_center", x: 0.5, y: 1 });
    });

    it("对象帧使用每帧独立 durationMs", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({
          frames: [
            { file: "frame_0.png", durationMs: 50 },
            { file: "frame_1.png", durationMs: 150 },
            { file: "frame_2.png", durationMs: 200 },
            { file: "frame_3.png", durationMs: 80 },
          ],
        }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.frames[0].durationMs).toBe(50);
      expect(result.frames[1].durationMs).toBe(150);
      expect(result.frames[2].durationMs).toBe(200);
      expect(result.frames[3].durationMs).toBe(80);
      expect(result.frames[0].cumulativeStartMs).toBe(0);
      expect(result.frames[0].cumulativeEndMs).toBe(50);
      expect(result.frames[1].cumulativeStartMs).toBe(50);
      expect(result.frames[1].cumulativeEndMs).toBe(200);
      expect(result.frames[2].cumulativeStartMs).toBe(200);
      expect(result.frames[2].cumulativeEndMs).toBe(400);
      expect(result.frames[3].cumulativeStartMs).toBe(400);
      expect(result.frames[3].cumulativeEndMs).toBe(480);
    });
  });

  describe("循环类型规范化", () => {
    it("loop 类型保持不变", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "loop" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.loopType).toBe("loop");
    });

    it("once 类型保持不变", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "once" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.loopType).toBe("once");
    });

    it("hold 类型保持不变", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "hold" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.loopType).toBe("hold");
    });

    it("ping_pong 类型保持不变", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "ping_pong" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.loopType).toBe("ping_pong");
    });

    it("旧版 pingpong 转换为 ping_pong 并产生警告", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "pingpong" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.loopType).toBe("ping_pong");
      expect(result.warnings.some((w) => w.includes("legacy_loop_type_alias"))).toBe(true);
    });

    it("新 schema 下未知循环类型抛出 PlaybackError", () => {
      expect(() =>
        normalizeActionConfig({
          raw: makeRawConfig({ loopType: "unknown_type" }),
          packageSnapshot: makePackageSnapshot({ schemaVersion: 2 }),
        }),
      ).toThrow(PlaybackError);
    });

    it("旧 schema 下未知循环类型默认为 loop 并产生警告", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "unknown_type" }),
        packageSnapshot: makePackageSnapshot({ schemaVersion: 1 }),
      });

      expect(result.loopType).toBe("loop");
      expect(result.warnings.some((w) => w.includes("unknown_loop_type_defaulting_to_loop"))).toBe(true);
    });
  });

  describe("帧时长解析", () => {
    it("缺少 frameDurationMs 时从 fps 计算帧时长", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ fps: 20, frameDurationMs: undefined as unknown as number }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.frames[0].durationMs).toBe(50);
      expect(result.frames[1].durationMs).toBe(50);
      expect(result.frames[2].durationMs).toBe(50);
      expect(result.frames[3].durationMs).toBe(50);
    });

    it("存在 frameDurationMs 时优先使用", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ fps: 20, frameDurationMs: 80 }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.frames[0].durationMs).toBe(80);
      expect(result.frames[1].durationMs).toBe(80);
    });

    it("旧版时序回退 - 无 fps 和 frameDurationMs, 旧 schema 使用 100ms 并产生警告", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({
          fps: undefined as unknown as number,
          frameDurationMs: undefined as unknown as number,
        }),
        packageSnapshot: makePackageSnapshot({ schemaVersion: 1 }),
      });

      expect(result.frames[0].durationMs).toBe(100);
      expect(result.warnings.some((w) => w.includes("legacy_timing_fallback"))).toBe(true);
    });
  });

  describe("返回目标解析", () => {
    it("once 循环无 returnAction 返回 default", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "once" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.returnTarget).toEqual({ type: "default" });
    });

    it("loop 循环无 returnAction 返回 none", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "loop" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.returnTarget).toEqual({ type: "none" });
    });

    it("有效的 returnAction 返回 action", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ actionKey: "idle", returnAction: "wave" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.returnTarget).toEqual({ type: "action", actionKey: "wave" });
    });

    it("自引用 returnAction 返回 default 并产生警告", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ actionKey: "idle", returnAction: "idle" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.returnTarget).toEqual({ type: "default" });
      expect(result.warnings.some((w) => w.includes("return_action_self_reference_ignored"))).toBe(true);
    });

    it("不存在的 returnAction 返回 default 并产生警告", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ returnAction: "nonexistent" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.returnTarget).toEqual({ type: "default" });
      expect(result.warnings.some((w) => w.includes("return_action_not_found"))).toBe(true);
    });
  });

  describe("锚点解析", () => {
    it("显式锚点值被使用", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ anchor: { type: "top_left", x: 10, y: 20 } }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.anchor).toEqual({ type: "top_left", x: 10, y: 20 });
    });

    it("缺少锚点时默认为 bottom_center 位于画布中心", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig(),
        packageSnapshot: makePackageSnapshot({ canvas: { width: 256, height: 256 } }),
      });

      expect(result.anchor).toEqual({ type: "bottom_center", x: 128, y: 256 });
    });
  });

  describe("错误处理", () => {
    it("空帧抛出 PlaybackError", () => {
      expect(() =>
        normalizeActionConfig({
          raw: makeRawConfig({ frames: [] }),
          packageSnapshot: makePackageSnapshot(),
        }),
      ).toThrow(PlaybackError);
    });

    it("新 schema 下帧数不匹配抛出 PlaybackError", () => {
      expect(() =>
        normalizeActionConfig({
          raw: makeRawConfig({
            frameCount: 5,
            frames: ["a.png", "b.png", "c.png", "d.png"],
          }),
          packageSnapshot: makePackageSnapshot({ schemaVersion: 2 }),
        }),
      ).toThrow(PlaybackError);
    });

    it("旧 schema 下帧数不匹配产生警告", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({
          frameCount: 5,
          frames: ["a.png", "b.png", "c.png", "d.png"],
        }),
        packageSnapshot: makePackageSnapshot({ schemaVersion: 1 }),
      });

      expect(result.warnings.some((w) => w.includes("frame_count_mismatch"))).toBe(true);
      expect(result.warnings.some((w) => w.includes("declared=5"))).toBe(true);
      expect(result.warnings.some((w) => w.includes("actual=4"))).toBe(true);
    });

    it("重复帧索引抛出 PlaybackError", () => {
      expect(() =>
        normalizeActionConfig({
          raw: makeRawConfig({
            frames: [
              { index: 0, file: "a.png" },
              { index: 1, file: "b.png" },
              { index: 0, file: "c.png" },
              { index: 2, file: "d.png" },
            ],
          }),
          packageSnapshot: makePackageSnapshot(),
        }),
      ).toThrow(PlaybackError);
    });
  });

  describe("时长计算", () => {
    it("变时长帧的 cycleDurationMs 和 baseDurationMs 计算正确", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({
          frames: [
            { file: "a.png", durationMs: 50 },
            { file: "b.png", durationMs: 100 },
            { file: "c.png", durationMs: 150 },
            { file: "d.png", durationMs: 200 },
          ],
        }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.cycleDurationMs).toBe(500);
      expect(result.baseDurationMs).toBe(125);
    });

    it("等时长帧的 cycleDurationMs 和 baseDurationMs 计算正确", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig(),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.cycleDurationMs).toBe(400);
      expect(result.baseDurationMs).toBe(100);
    });
  });

  describe("默认值", () => {
    it("loop 类型的 isStableStateCandidate 默认为 true", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "loop" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.isStableStateCandidate).toBe(true);
    });

    it("非 loop 类型的 isStableStateCandidate 默认为 false", () => {
      const result = normalizeActionConfig({
        raw: makeRawConfig({ loopType: "once" }),
        packageSnapshot: makePackageSnapshot(),
      });

      expect(result.isStableStateCandidate).toBe(false);
    });

    it("其他默认字段值正确", () => {
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
      expect(result.supportsDefaultIdle).toBe(true);
      expect(result.isTransitionOnly).toBe(false);
    });
  });
});

describe("createActionNormalizer", () => {
  it("返回可正常工作的规范化函数", () => {
    const normalizer = createActionNormalizer();
    const result = normalizer({
      raw: makeRawConfig(),
      packageSnapshot: makePackageSnapshot(),
    });

    expect(result.actionKey).toBe("idle");
    expect(result.frames).toHaveLength(4);
    expect(result.loopType).toBe("loop");
    expect(result.cycleDurationMs).toBe(400);
  });

  it("多次调用返回独立的规范化结果", () => {
    const normalizer = createActionNormalizer();

    const result1 = normalizer({
      raw: makeRawConfig({ actionKey: "idle", loopType: "loop" }),
      packageSnapshot: makePackageSnapshot(),
    });
    const result2 = normalizer({
      raw: makeRawConfig({ actionKey: "wave", loopType: "once", displayName: "Wave" }),
      packageSnapshot: makePackageSnapshot(),
    });

    expect(result1.actionKey).toBe("idle");
    expect(result1.loopType).toBe("loop");
    expect(result2.actionKey).toBe("wave");
    expect(result2.loopType).toBe("once");
    expect(result2.displayName).toBe("Wave");
  });
});
