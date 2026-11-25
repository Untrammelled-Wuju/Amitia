import { app, Menu, nativeTheme, shell, Tray, type BrowserWindow } from "electron";
import { configToLabel } from "../shared/deployment";
import type { DeploymentModeConfig } from "../shared/types";
import type { ConfigStore } from "./config-store";
import { IPC_CHANNELS } from "../shared/ipc";
import { getInitialBrandImage } from "./branding";
import { getAmitiaDataDir } from "./path-manager";
import { showMainWindow } from "./window";

let latestUpdateMenu: (() => Promise<void>) | null = null;
let latestContextMenu: Menu | null = null;

export function refreshTrayMenu() {
  latestUpdateMenu?.();
}

export function createAppTray(
  win: BrowserWindow,
  getConfig: () => DeploymentModeConfig,
  configStore: ConfigStore,
): {
  tray: Tray;
  refreshMenu: () => Promise<void>;
  setExtensionItems: (items: Electron.MenuItemConstructorOptions[]) => Promise<void>;
} {
  const icon = getInitialBrandImage(
    nativeTheme.shouldUseDarkColors ? "dark" : "light",
    "tray",
  );
  const tray = new Tray(icon);
  tray.setToolTip("Amitia");
  let extensionItems: Electron.MenuItemConstructorOptions[] = [];

  const updateMenu = async () => {
    const visible = win.isVisible();
    const config = getConfig();
    const autoLaunch = await configStore.getAutoLaunch();
    latestContextMenu = Menu.buildFromTemplate([
      {
        label: "显示 Amitia",
        enabled: !visible,
        click: () => showMainWindow(win),
      },
      { label: "隐藏窗口", enabled: visible, click: () => win.hide() },
      { label: `当前运行模式：${configToLabel(config)}`, enabled: false },
      { type: "separator" },
      {
        label: "开机自启动",
        type: "checkbox",
        checked: autoLaunch,
        click: async (menuItem) => {
          const next = menuItem.checked;
          await configStore.setAutoLaunch(next);
          app.setLoginItemSettings({ openAtLogin: next });
          win.webContents.send(IPC_CHANNELS.autoLaunchChanged, next);
        },
      },
      { type: "separator" },
      {
        label: "打开数据目录",
        click: () => void shell.openPath(getAmitiaDataDir()),
      },
      ...(extensionItems.length > 0
        ? [{ type: "separator" as const }, ...extensionItems]
        : []),
      { type: "separator" },
      {
        label: "退出 Amitia",
        click: () => {
          app.quit();
        },
      },
    ]);
    if (process.platform !== "darwin" && !tray.isDestroyed()) {
      tray.setContextMenu(latestContextMenu);
    }
  };

  tray.on("click", () => showMainWindow(win));
  tray.on("double-click", () => showMainWindow(win));
  tray.on("right-click", () => {
    if (process.platform !== "darwin") return;
    void (async () => {
      await updateMenu();
      if (!tray.isDestroyed() && latestContextMenu) {
        tray.popUpContextMenu(latestContextMenu);
      }
    })();
  });
  win.on("show", () => {
    void updateMenu();
  });
  win.on("hide", () => {
    void updateMenu();
  });
  void updateMenu();
  latestUpdateMenu = updateMenu;
  return {
    tray,
    refreshMenu: updateMenu,
    setExtensionItems: async (items) => {
      extensionItems = items;
      await updateMenu();
    },
  };
}
