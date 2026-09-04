import {
  ActionPriorities,
  EventSources,
  type DesktopPetActionScheduler,
} from "./action-scheduler";

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

const MIN_ENERGY = 0.05;
const MAX_ENERGY = 1;
const REST_ENTER_ENERGY = 0.28;
const DAY_IDLE_RECOVERY = 0.003;
const NIGHT_IDLE_DRAIN = 0.004;
const RESTING_RECOVERY = 0.018;
const ATTENTIVE_DRAIN = 0.0015;
const WORKING_DRAIN = 0.004;

const ATTENTIVE_INTERACTION_MS = 12000;
const CHAT_ATTENTIVE_MS = 25000;
const WORKING_ACTIVITY_MS = 60000;

function clamp01(value: number): number {
  return Math.max(0, Math.min(1, value));
}

function isNightHour(hour: number): boolean {
  return hour >= 0 && hour < 6;
}

export class DesktopPetVitalityController {
  private timer: ReturnType<typeof setInterval> | null = null;
  private lastAutonomousAt = 0;
  private activityExpiresAt = 0;
  private state: PetVitalitySnapshot = {
    mood: 0.65,
    energy: 0.8,
    attention: 0.5,
    boredom: 0.1,
    affinity: 0.5,
    lastInteractionAt: Date.now(),
    activity: "idle",
  };

  constructor(private readonly scheduler: DesktopPetActionScheduler) {}

  start(): void {
    if (this.timer) return;
    this.timer = setInterval(() => this.tick(), TICK_MS);
    this.timer.unref?.();
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }

  dispose(): void {
    this.stop();
  }

  notifyInteraction(kind: "click" | "hover" | "drag" | "chat" | "voice" | "tool"): void {
    const now = Date.now();
    this.state.lastInteractionAt = now;
    this.state.attention = clamp01(
      this.state.attention + (kind === "chat" || kind === "voice" ? 0.25 : 0.12),
    );
    this.state.boredom = clamp01(this.state.boredom - 0.35);

    if (kind === "click" || kind === "chat") {
      this.state.affinity = clamp01(this.state.affinity + 0.01);
    }

    // Any real user interaction wakes the pet. Previously a pet that entered
    // resting could stay there indefinitely even while it was being clicked or
    // dragged because only chat/voice/tool changed the activity state.
    if (kind === "tool") {
      this.setActivityWithExpiry("working", now + WORKING_ACTIVITY_MS);
    } else if (kind === "voice" || kind === "chat") {
      this.setActivityWithExpiry("attentive", now + CHAT_ATTENTIVE_MS);
    } else {
      this.setActivityWithExpiry("attentive", now + ATTENTIVE_INTERACTION_MS);
    }
  }

  setActivity(activity: PetVitalitySnapshot["activity"]): void {
    const now = Date.now();
    if (activity === "working") {
      this.setActivityWithExpiry(activity, now + WORKING_ACTIVITY_MS);
      return;
    }
    if (activity === "attentive") {
      this.setActivityWithExpiry(activity, now + CHAT_ATTENTIVE_MS);
      return;
    }
    this.setActivityWithExpiry(activity, 0);
  }

  snapshot(): PetVitalitySnapshot {
    return { ...this.state };
  }

  private setActivityWithExpiry(
    activity: PetVitalitySnapshot["activity"],
    expiresAt: number,
  ): void {
    this.state.activity = activity;
    this.activityExpiresAt = Math.max(0, expiresAt);
  }

  private tick(): void {
    const now = Date.now();
    const idleMs = Math.max(0, now - this.state.lastInteractionAt);

    if (this.activityExpiresAt > 0 && now >= this.activityExpiresAt) {
      this.setActivityWithExpiry("idle", 0);
    }

    const night = isNightHour(new Date(now).getHours());
    this.updateActivityForDayNight(night);
    this.updateEnergy(night);
    this.updateActivityAfterEnergy(night);

    this.state.attention = Math.max(0.15, this.state.attention - 0.01);
    this.state.boredom = clamp01(
      this.state.boredom + (idleMs > 60000 ? 0.02 : 0.005),
    );
    this.state.mood = Math.max(
      0.2,
      Math.min(
        0.9,
        0.45 + this.state.affinity * 0.35 - this.state.boredom * 0.15,
      ),
    );

    this.maybeScheduleAutonomousAction(now);
  }

  private updateActivityForDayNight(night: boolean): void {
    // Rest is an autonomous night state, not a permanent activity lock. When
    // daylight returns the pet wakes even if no chat/tool event arrived.
    if (!night && this.state.activity === "resting") {
      this.setActivityWithExpiry("idle", 0);
    }
  }

  private updateEnergy(night: boolean): void {
    let delta = 0;
    switch (this.state.activity) {
      case "working":
        delta = -WORKING_DRAIN;
        break;
      case "attentive":
        delta = -ATTENTIVE_DRAIN;
        break;
      case "resting":
        delta = RESTING_RECOVERY;
        break;
      case "idle":
      default:
        delta = night ? -NIGHT_IDLE_DRAIN : DAY_IDLE_RECOVERY;
        break;
    }

    this.state.energy = Math.max(
      MIN_ENERGY,
      Math.min(MAX_ENERGY, this.state.energy + delta),
    );
  }

  private updateActivityAfterEnergy(night: boolean): void {
    if (
      night &&
      this.state.activity === "idle" &&
      this.state.energy <= REST_ENTER_ENERGY
    ) {
      this.setActivityWithExpiry("resting", 0);
      return;
    }

    if (this.state.activity === "resting" && !night) {
      this.setActivityWithExpiry("idle", 0);
    }
  }

  private maybeScheduleAutonomousAction(now: number): void {
    if (now - this.lastAutonomousAt < AUTONOMOUS_MIN_GAP_MS) return;

    // Attentive/working are externally driven semantic states. Do not let the
    // local vitality layer invent an idle/tired action while Backend Behavior is
    // actively driving conversation or tool work.
    if (this.state.activity === "attentive" || this.state.activity === "working") {
      return;
    }

    const current = this.scheduler.getCurrent();
    if (current && current.priority > ActionPriorities.RANDOM_IDLE) return;

    let actionKey = "idle_look_around";
    if (this.state.activity === "resting" || this.state.energy < 0.25) {
      actionKey = "tired";
    } else if (this.state.boredom > 0.75) {
      actionKey = "stretch";
    } else if (this.state.mood > 0.78) {
      actionKey = "happy";
    }

    const result = this.scheduler.submit({
      actionKey,
      source: EventSources.AUTONOMOUS,
      priority: ActionPriorities.RANDOM_IDLE + 1,
      interrupt: false,
      dedupeKey: `autonomous_${actionKey}`,
    });
    if (result !== "rejected") this.lastAutonomousAt = now;
  }
}
