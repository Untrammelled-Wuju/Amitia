import { afterEach, describe, expect, it, vi } from "vitest";
import { IdleController } from "../idle-controller";

function makeLoaded() {
  const idle = {
    key: "idle",
    available: true,
    loopType: "loop",
    supportsDefaultIdle: true,
    category: "idle",
  };
  return {
    defaultAction: idle,
    actions: new Map([["idle", idle]]),
  };
}

describe("IdleController", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("applies new enabled state immediately while running", () => {
    vi.useFakeTimers();
    const scheduler = { submit: vi.fn(() => "accepted") };
    const controller = new IdleController(scheduler as never, {
      enabled: true,
      minIntervalSeconds: 30,
      maxIntervalSeconds: 30,
    });
    controller.attachLoaded(makeLoaded() as never);
    controller.start();

    expect(vi.getTimerCount()).toBe(1);
    controller.updateConfig({ enabled: false });
    expect(vi.getTimerCount()).toBe(0);
  });

  it("reschedules using the updated interval", () => {
    vi.useFakeTimers();
    const scheduler = { submit: vi.fn(() => "accepted") };
    const controller = new IdleController(scheduler as never, {
      enabled: true,
      minIntervalSeconds: 30,
      maxIntervalSeconds: 30,
    });
    controller.attachLoaded(makeLoaded() as never);
    controller.start();
    controller.updateConfig({ minIntervalSeconds: 5, maxIntervalSeconds: 5 });

    vi.advanceTimersByTime(5000);
    expect(scheduler.submit).toHaveBeenCalledTimes(3);
  });
});
