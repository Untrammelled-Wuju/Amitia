import { expect, it, vi } from "vitest";

const desktopAuthHeaders = vi.hoisted(() => vi.fn());
const identity = vi.hoisted(() => vi.fn());

vi.mock("./backend-session-client", () => ({
  getDesktopAuthHeaders: desktopAuthHeaders,
  getLocalAdminHeaders: vi.fn(),
}));
vi.mock("./device-mesh/local-agent-client", () => ({
  getMeshCloudAuth: vi.fn().mockResolvedValue(null),
  getMeshIdentity: identity,
}));
vi.mock("./core-endpoint", () => ({ getCoreBaseURL: () => "http://127.0.0.1:18899" }));

import { BusinessCoreClient } from "./business-core-client";
import { getMeshCloudAuth } from "./device-mesh/local-agent-client";

it("waits for desktop session headers before calling the business core", async () => {
  desktopAuthHeaders.mockResolvedValue({
    "X-Amitia-Desktop-Session": "session-token",
    "X-Amitia-Desktop-Instance": "desktop-instance",
  });
  identity.mockResolvedValue({ deviceId: "desktop-device", runtimeId: "runtime", platform: "darwin" });

  const headers = await new BusinessCoreClient("http://127.0.0.1:18899").authHeaders();

  expect(headers).toEqual({
    "X-Amitia-Desktop-Session": "session-token",
    "X-Amitia-Desktop-Instance": "desktop-instance",
    "X-Amitia-Device-ID": "desktop-device",
  });
});

it("resolves cloud authorization once per business request", async () => {
  const cloudAuth = vi.mocked(getMeshCloudAuth);
  cloudAuth.mockClear();
  const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200 });
  vi.stubGlobal("fetch", fetchMock);
  cloudAuth.mockResolvedValue({
    authorization: "AmitiaDevice token",
    cloudBaseUrl: "http://127.0.0.1:18899",
    spaceId: "space",
    deviceId: "device",
    runtimeId: "runtime",
    expiresAt: new Date(Date.now() + 60_000).toISOString(),
  });

  const response = await new BusinessCoreClient("http://127.0.0.1:18899").fetch("/api/health");

  expect(response).toEqual({ ok: true, status: 200 });
  expect(fetchMock).toHaveBeenCalledOnce();
  expect(cloudAuth).toHaveBeenCalledOnce();
  vi.unstubAllGlobals();
});

it("does not probe cloud authorization for a local center client", async () => {
  const cloudAuth = vi.mocked(getMeshCloudAuth);
  cloudAuth.mockClear();
  const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200 });
  vi.stubGlobal("fetch", fetchMock);

  await new BusinessCoreClient("http://127.0.0.1:18899", false).fetch("/api/health");

  expect(cloudAuth).not.toHaveBeenCalled();
  vi.unstubAllGlobals();
});
