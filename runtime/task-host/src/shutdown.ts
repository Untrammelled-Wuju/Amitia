import type { RpcClient } from "./rpc.js";

export interface ShutdownOptions {
  rpc: RpcClient;
  taskRunId: string;
  reason: string;
  flushProgress?: () => void;
  finalCheckpoint?: () => Promise<void>;
}

export async function gracefulShutdown(options: ShutdownOptions): Promise<void> {
  const { rpc, taskRunId, reason, flushProgress, finalCheckpoint } = options;

  if (flushProgress) {
    try {
      flushProgress();
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  if (finalCheckpoint) {
    try {
      await finalCheckpoint();
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  if (!rpc.isClosed()) {
    try {
      rpc.notify("task.shutdown", {
        task_run_id: taskRunId,
        reason,
      });
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  await sleep(100);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
