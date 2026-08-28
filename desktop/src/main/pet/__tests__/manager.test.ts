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
    restoreOnAppStart: true,
    scale: 1,
    positionX: 10,
    positionY: 20,
    screenId: "1",
    idleEnabled: true,
    idleIntervalMinSeconds: 30,
    idleIntervalMaxSeconds: 120,
    clickThroughMode: "none",
    soundEnabled: false,
    positionMode: "absolute",
    displayFingerprint: "",
    relativeX: 0,
    relativeY: 0,
    lastWindowWidth: 0,
    lastWindowHeight: 0,
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

describe("DesktopPetManager app-start restore policy", () => {
  it("disables authoritative desired state before Runtime v2 connects when restore is off", async () => {
    const manager = makeManager();
    const internal = manager as never as {
      listInstallations: ReturnType<typeof vi.fn>;
      fetchRuntimeSettings: ReturnType<typeof vi.fn>;
      callDisableApi: ReturnType<typeof vi.fn>;
      detectCorruption: ReturnType<typeof vi.fn>;
      loadAndValidateInstallation: ReturnType<typeof vi.fn>;
      setState: ReturnType<typeof vi.fn>;
      restoreActiveInstallation: () => Promise<void>;
    };
    internal.listInstallations = vi.fn(async () => [{
      id: "install-1",
      isActive: true,
      status: "enabled",
    }]);
    internal.fetchRuntimeSettings = vi.fn(async () =>
      settings({ restoreOnAppStart: false }),
    );
    internal.callDisableApi = vi.fn(async () => undefined);
    internal.detectCorruption = vi.fn();
    internal.loadAndValidateInstallation = vi.fn();
    internal.setState = vi.fn();

    await internal.restoreActiveInstallation();

    expect(internal.callDisableApi).toHaveBeenCalledWith("install-1");
    expect(internal.detectCorruption).not.toHaveBeenCalled();
    expect(internal.loadAndValidateInstallation).not.toHaveBeenCalled();
    expect(internal.setState).toHaveBeenCalledWith(
      "ready",
      null,
      "启动恢复已关闭",
    );
  });
});

describe("DesktopPetManager desired-state convergence", () => {
  it("applies settings and default action for an already-running installation", async () => {
    const manager = makeManager();
    const idleRuntime = { key: "idle", available: true };
    const waveRuntime = { key: "wave", available: true };
    const internal = manager as never as {
      activeInstallationId: string;
      state: string;
      activeInstallation: { currentReleaseId: string; defaultActionKey: string };
      activeSettings: RuntimeSettingsInfo;
      loadedInstallation: {
        defaultAction: typeof idleRuntime;
        actions: Map<string, typeof idleRuntime>;
      };
      animationIpc: { sendUpdateDefaultAction: ReturnType<typeof vi.fn> };
      idleController: {
        playDefaultIdle: ReturnType<typeof vi.fn>;
        updateConfig: ReturnType<typeof vi.fn>;
      };
      windowAdapter: {
        setScale: ReturnType<typeof vi.fn>;
        setAlwaysOnTop: ReturnType<typeof vi.fn>;
        setPosition: ReturnType<typeof vi.fn>;
        setSoundEnabled: ReturnType<typeof vi.fn>;
        setClickThroughMode: ReturnType<typeof vi.fn>;
      };
      clickThroughController: { setMode: ReturnType<typeof vi.fn> };
      enableInstallationInternal: ReturnType<typeof vi.fn>;
      runLifecycleMutation: <T>(fn: () => Promise<T>) => Promise<T>;
      applyDesiredStateCommand: (command: unknown) => Promise<void>;
    };
    internal.activeInstallationId = "install-1";
    internal.state = "enabled";
    internal.activeInstallation = {
      currentReleaseId: "release-1",
      defaultActionKey: "idle",
    };
    internal.activeSettings = settings();
    internal.loadedInstallation = {
      defaultAction: idleRuntime,
      actions: new Map([
        ["idle", idleRuntime],
        ["wave", waveRuntime],
      ]),
    };
    internal.animationIpc = {
      sendUpdateDefaultAction: vi.fn(() => ({ status: "delivered" })),
    };
    internal.idleController = {
      playDefaultIdle: vi.fn(),
      updateConfig: vi.fn(),
    };
    internal.windowAdapter = {
      setScale: vi.fn(async () => undefined),
      setAlwaysOnTop: vi.fn(async () => undefined),
      setPosition: vi.fn(async () => undefined),
      setSoundEnabled: vi.fn(async () => undefined),
      setClickThroughMode: vi.fn(async () => undefined),
    };
    internal.clickThroughController = { setMode: vi.fn() };
    internal.enableInstallationInternal = vi.fn(async () => undefined);
    internal.runLifecycleMutation = async <T>(fn: () => Promise<T>) => fn();

    await internal.applyDesiredStateCommand({
      installationId: "install-1",
      releaseId: "release-1",
      settingsRevision: 7,
      payload: {
        installationId: "install-1",
        releaseId: "release-1",
        defaultActionKey: "wave",
        settingsRevision: 7,
        settingsSnapshot: {
          installationId: "install-1",
          settingsRevision: 7,
          alwaysOnTop: 0,
          restoreOnAppStart: 1,
          scale: 1.5,
          positionX: 101,
          positionY: 202,
          screenId: "2",
          idleEnabled: 0,
          idleIntervalMinSeconds: 8,
          idleIntervalMaxSeconds: 16,
          clickThroughMode: "alpha",
          soundEnabled: 1,
          positionMode: "absolute",
          displayFingerprint: "",
          relativeX: 0,
          relativeY: 0,
          lastWindowWidth: 0,
          lastWindowHeight: 0,
        },
      },
    });

    expect(internal.enableInstallationInternal).toHaveBeenCalledWith(
      "install-1",
      false,
      false,
    );
    expect(internal.windowAdapter.setScale).toHaveBeenCalledWith(1.5);
    expect(internal.windowAdapter.setAlwaysOnTop).toHaveBeenCalledWith(false);
    expect(internal.windowAdapter.setPosition).toHaveBeenCalledWith(101, 202, "2");
    expect(internal.idleController.updateConfig).toHaveBeenCalledWith({
      enabled: false,
      minIntervalSeconds: 8,
      maxIntervalSeconds: 16,
    });
    expect(internal.animationIpc.sendUpdateDefaultAction).toHaveBeenCalledWith("wave");
    expect(internal.activeInstallation.defaultActionKey).toBe("wave");
    expect(internal.loadedInstallation.defaultAction).toBe(waveRuntime);
    expect(internal.activeSettings.settingsRevision).toBe(7);
  });

  it("waits for renderer readiness before committing a queued default action", async () => {
    const manager = makeManager();
    const idleRuntime = { key: "idle", available: true };
    const waveRuntime = { key: "wave", available: true };
    const sendUpdateDefaultAction = vi.fn()
      .mockReturnValueOnce({ status: "queued" })
      .mockReturnValueOnce({ status: "delivered" });
    const waitForRuntimeReady = vi.fn(async () => ({
      packageId: "pkg",
      packageRevision: 1,
    }));
    const internal = manager as never as {
      activeInstallation: { defaultActionKey: string };
      loadedInstallation: {
        defaultAction: typeof idleRuntime;
        actions: Map<string, typeof idleRuntime>;
      };
      animationIpc: {
        sendUpdateDefaultAction: typeof sendUpdateDefaultAction;
        waitForRuntimeReady: typeof waitForRuntimeReady;
      };
      idleController: { playDefaultIdle: ReturnType<typeof vi.fn> };
      applyDefaultActionLocal: (actionKey: string) => Promise<void>;
    };
    internal.activeInstallation = { defaultActionKey: "idle" };
    internal.loadedInstallation = {
      defaultAction: idleRuntime,
      actions: new Map([
        ["idle", idleRuntime],
        ["wave", waveRuntime],
      ]),
    };
    internal.animationIpc = { sendUpdateDefaultAction, waitForRuntimeReady };
    internal.idleController = { playDefaultIdle: vi.fn() };

    const promise = internal.applyDefaultActionLocal("wave");
    expect(internal.activeInstallation.defaultActionKey).toBe("idle");
    await promise;

    expect(waitForRuntimeReady).toHaveBeenCalledTimes(1);
    expect(sendUpdateDefaultAction).toHaveBeenCalledTimes(2);
    expect(internal.activeInstallation.defaultActionKey).toBe("wave");
    expect(internal.loadedInstallation.defaultAction).toBe(waveRuntime);
  });

  it("falls back to authoritative settings when a desired snapshot is incomplete", async () => {
    const manager = makeManager();
    const fetched = settings({
      settingsRevision: 5,
      alwaysOnTop: false,
      scale: 2,
      clickThroughMode: "full",
    });
    const internal = manager as never as {
      activeInstallationId: string;
      activeSettings: RuntimeSettingsInfo;
      windowAdapter: {
        setScale: ReturnType<typeof vi.fn>;
        setAlwaysOnTop: ReturnType<typeof vi.fn>;
        setPosition: ReturnType<typeof vi.fn>;
        setSoundEnabled: ReturnType<typeof vi.fn>;
        setClickThroughMode: ReturnType<typeof vi.fn>;
      };
      clickThroughController: { setMode: ReturnType<typeof vi.fn> };
      idleController: { updateConfig: ReturnType<typeof vi.fn> };
      fetchRuntimeSettings: ReturnType<typeof vi.fn>;
      applyDesiredSettings: (
        installationId: string,
        revision: number,
        snapshot: unknown,
      ) => Promise<void>;
    };
    internal.activeInstallationId = "install-1";
    internal.activeSettings = settings({ settingsRevision: 1 });
    internal.windowAdapter = {
      setScale: vi.fn(async () => undefined),
      setAlwaysOnTop: vi.fn(async () => undefined),
      setPosition: vi.fn(async () => undefined),
      setSoundEnabled: vi.fn(async () => undefined),
      setClickThroughMode: vi.fn(async () => undefined),
    };
    internal.clickThroughController = { setMode: vi.fn() };
    internal.idleController = { updateConfig: vi.fn() };
    internal.fetchRuntimeSettings = vi.fn(async () => fetched);

    await internal.applyDesiredSettings("install-1", 5, {
      installationId: "install-1",
      settingsRevision: 5,
      alwaysOnTop: 0,
      // Deliberately omit the rest of the required runtime snapshot.
    });

    expect(internal.fetchRuntimeSettings).toHaveBeenCalledWith("install-1");
    expect(internal.windowAdapter.setScale).toHaveBeenCalledWith(2);
    expect(internal.windowAdapter.setClickThroughMode).toHaveBeenCalledWith("full");
    expect(internal.clickThroughController.setMode).toHaveBeenCalledWith("full");
    expect(internal.activeSettings.settingsRevision).toBe(5);
  });

  it("forces a runtime reload when the desired release changes", async () => {
    const manager = makeManager();
    const internal = manager as never as {
      activeInstallationId: string;
      state: string;
      activeInstallation: { currentReleaseId: string; defaultActionKey: string };
      activeSettings: RuntimeSettingsInfo;
      enableInstallationInternal: ReturnType<typeof vi.fn>;
      runLifecycleMutation: <T>(fn: () => Promise<T>) => Promise<T>;
      applyDesiredStateCommand: (command: unknown) => Promise<void>;
    };
    internal.activeInstallationId = "install-1";
    internal.state = "enabled";
    internal.activeInstallation = {
      currentReleaseId: "release-old",
      defaultActionKey: "idle",
    };
    internal.activeSettings = settings();
    internal.enableInstallationInternal = vi.fn(async () => {
      internal.activeInstallation.currentReleaseId = "release-new";
    });
    internal.runLifecycleMutation = async <T>(fn: () => Promise<T>) => fn();

    await internal.applyDesiredStateCommand({
      installationId: "install-1",
      releaseId: "release-new",
      payload: {
        installationId: "install-1",
        releaseId: "release-new",
      },
    });

    expect(internal.enableInstallationInternal).toHaveBeenCalledWith(
      "install-1",
      false,
      true,
    );
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
