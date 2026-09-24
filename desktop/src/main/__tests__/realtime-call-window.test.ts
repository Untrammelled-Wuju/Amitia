import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => {
  const handlers = new Map<string, (...args: any[]) => unknown>();
  const windows: any[] = [];

  class BrowserWindow {
    static fromWebContents() {
      return null;
    }

    options: Record<string, unknown>;
    webContents = {
      setWindowOpenHandler: vi.fn(),
      send: vi.fn(),
    };
    isDestroyed = vi.fn(() => false);
    isMinimized = vi.fn(() => false);
    restore = vi.fn();
    show = vi.fn();
    focus = vi.fn();
    close = vi.fn();
    once = vi.fn();
    on = vi.fn();
    loadURL = vi.fn(async () => undefined);

    constructor(options: Record<string, unknown>) {
      this.options = options;
      windows.push(this);
    }
  }

  return {
    handlers,
    windows,
    BrowserWindow,
    buildRendererURL: vi.fn(() => "http://127.0.0.1:15178/#/call-window"),
    ipcMain: {
      handle: vi.fn((channel: string, handler: (...args: any[]) => unknown) => {
        handlers.set(channel, handler);
      }),
    },
    screen: {
      getCursorScreenPoint: vi.fn(() => ({ x: 100, y: 100 })),
      getDisplayNearestPoint: vi.fn(() => ({
        workArea: { x: 0, y: 0, width: 1920, height: 1080 },
      })),
    },
    shell: { openExternal: vi.fn() },
  };
});

vi.mock("electron", () => ({
  BrowserWindow: mocks.BrowserWindow,
  ipcMain: mocks.ipcMain,
  screen: mocks.screen,
  shell: mocks.shell,
}));

vi.mock("../window", () => ({
  buildRendererURL: mocks.buildRendererURL,
}));

import { IPC_CHANNELS } from "../../shared/ipc";
import { registerRealtimeCallWindowHandlers } from "../realtime-call-window";

describe("realtime call window", () => {
  it("opens a compact independent window and focuses the existing window", async () => {
    const mainWindow = {
      getBounds: () => ({ x: 100, y: 60, width: 1280, height: 820 }),
    };
    registerRealtimeCallWindowHandlers(() => mainWindow as any);

    const open = mocks.handlers.get(IPC_CHANNELS.openRealtimeCallWindow);
    const close = mocks.handlers.get(IPC_CHANNELS.closeRealtimeCallWindow);
    expect(open).toBeTypeOf("function");
    expect(close).toBeTypeOf("function");

    await open!({}, {
      mode: "voice",
      conversationId: "conversation-1",
      charName: "小晴",
    });

    expect(mocks.windows).toHaveLength(1);
    expect(mocks.windows[0].options).toMatchObject({
      width: 390,
      height: 680,
      frame: false,
      alwaysOnTop: true,
      resizable: true,
    });
    expect(mocks.buildRendererURL).toHaveBeenCalledWith(
      "/call-window",
      expect.objectContaining({
        mode: "voice",
        conversationId: "conversation-1",
        charName: "小晴",
      }),
    );

    await open!({}, { mode: "video" });
    expect(mocks.windows).toHaveLength(1);
    expect(mocks.windows[0].show).toHaveBeenCalled();
    expect(mocks.windows[0].focus).toHaveBeenCalled();

    close!({});
    expect(mocks.windows[0].close).toHaveBeenCalled();
  });
});
