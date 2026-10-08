import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearRuntimeCache, getApiBaseURLForPath, LOCAL_DEVICE_RUNTIME_BASE_URL } from "../runtime/runtime-adapter";

describe("角色归属即时路由", () => {
  const fetchMock = vi.fn();
  let coordinated: boolean;
  beforeEach(() => {
    coordinated = false;
    clearRuntimeCache();
    Object.defineProperty(window, "amitiaDesktop", { configurable: true, value: {
      getDeploymentConfig: vi.fn().mockResolvedValue({ mode: "cloud", serverURL: "https://core.example" }),
      getMeshStatus: vi.fn().mockResolvedValue({ cloudBaseUrl: "https://core.example" }),
      getBackendAuthHeaders: vi.fn().mockResolvedValue({}),
    } });
    fetchMock.mockReset().mockImplementation(async () => ({ ok: true, json: async () => ({ policy: { coordinated } }) }));
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => { delete (window as any).amitiaDesktop; vi.unstubAllGlobals(); clearRuntimeCache(); });

  it("毫秒内 OFF→ON→OFF 每次角色读写均使用当前模式", async () => {
    expect(await getApiBaseURLForPath("/api/characters/authority")).toBe(LOCAL_DEVICE_RUNTIME_BASE_URL);
    coordinated = true;
    expect(await getApiBaseURLForPath("/api/characters/same-role")).toBe(LOCAL_DEVICE_RUNTIME_BASE_URL + "/internal/device-mesh/provider");
    coordinated = false;
    expect(await getApiBaseURLForPath("/api/companion/role-profile")).toBe(LOCAL_DEVICE_RUNTIME_BASE_URL);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("查询模式途中切换 Core，旧响应无法决定后续角色请求", async () => {
    let finish!: (response: unknown) => void;
    fetchMock.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const routing = getApiBaseURLForPath("/api/characters/same-role");
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    clearRuntimeCache();
    finish({ ok: true, json: async () => ({ policy: { coordinated: false } }) });
    await expect(routing).rejects.toThrow("Core 已切换");
  });
});
