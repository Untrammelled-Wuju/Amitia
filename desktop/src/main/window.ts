import { join, resolve } from "node:path"
import { BrowserWindow, shell } from "electron"

export function createMainWindow(): BrowserWindow {
  const win = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 960,
    minHeight: 640,
    title: "Amitia",
    show: false,
    webPreferences: {
      preload: join(__dirname, "../preload/index.js"),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true,
      webSecurity: true,
    },
  })

  win.once("ready-to-show", () => win.show())

  win.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith("http://") || url.startsWith("https://") || url.startsWith("mailto:")) {
      void shell.openExternal(url)
    }
    return { action: "deny" }
  })

  win.webContents.on("will-navigate", (event, url) => {
    const current = win.webContents.getURL()
    if (current && !isAllowedNavigation(url)) {
      event.preventDefault()
      void shell.openExternal(url)
    }
  })

  const devServerURL = process.env.AMITIA_DESKTOP_DEV_SERVER_URL || "http://127.0.0.1:5178"
  if (!process.env.NODE_ENV || process.env.NODE_ENV === "development") {
    void win.loadURL(devServerURL)
    win.webContents.openDevTools({ mode: "detach" })
  } else {
    void win.loadFile(resolve(__dirname, "../renderer/index.html"))
  }

  return win
}

function isAllowedNavigation(url: string): boolean {
  return url.startsWith("file://") || url.startsWith("http://127.0.0.1:5178") || url.startsWith("http://localhost:5178")
}
