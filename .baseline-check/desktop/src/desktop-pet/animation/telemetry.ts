import type {
  AnimationDiagnostics,
  PlaybackErrorView,
  PlaybackEvent,
  PlaybackSnapshot,
  PlayerPhase,
} from "./contracts";
import { ANIMATION_ENGINE_VERSION } from "./contracts";

interface MetricEntry {
  readonly timestamp: number;
  readonly value: number;
}

interface TransitionEntry {
  readonly from: string;
  readonly to: string;
  readonly reason: string;
  readonly timestamp: number;
}

export class PlaybackTelemetry {
  private defaultActionFirstFrameMs: number | null = null;
  private actionSwitchFirstFrameMs = new Map<string, number>();
  private actionLoadMs = new Map<string, MetricEntry[]>();
  private frameDecodeMs: number[] = [];
  private framePresentMs: number[] = [];
  private estimatedDroppedFrames = 0;
  private commandRejectCount = 0;
  private actionLoadFailureCount = 0;
  private fallbackCount = 0;
  private decodedCacheHitRate = { hits: 0, misses: 0 };
  private queueLengthSamples: number[] = [];
  private activeDecodeJobs = 0;
  private clockLargeGapCount = 0;
  private rendererRecoveryCount = 0;
  private recentTransitions: TransitionEntry[] = [];
  private recentErrors: PlaybackErrorView[] = [];
  private maxHistorySize = 100;

  recordDefaultActionFirstFrame(ms: number): void {
    if (this.defaultActionFirstFrameMs === null) {
      this.defaultActionFirstFrameMs = ms;
    }
  }

  recordActionSwitchFirstFrame(actionKey: string, ms: number): void {
    this.actionSwitchFirstFrameMs.set(actionKey, ms);
  }

  recordActionLoad(actionKey: string, ms: number): void {
    const entries = this.actionLoadMs.get(actionKey) ?? [];
    entries.push({ timestamp: Date.now(), value: ms });
    if (entries.length > this.maxHistorySize) {
      entries.shift();
    }
    this.actionLoadMs.set(actionKey, entries);
  }

  recordFrameDecode(ms: number): void {
    this.frameDecodeMs.push(ms);
    if (this.frameDecodeMs.length > this.maxHistorySize) {
      this.frameDecodeMs.shift();
    }
  }

  recordFramePresent(ms: number): void {
    this.framePresentMs.push(ms);
    if (this.framePresentMs.length > this.maxHistorySize) {
      this.framePresentMs.shift();
    }
  }

  recordDroppedFrames(count: number): void {
    this.estimatedDroppedFrames += count;
  }

  recordCommandReject(): void {
    this.commandRejectCount += 1;
  }

  recordActionLoadFailure(): void {
    this.actionLoadFailureCount += 1;
  }

  recordFallback(): void {
    this.fallbackCount += 1;
  }

  recordCacheHit(): void {
    this.decodedCacheHitRate.hits += 1;
  }

  recordCacheMiss(): void {
    this.decodedCacheHitRate.misses += 1;
  }

  recordQueueLength(length: number): void {
    this.queueLengthSamples.push(length);
    if (this.queueLengthSamples.length > this.maxHistorySize) {
      this.queueLengthSamples.shift();
    }
  }

  recordActiveDecodeJobs(count: number): void {
    this.activeDecodeJobs = count;
  }

  recordClockLargeGap(): void {
    this.clockLargeGapCount += 1;
  }

  recordRendererRecovery(): void {
    this.rendererRecoveryCount += 1;
  }

  recordTransition(from: string, to: string, reason: string): void {
    this.recentTransitions.push({
      from,
      to,
      reason,
      timestamp: Date.now(),
    });
    if (this.recentTransitions.length > this.maxHistorySize) {
      this.recentTransitions.shift();
    }
  }

  recordError(error: PlaybackErrorView): void {
    this.recentErrors.push(error);
    if (this.recentErrors.length > this.maxHistorySize) {
      this.recentErrors.shift();
    }
  }

  getDiagnostics(
    snapshot: PlaybackSnapshot,
    currentAction: {
      key: string;
      loopType: string;
      frameCount: number;
      cycleDurationMs: number;
      loadedBytes: number;
    } | null,
    queue: ReadonlyArray<{ actionKey: string; priority: number; expiresAt?: string }>,
    cacheStats: { budgetBytes: number; usedBytes: number; entries: number },
    clockInfo: { visible: boolean; suspended: boolean; lastGapMs: number },
  ): AnimationDiagnostics {
    return {
      engineVersion: ANIMATION_ENGINE_VERSION,
      snapshot,
      currentAction: currentAction
        ? {
            key: currentAction.key,
            loopType: currentAction.loopType as "loop" | "once" | "hold" | "ping_pong",
            frameCount: currentAction.frameCount,
            cycleDurationMs: currentAction.cycleDurationMs,
            loadedBytes: currentAction.loadedBytes,
          }
        : undefined,
      queue,
      cache: cacheStats,
      clock: clockInfo,
      recentTransitions: this.recentTransitions.slice(-20).map((t) => ({
        from: t.from,
        to: t.to,
        reason: t.reason,
      })),
      recentErrors: this.recentErrors.slice(-20),
    };
  }

  getMetrics(): {
    defaultActionFirstFrameMs: number | null;
    actionSwitchFirstFrameMs: Map<string, number>;
    frameDecodeMsP50: number;
    frameDecodeMsP95: number;
    framePresentMsP50: number;
    framePresentMsP95: number;
    estimatedDroppedFrames: number;
    commandRejectCount: number;
    actionLoadFailureCount: number;
    fallbackCount: number;
    cacheHitRate: number;
    queueLengthMax: number;
    activeDecodeJobs: number;
    clockLargeGapCount: number;
    rendererRecoveryCount: number;
  } {
    return {
      defaultActionFirstFrameMs: this.defaultActionFirstFrameMs,
      actionSwitchFirstFrameMs: this.actionSwitchFirstFrameMs,
      frameDecodeMsP50: percentile(this.frameDecodeMs, 0.5),
      frameDecodeMsP95: percentile(this.frameDecodeMs, 0.95),
      framePresentMsP50: percentile(this.framePresentMs, 0.5),
      framePresentMsP95: percentile(this.framePresentMs, 0.95),
      estimatedDroppedFrames: this.estimatedDroppedFrames,
      commandRejectCount: this.commandRejectCount,
      actionLoadFailureCount: this.actionLoadFailureCount,
      fallbackCount: this.fallbackCount,
      cacheHitRate: this.decodedCacheHitRate.hits + this.decodedCacheHitRate.misses > 0
        ? this.decodedCacheHitRate.hits / (this.decodedCacheHitRate.hits + this.decodedCacheHitRate.misses)
        : 0,
      queueLengthMax: this.queueLengthSamples.length > 0
        ? Math.max(...this.queueLengthSamples)
        : 0,
      activeDecodeJobs: this.activeDecodeJobs,
      clockLargeGapCount: this.clockLargeGapCount,
      rendererRecoveryCount: this.rendererRecoveryCount,
    };
  }

  reset(): void {
    this.defaultActionFirstFrameMs = null;
    this.actionSwitchFirstFrameMs.clear();
    this.actionLoadMs.clear();
    this.frameDecodeMs = [];
    this.framePresentMs = [];
    this.estimatedDroppedFrames = 0;
    this.commandRejectCount = 0;
    this.actionLoadFailureCount = 0;
    this.fallbackCount = 0;
    this.decodedCacheHitRate = { hits: 0, misses: 0 };
    this.queueLengthSamples = [];
    this.activeDecodeJobs = 0;
    this.clockLargeGapCount = 0;
    this.rendererRecoveryCount = 0;
    this.recentTransitions = [];
    this.recentErrors = [];
  }
}

function percentile(arr: number[], p: number): number {
  if (arr.length === 0) return 0;
  const sorted = [...arr].sort((a, b) => a - b);
  const idx = Math.min(Math.floor(sorted.length * p), sorted.length - 1);
  return sorted[idx];
}

export type { PlaybackEvent, PlayerPhase };
