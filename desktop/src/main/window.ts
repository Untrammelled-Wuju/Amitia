import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";
import { app, BrowserWindow, nativeTheme, shell } from "electron";
import { getInitialBrandImage } from "./branding";
import { get as httpGet } from "node:http";

const currentDir = dirname(fileURLToPath(import.meta.url));

const DEV_SERVER_URL =
  process.env.VITE_DEV_SERVER_URL ||
  process.env.AMITIA_DESKTOP_DEV_SERVER_URL ||
  "http://127.0.0.1:5178";

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
    frame: false,
    show: false,
    webPreferences: {
      preload: preloadPath,
      sandbox: false,
      nodeIntegration: false,
      contextIsolation: true,
      webSecurity: true,
    },
  });

  win.once("ready-to-show", () => {
    win.show();
  });

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

  if (!app.isPackaged) {
    void waitForDevServer().then((ok) => {
      if (ok) {
        void win.loadURL(DEV_SERVER_URL);
        win.webContents.openDevTools({ mode: "detach" });
      } else {
        win.loadURL(
          `data:text/html,<html><body style="background:#1a1a2e;color:#e0e0e0;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;font-family:system-ui"><div style="text-align:center"><h1 style="font-size:20px;margin-bottom:8px">无法连接到开发服务器</h1><p style="color:#888;margin-bottom:24px">${DEV_SERVER_URL}</p><button onclick="location.reload()" style="padding:8px 24px;background:#4a6cf7;color:#fff;border:none;border-radius:6px;cursor:pointer;font-size:14px">重试</button></div></body></html>`,
        );
      }
    });
  } else {
    void win.loadFile(resolve(currentDir, "../renderer/index.html"), {
      hash: "/",
    });
  }

  return win;
}

function isAllowedNavigation(url: string): boolean {
  return (
    url.startsWith("file://") ||
    url.startsWith(DEV_SERVER_URL) ||
    url.startsWith(DEV_SERVER_URL.replace("127.0.0.1", "localhost"))
  );
}
