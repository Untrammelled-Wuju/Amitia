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
      toDataURL: vi.fn(() => "data:image/png;base64,"),
    })),
    createFromBuffer: vi.fn(() => ({
      isEmpty: vi.fn(() => false),
      getSize: vi.fn(() => ({ width: 1, height: 1 })),
      toBitmap: vi.fn(() => Buffer.alloc(4)),
      toDataURL: vi.fn(() => "data:image/png;base64,"),
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
import { getRuntimeId } from "../runtime-identity";

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

  it("fails closed when authoritative settings flags are missing or malformed", async () => {
    const manager = makeManager();
    const internal = manager as never as {
      request: ReturnType<typeof vi.fn>;
    };
    internal.request = vi.fn(async () => ({
      installationId: "install-1",
      settingsRevision: 1,
      alwaysOnTop: 1,
      // restoreOnAppStart intentionally missing: undefined must never mean true.
      scale: 1,
      positionX: 0,
      positionY: 0,
      screenId: "1",
      idleEnabled: 1,
      idleIntervalMinSeconds: 30,
      idleIntervalMaxSeconds: 120,
      clickThroughMode: "off",
      soundEnabled: 0,
      positionMode: "absolute",
    }));

    await expect(manager.getRuntimeSettings("install-1")).rejects.toThrow(
      "RUNTIME_SETTINGS_INVALID: restoreOnAppStart must be 0 or 1",
    );
  });

  it("rejects runtime settings returned for a different installation", async () => {
    const manager = makeManager();
    const internal = manager as never as {
      request: ReturnType<typeof vi.fn>;
    };
    internal.request = vi.fn(async () => ({
      installationId: "install-other",
      settingsRevision: 1,
      alwaysOnTop: 1,
      restoreOnAppStart: 1,
      scale: 1,
      positionX: 0,
      positionY: 0,
      screenId: "1",
      idleEnabled: 1,
      idleIntervalMinSeconds: 30,
      idleIntervalMaxSeconds: 120,
      clickThroughMode: "off",
      soundEnabled: 0,
      positionMode: "absolute",
    }));

    await expect(manager.getRuntimeSettings("install-1")).rejects.toThrow(
      "RUNTIME_SETTINGS_ID_MISMATCH",
    );
  });
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

  it("does not apply public settings mutations before Runtime v2 convergence", async () => {
    const manager = makeManager();
    const initial = settings({ settingsRevision: 3, scale: 1 });
    const callUpdateSettingsApi = vi.fn(async () => ({
      operationId: "opin-settings",
      status: "waiting_runtime_ack",
      stage: "waiting_runtime_ack",
      desiredRevision: 8,
      settings: settings({ settingsRevision: 4, scale: 2 }),
    }));
    const applyRuntimeSettingsLocal = vi.fn(async () => undefined);
    const internal = manager as never as {
      activeInstallationId: string;
      activeSettings: RuntimeSettingsInfo;
      callUpdateSettingsApi: typeof callUpdateSettingsApi;
      applyRuntimeSettingsLocal: typeof applyRuntimeSettingsLocal;
    };
    internal.activeInstallationId = "install-1";
    internal.activeSettings = initial;
    internal.callUpdateSettingsApi = callUpdateSettingsApi;
    internal.applyRuntimeSettingsLocal = applyRuntimeSettingsLocal;

    const result = await manager.updateSettings({ scale: 2 });

    expect(result).toMatchObject({
      operationId: "opin-settings",
      status: "waiting_runtime_ack",
      desiredRevision: 8,
    });
    expect(callUpdateSettingsApi).toHaveBeenCalledTimes(1);
    expect(applyRuntimeSettingsLocal).not.toHaveBeenCalled();
    expect(internal.activeSettings).toBe(initial);
    expect(internal.activeSettings.settingsRevision).toBe(3);
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

  it("does not apply a public default-action mutation before Runtime v2 convergence", async () => {
    const manager = makeManager();
    const callUpdateDefaultActionApi = vi.fn(async () => ({
      operationId: "opin-default",
      status: "waiting_runtime_ack",
      stage: "waiting_runtime_ack",
      desiredRevision: 9,
    }));
    const applyDefaultActionLocal = vi.fn(async () => undefined);
    const internal = manager as never as {
      activeInstallationId: string;
      callUpdateDefaultActionApi: typeof callUpdateDefaultActionApi;
      applyDefaultActionLocal: typeof applyDefaultActionLocal;
    };
    internal.activeInstallationId = "install-1";
    internal.callUpdateDefaultActionApi = callUpdateDefaultActionApi;
    internal.applyDefaultActionLocal = applyDefaultActionLocal;

    const result = await manager.updateDefaultAction("wave");

    expect(result).toMatchObject({
      operationId: "opin-default",
      status: "waiting_runtime_ack",
      desiredRevision: 9,
    });
    expect(callUpdateDefaultActionApi).toHaveBeenCalledWith("install-1", "wave");
    expect(applyDefaultActionLocal).not.toHaveBeenCalled();
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

describe("DesktopPetManager position persistence authority", () => {
  it("persists a physical drag without claiming the returned settings revision is applied", async () => {
    const manager = makeManager();
    const initial = settings({ settingsRevision: 3, positionX: 10, positionY: 20 });
    const callUpdateSettingsApi = vi.fn(async () => ({
      operationId: "opin-position",
      status: "waiting_runtime_ack",
      stage: "waiting_runtime_ack",
      desiredRevision: 10,
      settings: settings({ settingsRevision: 4, positionX: 300, positionY: 400 }),
    }));
    const internal = manager as never as {
      activeInstallationId: string;
      activeSettings: RuntimeSettingsInfo;
      lastAppliedSettingsRevision: number;
      windowAdapter: { snapshotRuntimePosition: ReturnType<typeof vi.fn> };
      callUpdateSettingsApi: typeof callUpdateSettingsApi;
      persistRuntimePosition: () => Promise<void>;
    };
    internal.activeInstallationId = "install-1";
    internal.activeSettings = initial;
    internal.lastAppliedSettingsRevision = 3;
    internal.windowAdapter = {
      snapshotRuntimePosition: vi.fn(() => ({
        x: 300,
        y: 400,
        screenId: "2",
        scale: 1.25,
      })),
    };
    internal.callUpdateSettingsApi = callUpdateSettingsApi;

    await internal.persistRuntimePosition();

    expect(callUpdateSettingsApi).toHaveBeenCalledTimes(1);
    expect(internal.activeSettings).toBe(initial);
    expect(internal.activeSettings.settingsRevision).toBe(3);
    expect(internal.lastAppliedSettingsRevision).toBe(3);
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

describe("DesktopPetManager lifecycle serialization", () => {
  it("queues shutdown behind an in-flight lifecycle mutation", async () => {
    const manager = makeManager();
    let releaseMutation: (() => void) | null = null;
    const internal = manager as never as {
      runLifecycleMutation: <T>(operation: () => Promise<T>) => Promise<T>;
      teardownRecoveryHandlers: ReturnType<typeof vi.fn>;
      stopBridge: ReturnType<typeof vi.fn>;
      stopRuntime: ReturnType<typeof vi.fn>;
      setState: ReturnType<typeof vi.fn>;
    };
    internal.teardownRecoveryHandlers = vi.fn();
    internal.stopBridge = vi.fn();
    internal.stopRuntime = vi.fn(async () => undefined);
    internal.setState = vi.fn();

    const mutation = internal.runLifecycleMutation(
      () => new Promise<void>((resolve) => {
        releaseMutation = resolve;
      }),
    );
    const shutdown = manager.shutdown();

    await Promise.resolve();
    expect(internal.stopRuntime).not.toHaveBeenCalled();

    releaseMutation?.();
    await mutation;
    await shutdown;

    expect(internal.teardownRecoveryHandlers).toHaveBeenCalledTimes(1);
    expect(internal.stopBridge).toHaveBeenCalledTimes(1);
    expect(internal.stopRuntime).toHaveBeenCalledTimes(1);
    expect(internal.setState).toHaveBeenCalledWith(
      "uninitialized",
      null,
      "shutdown",
    );
  });

  it("serializes recovery work through the lifecycle queue", async () => {
    const manager = makeManager();
    const calls: string[] = [];
    const internal = manager as never as {
      runLifecycleMutation: <T>(operation: () => Promise<T>) => Promise<T>;
      recoverRuntime: (reason: "manual") => Promise<void>;
      recoverRuntimeInternal: (reason: "manual") => Promise<void>;
    };
    internal.recoverRuntimeInternal = vi.fn(async () => {
      calls.push("recovery");
    });

    let releaseMutation: (() => void) | null = null;
    const mutation = internal.runLifecycleMutation(
      () => new Promise<void>((resolve) => {
        calls.push("mutation-start");
        releaseMutation = () => {
          calls.push("mutation-end");
          resolve();
        };
      }),
    );
    const recovery = internal.recoverRuntime("manual");

    await Promise.resolve();
    expect(calls).toEqual(["mutation-start"]);

    releaseMutation?.();
    await mutation;
    await recovery;

    expect(calls).toEqual(["mutation-start", "mutation-end", "recovery"]);
  });
});

describe("DesktopPetManager manual play authority", () => {
  it("publishes a manual action only through the Runtime v2 backend path", async () => {
    const manager = makeManager();
    const schedulerSubmit = vi.fn();
    const callPlayActionApi = vi.fn(async () => undefined);
    const internal = manager as never as {
      state: string;
      scheduler: { submit: typeof schedulerSubmit };
      loadedInstallation: { actions: Map<string, { key: string; available: boolean }> };
      dragController: { isDragging: ReturnType<typeof vi.fn> };
      callPlayActionApi: typeof callPlayActionApi;
    };
    internal.state = "enabled";
    internal.scheduler = { submit: schedulerSubmit };
    internal.loadedInstallation = {
      actions: new Map([["wave", { key: "wave", available: true }]]),
    };
    internal.dragController = { isDragging: vi.fn(() => false) };
    internal.callPlayActionApi = callPlayActionApi;

    await manager.playAction("wave");

    expect(callPlayActionApi).toHaveBeenCalledTimes(1);
    expect(callPlayActionApi).toHaveBeenCalledWith("wave");
    expect(schedulerSubmit).not.toHaveBeenCalled();
  });

  it("rejects an unavailable manual action before publishing a command", async () => {
    const manager = makeManager();
    const callPlayActionApi = vi.fn(async () => undefined);
    const internal = manager as never as {
      state: string;
      scheduler: { submit: ReturnType<typeof vi.fn> };
      loadedInstallation: { actions: Map<string, { key: string; available: boolean }> };
      dragController: { isDragging: ReturnType<typeof vi.fn> };
      callPlayActionApi: typeof callPlayActionApi;
    };
    internal.state = "enabled";
    internal.scheduler = { submit: vi.fn() };
    internal.loadedInstallation = {
      actions: new Map([["wave", { key: "wave", available: false }]]),
    };
    internal.dragController = { isDragging: vi.fn(() => false) };
    internal.callPlayActionApi = callPlayActionApi;

    await expect(manager.playAction("wave")).rejects.toThrow("ACTION_NOT_FOUND: wave");
    expect(callPlayActionApi).not.toHaveBeenCalled();
  });
});


describe("DesktopPetManager installation switching", () => {
  it("restores the previous pet when the target pet fails to become ready", async () => {
    const manager = makeManager();
    const switchFailure = new Error("target runtime failed");
    const disableInternal = vi.fn(async () => undefined);
    const enableInstallationInternal = vi
      .fn<(_: string, __: boolean, ___: boolean) => Promise<void>>()
      .mockRejectedValueOnce(switchFailure)
      .mockResolvedValueOnce(undefined);
    const internal = manager as never as {
      state: string;
      activeInstallationId: string | null;
      ensureInitialized: () => Promise<void>;
      disableInternal: (notifyBackend: boolean) => Promise<void>;
      enableInstallationInternal: (
        installationId: string,
        notifyBackend: boolean,
        restoreOnAppStart: boolean,
      ) => Promise<void>;
    };
    internal.state = "enabled";
    internal.activeInstallationId = "pet-old";
    internal.ensureInitialized = vi.fn(async () => undefined);
    internal.disableInternal = disableInternal;
    internal.enableInstallationInternal = enableInstallationInternal;

    await expect(manager.switchInstallation("pet-new")).rejects.toBe(switchFailure);

    expect(disableInternal).toHaveBeenCalledWith(false);
    expect(enableInstallationInternal).toHaveBeenNthCalledWith(
      1,
      "pet-new",
      true,
      false,
    );
    expect(enableInstallationInternal).toHaveBeenNthCalledWith(
      2,
      "pet-old",
      true,
      false,
    );
  });
});

describe("DesktopPetManager character reconciliation", () => {
  it("propagates installation lookup failures so CharacterWatcher can retry", async () => {
    const manager = makeManager();
    const lookupFailure = new Error("installation lookup failed");
    const internal = manager as never as {
      ensureInitialized: () => Promise<void>;
      listInstallations: () => Promise<unknown[]>;
    };
    internal.ensureInitialized = vi.fn(async () => undefined);
    internal.listInstallations = vi.fn(async () => {
      throw lookupFailure;
    });

    await expect(manager.handleCharacterSwitched("character-a")).rejects.toBe(
      lookupFailure,
    );
  });

  it("selects the most recently enabled usable pet for a newly active character", async () => {
    const manager = makeManager();
    const internal = manager as never as {
      state: string;
      activeInstallationId: string | null;
      ensureInitialized: () => Promise<void>;
      listInstallations: () => Promise<Array<{
        id: string;
        characterId: string;
        status: string;
        lastEnabledAt: string;
        createdAt: string;
      }>>;
      switchInstallation: (installationId: string) => Promise<void>;
    };
    internal.state = "enabled";
    internal.activeInstallationId = "pet-character-a";
    internal.ensureInitialized = vi.fn(async () => undefined);
    internal.listInstallations = vi.fn(async () => [
      {
        id: "pet-b-newer-install",
        characterId: "character-b",
        status: "installed",
        lastEnabledAt: "",
        createdAt: "2026-08-28T12:00:00Z",
      },
      {
        id: "pet-b-preferred",
        characterId: "character-b",
        status: "disabled",
        lastEnabledAt: "2026-08-28T10:00:00Z",
        createdAt: "2026-08-20T12:00:00Z",
      },
      {
        id: "pet-b-invalid",
        characterId: "character-b",
        status: "invalid",
        lastEnabledAt: "2026-08-29T10:00:00Z",
        createdAt: "2026-08-29T10:00:00Z",
      },
    ]);
    internal.switchInstallation = vi.fn(async () => undefined);

    await manager.handleCharacterSwitched("character-b");

    expect(internal.switchInstallation).toHaveBeenCalledTimes(1);
    expect(internal.switchInstallation).toHaveBeenCalledWith("pet-b-preferred");
  });

  it("disables the previous character pet when the new character has no usable installation", async () => {
    const manager = makeManager();
    const internal = manager as never as {
      state: string;
      activeInstallationId: string | null;
      ensureInitialized: () => Promise<void>;
      listInstallations: () => Promise<unknown[]>;
      disableInstallation: () => Promise<void>;
    };
    internal.state = "enabled";
    internal.activeInstallationId = "pet-character-a";
    internal.ensureInitialized = vi.fn(async () => undefined);
    internal.listInstallations = vi.fn(async () => []);
    internal.disableInstallation = vi.fn(async () => undefined);

    await manager.handleCharacterSwitched("character-without-pet");

    expect(internal.disableInstallation).toHaveBeenCalledTimes(1);
  });

  it("propagates switch failures so CharacterWatcher does not commit the new character", async () => {
    const manager = makeManager();
    const switchFailure = new Error("switch failed");
    const internal = manager as never as {
      state: string;
      activeInstallationId: string | null;
      ensureInitialized: () => Promise<void>;
      listInstallations: () => Promise<Array<{
        id: string;
        characterId: string;
        status: string;
      }>>;
      switchInstallation: (installationId: string) => Promise<void>;
    };
    internal.state = "enabled";
    internal.activeInstallationId = "pet-old";
    internal.ensureInitialized = vi.fn(async () => undefined);
    internal.listInstallations = vi.fn(async () => [
      { id: "pet-new", characterId: "character-b", status: "enabled" },
    ]);
    internal.switchInstallation = vi.fn(async () => {
      throw switchFailure;
    });

    await expect(manager.handleCharacterSwitched("character-b")).rejects.toBe(
      switchFailure,
    );
    expect(internal.switchInstallation).toHaveBeenCalledWith("pet-new");
  });
});

describe("DesktopPetManager Runtime v2 play command validation", () => {
  it("rejects legacy queue policy before scheduling", async () => {
    const manager = makeManager();
    const submit = vi.fn(() => "played" as const);
    const internal = manager as never as {
      activeInstallationId: string;
      activeInstallation: { characterId: string };
      loadedInstallation: { actions: Map<string, { key: string; available: boolean }> };
      scheduler: { submit: typeof submit };
      buildRuntimeHooks: () => {
        onCommand: (command: unknown, envelope: unknown) => Promise<{
          status: string;
          errorCode: string;
        }>;
      };
    };
    internal.activeInstallationId = "install-1";
    internal.activeInstallation = { characterId: "character-1" };
    internal.loadedInstallation = {
      actions: new Map([["wave", { key: "wave", available: true }]]),
    };
    internal.scheduler = { submit };

    const result = await internal.buildRuntimeHooks().onCommand(
      {
        commandId: "cmd-1",
        commandType: "runtime.command.play_action",
        installationId: "install-1",
        expiresAt: new Date(Date.now() + 60_000).toISOString(),
        payload: {
          installationId: "install-1",
          runtimeId: getRuntimeId(),
          petInstanceId: getRuntimeId(),
          characterId: "character-1",
          actionKey: "wave",
          queuePolicy: "replace",
          semantic: "manual",
        },
      },
      {},
    );

    expect(result.status).toBe("rejected");
    expect(result.errorCode).toBe("INVALID_QUEUE_POLICY");
    expect(submit).not.toHaveBeenCalled();
  });

  it("maps a canonical manual command to one interrupting manual scheduler request", async () => {
    const manager = makeManager();
    const submit = vi.fn(() => "played" as const);
    const internal = manager as never as {
      activeInstallationId: string;
      activeInstallation: { characterId: string };
      loadedInstallation: { actions: Map<string, { key: string; available: boolean }> };
      scheduler: { submit: typeof submit };
      buildRuntimeHooks: () => {
        onCommand: (command: unknown, envelope: unknown) => Promise<{
          status: string;
          errorCode: string;
        }>;
      };
    };
    internal.activeInstallationId = "install-1";
    internal.activeInstallation = { characterId: "character-1" };
    internal.loadedInstallation = {
      actions: new Map([["wave", { key: "wave", available: true }]]),
    };
    internal.scheduler = { submit };

    const result = await internal.buildRuntimeHooks().onCommand(
      {
        commandId: "cmd-2",
        commandType: "runtime.command.play_action",
        installationId: "install-1",
        expiresAt: new Date(Date.now() + 60_000).toISOString(),
        payload: {
          installationId: "install-1",
          runtimeId: getRuntimeId(),
          petInstanceId: getRuntimeId(),
          characterId: "character-1",
          actionKey: "wave",
          queuePolicy: "replace_current",
          semantic: "manual",
        },
      },
      {},
    );

    expect(result.status).toBe("accepted");
    expect(result.errorCode).toBe("");
    expect(submit).toHaveBeenCalledTimes(1);
    expect(submit).toHaveBeenCalledWith(
      expect.objectContaining({
        actionKey: "wave",
        source: "manual",
        priority: 80,
        interrupt: true,
        dedupeKey: "runtime_cmd-2",
      }),
    );
  });
});

describe("DesktopPetManager Runtime report rejection safety", () => {
  it("observes pointer report rejection instead of leaking an unhandled promise", async () => {
    const manager = makeManager();
    const warning = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    const internal = manager as never as {
      runtimeHandler: { sendRuntimeEvent: ReturnType<typeof vi.fn> };
      handleClick: (x: number, y: number) => void;
    };
    internal.runtimeHandler = {
      sendRuntimeEvent: vi.fn(() => Promise.reject(new Error("runtime socket not open"))),
    };

    internal.handleClick(10, 20);
    await Promise.resolve();
    await Promise.resolve();

    expect(warning).toHaveBeenCalledWith(
      "[DesktopPetManager] 上报单击事件失败:",
      "runtime socket not open",
    );
    warning.mockRestore();
  });
});
