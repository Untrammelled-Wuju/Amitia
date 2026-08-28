import { BrowserWindow, screen } from "electron";
import { ClickHitTester } from "./click-hit-test";
import type { DesktopPetWindowAdapter } from "./window-adapter";
import type { ClickThroughMode } from "./types";

const POLL_INTERVAL_MS = 16;

export class ClickThroughController {
  private readonly tester: ClickHitTester;
  private readonly adapter: DesktopPetWindowAdapter;
  private win: BrowserWindow | null = null;
  private isDragging = false;
  private mode: ClickThroughMode = "none";
  private hitMaskThreshold = 0;
  private isIgnoring = false;
  private pollTimer: ReturnType<typeof setInterval> | null = null;
  private lastProcessTime = 0;
  private lastContentSize: { width: number; height: number } | null = null;
  private readonly boundTick = (): void => {
    this.processMousePosition();
  };

  constructor(adapter: DesktopPetWindowAdapter, alphaThreshold?: number) {
    this.adapter = adapter;
    this.tester = new ClickHitTester(alphaThreshold);
  }

  attach(win: BrowserWindow): void {
    this.detach();
    this.win = win;
    this.tester.clear();
    this.isIgnoring = false;
    this.isDragging = false;
    this.lastProcessTime = 0;
    this.lastContentSize = null;
    win.once("closed", () => {
      this.detach();
    });
    this.startPolling();
  }

  detach(): void {
    this.stopPolling();
    const win = this.win;
    if (win && !win.isDestroyed()) {
      try {
        win.setIgnoreMouseEvents(false);
      } catch {
        void 0;
      }
    }
    this.win = null;
    this.tester.clear();
    this.isDragging = false;
    this.isIgnoring = false;
    this.lastProcessTime = 0;
    this.lastContentSize = null;
  }

  updateFrame(width: number, height: number, alphaData: Uint8Array): void {
    this.tester.setFrame(width, height, alphaData);
  }

  updateHitMask(
    width: number,
    height: number,
    data: Uint8Array,
    threshold: number,
  ): void {
    this.tester.setFrame(width, height, data, threshold);
    this.hitMaskThreshold = threshold;
  }

  setMode(mode: ClickThroughMode): void {
    this.mode = mode;
    this.lastProcessTime = 0;
    if (mode === "none") {
      this.setIgnoreState(false);
      return;
    }
    if (mode === "full") {
      this.setIgnoreState(true);
      return;
    }
    this.processMousePosition();
  }

  setDragging(dragging: boolean): void {
    this.isDragging = dragging;
    if (dragging) {
      this.setIgnoreState(false);
      return;
    }
    this.lastProcessTime = 0;
    this.processMousePosition();
  }

  private startPolling(): void {
    if (this.pollTimer) return;
    this.pollTimer = setInterval(this.boundTick, POLL_INTERVAL_MS);
  }

  private stopPolling(): void {
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  private processMousePosition(): void {
    const win = this.win;
    if (!win || win.isDestroyed()) return;
    if (this.isDragging) return;
    if (this.mode === "none") return;
    if (this.mode === "full") {
      this.setIgnoreState(true);
      return;
    }

    let contentWidth = 0;
    let contentHeight = 0;
    try {
      const [cw, ch] = win.getContentSize();
      contentWidth = cw;
      contentHeight = ch;
    } catch {
      void 0;
    }
    const sizeChanged =
      this.lastContentSize === null ||
      this.lastContentSize.width !== contentWidth ||
      this.lastContentSize.height !== contentHeight;
    if (sizeChanged) {
      this.lastContentSize = { width: contentWidth, height: contentHeight };
      this.lastProcessTime = 0;
    }

    const now = Date.now();
    if (now - this.lastProcessTime < POLL_INTERVAL_MS) return;
    this.lastProcessTime = now;

    if (contentWidth <= 0 || contentHeight <= 0) return;

    const cursor = screen.getCursorScreenPoint();
    const [winX, winY] = win.getPosition();
    const localX = cursor.x - winX;
    const localY = cursor.y - winY;
    if (
      localX < 0 ||
      localY < 0 ||
      localX > contentWidth ||
      localY > contentHeight
    ) {
      this.setIgnoreState(true);
      return;
    }
    const x01 = localX / contentWidth;
    const y01 = localY / contentHeight;

    let hit: boolean;
    if (this.mode === "boundingBox") {
      const box = this.tester.getNormalizedBoundingBox();
      if (box.maxX < box.minX || box.maxY < box.minY) {
        hit = false;
      } else {
        hit =
          x01 >= box.minX &&
          x01 <= box.maxX &&
          y01 >= box.minY &&
          y01 <= box.maxY;
      }
    } else {
      if (!this.tester.hasFrame()) {
        const box = this.tester.getNormalizedBoundingBox();
        if (box.maxX < box.minX || box.maxY < box.minY) {
          hit = false;
        } else {
          hit =
            x01 >= box.minX &&
            x01 <= box.maxX &&
            y01 >= box.minY &&
            y01 <= box.maxY;
        }
      } else {
        hit = this.tester.isHitNormalized(x01, y01);
      }
    }
    this.setIgnoreState(!hit);
  }

  private setIgnoreState(ignore: boolean): void {
    const win = this.win;
    if (!win || win.isDestroyed()) return;
    const effective = this.isDragging ? false : ignore;
    if (this.isIgnoring === effective) return;
    this.isIgnoring = effective;
    if (effective) {
      win.setIgnoreMouseEvents(true, { forward: true });
    } else {
      win.setIgnoreMouseEvents(false);
    }
  }
}
