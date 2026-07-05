import { BrowserWindow, dialog, ipcMain, app, shell } from "electron"
import { autoUpdater, UpdateInfo } from "electron-updater"
import log from "electron-log"
import path from "path"
import { getAmitiaDataDir } from "./path-manager"

let mainWindow: BrowserWindow | null = null
let startupResolve: (() => void) | null = null
let pendingUpdateInfo: UpdateInfo | null = null

const GITEE_RELEASES_URL = "https://gitee.com/Untrammelled-Wuju/Amitia/releases"

export function registerUpdateManager(win: BrowserWindow): void {
  mainWindow = win

  const logDir = path.join(getAmitiaDataDir(), "logs")
  log.transports.file.resolvePathFn = () => path.join(logDir, "update.log")
  log.transports.file.level = "info"
  autoUpdater.logger = log
  autoUpdater.autoDownload = false
  autoUpdater.autoInstallOnAppQuit = false

  autoUpdater.on("checking-for-update", () => {
    console.log("[UpdateManager] 正在检查更新...")
    mainWindow?.webContents.send("update:checking")
  })

  autoUpdater.on("update-available", (info: UpdateInfo) => {
    console.log("[UpdateManager] 发现新版本:", info.version)
    pendingUpdateInfo = info
    mainWindow?.webContents.send("update:available", {
      version: info.version,
      releaseDate: info.releaseDate,
      releaseNotes: info.releaseNotes,
    })

    const currentVersion = app.getVersion()
    dialog.showMessageBox(mainWindow!, {
      type: "info",
      title: "发现新版本",
      message: `发现新版本 v${info.version}`,
      detail: [
        `当前版本: v${currentVersion}`,
        `最新版本: v${info.version}`,
        "",
        info.releaseNotes ? `更新内容:\n${String(info.releaseNotes)}` : "",
      ].filter(Boolean).join("\n"),
      buttons: ["立即下载", "稍后提醒", "Gitee 备用下载"],
      defaultId: 0,
      cancelId: 1,
    }).then((result) => {
      if (result.response === 0) {
        autoUpdater.downloadUpdate()
      } else if (result.response === 2) {
        void shell.openExternal(GITEE_RELEASES_URL)
        pendingUpdateInfo = null
        startupResolve?.()
        startupResolve = null
      } else {
        pendingUpdateInfo = null
        startupResolve?.()
        startupResolve = null
      }
    })
  })

  autoUpdater.on("update-not-available", () => {
    console.log("[UpdateManager] 已是最新版本")
    mainWindow?.webContents.send("update:not-available")
    pendingUpdateInfo = null
    startupResolve?.()
    startupResolve = null
  })

  autoUpdater.on("download-progress", (progress) => {
    mainWindow?.webContents.send("update:download-progress", {
      percent: progress.percent,
      transferred: progress.transferred,
      total: progress.total,
      bytesPerSecond: progress.bytesPerSecond,
    })
  })

  autoUpdater.on("update-downloaded", (info) => {
    console.log("[UpdateManager] 更新已下载:", info.version)
    pendingUpdateInfo = null
    mainWindow?.webContents.send("update:downloaded", { version: info.version })
    dialog.showMessageBox(mainWindow!, {
      type: "info",
      title: "更新已下载",
      message: `v${info.version} 已下载完成`,
      detail: "是否立即重启并安装更新?",
      buttons: ["立即重启", "稍后处理"],
      defaultId: 0,
      cancelId: 1,
    }).then((result) => {
      if (result.response === 0) {
        autoUpdater.quitAndInstall(false, true)
      }
    })
  })

  autoUpdater.on("error", (error) => {
    console.error("[UpdateManager] 更新错误:", error.message)
    mainWindow?.webContents.send("update:error", { message: error.message })
    if (startupResolve) {
      pendingUpdateInfo = null
      startupResolve()
      startupResolve = null
    }
  })

  ipcMain.handle("update:check-on-startup", async () => {
    try {
      const result = await autoUpdater.checkForUpdates()
      return result
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error)
      console.error("[UpdateManager] 检查更新异常:", msg)
      mainWindow?.webContents.send("update:error", { message: msg })
      return null
    }
  })

  ipcMain.handle("update:download", async () => {
    try {
      await autoUpdater.downloadUpdate()
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error)
      console.error("[UpdateManager] 下载更新失败:", msg)
      mainWindow?.webContents.send("update:error", { message: msg })
    }
  })

  ipcMain.handle("update:install-now", () => {
    autoUpdater.quitAndInstall(false, true)
  })

  ipcMain.handle("update:cancel-and-enter", () => {
    pendingUpdateInfo = null
    startupResolve?.()
    startupResolve = null
  })

  ipcMain.handle("update:get-current-version", () => {
    return app.getVersion()
  })

  ipcMain.handle("update:open-gitee-release", () => {
    return shell.openExternal(GITEE_RELEASES_URL)
  })

  ipcMain.handle("update:check-now", async () => {
    try {
      const result = await autoUpdater.checkForUpdates()
      if (!result || !result.updateInfo) {
        dialog.showMessageBox(mainWindow!, {
          type: "info",
          title: "检查更新",
          message: "当前已是最新版本",
          buttons: ["确定"],
        })
      }
      return result
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error)
      console.error("[UpdateManager] 手动检查更新失败:", msg)
      dialog.showMessageBox(mainWindow!, {
        type: "error",
        title: "检查更新失败",
        message: "无法连接到更新服务器",
        detail: msg,
        buttons: ["确定"],
      })
      return null
    }
  })
}

export async function waitForStartupCheck(): Promise<void> {
  try {
    const result = await autoUpdater.checkForUpdates()
    if (!result || !result.updateInfo) {
      console.log("[UpdateManager] 启动检测: 无更新 (dev模式或已是最新)")
      return
    }
    console.log("[UpdateManager] 启动检测: 发现新版本, 等待用户选择...")
    return new Promise((resolve) => {
      startupResolve = resolve
    })
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error)
    console.error("[UpdateManager] 启动检查更新异常:", msg)
  }
}
