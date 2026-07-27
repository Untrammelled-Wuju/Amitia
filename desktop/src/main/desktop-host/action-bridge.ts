import { BrowserWindow, ipcMain, type IpcMainEvent } from "electron";
import { randomUUID } from "node:crypto";
import { DESKTOP_IPC_CHANNELS } from "./ipc-channels";
import type { ActionInvokeRequest, ActionInvokeResult } from "./types";

const ACTION_RESPONSE_CHANNEL = "desktop:action:bridge:response";

interface ActionBridgeResponse {
  requestId: string;
  result?: ActionInvokeResult;
  error?: string;
}

interface PendingRequest {
  resolve: (result: ActionInvokeResult) => void;
  reject: (error: Error) => void;
  timer: NodeJS.Timeout;
}

export class DesktopActionBridge {
  private readonly mainWindow: BrowserWindow;
  private readonly timeoutMs: number;
  private readonly pending = new Map<string, PendingRequest>();
  private responseHandler:
    | ((event: IpcMainEvent, payload: ActionBridgeResponse) => void)
    | null = null;

  constructor(mainWindow: BrowserWindow, timeoutMs = 30000) {
    this.mainWindow = mainWindow;
    this.timeoutMs = timeoutMs;
    this.registerResponseListener();
  }

  private registerResponseListener(): void {
    this.responseHandler = (_event, payload) => {
      const pending = this.pending.get(payload.requestId);
      if (!pending) return;
      clearTimeout(pending.timer);
      this.pending.delete(payload.requestId);
      if (payload.error) {
        pending.reject(new Error(payload.error));
      } else if (payload.result) {
        pending.resolve(payload.result);
      } else {
        pending.reject(new Error("Action invoke returned empty result"));
      }
    };
    ipcMain.on(ACTION_RESPONSE_CHANNEL, this.responseHandler);
  }

  async invokeAction(
    request: ActionInvokeRequest,
  ): Promise<ActionInvokeResult> {
    if (this.mainWindow.isDestroyed()) {
      return { success: false, error: "Main window is destroyed" };
    }
    const requestId = randomUUID();
    const promise = new Promise<ActionInvokeResult>((resolve, reject) => {
      const timer = setTimeout(() => {
        if (this.pending.delete(requestId)) {
          reject(new Error("Action invoke timed out"));
        }
      }, this.timeoutMs);
      this.pending.set(requestId, { resolve, reject, timer });
    });
    this.mainWindow.webContents.send(DESKTOP_IPC_CHANNELS.ACTION_INVOKE, {
      requestId,
      request,
    });
    try {
      return await promise;
    } catch (err) {
      return {
        success: false,
        error: err instanceof Error ? err.message : String(err),
      };
    }
  }

  cleanup(): void {
    if (this.responseHandler) {
      ipcMain.removeListener(ACTION_RESPONSE_CHANNEL, this.responseHandler);
      this.responseHandler = null;
    }
    for (const [, pending] of this.pending) {
      clearTimeout(pending.timer);
      pending.reject(new Error("Action bridge cleanup"));
    }
    this.pending.clear();
  }
}
