import { BrowserWindow, type MenuItemConstructorOptions } from "electron";
import { MenuHost } from "./menu-host";
import { TrayHost } from "./tray-host";
import { ShortcutHost } from "./shortcut-host";
import { GlobalShortcutHost } from "./global-shortcut-host";
import { DesktopActionBridge } from "./action-bridge";
import { SnapshotApplier } from "./snapshot-applier";
import type { BusinessCoreClient } from "../business-core-client";
import type {
  ActionInvokeRequest,
  ActionInvokeResult,
  DesktopSnapshot,
} from "./types";

export class DesktopHostManager {
  private readonly menuHost: MenuHost;
  private readonly trayHost: TrayHost;
  private readonly shortcutHost: ShortcutHost;
  private readonly globalShortcutHost: GlobalShortcutHost;
  private readonly actionBridge: DesktopActionBridge;
  private readonly snapshotApplier: SnapshotApplier;
  private currentSnapshot: DesktopSnapshot | null = null;

  constructor(
    mainWindow: BrowserWindow,
    setExtensionTrayItems: (
      items: MenuItemConstructorOptions[],
    ) => Promise<void>,
    businessCoreClient: BusinessCoreClient,
  ) {
    this.actionBridge = new DesktopActionBridge(mainWindow, businessCoreClient);
    this.menuHost = new MenuHost(mainWindow, this.actionBridge);
    this.trayHost = new TrayHost(
      this.actionBridge,
      setExtensionTrayItems,
    );
    this.shortcutHost = new ShortcutHost(mainWindow, this.actionBridge);
    this.globalShortcutHost = new GlobalShortcutHost(this.actionBridge);
    this.snapshotApplier = new SnapshotApplier(
      this.menuHost,
      this.trayHost,
      this.shortcutHost,
      this.globalShortcutHost,
    );
  }

  applySnapshot(snapshot: DesktopSnapshot): boolean {
    const ok = this.snapshotApplier.apply(snapshot);
    if (ok) {
      this.currentSnapshot = snapshot;
    }
    return ok;
  }

  async invokeAction(
    request: ActionInvokeRequest,
  ): Promise<ActionInvokeResult> {
    return this.actionBridge.invokeAction(request);
  }

  cleanup(): void {
    this.shortcutHost.cleanup();
    this.globalShortcutHost.cleanup();
    this.menuHost.cleanup();
    this.trayHost.cleanup();
    this.actionBridge.cleanup();
    this.currentSnapshot = null;
  }

  getCurrentSnapshot(): DesktopSnapshot | null {
    return this.currentSnapshot;
  }
}
