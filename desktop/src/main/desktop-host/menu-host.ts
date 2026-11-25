import {
  BrowserWindow,
  Menu,
  MenuItem,
  type MenuItemConstructorOptions,
} from "electron";
import type { DesktopActionBridge } from "./action-bridge";
import type { DesktopSnapshot, ResolvedContribution } from "./types";

const HOST_ACTION_CHANNEL = "desktop:host:action";
const ROOT_MENU_ORDER = ["file", "edit", "view", "tools", "help"] as const;

export class MenuHost {
  private readonly mainWindow: BrowserWindow;
  private readonly actionBridge: DesktopActionBridge;
  private readonly nativeMenu: Menu | null;
  private currentMenu: Menu | null = null;

  constructor(mainWindow: BrowserWindow, actionBridge: DesktopActionBridge) {
    this.mainWindow = mainWindow;
    this.actionBridge = actionBridge;
    this.nativeMenu = Menu.getApplicationMenu();
  }

  applySnapshot(snapshot: DesktopSnapshot): void {
    this.currentMenu = this.buildMenu(snapshot);
    Menu.setApplicationMenu(this.currentMenu);
  }

  cleanup(): void {
    this.currentMenu = null;
    Menu.setApplicationMenu(this.nativeMenu);
  }

  private buildMenu(snapshot: DesktopSnapshot): Menu {
    const template: MenuItemConstructorOptions[] = [];
    for (const root of ROOT_MENU_ORDER) {
      const hostItems = this.buildHostMenuItems(root);
      const extensionItems = this.buildExtensionMenuItems(
        snapshot.menuTree[`app.menu.${root}.extensions`] ?? [],
      );
      if (hostItems.length === 0 && extensionItems[0]?.type === "separator") extensionItems.shift();
      const items = [...hostItems, ...extensionItems];
      if (items.length > 0) template.push({ label: this.rootLabel(root), submenu: items });
    }
    const menu = new Menu();
    for (const item of this.nativeMenu?.items ?? []) menu.append(item);
    if (template.length > 0) {
      menu.append(new MenuItem({ label: "Amitia", submenu: template }));
    }
    return menu;
  }

  private rootLabel(root: string): string {
    switch (root) {
      case "file":
        return "文件";
      case "edit":
        return "编辑";
      case "view":
        return "视图";
      case "tools":
        return "工具";
      case "help":
        return "帮助";
      default:
        return root;
    }
  }

  private buildHostMenuItems(root: string): MenuItemConstructorOptions[] {
    switch (root) {
      case "tools":
        return [
          {
            label: "检查更新",
            click: () => this.emitHostAction("check-update"),
          },
          {
            label: "权限管理",
            click: () => this.emitHostAction("open-permissions"),
          },
        ];
      case "help":
        return [
          {
            label: "关于 Amitia",
            click: () => this.emitHostAction("about"),
          },
        ];
      default:
        return [];
    }
  }

  private emitHostAction(action: string): void {
    if (this.mainWindow.isDestroyed()) return;
    this.mainWindow.webContents.send(HOST_ACTION_CHANNEL, { action });
  }

  private buildExtensionMenuItems(
    contribs: ResolvedContribution[],
  ): MenuItemConstructorOptions[] {
    const registered = contribs.filter((c) => c.status === "registered");
    if (registered.length === 0) return [];
    const sorted = [...registered].sort(
      (a, b) => a.definition.order.priority - b.definition.order.priority,
    );
    const items: MenuItemConstructorOptions[] = [{ type: "separator" }];
    let lastGroup: string | undefined;
    for (const contrib of sorted) {
      const group = contrib.definition.order.group;
      if (group && lastGroup !== undefined && group !== lastGroup) {
        items.push({ type: "separator" });
      }
      lastGroup = group;
      items.push(this.buildExtensionMenuItem(contrib));
    }
    return items;
  }

  private buildExtensionMenuItem(
    contrib: ResolvedContribution,
  ): MenuItemConstructorOptions {
    const def = contrib.definition;
    return {
      id: `ext:${def.contributionId}`,
      label: contrib.effectiveLabel,
      enabled: this.evaluateEnabled(contrib),
      click: () => {
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
