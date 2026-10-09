import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ clear: vi.fn(), sign: vi.fn(async (_base: string, _url: string, init: RequestInit) => init) }));
vi.mock("../runtime/web-device-mesh", () => ({
  getWebDeviceMeshIdentity: () => ({ deviceId: "device-a" }),
  getWebDeviceAuthHeaders: () => ({ Authorization: "AmitiaDevice test-only" }),
  clearWebDeviceCredential: mocks.clear,
  signWebAuthenticatedFetch: mocks.sign,
}));
import { deprovisionCurrentDeviceMesh, resetRuntimeConnectionCache } from "../runtime/runtime-adapter";

describe("解绑冻结原设备管理权限", () => {
  beforeEach(() => {
    window.amitiaDesktop = undefined;
    localStorage.setItem("amitia.web.deployment.v1", JSON.stringify({ mode: "cloud", serverURL: "https://core-b.test" }));
    mocks.clear.mockReset(); mocks.sign.mockClear();
    resetRuntimeConnectionCache();
  });
  it("先读取实际认证policy，仅用其原Core和版本撤销", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce({ ok: true, json: async () => ({ coreId: "core-b", policy: { deviceId: "device-a", providerEpoch: 2, modeRevision: 3, permissionRevision: 4 } }) }).mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ ok: true, deviceId: "device-a" }) });
    vi.stubGlobal("fetch", fetcher);
    await deprovisionCurrentDeviceMesh("https://core-b.test");
    expect(fetcher.mock.calls[0][0]).toBe("https://core-b.test/api/device-mesh/v1/coordination/me");
    expect(fetcher.mock.calls[1][1]).toMatchObject({ method: "DELETE", headers: { "X-Amitia-Expected-Core-ID": "core-b", "X-Amitia-Expected-Configuration-Policy": "2:3:4" } });
    expect(mocks.clear).toHaveBeenCalledOnce();
  });
  it("读取policy时Core切换立即拒绝且保留凭据", async () => {
    const fetcher = vi.fn(async () => {
      resetRuntimeConnectionCache();
      return { ok: true, json: async () => ({ coreId: "core-b", policy: { deviceId: "device-a", providerEpoch: 2, modeRevision: 3, permissionRevision: 4 } }) };
    });
    vi.stubGlobal("fetch", fetcher);
    await expect(deprovisionCurrentDeviceMesh("https://core-b.test")).rejects.toThrow("变化");
    expect(fetcher).toHaveBeenCalledOnce();
    expect(mocks.clear).not.toHaveBeenCalled();
  });
  it("Core明确拒绝原policy时不清除凭据或刷新重试", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce({ ok: true, json: async () => ({ data: { coreId: "core-b", policy: { deviceId: "device-a", providerEpoch: 2, modeRevision: 3, permissionRevision: 4 } } }) }).mockResolvedValueOnce({ ok: false, status: 409, json: async () => ({ message: "原权限已变化" }) });
    vi.stubGlobal("fetch", fetcher);
    await expect(deprovisionCurrentDeviceMesh("https://core-b.test")).rejects.toThrow("原权限已变化");
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(mocks.clear).not.toHaveBeenCalled();
  });
  it("policy属于另一台设备时不撤销", async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ coreId: "core-b", policy: { deviceId: "device-other", providerEpoch: 2, modeRevision: 3, permissionRevision: 4 } }) });
    vi.stubGlobal("fetch", fetcher);
    await expect(deprovisionCurrentDeviceMesh("https://core-b.test")).rejects.toThrow("无法确认");
    expect(fetcher).toHaveBeenCalledOnce();
    expect(mocks.clear).not.toHaveBeenCalled();
  });
  it("确认框的旧权限不能用重新读取的新权限自动替换", async () => {
    const fetcher = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ coreId: "core-b", canAdminister: false, policy: { deviceId: "device-a", coordinated: false, administrator: false, providerEpoch: 2, modeRevision: 3, permissionRevision: 6 } }) });
    vi.stubGlobal("fetch", fetcher);
    const intent = { apiBaseURL: "https://core-b.test", controller: new AbortController(), state: { coreId: "core-b", canAdminister: false, policy: { deviceId: "device-a", coordinated: false, administrator: false, providerEpoch: 2, modeRevision: 3, permissionRevision: 4 } } };
    await expect(deprovisionCurrentDeviceMesh("https://core-b.test", intent)).rejects.toThrow("原解绑权限已变化");
    expect(fetcher).toHaveBeenCalledOnce();
    expect(mocks.clear).not.toHaveBeenCalled();
  });
  it("错误设备的撤销回执不能清除本地绑定", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce({ ok: true, json: async () => ({ coreId: "core-b", policy: { deviceId: "device-a", providerEpoch: 2, modeRevision: 3, permissionRevision: 4 } }) }).mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ ok: true, deviceId: "another-device" }) });
    vi.stubGlobal("fetch", fetcher);
    await expect(deprovisionCurrentDeviceMesh("https://core-b.test")).rejects.toThrow("未确认");
    expect(mocks.clear).not.toHaveBeenCalled();
  });
  it("桌面解绑经已认证Source代理读取policy和撤销，不裸访问Cloud", async () => {
    const deprovision = vi.fn();
    window.amitiaDesktop = {
      getDeploymentConfig: async () => ({ mode: "cloud", serverURL: "https://core-b.test" }),
      getMeshStatus: async () => ({ cloudBaseUrl: "https://core-b.test" }),
      getMeshIdentity: async () => ({ deviceId: "device-a" }),
      getBackendAuthHeaders: async () => ({ "X-Amitia-Desktop-Session": "test-session" }),
      deprovisionMesh: deprovision,
    } as any;
    const fetcher = vi.fn().mockResolvedValueOnce({ ok: true, json: async () => ({ coreId: "core-b", policy: { deviceId: "device-a", providerEpoch: 2, modeRevision: 3, permissionRevision: 4 } }) }).mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ ok: true, deviceId: "device-a" }) });
    vi.stubGlobal("fetch", fetcher);
    await deprovisionCurrentDeviceMesh("http://127.0.0.1:18899/internal/device-mesh/provider");
    expect(fetcher.mock.calls.map(call => call[0])).toEqual([
      "http://127.0.0.1:18899/internal/device-mesh/provider/api/device-mesh/v1/coordination/me",
      "http://127.0.0.1:18899/internal/device-mesh/provider/api/device-mesh/v1/devices/device-a",
    ]);
    expect(deprovision).toHaveBeenCalledOnce();
    expect(mocks.sign).not.toHaveBeenCalled();
  });
});
