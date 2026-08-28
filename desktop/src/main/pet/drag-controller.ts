import { BrowserWindow, screen } from "electron";
import type { ClickThroughController } from "./click-through-controller";
import type { DesktopPetWindowAdapter } from "./window-adapter";
import type { PetDragIpcPayload } from "../../shared/animation-ipc";

export type DragEvent = "drag-start" | "drag-move" | "drag-end";

export interface DragState {
  isDragging: boolean;
  startX: number;
  startY: number;
  currentX: number;
  currentY: number;
  startScreenId: string;
  currentScreenId: string;
}

const PERSIST_THROTTLE_MS = 500;

export class DragController {
  private readonly adapter: DesktopPetWindowAdapter;
  private readonly clickThrough: ClickThroughController;
  private readonly onEvent: (event: DragEvent, state: DragState) => void;
  private win: BrowserWindow | null = null;
  private dragging = false;
  private state: DragState = {
    isDragging: false,
    startX: 0,
    startY: 0,
    currentX: 0,
    currentY: 0,
    startScreenId: "",
    currentScreenId: "",
  };
  private lastPersistAt = 0;
  private grabOffsetX = 0;
  private grabOffsetY = 0;

  constructor(
    adapter: DesktopPetWindowAdapter,
    clickThrough: ClickThroughController,
    onEvent: (event: DragEvent, state: DragState) => void,
  ) {
    this.adapter = adapter;
    this.clickThrough = clickThrough;
    this.onEvent = onEvent;
  }

  attach(win: BrowserWindow): void {
    this.detach();
    this.win = win;
  }

  detach(): void {
    this.dragging = false;
    this.win = null;
    this.lastPersistAt = 0;
    this.grabOffsetX = 0;
    this.grabOffsetY = 0;
    this.resetState();
  }

  isDragging(): boolean {
    return this.dragging;
  }

  getState(): DragState {
    return { ...this.state };
  }

  handleDragStart(payload: PetDragIpcPayload): void {
    if (this.dragging) return;
    const win = this.win;
    if (!win || win.isDestroyed()) return;

    const { screenX, screenY } = payload;
    const screenId = this.resolveScreenId(screenX, screenY);
    const [windowX, windowY] = win.getPosition();
    this.grabOffsetX = screenX - windowX;
    this.grabOffsetY = screenY - windowY;

    this.dragging = true;
    this.state = {
      isDragging: true,
      startX: screenX,
      startY: screenY,
      currentX: screenX,
      currentY: screenY,
      startScreenId: screenId,
      currentScreenId: screenId,
    };

    this.clickThrough.setDragging(true);
    this.onEvent("drag-start", this.getState());
  }

  handleDragMove(payload: PetDragIpcPayload): void {
    if (!this.dragging) return;
    const win = this.win;
    if (!win || win.isDestroyed()) return;

    const { screenX, screenY } = payload;
    const display = screen.getDisplayNearestPoint({ x: screenX, y: screenY });
    if (!display) return;

    const screenId = String(display.id);
    if (screenId !== this.state.currentScreenId) {
      this.adapter.getDpiScale();
    }

    const targetX = screenX - this.grabOffsetX;
    const targetY = screenY - this.grabOffsetY;
    const relX = targetX - display.bounds.x;
    const relY = targetY - display.bounds.y;
    void this.adapter.setPosition(relX, relY, screenId);

    this.state.currentX = screenX;
    this.state.currentY = screenY;
    this.state.currentScreenId = screenId;

    this.onEvent("drag-move", this.getState());
    this.maybePersist();
  }

  handleDragEnd(payload: PetDragIpcPayload): void {
    if (!this.dragging) return;
    const win = this.win;
    if (!win || win.isDestroyed()) return;

    const { screenX, screenY } = payload;
    const display = screen.getDisplayNearestPoint({ x: screenX, y: screenY });
    const screenId = display
      ? String(display.id)
      : this.state.currentScreenId;

    const targetX = screenX - this.grabOffsetX;
    const targetY = screenY - this.grabOffsetY;
    if (display) {
      const relX = targetX - display.bounds.x;
      const relY = targetY - display.bounds.y;
      void this.adapter.setPosition(relX, relY, screenId);
    } else {
      void this.adapter.setPosition(targetX, targetY);
    }

    this.state.currentX = screenX;
    this.state.currentY = screenY;
    this.state.currentScreenId = screenId;

    const snapshot = this.adapter.snapshotRuntimePosition();

    this.dragging = false;
    this.state.isDragging = false;
    this.grabOffsetX = 0;
    this.grabOffsetY = 0;
    this.clickThrough.setDragging(false);
    this.onEvent("drag-end", this.getState());
    void snapshot;
  }

  handleDragCancel(payload: PetDragIpcPayload): void {
    if (!this.dragging) return;
    this.dragging = false;
    this.state.isDragging = false;
    this.grabOffsetX = 0;
    this.grabOffsetY = 0;
    this.clickThrough.setDragging(false);
    this.onEvent("drag-end", this.getState());
    void payload;
  }

  private resolveScreenId(x: number, y: number): string {
    const display = screen.getDisplayNearestPoint({ x, y });
    return display
      ? String(display.id)
      : String(screen.getPrimaryDisplay().id);
  }

  private maybePersist(): void {
    const now = Date.now();
    if (now - this.lastPersistAt < PERSIST_THROTTLE_MS) return;
    this.lastPersistAt = now;
    this.adapter.snapshotRuntimePosition();
  }

  private resetState(): void {
    this.state = {
      isDragging: false,
      startX: 0,
      startY: 0,
      currentX: 0,
      currentY: 0,
      startScreenId: "",
      currentScreenId: "",
    };
  }
}
