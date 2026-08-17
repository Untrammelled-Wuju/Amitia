import { clipboard, type BrowserWindow } from "electron";
import { getLocalAdminHeaders } from "./backend-session-client";

const CORE_HOST = "127.0.0.1";
const CORE_PORT = 18899;
const SSE_PATH = "/api/proactive-sse";
const CLIPBOARD_RESPONSE_PATH = "/api/extensions/ui/clipboard-response";
const CLIENT_ID = "electron-main-clipboard";
const RECONNECT_INTERVAL = 5000;
const MAX_TEXT_SIZE = 1 * 1024 * 1024;

interface ClipboardRequestPayload {
  requestId: string;
  operation: string;
  text?: string;
}

export class ClipboardBridge {
  private mainWindow: BrowserWindow;
  private stopped = true;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private abortController: AbortController | null = null;

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
  }

  private async getAuthHeaders(): Promise<Record<string, string> | null> {
    if (this.mainWindow.isDestroyed()) return null;
    try {
      return await getLocalAdminHeaders();
    } catch {
      return null;
    }
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
    if (!requestId || !operation) {
      console.warn("[ClipboardBridge] clipboard_request 缺少必要字段");
      return;
    }

    try {
      if (operation === "write") {
        const writeText = text || "";
        if (writeText.length > MAX_TEXT_SIZE) {
          await this.respond(headers, requestId, "", "clipboard text exceeds maximum size");
          return;
        }
        clipboard.writeText(writeText);
        await this.respond(headers, requestId, "", null);
      } else if (operation === "read") {
        const clipText = clipboard.readText();
        const truncated = clipText.length > MAX_TEXT_SIZE
          ? clipText.slice(0, MAX_TEXT_SIZE)
          : clipText;
        await this.respond(headers, requestId, truncated, null);
      } else {
        await this.respond(headers, requestId, "", `unsupported operation: ${operation}`);
      }
    } catch (err) {
      await this.respond(headers, requestId, "", String(err));
    }
  }

  private async respond(
    headers: Record<string, string>,
    requestId: string,
    text: string,
    error: string | null,
  ): Promise<void> {
    try {
      const body = error
        ? JSON.stringify({ requestId, error })
        : JSON.stringify({ requestId, text });

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
