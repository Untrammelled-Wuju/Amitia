import { afterEach, describe, expect, it, vi } from "vitest";
import { DesktopPetVitalityController } from "../vitality-controller";

function makeScheduler() {
  return {
    getCurrent: vi.fn(() => null),
    submit: vi.fn(() => "played"),
  };
}

describe("DesktopPetVitalityController", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("does not decay daytime idle energy into permanent tired state", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 12, 0, 0));
    const scheduler = makeScheduler();
    const controller = new DesktopPetVitalityController(scheduler as never);
    controller.start();

    vi.advanceTimersByTime(60 * 60 * 1000);

    const snapshot = controller.snapshot();
    expect(snapshot.energy).toBeGreaterThanOrEqual(0.8);
    expect(snapshot.activity).toBe("idle");
    controller.dispose();
  });

  it("enters night rest, recovers energy, and wakes when daylight returns", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 1, 0, 0));
    const scheduler = makeScheduler();
    const controller = new DesktopPetVitalityController(scheduler as never);
    controller.start();

    // 130 idle ticks drain 0.8 -> roughly 0.28 and enter resting.
    vi.advanceTimersByTime(130 * 5000);
    expect(controller.snapshot().activity).toBe("resting");

    // Rest regenerates energy but remains a stable night state instead of
    // oscillating between idle/rest every few minutes.
    vi.advanceTimersByTime(45 * 5000);
    const recovered = controller.snapshot();
    expect(recovered.energy).toBeGreaterThanOrEqual(0.95);
    expect(recovered.activity).toBe("resting");

    vi.setSystemTime(new Date(2026, 7, 31, 8, 0, 0));
    vi.advanceTimersByTime(5000);
    expect(controller.snapshot().activity).toBe("idle");
    controller.dispose();
  });

  it("wakes resting state immediately on real user interaction", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 1, 0, 0));
    const scheduler = makeScheduler();
    const controller = new DesktopPetVitalityController(scheduler as never);
    controller.start();

    vi.advanceTimersByTime(130 * 5000);
    expect(controller.snapshot().activity).toBe("resting");

    controller.notifyInteraction("click");
    expect(controller.snapshot().activity).toBe("attentive");
    controller.dispose();
  });

  it("does not inject autonomous idle actions while tool work owns attention", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 12, 0, 0));
    const scheduler = makeScheduler();
    const controller = new DesktopPetVitalityController(scheduler as never);
    controller.start();
    controller.notifyInteraction("tool");

    vi.advanceTimersByTime(45 * 1000);

    expect(controller.snapshot().activity).toBe("working");
    expect(scheduler.submit).not.toHaveBeenCalled();
    controller.dispose();
  });
});
