import { describe, expect, it, vi } from "vitest";

vi.mock("electron", () => ({
  app: {
    getPath: vi.fn(() => "/tmp/amitia-test"),
    getVersion: vi.fn(() => "26.1.8"),
    setLoginItemSettings: vi.fn(),
  },
  BrowserWindow: class {},
  ipcMain: {
    on: vi.fn(),
    off: vi.fn(),
    handle: vi.fn(),
    removeHandler: vi.fn(),
    removeListener: vi.fn(),
  },
  powerMonitor: { on: vi.fn(), off: vi.fn() },
  protocol: {
    registerSchemesAsPrivileged: vi.fn(),
    handle: vi.fn(),
  },
  nativeImage: {
    createFromPath: vi.fn(() => ({
      isEmpty: vi.fn(() => false),
      getSize: vi.fn(() => ({ width: 1, height: 1 })),
      toBitmap: vi.fn(() => Buffer.alloc(4)),
    })),
  },
  screen: {
    on: vi.fn(),
    off: vi.fn(),
    getAllDisplays: vi.fn(() => []),
    getPrimaryDisplay: vi.fn(() => ({ id: 1, bounds: { x: 0, y: 0, width: 1920, height: 1080 } })),
  },
}));

import { DesktopPetManager, type RuntimeSettingsInfo } from "../manager";

function makeManager(): DesktopPetManager {
  return new DesktopPetManager({
    resourceLoader: {} as never,
    resourceCache: {} as never,
    petLogger: {
      logRuntimeCrash: vi.fn(),
      logDisable: vi.fn(),
    } as never,
  });
}

function settings(overrides: Partial<RuntimeSettingsInfo> = {}): RuntimeSettingsInfo {
  return {
    installationId: "install-1",
    settingsRevision: 1,
    alwaysOnTop: true,
    launchOnStartup: false,
    scale: 1,
    positionX: 10,
    positionY: 20,
    screenId: "1",
    idleEnabled: true,
    idleIntervalMinSeconds: 30,
    idleIntervalMaxSeconds: 120,
    clickThroughMode: "none",
    soundEnabled: false,
    ...overrides,
  };
}

describe("DesktopPetManager runtime settings", () => {
  it("applies every runtime-backed setting before committing the revision", async () => {
    const manager = makeManager();
    const internal = manager as never as {
      activeInstallationId: string;
      activeSettings: RuntimeSettingsInfo;
      lastAppliedSettingsRevision: number;
      windowAdapter: {
        setScale: ReturnType<typeof vi.fn>;
        setAlwaysOnTop: ReturnType<typeof vi.fn>;
        setPosition: ReturnType<typeof vi.fn>;
        setSoundEnabled: ReturnType<typeof vi.fn>;
        setClickThroughMode: ReturnType<typeof vi.fn>;
      };
      clickThroughController: { setMode: ReturnType<typeof vi.fn> };
      idleController: { updateConfig: ReturnType<typeof vi.fn> };
      applyRuntimeSettingsLocal: (updates: Partial<RuntimeSettingsInfo>, revision: number) => Promise<void>;
    };
    internal.activeInstallationId = "install-1";
    internal.activeSettings = settings();
    internal.lastAppliedSettingsRevision = 1;
    internal.windowAdapter = {
      setScale: vi.fn(async () => undefined),
      setAlwaysOnTop: vi.fn(async () => undefined),
      setPosition: vi.fn(async () => undefined),
      setSoundEnabled: vi.fn(async () => undefined),
      setClickThroughMode: vi.fn(async () => undefined),
    };
    internal.clickThroughController = { setMode: vi.fn() };
    internal.idleController = { updateConfig: vi.fn() };

    await internal.applyRuntimeSettingsLocal(
      {
        scale: 4,
        alwaysOnTop: false,
        positionX: 100,
        positionY: 200,
        screenId: "2",
        soundEnabled: true,
        clickThroughMode: "alpha",
        idleEnabled: false,
        idleIntervalMinSeconds: 5,
        idleIntervalMaxSeconds: 10,
      },
      7,
    );

    expect(internal.windowAdapter.setScale).toHaveBeenCalledWith(4);
    expect(internal.windowAdapter.setAlwaysOnTop).toHaveBeenCalledWith(false);
    expect(internal.windowAdapter.setPosition).toHaveBeenCalledWith(100, 200, "2");
    expect(internal.windowAdapter.setSoundEnabled).toHaveBeenCalledWith(true);
    expect(internal.windowAdapter.setClickThroughMode).toHaveBeenCalledWith("alpha");
    expect(internal.clickThroughController.setMode).toHaveBeenCalledWith("alpha");
    expect(internal.idleController.updateConfig).toHaveBeenCalledWith({
      enabled: false,
      minIntervalSeconds: 5,
      maxIntervalSeconds: 10,
    });
    expect(internal.activeSettings.settingsRevision).toBe(7);
    expect(internal.lastAppliedSettingsRevision).toBe(7);
  });

  it("does not commit activeSettings/revision when a runtime side effect fails", async () => {
    const manager = makeManager();
    const initial = settings();
    const internal = manager as never as {
      activeInstallationId: string;
      activeSettings: RuntimeSettingsInfo;
      lastAppliedSettingsRevision: number;
      windowAdapter: {
        setPosition: ReturnType<typeof vi.fn>;
      };
      clickThroughController: null;
      idleController: null;
      applyRuntimeSettingsLocal: (updates: Partial<RuntimeSettingsInfo>, revision: number) => Promise<void>;
    };
    internal.activeInstallationId = "install-1";
    internal.activeSettings = initial;
    internal.lastAppliedSettingsRevision = 1;
    internal.windowAdapter = {
      setPosition: vi.fn(async () => {
        throw new Error("position failed");
      }),
    };
    internal.clickThroughController = null;
    internal.idleController = null;

    await expect(
      internal.applyRuntimeSettingsLocal({ positionX: 99 }, 9),
    ).rejects.toThrow("position failed");
    expect(internal.activeSettings).toBe(initial);
    expect(internal.lastAppliedSettingsRevision).toBe(1);
  });
});

describe("DesktopPetManager disable transaction", () => {
  it("keeps the local runtime active when authoritative backend disable fails", async () => {
    const manager = makeManager();
    const internal = manager as never as {
      activeInstallationId: string;
      activeInstallation: object;
      activeSettings: RuntimeSettingsInfo;
      persistRuntimePosition: ReturnType<typeof vi.fn>;
      callDisableApi: ReturnType<typeof vi.fn>;
      stopRuntime: ReturnType<typeof vi.fn>;
      teardownRecoveryHandlers: ReturnType<typeof vi.fn>;
      disableInternal: (notifyBackend: boolean, persistPosition?: boolean) => Promise<void>;
    };
    internal.activeInstallationId = "install-1";
    internal.activeInstallation = {};
    internal.activeSettings = settings();
    internal.persistRuntimePosition = vi.fn(async () => undefined);
    internal.callDisableApi = vi.fn(async () => {
      throw new Error("backend unavailable");
    });
    internal.stopRuntime = vi.fn(async () => undefined);
    internal.teardownRecoveryHandlers = vi.fn();

    await expect(internal.disableInternal(true)).rejects.toThrow("backend unavailable");
    expect(internal.stopRuntime).not.toHaveBeenCalled();
    expect(internal.teardownRecoveryHandlers).not.toHaveBeenCalled();
    expect(internal.activeInstallationId).toBe("install-1");
  });
});
