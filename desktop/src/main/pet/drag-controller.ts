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

    const relX = screenX - display.bounds.x;
    const relY = screenY - display.bounds.y;
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

    if (display) {
      const relX = screenX - display.bounds.x;
      const relY = screenY - display.bounds.y;
      void this.adapter.setPosition(relX, relY, screenId);
    } else {
      void this.adapter.setPosition(screenX, screenY);
    }

    this.state.currentX = screenX;
    this.state.currentY = screenY;
    this.state.currentScreenId = screenId;

    const snapshot = this.adapter.snapshotRuntimePosition();

    this.dragging = false;
    this.state.isDragging = false;
    this.clickThrough.setDragging(false);
    this.onEvent("drag-end", this.getState());
    void snapshot;
  }

  handleDragCancel(payload: PetDragIpcPayload): void {
    if (!this.dragging) return;
    this.dragging = false;
    this.state.isDragging = false;
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
