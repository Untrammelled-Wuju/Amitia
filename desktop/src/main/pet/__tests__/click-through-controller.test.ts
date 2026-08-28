import { beforeEach, describe, expect, it, vi } from "vitest";

const mouse = vi.hoisted(() => ({ x: 50, y: 50 }));
vi.mock("electron", () => ({
  BrowserWindow: class {},
  screen: {
    getCursorScreenPoint: vi.fn(() => ({ x: mouse.x, y: mouse.y })),
  },
}));

import { ClickThroughController } from "../click-through-controller";

describe("ClickThroughController", () => {
  let win: {
    isDestroyed: ReturnType<typeof vi.fn>;
    once: ReturnType<typeof vi.fn>;
    setIgnoreMouseEvents: ReturnType<typeof vi.fn>;
    getContentSize: ReturnType<typeof vi.fn>;
    getPosition: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    mouse.x = 50;
    mouse.y = 50;
    win = {
      isDestroyed: vi.fn(() => false),
      once: vi.fn(),
      setIgnoreMouseEvents: vi.fn(),
      getContentSize: vi.fn(() => [100, 100]),
      getPosition: vi.fn(() => [0, 0]),
    };
  });

  it("activates alpha hit-testing when alpha mode is selected", () => {
    const controller = new ClickThroughController({} as never, 10);
    controller.attach(win as never);
    controller.updateFrame(2, 2, new Uint8Array([255, 255, 255, 255]));
    mouse.x = 150;
    mouse.y = 150;
    controller.setMode("alpha");

    expect(win.setIgnoreMouseEvents).toHaveBeenCalledWith(true, { forward: true });
    controller.detach();
  });

  it("ignores mouse input outside the alpha hit area", () => {
    const controller = new ClickThroughController({} as never, 10);
    controller.attach(win as never);
    controller.updateFrame(2, 2, new Uint8Array([0, 0, 0, 255]));
    mouse.x = 10;
    mouse.y = 10;
    controller.setMode("alpha");

    expect(win.setIgnoreMouseEvents).toHaveBeenCalledWith(true, { forward: true });
    controller.detach();
  });

  it("full mode always ignores the whole window", () => {
    const controller = new ClickThroughController({} as never, 10);
    controller.attach(win as never);
    controller.setMode("full");

    expect(win.setIgnoreMouseEvents).toHaveBeenCalledWith(true, { forward: true });
    controller.detach();
  });
});
