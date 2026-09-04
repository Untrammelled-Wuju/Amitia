import type { TaskProgressClient, TaskProgressReport } from "@amitia/plugin-sdk";
import type { RpcClient } from "./rpc.js";

const THROTTLE_INTERVAL_MS = 200;

export class ProgressClient implements TaskProgressClient {
  private sequence: number = 0;
  private lastSendTime: number = 0;
  private pendingReport: TaskProgressReport | null = null;
  private flushTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly rpc: RpcClient,
    private readonly taskRunId: string,
  ) {}

  async report(progress: TaskProgressReport): Promise<void> {
    this.sequence += 1;
    const now = Date.now();
    const elapsed = now - this.lastSendTime;
    const percentage = this.calculatePercentage(progress);

    if (elapsed >= THROTTLE_INTERVAL_MS) {
      this.lastSendTime = now;
      this.pendingReport = null;
      this.sendReport(progress, percentage);
      return;
    }

    this.pendingReport = progress;
    if (this.flushTimer === null) {
      const delay = THROTTLE_INTERVAL_MS - elapsed;
      this.flushTimer = setTimeout(() => {
        this.flushTimer = null;
        if (this.pendingReport !== null) {
          const pending = this.pendingReport;
          this.pendingReport = null;
          this.lastSendTime = Date.now();
          const pct = this.calculatePercentage(pending);
          this.sendReport(pending, pct);
        }
      }, delay);
    }
  }

  private calculatePercentage(progress: TaskProgressReport): number | undefined {
    if (
      progress.total !== undefined &&
      progress.current !== undefined &&
      progress.total > 0
    ) {
      return Math.min(100, Math.round((progress.current / progress.total) * 100));
    }
    return undefined;
  }

  private sendReport(progress: TaskProgressReport, percentage: number | undefined): void {
    this.rpc.notify("task.progress", {
      task_run_id: this.taskRunId,
      sequence: this.sequence,
      current: progress.current,
      total: progress.total,
      percentage,
      stage: progress.stage,
      message: progress.message,
    });
  }

  flush(): void {
    if (this.flushTimer !== null) {
      clearTimeout(this.flushTimer);
      this.flushTimer = null;
    }
    if (this.pendingReport !== null) {
      const pending = this.pendingReport;
      this.pendingReport = null;
      this.lastSendTime = Date.now();
      const pct = this.calculatePercentage(pending);
      this.sendReport(pending, pct);
    }
  }
}
