import type { TaskCheckpointClient, TaskCheckpoint } from "@amitia/plugin-sdk";
import type { RpcClient } from "./rpc.js";

export class CheckpointClient implements TaskCheckpointClient {
  private currentCheckpoint: TaskCheckpoint | null;
  private version: number;

  constructor(
    private readonly rpc: RpcClient,
    private readonly taskRunId: string,
    initialCheckpoint: TaskCheckpoint | null,
  ) {
    this.currentCheckpoint = initialCheckpoint;
    this.version = initialCheckpoint?.cursor ?? 0;
  }

  async save(checkpoint: TaskCheckpoint): Promise<void> {
    this.version += 1;
    const enrichedCheckpoint: TaskCheckpoint = {
      ...checkpoint,
      cursor: this.version,
      savedAt: checkpoint.savedAt || new Date().toISOString(),
    };
    this.currentCheckpoint = enrichedCheckpoint;
    this.rpc.notify("task.checkpoint", {
      task_run_id: this.taskRunId,
      version: this.version,
      payload: enrichedCheckpoint,
    });
  }

  async load(): Promise<TaskCheckpoint | null> {
    return this.currentCheckpoint;
  }

  getCurrent(): TaskCheckpoint | null {
    return this.currentCheckpoint;
  }
}
