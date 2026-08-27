import { screen } from "electron";
import {
  ActionPriorities,
  EventSources,
  type DesktopPetActionScheduler,
} from "./action-scheduler";
import type { DesktopPetWindowAdapter } from "./window-adapter";

const WORLD_TICK_MS = 1000;
const MOVE_TICK_MS = 40;
const MIN_WANDER_DELAY_MS = 18000;
const MAX_WANDER_DELAY_MS = 48000;
const MIN_WANDER_DISTANCE_PX = 60;
const MAX_WANDER_DISTANCE_PX = 220;
const WALK_SPEED_PX_PER_TICK = 4;
const GRAVITY_PX_PER_TICK = 3.2;
const MAX_FALL_SPEED_PX_PER_TICK = 28;
const EDGE_PADDING_PX = 4;

function randomBetween(min: number, max: number): number {
  return min + Math.random() * Math.max(0, max - min);
}

/**
 * DesktopPetWorldController gives the pet a small amount of physical presence
 * in the desktop world.  It intentionally owns only window motion; semantic
 * action arbitration stays in DesktopPetActionScheduler.
 */
export class DesktopPetWorldController {
  private readonly scheduler: DesktopPetActionScheduler;
  private readonly windowAdapter: DesktopPetWindowAdapter;
  private worldTimer: ReturnType<typeof setInterval> | null = null;
  private movementTimer: ReturnType<typeof setInterval> | null = null;
  private nextWanderAt = 0;
  private dragging = false;
  private falling = false;

  constructor(
    scheduler: DesktopPetActionScheduler,
    windowAdapter: DesktopPetWindowAdapter,
  ) {
    this.scheduler = scheduler;
    this.windowAdapter = windowAdapter;
  }

  start(): void {
    if (this.worldTimer) return;
    this.scheduleNextWander();
    this.worldTimer = setInterval(() => this.tick(), WORLD_TICK_MS);
    this.worldTimer.unref?.();
  }

  stop(): void {
    if (this.worldTimer) {
      clearInterval(this.worldTimer);
      this.worldTimer = null;
    }
    this.stopMovement();
    this.dragging = false;
    this.falling = false;
  }

  dispose(): void {
    this.stop();
  }

  setDragging(dragging: boolean): void {
    this.dragging = dragging;
    if (dragging) {
      this.stopMovement();
      this.falling = false;
    }
  }

  onDrop(): void {
    this.dragging = false;
    this.startFallIfNeeded();
    this.scheduleNextWander();
  }

  private tick(): void {
    if (this.dragging || this.falling || Date.now() < this.nextWanderAt) {
      return;
    }

    const win = this.windowAdapter.getNativeWindow();
    if (!win || win.isDestroyed() || !win.isVisible()) {
      this.scheduleNextWander();
      return;
    }

    const current = this.scheduler.getCurrent();
    if (current && current.priority > ActionPriorities.RANDOM_IDLE) {
      return;
    }

    this.startWander();
    this.scheduleNextWander();
  }

  private startWander(): void {
    if (this.movementTimer || this.dragging || this.falling) return;
    const win = this.windowAdapter.getNativeWindow();
    if (!win || win.isDestroyed()) return;

    const [x, y] = win.getPosition();
    const [width] = win.getSize();
    const display = screen.getDisplayNearestPoint({
      x: x + Math.floor(width / 2),
      y,
    });
    const work = display.workArea;
    const minX = work.x + EDGE_PADDING_PX;
    const maxX = work.x + work.width - width - EDGE_PADDING_PX;
    if (maxX <= minX) return;

    const direction = Math.random() < 0.5 ? -1 : 1;
    const distance = Math.round(
      randomBetween(MIN_WANDER_DISTANCE_PX, MAX_WANDER_DISTANCE_PX),
    );
    const desiredX = Math.max(minX, Math.min(maxX, x + direction * distance));
    if (Math.abs(desiredX - x) < 12) return;

    const resolvedDirection = desiredX < x ? -1 : 1;
    const requestKey = resolvedDirection < 0 ? "walk_left" : "walk_right";
    this.scheduler.submit({
      actionKey: requestKey,
      source: EventSources.AUTONOMOUS,
      priority: ActionPriorities.RANDOM_IDLE,
      interrupt: false,
      dedupeKey: "desktop_world_wander",
      metadata: { worldMotion: "wander" },
    });

    this.movementTimer = setInterval(() => {
      if (this.dragging || this.falling) {
        this.stopMovement();
        return;
      }
      const currentWin = this.windowAdapter.getNativeWindow();
      if (!currentWin || currentWin.isDestroyed()) {
        this.stopMovement();
        return;
      }
      const [currentX, currentY] = currentWin.getPosition();
      const remaining = desiredX - currentX;
      if (Math.abs(remaining) <= WALK_SPEED_PX_PER_TICK) {
        currentWin.setPosition(Math.round(desiredX), currentY, false);
        this.stopMovement();
        return;
      }
      const step = Math.sign(remaining) * WALK_SPEED_PX_PER_TICK;
      currentWin.setPosition(Math.round(currentX + step), currentY, false);
    }, MOVE_TICK_MS);
    this.movementTimer.unref?.();
  }

  private startFallIfNeeded(): void {
    if (this.dragging || this.movementTimer) return;
    const win = this.windowAdapter.getNativeWindow();
    if (!win || win.isDestroyed()) return;

    const [x, y] = win.getPosition();
    const [width, height] = win.getSize();
    const display = screen.getDisplayNearestPoint({
      x: x + Math.floor(width / 2),
      y: y + Math.floor(height / 2),
    });
    const work = display.workArea;
    const floorY = work.y + work.height - height - EDGE_PADDING_PX;
    const clampedX = Math.max(
      work.x + EDGE_PADDING_PX,
      Math.min(work.x + work.width - width - EDGE_PADDING_PX, x),
    );

    if (y >= floorY - 2) {
      win.setPosition(Math.round(clampedX), Math.round(floorY), false);
      this.scheduler.submit({
        actionKey: "land",
        source: EventSources.AUTONOMOUS,
        priority: ActionPriorities.FALL,
        interrupt: true,
        dedupeKey: "desktop_world_land",
        metadata: { worldMotion: "land" },
      });
      return;
    }

    this.falling = true;
    this.scheduler.submit({
      actionKey: "fall",
      source: EventSources.AUTONOMOUS,
      priority: ActionPriorities.FALL,
      interrupt: true,
      dedupeKey: "desktop_world_fall",
      metadata: { worldMotion: "fall" },
    });

    let velocity = 0;
    this.movementTimer = setInterval(() => {
      if (this.dragging) {
        this.falling = false;
        this.stopMovement();
        return;
      }
      const currentWin = this.windowAdapter.getNativeWindow();
      if (!currentWin || currentWin.isDestroyed()) {
        this.falling = false;
        this.stopMovement();
        return;
      }

      const [currentX, currentY] = currentWin.getPosition();
      velocity = Math.min(MAX_FALL_SPEED_PX_PER_TICK, velocity + GRAVITY_PX_PER_TICK);
      const nextY = Math.min(floorY, currentY + velocity);
      const nextX = Math.max(
        work.x + EDGE_PADDING_PX,
        Math.min(work.x + work.width - width - EDGE_PADDING_PX, currentX),
      );
      currentWin.setPosition(Math.round(nextX), Math.round(nextY), false);

      if (nextY >= floorY) {
        this.falling = false;
        this.stopMovement();
        this.scheduler.submit({
          actionKey: "land",
          source: EventSources.AUTONOMOUS,
          priority: ActionPriorities.FALL,
          interrupt: true,
          dedupeKey: "desktop_world_land",
          metadata: { worldMotion: "land" },
        });
      }
    }, MOVE_TICK_MS);
    this.movementTimer.unref?.();
  }

  private stopMovement(): void {
    if (this.movementTimer) {
      clearInterval(this.movementTimer);
      this.movementTimer = null;
    }
  }

  private scheduleNextWander(): void {
    this.nextWanderAt = Date.now() + Math.round(
      randomBetween(MIN_WANDER_DELAY_MS, MAX_WANDER_DELAY_MS),
    );
  }
}
