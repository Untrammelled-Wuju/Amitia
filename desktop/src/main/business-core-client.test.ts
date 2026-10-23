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
