import { vi } from "vitest";
import { join } from "node:path";
import { tmpdir } from "node:os";
import type { LoadedInstallation, RuntimeAction } from "../resource-loader";

export interface MockWindowOptions {
  width?: number;
  height?: number;
  x?: number;
  y?: number;
  destroyed?: boolean;
  visible?: boolean;
  alwaysOnTop?: boolean;
}

export interface MockDisplay {
  id: number;
  bounds: { x: number; y: number; width: number; height: number };
  workArea: { x: number; y: number; width: number; height: number };
  scaleFactor: number;
  label: string;
}

export function createMockDisplay(
  overrides: Partial<MockDisplay> = {},
): MockDisplay {
  const base: MockDisplay = {
    id: 1,
    bounds: { x: 0, y: 0, width: 1920, height: 1080 },
    workArea: { x: 0, y: 0, width: 1920, height: 1040 },
    scaleFactor: 1,
    label: "Primary",
  };
  return { ...base, ...overrides, bounds: { ...base.bounds, ...overrides.bounds }, workArea: { ...base.workArea, ...overrides.workArea } };
}

export interface MockBrowserWindow {
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
  webContents: {
    on: ReturnType<typeof vi.fn>;
    off: ReturnType<typeof vi.fn>;
    setAudioMuted: ReturnType<typeof vi.fn>;
  };
  __options: Record<string, unknown>;
  __position: { x: number; y: number };
  __size: { width: number; height: number };
  __destroyed: boolean;
}

export function createMockBrowserWindow(
  options: MockWindowOptions = {},
): MockBrowserWindow {
  const state = { destroyed: options.destroyed ?? false };
  const position = { x: options.x ?? 0, y: options.y ?? 0 };
  const size = {
    width: options.width ?? 256,
    height: options.height ?? 256,
  };
  const win: MockBrowserWindow = {
    on: vi.fn(),
    off: vi.fn(),
    once: vi.fn((event: string, cb: () => void) => {
      if (event === "ready-to-show") {
        cb();
      }
    }),
    show: vi.fn(),
    hide: vi.fn(),
    close: vi.fn(),
    destroy: vi.fn(() => {
      state.destroyed = true;
    }),
    setPosition: vi.fn((x: number, y: number) => {
      position.x = x;
      position.y = y;
    }),
    getPosition: vi.fn(() => [position.x, position.y]),
    getSize: vi.fn(() => [size.width, size.height]),
    setSize: vi.fn((w: number, h: number) => {
      size.width = w;
      size.height = h;
    }),
    setAlwaysOnTop: vi.fn(),
    setSkipTaskbar: vi.fn(),
    setIgnoreMouseEvents: vi.fn(),
    isDestroyed: vi.fn(() => state.destroyed),
    webContents: {
      on: vi.fn(),
      off: vi.fn(),
      setAudioMuted: vi.fn(),
    },
    __options: options as Record<string, unknown>,
    __position: position,
    __size: size,
    __destroyed: state.destroyed,
  };
  return win;
}

export function installElectronMock(
  windowMock: MockBrowserWindow,
  displays: MockDisplay[] = [createMockDisplay()],
): {
  browserWindowCtor: ReturnType<typeof vi.fn>;
  screenMock: Record<string, ReturnType<typeof vi.fn>>;
  appMock: {
    getPath: ReturnType<typeof vi.fn>;
    dock: {
      hide: ReturnType<typeof vi.fn>;
      show: ReturnType<typeof vi.fn>;
    };
  };
  restore: () => void;
} {
  const browserWindowCtor = vi.fn(
    (opts?: Record<string, unknown>) => {
      if (opts) {
        (windowMock as MockBrowserWindow).__options = opts;
        if (typeof opts.width === "number") {
          (windowMock as MockBrowserWindow).__size.width = opts.width;
        }
        if (typeof opts.height === "number") {
          (windowMock as MockBrowserWindow).__size.height = opts.height;
        }
        if (typeof opts.x === "number") {
          (windowMock as MockBrowserWindow).__position.x = opts.x;
        }
        if (typeof opts.y === "number") {
          (windowMock as MockBrowserWindow).__position.y = opts.y;
        }
      }
      return windowMock;
    },
  );

  const primary = displays.find((d) => d.id === 1) ?? displays[0];

  const screenMock = {
    getPrimaryDisplay: vi.fn(() => primary),
    getAllDisplays: vi.fn(() => displays),
    getDisplayNearestPoint: vi.fn((point: { x: number; y: number }) => {
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
    }),
    getDisplayMatching: vi.fn(
      (rect: { x: number; y: number; width: number; height: number }) => {
        const centerX = rect.x + rect.width / 2;
        const centerY = rect.y + rect.height / 2;
        for (const d of displays) {
          const b = d.bounds;
          if (
            centerX >= b.x &&
            centerX < b.x + b.width &&
            centerY >= b.y &&
            centerY < b.y + b.height
          ) {
            return d;
          }
        }
        return primary;
      },
    ),
    on: vi.fn(),
    off: vi.fn(),
  };

  const appMock = {
    getPath: vi.fn((name: string) => {
      if (name === "userData") {
        return join(tmpdir(), "amitia-test-pet");
      }
      return join(tmpdir(), "amitia-test");
    }),
    dock: {
      hide: vi.fn(),
      show: vi.fn(),
    },
  };

  const electronMock = {
    BrowserWindow: browserWindowCtor,
    screen: screenMock,
    app: appMock,
    nativeImage: {
      createFromPath: vi.fn(() => ({
        isEmpty: vi.fn(() => false),
        getSize: vi.fn(() => ({ width: 128, height: 128 })),
        toDataURL: vi.fn(() => "data:image/png;base64,AAA"),
        toBitmap: vi.fn(() => Buffer.alloc(128 * 128 * 4, 255)),
      })),
    },
    powerMonitor: {
      on: vi.fn(),
      off: vi.fn(),
    },
  };

  vi.stubGlobal("electron", electronMock);
  return {
    browserWindowCtor,
    screenMock,
    appMock,
    restore: () => {
      vi.unstubAllGlobals();
    },
  };
}

export interface ManualRafController {
  nextTick: (deltaMs: number) => void;
  advance: (totalMs: number, intervalMs?: number) => void;
  getTime: () => number;
  restore: () => void;
}

export function installManualRaf(): ManualRafController {
  let rafTime = 0;
  let queue: Array<{ cb: (ts: number) => void; id: number }> = [];
  let nextId = 1;

  const originalPerformance =
    typeof performance !== "undefined" ? performance : undefined;
  const originalRaf =
    typeof globalThis.requestAnimationFrame === "function"
      ? globalThis.requestAnimationFrame
      : undefined;
  const originalCaf =
    typeof globalThis.cancelAnimationFrame === "function"
      ? globalThis.cancelAnimationFrame
      : undefined;

  vi.stubGlobal("performance", {
    now: () => rafTime,
  });
  vi.stubGlobal("requestAnimationFrame", (cb: (ts: number) => void) => {
    const id = nextId++;
    queue.push({ cb, id });
    return id;
  });
  vi.stubGlobal("cancelAnimationFrame", (id: number) => {
    queue = queue.filter((item) => item.id !== id);
  });

  const drainOnce = (deltaMs: number): void => {
    rafTime += deltaMs;
    if (queue.length === 0) return;
    const current = queue;
    queue = [];
    const ts = rafTime;
    for (const item of current) {
      item.cb(ts);
    }
  };

  return {
    nextTick: (deltaMs: number) => {
      drainOnce(deltaMs);
    },
    advance: (totalMs: number, intervalMs = 16) => {
      let remaining = totalMs;
      let guard = 0;
      while (remaining > 0 && queue.length > 0 && guard < 10000) {
        guard += 1;
        const step = Math.min(intervalMs, remaining);
        drainOnce(step);
        remaining -= step;
      }
    },
    getTime: () => rafTime,
    restore: () => {
      queue = [];
      rafTime = 0;
      if (originalPerformance) {
        vi.stubGlobal("performance", originalPerformance);
      } else {
        vi.unstubAllGlobals();
      }
      if (originalRaf) {
        vi.stubGlobal("requestAnimationFrame", originalRaf);
      }
      if (originalCaf) {
        vi.stubGlobal("cancelAnimationFrame", originalCaf);
      }
    },
  };
}

export function makeRuntimeAction(
  overrides: Partial<RuntimeAction> = {},
): RuntimeAction {
  const base: RuntimeAction = {
    key: "idle",
    name: "Idle",
    version: "1",
    loopType: "loop",
    fps: 8,
    frameDurationMs: 125,
    frameCount: 4,
    frames: [
      "frame0.png",
      "frame1.png",
      "frame2.png",
      "frame3.png",
    ],
    interruptible: true,
    available: true,
  };
  return { ...base, ...overrides };
}

export function makeLoadedInstallation(
  overrides: Partial<LoadedInstallation> = {},
): LoadedInstallation {
  const idle = makeRuntimeAction({
    key: "idle",
    name: "Idle",
    frameCount: 4,
    frameDurationMs: 125,
    frames: ["idle0.png", "idle1.png", "idle2.png", "idle3.png"],
  });
  const happy = makeRuntimeAction({
    key: "happy",
    name: "Happy",
    frameCount: 3,
    frames: ["happy0.png", "happy1.png", "happy2.png"],
  });
  const wave = makeRuntimeAction({
    key: "wave",
    name: "Wave",
    frameCount: 3,
    frames: ["wave0.png", "wave1.png", "wave2.png"],
  });
  const actions = new Map<string, RuntimeAction>();
  actions.set("idle", idle);
  actions.set("happy", happy);
  actions.set("wave", wave);

  const base: LoadedInstallation = {
    installationId: "test-install",
    manifest: {
      packageId: "test-install",
      schemaVersion: 1,
      name: "Test",
      characterId: "char-1",
      canvas: { width: 256, height: 256 },
      defaultAction: "idle",
      actions: [
        {
          key: "idle",
          name: "Idle",
          version: "1",
          loopType: "loop",
          fps: 8,
          frameDurationMs: 125,
          frameCount: 4,
          frames: ["idle0.png", "idle1.png", "idle2.png", "idle3.png"],
          interruptible: true,
        },
        {
          key: "happy",
          name: "Happy",
          version: "1",
          loopType: "once",
          fps: 8,
          frameDurationMs: 125,
          frameCount: 3,
          frames: ["happy0.png", "happy1.png", "happy2.png"],
          interruptible: true,
        },
        {
          key: "wave",
          name: "Wave",
          version: "1",
          loopType: "once",
          fps: 8,
          frameDurationMs: 125,
          frameCount: 3,
          frames: ["wave0.png", "wave1.png", "wave2.png"],
          interruptible: true,
        },
      ],
    },
    actions,
    defaultAction: idle,
    installPath: "/test/install",
    manifestPath: "/test/install/manifest.json",
    previewPath: "/test/install/preview.png",
  };
  return { ...base, ...overrides };
}
