import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { BrowserWindow, nativeImage, Tray } from "electron";

const currentDir = dirname(fileURLToPath(import.meta.url));

export type BrandTheme = "light" | "dark";

const trayTemplatePath = join(currentDir, "../../resources/trayTemplate.png");
const trayPixelSize = { width: 16, height: 16 };

function getTrayImage(theme: BrandTheme) {
  if (process.platform === "darwin") {
    const template = nativeImage.createFromPath(trayTemplatePath);
    template.setTemplateImage(true);
    return template;
  }
  const variant = theme === "dark" ? "light" : "dark";
  return nativeImage
    .createFromPath(join(currentDir, `../../resources/tray-${variant}.png`))
    .resize(trayPixelSize);
}

function getBrandImage(theme: BrandTheme, target: "icon" | "tray") {
  if (target === "tray") return getTrayImage(theme);
  const variant = theme === "dark" ? "light" : "dark";
  return nativeImage.createFromPath(
    join(currentDir, `../../resources/icon-${variant}.ico`),
  );
}

export function applyBrandTheme(
  theme: BrandTheme,
  win: BrowserWindow | null,
  tray: Tray | null,
) {
  if (win && !win.isDestroyed()) win.setIcon(getBrandImage(theme, "icon"));
  if (process.platform !== "darwin" && tray && !tray.isDestroyed()) {
    tray.setImage(getTrayImage(theme));
  }
}

export function getInitialBrandImage(
  theme: BrandTheme,
  target: "icon" | "tray",
) {
  return getBrandImage(theme, target);
}
