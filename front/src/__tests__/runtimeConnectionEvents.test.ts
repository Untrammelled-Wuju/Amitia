import { beforeEach, describe, expect, it, vi } from "vitest";

const endpoint = { value: "http://127.0.0.1:18899" };
let statusListener: ((status: { state: string }) => void) | null = null;
const desktopApi = {
  getDeploymentConfig: vi.fn(async () => ({ mode: "local" as const })),
  getRuntimeEndpoint: vi.fn(async () => ({ httpBaseURL: endpoint.value })),
  onRuntimeStatusChanged: vi.fn((listener: (status: { state: string }) => void) => {
    statusListener = listener;
    return () => { statusListener = null; };
  }),
};

describe("runtime connection events", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    endpoint.value = "http://127.0.0.1:18899";
    statusListener = null;
    Object.defineProperty(window, "amitiaDesktop", {
      configurable: true,
      value: desktopApi,
    });
  });

  it("相同 Core 地址的 starting 和 ready 不广播连接变化", async () => {
    const adapter = await import("../runtime/runtime-adapter");
    await adapter.getRuntimeConnection();
    const listener = vi.fn();
    window.addEventListener("amitia:runtime-connection-changed", listener);
    try {
      statusListener?.({ state: "starting" });
      statusListener?.({ state: "ready" });
      await Promise.resolve();
      await Promise.resolve();
      expect(listener).not.toHaveBeenCalled();
    } finally {
      window.removeEventListener("amitia:runtime-connection-changed", listener);
    }
  });

  it("Core 地址实际变化时广播连接变化", async () => {
    const adapter = await import("../runtime/runtime-adapter");
    await adapter.getRuntimeConnection();
    const listener = vi.fn();
    window.addEventListener("amitia:runtime-connection-changed", listener);
    try {
      endpoint.value = "http://127.0.0.1:19999";
      statusListener?.({ state: "ready" });
      await vi.waitFor(() => expect(listener).toHaveBeenCalledTimes(1));
    } finally {
      window.removeEventListener("amitia:runtime-connection-changed", listener);
    }
  });
});
