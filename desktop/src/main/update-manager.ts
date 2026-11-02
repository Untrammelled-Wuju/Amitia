import { BrowserWindow, ipcMain, app, shell } from "electron";
import { autoUpdater, UpdateInfo } from "electron-updater";
import path from "path";
import fs from "fs";
import { format } from "node:util";
import { getAmitiaDataDir, getInstallDir, isDevMode } from "./path-manager";
import {
  desktopUpdateChannel,
  resolveAllowBetaChannel,
} from "./update-channel";
import { installSegmentedFullDownload } from "./segmented-updater";
import { DailySizeRotatingLog } from "./log-rotation";

let mainWindow: BrowserWindow | null = null;
let startupResolve: (() => void) | null = null;
let pendingUpdateInfo: UpdateInfo | null = null;
let allowBetaChannel = false;
let installInProgress = false;
let downloadedUpdateReady = false;
let updateLogWriter: DailySizeRotatingLog | null = null;

type UpdateInfoWithPresentation = UpdateInfo & {
  updateTitle?: unknown;
  highlights?: unknown;
};

export interface UpdateManagerHooks {
  prepareForInstall?: () => Promise<void>;
}

const RELEASES_URL = "https://amitia.untrammelled.top/amitia";
const UPDATE_TEST_MODE = process.env.AMITIA_UPDATE_TEST_MODE === "1";
const AUTO_UPDATE_SUPPORTED =
  process.platform === "win32" || UPDATE_TEST_MODE;

function normalizeHighlights(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value
    .map((item) => String(item ?? "").trim())
    .filter(Boolean)
    .slice(0, 8);
}

function updatePresentation(info: UpdateInfo) {
  const extended = info as UpdateInfoWithPresentation;
  const installerSize = Array.isArray(info.files)
    ? Number(info.files.find((file) => file.url)?.size)
    : 0;
  return {
    version: info.version,
    releaseDate: info.releaseDate,
    releaseNotes: info.releaseNotes,
    updateTitle:
      typeof extended.updateTitle === "string"
        ? extended.updateTitle.trim()
        : "",
    highlights: normalizeHighlights(extended.highlights),
    size: Number.isFinite(installerSize) && installerSize > 0 ? installerSize : 0,
    channel: allowBetaChannel ? "beta" : "stable",
    currentVersion: app.getVersion(),
  };
}

function updatePreferencePath(): string {
  return path.join(getAmitiaDataDir(), "update-preferences.json");
}

interface UpdatePreferences {
  allowBetaChannel?: boolean;
  skippedVersion?: string;
}

function readUpdatePreferences(): UpdatePreferences {
  const file = updatePreferencePath();
  if (!fs.existsSync(file)) return {};
  const parsed = JSON.parse(fs.readFileSync(file, "utf-8")) as UpdatePreferences;
  return parsed && typeof parsed === "object" ? parsed : {};
}

function writeUpdatePreferences(patch: UpdatePreferences): void {
  fs.mkdirSync(path.dirname(updatePreferencePath()), { recursive: true });
  const current = readUpdatePreferences();
  fs.writeFileSync(
    updatePreferencePath(),
    JSON.stringify({ ...current, ...patch }, null, 2),
    "utf-8",
  );
}

function loadAllowBetaChannel(): boolean {
  try {
    const parsed = readUpdatePreferences();
    return resolveAllowBetaChannel(parsed.allowBetaChannel, app.getVersion());
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.error("[UpdateManager] 读取更新通道配置失败:", msg);
    return resolveAllowBetaChannel(undefined, app.getVersion());
  }
}

function persistAllowBetaChannel(value: boolean): void {
  try {
    writeUpdatePreferences({ allowBetaChannel: value });
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.error("[UpdateManager] 写入更新通道配置失败:", msg);
  }
}

function loadSkippedVersion(): string {
  try {
    const value = readUpdatePreferences().skippedVersion;
    return typeof value === "string" ? value.trim() : "";
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.error("[UpdateManager] 读取跳过版本配置失败:", msg);
    return "";
  }
}

function persistSkippedVersion(version: string): void {
  try {
    writeUpdatePreferences({ skippedVersion: version.trim() });
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.error("[UpdateManager] 写入跳过版本配置失败:", msg);
  }
}

function applyChannelPreference(): void {
  const testFeedUrl = process.env.AMITIA_UPDATE_TEST_FEED_URL?.trim();
  if (UPDATE_TEST_MODE && testFeedUrl) {
    autoUpdater.setFeedURL({
      provider: "generic",
      url: testFeedUrl,
      channel: "latest",
    });
  }
  autoUpdater.channel = desktopUpdateChannel(allowBetaChannel);
  autoUpdater.allowPrerelease = allowBetaChannel;
  autoUpdater.allowDowngrade = false;
}

export function registerUpdateManager(win: BrowserWindow, hooks: UpdateManagerHooks = {}): void {
  mainWindow = win;

  const logDir = path.join(getAmitiaDataDir(), "logs");
  updateLogWriter ??= new DailySizeRotatingLog(logDir, "update");
  autoUpdater.logger = {
    info: (message?: unknown) => writeUpdateLog("INFO", message),
    warn: (message?: unknown) => writeUpdateLog("WARN", message),
    error: (message?: unknown) => writeUpdateLog("ERROR", message),
    debug: (message: string) => writeUpdateLog("DEBUG", message),
  };
  autoUpdater.autoDownload = false;
  autoUpdater.autoInstallOnAppQuit = false;
  installSegmentedFullDownload(autoUpdater);
  allowBetaChannel = loadAllowBetaChannel();
  applyChannelPreference();

  autoUpdater.on("checking-for-update", () => {
    console.log("[UpdateManager] 正在检查更新...");
    mainWindow?.webContents.send("update:checking");
  });

  autoUpdater.on("update-available", (info: UpdateInfo) => {
    console.log("[UpdateManager] 发现新版本:", info.version);
    if (loadSkippedVersion() === info.version) {
      console.log("[UpdateManager] 已跳过该版本:", info.version);
      pendingUpdateInfo = null;
      downloadedUpdateReady = false;
      mainWindow?.webContents.send("update:not-available");
      startupResolve?.();
      startupResolve = null;
      return;
    }
    pendingUpdateInfo = info;
    downloadedUpdateReady = false;
    mainWindow?.webContents.send("update:available", updatePresentation(info));
  });

  autoUpdater.on("update-not-available", () => {
    console.log("[UpdateManager] 已是最新版本");
    mainWindow?.webContents.send("update:not-available");
    pendingUpdateInfo = null;
    downloadedUpdateReady = false;
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
    downloadedUpdateReady = true;
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
    if (pendingUpdateInfo?.version) {
      persistSkippedVersion(pendingUpdateInfo.version);
    }
    pendingUpdateInfo = null;
    startupResolve?.();
    startupResolve = null;
  });

  const installUpdate = async (): Promise<void> => {
    if (installInProgress) {
      return;
    }
    if (!downloadedUpdateReady) {
      mainWindow?.webContents.send("update:error", { message: "更新尚未下载完成" });
      return;
    }
    installInProgress = true;
    try {
      await hooks.prepareForInstall?.();
      autoUpdater.quitAndInstall(true, true);
    } catch (error) {
      installInProgress = false;
      const msg = error instanceof Error ? error.message : String(error);
      console.error("[UpdateManager] 更新安装准备失败:", msg);
      mainWindow?.webContents.send("update:error", { message: msg });
    }
  };

  ipcMain.handle("update:restart-now", () => installUpdate());

  ipcMain.handle("update:restart-later", () => {
    pendingUpdateInfo = null;
  });

  ipcMain.handle("update:check-on-startup", async () => {
    if (!AUTO_UPDATE_SUPPORTED) {
      mainWindow?.webContents.send("update:not-available");
      return null;
    }
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

  ipcMain.handle("update:install-now", () => installUpdate());

  ipcMain.handle("update:cancel-and-enter", () => {
    pendingUpdateInfo = null;
    startupResolve?.();
    startupResolve = null;
  });

  ipcMain.handle("update:get-current-version", () => {
    return app.getVersion();
  });

  ipcMain.handle("update:get-pending", () => {
    if (!pendingUpdateInfo) return null;
    return updatePresentation(pendingUpdateInfo);
  });

  ipcMain.handle("update:get-channel", () => {
    return { allowBetaChannel };
  });

  ipcMain.handle("update:set-channel", (_event, value: unknown) => {
    allowBetaChannel = value === true;
    applyChannelPreference();
    persistAllowBetaChannel(allowBetaChannel);
    pendingUpdateInfo = null;
    downloadedUpdateReady = false;
    return { allowBetaChannel };
  });

  ipcMain.handle("update:open-gitee-release", () => {
    return shell.openExternal(RELEASES_URL);
  });

  ipcMain.handle("update:check-now", async () => {
    if (!AUTO_UPDATE_SUPPORTED) {
      mainWindow?.webContents.send("update:not-available");
      return null;
    }
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
      const notesPath = !isDevMode()
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

export function closeUpdateLogger(): void {
  if (!updateLogWriter) return;
  const writer = updateLogWriter;
  updateLogWriter = null;
  writer.close();
}

function writeUpdateLog(level: string, message?: unknown): void {
  if (!updateLogWriter) return;
  try {
    updateLogWriter.write(
      `[${new Date().toISOString()}] [${level}] ${format(message ?? "")}\n`,
    );
  } catch (error) {
    console.error("[UpdateManager] 写入更新日志失败:", error);
  }
}

export async function waitForStartupCheck(): Promise<void> {
  if (isDevMode()) {
    console.log("[UpdateManager] 开发模式, 跳过更新检查");
    return;
  }
  if (!AUTO_UPDATE_SUPPORTED) {
    console.log("[UpdateManager] 当前平台不使用桌面自动更新通道, 跳过更新检查");
    return;
  }
  try {
    const result = await autoUpdater.checkForUpdates();
    if (!result || !result.isUpdateAvailable) {
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
