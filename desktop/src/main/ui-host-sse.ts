import { Notification, BrowserWindow, dialog } from "electron";
import { IPC_CHANNELS } from "../shared/ipc";
import type { BusinessCoreClient } from "./business-core-client";
import { getMeshIdentity } from "./device-mesh/local-agent-client";

interface SSEEventEnvelope {
  eventType: string;
  requestId: string;
  sessionId: string;
  extensionId: string;
  payload: Record<string, unknown>;
  expiresAt?: string;
  timestamp: string;
}

export interface UIHostSSEOptions {
  businessCore: BusinessCoreClient;
  mainWindow: () => BrowserWindow | null;
}

const MAX_RETRIES = 10;
const BASE_RECONNECT_MS = 2000;
const MAX_RECONNECT_MS = 30000;
const DEDUP_MAX_SIZE = 200;

const severityMap: Record<string, "info" | "error" | "warning" | "none"> = {
  info: "info",
  success: "info",
  warning: "warning",
  error: "error",
  critical: "error",
};

export class UIHostSSE {
  private readonly businessCore: BusinessCoreClient;
  private readonly mainWindow: () => BrowserWindow | null;
  private abortController: AbortController | null = null;
  private stopped = true;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempts = 0;
  private readonly processedRequestIds = new Set<string>();

  constructor(options: UIHostSSEOptions) {
    this.businessCore = options.businessCore;
    this.mainWindow = options.mainWindow;
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

  private async connect(): Promise<void> {
    if (this.stopped) return;

    this.abortController = new AbortController();
    const path = "/api/proactive-sse?clientId=electron-ui-host";

    try {
      const identity = await getMeshIdentity();
      if (identity?.deviceId) {
        const response = await this.businessCore.fetch('/api/notifications/devices');
        if (!response.ok) throw new Error('桌面通知配置读取失败');
        const body = await response.json() as { data?: Array<{ deviceId: string; platform: string }> };
        if (!Array.isArray(body.data)) throw new Error('桌面通知配置响应无效');
        if (!body.data.some(endpoint => endpoint.deviceId === identity.deviceId && endpoint.platform === 'desktop')) {
          const preference = await this.businessCore.fetch('/api/browser-notifications/status?deviceId=' + encodeURIComponent(identity.deviceId));
          if (!preference.ok) throw new Error('桌面通知偏好读取失败');
          const preferenceBody = await preference.json() as { data?: { enabled?: boolean } };
          if (typeof preferenceBody.data?.enabled !== 'boolean') throw new Error('桌面通知偏好响应无效');
          const registration = await this.businessCore.fetch('/api/notifications/devices/register', { method: 'POST', body: JSON.stringify({ deviceId: identity.deviceId, platform: 'desktop', preferredProvider: 'desktop-sse', systemNotificationsEnabled: preferenceBody.data.enabled }) });
          if (!registration.ok) throw new Error('桌面通知通道注册失败');
          const receipt = await registration.json() as { data?: { id?: string; deviceId?: string; platform?: string } };
          if (!receipt.data?.id || receipt.data.deviceId !== identity.deviceId || receipt.data.platform !== 'desktop') throw new Error('桌面通知通道保存未确认');
        }
      }
      const response = await this.businessCore.fetch(path, {
        method: "GET",
        headers: {
          Accept: "text/event-stream",
          "Cache-Control": "no-cache",
        },
        signal: this.abortController.signal,
      });

      if (response.status !== 200 || !response.body) {
        this.scheduleReconnect();
        return;
      }

      this.reconnectAttempts = 0;
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const events = buffer.split("\n\n");
          buffer = events.pop() || "";
          for (const raw of events) {
            this.parseAndDispatch(raw);
          }
        }
      } finally {
        reader.releaseLock();
      }

      if (!this.stopped) {
        this.scheduleReconnect();
      }
    } catch {
      if (!this.stopped) {
        this.scheduleReconnect();
      }
    }
  }

  private parseAndDispatch(raw: string): void {
    let eventName = "message";
    let dataStr = "";
    for (const line of raw.split("\n")) {
      const trimmed = line.trimEnd();
      if (trimmed.startsWith("event:")) {
        eventName = trimmed.slice(6).trim();
      } else if (trimmed.startsWith("data:")) {
        dataStr += trimmed.slice(5).trim();
      }
    }
    if (!dataStr) return;
    let envelope: SSEEventEnvelope;
    try {
      envelope = JSON.parse(dataStr) as SSEEventEnvelope;
    } catch {
      return;
    }
    if (!this.shouldProcess(envelope)) return;
    switch (eventName) {
      case "scheduled_notification":
        this.handleScheduledNotification(envelope);
        break;
      case "ui_notify":
        this.handleNotify(envelope);
        break;
      case "ui_dialog":
        this.handleDialog(envelope);
        break;
      case "ui_navigate":
        this.handleNavigate(envelope);
        break;
    }
  }

  private shouldProcess(envelope: SSEEventEnvelope): boolean {
    if (!envelope.requestId) return false;
    if (this.isExpired(envelope.expiresAt)) return false;
    if (this.processedRequestIds.has(envelope.requestId)) return false;
    this.processedRequestIds.add(envelope.requestId);
    if (this.processedRequestIds.size > DEDUP_MAX_SIZE) {
      const first = this.processedRequestIds.values().next().value;
      if (first) this.processedRequestIds.delete(first);
    }
    return true;
  }

  private isExpired(expiresAt?: string): boolean {
    if (!expiresAt) return false;
    try {
      return new Date(expiresAt).getTime() < Date.now();
    } catch {
      return false;
    }
  }

  private handleNotify(envelope: SSEEventEnvelope): void {
    if (!envelope.payload) return;
    const title = (envelope.payload.title as string) || "通知";
    const body = (envelope.payload.body as string) || "";
    const notif = new Notification({ title, body });
    notif.show();
  }

  private handleScheduledNotification(envelope: SSEEventEnvelope): void {
    const payload = envelope.payload;
    if (!payload) return;
    const acknowledge = (state: string) => {
      void this.businessCore.fetch('/api/notifications/scheduled/' + encodeURIComponent(envelope.requestId) + '/ack', { method: 'POST', body: JSON.stringify({ state }) }).catch(() => {});
    };
    if (!Notification.isSupported()) { acknowledge('display_failed'); return; }
    const notification = new Notification({ title: String(payload.title || 'Amitia'), body: String(payload.body || ''), silent: payload.sound === false });
    notification.on('show', () => acknowledge('displayed'));
    notification.on('failed', () => acknowledge('display_failed'));
    notification.on('click', () => {
      acknowledge('clicked');
      const win = this.mainWindow();
      if (!win || win.isDestroyed()) return;
      win.show();
      win.focus();
      const conversation = String(payload.conversationId || '');
      if (conversation) win.webContents.send(IPC_CHANNELS.uiNavigate, '/chat?conversationId=' + encodeURIComponent(conversation) + '&characterId=' + encodeURIComponent(String(payload.characterId || '')) + '&ownerId=' + encodeURIComponent(String(payload.ownerId || '')));
    });
    notification.show();
  }

  private async handleDialog(envelope: SSEEventEnvelope): Promise<void> {
    if (!envelope.payload) return;
    const dialogId = envelope.payload.dialogId as string;
    const message = (envelope.payload.message as string) || "";
    const buttons = (envelope.payload.buttons as string[]) || ["确定"];
    const win = this.mainWindow();
    const options = {
      type: severityMap[envelope.payload.severity as string] || "info" as const,
      title: "对话框",
      message,
      buttons,
      cancelId: buttons.length - 1,
    };
    let resultIndex = 0;
    try {
      const result = win && !win.isDestroyed()
        ? await dialog.showMessageBox(win, options)
        : await dialog.showMessageBox(options);
      resultIndex = result.response;
    } catch {
      resultIndex = buttons.length - 1;
    }
    const result = buttons[resultIndex] || "closed";
    void this.sendDialogResponse(dialogId, result);
  }

  private handleNavigate(envelope: SSEEventEnvelope): void {
    if (!envelope.payload) return;
    const target = envelope.payload.target as string;
    if (!target) return;
    const win = this.mainWindow();
    if (win && !win.isDestroyed()) {
      win.webContents.send(IPC_CHANNELS.uiNavigate, target);
    }
  }

  private async sendDialogResponse(dialogId: string, result: string): Promise<void> {
    await this.businessCore.fetch(
      "/api/extensions/ui/dialog-response",
      {
        method: "POST",
        body: JSON.stringify({ dialogId, result }),
      },
    ).catch(() => {});
  }

  private getReconnectDelay(): number {
    const delay = Math.min(
      BASE_RECONNECT_MS * Math.pow(2, this.reconnectAttempts),
      MAX_RECONNECT_MS,
    );
    return delay + Math.random() * 500;
  }

  private scheduleReconnect(): void {
    if (this.stopped) return;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
    }
    if (this.reconnectAttempts >= MAX_RETRIES) {
      this.reconnectAttempts = 0;
      this.reconnectTimer = setTimeout(() => {
        void this.connect();
      }, MAX_RECONNECT_MS);
      return;
    }
    const delay = this.getReconnectDelay();
    this.reconnectAttempts++;
    this.reconnectTimer = setTimeout(() => {
      void this.connect();
    }, delay);
  }
}
