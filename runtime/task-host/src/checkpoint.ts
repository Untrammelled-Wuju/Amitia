import type { TaskCheckpointClient, TaskCheckpoint } from "@amitia/plugin-sdk";
import type { RpcClient } from "./rpc.js";

export class CheckpointClient implements TaskCheckpointClient {
  private currentCheckpoint: TaskCheckpoint | null;
  private version: number;
  private saving = false;
  private saveFailed = false;
  private pendingSave: Promise<void> | null = null;

  constructor(
    private readonly rpc: RpcClient,
    private readonly taskRunId: string,
    initialCheckpoint: TaskCheckpoint | null,
  ) {
    const cursor = initialCheckpoint?.cursor ?? 0;
    if (!Number.isSafeInteger(cursor) || cursor < 0) {
      throw new Error("检查点版本无效");
    }
    this.currentCheckpoint = initialCheckpoint;
    this.version = cursor;
  }

  async save(checkpoint: TaskCheckpoint): Promise<void> {
    if (this.saving || this.saveFailed) throw new Error("检查点正在保存或上次保存尚未确认");
    if (this.version >= Number.MAX_SAFE_INTEGER) {
      throw new Error("检查点版本已超出范围");
    }
    const nextVersion = this.version + 1;
    const enrichedCheckpoint: TaskCheckpoint = {
      ...checkpoint,
      cursor: nextVersion,
      savedAt: checkpoint.savedAt || new Date().toISOString(),
    };
    this.saving = true;
    this.pendingSave = this.confirmSave(nextVersion, enrichedCheckpoint);
    await this.pendingSave;
  }

  async waitForConfirmation(): Promise<void> {
    if (this.pendingSave) await this.pendingSave;
    if (this.saveFailed) throw new Error("检查点保存尚未确认");
  }

  private async confirmSave(nextVersion: number, enrichedCheckpoint: TaskCheckpoint): Promise<void> {
    try {
      const acknowledgement = await this.rpc.call<{ version: number }>("task.checkpoint.save", {
        task_run_id: this.taskRunId,
        version: nextVersion,
        payload: enrichedCheckpoint,
      });
      if (acknowledgement?.version !== nextVersion) throw new Error("检查点保存确认版本不一致");
      this.version = nextVersion;
      this.currentCheckpoint = enrichedCheckpoint;
    } catch (error) {
      this.saveFailed = true;
      throw error;
    } finally {
      this.saving = false;
      this.pendingSave = null;
    }
  }

  async load(): Promise<TaskCheckpoint | null> {
    return this.currentCheckpoint;
  }

  getCurrent(): TaskCheckpoint | null {
    return this.currentCheckpoint;
  }
}
