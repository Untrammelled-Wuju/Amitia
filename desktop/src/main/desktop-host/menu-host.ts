import {
  app,
  BrowserWindow,
  Menu,
  type MenuItemConstructorOptions,
} from "electron";
import type { DesktopActionBridge } from "./action-bridge";
import type { DesktopSnapshot, ResolvedContribution } from "./types";

const HOST_ACTION_CHANNEL = "desktop:host:action";
const ROOT_MENU_ORDER = ["file", "edit", "view", "tools", "help"] as const;

export class MenuHost {
  private readonly mainWindow: BrowserWindow;
  private readonly actionBridge: DesktopActionBridge;
  private currentMenu: Menu | null = null;
  private isFocused = false;
  private focusHandler: (() => void) | null = null;
  private blurHandler: (() => void) | null = null;

  constructor(mainWindow: BrowserWindow, actionBridge: DesktopActionBridge) {
    this.mainWindow = mainWindow;
    this.actionBridge = actionBridge;
    this.isFocused = mainWindow.isFocused();
    this.attachWindowListeners();
  }

  applySnapshot(snapshot: DesktopSnapshot): void {
    this.currentMenu = this.buildMenu(snapshot);
    if (this.isFocused) {
      this.applyCurrentMenu();
    }
  }

  cleanup(): void {
    if (this.focusHandler) {
      this.mainWindow.removeListener("focus", this.focusHandler);
      this.focusHandler = null;
    }
    if (this.blurHandler) {
      this.mainWindow.removeListener("blur", this.blurHandler);
      this.blurHandler = null;
    }
    this.currentMenu = null;
    Menu.setApplicationMenu(null);
  }

  private attachWindowListeners(): void {
    this.focusHandler = () => {
      this.isFocused = true;
      this.applyCurrentMenu();
    };
    this.blurHandler = () => {
      this.isFocused = false;
      this.clearApplicationMenu();
    };
    this.mainWindow.on("focus", this.focusHandler);
    this.mainWindow.on("blur", this.blurHandler);
  }

  private applyCurrentMenu(): void {
    if (this.currentMenu) {
      Menu.setApplicationMenu(this.currentMenu);
    }
  }

  private clearApplicationMenu(): void {
    Menu.setApplicationMenu(null);
  }

  private buildMenu(snapshot: DesktopSnapshot): Menu {
    const template: MenuItemConstructorOptions[] = [];
    for (const root of ROOT_MENU_ORDER) {
      const hostItems = this.buildHostMenuItems(root);
      const extensionItems = this.buildExtensionMenuItems(
        snapshot.menuTree[`app.menu.${root}.extensions`] ?? [],
      );
      template.push({
        label: this.rootLabel(root),
        submenu: [...hostItems, ...extensionItems],
      });
    }
    return Menu.buildFromTemplate(template);
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
      case "file":
        return [
          {
            label: "退出 Amitia",
            accelerator: "CmdOrCtrl+Q",
            click: () => app.quit(),
          },
        ];
      case "edit":
        return [
          { role: "undo", label: "撤销" },
          { role: "redo", label: "重做" },
          { type: "separator" },
          { role: "cut", label: "剪切" },
          { role: "copy", label: "复制" },
          { role: "paste", label: "粘贴" },
          { role: "selectAll", label: "全选" },
        ];
      case "view":
        return [
          { role: "reload", label: "重载" },
          { role: "forceReload", label: "强制重载" },
          { role: "toggleDevTools", label: "开发者工具" },
          { type: "separator" },
          { role: "resetZoom", label: "重置缩放" },
          { role: "zoomIn", label: "放大" },
          { role: "zoomOut", label: "缩小" },
          { type: "separator" },
          { role: "togglefullscreen", label: "全屏" },
        ];
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
