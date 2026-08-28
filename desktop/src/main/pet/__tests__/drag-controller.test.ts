import { beforeEach, describe, expect, it, vi } from "vitest";

const electronState = vi.hoisted(() => ({
  display: {
    id: 1,
    bounds: { x: 0, y: 0, width: 1920, height: 1080 },
    workArea: { x: 0, y: 0, width: 1920, height: 1040 },
    scaleFactor: 1,
    label: "Primary",
  },
}));

vi.mock("electron", () => ({
  BrowserWindow: class {},
  screen: {
    getDisplayNearestPoint: vi.fn(() => electronState.display),
    getPrimaryDisplay: vi.fn(() => electronState.display),
  },
}));

import { DragController } from "../drag-controller";
import type { PetDragIpcPayload } from "../../../shared/animation-ipc";

function payload(screenX: number, screenY: number): PetDragIpcPayload {
  return {
    pointerId: 1,
    screenX,
    screenY,
    canvasX: 0,
    canvasY: 0,
    occurredAt: Date.now(),
  };
}

describe("DragController", () => {
  let adapter: {
    setPosition: ReturnType<typeof vi.fn>;
    getDpiScale: ReturnType<typeof vi.fn>;
    snapshotRuntimePosition: ReturnType<typeof vi.fn>;
  };
  let clickThrough: { setDragging: ReturnType<typeof vi.fn> };
  let win: { isDestroyed: ReturnType<typeof vi.fn>; getPosition: ReturnType<typeof vi.fn> };

  beforeEach(() => {
    adapter = {
      setPosition: vi.fn(async () => undefined),
      getDpiScale: vi.fn(() => 1),
      snapshotRuntimePosition: vi.fn(() => ({ screenId: "1", x: 0, y: 0, scale: 1 })),
    };
    clickThrough = { setDragging: vi.fn() };
    win = {
      isDestroyed: vi.fn(() => false),
      getPosition: vi.fn(() => [100, 200]),
    };
  });

  it("keeps the original grab offset while dragging", () => {
    const controller = new DragController(
      adapter as never,
      clickThrough as never,
      vi.fn(),
    );
    controller.attach(win as never);

    controller.handleDragStart(payload(130, 245));
    controller.handleDragMove(payload(230, 345));

    expect(adapter.setPosition).toHaveBeenLastCalledWith(200, 300, "1");
    expect(clickThrough.setDragging).toHaveBeenCalledWith(true);
  });

  it("keeps the grab offset on drag end and releases click-through lock", () => {
    const controller = new DragController(
      adapter as never,
      clickThrough as never,
      vi.fn(),
    );
    controller.attach(win as never);

    controller.handleDragStart(payload(120, 220));
    controller.handleDragEnd(payload(320, 420));

    expect(adapter.setPosition).toHaveBeenLastCalledWith(300, 400, "1");
    expect(clickThrough.setDragging).toHaveBeenLastCalledWith(false);
    expect(controller.isDragging()).toBe(false);
  });
});
