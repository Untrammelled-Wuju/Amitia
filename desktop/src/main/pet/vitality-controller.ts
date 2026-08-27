import { screen } from "electron";
import { ActionPriorities, EventSources, type DesktopPetActionScheduler } from "./action-scheduler";
import type { DesktopPetWindowAdapter } from "./window-adapter";

export interface PetVitalitySnapshot {
  mood: number;
  energy: number;
  attention: number;
  boredom: number;
  affinity: number;
  lastInteractionAt: number;
  activity: "idle" | "attentive" | "working" | "resting";
}

const TICK_MS = 5000;
const AUTONOMOUS_MIN_GAP_MS = 30000;
const CURSOR_NEAR_DISTANCE = 180;

export class DesktopPetVitalityController {
  private timer: ReturnType<typeof setInterval> | null = null;
  private lastAutonomousAt = 0;
  private activityExpiresAt = 0;
  private state: PetVitalitySnapshot = {
    mood: 0.65, energy: 0.8, attention: 0.5, boredom: 0.1, affinity: 0.5,
    lastInteractionAt: Date.now(), activity: "idle",
  };

  constructor(
    private readonly scheduler: DesktopPetActionScheduler,
    private readonly windowAdapter: DesktopPetWindowAdapter,
  ) {}

  start(): void {
    if (this.timer) return;
    this.timer = setInterval(() => this.tick(), TICK_MS);
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }

  dispose(): void { this.stop(); }

  notifyInteraction(kind: "click" | "hover" | "drag" | "chat" | "voice" | "tool"): void {
    const now = Date.now();
    this.state.lastInteractionAt = now;
    this.state.attention = Math.min(1, this.state.attention + (kind === "chat" || kind === "voice" ? 0.25 : 0.12));
    this.state.boredom = Math.max(0, this.state.boredom - 0.35);
    if (kind === "click" || kind === "chat") this.state.affinity = Math.min(1, this.state.affinity + 0.01);
    if (kind === "tool") {
      this.state.activity = "working";
      this.activityExpiresAt = now + 60000;
    } else if (kind === "voice" || kind === "chat") {
      this.state.activity = "attentive";
      this.activityExpiresAt = now + 25000;
    }
  }

  setActivity(activity: PetVitalitySnapshot["activity"]): void {
    this.state.activity = activity;
    this.activityExpiresAt = activity === "working"
      ? Date.now() + 60000
      : activity === "attentive"
        ? Date.now() + 25000
        : 0;
  }
  snapshot(): PetVitalitySnapshot { return { ...this.state }; }

  private tick(): void {
    const now = Date.now();
    const idleMs = now - this.state.lastInteractionAt;
    if (this.activityExpiresAt > 0 && now >= this.activityExpiresAt) {
      this.state.activity = "idle";
      this.activityExpiresAt = 0;
    }
    const hour = new Date().getHours();
    const night = hour >= 0 && hour < 6;
    this.state.energy = Math.max(0.05, this.state.energy - (night ? 0.006 : 0.002));
    if (night && this.state.energy < 0.22 && this.state.activity === "idle") {
      this.state.activity = "resting";
    }
    this.state.attention = Math.max(0.15, this.state.attention - 0.01);
    this.state.boredom = Math.min(1, this.state.boredom + (idleMs > 60000 ? 0.02 : 0.005));
    this.state.mood = Math.max(0.2, Math.min(0.9, 0.45 + this.state.affinity * 0.35 - this.state.boredom * 0.15));

    this.reactToCursor(now);
    if (now - this.lastAutonomousAt < AUTONOMOUS_MIN_GAP_MS) return;
    const current = this.scheduler.getCurrent();
    if (current && current.priority > ActionPriorities.RANDOM_IDLE) return;

    let actionKey = "idle_look_around";
    if (this.state.energy < 0.25) actionKey = "tired";
    else if (this.state.boredom > 0.75) actionKey = "stretch";
    else if (this.state.mood > 0.78) actionKey = "happy";

    const result = this.scheduler.submit({
      actionKey,
      source: EventSources.AUTONOMOUS,
      priority: ActionPriorities.RANDOM_IDLE + 1,
      interrupt: false,
      dedupeKey: `autonomous_${actionKey}`,
    });
    if (result !== "rejected") this.lastAutonomousAt = now;
  }

  private reactToCursor(now: number): void {
    const win = this.windowAdapter.getNativeWindow();
    if (!win || win.isDestroyed()) return;
    const cursor = screen.getCursorScreenPoint();
    const bounds = win.getBounds();
    const cx = bounds.x + bounds.width / 2;
    const cy = bounds.y + bounds.height / 2;
    const distance = Math.hypot(cursor.x - cx, cursor.y - cy);
    if (distance > CURSOR_NEAR_DISTANCE || now - this.lastAutonomousAt < 8000) return;
    const result = this.scheduler.submit({
      actionKey: "hovered",
      source: EventSources.AUTONOMOUS,
      priority: ActionPriorities.EMOTION,
      interrupt: false,
      dedupeKey: "cursor_near_attention",
    });
    if (result !== "rejected") this.lastAutonomousAt = now;
  }
}
