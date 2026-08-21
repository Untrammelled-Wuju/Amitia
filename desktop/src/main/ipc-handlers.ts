import { app, BrowserWindow, clipboard, dialog, ipcMain, shell } from "electron";
import { promises as fs } from "node:fs";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { IPC_CHANNELS } from "../shared/ipc";
import type { DeploymentModeConfig } from "../shared/types";
import { ConfigStore } from "./config-store";
import type { DesktopRuntimeManager } from "../runtime/runtime-manager";
import { refreshTrayMenu } from "./tray";
import { setAuthToken } from "./auth-token-store";
import { getDesktopAuthHeaders } from "./backend-session-client";
import { getMeshCoordinator } from "./device-mesh/coordinator";
import { getMeshIdentity, getMeshStatus } from "./device-mesh/local-agent-client";
import {
  listDevices as cloudListDevices,
  revokeDevice as cloudRevokeDevice,
  probeRuntime as cloudProbeRuntime,
  createBootstrapTicket,
} from "./device-mesh/remote-bootstrap-client";

export function registerIpcHandlers(
  configStore: ConfigStore,
  runtimeManager: DesktopRuntimeManager,
  onDeploymentConfigSaved?: (config: DeploymentModeConfig) => void | Promise<void>,
  getMainWindow?: () => BrowserWindow | null,
): void {
  ipcMain.handle(IPC_CHANNELS.getEnvironment, () => ({
    platform: process.platform,
    arch: process.arch,
    version: app.getVersion(),
    isPackaged: app.isPackaged,
  }));

  ipcMain.handle(IPC_CHANNELS.getDeploymentConfig, async () => {
    return configStore.getDeploymentConfig();
  });

  ipcMain.handle(
    IPC_CHANNELS.saveDeploymentConfig,
    async (_event, config: DeploymentModeConfig) => {
      const next = await configStore.saveDeploymentConfig(config);
      runtimeManager.setDeploymentConfig(next);
      await onDeploymentConfigSaved?.(next);
      return next;
    },
  );

  ipcMain.handle(IPC_CHANNELS.getRuntimeStatus, () =>
    runtimeManager.getStatus(),
  );

  ipcMain.handle(IPC_CHANNELS.getAutoLaunch, async () => {
    return configStore.getAutoLaunch();
  });

  ipcMain.handle(
    IPC_CHANNELS.setAutoLaunch,
    async (_event, enabled: boolean) => {
      await configStore.setAutoLaunch(enabled);
      app.setLoginItemSettings({ openAtLogin: enabled });
      refreshTrayMenu();
      return enabled;
    },
  );
  ipcMain.handle(IPC_CHANNELS.openLogsDirectory, async () => {
    await shell.openPath(app.getPath("logs"));
  });

  ipcMain.handle(IPC_CHANNELS.selectAgentSkillDirectory, async (event) => {
    const window = BrowserWindow.fromWebContents(event.sender);
    const options = { properties: ["openDirectory"] as Array<"openDirectory"> };
    const result = window
      ? await dialog.showOpenDialog(window, options)
      : await dialog.showOpenDialog(options);
    if (result.canceled || !result.filePaths[0]) return null;
    const root = result.filePaths[0];
    const files: Array<{ path: string; name: string; base64: string }> = [];
    let total = 0;
    const visit = async (directory: string, depth: number): Promise<void> => {
      if (depth > 12) throw new Error("Agent Skill 目录层级超过限制");
      const entries = await fs.readdir(directory, { withFileTypes: true });
      for (const entry of entries) {
        const fullPath = path.join(directory, entry.name);
        const stat = await fs.lstat(fullPath);
        if (stat.isSymbolicLink())
          throw new Error("Agent Skill 目录不能包含符号链接");
        if (entry.isDirectory()) {
          await visit(fullPath, depth + 1);
          continue;
        }
        if (!entry.isFile())
          throw new Error("Agent Skill 目录包含不支持的文件类型");
        if (
          files.length >= 500 ||
          stat.size > 20 * 1024 * 1024 ||
          total + stat.size > 50 * 1024 * 1024
        )
          throw new Error("Agent Skill 目录超过安全限制");
        const relative = path
          .relative(root, fullPath)
          .split(path.sep)
          .join("/");
        const content = await fs.readFile(fullPath);
        total += content.length;
        files.push({
          path: relative,
          name: entry.name,
          base64: content.toString("base64"),
        });
      }
    };
    await visit(root, 1);
    return { rootName: path.basename(root), files };
  });

  ipcMain.handle(IPC_CHANNELS.selectMCPRoot, async (event) => {
    const window = BrowserWindow.fromWebContents(event.sender);
    const options = { properties: ["openDirectory"] as Array<"openDirectory"> };
    const result = window
      ? await dialog.showOpenDialog(window, options)
      : await dialog.showOpenDialog(options);
    if (result.canceled || !result.filePaths[0]) return null;
    const selected = path.resolve(result.filePaths[0]);
    const home = path.resolve(app.getPath("home"));
    const blocked = [
      app.getPath("userData"),
      app.getPath("appData"),
      path.join(home, ".ssh"),
      path.join(home, ".config", "google-chrome"),
      path.join(home, ".config", "chromium"),
    ].map((value) => path.resolve(value).toLowerCase());
    const normalized = selected.toLowerCase();
    if (
      normalized === home.toLowerCase() ||
      blocked.some(
        (value) =>
          normalized === value || normalized.startsWith(value + path.sep),
      )
    )
      throw new Error("该目录包含敏感的应用或账户数据，不能授权给 MCP 服务");
    const stat = await fs.lstat(selected);
    if (!stat.isDirectory() || stat.isSymbolicLink())
      throw new Error("只能授权真实本地目录");
    return { path: selected, name: path.basename(selected) };
  });

  ipcMain.handle(IPC_CHANNELS.selectExtensionPackage, async (event) => {
    const window = BrowserWindow.fromWebContents(event.sender);
    const options = {
      properties: ["openFile"] as Array<"openFile">,
      filters: [{ name: "Amitia 扩展包", extensions: ["amitiax", "zip"] }],
    };
    const result = window
      ? await dialog.showOpenDialog(window, options)
      : await dialog.showOpenDialog(options);
    if (result.canceled || !result.filePaths[0]) return null;
    const selected = result.filePaths[0];
    const stat = await fs.lstat(selected);
    if (
      !stat.isFile() ||
      stat.isSymbolicLink() ||
      stat.size > 100 * 1024 * 1024
    )
      throw new Error("扩展包文件无效或超过 100 MB");
    const content = await fs.readFile(selected);
    return {
      name: path.basename(selected),
      size: content.length,
      base64: content.toString("base64"),
    };
  });

  ipcMain.handle(
    IPC_CHANNELS.saveExtensionPackage,
    async (event, request: { suggestedName?: unknown; base64?: unknown }) => {
      if (
        !request ||
        typeof request.suggestedName !== "string" ||
        typeof request.base64 !== "string"
      )
        throw new Error("导出保存参数无效");
      const suggestedName = path
        .basename(request.suggestedName)
        .replace(/[^A-Za-z0-9._-]/g, "-");
      if (!suggestedName || !/\.(amitiax|zip)$/i.test(suggestedName))
        throw new Error("导出文件名无效");
      const content = Buffer.from(request.base64, "base64");
      if (!content.length || content.length > 100 * 1024 * 1024)
        throw new Error("导出内容无效或超过 100 MB");
      const window = BrowserWindow.fromWebContents(event.sender);
      const options = {
        defaultPath: suggestedName,
        filters: [{ name: "Amitia 扩展包", extensions: ["amitiax", "zip"] }],
      };
      const result = window
        ? await dialog.showSaveDialog(window, options)
        : await dialog.showSaveDialog(options);
      if (result.canceled || !result.filePath) return { saved: false };
      const tempRoot = path.join(
        app.getPath("temp"),
        "amitia-extension-exports",
      );
      const tempFile = path.join(tempRoot, randomUUID());
      await fs.mkdir(tempRoot, { recursive: true });
      try {
        await fs.writeFile(tempFile, content, { mode: 0o600 });
        await fs.copyFile(tempFile, result.filePath);
        return { saved: true, fileName: path.basename(result.filePath) };
      } finally {
        await fs.rm(tempFile, { force: true });
      }
    },
  );

  ipcMain.handle(IPC_CHANNELS.minimizeWindow, (event) => {
    BrowserWindow.fromWebContents(event.sender)?.minimize();
  });

  ipcMain.handle(IPC_CHANNELS.toggleMaximizeWindow, (event) => {
    const win = BrowserWindow.fromWebContents(event.sender);
    if (!win) return false;
    if (win.isMaximized()) {
      win.unmaximize();
      return false;
    }
    win.maximize();
    return true;
  });

  ipcMain.handle(IPC_CHANNELS.closeWindow, (event) => {
    BrowserWindow.fromWebContents(event.sender)?.close();
  });

  ipcMain.handle(IPC_CHANNELS.quitApp, () => {
    app.quit();
  });

  ipcMain.handle("window-minimize", (event) => {
    BrowserWindow.fromWebContents(event.sender)?.minimize();
  });

  ipcMain.handle("window-toggle-maximize", (event) => {
    const win = BrowserWindow.fromWebContents(event.sender);
    if (!win) return false;
    if (win.isMaximized()) {
      win.unmaximize();
      return false;
    }
    win.maximize();
    return true;
  });

  ipcMain.handle("window-close", (event) => {
    BrowserWindow.fromWebContents(event.sender)?.close();
  });

  ipcMain.handle("window-is-maximized", () => {
    return BrowserWindow.getFocusedWindow()?.isMaximized() || false;
  });

  ipcMain.handle("get-window-type", (event) => {
    const senderWindow = BrowserWindow.fromWebContents(event.sender);
    const allWindows = BrowserWindow.getAllWindows();
    return senderWindow === allWindows[0] ? "main" : "child";
  });

  ipcMain.handle(IPC_CHANNELS.clipboardWriteText, async (_event, text: string) => {
    if (typeof text !== "string" || text.length === 0) {
      throw new Error("clipboard write failed: text is empty");
    }
    if (text.length > 1024 * 1024) {
      throw new Error("clipboard write failed: text exceeds 1MB limit");
    }
    clipboard.writeText(text);
  });

  ipcMain.handle(IPC_CHANNELS.setAuthToken, async (_event, token: string) => {
    setAuthToken(token || null);
  });

  ipcMain.handle(IPC_CHANNELS.getBackendAuthHeaders, async () => {
    return getDesktopAuthHeaders();
  });

  ipcMain.handle(IPC_CHANNELS.meshGetStatus, async () => {
    return getMeshStatus();
  });

  ipcMain.handle(IPC_CHANNELS.meshGetIdentity, async () => {
    return getMeshIdentity();
  });

  ipcMain.handle(IPC_CHANNELS.meshProvision, async (_event, cloudBaseUrl: string) => {
    if (!cloudBaseUrl || typeof cloudBaseUrl !== "string") {
      throw new Error("cloudBaseUrl is required");
    }
    const coordinator = getMeshCoordinator(getMainWindow ?? (() => null));
    await coordinator.provision(cloudBaseUrl);
    return { ok: true };
  });

  ipcMain.handle(IPC_CHANNELS.meshDeprovision, async () => {
    const coordinator = getMeshCoordinator(getMainWindow ?? (() => null));
    await coordinator.deprovision();
    return { ok: true };
  });

  ipcMain.handle(IPC_CHANNELS.meshCloudListDevices, async (_event, cloudBaseUrl: string) => {
    return cloudListDevices(cloudBaseUrl);
  });

  ipcMain.handle(IPC_CHANNELS.meshCloudRevokeDevice, async (_event, cloudBaseUrl: string, deviceId: string) => {
    await cloudRevokeDevice(cloudBaseUrl, deviceId);
    return { ok: true };
  });

  ipcMain.handle(IPC_CHANNELS.meshCloudProbe, async (_event, cloudBaseUrl: string, deviceId: string, runtimeId: string) => {
    return cloudProbeRuntime(cloudBaseUrl, deviceId, runtimeId);
  });

  ipcMain.handle("amitia:mesh:cloud:create-ticket", async (_event, cloudBaseUrl: string, label?: string) => {
    const identity = await getMeshIdentity();
    if (!identity) {
      throw new Error("local device-mesh identity unavailable");
    }
    return createBootstrapTicket(cloudBaseUrl, {
      deviceId: identity.deviceId,
      runtimeId: identity.runtimeId,
      platform: identity.platform,
      label: label ?? `Desktop ${identity.deviceId}`,
    });
  });
}
