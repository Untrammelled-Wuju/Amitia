import { clipboard, type BrowserWindow } from "electron";
import { getLocalAdminHeaders } from "./backend-session-client";
import { getDeviceId } from "./pet/runtime-identity";

const CORE_HOST = "127.0.0.1";
const CORE_PORT = 18899;
const SSE_PATH = "/api/proactive-sse";
const CLIPBOARD_RESPONSE_PATH = "/api/extensions/ui/clipboard-response";
const CLIENT_ID = "electron-main-clipboard";
const RECONNECT_INTERVAL = 5000;
const MAX_TEXT_SIZE = 1 * 1024 * 1024;
const HOST_SESSION_PATH = "/api/extensions/ui/host-session";
const HOST_HEARTBEAT_PATH = "/api/extensions/ui/host-session/heartbeat";
const HOST_DISCONNECT_PATH = "/api/extensions/ui/host-session/disconnect";
const DEFAULT_HEARTBEAT_SECONDS = 60;

interface ClipboardRequestPayload {
  requestId: string;
  operation: string;
  text?: string;
  hostClientId?: string;
  hostSessionId?: string;
}

interface HostSessionResponse {
  hostClientId: string;
  hostSessionId: string;
  heartbeatIntervalSeconds?: number;
}

export class ClipboardBridge {
  private mainWindow: BrowserWindow;
  private stopped = true;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private abortController: AbortController | null = null;
  private hostSessionId = "";
  private heartbeatTimer: NodeJS.Timeout | null = null;

  constructor(mainWindow: BrowserWindow) {
    this.mainWindow = mainWindow;
  }

  start(): void {
    this.stopped = false;
    void this.connect();
  }

  stop(): void {
    this.stopped = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
    this.stopHeartbeat();
    void this.disconnectHostSession();
  }

  private async getAuthHeaders(): Promise<Record<string, string> | null> {
    if (this.mainWindow.isDestroyed()) return null;
    try {
      return await getLocalAdminHeaders();
    } catch {
      return null;
    }
  }

  private platformName(): string {
    switch (process.platform) {
      case "win32":
        return "windows";
      case "darwin":
        return "darwin";
      case "linux":
        return "linux";
      default:
        return "";
    }
  }

  private async postHostSession<T>(
    path: string,
    headers: Record<string, string>,
    body: Record<string, unknown>,
  ): Promise<T> {
    const response = await fetch(`http://${CORE_HOST}:${CORE_PORT}${path}`, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        ...headers,
      },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      throw new Error(`host session request failed: ${response.status}`);
    }
    return await response.json() as T;
  }

  private async registerHostSession(headers: Record<string, string>): Promise<void> {
    const result = await this.postHostSession<HostSessionResponse>(
      HOST_SESSION_PATH,
      headers,
      {
        hostClientId: CLIENT_ID,
        deviceId: getDeviceId(),
        platform: this.platformName(),
        windowId: "main",
        features: ["clipboard.read", "clipboard.write"],
      },
    );
    const sessionId = String(result.hostSessionId ?? "").trim();
    if (!sessionId) {
      throw new Error("clipboard host registration returned no hostSessionId");
    }
    this.hostSessionId = sessionId;
    this.stopHeartbeat();
    const seconds = Math.max(20, Number(result.heartbeatIntervalSeconds ?? DEFAULT_HEARTBEAT_SECONDS) || DEFAULT_HEARTBEAT_SECONDS);
    this.heartbeatTimer = setInterval(() => {
      if (this.stopped || !this.hostSessionId) return;
      void this.postHostSession(
        HOST_HEARTBEAT_PATH,
        headers,
        { hostClientId: CLIENT_ID, hostSessionId: this.hostSessionId },
      ).catch(() => {
        // The SSE reconnect path will register a fresh host session.
      });
    }, seconds * 1000);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private async disconnectHostSession(): Promise<void> {
    const sessionId = this.hostSessionId;
    this.hostSessionId = "";
    if (!sessionId) return;
    const headers = await this.getAuthHeaders();
    if (!headers) return;
    await this.postHostSession(
      HOST_DISCONNECT_PATH,
      headers,
      { hostClientId: CLIENT_ID, hostSessionId: sessionId },
    ).catch(() => {});
  }

  private async connect(): Promise<void> {
    if (this.stopped) return;

    const headers = await this.getAuthHeaders();
    if (!headers) {
      this.scheduleReconnect();
      return;
    }

    this.abortController = new AbortController();

    try {
      const response = await fetch(
        `http://${CORE_HOST}:${CORE_PORT}${SSE_PATH}?clientId=${CLIENT_ID}`,
        {
          headers: {
            Accept: "text/event-stream",
            ...headers,
            "Cache-Control": "no-cache",
          },
          signal: this.abortController.signal,
        },
      );

      if (!response.ok || !response.body) {
        console.warn(`[ClipboardBridge] SSE 连接失败: ${response.status}`);
        this.scheduleReconnect();
        return;
      }

      await this.registerHostSession(headers);
      console.log("[ClipboardBridge] SSE 连接成功，开始监听 clipboard_request 事件");

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (!this.stopped) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const events = buffer.split("\n\n");
        buffer = events.pop() || "";

        for (const eventStr of events) {
          this.parseAndHandleEvent(eventStr, headers);
        }
      }
    } catch (err) {
      if (!this.stopped) {
        console.warn("[ClipboardBridge] SSE 连接异常:", err);
      }
    }

    if (!this.stopped) {
      this.stopHeartbeat();
      this.hostSessionId = "";
      this.scheduleReconnect();
    }
  }

  private parseAndHandleEvent(eventStr: string, headers: Record<string, string>): void {
    const lines = eventStr.split("\n");
    let eventName = "";
    let dataStr = "";

    for (const line of lines) {
      if (line.startsWith("event:")) {
        eventName = line.slice(6).trim();
      } else if (line.startsWith("data:")) {
        dataStr += line.slice(5).trim();
      }
    }

    if (eventName === "clipboard_request" && dataStr) {
      void this.handleClipboardRequest(dataStr, headers);
    }
  }

  private async handleClipboardRequest(
    dataStr: string,
    headers: Record<string, string>,
  ): Promise<void> {
    let payload: ClipboardRequestPayload;
    try {
      payload = JSON.parse(dataStr);
    } catch {
      console.warn("[ClipboardBridge] 无法解析 clipboard_request 数据");
      return;
    }

    const { requestId, operation, text } = payload;
    const responseHostClientId = String(payload.hostClientId ?? CLIENT_ID).trim();
    const responseHostSessionId = String(payload.hostSessionId ?? this.hostSessionId).trim();
    if (!requestId || !operation) {
      console.warn("[ClipboardBridge] clipboard_request 缺少必要字段");
      return;
    }

    try {
      if (operation === "write") {
        const writeText = text || "";
        if (writeText.length > MAX_TEXT_SIZE) {
          await this.respond(headers, requestId, "", "clipboard text exceeds maximum size", responseHostClientId, responseHostSessionId);
          return;
        }
        clipboard.writeText(writeText);
        await this.respond(headers, requestId, "", null, responseHostClientId, responseHostSessionId);
      } else if (operation === "read") {
        const clipText = clipboard.readText();
        const truncated = clipText.length > MAX_TEXT_SIZE
          ? clipText.slice(0, MAX_TEXT_SIZE)
          : clipText;
        await this.respond(headers, requestId, truncated, null, responseHostClientId, responseHostSessionId);
      } else {
        await this.respond(headers, requestId, "", `unsupported operation: ${operation}`, responseHostClientId, responseHostSessionId);
      }
    } catch (err) {
      await this.respond(headers, requestId, "", String(err), responseHostClientId, responseHostSessionId);
    }
  }

  private async respond(
    headers: Record<string, string>,
    requestId: string,
    text: string,
    error: string | null,
    hostClientId: string,
    hostSessionId: string,
  ): Promise<void> {
    try {
      const identity = { hostClientId, hostSessionId };
      const body = error
        ? JSON.stringify({ requestId, error, ...identity })
        : JSON.stringify({ requestId, text, ...identity });

      const response = await fetch(
        `http://${CORE_HOST}:${CORE_PORT}${CLIPBOARD_RESPONSE_PATH}`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...headers,
          },
          body,
        },
      );

      if (!response.ok) {
        console.warn(`[ClipboardBridge] 回调失败: ${response.status}`);
      }
    } catch (err) {
      console.warn("[ClipboardBridge] 回调异常:", err);
    }
  }

  private scheduleReconnect(): void {
    if (this.stopped) return;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
    }
    this.reconnectTimer = setTimeout(() => {
      void this.connect();
    }, RECONNECT_INTERVAL);
  }
}
