import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  baseURL: vi.fn(),
  headers: vi.fn(),
  deployment: vi.fn(),
  credential: vi.fn(),
  meshIdentity: vi.fn(),
}));

vi.mock("@/runtime/runtime-adapter", () => ({
  getApiBaseURL: mocks.baseURL,
  getBackendAuthHeaders: mocks.headers,
  getDeploymentConfig: mocks.deployment,
}));

vi.mock("@/runtime/web-device-mesh", () => ({
  getWebDeviceCredential: mocks.credential,
  getWebDeviceMeshIdentity: mocks.meshIdentity,
}));

import { resolveUIHostDeviceId } from "../ui-runtime/deviceIdentity";

beforeEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  mocks.baseURL.mockResolvedValue("http://local.test");
  mocks.headers.mockResolvedValue({});
  mocks.deployment.mockResolvedValue({ mode: "local" });
  mocks.credential.mockReturnValue(null);
  mocks.meshIdentity.mockReturnValue({ deviceId: "dev_web_random", runtimeId: "rt_web", platform: "web" });
});

describe("UI host device identity", () => {
  it("local Web uses the authenticated Core device instead of a random browser identity", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      data: { policy: { deviceId: "dev_local" }, coreConsoleDeviceId: "dev_local" },
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(resolveUIHostDeviceId()).resolves.toBe("dev_local");
    expect(fetchMock).toHaveBeenCalledWith(
      "http://local.test/api/device-mesh/v1/coordination/me",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(mocks.meshIdentity).not.toHaveBeenCalled();
  });

  it("paired Cloud Web uses the credential device identity", async () => {
    mocks.deployment.mockResolvedValue({ mode: "cloud", serverURL: "https://cloud.test" });
    mocks.credential.mockReturnValue({ deviceId: "dev_cloud" });
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    await expect(resolveUIHostDeviceId()).resolves.toBe("dev_cloud");
    expect(fetchMock).not.toHaveBeenCalled();
    expect(mocks.meshIdentity).not.toHaveBeenCalled();
  });

  it("local identity lookup reports the complete Core failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(
      JSON.stringify({ code: 403, msg: "device identity mismatch" }),
      { status: 403, headers: { "Content-Type": "application/json" } },
    )));

    await expect(resolveUIHostDeviceId()).rejects.toThrow(
      '读取本机设备身份失败: HTTP 403 {"code":403,"msg":"device identity mismatch"}',
    );
  });
});
