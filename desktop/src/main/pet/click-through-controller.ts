import { BrowserWindow, screen } from "electron";
import { ClickHitTester } from "./click-hit-test";
import type { DesktopPetWindowAdapter } from "./window-adapter";

const POLL_INTERVAL_MS = 16;

export class ClickThroughController {
  private readonly tester: ClickHitTester;
  private readonly adapter: DesktopPetWindowAdapter;
  private win: BrowserWindow | null = null;
  private isDragging = false;
  private mode: "none" | "alpha" | "boundingBox" = "none";
  private hitMaskThreshold = 0;
  private isIgnoring = false;
  private pollTimer: ReturnType<typeof setInterval> | null = null;
  private lastProcessTime = 0;
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
    this.tester.setFrame(width, height, data);
    this.hitMaskThreshold = threshold;
  }

  setMode(mode: "none" | "alpha" | "boundingBox"): void {
    this.mode = mode;
    this.lastProcessTime = 0;
    if (mode === "none") {
      this.setIgnoreState(false);
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
    if (this.mode === "alpha" && !this.tester.hasFrame()) return;

    const now = Date.now();
    if (now - this.lastProcessTime < POLL_INTERVAL_MS) return;
    this.lastProcessTime = now;

    const cursor = screen.getCursorScreenPoint();
    const [winX, winY] = win.getPosition();
    const localX = cursor.x - winX;
    const localY = cursor.y - winY;

    let hit: boolean;
    if (this.mode === "boundingBox") {
      const box = this.tester.getBoundingBox();
      hit =
        localX >= box.minX &&
        localX <= box.maxX &&
        localY >= box.minY &&
        localY <= box.maxY;
    } else {
      hit = this.tester.isHit(localX, localY);
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
