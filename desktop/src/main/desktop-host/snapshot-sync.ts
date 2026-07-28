import type { BrowserWindow } from "electron";
import type { DesktopHostManager } from "./desktop-host";
import type { DesktopSnapshot } from "./types";

export class DesktopSnapshotSync {
  private readonly mainWindow: BrowserWindow;
  private readonly host: DesktopHostManager;
  private timer: NodeJS.Timeout | null = null;
  private stopped = false;
  private syncing = false;

  constructor(mainWindow: BrowserWindow, host: DesktopHostManager) {
    this.mainWindow = mainWindow;
    this.host = host;
  }

  start(): void {
    this.stopped = false;
    const sync = () => void this.syncNow();
    if (this.mainWindow.webContents.isLoading()) {
      this.mainWindow.webContents.once("did-finish-load", sync);
    } else {
      sync();
    }
    this.timer = setInterval(sync, 2000);
  }

  stop(): void {
    this.stopped = true;
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  async syncNow(): Promise<void> {
    if (this.stopped || this.syncing || this.mainWindow.isDestroyed()) return;
    this.syncing = true;
    try {
      const token = await this.mainWindow.webContents.executeJavaScript(
        'localStorage.getItem("ai-companion-token")',
        true,
      );
      if (typeof token !== "string" || token.length === 0) return;
      const response = await this.request("POST", token);
      if (!response.ok) {
        throw new Error(`Snapshot request failed: ${response.status}`);
      }
      const snapshot = (await response.json()) as DesktopSnapshot;
      const current = this.host.getCurrentSnapshot();
      if (
        current &&
        snapshot.generation <= current.generation &&
        snapshot.hash === current.hash
      ) {
        return;
      }
      if (!this.host.applySnapshot(snapshot)) {
        await this.report(token, snapshot, false, "snapshot apply rejected");
        throw new Error(`Snapshot apply rejected: ${snapshot.generation}`);
      }
      await this.report(token, snapshot, true);
    } catch (err) {
      console.warn("[DesktopHost] 快照同步失败:", err);
    } finally {
      this.syncing = false;
    }
  }

  private request(method: "GET" | "POST", token: string): Promise<Response> {
    return fetch(
      `http://127.0.0.1:18899/api/extensions/desktop/snapshot${method === "POST" ? "/build" : ""}`,
      {
        method,
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: method === "POST" ? "{}" : undefined,
      },
    );
  }

  private async report(
    token: string,
    snapshot: DesktopSnapshot,
    success: boolean,
    error?: string,
  ): Promise<void> {
    const response = await fetch(
      "http://127.0.0.1:18899/api/extensions/desktop/snapshot/apply-result",
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          generation: snapshot.generation,
          hash: snapshot.hash,
          success,
          error,
        }),
      },
    );
    if (!response.ok) {
      throw new Error(`Snapshot report failed: ${response.status}`);
    }
  }
}
