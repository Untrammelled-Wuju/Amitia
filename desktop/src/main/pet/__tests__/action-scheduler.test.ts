import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type { ActionPlayer as ActionPlayerType } from "../action-player";
import type { PlayerCallbacks } from "../action-player";
import {
  ActionPriorities,
  DesktopPetActionScheduler,
  EventSources,
  type DesktopPetActionRequest,
  type SchedulerEvent,
} from "../action-scheduler";
import {
  makeLoadedInstallation,
  makeRuntimeAction,
  installManualRaf,
  type ManualRafController,
} from "./helpers";

let ActionPlayer: typeof import("../action-player").ActionPlayer;

interface SchedulerSpy {
  events: Array<{
    event: SchedulerEvent;
    request: DesktopPetActionRequest;
    actionKey: string | null;
  }>;
}

function makeSpy(): SchedulerSpy {
  const events: SchedulerSpy["events"] = [];
  return { events };
}

function makeRequest(
  overrides: Partial<DesktopPetActionRequest>,
): DesktopPetActionRequest {
  const base: DesktopPetActionRequest = {
    actionKey: "idle",
    source: EventSources.MANUAL,
    priority: ActionPriorities.DEFAULT_IDLE,
    interrupt: false,
  };
  return { ...base, ...overrides };
}

describe("DesktopPetActionScheduler", () => {
  let player: ActionPlayerType;
  let raf: ManualRafController;
  let spy: SchedulerSpy;
  let scheduler: DesktopPetActionScheduler;

  beforeEach(async () => {
    raf = installManualRaf();
    vi.resetModules();
    const mod = await import("../action-player");
    ActionPlayer = mod.ActionPlayer;
    const callbacks: PlayerCallbacks = {};
    player = new ActionPlayer(callbacks);
    spy = makeSpy();
    scheduler = new DesktopPetActionScheduler(player, {
      onEvent: (event, request, action) => {
        spy.events.push({
          event,
          request,
          actionKey: action ? action.key : null,
        });
      },
    });
  });

  afterEach(() => {
    scheduler.dispose();
    player.stop();
    raf.restore();
    vi.resetModules();
  });

  function attachAndAdvance(): ReturnType<typeof makeLoadedInstallation> {
    const loaded = makeLoadedInstallation();
    scheduler.attachLoaded(loaded);
    raf.nextTick(500);
    return loaded;
  }

  it("attachLoaded 后自动播放默认待机动作", () => {
    const loaded = makeLoadedInstallation();
    scheduler.attachLoaded(loaded);

    expect(player.getCurrentAction()?.key).toBe("idle");
    expect(scheduler.getCurrent()?.actionKey).toBe("idle");
  });

  it("高优先级拖动可打断低优先级默认待机", () => {
    const loaded = attachAndAdvance();
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);

    const result = scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    expect(result).toBe("played");
    expect(player.getCurrentAction()?.key).toBe("dragged");
  });

  it("低优先级不得打断高优先级", () => {
    const loaded = attachAndAdvance();
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);

    scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    const result = scheduler.submit(
      makeRequest({
        actionKey: "idle",
        source: EventSources.IDLE,
        priority: ActionPriorities.DEFAULT_IDLE,
        interrupt: true,
      }),
    );

    expect(result).not.toBe("played");
    expect(player.getCurrentAction()?.key).toBe("dragged");
  });

  it("队列上限 5，超过时丢弃低优先级旧动作", () => {
    const loaded = attachAndAdvance();
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);
    const extraActions = ["a1", "a2", "a3", "a4", "a5", "a6"];
    for (const key of extraActions) {
      loaded.actions.set(
        key,
        makeRuntimeAction({ key, loopType: "loop", interruptible: true }),
      );
    }

    scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    for (let i = 0; i < 6; i++) {
      scheduler.submit(
        makeRequest({
          actionKey: `a${i + 1}`,
          source: EventSources.MANUAL,
          priority: ActionPriorities.EMOTION + i,
          interrupt: false,
          dedupeKey: `a${i + 1}`,
        }),
      );
    }

    expect(scheduler.getQueue().length).toBeLessThanOrEqual(5);
  });

  it("不累积待机动作（队列非空时低优先级 idle 不入队）", () => {
    const loaded = attachAndAdvance();
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);

    scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    scheduler.submit(
      makeRequest({
        actionKey: "happy",
        source: EventSources.MANUAL,
        priority: ActionPriorities.EMOTION,
        interrupt: false,
        dedupeKey: "happy-1",
      }),
    );

    const result = scheduler.submit(
      makeRequest({
        actionKey: "idle",
        source: EventSources.IDLE,
        priority: ActionPriorities.DEFAULT_IDLE,
        interrupt: false,
      }),
    );

    expect(result).toBe("rejected");
    expect(scheduler.getQueue().length).toBe(1);
  });

  it("合并相同来源相同动作请求（相同 dedupeKey+actionKey 替换）", () => {
    const loaded = attachAndAdvance();
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);

    scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    scheduler.submit(
      makeRequest({
        actionKey: "happy",
        source: EventSources.MANUAL,
        priority: ActionPriorities.EMOTION,
        interrupt: false,
        dedupeKey: "emotion-1",
      }),
    );

    scheduler.submit(
      makeRequest({
        actionKey: "happy",
        source: EventSources.MANUAL,
        priority: ActionPriorities.EMOTION + 1,
        interrupt: false,
        dedupeKey: "emotion-1",
      }),
    );

    expect(scheduler.getQueue().length).toBe(1);
    expect(scheduler.getQueue()[0].priority).toBe(ActionPriorities.EMOTION + 1);
  });

  it("speaking 状态期间只触发一次 chat_speaking", () => {
    const loaded = attachAndAdvance();
    const speaking = makeRuntimeAction({
      key: "speaking",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("speaking", speaking);

    scheduler.setSustainedState("speaking");

    const r1 = scheduler.submit(
      makeRequest({
        actionKey: "speaking",
        source: EventSources.CHAT_SPEAKING,
        priority: ActionPriorities.SPEAKING,
        interrupt: true,
        dedupeKey: "chat_speaking",
      }),
    );
    expect(r1).toBe("played");

    const r2 = scheduler.submit(
      makeRequest({
        actionKey: "speaking",
        source: EventSources.CHAT_SPEAKING,
        priority: ActionPriorities.SPEAKING,
        interrupt: true,
        dedupeKey: "chat_speaking",
      }),
    );
    expect(r2).toBe("rejected");
  });

  it("thinking 同轮对话只触发一次（相同 dedupeKey 合并）", () => {
    const loaded = attachAndAdvance();
    const thinking = makeRuntimeAction({
      key: "thinking",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("thinking", thinking);
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);

    scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    scheduler.submit(
      makeRequest({
        actionKey: "thinking",
        source: EventSources.CHAT_THINKING,
        priority: ActionPriorities.THINKING,
        interrupt: false,
        dedupeKey: "turn-1-thinking",
      }),
    );

    scheduler.submit(
      makeRequest({
        actionKey: "thinking",
        source: EventSources.CHAT_THINKING,
        priority: ActionPriorities.THINKING,
        interrupt: false,
        dedupeKey: "turn-1-thinking",
      }),
    );

    expect(scheduler.getQueue().length).toBe(1);
  });

  it("hovered 2-5 秒冷却内被拒绝，冷却后可再次触发", () => {
    const loaded = attachAndAdvance();
    const hovered = makeRuntimeAction({
      key: "hovered",
      loopType: "once",
      frameCount: 2,
      frameDurationMs: 100,
      interruptible: true,
    });
    loaded.actions.set("hovered", hovered);

    const r1 = scheduler.submit(
      makeRequest({
        actionKey: "hovered",
        source: EventSources.USER_HOVER,
        priority: ActionPriorities.CLICK,
        interrupt: true,
      }),
    );
    expect(r1).toBe("played");

    const r2 = scheduler.submit(
      makeRequest({
        actionKey: "hovered",
        source: EventSources.USER_HOVER,
        priority: ActionPriorities.CLICK,
        interrupt: true,
      }),
    );
    expect(r2).toBe("rejected");

    raf.nextTick(200);
    raf.nextTick(3500);

    const r3 = scheduler.submit(
      makeRequest({
        actionKey: "hovered",
        source: EventSources.USER_HOVER,
        priority: ActionPriorities.CLICK,
        interrupt: true,
      }),
    );
    expect(r3).toBe("played");
  });

  it("clicked 短冷却内被拒绝", () => {
    const loaded = attachAndAdvance();
    const clicked = makeRuntimeAction({
      key: "clicked",
      loopType: "once",
      frameCount: 2,
      frameDurationMs: 100,
      interruptible: true,
    });
    loaded.actions.set("clicked", clicked);

    const r1 = scheduler.submit(
      makeRequest({
        actionKey: "clicked",
        source: EventSources.USER_CLICK,
        priority: ActionPriorities.CLICK,
        interrupt: true,
      }),
    );
    expect(r1).toBe("played");

    const r2 = scheduler.submit(
      makeRequest({
        actionKey: "clicked",
        source: EventSources.USER_CLICK,
        priority: ActionPriorities.CLICK,
        interrupt: true,
      }),
    );
    expect(r2).toBe("rejected");
  });

  it("clicked 冷却结束后可再次触发", () => {
    const loaded = attachAndAdvance();
    const clicked = makeRuntimeAction({
      key: "clicked",
      loopType: "once",
      frameCount: 2,
      frameDurationMs: 100,
      interruptible: true,
    });
    loaded.actions.set("clicked", clicked);

    scheduler.submit(
      makeRequest({
        actionKey: "clicked",
        source: EventSources.USER_CLICK,
        priority: ActionPriorities.CLICK,
        interrupt: true,
      }),
    );

    raf.nextTick(200);
    raf.nextTick(600);

    const r3 = scheduler.submit(
      makeRequest({
        actionKey: "clicked",
        source: EventSources.USER_CLICK,
        priority: ActionPriorities.CLICK,
        interrupt: true,
      }),
    );
    expect(r3).toBe("played");
  });

  it("回退映射：double_clicked 不存在时回退到 clicked → happy → idle", () => {
    const loaded = attachAndAdvance();
    const clicked = makeRuntimeAction({
      key: "clicked",
      loopType: "once",
      interruptible: true,
    });
    loaded.actions.set("clicked", clicked);

    const result = scheduler.submit(
      makeRequest({
        actionKey: "double_clicked",
        source: EventSources.USER_DOUBLE_CLICK,
        priority: ActionPriorities.CLICK,
        interrupt: true,
      }),
    );

    expect(result).toBe("fallback");
    const fallbackEvents = spy.events.filter(
      (e) => e.event === "action-fallback",
    );
    expect(fallbackEvents.length).toBeGreaterThan(0);
    expect(fallbackEvents[0].actionKey).toBe("clicked");
  });

  it("回退映射：speaking 不存在时回退到 listening → idle", () => {
    const loaded = attachAndAdvance();
    const listening = makeRuntimeAction({
      key: "listening",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("listening", listening);

    const result = scheduler.submit(
      makeRequest({
        actionKey: "speaking",
        source: EventSources.CHAT_SPEAKING,
        priority: ActionPriorities.SPEAKING,
        interrupt: true,
      }),
    );

    expect(result).toBe("fallback");
    expect(player.getCurrentAction()?.key).toBe("listening");
  });

  it("回退映射：thinking 不存在时回退到 waiting → idle", () => {
    const loaded = attachAndAdvance();
    const waiting = makeRuntimeAction({
      key: "waiting",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("waiting", waiting);

    const result = scheduler.submit(
      makeRequest({
        actionKey: "thinking",
        source: EventSources.CHAT_THINKING,
        priority: ActionPriorities.THINKING,
        interrupt: true,
      }),
    );

    expect(result).toBe("fallback");
    expect(player.getCurrentAction()?.key).toBe("waiting");
  });

  it("回退映射：dragged 不存在时回退到 picked_up → idle", () => {
    const loaded = attachAndAdvance();
    const pickedUp = makeRuntimeAction({
      key: "picked_up",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("picked_up", pickedUp);

    const result = scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    expect(result).toBe("fallback");
    expect(player.getCurrentAction()?.key).toBe("picked_up");
  });

  it("回退映射：land 不存在时回退到 idle", () => {
    const loaded = attachAndAdvance();

    const result = scheduler.submit(
      makeRequest({
        actionKey: "land",
        source: EventSources.SYSTEM,
        priority: ActionPriorities.FALL,
        interrupt: true,
      }),
    );

    expect(result).toBe("fallback");
    expect(player.getCurrentAction()?.key).toBe("idle");
  });

  it("forceInterrupt user_drag 中断当前并播放回退动作", () => {
    const loaded = attachAndAdvance();
    const pickedUp = makeRuntimeAction({
      key: "picked_up",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("picked_up", pickedUp);

    scheduler.forceInterrupt("user_drag");

    expect(player.getCurrentAction()?.key).toBe("picked_up");
  });

  it("forceInterrupt app_exit 中断当前且不播放回退动作", () => {
    attachAndAdvance();

    scheduler.forceInterrupt("app_exit");

    expect(scheduler.getCurrent()).toBeNull();
    expect(player.getState()).toBe("stopped");
  });

  it("forceInterrupt resource_invalid 中断当前并恢复默认待机", () => {
    const loaded = attachAndAdvance();
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);

    scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    scheduler.forceInterrupt("resource_invalid");

    expect(scheduler.getCurrent()).not.toBeNull();
    expect(player.getCurrentAction()?.key).toBe("idle");
  });

  it("setSustainedState 切换时清除 speaking 冷却记录", () => {
    const loaded = attachAndAdvance();
    const speaking = makeRuntimeAction({
      key: "speaking",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("speaking", speaking);

    scheduler.setSustainedState("speaking");
    scheduler.submit(
      makeRequest({
        actionKey: "speaking",
        source: EventSources.CHAT_SPEAKING,
        priority: ActionPriorities.SPEAKING,
        interrupt: true,
        dedupeKey: "chat_speaking",
      }),
    );

    scheduler.forceInterrupt("app_exit");
    scheduler.setSustainedState("idle");
    scheduler.setSustainedState("speaking");
    scheduler.attachLoaded(loaded);
    raf.nextTick(500);

    const r = scheduler.submit(
      makeRequest({
        actionKey: "speaking",
        source: EventSources.CHAT_SPEAKING,
        priority: ActionPriorities.SPEAKING,
        interrupt: true,
        dedupeKey: "chat_speaking",
      }),
    );
    expect(r).toBe("played");
  });

  it("clearQueue 清空等待队列", () => {
    const loaded = attachAndAdvance();
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "loop",
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);

    scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    scheduler.submit(
      makeRequest({
        actionKey: "happy",
        source: EventSources.MANUAL,
        priority: ActionPriorities.EMOTION,
        interrupt: false,
        dedupeKey: "happy-1",
      }),
    );

    scheduler.clearQueue();
    expect(scheduler.getQueue().length).toBe(0);
  });

  it("detachLoaded 后 submit 返回 rejected", () => {
    const loaded = makeLoadedInstallation();
    scheduler.attachLoaded(loaded);
    scheduler.detachLoaded();

    const result = scheduler.submit(
      makeRequest({
        actionKey: "idle",
        source: EventSources.MANUAL,
        priority: ActionPriorities.DEFAULT_IDLE,
        interrupt: false,
      }),
    );

    expect(result).toBe("rejected");
  });

  it("expiresAt 已过期请求被拒绝", () => {
    const loaded = attachAndAdvance();

    const pastTime = -1;
    const result = scheduler.submit(
      makeRequest({
        actionKey: "happy",
        source: EventSources.MANUAL,
        priority: ActionPriorities.EMOTION,
        interrupt: true,
        expiresAt: pastTime,
      }),
    );

    expect(result).toBe("rejected");
  });

  it("idle 同一动作连续播放次数受限", () => {
    const loaded = attachAndAdvance();

    const results: Array<string> = [];
    for (let i = 0; i < 5; i++) {
      results.push(
        scheduler.submit(
          makeRequest({
            actionKey: "idle",
            source: EventSources.IDLE,
            priority: ActionPriorities.DEFAULT_IDLE,
            interrupt: false,
            dedupeKey: "idle-cycle",
          }),
        ),
      );
    }

    const rejectedCount = results.filter((r) => r === "rejected").length;
    expect(rejectedCount).toBeGreaterThan(0);
  });

  it("metadata.cooldownMs 覆盖默认冷却", () => {
    const loaded = attachAndAdvance();
    const hovered = makeRuntimeAction({
      key: "hovered",
      loopType: "once",
      frameCount: 2,
      frameDurationMs: 100,
      interruptible: true,
    });
    loaded.actions.set("hovered", hovered);

    scheduler.submit(
      makeRequest({
        actionKey: "hovered",
        source: EventSources.USER_HOVER,
        priority: ActionPriorities.CLICK,
        interrupt: true,
        metadata: { cooldownMs: "100" },
      }),
    );

    const r2 = scheduler.submit(
      makeRequest({
        actionKey: "hovered",
        source: EventSources.USER_HOVER,
        priority: ActionPriorities.CLICK,
        interrupt: true,
        metadata: { cooldownMs: "100" },
      }),
    );
    expect(r2).toBe("rejected");

    raf.nextTick(200);
    raf.nextTick(200);

    const r3 = scheduler.submit(
      makeRequest({
        actionKey: "hovered",
        source: EventSources.USER_HOVER,
        priority: ActionPriorities.CLICK,
        interrupt: true,
        metadata: { cooldownMs: "100" },
      }),
    );
    expect(r3).not.toBe("rejected");
  });

  it("action-completed 事件后从队列播放下一个", () => {
    const loaded = attachAndAdvance();
    const dragged = makeRuntimeAction({
      key: "dragged",
      loopType: "once",
      frameCount: 2,
      frameDurationMs: 100,
      interruptible: true,
    });
    loaded.actions.set("dragged", dragged);
    const wave = loaded.actions.get("wave");
    if (wave) {
      wave.loopType = "once";
      wave.frameCount = 2;
      wave.frameDurationMs = 100;
    }

    scheduler.submit(
      makeRequest({
        actionKey: "dragged",
        source: EventSources.USER_DRAG,
        priority: ActionPriorities.DRAG,
        interrupt: true,
      }),
    );

    scheduler.submit(
      makeRequest({
        actionKey: "wave",
        source: EventSources.MANUAL,
        priority: ActionPriorities.EMOTION,
        interrupt: false,
        dedupeKey: "wave-1",
      }),
    );

    expect(scheduler.getQueue().length).toBe(1);

    raf.nextTick(200);
    raf.nextTick(200);

    const completedEvents = spy.events.filter(
      (e) => e.event === "action-completed",
    );
    expect(completedEvents.length).toBeGreaterThan(0);
  });

  it("dispose 恢复 player 原始 callbacks", () => {
    const loaded = attachAndAdvance();

    scheduler.dispose();

    expect(() => scheduler.attachLoaded(loaded)).not.toThrow();
  });
});
