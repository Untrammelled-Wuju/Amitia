import type { BrowserWindow } from "electron";
import type { DesktopHostManager } from "./desktop-host";
import type { BusinessCoreClient } from "../business-core-client";
import type { DesktopSnapshot } from "./types";

export class DesktopSnapshotSync {
  private readonly mainWindow: BrowserWindow;
  private readonly host: DesktopHostManager;
  private readonly businessCoreClient: BusinessCoreClient;
  private timer: NodeJS.Timeout | null = null;
  private stopped = false;
  private syncing = false;

  constructor(mainWindow: BrowserWindow, host: DesktopHostManager, businessCoreClient: BusinessCoreClient) {
    this.mainWindow = mainWindow;
    this.host = host;
    this.businessCoreClient = businessCoreClient;
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
      const buildResponse = await this.businessCoreClient.fetch(
        "/api/extensions/desktop/snapshot/build",
        { method: "POST", body: "{}" },
      );
      if (!buildResponse.ok) {
        throw new Error(`Snapshot build failed: ${buildResponse.status}`);
      }
      const snapshot = (await buildResponse.json()) as DesktopSnapshot;
      const current = this.host.getCurrentSnapshot();
      if (
        current &&
        snapshot.generation <= current.generation &&
        snapshot.hash === current.hash
      ) {
        return;
      }
      if (!this.host.applySnapshot(snapshot)) {
        await this.businessCoreClient.fetch(
          "/api/extensions/desktop/snapshot/apply-result",
          {
            method: "POST",
            body: JSON.stringify({
              generation: snapshot.generation,
              hash: snapshot.hash,
              success: false,
              error: "snapshot apply rejected",
            }),
          },
        );
        throw new Error(`Snapshot apply rejected: ${snapshot.generation}`);
      }
      await this.businessCoreClient.fetch(
        "/api/extensions/desktop/snapshot/apply-result",
        {
          method: "POST",
          body: JSON.stringify({
            generation: snapshot.generation,
            hash: snapshot.hash,
            success: true,
          }),
        },
      );
    } catch (err) {
      console.warn("[DesktopHost] 快照同步失败:", err);
    } finally {
      this.syncing = false;
    }
  }
}
