import { BrowserWindow, ipcMain, type IpcMainEvent } from "electron";
import {
  ActionPriorities,
  EventSources,
  DesktopPetActionScheduler,
  type DesktopPetActionRequest,
} from "./action-scheduler";
import type { DragController, DragEvent, DragState } from "./drag-controller";

const PET_CLICK_CHANNEL = "pet:click";
const PET_DOUBLE_CLICK_CHANNEL = "pet:double-click";
const PET_HOVER_CHANNEL = "pet:hover";

const DEFAULT_HOVER_COOLDOWN_MS = 3000;
const DEFAULT_CLICK_COOLDOWN_MS = 500;
const DEFAULT_DOUBLE_CLICK_THRESHOLD_MS = 300;

const CLICKED_ACTION_KEY = "clicked";
const DOUBLE_CLICKED_ACTION_KEY = "double_clicked";
const HOVERED_ACTION_KEY = "hovered";
const DRAGGED_ACTION_KEY = "dragged";
const DROPPED_ACTION_KEY = "dropped";

function nowTimestamp(): number {
  if (
    typeof performance !== "undefined" &&
    typeof performance.now === "function"
  ) {
    return performance.now();
  }
  return Date.now();
}

export interface PetPointerPayload {
  x: number;
  y: number;
}

export class DesktopPetEventBridge {
  private scheduler: DesktopPetActionScheduler;
  private dragController: DragController;
  private hoverCooldownMs: number;
  private clickCooldownMs: number;
  private doubleClickThresholdMs: number;
  private lastHoverAt: number;
  private lastClickAt: number;
  private pendingClickTimer: ReturnType<typeof setTimeout> | null;
  private pendingClickX: number;
  private pendingClickY: number;
  private win: BrowserWindow | null = null;
  private closedListener: (() => void) | null = null;

  private readonly boundClick = (
    _event: IpcMainEvent,
    payload: PetPointerPayload,
  ): void => {
    if (!payload) return;
    this.handleClick(payload.x, payload.y);
  };

  private readonly boundDoubleClick = (
    _event: IpcMainEvent,
    payload: PetPointerPayload,
  ): void => {
    if (!payload) return;
    this.handleDoubleClick(payload.x, payload.y);
  };

  private readonly boundHover = (
    _event: IpcMainEvent,
    payload: PetPointerPayload,
  ): void => {
    if (!payload) return;
    this.handleHover(payload.x, payload.y);
  };

  constructor(
    scheduler: DesktopPetActionScheduler,
    dragController: DragController,
  ) {
    this.scheduler = scheduler;
    this.dragController = dragController;
    this.hoverCooldownMs = DEFAULT_HOVER_COOLDOWN_MS;
    this.clickCooldownMs = DEFAULT_CLICK_COOLDOWN_MS;
    this.doubleClickThresholdMs = DEFAULT_DOUBLE_CLICK_THRESHOLD_MS;
    this.lastHoverAt = 0;
    this.lastClickAt = 0;
    this.pendingClickTimer = null;
    this.pendingClickX = 0;
    this.pendingClickY = 0;
  }

  attach(win: BrowserWindow): void {
    this.detach();
    this.win = win;
    ipcMain.on(PET_CLICK_CHANNEL, this.boundClick);
    ipcMain.on(PET_DOUBLE_CLICK_CHANNEL, this.boundDoubleClick);
    ipcMain.on(PET_HOVER_CHANNEL, this.boundHover);
    this.closedListener = (): void => {
      this.detach();
    };
    win.once("closed", this.closedListener);
  }

  detach(): void {
    ipcMain.removeListener(PET_CLICK_CHANNEL, this.boundClick);
    ipcMain.removeListener(PET_DOUBLE_CLICK_CHANNEL, this.boundDoubleClick);
    ipcMain.removeListener(PET_HOVER_CHANNEL, this.boundHover);
    if (this.pendingClickTimer !== null) {
      clearTimeout(this.pendingClickTimer);
      this.pendingClickTimer = null;
    }
    const win = this.win;
    if (win && !win.isDestroyed() && this.closedListener) {
      win.removeListener("closed", this.closedListener);
    }
    this.closedListener = null;
    this.win = null;
  }

  handleClick(x: number, y: number): void {
    if (this.pendingClickTimer !== null) {
      clearTimeout(this.pendingClickTimer);
      this.pendingClickTimer = null;
      this.handleDoubleClick(x, y);
      return;
    }
    this.pendingClickX = x;
    this.pendingClickY = y;
    this.pendingClickTimer = setTimeout(() => {
      this.pendingClickTimer = null;
      this.processSingleClick();
    }, this.doubleClickThresholdMs);
  }

  handleDoubleClick(x: number, y: number): void {
    if (this.pendingClickTimer !== null) {
      clearTimeout(this.pendingClickTimer);
      this.pendingClickTimer = null;
    }
    const now = nowTimestamp();
    this.lastClickAt = now;
    const request: DesktopPetActionRequest = {
      actionKey: DOUBLE_CLICKED_ACTION_KEY,
      source: EventSources.USER_DOUBLE_CLICK,
      priority: ActionPriorities.CLICK,
      interrupt: true,
      metadata: { x: String(x), y: String(y) },
    };
    this.scheduler.submit(request);
  }

  handleHover(x: number, y: number): void {
    const now = nowTimestamp();
    if (now - this.lastHoverAt < this.hoverCooldownMs) {
      return;
    }
    this.lastHoverAt = now;
    const request: DesktopPetActionRequest = {
      actionKey: HOVERED_ACTION_KEY,
      source: EventSources.USER_HOVER,
      priority: ActionPriorities.EMOTION,
      interrupt: false,
      metadata: { x: String(x), y: String(y) },
    };
    this.scheduler.submit(request);
  }

  handleDragStart(): void {
    if (this.pendingClickTimer !== null) {
      clearTimeout(this.pendingClickTimer);
      this.pendingClickTimer = null;
    }
    this.scheduler.forceInterrupt("user_drag");
    const request: DesktopPetActionRequest = {
      actionKey: DRAGGED_ACTION_KEY,
      source: EventSources.USER_DRAG,
      priority: ActionPriorities.DRAG,
      interrupt: true,
    };
    this.scheduler.submit(request);
  }

  handleDragEnd(): void {
    const request: DesktopPetActionRequest = {
      actionKey: DROPPED_ACTION_KEY,
      source: EventSources.USER_DRAG,
      priority: ActionPriorities.DRAG,
      interrupt: true,
    };
    this.scheduler.submit(request);
  }

  onDragEvent(event: DragEvent, _state: DragState): void {
    if (event === "drag-start") {
      this.handleDragStart();
    } else if (event === "drag-end") {
      this.handleDragEnd();
    }
  }

  setHoverCooldownMs(ms: number): void {
    if (Number.isFinite(ms) && ms >= 0) {
      this.hoverCooldownMs = ms;
    }
  }

  setClickCooldownMs(ms: number): void {
    if (Number.isFinite(ms) && ms >= 0) {
      this.clickCooldownMs = ms;
    }
  }

  setDoubleClickThresholdMs(ms: number): void {
    if (Number.isFinite(ms) && ms >= 0) {
      this.doubleClickThresholdMs = ms;
    }
  }

  dispose(): void {
    this.detach();
  }

  private processSingleClick(): void {
    const now = nowTimestamp();
    if (now - this.lastClickAt < this.clickCooldownMs) {
      return;
    }
    this.lastClickAt = now;
    const request: DesktopPetActionRequest = {
      actionKey: CLICKED_ACTION_KEY,
      source: EventSources.USER_CLICK,
      priority: ActionPriorities.CLICK,
      interrupt: true,
      metadata: {
        x: String(this.pendingClickX),
        y: String(this.pendingClickY),
      },
    };
    this.scheduler.submit(request);
  }
}
