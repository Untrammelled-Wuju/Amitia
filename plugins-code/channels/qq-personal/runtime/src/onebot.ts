import { createHash, randomUUID, timingSafeEqual } from "node:crypto"
import type { IncomingMessage } from "node:http"
import type { Duplex } from "node:stream"
import WebSocket, { WebSocketServer } from "ws"
import type { QQPersonalConfig } from "./config.js"

export interface OneBotEvent {
  post_type?: string
  message_type?: "private" | "group"
  sub_type?: string
  self_id?: number | string
  user_id?: number | string
  group_id?: number | string
  message_id?: number | string
  message?: unknown
  raw_message?: string
  sender?: Record<string, unknown>
  to_me?: boolean
  [key: string]: unknown
}

interface OneBotResponse {
  status?: string
  retcode?: number
  data?: unknown
  echo?: string
  message?: string
  wording?: string
}

interface PendingAction {
  socket: WebSocket
  resolve: (value: unknown) => void
  reject: (reason: Error) => void
  timer: ReturnType<typeof setTimeout>
}

export interface BridgeConnectionStatus {
  connected: boolean
  clients: number
  accountIds: string[]
  connectedAt: string
  lastSeenAt: string
}

function tokenDigest(value: string): Buffer {
  return createHash("sha256").update(value).digest()
}

function tokensEqual(actual: string, expected: string): boolean {
  if (!actual || !expected) return false
  return timingSafeEqual(tokenDigest(actual), tokenDigest(expected))
}

function extractToken(request: IncomingMessage, url: URL): string {
  const authorization = String(request.headers.authorization || "").trim()
  const bearer = authorization.match(/^Bearer\s+(.+)$/i)?.[1] || ""
  const headerToken = String(request.headers["x-onebot-token"] || request.headers["x-access-token"] || "").trim()
  return bearer || headerToken || url.searchParams.get("access_token") || url.searchParams.get("token") || ""
}

export class OneBotBridge {
  private wss = new WebSocketServer({ noServer: true })
  private sockets = new Set<WebSocket>()
  private pending = new Map<string, PendingAction>()
  private accountBySocket = new Map<WebSocket, string>()
  private connectedAt = ""
  private lastSeenAt = ""
  private handler: ((event: OneBotEvent) => Promise<void>) | null = null
  private configProvider: () => QQPersonalConfig
  private pathProvider: () => string

  constructor(configProvider: () => QQPersonalConfig) {
    this.configProvider = configProvider
    this.pathProvider = () => configProvider().websocketPath
    this.wss.on("connection", (socket) => this.accept(socket))
  }

  onEvent(handler: (event: OneBotEvent) => Promise<void>): void {
    this.handler = handler
  }

  canAccept(): boolean {
    return this.configProvider().enabled
  }

  handleUpgrade(request: IncomingMessage, socket: Duplex, head: Buffer): void {
    const config = this.configProvider()
    const url = new URL(request.url || "/", `http://${request.headers.host || "127.0.0.1"}`)
    if (url.pathname !== this.pathProvider()) {
      socket.write("HTTP/1.1 404 Not Found\r\nConnection: close\r\n\r\n")
      socket.destroy()
      return
    }
    if (!config.enabled) {
      socket.write("HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\n\r\n")
      socket.destroy()
      return
    }
    if (!tokensEqual(extractToken(request, url), config.token)) {
      socket.write("HTTP/1.1 401 Unauthorized\r\nConnection: close\r\n\r\n")
      socket.destroy()
      return
    }
    this.wss.handleUpgrade(request, socket, head, (websocket) => {
      this.wss.emit("connection", websocket, request)
    })
  }

  private accept(socket: WebSocket): void {
    this.sockets.add(socket)
    if (!this.connectedAt) this.connectedAt = new Date().toISOString()
    socket.on("message", (data) => {
      this.handleMessage(socket, data.toString())
    })
    socket.on("close", () => {
      this.sockets.delete(socket)
      this.accountBySocket.delete(socket)
      this.failPending(new Error("OneBot WebSocket 已断开"), socket)
      if (!this.sockets.size) this.connectedAt = ""
    })
    socket.on("error", () => {
      this.sockets.delete(socket)
      this.accountBySocket.delete(socket)
      this.failPending(new Error("OneBot WebSocket 连接异常"), socket)
      if (!this.sockets.size) this.connectedAt = ""
    })
    void this.callActionOnSocket(socket, "get_login_info", {})
      .then((data) => {
        const accountId = String((data as Record<string, unknown>)?.user_id || "")
        if (accountId && this.sockets.has(socket)) this.accountBySocket.set(socket, accountId)
      })
      .catch((error) => {
  console.error("[original-error]", error);
})
  }

  private handleMessage(socket: WebSocket, raw: string): void {
    let payload: OneBotEvent | OneBotResponse
    try {
      payload = JSON.parse(raw)
    } catch (error) {
  console.error("[original-error]", error);
return
    }
    this.lastSeenAt = new Date().toISOString()
    const echo = String((payload as OneBotResponse).echo || "")
    if (echo) {
      const pending = this.pending.get(echo)
      if (!pending || pending.socket !== socket) return
      clearTimeout(pending.timer)
      this.pending.delete(echo)
      const response = payload as OneBotResponse
      if (response.retcode !== undefined && response.retcode !== 0) {
        pending.reject(new Error(response.wording || response.message || `OneBot action failed: ${response.retcode}`))
        return
      }
      if (response.status === "failed") {
        pending.reject(new Error(response.wording || response.message || "OneBot action failed"))
        return
      }
      pending.resolve(response.data ?? response)
      return
    }
    const event = payload as OneBotEvent
    const selfId = String(event.self_id || "")
    if (selfId) this.accountBySocket.set(socket, selfId)
    if (this.handler && event.post_type === "message") {
      void this.handler(event).catch((error) => {
  console.error("[original-error]", error);
})
    }
  }

  async callAction(action: string, params: Record<string, unknown>, timeoutMs = 15000): Promise<unknown> {
    const socket = this.activeSocket()
    if (!socket) throw new Error("SnowLuma OneBot 尚未连接")
    return this.callActionOnSocket(socket, action, params, timeoutMs)
  }

  private async callActionOnSocket(socket: WebSocket, action: string, params: Record<string, unknown>, timeoutMs = 15000): Promise<unknown> {
    const echo = randomUUID()
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(echo)
        reject(new Error(`OneBot action timeout: ${action}`))
      }, timeoutMs)
      this.pending.set(echo, { socket, resolve, reject, timer })
      try {
        socket.send(JSON.stringify({ action, params, echo }), (error) => {
          if (!error) return
          clearTimeout(timer)
          this.pending.delete(echo)
          reject(error)
        })
      } catch (error) {
        clearTimeout(timer)
        this.pending.delete(echo)
        reject(error instanceof Error ? error : new Error(String(error)))
      }
    })
  }

  async sendMessage(kind: "group" | "private", targetId: string, message: unknown[]): Promise<unknown> {
    const numericTarget = Number(targetId)
    if (!Number.isSafeInteger(numericTarget) || numericTarget <= 0) {
      throw new Error("QQ 群号或用户号无效")
    }
    const action = kind === "group" ? "send_group_msg" : "send_private_msg"
    const key = kind === "group" ? "group_id" : "user_id"
    return this.callAction(action, { [key]: numericTarget, message })
  }

  status(): BridgeConnectionStatus {
    const accountIds = Array.from(new Set(Array.from(this.accountBySocket.values()).filter(Boolean)))
    return {
      connected: this.sockets.size > 0,
      clients: this.sockets.size,
      accountIds,
      connectedAt: this.connectedAt,
      lastSeenAt: this.lastSeenAt,
    }
  }

  disconnectClients(): void {
    this.failPending(new Error("OneBot 连接已由 Amitia 断开"))
    for (const socket of this.sockets) {
      socket.close(1000, "Amitia disconnect")
    }
    this.sockets.clear()
    this.accountBySocket.clear()
    this.connectedAt = ""
  }

  close(): void {
    this.disconnectClients()
    this.wss.close()
  }

  private activeSocket(): WebSocket | undefined {
    return Array.from(this.sockets).reverse().find((socket) => socket.readyState === WebSocket.OPEN)
  }

  private failPending(error: Error, socket?: WebSocket): void {
    for (const [echo, pending] of this.pending) {
      if (socket && pending.socket !== socket) continue
      clearTimeout(pending.timer)
      pending.reject(error)
      this.pending.delete(echo)
    }
  }
}
