import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type { PlayerCallbacks } from "../action-player";
import type { RuntimeAction } from "../resource-loader";
import type { ActionPlayer as ActionPlayerType } from "../action-player";
import {
  makeRuntimeAction,
  makeLoadedInstallation,
  installManualRaf,
  type ManualRafController,
} from "./helpers";

let ActionPlayer: typeof import("../action-player").ActionPlayer;

function makeFrameCallbacks(): PlayerCallbacks & {
  frames: Array<{ key: string; frame: number }>;
  completes: Array<{ key: string; loop: number }>;
  switches: Array<{ newKey: string; oldKey: string | null }>;
  errors: Error[];
} {
  const frames: Array<{ key: string; frame: number }> = [];
  const completes: Array<{ key: string; loop: number }> = [];
  const switches: Array<{ newKey: string; oldKey: string | null }> = [];
  const errors: Error[] = [];
  return {
    frames,
    completes,
    switches,
    errors,
    onFrameChange: (key, frame) => {
      frames.push({ key, frame });
    },
    onActionComplete: (key, loop) => {
      completes.push({ key, loop });
    },
    onActionSwitch: (newKey, oldKey) => {
      switches.push({ newKey, oldKey });
    },
    onError: (err) => {
      errors.push(err);
    },
  };
}

describe("ActionPlayer", () => {
  let player: ActionPlayerType;
  let callbacks: ReturnType<typeof makeFrameCallbacks>;
  let raf: ManualRafController;

  beforeEach(async () => {
    callbacks = makeFrameCallbacks();
    raf = installManualRaf();
    vi.resetModules();
    const mod = await import("../action-player");
    ActionPlayer = mod.ActionPlayer;
    player = new ActionPlayer(callbacks);
  });

  afterEach(() => {
    player.stop();
    raf.restore();
    vi.resetModules();
  });

  it("loop 播放类型循环播放，到达末帧后回到首帧", () => {
    const action = makeRuntimeAction({
      key: "idle",
      loopType: "loop",
      frameCount: 4,
      frameDurationMs: 100,
      fps: 10,
    });

    player.play(action);

    expect(player.getState()).toBe("playing");
    expect(player.getCurrentFrameIndex()).toBe(0);

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(1);

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(2);

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(3);

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(0);
    expect(player.getLoopCount()).toBe(1);

    raf.advance(400, 100);
    expect(player.getLoopCount()).toBe(2);
  });

  it("once 播放类型播放完成后触发完成回调并停止", () => {
    const action = makeRuntimeAction({
      key: "wave",
      loopType: "once",
      frameCount: 3,
      frameDurationMs: 100,
      fps: 10,
    });

    player.play(action);
    expect(player.getState()).toBe("playing");

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(1);

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(2);

    raf.nextTick(100);
    expect(player.getState()).toBe("stopped");
    expect(player.getLoopCount()).toBe(1);
    expect(callbacks.completes).toEqual([{ key: "wave", loop: 1 }]);
  });

  it("hold 播放类型播到最后一帧后保持", () => {
    const action = makeRuntimeAction({
      key: "happy_hold",
      loopType: "hold",
      frameCount: 3,
      frameDurationMs: 100,
      fps: 10,
    });

    player.play(action);
    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(1);

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(2);

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(2);
    expect(player.getState()).toBe("paused");
    expect(player.getLoopCount()).toBe(1);
    expect(callbacks.completes).toHaveLength(1);
  });

  it("时间差推进基于真实时间差，不依赖固定步长", () => {
    const action = makeRuntimeAction({
      key: "timed",
      loopType: "loop",
      frameCount: 4,
      frameDurationMs: 50,
      fps: 20,
    });

    player.play(action);
    expect(player.getCurrentFrameIndex()).toBe(0);

    raf.nextTick(120);
    expect(player.getCurrentFrameIndex()).toBe(2);

    raf.nextTick(30);
    expect(player.getCurrentFrameIndex()).toBe(3);

    raf.nextTick(50);
    expect(player.getCurrentFrameIndex()).toBe(0);
    expect(player.getLoopCount()).toBe(1);
  });

  it("时间差极大时仍能限制单 tick 帧追赶并保留余数", () => {
    const action = makeRuntimeAction({
      key: "fast",
      loopType: "loop",
      frameCount: 4,
      frameDurationMs: 16,
      fps: 60,
    });

    player.play(action);
    raf.nextTick(2000);

    expect(player.getState()).toBe("playing");
    expect(player.getLoopCount()).toBeGreaterThan(0);
  });

  it("回退链优先返回 returnAction", () => {
    const loaded = makeLoadedInstallation();
    const actionWithReturn = makeRuntimeAction({
      key: "happy",
      loopType: "once",
      returnAction: "idle",
    });
    loaded.actions.set("happy", actionWithReturn);
    player.attachLoaded(loaded);

    const chain = player.getFallbackChain(actionWithReturn, loaded);

    expect(chain[0]?.key).toBe("idle");
    expect(chain.length).toBeGreaterThan(0);
  });

  it("回退链会包含持续动作映射中的项", () => {
    const loaded = makeLoadedInstallation();
    const thinking = makeRuntimeAction({
      key: "thinking",
      loopType: "loop",
    });
    const waiting = makeRuntimeAction({
      key: "waiting",
      loopType: "loop",
    });
    loaded.actions.set("thinking", thinking);
    loaded.actions.set("waiting", waiting);
    player.attachLoaded(loaded);
    player.setSustainedActionMap({ thinking: "waiting" });

    const chain = player.getFallbackChain(thinking, loaded);

    expect(chain[0]?.key).toBe("waiting");
  });

  it("回退链最后回退到默认待机动作", () => {
    const loaded = makeLoadedInstallation();
    const lonely = makeRuntimeAction({
      key: "lonely",
      loopType: "loop",
    });
    loaded.actions.set("lonely", lonely);
    player.attachLoaded(loaded);

    const chain = player.getFallbackChain(lonely, loaded);

    expect(chain).toContainEqual(
      expect.objectContaining({ key: "idle" }),
    );
  });

  it("回退链最终回退到任意可用待机动作", () => {
    const loaded = makeLoadedInstallation({
      defaultAction: null,
    });
    const lonely = makeRuntimeAction({
      key: "lonely",
      loopType: "loop",
    });
    loaded.actions.set("lonely", lonely);
    player.attachLoaded(loaded);

    const chain = player.getFallbackChain(lonely, loaded);

    expect(chain.length).toBeGreaterThan(0);
    expect(chain[0].available).toBe(true);
  });

  it("loaded 为空时回退链为空数组（最终由调用方走静态首帧兜底）", () => {
    const action = makeRuntimeAction({ key: "alone" });
    const chain = player.getFallbackChain(action, null);

    expect(chain).toEqual([]);
  });

  it("暂停会停止帧推进，恢复后继续", () => {
    const action = makeRuntimeAction({
      key: "idle",
      loopType: "loop",
      frameCount: 4,
      frameDurationMs: 100,
    });

    player.play(action);
    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(1);

    player.pause();
    expect(player.getState()).toBe("paused");

    raf.nextTick(500);
    expect(player.getCurrentFrameIndex()).toBe(1);

    player.resume();
    expect(player.getState()).toBe("playing");

    raf.nextTick(100);
    expect(player.getCurrentFrameIndex()).toBe(2);
  });

  it("停止会清空状态并阻止后续推进", () => {
    const action = makeRuntimeAction({
      key: "idle",
      loopType: "loop",
      frameCount: 4,
      frameDurationMs: 100,
    });

    player.play(action);
    raf.nextTick(100);

    player.stop();
    expect(player.getState()).toBe("stopped");
    expect(player.getCurrentFrameIndex()).toBe(0);
    expect(player.getLoopCount()).toBe(0);

    raf.nextTick(500);
    expect(player.getCurrentFrameIndex()).toBe(0);
  });

  it("switchAction 触发 onActionSwitch 回调并播放新动作", () => {
    const idle = makeRuntimeAction({ key: "idle", loopType: "loop" });
    const wave = makeRuntimeAction({ key: "wave", loopType: "once" });

    player.play(idle);
    callbacks.switches.length = 0;

    player.switchAction(wave);

    expect(callbacks.switches).toEqual([
      { newKey: "wave", oldKey: "idle" },
    ]);
    expect(player.getCurrentAction()?.key).toBe("wave");
  });

  it("switchAction 切换到相同动作不触发 onActionSwitch", () => {
    const idle = makeRuntimeAction({ key: "idle", loopType: "loop" });

    player.play(idle);
    callbacks.switches.length = 0;

    player.switchAction(idle);

    expect(callbacks.switches).toHaveLength(0);
  });

  it("切换动作时立即从首帧开始，不出现空帧或闪白", () => {
    const idle = makeRuntimeAction({
      key: "idle",
      loopType: "loop",
      frameCount: 4,
      frameDurationMs: 100,
    });
    const wave = makeRuntimeAction({
      key: "wave",
      loopType: "once",
      frameCount: 3,
      frameDurationMs: 100,
    });

    player.play(idle);
    raf.nextTick(200);
    expect(player.getCurrentFrameIndex()).toBe(2);

    callbacks.frames.length = 0;
    player.switchAction(wave);

    expect(player.getCurrentFrameIndex()).toBe(0);
    expect(player.getCurrentAction()?.key).toBe("wave");
    if (callbacks.frames.length > 0) {
      expect(callbacks.frames[0]).toEqual({ key: "wave", frame: 0 });
    }
  });

  it("播放不可用动作时上报错误且不进入播放状态", () => {
    const broken = makeRuntimeAction({
      key: "broken",
      available: false,
      loadError: "FRAME_MISSING",
    });

    player.play(broken);

    expect(player.getState()).toBe("idle");
    expect(callbacks.errors.length).toBeGreaterThan(0);
    expect(callbacks.errors[0].message).toContain("ACTION_UNAVAILABLE");
  });

  it("播放帧数或帧时长非正的动作时上报错误", () => {
    const invalid = makeRuntimeAction({
      key: "invalid",
      frameCount: 0,
      frameDurationMs: 0,
    });

    player.play(invalid);

    expect(player.getState()).toBe("idle");
    expect(callbacks.errors.length).toBeGreaterThan(0);
    expect(callbacks.errors[0].message).toContain("ACTION_INVALID_FRAMES");
  });

  it("onFrameChange 回调异常被捕获为 onError，不中断播放", () => {
    const boom = new Error("boom");
    const faulty: PlayerCallbacks = {
      onFrameChange: () => {
        throw boom;
      },
      onError: vi.fn(),
    };
    const p = new ActionPlayer(faulty);
    const action = makeRuntimeAction({
      key: "idle",
      loopType: "loop",
      frameCount: 2,
      frameDurationMs: 50,
    });

    p.play(action);
    raf.nextTick(50);

    expect(faulty.onError).toHaveBeenCalledWith(boom);
    p.stop();
  });

  it("空动作入参上报 ACTION_REQUIRED 错误", () => {
    player.play(null as unknown as RuntimeAction);

    expect(callbacks.errors.length).toBeGreaterThan(0);
    expect(callbacks.errors[0].message).toBe("ACTION_REQUIRED");
  });

  it("attachLoaded 与 detachLoaded 不抛错", () => {
    const loaded = makeLoadedInstallation();
    expect(() => player.attachLoaded(loaded)).not.toThrow();
    expect(() => player.detachLoaded()).not.toThrow();
  });
});
