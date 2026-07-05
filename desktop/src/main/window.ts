import { fileURLToPath } from "node:url"
import { dirname, join, resolve } from "node:path"
import { app, BrowserWindow, shell } from "electron"

const currentDir = dirname(fileURLToPath(import.meta.url))

export function createMainWindow(): BrowserWindow {
  const win = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 960,
    minHeight: 640,
    title: "Amitia",
    frame: false,
    show: false,
    webPreferences: {
      preload: join(currentDir, "../preload/index.mjs"),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true,
      webSecurity: true,
    },
  })

  win.once("ready-to-show", () => win.show())
  win.webContents.on("did-finish-load", () => {
    void injectDesktopTitleBar(win)
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

const desktopTitleBarCSS = String.raw`
.drag {
  -webkit-app-region: drag;
}
.no-drag {
  -webkit-app-region: no-drag;
}
#WindowControlButtons {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 1000;
  height: 34px;
  box-sizing: border-box;
  display: flex;
  align-items: center;
    justify-content: space-between;
  background: var(--console-sidebar, rgba(248,251,255,0.96));
  border-bottom: 1px solid var(--ac-color-border-light, #e5e7eb);
  color: var(--ac-color-text, #1f2937);
  font: 12px/1.2 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  user-select: none;
  -webkit-app-region: drag;
}
#WindowControlButtons .title-content {
  height: 100%;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  font-weight: 600;
}
#WindowControlButtons .title-mark {
  width: 18px;
  height: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 5px;
  background: var(--ac-color-primary, #2563eb);
  color: #ffffff;
  font-size: 11px;
}
#WindowControlButtons .title-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
#WindowControlButtons .window-actions {
  height: 100%;
  display: flex;
  -webkit-app-region: no-drag;
}
#WindowControlButtons .icon {
  width: 46px;
  height: 100%;
  border: 0;
  border-radius: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  color: var(--ac-color-text-secondary, #64748b);
  cursor: default;
  -webkit-app-region: no-drag;
}
#WindowControlButtons .icon:hover {
  background: rgba(128, 128, 128, 0.2);
  color: var(--ac-color-text, #111827);
}
#WindowControlButtons .icon.close:hover {
  background: #e81123;
  color: #ffffff;
}
#WindowControlButtons .icon svg {
  width: 14px;
  height: 14px;
  fill: currentColor;
}
html.amitia-desktop-shell body {
  padding-top: 34px;
  box-sizing: border-box;
  overflow: hidden;
}
html.amitia-desktop-shell #app {
  height: calc(100vh - 34px);
}
html.amitia-desktop-shell #app > .app-shell {
  height: 100%;
}

html.amitia-desktop-shell .el-message {
  top: 54px !important;
}
html.amitia-desktop-shell .el-notification {
  top: 54px !important;
}
`

async function injectDesktopTitleBar(win: BrowserWindow): Promise<void> {
  await win.webContents.executeJavaScript(`
(() => {
  if (document.getElementById("WindowControlButtons")) return
  const style = document.createElement("style")
  style.id = "WindowControlButtons-style"
  style.textContent = ${JSON.stringify(desktopTitleBarCSS)}
  const button = (title, svg, action) => {
    const node = document.createElement("div")
    node.className = "icon no-drag"
    node.title = title
    node.setAttribute("aria-label", title)
    node.innerHTML = svg
    node.addEventListener("click", action)
    return node
  }
  const bar = document.createElement("div")
  bar.id = "WindowControlButtons"
  bar.className = "drag"
  const title = document.createElement("div")
  title.className = "title-content"
  const mark = document.createElement("span")
  mark.className = "title-mark"
  mark.textContent = "A"
  const text = document.createElement("span")
  text.className = "title-text"
  text.textContent = "Amitia"
  const actions = document.createElement("div")
  actions.className = "window-actions"
  title.append(mark, text)
  actions.append(
    button("最小化", '<svg viewBox="0 0 1024 1024"><path d="M863.7 552.5H160.3c-10.6 0-19.2-8.6-19.2-19.2v-41.7c0-10.6 8.6-19.2 19.2-19.2h703.3c10.6 0 19.2 8.6 19.2 19.2v41.7c0 10.6-8.5 19.2-19.1 19.2z"></path></svg>', () => window.electronWindowApi?.minimize?.()),
    button("最大化", '<svg viewBox="0 0 1024 1024"><path d="M770.9 923.3H253.1c-83.8 0-151.9-68.2-151.9-151.9V253.6c0-83.8 68.2-151.9 151.9-151.9h517.8c83.8 0 151.9 68.2 151.9 151.9v517.8c0 83.8-68.1 151.9-151.9 151.9zM253.1 181.7c-39.7 0-71.9 32.3-71.9 71.9v517.8c0 39.7 32.3 71.9 71.9 71.9h517.8c39.7 0 71.9-32.3 71.9-71.9V253.6c0-39.7-32.3-71.9-71.9-71.9H253.1z"></path></svg>', () => window.electronWindowApi?.toggleMaximize?.()),
    button("关闭", '<svg viewBox="0 0 1024 1024"><path d="M897.6 183.5L183 898.1c-7.5 7.5-19.6 7.5-27.1 0l-29.5-29.5c-7.5-7.5-7.5-19.6 0-27.1L841 126.9c7.5-7.5 19.6-7.5 27.1 0l29.5 29.5c7.5 7.4 7.5 19.6 0 27.1z"></path><path d="M183 126.9l714.7 714.7c7.5 7.5 7.5 19.6 0 27.1l-29.5 29.5c-7.5 7.5-19.6 7.5-27.1 0L126.4 183.5c-7.5-7.5-7.5-19.6 0-27.1l29.5-29.5c7.4-7.5 19.6-7.5 27.1 0z"></path></svg>', () => window.electronWindowApi?.close?.()),
  )
  actions.lastElementChild?.classList.add("close")
  bar.append(title, actions)
  document.documentElement.classList.add("amitia-desktop-shell")
  document.head.append(style)
  document.body.prepend(bar)
})()
`, true)
}
