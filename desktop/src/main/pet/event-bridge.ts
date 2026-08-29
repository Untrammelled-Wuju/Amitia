import {
  ActionPriorities,
  EventSources,
  DesktopPetActionScheduler,
  type DesktopPetActionRequest,
} from "./action-scheduler";
import type { DragController, DragEvent, DragState } from "./drag-controller";

const DEFAULT_HOVER_COOLDOWN_MS = 3000;
const DEFAULT_CLICK_COOLDOWN_MS = 500;

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
  private lastHoverAt: number;
  private lastClickAt: number;
  private pendingClickX: number;
  private pendingClickY: number;

  constructor(
    scheduler: DesktopPetActionScheduler,
    dragController: DragController,
  ) {
    this.scheduler = scheduler;
    this.dragController = dragController;
    this.hoverCooldownMs = DEFAULT_HOVER_COOLDOWN_MS;
    this.clickCooldownMs = DEFAULT_CLICK_COOLDOWN_MS;
    this.lastHoverAt = 0;
    this.lastClickAt = 0;
    this.pendingClickX = 0;
    this.pendingClickY = 0;
  }

  handleClick(x: number, y: number): void {
    this.pendingClickX = x;
    this.pendingClickY = y;
    this.processSingleClick();
  }

  handleDoubleClick(x: number, y: number): void {
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
    } else if (event === "drag-cancel") {
      // Cancellation is not a successful drop and must not schedule dropped.
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

  dispose(): void {
    // Renderer is the sole click/double-click arbiter; Main owns no timers.
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
