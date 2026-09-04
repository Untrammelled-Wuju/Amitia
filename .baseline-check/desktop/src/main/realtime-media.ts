import { BrowserWindow, desktopCapturer, session, type WebContents } from "electron";

let registered = false;

export function registerRealtimeMediaHandlers(getMainWindow: () => BrowserWindow | null): void {
  if (registered) return;
  registered = true;

  session.defaultSession.setPermissionRequestHandler((webContents, permission, callback) => {
    const mainWindow = getMainWindow();
    const trusted = mainWindow != null && !mainWindow.isDestroyed() && mainWindow.webContents.id === webContents.id;
    if (!trusted) {
      callback(false);
      return;
    }
    callback(permission === "media" || permission === "display-capture");
  });

  session.defaultSession.setPermissionCheckHandler((webContents, permission) => {
    const mainWindow = getMainWindow();
    const trusted = isTrustedWebContents(webContents, mainWindow);
    return trusted && (permission === "media" || permission === "display-capture");
  });

  session.defaultSession.setDisplayMediaRequestHandler(
    async (request, callback) => {
      const mainWindow = getMainWindow();
      if (!mainWindow || mainWindow.isDestroyed()) {
        callback({});
        return;
      }
      const requestTop = request.frame.top;
      const mainFrame = mainWindow.webContents.mainFrame;
      const trustedFrame =
        requestTop != null &&
        requestTop.processId === mainFrame.processId &&
        requestTop.routingId === mainFrame.routingId;
      if (!trustedFrame) {
        callback({});
        return;
      }
      try {
        const sources = await desktopCapturer.getSources({
          types: ["screen", "window"],
          thumbnailSize: { width: 0, height: 0 },
          fetchWindowIcons: false,
        });
        const preferred = sources.find((source) => source.id.startsWith("screen:")) ?? sources[0];
        if (!preferred) {
          callback({});
          return;
        }
        callback({ video: preferred });
      } catch {
        callback({});
      }
    },
    { useSystemPicker: true },
  );
}

function isTrustedWebContents(webContents: WebContents | null, mainWindow: BrowserWindow | null): boolean {
  if (!webContents || !mainWindow || mainWindow.isDestroyed()) return false;
  return webContents.id === mainWindow.webContents.id;
}
