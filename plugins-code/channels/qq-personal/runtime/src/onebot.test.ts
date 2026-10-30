import assert from "node:assert/strict"
import http from "node:http"
import test from "node:test"
import WebSocket from "ws"
import { OneBotBridge } from "./onebot.js"
import type { QQPersonalConfig } from "./config.js"

const baseConfig: QQPersonalConfig = {
  enabled: true,
  listenHost: "127.0.0.1",
  listenPort: 6199,
  websocketPath: "/ws",
  token: "onebot-token",
  groupTrigger: "mention",
  triggerPrefix: "Amitia",
  ignoredUserIds: [],
  runtimeMode: "docker",
  snowlumaRoot: "",
  webuiPort: 5099,
  novncPort: 6081,
  containerName: "snowluma",
  dockerBin: "",
}

async function bridgeServer(bridge: OneBotBridge): Promise<{ server: http.Server; url: string }> {
  const server = http.createServer((_request, response) => {
    response.statusCode = 404
    response.end()
  })
  server.on("upgrade", (request, socket, head) => {
    bridge.handleUpgrade(request, socket, head)
  })
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject)
    server.listen(0, "127.0.0.1", resolve)
  })
  const address = server.address()
  assert.ok(address && typeof address !== "string")
  return { server, url: `ws://127.0.0.1:${address.port}` }
}

async function expectRejected(url: string, status: number): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    const socket = new WebSocket(url)
    socket.once("open", () => {
      socket.close()
      reject(new Error("expected WebSocket rejection"))
    })
    socket.once("unexpected-response", (_request, response) => {
      if (response.statusCode !== status) {
        reject(new Error(`unexpected HTTP ${response.statusCode}`))
      } else {
        resolve()
      }
      response.resume()
    })
    socket.once("error", (error) => reject(error))
  })
}

function actionServer(socket: WebSocket, calls: Array<{ action: string; params: Record<string, unknown> }>): void {
  socket.on("message", (raw) => {
    const payload = JSON.parse(raw.toString()) as {
      action: string
      params: Record<string, unknown>
      echo?: string
    }
    calls.push({ action: payload.action, params: payload.params })
    if (!payload.echo) return
    const data = payload.action === "get_login_info"
      ? { user_id: 2743073290 }
      : { message_id: 88 }
    socket.send(JSON.stringify({ status: "ok", retcode: 0, data, echo: payload.echo }))
  })
}

test("OneBot 反向连接校验路径、启用状态和 Token", async () => {
  let current = { ...baseConfig }
  const bridge = new OneBotBridge(() => current)
  const { server, url } = await bridgeServer(bridge)
  try {
    await expectRejected(`${url}/wrong?access_token=${current.token}`, 404)
    await expectRejected(`${url}/ws?access_token=wrong`, 401)
    current = { ...current, enabled: false }
    await expectRejected(`${url}/ws?access_token=${current.token}`, 503)
  } finally {
    bridge.close()
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})

test("OneBot 连接状态、动作调用和入站事件可用", async () => {
  const current = { ...baseConfig }
  const bridge = new OneBotBridge(() => current)
  const { server, url } = await bridgeServer(bridge)
  const calls: Array<{ action: string; params: Record<string, unknown> }> = []
  const events: unknown[] = []
  bridge.onEvent(async (event) => {
    events.push(event)
  })
  const socket = new WebSocket(`${url}/ws?access_token=${current.token}`)
  actionServer(socket, calls)
  try {
    await new Promise<void>((resolve, reject) => {
      socket.once("open", resolve)
      socket.once("error", reject)
    })
    await new Promise((resolve) => setTimeout(resolve, 10))
    const status = bridge.status()
    assert.equal(status.connected, true)
    assert.equal(status.clients, 1)
    assert.equal(status.accountIds[0], "2743073290")
    assert.ok(status.connectedAt)
    assert.ok(status.lastSeenAt)

    await bridge.callAction("get_status", {})
    assert.equal(calls.at(-1)?.action, "get_status")
    socket.send(JSON.stringify({
      post_type: "message",
      message_type: "private",
      self_id: 2743073290,
      user_id: 40004,
      message_id: 9,
      message: [{ type: "text", data: { text: "hello" } }],
    }))
    await new Promise((resolve) => setTimeout(resolve, 10))
    assert.equal(events.length, 1)
  } finally {
    socket.close()
    bridge.close()
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})

test("断开一端不会取消另一端上的待处理动作", async () => {
  const current = { ...baseConfig }
  const bridge = new OneBotBridge(() => current)
  const { server, url } = await bridgeServer(bridge)
  const callsA: Array<{ action: string; params: Record<string, unknown> }> = []
  const callsB: Array<{ action: string; params: Record<string, unknown> }> = []
  const first = new WebSocket(`${url}/ws?access_token=${current.token}`)
  actionServer(first, callsA)
  await new Promise<void>((resolve, reject) => {
    first.once("open", resolve)
    first.once("error", reject)
  })
  const second = new WebSocket(`${url}/ws?access_token=${current.token}`)
  actionServer(second, callsB)
  try {
    await new Promise<void>((resolve, reject) => {
      second.once("open", resolve)
      second.once("error", reject)
    })
    const pending = bridge.callAction("get_status", {})
    await new Promise((resolve) => setTimeout(resolve, 10))
    first.close()
    const result = await pending as Record<string, unknown>
    assert.equal(result.message_id, 88)
  } finally {
    first.close()
    second.close()
    bridge.close()
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})
