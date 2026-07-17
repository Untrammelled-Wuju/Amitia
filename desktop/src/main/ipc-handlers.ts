import { app, BrowserWindow, dialog, ipcMain, shell } from "electron"
import { promises as fs } from "node:fs"
import path from "node:path"
import { randomUUID } from "node:crypto"
import { IPC_CHANNELS } from "../shared/ipc"
import type { DeploymentModeConfig } from "../shared/types"
import { ConfigStore } from "./config-store"
import type { DesktopRuntimeManager } from "../runtime/runtime-manager"

export function registerIpcHandlers(
  configStore: ConfigStore,
  runtimeManager: DesktopRuntimeManager,
  onDeploymentConfigSaved?: (config: DeploymentModeConfig) => void,
): void {
  ipcMain.handle(IPC_CHANNELS.getEnvironment, () => ({
    platform: process.platform,
    arch: process.arch,
    version: app.getVersion(),
    isPackaged: app.isPackaged,
  }))

  ipcMain.handle(IPC_CHANNELS.getDeploymentConfig, async () => {
    return configStore.getDeploymentConfig()
  })

  ipcMain.handle(IPC_CHANNELS.saveDeploymentConfig, async (_event, config: DeploymentModeConfig) => {
    const next = await configStore.saveDeploymentConfig(config)
    runtimeManager.setDeploymentConfig(next)
    onDeploymentConfigSaved?.(next)
    return next
  })

  ipcMain.handle(IPC_CHANNELS.getRuntimeStatus, () => runtimeManager.getStatus())

  ipcMain.handle(IPC_CHANNELS.openLogsDirectory, async () => {
    await shell.openPath(app.getPath("logs"))
  })

  ipcMain.handle(IPC_CHANNELS.selectAgentSkillDirectory, async (event) => {
    const window = BrowserWindow.fromWebContents(event.sender)
    const options = { properties: ["openDirectory"] as Array<"openDirectory"> }
    const result = window ? await dialog.showOpenDialog(window, options) : await dialog.showOpenDialog(options)
    if (result.canceled || !result.filePaths[0]) return null
    const root = result.filePaths[0]
    const files: Array<{ path: string; name: string; base64: string }> = []
    let total = 0
    const visit = async (directory: string, depth: number): Promise<void> => {
      if (depth > 12) throw new Error("Agent Skill 目录层级超过限制")
      const entries = await fs.readdir(directory, { withFileTypes: true })
      for (const entry of entries) {
        const fullPath = path.join(directory, entry.name)
        const stat = await fs.lstat(fullPath)
        if (stat.isSymbolicLink()) throw new Error("Agent Skill 目录不能包含符号链接")
        if (entry.isDirectory()) {
          await visit(fullPath, depth + 1)
          continue
        }
        if (!entry.isFile()) throw new Error("Agent Skill 目录包含不支持的文件类型")
        if (files.length >= 500 || stat.size > 20 * 1024 * 1024 || total + stat.size > 50 * 1024 * 1024) throw new Error("Agent Skill 目录超过安全限制")
        const relative = path.relative(root, fullPath).split(path.sep).join("/")
        const content = await fs.readFile(fullPath)
        total += content.length
        files.push({ path: relative, name: entry.name, base64: content.toString("base64") })
      }
    }
    await visit(root, 1)
    return { rootName: path.basename(root), files }
  })

  ipcMain.handle(IPC_CHANNELS.selectExtensionPackage, async (event) => {
    const window = BrowserWindow.fromWebContents(event.sender)
    const options = { properties: ["openFile"] as Array<"openFile">, filters: [{ name: "Amitia 扩展包", extensions: ["amitiax", "zip"] }] }
    const result = window ? await dialog.showOpenDialog(window, options) : await dialog.showOpenDialog(options)
    if (result.canceled || !result.filePaths[0]) return null
    const selected = result.filePaths[0]
    const stat = await fs.lstat(selected)
    if (!stat.isFile() || stat.isSymbolicLink() || stat.size > 100 * 1024 * 1024) throw new Error("扩展包文件无效或超过 100 MB")
    const content = await fs.readFile(selected)
    return { name: path.basename(selected), size: content.length, base64: content.toString("base64") }
  })

  ipcMain.handle(IPC_CHANNELS.saveExtensionPackage, async (event, request: { suggestedName?: unknown; base64?: unknown }) => {
    if (!request || typeof request.suggestedName !== "string" || typeof request.base64 !== "string") throw new Error("导出保存参数无效")
    const suggestedName = path.basename(request.suggestedName).replace(/[^A-Za-z0-9._-]/g, "-")
    if (!suggestedName || !/\.(amitiax|zip)$/i.test(suggestedName)) throw new Error("导出文件名无效")
    const content = Buffer.from(request.base64, "base64")
    if (!content.length || content.length > 100 * 1024 * 1024) throw new Error("导出内容无效或超过 100 MB")
    const window = BrowserWindow.fromWebContents(event.sender)
    const options = { defaultPath: suggestedName, filters: [{ name: "Amitia 扩展包", extensions: ["amitiax", "zip"] }] }
    const result = window ? await dialog.showSaveDialog(window, options) : await dialog.showSaveDialog(options)
    if (result.canceled || !result.filePath) return { saved: false }
    const tempRoot = path.join(app.getPath("temp"), "amitia-extension-exports")
    const tempFile = path.join(tempRoot, randomUUID())
    await fs.mkdir(tempRoot, { recursive: true })
    try {
      await fs.writeFile(tempFile, content, { mode: 0o600 })
      await fs.copyFile(tempFile, result.filePath)
      return { saved: true, fileName: path.basename(result.filePath) }
    } finally {
      await fs.rm(tempFile, { force: true })
    }
  })

  ipcMain.handle(IPC_CHANNELS.minimizeWindow, (event) => {
    BrowserWindow.fromWebContents(event.sender)?.minimize()
  })

  ipcMain.handle(IPC_CHANNELS.toggleMaximizeWindow, (event) => {
    const win = BrowserWindow.fromWebContents(event.sender)
    if (!win) return false
    if (win.isMaximized()) {
      win.unmaximize()
      return false
    }
    win.maximize()
    return true
  })

  ipcMain.handle(IPC_CHANNELS.closeWindow, (event) => {
    BrowserWindow.fromWebContents(event.sender)?.close()
  })

  ipcMain.handle("window-minimize", (event) => {
    BrowserWindow.fromWebContents(event.sender)?.minimize()
  })

  ipcMain.handle("window-toggle-maximize", (event) => {
    const win = BrowserWindow.fromWebContents(event.sender)
    if (!win) return false
    if (win.isMaximized()) {
      win.unmaximize()
      return false
    }
    win.maximize()
    return true
  })

  ipcMain.handle("window-close", (event) => {
    BrowserWindow.fromWebContents(event.sender)?.close()
  })

  ipcMain.handle("window-is-maximized", () => {
    return BrowserWindow.getFocusedWindow()?.isMaximized() || false
  })

  ipcMain.handle("get-window-type", (event) => {
    const senderWindow = BrowserWindow.fromWebContents(event.sender)
    const allWindows = BrowserWindow.getAllWindows()
    return senderWindow === allWindows[0] ? "main" : "child"
  })
}
