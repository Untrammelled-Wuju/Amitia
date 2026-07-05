import { fileURLToPath } from "node:url"
import { dirname, join, resolve } from "node:path"
import { app, BrowserWindow, shell } from "electron"

const currentDir = dirname(fileURLToPath(import.meta.url))

export function createMainWindow(): BrowserWindow {
  const preloadPath = join(currentDir, "../preload/index.cjs")

  const win = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 1060,
    minHeight: 640,
    title: "Amitia",
    frame: false,
    show: false,
    webPreferences: {
      preload: preloadPath,
      sandbox: false,
      nodeIntegration: false,
      contextIsolation: true,
      webSecurity: true,
    },
  })

  win.once("ready-to-show", () => {
    win.show()
  })

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

  const devServerURL = process.env.VITE_DEV_SERVER_URL || process.env.AMITIA_DESKTOP_DEV_SERVER_URL || "http://127.0.0.1:5178"
  if (!app.isPackaged) {
    void win.loadURL(devServerURL)
    win.webContents.openDevTools({ mode: "detach" })
  } else {
    void win.loadFile(resolve(currentDir, "../renderer/index.html"))
  }

  return win
}

function isAllowedNavigation(url: string): boolean {
  const devServerURL = process.env.VITE_DEV_SERVER_URL
  return url.startsWith("file://") || (devServerURL ? url.startsWith(devServerURL) : false) || url.startsWith("http://127.0.0.1:5178") || url.startsWith("http://localhost:5178")
}
