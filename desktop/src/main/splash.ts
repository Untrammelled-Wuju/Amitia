import { app, BrowserWindow } from "electron";
import path from "node:path";
import { getInstallDir, isDevMode } from "./path-manager";

let splashWindow: BrowserWindow | null = null;

function getSplashPagePath(): string {
  if (isDevMode()) {
    return path.join(getInstallDir(), "src", "renderer", "splash.html");
  }
  return path.join(app.getAppPath(), "dist", "renderer", "splash.html");
}

export function showSplashWindow(): void {
  if (splashWindow && !splashWindow.isDestroyed()) {
    return;
  }

  const splash = new BrowserWindow({
    width: 180,
    height: 180,
    frame: false,
    transparent: true,
    hasShadow: false,
    resizable: false,
    movable: false,
    minimizable: false,
    maximizable: false,
    fullscreenable: false,
    closable: false,
    focusable: false,
    skipTaskbar: true,
    alwaysOnTop: true,
    center: true,
    show: false,
    backgroundColor: "#00000000",
    webPreferences: {
      sandbox: true,
      contextIsolation: true,
      nodeIntegration: false,
      devTools: false,
    },
  });

  splash.setMenuBarVisibility(false);
  splash.setIgnoreMouseEvents(true);
  splash.on("close", (event) => {
    if (!splash.isDestroyed()) {
      event.preventDefault();
    }
  });
  splash.once("ready-to-show", () => {
    if (!splash.isDestroyed()) {
      splash.show();
      console.log("[Splash] 启动加载页已显示");
    }
  });

  splashWindow = splash;

  void splash.loadFile(getSplashPagePath()).catch((error) => {
    console.error("[Splash] 启动加载页加载失败:", error);
    closeSplashWindow();
  });
}

export function closeSplashWindow(): void {
  const splash = splashWindow;
  splashWindow = null;
  if (splash && !splash.isDestroyed()) {
    splash.destroy();
    console.log("[Splash] 启动加载页已关闭");
  }
}
