import { afterEach, expect, it, vi } from "vitest";

vi.mock("./local-agent-client", () => ({
  getMeshCloudAuth: vi.fn().mockResolvedValue(null),
  getLocalAdminHeaders: vi.fn().mockReturnValue({}),
  signMeshPairingClaim: vi.fn().mockResolvedValue({ signature: "proof" }),
}));
vi.mock("../backend-session-client", () => ({
  getLocalAdminHeaders: vi.fn().mockReturnValue({}),
}));

import { claimPairing } from "./remote-bootstrap-client";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function eventResponse(body: string): Response {
  const encoded = new TextEncoder().encode(body);
  return new Response(
    new ReadableStream({
      start(controller) {
        controller.enqueue(encoded);
        controller.close();
      },
    }),
    {
      status: 200,
      headers: { "content-type": "text/event-stream" },
    },
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

it("审批事件流中断后重新订阅并领取票据", async () => {
  const responses = [
    jsonResponse(200, { spaceId: "space-a" }),
    jsonResponse(202, { requestId: "request-a" }),
    eventResponse(
      `event: pairing_pending\ndata: {"state":"pending","expiresAt":"${new Date(Date.now() + 60_000).toISOString()}"}\n\n`,
    ),
    eventResponse(
      'event: pairing_approved\ndata: {"state":"approved"}\n\n',
    ),
    jsonResponse(200, { ticket: "bootstrap-a", spaceId: "space-a" }),
  ];
  const fetchMock = vi.fn().mockImplementation(async () => responses.shift());
  vi.stubGlobal("fetch", fetchMock);

  const result = await claimPairing("http://127.0.0.1:18899", {
    deviceId: "device-a",
    runtimeId: "runtime-a",
    platform: "macos",
    offerToken: "offer-a",
  });

  expect(result.ticket).toBe("bootstrap-a");
  expect(fetchMock).toHaveBeenCalledTimes(5);
});
