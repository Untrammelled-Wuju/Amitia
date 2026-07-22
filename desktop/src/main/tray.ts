import { fileURLToPath } from "node:url"
import { dirname, join } from "node:path"
import { app, BrowserWindow, Menu, nativeImage, shell, Tray } from "electron"
import { configToLabel } from "../shared/deployment"
import type { DeploymentModeConfig } from "../shared/types"
import type { ConfigStore } from "./config-store"
import { IPC_CHANNELS } from "../shared/ipc"

const currentDir = dirname(fileURLToPath(import.meta.url))

export function createAppTray(
  win: BrowserWindow,
  getConfig: () => DeploymentModeConfig,
  configStore: ConfigStore,
): { tray: Tray; refreshMenu: () => Promise<void> } {
  const icon = nativeImage.createFromPath(join(currentDir, "../../resources/tray.png"))
  const tray = new Tray(icon)
  tray.setToolTip("Amitia")

  const updateMenu = async () => {
    const visible = win.isVisible()
    const config = getConfig()
    const autoLaunch = await configStore.getAutoLaunch()
    tray.setContextMenu(Menu.buildFromTemplate([
      { label: "显示 Amitia", enabled: !visible, click: () => showWindow(win) },
      { label: "隐藏窗口", enabled: visible, click: () => win.hide() },
      { label: `当前部署模式：${configToLabel(config)}`, enabled: false },
      { type: "separator" },
      {
        label: "开机自启动",
        type: "checkbox",
        checked: autoLaunch,
        click: async (menuItem) => {
          const next = menuItem.checked
          await configStore.setAutoLaunch(next)
          app.setLoginItemSettings({ openAtLogin: next })
          win.webContents.send(IPC_CHANNELS.autoLaunchChanged, next)
        },
      },
      { type: "separator" },
      { label: "打开日志目录", click: () => void shell.openPath(app.getPath("logs")) },
      { label: "退出 Amitia", click: () => { app.quit() } },
    ]))
  }

  tray.on("click", () => showWindow(win))
  tray.on("double-click", () => showWindow(win))
  win.on("show", () => { void updateMenu() })
  win.on("hide", () => { void updateMenu() })
  void updateMenu()
  return { tray, refreshMenu: updateMenu }
}

function showWindow(win: BrowserWindow): void {
  if (win.isMinimized()) win.restore()
  win.show()
  win.focus()
}
