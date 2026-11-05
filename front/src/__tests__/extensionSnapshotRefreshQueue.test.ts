import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  fetchUISnapshot: vi.fn(),
  fetchContributions: vi.fn(),
  fetchUIProfile: vi.fn(),
  createBridgeSession: vi.fn(),
  revokeBridgeSession: vi.fn(),
  updateUIProfile: vi.fn(),
  deleteUIProfileOverride: vi.fn(),
  resolveUIHostDeviceId: vi.fn(),
  resolveUIClientInfo: vi.fn(),
  getRuntimeConnection: vi.fn(),
  loadLastKnownGoodSnapshot: vi.fn(),
  saveLastKnownGoodSnapshot: vi.fn(),
}));

vi.mock("@/api/extension", () => ({
  fetchUISnapshot: mocks.fetchUISnapshot,
  fetchContributions: mocks.fetchContributions,
  fetchUIProfile: mocks.fetchUIProfile,
  createBridgeSession: mocks.createBridgeSession,
  revokeBridgeSession: mocks.revokeBridgeSession,
  updateUIProfile: mocks.updateUIProfile,
  deleteUIProfileOverride: mocks.deleteUIProfileOverride,
}));

vi.mock("@/ui-runtime/deviceIdentity", () => ({
  resolveUIHostDeviceId: mocks.resolveUIHostDeviceId,
}));

vi.mock("@/ui-runtime/clientInfo", () => ({
  resolveUIClientInfo: mocks.resolveUIClientInfo,
}));

vi.mock("@/runtime/runtime-adapter", () => ({
  getRuntimeConnection: mocks.getRuntimeConnection,
}));

vi.mock("@/ui-runtime/snapshotCache", () => ({
  loadLastKnownGoodSnapshot: mocks.loadLastKnownGoodSnapshot,
  saveLastKnownGoodSnapshot: mocks.saveLastKnownGoodSnapshot,
}));

vi.mock("@/composables/useHostEnvironment", () => ({
  resolveHostEnvironment: () => ({ platform: "macos" }),
}));

import { useExtensionUIStore } from "../stores/extensionUI";

function snapshot(generated: string) {
  return {
    slots: [],
    contributions: [],
    pendingContributions: [],
    generatedAt: generated,
    version: 1,
    providers: [],
    profileLayers: [],
    resolved: {},
    providerContext: { platform: "macos" },
    providerVersion: 1,
  };
}

describe("extension snapshot refresh queue", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setActivePinia(createPinia());
    mocks.resolveUIHostDeviceId.mockResolvedValue("device");
    mocks.resolveUIClientInfo.mockResolvedValue({ architecture: "arm64", appVersion: "test" });
    mocks.getRuntimeConnection.mockResolvedValue({ apiBaseURL: "http://127.0.0.1:1" });
    mocks.loadLastKnownGoodSnapshot.mockReturnValue(null);
  });

  it("runs another snapshot fetch when an invalidation arrives during loading", async () => {
    let resolveFirst: ((value: ReturnType<typeof snapshot>) => void) | undefined;
    mocks.fetchUISnapshot
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveFirst = resolve;
      }))
      .mockResolvedValueOnce(snapshot("after-event"));
    const store = useExtensionUIStore();

    const firstRefresh = store.refreshSnapshot(true);
    await vi.waitFor(() => expect(mocks.fetchUISnapshot).toHaveBeenCalledTimes(1));
    const eventRefresh = store.refreshSnapshot(true);
    resolveFirst?.(snapshot("before-event"));

    await Promise.all([firstRefresh, eventRefresh]);

    expect(mocks.fetchUISnapshot).toHaveBeenCalledTimes(2);
    expect(store.snapshot?.generatedAt).toBe("after-event");
  });
});
