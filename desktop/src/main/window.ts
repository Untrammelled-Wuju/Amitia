import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { app, BrowserWindow, nativeTheme, shell } from "electron";
import { getInitialBrandImage } from "./branding";
import { getInstallDir, isDevMode } from "./path-manager";
import { get as httpGet } from "node:http";
import { inspect } from "node:util";
import { APP_PROTOCOL_ORIGIN } from "./app-protocol";
import { windowChromeOptions } from "./window-chrome";
import { IPC_CHANNELS } from "../shared/ipc";
import { attachExternalWebUIHost } from "./external-web-ui";

const currentDir = dirname(fileURLToPath(import.meta.url));
const MAIN_RENDERER_LOAD_TIMEOUT_MS = 15000;
const RENDERER_RETRY_URL = `${APP_PROTOCOL_ORIGIN}/core-error-retry`;

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

function healthCheck(url: string, timeoutMs = 1000): Promise<void> {
  return new Promise((resolve, reject) => {
    const req = httpGet(url, (res) => {
      res.resume();
      if (res.statusCode === 200 || res.statusCode === 304) {
        resolve();
      } else {
        reject(new Error(`开发服务器 ${url} 返回 HTTP ${res.statusCode} ${res.statusMessage ?? ""}`));
      }
    });
    req.on("error", reject);
    req.setTimeout(timeoutMs, () => {
      req.destroy(new Error(`开发服务器 ${url} 请求超时`));
    });
  });
}

function installMacDeveloperShortcut(win: BrowserWindow): void {
  if (process.platform !== "darwin" || !app.isPackaged) return;
  win.webContents.on("before-input-event", (event, input) => {
    if (input.type !== "keyDown" || input.key !== "F12") return;
    event.preventDefault();
    win.webContents.toggleDevTools();
  });
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
  installMacDeveloperShortcut(win);
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
    if (url === RENDERER_RETRY_URL) {
      event.preventDefault();
      void revealMainWindow(win);
      return;
    }
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

async function loadErrorPage(win: BrowserWindow, error: unknown): Promise<void> {
  if (win.isDestroyed()) {
    return;
  }
  try {
    if (isDevMode()) {
      await win.loadFile(join(getInstallDir(), "src", "renderer", "core-error.html"));
    } else {
      await win.loadURL(`${APP_PROTOCOL_ORIGIN}/core-error.html`);
    }
    const detail = inspect(error, { depth: null, maxArrayLength: null, maxStringLength: null });
    await win.webContents.executeJavaScript(
      `document.querySelector("#original-error").textContent = ${JSON.stringify(detail)};`,
    );
  } catch (pageError) {
    console.error("[AmitiaDesktop] 启动错误页加载失败:", new AggregateError([error, pageError]));
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
    await loadMainRenderer(win);
  } catch (error) {
    console.error("[AmitiaDesktop] 主界面加载失败:", error);
    if (!win.isDestroyed()) {
      win.webContents.stop();
      await loadErrorPage(win, error);
    }
  }

  await firstPaint;

  if (!win.isDestroyed()) {
    win.show();
    win.focus();
  }
}

async function loadMainRenderer(win: BrowserWindow): Promise<void> {
  if (!isDevMode()) {
    await withTimeout(win.loadURL(`${APP_PROTOCOL_ORIGIN}/#/`), MAIN_RENDERER_LOAD_TIMEOUT_MS, "主界面加载超时");
    return;
  }
  if (!DEV_SERVER_URL) {
    throw new Error("未配置 VITE_DEV_SERVER_URL");
  }
  const deadline = Date.now() + MAIN_RENDERER_LOAD_TIMEOUT_MS;
  for (;;) {
    try {
      await healthCheck(`${DEV_SERVER_URL}/`);
      break;
    } catch (error) {
      if (Date.now() >= deadline) {
        throw new Error(`无法连接到开发服务器 ${DEV_SERVER_URL}`, { cause: error });
      }
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
  }
  const remaining = Math.max(1, deadline - Date.now());
  await withTimeout(win.loadURL(DEV_SERVER_URL), remaining, "主界面加载超时");
  win.webContents.openDevTools({ mode: "detach" });
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
