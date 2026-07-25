import { BrowserWindow, ipcMain, screen } from "electron";
import type { ClickThroughController } from "./click-through-controller";
import type { DesktopPetWindowAdapter } from "./window-adapter";
import type { PetRuntimePositionRecord } from "./types";

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

export interface DragPayload {
  x: number;
  y: number;
  screenId?: string;
}

export type DragRendererMessage =
  | { type: "state"; payload: DragState }
  | { type: "persist"; payload: PetRuntimePositionRecord };

const DRAG_START_CHANNEL = "pet:drag-start";
const DRAG_MOVE_CHANNEL = "pet:drag-move";
const DRAG_END_CHANNEL = "pet:drag-end";
const DRAG_STATE_CHANNEL = "pet:drag-state";

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
  private closedListener: (() => void) | null = null;

  private readonly boundDragStart = (
    _event: Electron.IpcMainEvent,
    payload: DragPayload,
  ): void => {
    void this.handleDragStart(payload);
  };

  private readonly boundDragMove = (
    _event: Electron.IpcMainEvent,
    payload: DragPayload,
  ): void => {
    void this.handleDragMove(payload);
  };

  private readonly boundDragEnd = (
    _event: Electron.IpcMainEvent,
    payload: DragPayload,
  ): void => {
    void this.handleDragEnd(payload);
  };

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
    ipcMain.on(DRAG_START_CHANNEL, this.boundDragStart);
    ipcMain.on(DRAG_MOVE_CHANNEL, this.boundDragMove);
    ipcMain.on(DRAG_END_CHANNEL, this.boundDragEnd);
    this.closedListener = (): void => {
      this.detach();
    };
    win.once("closed", this.closedListener);
  }

  detach(): void {
    ipcMain.removeListener(DRAG_START_CHANNEL, this.boundDragStart);
    ipcMain.removeListener(DRAG_MOVE_CHANNEL, this.boundDragMove);
    ipcMain.removeListener(DRAG_END_CHANNEL, this.boundDragEnd);
    const win = this.win;
    if (win && !win.isDestroyed() && this.closedListener) {
      win.removeListener("closed", this.closedListener);
    }
    this.closedListener = null;
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

  private async handleDragStart(payload: DragPayload): Promise<void> {
    if (this.dragging) return;
    const win = this.win;
    if (!win || win.isDestroyed()) return;

    const { x, y } = payload;
    const screenId = this.resolveScreenId(payload.screenId, x, y);

    this.dragging = true;
    this.state = {
      isDragging: true,
      startX: x,
      startY: y,
      currentX: x,
      currentY: y,
      startScreenId: screenId,
      currentScreenId: screenId,
    };

    this.clickThrough.setDragging(true);
    this.sendStateMessage({ type: "state", payload: this.getState() });
    this.onEvent("drag-start", this.getState());
  }

  private async handleDragMove(payload: DragPayload): Promise<void> {
    if (!this.dragging) return;
    const win = this.win;
    if (!win || win.isDestroyed()) return;

    const { x, y } = payload;
    const display = screen.getDisplayNearestPoint({ x, y });
    if (!display) return;

    const screenId = String(display.id);
    if (screenId !== this.state.currentScreenId) {
      this.adapter.getDpiScale();
    }

    const relX = x - display.bounds.x;
    const relY = y - display.bounds.y;
    await this.adapter.setPosition(relX, relY, screenId);

    this.state.currentX = x;
    this.state.currentY = y;
    this.state.currentScreenId = screenId;

    this.sendStateMessage({ type: "state", payload: this.getState() });
    this.onEvent("drag-move", this.getState());
    this.maybePersist();
  }

  private async handleDragEnd(payload: DragPayload): Promise<void> {
    if (!this.dragging) return;
    const win = this.win;
    if (!win || win.isDestroyed()) return;

    const { x, y } = payload;
    const display = screen.getDisplayNearestPoint({ x, y });
    const screenId = display
      ? String(display.id)
      : this.state.currentScreenId;

    if (display) {
      const relX = x - display.bounds.x;
      const relY = y - display.bounds.y;
      await this.adapter.setPosition(relX, relY, screenId);
    } else {
      await this.adapter.setPosition(x, y);
    }

    this.state.currentX = x;
    this.state.currentY = y;
    this.state.currentScreenId = screenId;

    const snapshot = this.adapter.snapshotRuntimePosition();
    this.sendStateMessage({ type: "persist", payload: snapshot });

    this.dragging = false;
    this.state.isDragging = false;
    this.clickThrough.setDragging(false);
    this.sendStateMessage({ type: "state", payload: this.getState() });
    this.onEvent("drag-end", this.getState());
  }

  private resolveScreenId(
    screenId: string | undefined,
    x: number,
    y: number,
  ): string {
    if (screenId) return screenId;
    const display = screen.getDisplayNearestPoint({ x, y });
    return display
      ? String(display.id)
      : String(screen.getPrimaryDisplay().id);
  }

  private maybePersist(): void {
    const now = Date.now();
    if (now - this.lastPersistAt < PERSIST_THROTTLE_MS) return;
    this.lastPersistAt = now;
    const snapshot = this.adapter.snapshotRuntimePosition();
    this.sendStateMessage({ type: "persist", payload: snapshot });
  }

  private sendStateMessage(message: DragRendererMessage): void {
    const win = this.win;
    if (!win || win.isDestroyed()) return;
    win.webContents.send(DRAG_STATE_CHANNEL, message);
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
