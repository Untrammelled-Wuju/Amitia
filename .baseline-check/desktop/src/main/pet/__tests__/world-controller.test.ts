import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("electron", () => ({
  screen: {
    getDisplayNearestPoint: vi.fn(() => ({
      id: 1,
      workArea: { x: 0, y: 0, width: 1200, height: 800 },
    })),
  },
}));

import { screen } from "electron";

import { ActionPriorities } from "../action-scheduler";
import { DesktopPetWorldController } from "../world-controller";

function makeWindow(x = 100, y = 100) {
  let position: [number, number] = [x, y];
  const win = {
    isDestroyed: vi.fn(() => false),
    isVisible: vi.fn(() => true),
    getPosition: vi.fn(() => [...position] as [number, number]),
    getSize: vi.fn(() => [100, 100] as [number, number]),
    setPosition: vi.fn((nextX: number, nextY: number) => {
      position = [nextX, nextY];
    }),
  };
  return { win, position: () => [...position] as [number, number] };
}

describe("DesktopPetWorldController", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("does not move the native window until renderer playback actually starts", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 12, 0, 0));
    vi.spyOn(Math, "random").mockReturnValue(0);

    const { win, position } = makeWindow(300, 100);
    let submitted = false;
    let playbackStarted = false;
    const scheduler = {
      getCurrent: vi.fn(() => submitted
        ? { actionKey: "walk_left", priority: ActionPriorities.RANDOM_IDLE }
        : null),
      submit: vi.fn(() => {
        submitted = true;
        return "played";
      }),
      isCurrentPlaybackStarted: vi.fn(() => playbackStarted),
    };
    const adapter = { getNativeWindow: vi.fn(() => win) };
    const controller = new DesktopPetWorldController(
      scheduler as never,
      adapter as never,
    );
    controller.start();

    vi.advanceTimersByTime(18_250);
    expect(scheduler.submit).toHaveBeenCalled();
    expect(position()[0]).toBe(300);

    playbackStarted = true;
    vi.advanceTimersByTime(120);
    expect(position()[0]).toBeLessThan(300);
    controller.dispose();
  });

  it("falls to the work-area floor and emits land after drop", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 12, 0, 0));

    const { win, position } = makeWindow(300, 100);
    const scheduler = {
      getCurrent: vi.fn(() => null),
      submit: vi.fn(() => "played"),
      isCurrentPlaybackStarted: vi.fn(() => false),
    };
    const adapter = { getNativeWindow: vi.fn(() => win) };
    const controller = new DesktopPetWorldController(
      scheduler as never,
      adapter as never,
    );

    controller.onDrop();
    vi.advanceTimersByTime(3000);

    expect(position()[1]).toBe(696);
    expect(scheduler.submit).toHaveBeenCalledWith(
      expect.objectContaining({ actionKey: "fall" }),
    );
    expect(scheduler.submit).toHaveBeenCalledWith(
      expect.objectContaining({ actionKey: "land" }),
    );
    controller.dispose();
  });

  it("recomputes the floor while falling when the nearest display work area changes", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 12, 0, 0));

    const getDisplayNearestPoint = vi.mocked(screen.getDisplayNearestPoint);
    getDisplayNearestPoint.mockReturnValue({
      id: 1,
      workArea: { x: 0, y: 0, width: 1200, height: 800 },
    } as never);

    const { win, position } = makeWindow(300, 100);
    const scheduler = {
      getCurrent: vi.fn(() => null),
      submit: vi.fn(() => "played"),
      isCurrentPlaybackStarted: vi.fn(() => false),
    };
    const adapter = { getNativeWindow: vi.fn(() => win) };
    const controller = new DesktopPetWorldController(
      scheduler as never,
      adapter as never,
    );

    controller.onDrop();
    vi.advanceTimersByTime(200);
    getDisplayNearestPoint.mockReturnValue({
      id: 2,
      workArea: { x: 0, y: 0, width: 1200, height: 500 },
    } as never);
    vi.advanceTimersByTime(3000);

    expect(position()[1]).toBe(396);
    expect(scheduler.submit).toHaveBeenCalledWith(
      expect.objectContaining({ actionKey: "land" }),
    );
    controller.dispose();
  });

  it("settles a restored airborne position on startup and reports final coordinates", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 12, 0, 0));

    const { win, position } = makeWindow(300, 100);
    const scheduler = {
      getCurrent: vi.fn(() => null),
      submit: vi.fn(() => "played"),
      isCurrentPlaybackStarted: vi.fn(() => false),
    };
    const adapter = { getNativeWindow: vi.fn(() => win) };
    const settled = vi.fn();
    const controller = new DesktopPetWorldController(
      scheduler as never,
      adapter as never,
      settled,
    );

    controller.start();
    vi.advanceTimersByTime(3000);

    expect(position()[1]).toBe(696);
    expect(settled).toHaveBeenCalled();
    controller.dispose();
  });

  it("clears world and movement timers when stopped", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 31, 12, 0, 0));
    vi.spyOn(Math, "random").mockReturnValue(0);

    const { win } = makeWindow(300, 100);
    const scheduler = {
      getCurrent: vi.fn(() => null),
      submit: vi.fn(() => "played"),
      isCurrentPlaybackStarted: vi.fn(() => false),
    };
    const adapter = { getNativeWindow: vi.fn(() => win) };
    const controller = new DesktopPetWorldController(
      scheduler as never,
      adapter as never,
    );
    controller.start();
    vi.advanceTimersByTime(18_100);
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    controller.stop();
    expect(vi.getTimerCount()).toBe(0);
  });
});
