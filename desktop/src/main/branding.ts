import { fileURLToPath } from "node:url"
import { dirname, join } from "node:path"
import { BrowserWindow, nativeImage, Tray } from "electron"

const currentDir = dirname(fileURLToPath(import.meta.url))

export type BrandTheme = "light" | "dark"

function getBrandImage(theme: BrandTheme, target: "icon" | "tray") {
  const variant = theme === "dark" ? "light" : "dark"
  const extension = target === "icon" ? "ico" : "png"
  return nativeImage.createFromPath(join(currentDir, `../../resources/${target}-${variant}.${extension}`))
}

export function applyBrandTheme(theme: BrandTheme, win: BrowserWindow | null, tray: Tray | null) {
  if (win && !win.isDestroyed()) win.setIcon(getBrandImage(theme, "icon"))
  if (tray && !tray.isDestroyed()) tray.setImage(getBrandImage(theme, "tray"))
}

export function getInitialBrandImage(theme: BrandTheme, target: "icon" | "tray") {
  return getBrandImage(theme, target)
}
