import { BrowserWindow, ipcMain, app, shell } from "electron";
import { autoUpdater, UpdateInfo } from "electron-updater";
import log from "electron-log";
import path from "path";
import fs from "fs";
import { getAmitiaDataDir, getInstallDir } from "./path-manager";

let mainWindow: BrowserWindow | null = null;
let startupResolve: (() => void) | null = null;
let pendingUpdateInfo: UpdateInfo | null = null;

const GITEE_RELEASES_URL =
  "https://gitee.com/Untrammelled-Wuju/Amitia/releases";

export function registerUpdateManager(win: BrowserWindow): void {
  mainWindow = win;

  const logDir = path.join(getAmitiaDataDir(), "logs");
  log.transports.file.resolvePathFn = () => path.join(logDir, "update.log");
  log.transports.file.level = "info";
  autoUpdater.logger = log;
  autoUpdater.autoDownload = false;
  autoUpdater.autoInstallOnAppQuit = false;

  autoUpdater.on("checking-for-update", () => {
    console.log("[UpdateManager] 正在检查更新...");
    mainWindow?.webContents.send("update:checking");
  });

  autoUpdater.on("update-available", (info: UpdateInfo) => {
    console.log("[UpdateManager] 发现新版本:", info.version);
    pendingUpdateInfo = info;
    mainWindow?.webContents.send("update:available", {
      version: info.version,
      releaseDate: info.releaseDate,
      releaseNotes: info.releaseNotes,
      currentVersion: app.getVersion(),
    });
  });

  autoUpdater.on("update-not-available", () => {
    console.log("[UpdateManager] 已是最新版本");
    mainWindow?.webContents.send("update:not-available");
    pendingUpdateInfo = null;
    startupResolve?.();
    startupResolve = null;
  });

  autoUpdater.on("download-progress", (progress) => {
    mainWindow?.webContents.send("update:download-progress", {
      percent: progress.percent,
      transferred: progress.transferred,
      total: progress.total,
      bytesPerSecond: progress.bytesPerSecond,
    });
    console.log(
      `[UpdateManager] 下载进度: ${progress.percent.toFixed(1)}%, ${(progress.transferred / 1024 / 1024).toFixed(2)}MB / ${(progress.total / 1024 / 1024).toFixed(2)}MB @ ${(progress.bytesPerSecond / 1024 / 1024).toFixed(2)}MB/s`,
    );
  });

  autoUpdater.on("update-downloaded", (info) => {
    console.log("[UpdateManager] 更新已下载:", info.version);
    pendingUpdateInfo = null;
    mainWindow?.webContents.send("update:downloaded", {
      version: info.version,
    });
  });

  autoUpdater.on("error", (error) => {
    console.error("[UpdateManager] 更新错误:", error.message);
    mainWindow?.webContents.send("update:error", { message: error.message });
    if (startupResolve) {
      pendingUpdateInfo = null;
      startupResolve();
      startupResolve = null;
    }
  });

  ipcMain.handle("update:start-download", async () => {
    try {
      await autoUpdater.downloadUpdate();
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error);
      console.error("[UpdateManager] 下载更新失败:", msg);
      mainWindow?.webContents.send("update:error", { message: msg });
    }
  });

  ipcMain.handle("update:skip-version", () => {
    pendingUpdateInfo = null;
    startupResolve?.();
    startupResolve = null;
  });

  ipcMain.handle("update:restart-now", () => {
    autoUpdater.quitAndInstall(false, true);
  });

  ipcMain.handle("update:restart-later", () => {
    pendingUpdateInfo = null;
  });

  ipcMain.handle("update:check-on-startup", async () => {
    try {
      const result = await autoUpdater.checkForUpdates();
      return result;
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error);
      console.error("[UpdateManager] 检查更新异常:", msg);
      mainWindow?.webContents.send("update:error", { message: msg });
      return null;
    }
  });

  ipcMain.handle("update:download", async () => {
    try {
      await autoUpdater.downloadUpdate();
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error);
      console.error("[UpdateManager] 下载更新失败:", msg);
      mainWindow?.webContents.send("update:error", { message: msg });
    }
  });

  ipcMain.handle("update:install-now", () => {
    autoUpdater.quitAndInstall(false, true);
  });

  ipcMain.handle("update:cancel-and-enter", () => {
    pendingUpdateInfo = null;
    startupResolve?.();
    startupResolve = null;
  });

  ipcMain.handle("update:get-current-version", () => {
    return app.getVersion();
  });

  ipcMain.handle("update:open-gitee-release", () => {
    return shell.openExternal(GITEE_RELEASES_URL);
  });

  ipcMain.handle("update:check-now", async () => {
    try {
      const result = await autoUpdater.checkForUpdates();
      return result;
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error);
      console.error("[UpdateManager] 手动检查更新失败:", msg);
      mainWindow?.webContents.send("update:error", { message: msg });
      return null;
    }
  });

  ipcMain.handle("release-notes:get", () => {
    try {
      const notesPath = app.isPackaged
        ? path.join(process.resourcesPath, "release-notes.md")
        : path.join(getInstallDir(), "release-notes.md");
      if (fs.existsSync(notesPath)) {
        return fs.readFileSync(notesPath, "utf-8");
      }
      return "";
    } catch (error) {
      const msg = error instanceof Error ? error.message : String(error);
      console.error("[UpdateManager] 读取 release notes 失败:", msg);
      return "";
    }
  });
}

export async function waitForStartupCheck(): Promise<void> {
  try {
    const result = await autoUpdater.checkForUpdates();
    if (!result || !result.updateInfo) {
      console.log("[UpdateManager] 启动检测: 无更新 (dev模式或已是最新)");
      return;
    }
    console.log("[UpdateManager] 启动检测: 发现新版本, 等待用户选择...");
    return new Promise((resolve) => {
      startupResolve = resolve;
    });
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.error("[UpdateManager] 启动检查更新异常:", msg);
  }
}
