import type { LoadedInstallation, RuntimeAction } from "./resource-loader";
import type { DesktopPetPlayerPort } from "./player-port";
import type { DesktopPetActionScheduler } from "./action-scheduler";

export interface IdleControllerConfig {
  enabled: boolean;
  minIntervalSeconds: number;
  maxIntervalSeconds: number;
  maxRepeatCount: number;
  recentActionWeight: number;
}

const DEFAULT_IDLE_CONFIG: IdleControllerConfig = {
  enabled: true,
  minIntervalSeconds: 30,
  maxIntervalSeconds: 120,
  maxRepeatCount: 2,
  recentActionWeight: 0.3,
};

const RECENT_ACTION_HISTORY_SIZE = 3;
const SMALL_POOL_THRESHOLD = 3;

interface WeightedCandidate {
  action: RuntimeAction;
  weight: number;
}

export class IdleController {
  private player: DesktopPetPlayerPort;
  private scheduler: DesktopPetActionScheduler;
  private loaded: LoadedInstallation | null = null;
  private config: IdleControllerConfig;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private recentActions: string[] = [];
  private lastRandomAction: string | null = null;
  private lastRandomRepeatCount = 0;
  private running = false;

  constructor(
    player: DesktopPetPlayerPort,
    scheduler: DesktopPetActionScheduler,
    config?: Partial<IdleControllerConfig>,
  ) {
    this.player = player;
    this.scheduler = scheduler;
    this.config = { ...DEFAULT_IDLE_CONFIG, ...(config ?? {}) };
  }

  attachLoaded(loaded: LoadedInstallation): void {
    this.loaded = loaded;
  }

  detachLoaded(): void {
    this.stop();
    this.loaded = null;
  }

  start(): void {
    if (this.running) return;
    this.running = true;
    this.recentActions = [];
    this.lastRandomAction = null;
    this.lastRandomRepeatCount = 0;
    this.playDefaultIdle();
    if (this.config.enabled) {
      this.scheduleNext();
    }
  }

  stop(): void {
    this.running = false;
    this.cancelTimer();
  }

  reset(): void {
    if (!this.running) return;
    this.cancelTimer();
    this.playDefaultIdle();
    if (this.config.enabled) {
      this.scheduleNext();
    }
  }

  playDefaultIdle(): void {
    if (!this.loaded) return;
    const defaultAction = this.loaded.defaultAction;
    if (!defaultAction) return;
    if (!defaultAction.available) return;
    if (defaultAction.loopType !== "loop") return;
    this.player.play(defaultAction);
  }

  playRandomIdle(): void {
    if (!this.loaded) return;
    const action = this.selectRandomAction();
    if (!action) {
      this.playDefaultIdle();
      return;
    }
    this.recordRecentAction(action.key);
    if (action.key === this.lastRandomAction) {
      this.lastRandomRepeatCount += 1;
    } else {
      this.lastRandomAction = action.key;
      this.lastRandomRepeatCount = 1;
    }
    this.player.play(action);
  }

  private selectRandomAction(): RuntimeAction | null {
    if (!this.loaded) return null;
    const candidates = this.collectCandidates();
    if (candidates.length === 0) return null;
    const weighted = this.applyWeights(candidates);
    if (weighted.length === 0) return null;
    return this.pickWeighted(weighted);
  }

  private collectCandidates(): RuntimeAction[] {
    if (!this.loaded) return [];
    const result: RuntimeAction[] = [];
    for (const action of this.loaded.actions.values()) {
      if (!action.available) continue;
      if (action.loopType !== "loop") continue;
      result.push(action);
    }
    return result;
  }

  private applyWeights(candidates: RuntimeAction[]): WeightedCandidate[] {
    const smallPool = candidates.length <= SMALL_POOL_THRESHOLD;
    const defaultKey = this.loaded?.defaultAction?.key ?? "";
    const result: WeightedCandidate[] = [];
    for (const action of candidates) {
      let weight = 1;
      const isDefault = action.key === defaultKey;
      if (!smallPool && !isDefault) {
        if (
          action.key === this.lastRandomAction &&
          this.lastRandomRepeatCount >= this.config.maxRepeatCount
        ) {
          continue;
        }
        if (this.recentActions.includes(action.key)) {
          weight *= this.config.recentActionWeight;
        }
      }
      if (weight <= 0) continue;
      result.push({ action, weight });
    }
    return result;
  }

  private pickWeighted(
    weighted: WeightedCandidate[],
  ): RuntimeAction | null {
    if (weighted.length === 0) return null;
    const total = weighted.reduce((sum, entry) => sum + entry.weight, 0);
    if (total <= 0) return weighted[0].action;
    let pick = Math.random() * total;
    for (const entry of weighted) {
      pick -= entry.weight;
      if (pick <= 0) return entry.action;
    }
    return weighted[weighted.length - 1].action;
  }

  private recordRecentAction(key: string): void {
    this.recentActions.unshift(key);
    if (this.recentActions.length > RECENT_ACTION_HISTORY_SIZE) {
      this.recentActions.length = RECENT_ACTION_HISTORY_SIZE;
    }
  }

  private scheduleNext(): void {
    this.cancelTimer();
    const delayMs = this.randomIntervalMs();
    this.timer = setTimeout(() => this.onTimerFire(), delayMs);
  }

  private onTimerFire(): void {
    this.timer = null;
    if (!this.running) return;
    this.playRandomIdle();
    this.scheduleNext();
  }

  private randomIntervalMs(): number {
    const minSec = Math.max(0, this.config.minIntervalSeconds);
    const maxSec = Math.max(minSec, this.config.maxIntervalSeconds);
    const seconds = minSec + Math.random() * (maxSec - minSec);
    return Math.round(seconds * 1000);
  }

  private cancelTimer(): void {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }
}
