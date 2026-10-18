import { app, BrowserWindow } from "electron";
import path from "node:path";
import { getInstallDir, isDevMode } from "./path-manager";

let splashWindow: BrowserWindow | null = null;
let splashShowTimer: ReturnType<typeof setTimeout> | null = null;
let splashFailureHandler: ((error: Error) => void) | null = null;

function getSplashPagePath(): string {
  if (isDevMode()) {
    return path.join(getInstallDir(), "src", "renderer", "splash.html");
  }
  return path.join(app.getAppPath(), "dist", "renderer", "splash.html");
}

export function showSplashWindow(
  onFailure?: (error: Error) => void,
): void {
  if (splashWindow && !splashWindow.isDestroyed()) {
    return;
  }
  if (onFailure) {
    splashFailureHandler = onFailure;
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
  splash.setAlwaysOnTop(true, "floating");
  splash.on("close", (event) => {
    if (!splash.isDestroyed()) {
      event.preventDefault();
    }
  });

  splashWindow = splash;

  let shown = false;
  let failed = false;
  const showSplash = () => {
    if (shown || splash.isDestroyed() || splashWindow !== splash) {
      return;
    }
    shown = true;
    splash.center();
    splash.showInactive();
    splash.moveTop();
    console.log("[Splash] 启动加载页已显示");
  };

  const failSplash = (error: unknown) => {
    if (failed || splashWindow !== splash) {
      return;
    }
    failed = true;
    console.error("[Splash] 启动加载页加载失败:", error);
    closeSplashWindow();
    const failure =
      error instanceof Error ? error : new Error(String(error));
    splashFailureHandler?.(failure);
  };

  splash.once("ready-to-show", showSplash);
  splash.webContents.once("did-finish-load", showSplash);
  splashShowTimer = setTimeout(showSplash, 1000);

  splash.webContents.on("render-process-gone", (_event, details) => {
    failSplash(new Error(`启动加载页渲染进程退出: ${details.reason}`));
  });
  splash.webContents.on(
    "did-fail-load",
    (_event, errorCode, errorDescription, validatedURL, isMainFrame) => {
      if (!isMainFrame) {
        return;
      }
      failSplash(
        new Error(
          `启动加载页加载失败: ${errorCode} ${errorDescription} ${validatedURL}`,
        ),
      );
    },
  );

  void splash.loadFile(getSplashPagePath()).catch((error) => {
    failSplash(error);
  });
}

export function revealSplashWindow(): void {
  const splash = splashWindow;
  if (!splash || splash.isDestroyed()) {
    showSplashWindow(splashFailureHandler ?? undefined);
    return;
  }
  splash.center();
  splash.showInactive();
  splash.moveTop();
}

export function closeSplashWindow(): void {
  if (splashShowTimer) {
    clearTimeout(splashShowTimer);
    splashShowTimer = null;
  }
  const splash = splashWindow;
  splashWindow = null;
  if (splash && !splash.isDestroyed()) {
    splash.destroy();
    console.log("[Splash] 启动加载页已关闭");
  }
}
