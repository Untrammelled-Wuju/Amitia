import type { LoadedInstallation, RuntimeAction } from "./resource-loader";
import { ActionPriorities, EventSources, type DesktopPetActionScheduler } from "./action-scheduler";

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
  private scheduler: DesktopPetActionScheduler;
  private loaded: LoadedInstallation | null = null;
  private config: IdleControllerConfig;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private recentActions: string[] = [];
  private lastRandomAction: string | null = null;
  private lastRandomRepeatCount = 0;
  private running = false;

  constructor(
    scheduler: DesktopPetActionScheduler,
    config?: Partial<IdleControllerConfig>,
  ) {
    this.scheduler = scheduler;
    this.config = this.normalizeConfig({ ...DEFAULT_IDLE_CONFIG, ...(config ?? {}) });
  }

  updateConfig(config: Partial<IdleControllerConfig>): void {
    this.config = this.normalizeConfig({ ...this.config, ...config });
    if (this.running) {
      this.reset();
    }
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
    this.scheduler.submit({
      actionKey: defaultAction.key,
      source: EventSources.IDLE,
      priority: ActionPriorities.DEFAULT_IDLE,
      interrupt: false,
      dedupeKey: `default_idle_${defaultAction.key}`,
    });
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
    this.scheduler.submit({
      actionKey: action.key,
      source: EventSources.IDLE,
      priority: ActionPriorities.RANDOM_IDLE,
      interrupt: false,
      dedupeKey: `random_idle_${action.key}`,
    });
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
      const category = (action.category ?? "").toLowerCase();
      const key = action.key.toLowerCase();
      const idleEligible = action.supportsDefaultIdle === true || category === "idle" || key.startsWith("idle_") || key === "idle";
      if (!idleEligible) continue;
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

  private normalizeConfig(config: IdleControllerConfig): IdleControllerConfig {
    const minIntervalSeconds = Number.isFinite(config.minIntervalSeconds)
      ? Math.max(0, config.minIntervalSeconds)
      : DEFAULT_IDLE_CONFIG.minIntervalSeconds;
    const maxIntervalSeconds = Number.isFinite(config.maxIntervalSeconds)
      ? Math.max(minIntervalSeconds, config.maxIntervalSeconds)
      : Math.max(minIntervalSeconds, DEFAULT_IDLE_CONFIG.maxIntervalSeconds);
    const maxRepeatCount = Number.isFinite(config.maxRepeatCount)
      ? Math.max(1, Math.round(config.maxRepeatCount))
      : DEFAULT_IDLE_CONFIG.maxRepeatCount;
    const recentActionWeight = Number.isFinite(config.recentActionWeight)
      ? Math.min(1, Math.max(0, config.recentActionWeight))
      : DEFAULT_IDLE_CONFIG.recentActionWeight;
    return {
      enabled: config.enabled === true,
      minIntervalSeconds,
      maxIntervalSeconds,
      maxRepeatCount,
      recentActionWeight,
    };
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
