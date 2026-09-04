import { reactive, computed } from "vue";

export interface PerformanceMetric {
  contributionId: string;
  slotId: string;
  loadTimeMs: number;
  renderTimeMs?: number;
  actionCount: number;
  errorCount: number;
  timestamp: number;
}

export interface PerformanceBudget {
  loadTimeMs?: number;
  renderTimeMs?: number;
  actionCount?: number;
  errorCount?: number;
}

export interface BudgetViolation {
  field: keyof PerformanceBudget;
  value: number;
  budget: number;
  message: string;
}

const MAX_METRICS = 100;

const metricsMap = reactive(new Map<string, PerformanceMetric>());

function metricKey(contributionId: string, slotId: string): string {
  return `${contributionId}::${slotId}`;
}

function trimIfNeeded(): void {
  if (metricsMap.size <= MAX_METRICS) return;
  const entries = [...metricsMap.entries()].sort((a, b) => a[1].timestamp - b[1].timestamp);
  const toRemove = entries.slice(0, entries.length - MAX_METRICS);
  for (const [key] of toRemove) {
    metricsMap.delete(key);
  }
}

const DEFAULT_BUDGET: Required<PerformanceBudget> = {
  loadTimeMs: 3000,
  renderTimeMs: Number.POSITIVE_INFINITY,
  actionCount: Number.POSITIVE_INFINITY,
  errorCount: 5,
};

export function useExtensionPerformance() {
  const metrics = computed<PerformanceMetric[]>(() => [...metricsMap.values()]);

  function recordLoad(contributionId: string, slotId: string, loadTimeMs: number): void {
    const key = metricKey(contributionId, slotId);
    const existing = metricsMap.get(key);
    metricsMap.set(key, {
      contributionId,
      slotId,
      loadTimeMs,
      renderTimeMs: existing?.renderTimeMs,
      actionCount: existing?.actionCount ?? 0,
      errorCount: existing?.errorCount ?? 0,
      timestamp: Date.now(),
    });
    trimIfNeeded();
  }

  function recordAction(contributionId: string, slotId: string): void {
    const key = metricKey(contributionId, slotId);
    const existing = metricsMap.get(key);
    if (existing) {
      metricsMap.set(key, {
        ...existing,
        actionCount: existing.actionCount + 1,
        timestamp: Date.now(),
      });
    } else {
      metricsMap.set(key, {
        contributionId,
        slotId,
        loadTimeMs: 0,
        actionCount: 1,
        errorCount: 0,
        timestamp: Date.now(),
      });
      trimIfNeeded();
    }
  }

  function recordError(contributionId: string, slotId: string): void {
    const key = metricKey(contributionId, slotId);
    const existing = metricsMap.get(key);
    if (existing) {
      metricsMap.set(key, {
        ...existing,
        errorCount: existing.errorCount + 1,
        timestamp: Date.now(),
      });
    } else {
      metricsMap.set(key, {
        contributionId,
        slotId,
        loadTimeMs: 0,
        actionCount: 0,
        errorCount: 1,
        timestamp: Date.now(),
      });
      trimIfNeeded();
    }
  }

  function getMetrics(contributionId?: string): PerformanceMetric[] {
    if (contributionId) {
      return [...metricsMap.values()].filter((m) => m.contributionId === contributionId);
    }
    return [...metricsMap.values()];
  }

  function clearMetrics(): void {
    metricsMap.clear();
  }

  function checkBudget(
    metric: PerformanceMetric,
    budget: PerformanceBudget = {}
  ): BudgetViolation[] {
    const merged: Required<PerformanceBudget> = { ...DEFAULT_BUDGET, ...budget };
    const violations: BudgetViolation[] = [];
    if (metric.loadTimeMs > merged.loadTimeMs) {
      violations.push({
        field: "loadTimeMs",
        value: metric.loadTimeMs,
        budget: merged.loadTimeMs,
        message: `加载时间 ${metric.loadTimeMs}ms 超出预算 ${merged.loadTimeMs}ms`,
      });
    }
    if (metric.renderTimeMs !== undefined && metric.renderTimeMs > merged.renderTimeMs) {
      violations.push({
        field: "renderTimeMs",
        value: metric.renderTimeMs,
        budget: merged.renderTimeMs,
        message: `渲染时间 ${metric.renderTimeMs}ms 超出预算 ${merged.renderTimeMs}ms`,
      });
    }
    if (metric.actionCount > merged.actionCount) {
      violations.push({
        field: "actionCount",
        value: metric.actionCount,
        budget: merged.actionCount,
        message: `操作次数 ${metric.actionCount} 超出预算 ${merged.actionCount}`,
      });
    }
    if (metric.errorCount > merged.errorCount) {
      violations.push({
        field: "errorCount",
        value: metric.errorCount,
        budget: merged.errorCount,
        message: `错误次数 ${metric.errorCount} 超出预算 ${merged.errorCount}`,
      });
    }
    return violations;
  }

  return {
    metrics,
    recordLoad,
    recordAction,
    recordError,
    getMetrics,
    clearMetrics,
    checkBudget,
  };
}
