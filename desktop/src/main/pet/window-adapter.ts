import { app, BrowserWindow, screen } from "electron";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  PET_WINDOW_SCALE_DEFAULT,
  PET_WINDOW_SCALE_MAX,
  PET_WINDOW_SCALE_MIN,
} from "./types";
import type {
  ClickThroughMode,
  DesktopPetWindowOptions,
  PetRuntimePositionRecord,
  PetWindowEvent,
  PetWindowEventListener,
  Position,
  ScreenInfo,
} from "./types";

const currentDir = dirname(fileURLToPath(import.meta.url));

const VISIBLE_MARGIN = 20;
const EDGE_OFFSET = 40;

function clampScale(scale: number): number {
  if (!Number.isFinite(scale)) return PET_WINDOW_SCALE_DEFAULT;
  return Math.min(PET_WINDOW_SCALE_MAX, Math.max(PET_WINDOW_SCALE_MIN, scale));
}

function isClickThroughMode(value: unknown): value is ClickThroughMode {
  return value === "alpha" || value === "boundingBox" || value === "none";
}

export class DesktopPetWindowAdapter {
  private window: BrowserWindow | null = null;

  private readonly listeners: Map<PetWindowEvent, Set<PetWindowEventListener>> =
    new Map();

  private options: DesktopPetWindowOptions;

  private readonly onDisplayRemoved = (
    _event: Electron.Event,
    display: Electron.Display,
  ): void => {
    this.handleDisplayRemoved(display);
  };

  private readonly onDisplayMetricsChanged = (
    _event: Electron.Event,
    _display: Electron.Display,
    changedMetrics: string[],
  ): void => {
    if (
      changedMetrics.includes("bounds") ||
      changedMetrics.includes("workArea") ||
      changedMetrics.includes("scaleFactor")
    ) {
      this.ensureVisible();
    }
  };

  constructor(options: DesktopPetWindowOptions) {
    this.options = {
      ...options,
      scale: clampScale(options.scale),
      clickThroughMode: isClickThroughMode(options.clickThroughMode)
        ? options.clickThroughMode
        : "none",
    };
  }

  async create(): Promise<BrowserWindow> {
    if (this.window && !this.window.isDestroyed()) {
      return this.window;
    }

    const { width, height } = this.calculateSize();
    const initial = this.resolveInitialPosition();
    const preloadPath = join(currentDir, "../preload/index.cjs");

    const constructorOptions: Electron.BrowserWindowConstructorOptions = {
      width,
      height,
      x: Math.round(initial.x),
      y: Math.round(initial.y),
      transparent: true,
      frame: false,
      skipTaskbar: true,
      alwaysOnTop: this.options.alwaysOnTop,
      resizable: false,
      hasShadow: false,
      show: false,
      focusable: true,
      webPreferences: {
        preload: preloadPath,
        sandbox: false,
        nodeIntegration: false,
        contextIsolation: true,
        webSecurity: true,
        backgroundThrottling: false,
      },
    };

    if (process.platform === "darwin") {
      constructorOptions.type = "panel";
    }

    this.window = new BrowserWindow(constructorOptions);

    this.configureForPlatform();
    this.applyClickThroughMode(this.options.clickThroughMode ?? "none");
    this.registerWindowEvents();
    this.registerScreenEvents();

    this.window.once("ready-to-show", () => {
      this.window?.show();
    });

    return this.window;
  }

  async destroy(): Promise<void> {
    this.unregisterScreenEvents();
    const win = this.window;
    this.window = null;
    if (win && !win.isDestroyed()) {
      win.close();
    }
    this.listeners.clear();
  }

  async setPosition(x: number, y: number, screenId?: string): Promise<void> {
    const win = this.window;
    if (!win || win.isDestroyed()) return;

    if (screenId) {
      const target = this.findScreenById(screenId);
      if (target) {
        const physicalX = Math.round(target.bounds.x + x);
        const physicalY = Math.round(target.bounds.y + y);
        win.setPosition(physicalX, physicalY);
        return;
      }
    }

    win.setPosition(Math.round(x), Math.round(y));
  }

  getPosition(): Position {
    const win = this.window;
    if (!win || win.isDestroyed()) {
      const fallback = this.options.position ?? { x: 0, y: 0 };
      return { x: fallback.x, y: fallback.y, screenId: fallback.screenId };
    }
    const [x, y] = win.getPosition();
    const display = screen.getDisplayNearestPoint({ x, y });
    return {
      x,
      y,
      screenId: display ? String(display.id) : undefined,
    };
  }

  async setScale(scale: number): Promise<void> {
    const next = clampScale(scale);
    this.options.scale = next;
    this.recalculateSize();
  }

  async setAlwaysOnTop(alwaysOnTop: boolean): Promise<void> {
    this.options.alwaysOnTop = alwaysOnTop;
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    if (alwaysOnTop) {
      const level: "floating" | "screen-saver" =
        process.platform === "darwin" ? "floating" : "screen-saver";
      win.setAlwaysOnTop(true, level);
    } else {
      win.setAlwaysOnTop(false);
    }
  }

  async setClickThroughMode(mode: string): Promise<void> {
    const next: ClickThroughMode = isClickThroughMode(mode) ? mode : "none";
    this.options.clickThroughMode = next;
    this.applyClickThroughMode(next);
  }

  on(event: PetWindowEvent, listener: PetWindowEventListener): void {
    let set = this.listeners.get(event);
    if (!set) {
      set = new Set();
      this.listeners.set(event, set);
    }
    set.add(listener);
  }

  off(event: PetWindowEvent, listener: PetWindowEventListener): void {
    const set = this.listeners.get(event);
    if (!set) return;
    set.delete(listener);
  }

  isCreated(): boolean {
    return !!this.window && !this.window.isDestroyed();
  }

  getNativeWindow(): BrowserWindow | null {
    const win = this.window;
    return win && !win.isDestroyed() ? win : null;
  }

  getOptions(): DesktopPetWindowOptions {
    return { ...this.options };
  }

  recalculateSize(): { width: number; height: number } {
    const win = this.window;
    const { width, height } = this.calculateSize();
    if (win && !win.isDestroyed()) {
      win.setSize(width, height);
      this.ensureVisible();
    }
    this.emit("resize");
    return { width, height };
  }

  getDpiScale(): number {
    const win = this.window;
    if (!win || win.isDestroyed()) {
      return screen.getPrimaryDisplay().scaleFactor || 1;
    }
    const [x, y] = win.getPosition();
    const display = screen.getDisplayNearestPoint({ x, y });
    return display ? display.scaleFactor || 1 : 1;
  }

  listScreens(): ScreenInfo[] {
    const primaryId = screen.getPrimaryDisplay().id;
    return screen.getAllDisplays().map((d) => ({
      id: String(d.id),
      bounds: { ...d.bounds },
      workArea: { ...d.workArea },
      scaleFactor: d.scaleFactor,
      isPrimary: d.id === primaryId,
      label: d.label,
    }));
  }

  restorePosition(record: PetRuntimePositionRecord): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;

    if (typeof record.scale === "number" && Number.isFinite(record.scale)) {
      this.options.scale = clampScale(record.scale);
      this.recalculateSize();
    }

    const target = record.screenId
      ? this.findScreenById(record.screenId)
      : null;

    if (!target) {
      this.ensureVisible();
      return;
    }

    const x = target.bounds.x + record.x;
    const y = target.bounds.y + record.y;
    win.setPosition(Math.round(x), Math.round(y));
    this.ensureVisible();
  }

  moveToScreen(screenId: string): boolean {
    const win = this.window;
    if (!win || win.isDestroyed()) return false;
    const target = this.findScreenById(screenId);
    if (!target) return false;

    const [currentX, currentY] = win.getPosition();
    const currentDisplay = screen.getDisplayNearestPoint({
      x: currentX,
      y: currentY,
    });
    const baseBounds = currentDisplay
      ? currentDisplay.bounds
      : screen.getPrimaryDisplay().bounds;
    const relX = currentX - baseBounds.x;
    const relY = currentY - baseBounds.y;

    const newX = target.bounds.x + relX;
    const newY = target.bounds.y + relY;
    win.setPosition(Math.round(newX), Math.round(newY));
    this.ensureVisible();
    return true;
  }

  snapshotRuntimePosition(): PetRuntimePositionRecord {
    const pos = this.getPosition();
    const display = pos.screenId
      ? this.findScreenById(pos.screenId)
      : screen.getDisplayNearestPoint({ x: pos.x, y: pos.y });
    const bounds = display ? display.bounds : { x: 0, y: 0 };
    return {
      screenId: display
        ? String(display.id)
        : String(screen.getPrimaryDisplay().id),
      x: pos.x - bounds.x,
      y: pos.y - bounds.y,
      scale: this.options.scale,
    };
  }

  private calculateSize(): { width: number; height: number } {
    const scale = clampScale(this.options.scale);
    const width = Math.max(1, Math.round(this.options.canvasWidth * scale));
    const height = Math.max(1, Math.round(this.options.canvasHeight * scale));
    return { width, height };
  }

  private resolveInitialPosition(): Position {
    const pos = this.options.position;
    if (pos && typeof pos.x === "number" && typeof pos.y === "number") {
      if (pos.screenId) {
        const target = this.findScreenById(pos.screenId);
        if (target) {
          return {
            x: target.bounds.x + pos.x,
            y: target.bounds.y + pos.y,
            screenId: String(target.id),
          };
        }
      }
      return { x: pos.x, y: pos.y };
    }

    const primary = screen.getPrimaryDisplay();
    const { width, height } = this.calculateSize();
    return {
      x: primary.workArea.x + primary.workArea.width - width - EDGE_OFFSET,
      y: primary.workArea.y + primary.workArea.height - height - EDGE_OFFSET,
      screenId: String(primary.id),
    };
  }

  private configureForPlatform(): void {
    if (process.platform === "win32") {
      this.configureForWindows();
    } else if (process.platform === "darwin") {
      this.configureForMacOS();
    } else {
      this.configureForLinux();
    }
  }

  private configureForWindows(): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    win.setSkipTaskbar(true);
    if (this.options.alwaysOnTop) {
      win.setAlwaysOnTop(true, "screen-saver");
    }
  }

  private configureForMacOS(): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    win.setSkipTaskbar(true);
    if (this.options.alwaysOnTop) {
      win.setAlwaysOnTop(true, "floating");
    }
    if (app.dock) {
      app.dock.hide();
    }
  }

  private configureForLinux(): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    win.setSkipTaskbar(true);
    if (this.options.alwaysOnTop) {
      win.setAlwaysOnTop(true);
    }
  }

  private applyClickThroughMode(mode: ClickThroughMode): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    switch (mode) {
      case "none":
        win.setIgnoreMouseEvents(false);
        break;
      case "alpha":
      case "boundingBox":
        win.setIgnoreMouseEvents(false, { forward: true });
        break;
    }
  }

  private findScreenById(screenId: string): Electron.Display | null {
    const all = screen.getAllDisplays();
    return all.find((d) => String(d.id) === screenId) ?? null;
  }

  private ensureVisible(): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;

    const [x, y] = win.getPosition();
    const [w, h] = win.getSize();
    const display = screen.getDisplayMatching({
      x,
      y,
      width: w,
      height: h,
    });
    if (!display) return;

    const wa = display.workArea;
    let newX = x;
    let newY = y;

    if (x + w <= wa.x + VISIBLE_MARGIN) {
      newX = wa.x + VISIBLE_MARGIN - w;
    }
    if (y + h <= wa.y + VISIBLE_MARGIN) {
      newY = wa.y + VISIBLE_MARGIN - h;
    }
    if (x >= wa.x + wa.width - VISIBLE_MARGIN) {
      newX = wa.x + wa.width - VISIBLE_MARGIN;
    }
    if (y >= wa.y + wa.height - VISIBLE_MARGIN) {
      newY = wa.y + wa.height - VISIBLE_MARGIN;
    }

    if (newX !== x || newY !== y) {
      win.setPosition(Math.round(newX), Math.round(newY));
    }
  }

  private handleDisplayRemoved(display: Electron.Display): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;

    const [x, y] = win.getPosition();
    const current = screen.getDisplayNearestPoint({ x, y });
    if (!current || current.id === display.id) {
      const primary = screen.getPrimaryDisplay();
      win.setPosition(
        Math.round(primary.workArea.x + EDGE_OFFSET),
        Math.round(primary.workArea.y + EDGE_OFFSET),
      );
    } else {
      this.ensureVisible();
    }
  }

  private registerWindowEvents(): void {
    const win = this.window;
    if (!win) return;
    win.on("move", () => this.emit("move"));
    win.on("resize", () => this.emit("resize"));
    win.on("closed", () => {
      this.emit("close");
      this.unregisterScreenEvents();
    });
    win.webContents.on("render-process-gone", () => this.emit("crashed"));
    win.webContents.on("unresponsive", () => this.emit("crashed"));
  }

  private registerScreenEvents(): void {
    screen.on("display-removed", this.onDisplayRemoved);
    screen.on("display-metrics-changed", this.onDisplayMetricsChanged);
  }

  private unregisterScreenEvents(): void {
    screen.off("display-removed", this.onDisplayRemoved);
    screen.off("display-metrics-changed", this.onDisplayMetricsChanged);
  }

  private emit(event: PetWindowEvent, ...args: unknown[]): void {
    const set = this.listeners.get(event);
    if (!set) return;
    for (const fn of set) {
      try {
        fn(...args);
      } catch (err) {
        console.error("[DesktopPetWindowAdapter] 事件监听器异常", err);
      }
    }
  }
}
