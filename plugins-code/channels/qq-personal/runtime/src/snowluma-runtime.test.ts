import assert from "node:assert/strict"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { readSnowLumaInfo } from "./snowluma-info.js"
import {
  prepareSnowLumaOneBotConfig,
  resolveReverseWebSocketTarget,
  SnowLumaRuntimeManager,
  validateNativeSnowLuma,
} from "./snowluma-runtime.js"
import type { QQPersonalConfig } from "./config.js"

function tempRoot(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "qq-personal-runtime-"))
}

function config(root: string, port = 5099): QQPersonalConfig {
  return {
    enabled: true,
    listenHost: "127.0.0.1",
    listenPort: 6199,
    websocketPath: "/ws",
    token: "plugin-token",
    groupTrigger: "mention",
    triggerPrefix: "Amitia",
    ignoredUserIds: [],
    runtimeMode: "native",
    snowlumaRoot: root,
    webuiPort: port,
    novncPort: 6081,
    containerName: "snowluma",
    dockerBin: "",
  }
}

function writeFakeSnowLuma(root: string): void {
  const nodePath = path.join(root, process.platform === "win32" ? "node.exe" : "node")
  try {
    fs.linkSync(process.execPath, nodePath)
  } catch {
    fs.copyFileSync(process.execPath, nodePath)
  }
  fs.writeFileSync(path.join(root, "index.mjs"), [
    'import fs from "node:fs"',
    'import http from "node:http"',
    'if (!process.env.SNOWLUMA_UPDATE_ROOT) throw new Error("SNOWLUMA_UPDATE_ROOT missing")',
    'fs.mkdirSync("config", { recursive: true })',
    'fs.writeFileSync("config/webui.json", JSON.stringify({ passwordHash: "0".repeat(128), passwordSalt: "0".repeat(32), mustChangePassword: true, generatedAt: new Date().toISOString(), updatedAt: new Date().toISOString() }))',
    'process.stdout.write("initial credentials: user=admin password=TempPass!42\\n")',
    'const port = Number(process.env.SNOWLUMA_WEBUI_PORT || 5099)',
    'http.createServer((request, response) => response.end("ok")).listen(port, "127.0.0.1")',
  ].join("\n"))
}

test("creates a managed native config without 3000 or 3001 listeners", () => {
  const root = tempRoot()
  const result = prepareSnowLumaOneBotConfig(root, "ws://127.0.0.1:6199/ws", "secret-token")
  const value = JSON.parse(fs.readFileSync(path.join(root, "config", "onebot.json"), "utf8"))
  assert.equal(result.created, true)
  assert.deepEqual(value.networks.httpServers, [])
  assert.deepEqual(value.networks.wsServers, [])
  assert.equal(value.networks.wsClients[0].url, "ws://127.0.0.1:6199/ws")
  assert.equal(value.networks.wsClients[0].accessToken, "secret-token")
  assert.equal(JSON.stringify(value).includes("3000"), false)
  assert.equal(JSON.stringify(value).includes("3001"), false)
})

test("updates only the plugin-managed WebSocket client", () => {
  const root = tempRoot()
  fs.mkdirSync(path.join(root, "config"), { recursive: true })
  fs.writeFileSync(path.join(root, "config", "onebot.json"), JSON.stringify({
    networks: {
      httpServers: [],
      httpClients: [],
      wsServers: [],
      wsClients: [
        { name: "amitia", enabled: true, url: "ws://127.0.0.1:6000/ws", accessToken: "old" },
        { name: "custom", enabled: true, url: "ws://127.0.0.1:6001/ws", accessToken: "keep" },
      ],
    },
  }))
  const result = prepareSnowLumaOneBotConfig(root, "ws://127.0.0.1:6199/ws", "new")
  const value = JSON.parse(fs.readFileSync(path.join(root, "config", "onebot.json"), "utf8"))
  assert.equal(result.updated, true)
  assert.equal(value.networks.wsClients[0].accessToken, "new")
  assert.equal(value.networks.wsClients[1].accessToken, "keep")
})

test("rejects an existing enabled listener on port 3000", () => {
  const root = tempRoot()
  fs.mkdirSync(path.join(root, "config"), { recursive: true })
  fs.writeFileSync(path.join(root, "config", "onebot.json"), JSON.stringify({
    networks: {
      httpServers: [{ name: "http-default", enabled: true, host: "127.0.0.1", port: 3000, path: "/" }],
      httpClients: [],
      wsServers: [],
      wsClients: [],
    },
  }))
  assert.throws(() => prepareSnowLumaOneBotConfig(root, "ws://127.0.0.1:6199/ws", "token"), /3000/)
})

test("uses the project Node and validates the entry point", () => {
  const root = tempRoot()
  fs.writeFileSync(path.join(root, "index.mjs"), "")
  assert.equal(validateNativeSnowLuma(root, root).nodeBin, process.execPath)
})

test("rejects native SnowLuma paths outside the plugin data root", () => {
  const allowed = tempRoot()
  const outside = tempRoot()
  fs.writeFileSync(path.join(outside, "index.mjs"), "")
  assert.throws(() => validateNativeSnowLuma(outside, allowed), /插件数据目录/)
})

test("uses loopback for native reverse WebSocket targets", async () => {
  const root = tempRoot()
  const native = await resolveReverseWebSocketTarget(config(root), "linux")
  assert.equal(native.url, "ws://127.0.0.1:6199/ws")
})

test("selects platform-safe Docker reverse WebSocket fallbacks", async () => {
  const root = tempRoot()
  const target = {
    ...config(root),
    runtimeMode: "docker" as const,
    dockerBin: path.join(root, "missing-docker"),
  }
  const linux = await resolveReverseWebSocketTarget(target, "linux")
  assert.equal(linux.url, "ws://127.0.0.1:6199/ws")
  assert.match(linux.warning, /host/)
  const windows = await resolveReverseWebSocketTarget(target, "win32")
  assert.equal(windows.url, "ws://host.docker.internal:6199/ws")
  assert.equal(windows.warning, "")
})

test("starts a managed native SnowLuma process and captures its bootstrap password", async () => {
  const root = tempRoot()
  writeFakeSnowLuma(root)
  const manager = new SnowLumaRuntimeManager(root)
  const next = config(root, 15999)
  let pid = 0
  try {
    const status = await manager.start(next)
    pid = status.pid
    assert.equal(status.running, true)
    assert.equal(manager.webuiBootstrapPassword(), "TempPass!42")
    const info = await readSnowLumaInfo({
      runtimeMode: "native",
      snowlumaRoot: root,
      bootstrapPassword: manager.webuiBootstrapPassword(),
      containerName: "snowluma",
      webuiPort: next.webuiPort,
      novncPort: next.novncPort,
    })
    assert.equal(info.webuiPassword, "TempPass!42")
    assert.equal(info.webuiPasswordState, "temporary")
    assert.equal(info.novncUrl, "")
  } finally {
    await manager.stop()
    assert.throws(() => process.kill(pid, 0), /ESRCH/)
  }
})
