import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearRuntimeCache, getApiBaseURLForPath, LOCAL_DEVICE_RUNTIME_BASE_URL } from "../runtime/runtime-adapter";

describe("角色路由", () => {
  beforeEach(() => clearRuntimeCache());
  afterEach(() => {
    delete (window as any).amitiaDesktop;
    clearRuntimeCache();
  });

  it("本地模式使用本地服务，云端统筹模式始终使用 Core", async () => {
    let mode = "cloud";
    const getDeploymentConfig = vi.fn(async () => mode === "cloud"
      ? { mode: "cloud", serverURL: "https://core.example" }
      : { mode: "local" });
    Object.defineProperty(window, "amitiaDesktop", { configurable: true, value: {
      getDeploymentConfig,
      getMeshStatus: vi.fn().mockResolvedValue({ cloudBaseUrl: "https://core.example" }),
      getBackendAuthHeaders: vi.fn().mockResolvedValue({}),
    } });

    expect(await getApiBaseURLForPath("/api/characters/authority")).toBe(
      LOCAL_DEVICE_RUNTIME_BASE_URL + "/internal/device-mesh/provider",
    );
    mode = "local";
    clearRuntimeCache();
    expect(await getApiBaseURLForPath("/api/companion/role-profile")).toBe(
      LOCAL_DEVICE_RUNTIME_BASE_URL,
    );
  });
});
