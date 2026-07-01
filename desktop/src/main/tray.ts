import { join } from "node:path"
import { app, BrowserWindow, Menu, nativeImage, shell, Tray } from "electron"
import { configToLabel } from "../shared/deployment"
import type { DeploymentModeConfig } from "../shared/types"

export function createAppTray(win: BrowserWindow, getConfig: () => DeploymentModeConfig): Tray {
  const icon = nativeImage.createFromPath(join(__dirname, "../../resources/tray.svg"))
  const tray = new Tray(icon.resize({ width: 16, height: 16 }))
  tray.setToolTip("Amitia")

  const updateMenu = () => {
    const visible = win.isVisible()
    const config = getConfig()
    tray.setContextMenu(Menu.buildFromTemplate([
      { label: "显示 Amitia", enabled: !visible, click: () => showWindow(win) },
      { label: "隐藏窗口", enabled: visible, click: () => win.hide() },
      { label: `当前部署模式：${configToLabel(config)}`, enabled: false },
      { type: "separator" },
      { label: "打开日志目录", click: () => void shell.openPath(app.getPath("logs")) },
      { label: "退出 Amitia", click: () => { app.exit(0) } },
    ]))
  }

  tray.on("click", () => showWindow(win))
  tray.on("double-click", () => showWindow(win))
  win.on("show", updateMenu)
  win.on("hide", updateMenu)
  updateMenu()
  return tray
}

function showWindow(win: BrowserWindow): void {
  if (win.isMinimized()) win.restore()
  win.show()
  win.focus()
}
