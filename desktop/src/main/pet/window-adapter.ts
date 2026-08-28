import { BrowserWindow, screen } from "electron";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { isDevMode } from "../path-manager";
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

const DEV_SERVER_URL =
  process.env.VITE_DEV_SERVER_URL ||
  process.env.AMITIA_DESKTOP_DEV_SERVER_URL ||
  "";

const VISIBLE_MARGIN = 20;
const EDGE_OFFSET = 40;

const LOAD_TIMEOUT_MS = 15000;

export class DesktopPetWindowLoadError extends Error {
  readonly reason: string;
  constructor(reason: string, detail?: string) {
    super(`DesktopPetWindowLoadError: ${reason}${detail ? ` (${detail})` : ""}`);
    this.name = "DesktopPetWindowLoadError";
    this.reason = reason;
  }
}

function clampScale(scale: number): number {
  if (!Number.isFinite(scale)) return PET_WINDOW_SCALE_DEFAULT;
  return Math.min(PET_WINDOW_SCALE_MAX, Math.max(PET_WINDOW_SCALE_MIN, scale));
}

function isClickThroughMode(value: unknown): value is ClickThroughMode {
  return value === "alpha" || value === "boundingBox" || value === "full" || value === "none";
}

export class DesktopPetWindowAdapter {
  private window: BrowserWindow | null = null;

  private readonly listeners: Map<PetWindowEvent, Set<PetWindowEventListener>> =
    new Map();

  private options: DesktopPetWindowOptions;

  private intentionalClose = false;

  private loadTimeout: ReturnType<typeof setTimeout> | null = null;
  private loadSettled = false;

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

  createWindow(): BrowserWindow {
    if (this.window && !this.window.isDestroyed()) {
      return this.window;
    }

    const { width, height } = this.calculateSize();
    const initial = this.resolveInitialPosition();
    const preloadPath = join(currentDir, "../preload/animation-preload.cjs");

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
      focusable: false,
      webPreferences: {
        preload: preloadPath,
        sandbox: true,
        nodeIntegration: false,
        contextIsolation: true,
        webSecurity: true,
        allowRunningInsecureContent: false,
        backgroundThrottling: false,
      },
    };

    if (process.platform === "darwin") {
      constructorOptions.type = "panel";
    }

    this.intentionalClose = false;
    this.window = new BrowserWindow(constructorOptions);

    this.configureForPlatform();
    this.applySoundEnabled(this.options.soundEnabled ?? false);
    this.applyClickThroughMode(this.options.clickThroughMode ?? "none");
    this.registerWindowEvents();
    this.registerScreenEvents();

    return this.window;
  }

  async loadRenderer(): Promise<void> {
    const win = this.window;
    if (!win || win.isDestroyed()) {
      throw new DesktopPetWindowLoadError("window_not_available");
    }

    this.loadSettled = false;

    return new Promise<void>((resolve, reject) => {
      const onRenderProcessGone = (
        _event: Electron.Event,
        details: Electron.RenderProcessGoneDetails,
      ): void => {
        settle(
          new DesktopPetWindowLoadError(
            "render-process-gone",
            `reason=${details?.reason ?? "unknown"}`,
          ),
        );
      };
      const onUnresponsive = (): void => {
        settle(new DesktopPetWindowLoadError("unresponsive"));
      };

      const cleanup = (): void => {
        if (this.loadTimeout) {
          clearTimeout(this.loadTimeout);
          this.loadTimeout = null;
        }
        // Remove only the temporary startup listeners installed by this load.
        // Long-lived crash listeners from registerWindowEvents() must survive
        // navigation so renderer recovery keeps working after initial startup.
        win.webContents.off("render-process-gone", onRenderProcessGone);
        win.webContents.off("unresponsive", onUnresponsive);
      };

      const settle = (error: Error | null): void => {
        if (this.loadSettled) return;
        this.loadSettled = true;
        cleanup();
        if (error) {
          reject(error);
        } else {
          resolve();
        }
      };

      win.webContents.once("render-process-gone", onRenderProcessGone);
      win.webContents.once("unresponsive", onUnresponsive);

      this.loadTimeout = setTimeout(() => {
        settle(new DesktopPetWindowLoadError("load_timeout", `${LOAD_TIMEOUT_MS}ms`));
      }, LOAD_TIMEOUT_MS);

      const loadPackagedRenderer = async (): Promise<void> => {
        const petHtmlPath = join(currentDir, "../renderer/pet.html");
        await win.loadFile(petHtmlPath);
      };

      const doLoad = async (): Promise<void> => {
        if (isDevMode() && DEV_SERVER_URL) {
          const petDevUrl = `${DEV_SERVER_URL.replace(/\/$/, "")}/pet.html`;
          try {
            await win.loadURL(petDevUrl);
            return;
          } catch {
            // Development server may be unavailable during Electron startup.
            // Fall back to the packaged renderer; do not let the failed dev
            // navigation consume the whole startup attempt.
            await loadPackagedRenderer();
            return;
          }
        }
        await loadPackagedRenderer();
      };

      void doLoad().then(
        () => settle(null),
        (err: unknown) =>
          settle(
            new DesktopPetWindowLoadError(
              "load_failed",
              err instanceof Error ? err.message : String(err),
            ),
          ),
      );
    });
  }

  showWhenRuntimeReady(): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    if (this.loadTimeout) {
      clearTimeout(this.loadTimeout);
      this.loadTimeout = null;
    }
    win.show();
  }

  async create(): Promise<BrowserWindow> {
    const win = this.createWindow();
    await this.loadRenderer();
    return win;
  }

  async destroy(): Promise<void> {
    this.unregisterScreenEvents();
    if (this.loadTimeout) {
      clearTimeout(this.loadTimeout);
      this.loadTimeout = null;
    }
    this.loadSettled = false;
    const win = this.window;
    this.window = null;
    if (win && !win.isDestroyed()) {
      this.intentionalClose = true;
      win.destroy();
    }
    this.listeners.clear();
  }

  setIntentionalClose(value: boolean): void {
    this.intentionalClose = value;
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

  async setSoundEnabled(enabled: boolean): Promise<void> {
    this.options.soundEnabled = enabled;
    this.applySoundEnabled(enabled);
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
    // Do not call app.dock.hide() here. That API changes the Dock visibility
    // for the entire Electron application, not just the pet panel, and would
    // hide the main app's Dock entry while the pet is enabled.
  }

  private configureForLinux(): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    win.setSkipTaskbar(true);
    if (this.options.alwaysOnTop) {
      win.setAlwaysOnTop(true);
    }
  }

  private applySoundEnabled(enabled: boolean): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    win.webContents.setAudioMuted(!enabled);
  }

  private applyClickThroughMode(mode: ClickThroughMode): void {
    const win = this.window;
    if (!win || win.isDestroyed()) return;
    switch (mode) {
      case "none":
        win.setIgnoreMouseEvents(false);
        break;
      case "full":
        win.setIgnoreMouseEvents(true, { forward: true });
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
      if (!this.intentionalClose) {
        this.emit("close");
      }
      this.unregisterScreenEvents();
    });
    win.webContents.on("render-process-gone", () => this.emit("crashed"));
    win.webContents.on("unresponsive", () => this.emit("crashed"));
    // The pet renderer is a closed local surface. It never needs renderer-
    // initiated top-level navigation or popup windows; deny both so a compromised
    // asset cannot turn this privileged Electron surface into a general browser.
    win.webContents.on("will-navigate", (event) => event.preventDefault());
    win.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
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
