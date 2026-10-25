import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { app, BrowserWindow, nativeTheme, shell } from "electron";
import { getInitialBrandImage } from "./branding";
import { isDevMode } from "./path-manager";
import { get as httpGet } from "node:http";
import { APP_PROTOCOL_ORIGIN } from "./app-protocol";
import { windowChromeOptions } from "./window-chrome";
import { IPC_CHANNELS } from "../shared/ipc";
import { attachExternalWebUIHost } from "./external-web-ui";

const currentDir = dirname(fileURLToPath(import.meta.url));
const MAIN_RENDERER_LOAD_TIMEOUT_MS = 15000;

const DEV_SERVER_URL = (
  process.env.VITE_DEV_SERVER_URL ||
  process.env.AMITIA_DESKTOP_DEV_SERVER_URL ||
  ""
).replace(/\/$/, "");

export function buildRendererURL(
  path: string,
  query: Record<string, string | undefined> = {},
): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    const normalized = value?.trim();
    if (normalized) search.set(key, normalized);
  }
  const params = search.toString();
  const hash = `#${normalizedPath}${params ? `?${params}` : ""}`;
  return isDevMode()
    ? `${DEV_SERVER_URL}/${hash}`
    : `${APP_PROTOCOL_ORIGIN}/${hash}`;
}

function healthCheck(url: string, timeoutMs = 3000): Promise<boolean> {
  return new Promise((resolve) => {
    const req = httpGet(url, (res) => {
      res.resume();
      resolve(res.statusCode === 200 || res.statusCode === 304);
    });
    req.on("error", () => resolve(false));
    req.setTimeout(timeoutMs, () => {
      req.destroy();
      resolve(false);
    });
  });
}

async function waitForDevServer(
  maxRetries = 40,
  intervalMs = 1500,
): Promise<boolean> {
  if (!DEV_SERVER_URL) return false;
  const target = `${DEV_SERVER_URL}/`;
  for (let i = 0; i < maxRetries; i++) {
    if (await healthCheck(target)) return true;
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  return false;
}

export function createMainWindow(): BrowserWindow {
  const preloadPath = join(currentDir, "../preload/index.cjs");

  const win = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 1060,
    minHeight: 640,
    title: "Amitia",
    icon: getInitialBrandImage(
      nativeTheme.shouldUseDarkColors ? "dark" : "light",
      "icon",
    ),
    ...windowChromeOptions(process.platform, nativeTheme.shouldUseDarkColors),
    show: false,
    webPreferences: {
      preload: preloadPath,
      sandbox: false,
      nodeIntegration: false,
      contextIsolation: true,
      webSecurity: true,
      webviewTag: true,
    },
  });
  attachExternalWebUIHost(win);

  const sendWindowState = () => {
    if (!win.isDestroyed() && !win.webContents.isDestroyed()) {
      win.webContents.send(IPC_CHANNELS.windowStateChanged, {
        fullscreen: win.isFullScreen(),
        maximized: win.isMaximized(),
      });
    }
  };
  win.on("enter-full-screen", sendWindowState);
  win.on("leave-full-screen", sendWindowState);
  win.on("maximize", sendWindowState);
  win.on("unmaximize", sendWindowState);
  win.webContents.on("did-finish-load", sendWindowState);

  win.webContents.setWindowOpenHandler(({ url }) => {
    if (
      url.startsWith("http://") ||
      url.startsWith("https://") ||
      url.startsWith("mailto:")
    ) {
      void shell.openExternal(url);
    }
    return { action: "deny" };
  });

  win.webContents.on("will-navigate", (event, url) => {
    const current = win.webContents.getURL();
    if (current && !isAllowedNavigation(url)) {
      event.preventDefault();
      void shell.openExternal(url);
    }
  });

  return win;
}

function withTimeout<T>(
  promise: Promise<T>,
  timeoutMs: number,
  message: string,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(message)), timeoutMs);
    promise.then(
      (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      (error) => {
        clearTimeout(timer);
        reject(error);
      },
    );
  });
}

async function loadErrorPage(win: BrowserWindow): Promise<void> {
  if (win.isDestroyed()) {
    return;
  }
  try {
    await win.loadURL(`${APP_PROTOCOL_ORIGIN}/core-error.html`);
  } catch (error) {
    console.error("[AmitiaDesktop] 启动错误页加载失败:", error);
  }
}

export async function revealMainWindow(win: BrowserWindow): Promise<void> {
  const firstPaint = new Promise<void>((resolve) => {
    const timer = setTimeout(resolve, 3000);
    win.once("ready-to-show", () => {
      clearTimeout(timer);
      resolve();
    });
  });

  try {
    await withTimeout(
      loadMainRenderer(win),
      MAIN_RENDERER_LOAD_TIMEOUT_MS,
      "主界面加载超时",
    );
  } catch (error) {
    console.error("[AmitiaDesktop] 主界面加载失败:", error);
    await loadErrorPage(win);
  }

  await firstPaint;

  if (!win.isDestroyed()) {
    win.show();
    win.focus();
  }
}

async function loadMainRenderer(win: BrowserWindow): Promise<void> {
  if (!isDevMode()) {
    await win.loadURL(`${APP_PROTOCOL_ORIGIN}/#/`);
    return;
  }

  const ready = await waitForDevServer();
  if (ready) {
    await win.loadURL(DEV_SERVER_URL);
    win.webContents.openDevTools({ mode: "detach" });
    return;
  }

  await win.loadURL(
    `data:text/html,<html><body style="background:#1a1a2e;color:#e0e0e0;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;font-family:system-ui"><div style="text-align:center"><h1 style="font-size:20px;margin-bottom:8px">无法连接到开发服务器</h1><p style="color:#888;margin-bottom:24px">${DEV_SERVER_URL || "未配置 VITE_DEV_SERVER_URL"}</p><button onclick="location.reload()" style="padding:8px 24px;background:#4a6cf7;color:#fff;border:none;border-radius:6px;cursor:pointer;font-size:14px">重试</button></div></body></html>`,
  );
}

function isAllowedNavigation(url: string): boolean {
  const devServerAllowed = DEV_SERVER_URL
    ? url.startsWith(DEV_SERVER_URL) ||
      url.startsWith(DEV_SERVER_URL.replace("127.0.0.1", "localhost"))
    : false;
  return (
    url.startsWith(`${APP_PROTOCOL_ORIGIN}/`) ||
    devServerAllowed
  );
}
