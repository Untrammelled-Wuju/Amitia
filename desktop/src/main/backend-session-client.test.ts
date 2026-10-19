import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import http from "node:http";
import type { AddressInfo } from "node:net";
import type { CoreEndpoint } from "./core-endpoint";

const state = vi.hoisted(() => ({
  endpoint: null as CoreEndpoint | null,
  fresh: null as CoreEndpoint | null,
  invalidations: 0,
}));

vi.mock("./path-manager", () => ({
  getBackendDataDir: () => "",
  ensureLocalToken: () => "",
}));

vi.mock("./desktop-identity", () => ({
  getDesktopInstanceID: () => "test-desktop",
}));

vi.mock("./core-endpoint", async () => {
  const actual = await vi.importActual<typeof import("./core-endpoint")>("./core-endpoint");
  return {
    ...actual,
    waitForCoreEndpoint: vi.fn(async () => {
      if (!state.endpoint) {
        throw new Error("Amitia Core 尚未就绪");
      }
      return state.endpoint;
    }),
    invalidateCoreEndpoint: vi.fn(() => {
      state.invalidations += 1;
      state.endpoint = state.fresh;
    }),
  };
});

import { requestJSON } from "./backend-session-client";

function endpoint(port: number, instanceId: string): CoreEndpoint {
  return {
    schemaVersion: 1,
    instanceId,
    pid: port,
    startedAt: "2026-10-19T00:00:00Z",
    host: "127.0.0.1",
    port,
    httpBaseURL: `http://127.0.0.1:${port}`,
    websocketBaseURL: `ws://127.0.0.1:${port}`,
  };
}

async function closedPort(exclude: number): Promise<number> {
  for (let attempt = 0; attempt < 10; attempt += 1) {
    const server = http.createServer();
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", () => resolve()));
    const port = (server.address() as AddressInfo).port;
    await new Promise<void>((resolve) => server.close(() => resolve()));
    if (port !== exclude) {
      return port;
    }
  }
  throw new Error("无法分配关闭端口");
}

describe("backend session core endpoint retry", () => {
  let server: http.Server | null = null;
  let freshPort = 0;

  beforeEach(async () => {
    state.endpoint = null;
    state.fresh = null;
    state.invalidations = 0;
    server = http.createServer((req, res) => {
      req.resume();
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        sessionToken: "token",
        expiresAt: new Date(Date.now() + 3_600_000).toISOString(),
      }));
    });
    await new Promise<void>((resolve) => server?.listen(0, "127.0.0.1", () => resolve()));
    freshPort = (server.address() as AddressInfo).port;
  });

  afterEach(async () => {
    if (server) {
      await new Promise<void>((resolve) => server?.close(() => resolve()));
      server = null;
    }
  });

  it("retries on ECONNREFUSED after switching to the fresh endpoint", async () => {
    const stalePort = await closedPort(freshPort);
    state.endpoint = endpoint(stalePort, "space-stale");
    state.fresh = endpoint(freshPort, "space-fresh");

    const result = await requestJSON<{ sessionToken: string }>(
      "POST",
      "/api/local/sessions",
      {},
      {},
    );

    expect(result.sessionToken).toBe("token");
    expect(state.invalidations).toBe(1);
  });
});
