import { BrowserWindow, ipcMain, screen, shell } from "electron";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { IPC_CHANNELS } from "../shared/ipc";
import type {
  RealtimeCallMode,
  RealtimeCallWindowRequest,
} from "../shared/types";
import { buildRendererURL } from "./window";

const currentDir = dirname(fileURLToPath(import.meta.url));
const CALL_WINDOW_WIDTH = 390;
const CALL_WINDOW_HEIGHT = 680;

let callWindow: BrowserWindow | null = null;
let handlersRegistered = false;

function normalizeMode(value: unknown): RealtimeCallMode {
  return value === "video" || value === "screen" ? value : "voice";
}

function normalizeText(value: unknown, maxLength = 2048): string {
  return typeof value === "string" ? value.trim().slice(0, maxLength) : "";
}

function resolveWindowPosition(mainWindow: BrowserWindow | null) {
  const workArea = screen.getDisplayNearestPoint(
    screen.getCursorScreenPoint(),
  ).workArea;
  const mainBounds = mainWindow?.getBounds();
  if (!mainBounds) {
    return {
      x: workArea.x + workArea.width - CALL_WINDOW_WIDTH - 24,
      y: workArea.y + workArea.height - CALL_WINDOW_HEIGHT - 24,
    };
  }
  return {
    x: Math.min(
      Math.max(workArea.x + 12, mainBounds.x + mainBounds.width - CALL_WINDOW_WIDTH - 24),
      workArea.x + workArea.width - CALL_WINDOW_WIDTH - 12,
    ),
    y: Math.min(
      Math.max(workArea.y + 12, mainBounds.y + mainBounds.height - CALL_WINDOW_HEIGHT - 24),
      workArea.y + workArea.height - CALL_WINDOW_HEIGHT - 12,
    ),
  };
}

export function registerRealtimeCallWindowHandlers(
  getMainWindow: () => BrowserWindow | null,
): void {
  if (handlersRegistered) return;
  handlersRegistered = true;

  ipcMain.handle(
    IPC_CHANNELS.openRealtimeCallWindow,
    async (_event, payload: RealtimeCallWindowRequest) => {
      if (callWindow && !callWindow.isDestroyed()) {
        if (callWindow.isMinimized()) callWindow.restore();
        callWindow.show();
        callWindow.focus();
        return { opened: true };
      }

      const mainWindow = getMainWindow();
      const position = resolveWindowPosition(mainWindow);
      const preloadPath = join(currentDir, "../preload/index.cjs");
      const window = new BrowserWindow({
        width: CALL_WINDOW_WIDTH,
        height: CALL_WINDOW_HEIGHT,
        minWidth: 350,
        minHeight: 560,
        x: position.x,
        y: position.y,
        title: "Amitia 通话",
        frame: false,
        show: false,
        resizable: true,
        maximizable: false,
        alwaysOnTop: true,
        backgroundColor: "#121212",
        webPreferences: {
          preload: preloadPath,
          sandbox: false,
          nodeIntegration: false,
          contextIsolation: true,
          webSecurity: true,
        },
      });
      callWindow = window;

      window.webContents.setWindowOpenHandler(({ url }) => {
        if (url.startsWith("http://") || url.startsWith("https://")) {
          void shell.openExternal(url);
        }
        return { action: "deny" };
      });

      window.once("ready-to-show", () => {
        if (!window.isDestroyed()) window.show();
      });

      window.on("closed", () => {
        callWindow = null;
        const target = getMainWindow();
        if (target && !target.isDestroyed()) {
          target.webContents.send(IPC_CHANNELS.realtimeCallWindowClosed);
        }
      });

      const request = payload ?? { mode: "voice" };
      const url = buildRendererURL("/call-window", {
        mode: normalizeMode(request.mode),
        voiceType: normalizeText(request.voiceType, 256),
        resourceId: normalizeText(request.resourceId, 256),
        conversationId: normalizeText(request.conversationId, 256),
        charName: normalizeText(request.charName, 256),
        charAvatar: normalizeText(request.charAvatar),
      });
      await window.loadURL(url);
      return { opened: true };
    },
  );

  ipcMain.handle(IPC_CHANNELS.closeRealtimeCallWindow, () => {
    if (callWindow && !callWindow.isDestroyed()) {
      callWindow.close();
    }
  });
}
