import { afterEach, beforeEach, describe, expect, it } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import {
  coreEndpointPath,
  coreEndpointIdentity,
  getCoreEndpoint,
  getCoreEndpointEpoch,
  invalidateCoreEndpoint,
  isCoreEndpointOwnedByProcess,
  parseCoreEndpoint,
  readCoreEndpoint,
  removeCoreEndpoint,
  setCoreEndpoint,
  waitForCoreEndpoint,
} from "./core-endpoint";

let dataDir = "";

function writeEndpoint(overrides: Record<string, unknown> = {}): void {
  const endpoint = {
    schemaVersion: 1,
    version: "26.2.0-beta.8",
    instanceId: "space-test",
    pid: 4321,
    startedAt: new Date().toISOString(),
    host: "127.0.0.1",
    port: 59922,
    httpBaseURL: "http://127.0.0.1:59922",
    websocketBaseURL: "ws://127.0.0.1:59922",
    ...overrides,
  };
  fs.mkdirSync(path.dirname(coreEndpointPath(dataDir)), { recursive: true });
  fs.writeFileSync(coreEndpointPath(dataDir), JSON.stringify(endpoint), "utf8");
}

describe("core endpoint lifecycle", () => {
  beforeEach(() => {
    dataDir = fs.mkdtempSync(path.join(os.tmpdir(), "amitia-endpoint-"));
    setCoreEndpoint(null);
  });

  afterEach(() => {
    setCoreEndpoint(null);
    fs.rmSync(dataDir, { recursive: true, force: true });
  });

  it("读取残留端点不会把它设为当前端点", () => {
    writeEndpoint();

    const endpoint = readCoreEndpoint(dataDir);

    expect(endpoint?.port).toBe(59922);
    expect(getCoreEndpoint()).toBeNull();
  });

  it("只接受 PID 与启动时间都匹配的端点", () => {
    const startedAt = new Date().toISOString();
    const endpoint = {
      schemaVersion: 1,
      pid: 4321,
      startedAt,
      host: "127.0.0.1",
      port: 59922,
      httpBaseURL: "http://127.0.0.1:59922",
      websocketBaseURL: "ws://127.0.0.1:59922",
    };

    expect(isCoreEndpointOwnedByProcess(endpoint, 4321, Date.now() - 1000)).toBe(true);
    expect(isCoreEndpointOwnedByProcess(endpoint, 9999, Date.now() - 1000)).toBe(false);
    expect(isCoreEndpointOwnedByProcess(endpoint, 4321, Date.now() + 5000)).toBe(false);
  });

  it("只删除属于指定 PID 的端点文件", () => {
    writeEndpoint({ pid: 4321 });

    expect(removeCoreEndpoint(dataDir, 9999)).toBe(false);
    expect(fs.existsSync(coreEndpointPath(dataDir))).toBe(true);

    expect(removeCoreEndpoint(dataDir, 4321)).toBe(true);
    expect(fs.existsSync(coreEndpointPath(dataDir))).toBe(false);
  });

  it("拒绝非回环地址的端点", () => {
    writeEndpoint({
      host: "192.168.1.10",
      httpBaseURL: "http://192.168.1.10:59922",
      websocketBaseURL: "ws://192.168.1.10:59922",
    });

    expect(readCoreEndpoint(dataDir)).toBeNull();
  });

  it("等待中的请求只在新端点就绪后返回", async () => {
    const pending = waitForCoreEndpoint(1000);
    const endpoint = parseCoreEndpoint({
      schemaVersion: 1,
      pid: 4321,
      startedAt: new Date().toISOString(),
      host: "127.0.0.1",
      port: 59922,
      httpBaseURL: "http://127.0.0.1:59922",
      websocketBaseURL: "ws://127.0.0.1:59922",
    });

    if (!endpoint) {
      throw new Error("endpoint parse failed");
    }
    setCoreEndpoint(endpoint);

    await expect(pending).resolves.toMatchObject({ port: 59922 });
  });

  it("核心端点长时间不可用时返回明确错误", async () => {
    await expect(waitForCoreEndpoint(10)).rejects.toThrow("Amitia Core 尚未就绪");
  });

  it("失效端点后等待者只接收新端点", async () => {
    const first = parseCoreEndpoint({
      schemaVersion: 1,
      instanceId: "space-first",
      pid: 4321,
      startedAt: new Date().toISOString(),
      host: "127.0.0.1",
      port: 59922,
      httpBaseURL: "http://127.0.0.1:59922",
      websocketBaseURL: "ws://127.0.0.1:59922",
    });
    const second = parseCoreEndpoint({
      schemaVersion: 1,
      instanceId: "space-second",
      pid: 4322,
      startedAt: new Date().toISOString(),
      host: "127.0.0.1",
      port: 59923,
      httpBaseURL: "http://127.0.0.1:59923",
      websocketBaseURL: "ws://127.0.0.1:59923",
    });
    if (!first || !second) {
      throw new Error("endpoint parse failed");
    }

    setCoreEndpoint(first);
    const epoch = getCoreEndpointEpoch();
    invalidateCoreEndpoint();

    expect(getCoreEndpoint()).toBeNull();
    expect(getCoreEndpointEpoch()).toBeGreaterThan(epoch);

    const pending = waitForCoreEndpoint(1000);
    setCoreEndpoint(second);

    await expect(pending).resolves.toMatchObject({ port: 59923 });
    expect(coreEndpointIdentity(second)).toContain("space-second");
  });
});
