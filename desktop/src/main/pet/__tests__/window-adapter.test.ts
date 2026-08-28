import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type { DesktopPetWindowOptions } from "../types";
import {
  PET_WINDOW_SCALE_DEFAULT,
  PET_WINDOW_SCALE_MAX,
  PET_WINDOW_SCALE_MIN,
} from "../types";

const mockState = vi.hoisted(() => {
  const state = {
    BrowserWindow: vi.fn() as unknown as ReturnType<typeof vi.fn>,
    screen: {
      getPrimaryDisplay: vi.fn() as unknown as ReturnType<typeof vi.fn>,
      getAllDisplays: vi.fn() as unknown as ReturnType<typeof vi.fn>,
      getDisplayNearestPoint: vi.fn() as unknown as ReturnType<typeof vi.fn>,
      getDisplayMatching: vi.fn() as unknown as ReturnType<typeof vi.fn>,
      on: vi.fn() as unknown as ReturnType<typeof vi.fn>,
      off: vi.fn() as unknown as ReturnType<typeof vi.fn>,
    },
    app: {
      getPath: vi.fn() as unknown as ReturnType<typeof vi.fn>,
      dock: {
        hide: vi.fn() as unknown as ReturnType<typeof vi.fn>,
        show: vi.fn() as unknown as ReturnType<typeof vi.fn>,
      },
    },
    nativeImage: {
      createFromPath: vi.fn() as unknown as ReturnType<typeof vi.fn>,
    },
    powerMonitor: {
      on: vi.fn() as unknown as ReturnType<typeof vi.fn>,
      off: vi.fn() as unknown as ReturnType<typeof vi.fn>,
    },
  };
  return state;
});

vi.mock("electron", () => mockState);

interface MockDisplay {
  id: number;
  bounds: { x: number; y: number; width: number; height: number };
  workArea: { x: number; y: number; width: number; height: number };
  scaleFactor: number;
  label: string;
}

interface MockWindowState {
  position: { x: number; y: number };
  size: { width: number; height: number };
  destroyed: boolean;
  options: Record<string, unknown>;
  listeners: Map<string, Array<(...args: unknown[]) => void>>;
  webContentsListeners: Map<string, Array<(...args: unknown[]) => void>>;
  onceListeners: Map<string, Array<(...args: unknown[]) => void>>;
}

function createMockDisplay(overrides: Partial<MockDisplay> = {}): MockDisplay {
  const base: MockDisplay = {
    id: 1,
    bounds: { x: 0, y: 0, width: 1920, height: 1080 },
    workArea: { x: 0, y: 0, width: 1920, height: 1040 },
    scaleFactor: 1,
    label: "Primary",
  };
  return {
    ...base,
    ...overrides,
    bounds: { ...base.bounds, ...overrides.bounds },
    workArea: { ...base.workArea, ...overrides.workArea },
  };
}

function createMockWindow(
  options: {
    x?: number;
    y?: number;
    width?: number;
    height?: number;
  } = {},
): MockWindowState & {
  on: ReturnType<typeof vi.fn>;
  off: ReturnType<typeof vi.fn>;
  once: ReturnType<typeof vi.fn>;
  show: ReturnType<typeof vi.fn>;
  hide: ReturnType<typeof vi.fn>;
  close: ReturnType<typeof vi.fn>;
  destroy: ReturnType<typeof vi.fn>;
  setPosition: ReturnType<typeof vi.fn>;
  getPosition: ReturnType<typeof vi.fn>;
  getSize: ReturnType<typeof vi.fn>;
  setSize: ReturnType<typeof vi.fn>;
  setAlwaysOnTop: ReturnType<typeof vi.fn>;
  setSkipTaskbar: ReturnType<typeof vi.fn>;
  setIgnoreMouseEvents: ReturnType<typeof vi.fn>;
  isDestroyed: ReturnType<typeof vi.fn>;
  loadFile: ReturnType<typeof vi.fn>;
  loadURL: ReturnType<typeof vi.fn>;
  webContents: {
    on: ReturnType<typeof vi.fn>;
    once: ReturnType<typeof vi.fn>;
    off: ReturnType<typeof vi.fn>;
    setAudioMuted: ReturnType<typeof vi.fn>;
  };
} {
  const state: MockWindowState = {
    position: { x: options.x ?? 0, y: options.y ?? 0 },
    size: {
      width: options.width ?? 256,
      height: options.height ?? 256,
    },
    destroyed: false,
    options: {},
    listeners: new Map(),
    webContentsListeners: new Map(),
    onceListeners: new Map(),
  };

  const on = vi.fn((event: string, cb: (...args: unknown[]) => void) => {
    if (!state.listeners.has(event)) state.listeners.set(event, []);
    state.listeners.get(event)!.push(cb);
  });
  const off = vi.fn((event: string, cb: (...args: unknown[]) => void) => {
    const arr = state.listeners.get(event);
    if (arr) {
      const idx = arr.indexOf(cb);
      if (idx >= 0) arr.splice(idx, 1);
    }
  });
  const once = vi.fn((event: string, cb: (...args: unknown[]) => void) => {
    if (event === "ready-to-show") {
      cb();
    } else {
      if (!state.onceListeners.has(event)) state.onceListeners.set(event, []);
      state.onceListeners.get(event)!.push(cb);
    }
  });

  return {
    on,
    off,
    once,
    show: vi.fn(),
    hide: vi.fn(),
    close: vi.fn(),
    destroy: vi.fn(() => {
      state.destroyed = true;
    }),
    setPosition: vi.fn((x: number, y: number) => {
      state.position.x = x;
      state.position.y = y;
    }),
    getPosition: vi.fn(() => [state.position.x, state.position.y]),
    getSize: vi.fn(() => [state.size.width, state.size.height]),
    setSize: vi.fn((w: number, h: number) => {
      state.size.width = w;
      state.size.height = h;
    }),
    setAlwaysOnTop: vi.fn(),
    setSkipTaskbar: vi.fn(),
    setIgnoreMouseEvents: vi.fn(),
    isDestroyed: vi.fn(() => state.destroyed),
    loadFile: vi.fn(async () => undefined),
    loadURL: vi.fn(async () => undefined),
    webContents: {
      on: vi.fn((event: string, cb: (...args: unknown[]) => void) => {
        if (!state.webContentsListeners.has(event))
          state.webContentsListeners.set(event, []);
        state.webContentsListeners.get(event)!.push(cb);
      }),
      once: vi.fn((event: string, cb: (...args: unknown[]) => void) => {
        if (!state.webContentsListeners.has(event))
          state.webContentsListeners.set(event, []);
        state.webContentsListeners.get(event)!.push(cb);
      }),
      off: vi.fn((event: string, cb: (...args: unknown[]) => void) => {
        const arr = state.webContentsListeners.get(event);
        if (!arr) return;
        const idx = arr.indexOf(cb);
        if (idx >= 0) arr.splice(idx, 1);
      }),
      setAudioMuted: vi.fn(),
    },
    ...state,
  };
}

type MockWindow = ReturnType<typeof createMockWindow>;

function installMocks(
  windowMock: MockWindow,
  displays: MockDisplay[] = [createMockDisplay()],
): void {
  const primary = displays.find((d) => d.id === 1) ?? displays[0];

  mockState.BrowserWindow.mockImplementation(function (opts?: Record<string, unknown>) {
    if (opts) {
      windowMock.options = opts;
      if (typeof opts.width === "number") windowMock.size.width = opts.width;
      if (typeof opts.height === "number") windowMock.size.height = opts.height;
      if (typeof opts.x === "number") windowMock.position.x = opts.x;
      if (typeof opts.y === "number") windowMock.position.y = opts.y;
    }
    return windowMock;
  });

  mockState.screen.getPrimaryDisplay.mockReturnValue(primary);
  mockState.screen.getAllDisplays.mockReturnValue(displays);
  mockState.screen.getDisplayNearestPoint.mockImplementation(
    (point: { x: number; y: number }) => {
      for (const d of displays) {
        const b = d.bounds;
        if (
          point.x >= b.x &&
          point.x < b.x + b.width &&
          point.y >= b.y &&
          point.y < b.y + b.height
        ) {
          return d;
        }
      }
      return primary;
    },
  );
  mockState.screen.getDisplayMatching.mockImplementation(
    (rect: { x: number; y: number; width: number; height: number }) => {
      const cx = rect.x + rect.width / 2;
      const cy = rect.y + rect.height / 2;
      for (const d of displays) {
        const b = d.bounds;
        if (cx >= b.x && cx < b.x + b.width && cy >= b.y && cy < b.y + b.height) {
          return d;
        }
      }
      return primary;
    },
  );
  mockState.screen.on.mockClear();
  mockState.screen.off.mockClear();
}

function makeOptions(
  overrides: Partial<DesktopPetWindowOptions> = {},
): DesktopPetWindowOptions {
  return {
    canvasWidth: 256,
    canvasHeight: 256,
    scale: PET_WINDOW_SCALE_DEFAULT,
    alwaysOnTop: true,
    clickThroughMode: "none",
    ...overrides,
  };
}

describe("DesktopPetWindowAdapter", () => {
  let DesktopPetWindowAdapter: typeof import("../window-adapter").DesktopPetWindowAdapter;
  let windowMock: MockWindow;

  beforeEach(async () => {
    vi.resetModules();
    windowMock = createMockWindow();
    installMocks(windowMock);
    const mod = await import("../window-adapter");
    DesktopPetWindowAdapter = mod.DesktopPetWindowAdapter;
  });

  afterEach(() => {
    vi.resetModules();
  });

  it("create 创建透明无边框且跳过任务栏的窗口", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    expect(windowMock.options.transparent).toBe(true);
    expect(windowMock.options.frame).toBe(false);
    expect(windowMock.options.skipTaskbar).toBe(true);
    expect(windowMock.options.hasShadow).toBe(false);
    expect(windowMock.options.resizable).toBe(false);
    expect(windowMock.options.show).toBe(false);
  });

  it("create 默认置顶时 alwaysOnTop 透传到窗口选项", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ alwaysOnTop: true }),
    );
    await adapter.create();

    expect(windowMock.options.alwaysOnTop).toBe(true);
  });

  it("create 窗口尺寸根据画布与缩放计算", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ canvasWidth: 200, canvasHeight: 300, scale: 1.5 }),
    );
    await adapter.create();

    expect(windowMock.options.width).toBe(300);
    expect(windowMock.options.height).toBe(450);
  });

  it("create 已存在且未销毁的窗口时返回同一实例", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    const first = await adapter.create();
    const second = await adapter.create();

    expect(second).toBe(first);
  });

  it("create 只完成 renderer 加载，RuntimeReady 前不显示窗口", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    expect(windowMock.show).not.toHaveBeenCalled();

    adapter.showWhenRuntimeReady();
    expect(windowMock.show).toHaveBeenCalledTimes(1);
  });

  it("create 注册 move/resize/closed 与 webContents 事件", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    const onCalls = windowMock.on.mock.calls;
    expect(onCalls.find((c) => c[0] === "move")).toBeDefined();
    expect(onCalls.find((c) => c[0] === "resize")).toBeDefined();
    expect(onCalls.find((c) => c[0] === "closed")).toBeDefined();

    const wcCalls = windowMock.webContents.on.mock.calls;
    expect(wcCalls.find((c) => c[0] === "render-process-gone")).toBeDefined();
    expect(wcCalls.find((c) => c[0] === "unresponsive")).toBeDefined();
  });

  it("destroy 关闭窗口并清空监听器", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    await adapter.destroy();

    expect(windowMock.destroy).toHaveBeenCalled();
    expect(adapter.isCreated()).toBe(false);
    expect(adapter.getNativeWindow()).toBeNull();
  });

  it("destroy 已销毁的窗口不重复调用 destroy", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();
    await adapter.destroy();

    windowMock.destroy.mockClear();
    await adapter.destroy();
    expect(windowMock.destroy).not.toHaveBeenCalled();
  });

  it("isCreated 在创建前为 false，创建后为 true", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    expect(adapter.isCreated()).toBe(false);

    await adapter.create();
    expect(adapter.isCreated()).toBe(true);
  });

  it("getNativeWindow 返回底层 BrowserWindow", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    expect(adapter.getNativeWindow()).toBe(windowMock);
  });

  it("getNativeWindow 窗口已销毁时返回 null", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    (windowMock.destroy as () => void)();

    expect(adapter.getNativeWindow()).toBeNull();
  });

  it("setPosition 设置窗口坐标", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    await adapter.setPosition(100, 200);

    expect(windowMock.setPosition).toHaveBeenCalledWith(100, 200);
  });

  it("setPosition 指定 screenId 时基于目标显示器偏移", async () => {
    const second = createMockDisplay({
      id: 2,
      bounds: { x: 1920, y: 0, width: 1920, height: 1080 },
      workArea: { x: 1920, y: 0, width: 1920, height: 1040 },
      label: "External",
    });
    installMocks(windowMock, [createMockDisplay(), second]);

    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    await adapter.setPosition(50, 60, "2");

    expect(windowMock.setPosition).toHaveBeenCalledWith(1970, 60);
  });

  it("setPosition 窗口未创建时为空操作", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());

    await expect(adapter.setPosition(10, 20)).resolves.toBeUndefined();
    expect(windowMock.setPosition).not.toHaveBeenCalled();
  });

  it("getPosition 窗口未创建时回退到 options.position", () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ position: { x: 30, y: 40, screenId: "1" } }),
    );

    const pos = adapter.getPosition();
    expect(pos.x).toBe(30);
    expect(pos.y).toBe(40);
    expect(pos.screenId).toBe("1");
  });

  it("getPosition 窗口已创建时返回窗口坐标与最近显示器", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    windowMock.position.x = 120;
    windowMock.position.y = 220;
    const pos = adapter.getPosition();
    expect(pos.x).toBe(120);
    expect(pos.y).toBe(220);
    expect(pos.screenId).toBe("1");
  });

  it("setScale 钳制到合法范围并触发尺寸重算", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ canvasWidth: 100, canvasHeight: 100 }),
    );
    await adapter.create();

    await adapter.setScale(10);
    expect(adapter.getOptions().scale).toBe(PET_WINDOW_SCALE_MAX);
    expect(windowMock.setSize).toHaveBeenCalledWith(
      100 * PET_WINDOW_SCALE_MAX,
      100 * PET_WINDOW_SCALE_MAX,
    );

    await adapter.setScale(-1);
    expect(adapter.getOptions().scale).toBe(PET_WINDOW_SCALE_MIN);
  });

  it("setScale 非有限值回退到默认缩放", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    await adapter.setScale(Number.NaN);
    expect(adapter.getOptions().scale).toBe(PET_WINDOW_SCALE_DEFAULT);
  });

  it("setScale 窗口未创建时仅更新 options", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());

    await adapter.setScale(1.8);
    expect(adapter.getOptions().scale).toBe(1.8);
    expect(windowMock.setSize).not.toHaveBeenCalled();
  });

  it("setAlwaysOnTop 启用时调用 setAlwaysOnTop(true, level)", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ alwaysOnTop: false }),
    );
    await adapter.create();

    await adapter.setAlwaysOnTop(true);
    expect(windowMock.setAlwaysOnTop).toHaveBeenCalledWith(
      true,
      expect.any(String),
    );
  });

  it("setAlwaysOnTop 关闭时调用 setAlwaysOnTop(false)", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    await adapter.setAlwaysOnTop(false);
    expect(windowMock.setAlwaysOnTop).toHaveBeenCalledWith(false);
  });

  it("setClickThroughMode alpha 模式启用 forward 鼠标事件", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ clickThroughMode: "none" }),
    );
    await adapter.create();

    await adapter.setClickThroughMode("alpha");
    expect(windowMock.setIgnoreMouseEvents).toHaveBeenCalledWith(false, {
      forward: true,
    });
  });

  it("setClickThroughMode none 模式关闭穿透", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ clickThroughMode: "alpha" }),
    );
    await adapter.create();

    await adapter.setClickThroughMode("none");
    expect(windowMock.setIgnoreMouseEvents).toHaveBeenCalledWith(false);
  });

  it("setClickThroughMode 非法值回退到 none", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    await adapter.setClickThroughMode("invalid-mode");
    expect(adapter.getOptions().clickThroughMode).toBe("none");
  });

  it("on/off 注册与注销事件监听器", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    const moveListeners = windowMock.listeners.get("move") ?? [];
    expect(moveListeners.length).toBeGreaterThan(0);
    const moveHandler = moveListeners[0];

    const listener = vi.fn();
    adapter.on("move", listener);
    moveHandler();

    expect(listener).toHaveBeenCalledTimes(1);

    adapter.off("move", listener);
    moveHandler();
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it("resize 事件通过窗口 resize 触发监听器", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    const resizeListeners = windowMock.listeners.get("resize") ?? [];
    expect(resizeListeners.length).toBeGreaterThan(0);

    const listener = vi.fn();
    adapter.on("resize", listener);
    resizeListeners[0]();
    expect(listener).toHaveBeenCalled();
  });

  it("close 事件触发监听器", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    const closedListeners = windowMock.listeners.get("closed") ?? [];
    expect(closedListeners.length).toBeGreaterThan(0);

    const listener = vi.fn();
    adapter.on("close", listener);
    closedListeners[0]();
    expect(listener).toHaveBeenCalled();
  });

  it("crashed 事件通过 webContents render-process-gone 触发", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    const goneListeners =
      windowMock.webContentsListeners.get("render-process-gone") ?? [];
    // loadRenderer installs a temporary startup crash listener, but cleanup
    // must remove only that listener and preserve the long-lived recovery one.
    expect(goneListeners).toHaveLength(1);

    const listener = vi.fn();
    adapter.on("crashed", listener);
    goneListeners[0]();
    expect(listener).toHaveBeenCalled();
  });

  it("recalculateSize 按当前缩放更新窗口尺寸", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ canvasWidth: 200, canvasHeight: 200, scale: 2 }),
    );
    await adapter.create();

    windowMock.setSize.mockClear();
    const result = adapter.recalculateSize();
    expect(result.width).toBe(400);
    expect(result.height).toBe(400);
    expect(windowMock.setSize).toHaveBeenCalledWith(400, 400);
  });

  it("recalculateSize 窗口未创建时只返回计算尺寸", () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ canvasWidth: 128, canvasHeight: 128, scale: 1 }),
    );

    const result = adapter.recalculateSize();
    expect(result.width).toBe(128);
    expect(result.height).toBe(128);
  });

  it("getDpiScale 窗口未创建时返回主显示器 scaleFactor", () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    expect(adapter.getDpiScale()).toBe(1);
  });

  it("getDpiScale 窗口已创建时返回最近显示器 scaleFactor", async () => {
    const second = createMockDisplay({
      id: 2,
      bounds: { x: 1920, y: 0, width: 1920, height: 1080 },
      workArea: { x: 1920, y: 0, width: 1920, height: 1040 },
      scaleFactor: 2,
      label: "HiDPI",
    });
    installMocks(windowMock, [createMockDisplay(), second]);

    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();
    windowMock.position.x = 2000;
    windowMock.position.y = 100;

    expect(adapter.getDpiScale()).toBe(2);
  });

  it("listScreens 返回所有显示器信息", async () => {
    const primary = createMockDisplay();
    const external = createMockDisplay({
      id: 2,
      bounds: { x: 1920, y: 0, width: 1920, height: 1080 },
      workArea: { x: 1920, y: 0, width: 1920, height: 1040 },
      label: "External",
    });
    installMocks(windowMock, [primary, external]);

    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    const screens = adapter.listScreens();
    expect(screens).toHaveLength(2);
    expect(screens[0].isPrimary).toBe(true);
    expect(screens[1].isPrimary).toBe(false);
    expect(screens[1].label).toBe("External");
  });

  it("restorePosition 根据记录恢复位置与缩放", async () => {
    const second = createMockDisplay({
      id: 2,
      bounds: { x: 1920, y: 0, width: 1920, height: 1080 },
      workArea: { x: 1920, y: 0, width: 1920, height: 1040 },
      label: "External",
    });
    installMocks(windowMock, [createMockDisplay(), second]);

    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ canvasWidth: 100, canvasHeight: 100 }),
    );
    await adapter.create();

    windowMock.setPosition.mockClear();
    adapter.restorePosition({ screenId: "2", x: 50, y: 60, scale: 1.5 });

    expect(adapter.getOptions().scale).toBe(1.5);
    expect(windowMock.setSize).toHaveBeenCalledWith(150, 150);
    expect(windowMock.setPosition).toHaveBeenCalledWith(1970, 60);
  });

  it("restorePosition 目标显示器不存在时仅确保可见", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();
    windowMock.position.x = 5000;
    windowMock.position.y = 5000;

    windowMock.setPosition.mockClear();
    adapter.restorePosition({ screenId: "missing", x: 10, y: 20, scale: 1 });

    expect(windowMock.setPosition).toHaveBeenCalled();
  });

  it("restorePosition 窗口未创建时为空操作", () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    expect(() =>
      adapter.restorePosition({ screenId: "1", x: 0, y: 0, scale: 1 }),
    ).not.toThrow();
  });

  it("moveToScreen 将窗口迁移到目标显示器", async () => {
    const primary = createMockDisplay();
    const external = createMockDisplay({
      id: 2,
      bounds: { x: 1920, y: 0, width: 1920, height: 1080 },
      workArea: { x: 1920, y: 0, width: 1920, height: 1040 },
      label: "External",
    });
    installMocks(windowMock, [primary, external]);

    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();
    windowMock.position.x = 100;
    windowMock.position.y = 100;

    windowMock.setPosition.mockClear();
    const ok = adapter.moveToScreen("2");
    expect(ok).toBe(true);
    expect(windowMock.setPosition).toHaveBeenCalledWith(2020, 100);
  });

  it("moveToScreen 目标不存在时返回 false", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();

    expect(adapter.moveToScreen("missing")).toBe(false);
  });

  it("moveToScreen 窗口未创建时返回 false", () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    expect(adapter.moveToScreen("1")).toBe(false);
  });

  it("snapshotRuntimePosition 返回相对显示器的坐标记录", async () => {
    const external = createMockDisplay({
      id: 2,
      bounds: { x: 1920, y: 0, width: 1920, height: 1080 },
      workArea: { x: 1920, y: 0, width: 1920, height: 1040 },
      label: "External",
    });
    installMocks(windowMock, [createMockDisplay(), external]);

    const adapter = new DesktopPetWindowAdapter(makeOptions({ scale: 1.25 }));
    await adapter.create();
    windowMock.position.x = 2000;
    windowMock.position.y = 100;

    const snap = adapter.snapshotRuntimePosition();
    expect(snap.screenId).toBe("2");
    expect(snap.x).toBe(80);
    expect(snap.y).toBe(100);
    expect(snap.scale).toBe(1.25);
  });

  it("constructor 钳制非法 scale 到默认值", () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ scale: Number.POSITIVE_INFINITY }),
    );
    expect(adapter.getOptions().scale).toBe(PET_WINDOW_SCALE_DEFAULT);
  });

  it("constructor 钳制超限 scale 到边界值", () => {
    const tooLarge = new DesktopPetWindowAdapter(makeOptions({ scale: 10 }));
    expect(tooLarge.getOptions().scale).toBe(PET_WINDOW_SCALE_MAX);

    const tooSmall = new DesktopPetWindowAdapter(makeOptions({ scale: 0.1 }));
    expect(tooSmall.getOptions().scale).toBe(PET_WINDOW_SCALE_MIN);
  });

  it("constructor 非法 clickThroughMode 回退到 none", () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ clickThroughMode: "invalid" as never }),
    );
    expect(adapter.getOptions().clickThroughMode).toBe("none");
  });

  it("getOptions 返回 options 副本，修改不影响内部状态", async () => {
    const adapter = new DesktopPetWindowAdapter(makeOptions());
    const opts = adapter.getOptions();
    opts.scale = 99;

    expect(adapter.getOptions().scale).not.toBe(99);
  });

  it("create 未提供 position 时默认置于主显示器右下角", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ canvasWidth: 100, canvasHeight: 100, scale: 1 }),
    );
    await adapter.create();

    const primary = createMockDisplay();
    const expectedX = primary.workArea.x + primary.workArea.width - 100 - 40;
    const expectedY = primary.workArea.y + primary.workArea.height - 100 - 40;

    expect(windowMock.options.x).toBe(Math.round(expectedX));
    expect(windowMock.options.y).toBe(Math.round(expectedY));
  });

  it("create 提供 position 且 screenId 匹配时基于目标显示器偏移", async () => {
    const external = createMockDisplay({
      id: 2,
      bounds: { x: 1920, y: 0, width: 1920, height: 1080 },
      workArea: { x: 1920, y: 0, width: 1920, height: 1040 },
      label: "External",
    });
    installMocks(windowMock, [createMockDisplay(), external]);

    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ position: { x: 50, y: 60, screenId: "2" } }),
    );
    await adapter.create();

    expect(windowMock.options.x).toBe(1970);
    expect(windowMock.options.y).toBe(60);
  });

  it("create 提供 position 但 screenId 不匹配时使用绝对坐标", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ position: { x: 200, y: 300, screenId: "missing" } }),
    );
    await adapter.create();

    expect(windowMock.options.x).toBe(200);
    expect(windowMock.options.y).toBe(300);
  });

  it("ensureVisible 窗口超出工作区右侧时移回可见区域", async () => {
    const primary = createMockDisplay();
    installMocks(windowMock, [primary]);

    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();
    windowMock.position.x = primary.workArea.x + primary.workArea.width + 100;
    windowMock.position.y = 100;

    windowMock.setPosition.mockClear();
    adapter.recalculateSize();

    const setPositionCalls = windowMock.setPosition.mock.calls;
    expect(setPositionCalls.length).toBeGreaterThan(0);
    const lastCall = setPositionCalls[setPositionCalls.length - 1];
    expect(lastCall[0]).toBeLessThanOrEqual(
      primary.workArea.x + primary.workArea.width - 20,
    );
  });

  it("soundEnabled 通过 webContents 音频静音状态真实生效", async () => {
    const adapter = new DesktopPetWindowAdapter(
      makeOptions({ soundEnabled: false }),
    );
    await adapter.create();

    expect(windowMock.webContents.setAudioMuted).toHaveBeenCalledWith(true);

    await adapter.setSoundEnabled(true);
    expect(windowMock.webContents.setAudioMuted).toHaveBeenLastCalledWith(false);
    expect(adapter.getOptions().soundEnabled).toBe(true);
  });

  it("屏幕 display-metrics-changed 事件触发 ensureVisible", async () => {
    const primary = createMockDisplay();
    installMocks(windowMock, [primary]);

    const adapter = new DesktopPetWindowAdapter(makeOptions());
    await adapter.create();
    windowMock.position.x = primary.workArea.x + primary.workArea.width + 100;
    windowMock.position.y = 100;

    const metricsCalls = mockState.screen.on.mock.calls;
    const metricsCall = metricsCalls.find(
      (call) => call[0] === "display-metrics-changed",
    );
    expect(metricsCall).toBeDefined();

    windowMock.setPosition.mockClear();
    const cb = metricsCall![1] as (...args: unknown[]) => void;
    cb({} as never, primary, ["bounds"]);
    expect(windowMock.setPosition).toHaveBeenCalled();
  });
});
