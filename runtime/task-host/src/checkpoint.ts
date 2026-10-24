import type { TaskCheckpointClient, TaskCheckpoint, TaskNextEffect } from "@amitia/plugin-sdk/tasks";
import { createHash } from "node:crypto";
import type { RpcClient } from "./rpc.js";

const validEffectID = (id: unknown): id is string =>
  typeof id === "string" && /^[a-zA-Z0-9_.:-]{1,128}$/.test(id);

function hashEffectInput(value: unknown): string {
  const serialized = JSON.stringify(value);
  if (typeof serialized !== "string" || Buffer.byteLength(serialized, "utf8") > 65536) {
    throw new Error("计划的效果参数必须是64KiB以内的可序列化JSON");
  }
  return createHash("sha256").update(serialized, "utf8").digest("hex");
}

function normalizeNextEffects(effects: readonly TaskNextEffect[]): TaskNextEffect[] {
  if (!Array.isArray(effects) || effects.length < 1 || effects.length > 32) {
    throw new Error("nextEffects 必须包含1-32项");
  }
  const seen = new Set<string>();
  return effects.map(effect => {
    if (!effect || !validEffectID(effect.effectId) || seen.has(effect.effectId)) {
      throw new Error("nextEffects 包含无效或重复的EffectID");
    }
    seen.add(effect.effectId);
    if (effect.method === "task.host.executeTool") {
      if (typeof effect.toolId !== "string" || !effect.toolId || effect.toolId.length > 256 ||
          (effect.timeoutMs !== undefined &&
            (!Number.isSafeInteger(effect.timeoutMs) || effect.timeoutMs < 0 || effect.timeoutMs > 5000))) {
        throw new Error("nextEffects 工具标识或超时参数无效");
      }
      const inputHash = effect.input === undefined ? effect.inputHash : hashEffectInput(effect.input);
      if (typeof inputHash !== "string" || !/^[a-f0-9]{64}$/.test(inputHash)) {
        throw new Error("nextEffects 缺少工具参数的SHA-256摘要");
      }
      return { effectId: effect.effectId, method: effect.method, toolId: effect.toolId,
        inputHash, timeoutMs: effect.timeoutMs ?? 0 };
    }
    if (effect.method === "task.host.emitEvent") {
      if (typeof effect.type !== "string" || !effect.type || effect.type.length > 256) {
        throw new Error("nextEffects 事件类型无效");
      }
      const payloadHash = effect.payload === undefined ? effect.payloadHash : hashEffectInput(effect.payload);
      if (typeof payloadHash !== "string" || !/^[a-f0-9]{64}$/.test(payloadHash)) {
        throw new Error("nextEffects 缺少事件载荷的SHA-256摘要");
      }
      return { effectId: effect.effectId, method: effect.method, type: effect.type, payloadHash };
    }
    throw new Error("nextEffects 包含不允许的执行方法");
  });
}

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
    if (checkpoint.nextEffectIds !== undefined) {
      const ids = checkpoint.nextEffectIds;
      if (!Array.isArray(ids) || ids.length === 0 || ids.length > 32 ||
          ids.some(id => typeof id !== "string" || !/^[a-zA-Z0-9_.:-]{1,128}$/.test(id)) ||
          new Set(ids).size !== ids.length) {
        throw new Error("检查点 nextEffectIds 必须为1-32个不重复的稳定效果编号");
      }
    }
    if (checkpoint.nextEffects !== undefined && checkpoint.nextEffectIds !== undefined) {
      throw new Error("新旧后续效果计划不可同时存在");
    }
    const nextEffects = checkpoint.nextEffects === undefined
      ? undefined : normalizeNextEffects(checkpoint.nextEffects);
    const nextVersion = this.version + 1;
    const enrichedCheckpoint: TaskCheckpoint = {
      ...checkpoint,
      ...(nextEffects === undefined ? {} : { nextEffects }),
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
