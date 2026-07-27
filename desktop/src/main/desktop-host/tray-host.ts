import {
  app,
  BrowserWindow,
  Menu,
  Tray,
  nativeTheme,
  type MenuItemConstructorOptions,
} from "electron";
import { getInitialBrandImage } from "../branding";
import type { DesktopActionBridge } from "./action-bridge";
import type { DesktopSnapshot, ResolvedContribution } from "./types";

const TRAY_GROUP_ORDER = ["quick_actions", "extensions", "status"] as const;

export class TrayHost {
  private readonly mainWindow: BrowserWindow;
  private readonly tray: Tray;
  private readonly actionBridge: DesktopActionBridge;

  constructor(
    mainWindow: BrowserWindow,
    tray: Tray,
    actionBridge: DesktopActionBridge,
  ) {
    this.mainWindow = mainWindow;
    this.tray = tray;
    this.actionBridge = actionBridge;
    this.tray.setToolTip("Amitia");
    this.refreshIcon();
  }

  applySnapshot(snapshot: DesktopSnapshot): void {
    if (this.tray.isDestroyed()) return;
    const template = this.buildTemplate(snapshot);
    this.tray.setContextMenu(Menu.buildFromTemplate(template));
  }

  cleanup(): void {
    if (!this.tray.isDestroyed()) {
      this.tray.setContextMenu(null);
    }
  }

  private refreshIcon(): void {
    if (this.tray.isDestroyed()) return;
    const icon = getInitialBrandImage(
      nativeTheme.shouldUseDarkColors ? "dark" : "light",
      "tray",
    );
    this.tray.setImage(icon);
  }

  private buildTemplate(
    snapshot: DesktopSnapshot,
  ): MenuItemConstructorOptions[] {
    const items: MenuItemConstructorOptions[] = [];
    items.push(...this.buildHostItems());
    for (const group of TRAY_GROUP_ORDER) {
      const contribs = snapshot.trayTree[group] ?? [];
      const built = this.buildExtensionItems(contribs);
      if (built.length > 0) {
        items.push({ type: "separator" });
        items.push(...built);
      }
    }
    items.push({ type: "separator" });
    items.push({
      label: "退出 Amitia",
      click: () => app.quit(),
    });
    return items;
  }

  private buildHostItems(): MenuItemConstructorOptions[] {
    const visible =
      !this.mainWindow.isDestroyed() && this.mainWindow.isVisible();
    return [
      {
        label: "显示 Amitia",
        enabled: !visible,
        click: () => this.showWindow(),
      },
      {
        label: "隐藏窗口",
        enabled: visible,
        click: () => {
          if (!this.mainWindow.isDestroyed()) this.mainWindow.hide();
        },
      },
    ];
  }

  private showWindow(): void {
    if (this.mainWindow.isDestroyed()) return;
    if (this.mainWindow.isMinimized()) this.mainWindow.restore();
    this.mainWindow.show();
    this.mainWindow.focus();
  }

  private buildExtensionItems(
    contribs: ResolvedContribution[],
  ): MenuItemConstructorOptions[] {
    const registered = contribs.filter((c) => c.status === "registered");
    if (registered.length === 0) return [];
    const sorted = [...registered].sort(
      (a, b) => a.definition.order.priority - b.definition.order.priority,
    );
    const items: MenuItemConstructorOptions[] = [];
    let lastGroup: string | undefined;
    for (const contrib of sorted) {
      const group = contrib.definition.order.group;
      if (group && lastGroup !== undefined && group !== lastGroup) {
        items.push({ type: "separator" });
      }
      lastGroup = group;
      items.push(this.buildExtensionItem(contrib));
    }
    return items;
  }

  private buildExtensionItem(
    contrib: ResolvedContribution,
  ): MenuItemConstructorOptions {
    const def = contrib.definition;
    const isStatus =
      def.desktopType === "app.tray.item" && def.target === "status";
    return {
      id: `ext:${def.contributionId}`,
      label: contrib.effectiveLabel,
      enabled: isStatus ? false : this.evaluateEnabled(contrib),
      click: isStatus
        ? undefined
        : () => {
            void this.actionBridge.invokeAction({
              contributionId: def.contributionId,
              extensionId: def.extensionId,
              input: def.action.input,
            });
          },
    };
  }

  private evaluateEnabled(contrib: ResolvedContribution): boolean {
    const rule = contrib.definition.enabledRule;
    if (rule?.platform && !rule.platform.includes(process.platform)) {
      return false;
    }
    return true;
  }
}
