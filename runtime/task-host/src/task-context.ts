import type {
  TaskContext,
  TaskLogger,
  TaskCheckpoint,
  TaskArtifactClient,
  TaskArtifact,
  TaskArtifactOptions,
  TaskStorageClient,
  TaskHostClient,
} from "@amitia/plugin-sdk";
import type { RpcClient } from "./rpc.js";
import { CheckpointClient } from "./checkpoint.js";
import { ProgressClient } from "./progress.js";

export interface TaskContextOptions {
  rpc: RpcClient;
  taskRunId: string;
  taskId: string;
  deadline?: number;
  attempt: number;
  maxAttempts: number;
  initialCheckpoint: TaskCheckpoint | null;
  signal: AbortSignal;
}

export class LoggerClient implements TaskLogger {
  constructor(
    private readonly rpc: RpcClient,
    private readonly taskRunId: string,
  ) {}

  debug(message: string, fields?: Record<string, unknown>): void {
    this.rpc.notify("log.write", {
      task_run_id: this.taskRunId,
      level: "debug",
      message,
      fields,
    });
  }

  info(message: string, fields?: Record<string, unknown>): void {
    this.rpc.notify("log.write", {
      task_run_id: this.taskRunId,
      level: "info",
      message,
      fields,
    });
  }

  warn(message: string, fields?: Record<string, unknown>): void {
    this.rpc.notify("log.write", {
      task_run_id: this.taskRunId,
      level: "warn",
      message,
      fields,
    });
  }

  error(message: string, fields?: Record<string, unknown>): void {
    this.rpc.notify("log.write", {
      task_run_id: this.taskRunId,
      level: "error",
      message,
      fields,
    });
  }
}

export class ArtifactClient implements TaskArtifactClient {
  constructor(
    private readonly rpc: RpcClient,
    private readonly taskRunId: string,
  ) {}

  async saveFile(
    name: string,
    content: Uint8Array,
    options?: TaskArtifactOptions,
  ): Promise<TaskArtifact> {
    const response = await this.rpc.call<TaskArtifact>("task.artifact.saveFile", {
      task_run_id: this.taskRunId,
      name,
      content: Buffer.from(content).toString("base64"),
      options,
    });
    return response;
  }

  async saveData(
    name: string,
    data: unknown,
    options?: TaskArtifactOptions,
  ): Promise<TaskArtifact> {
    const response = await this.rpc.call<TaskArtifact>("task.artifact.saveData", {
      task_run_id: this.taskRunId,
      name,
      data,
      options,
    });
    return response;
  }

  async list(): Promise<TaskArtifact[]> {
    const response = await this.rpc.call<TaskArtifact[]>("task.artifact.list", {
      task_run_id: this.taskRunId,
    });
    return response;
  }
}

export class StorageClient implements TaskStorageClient {
  constructor(
    private readonly rpc: RpcClient,
    private readonly taskRunId: string,
  ) {}

  async get<T>(key: string): Promise<T | null> {
    const response = await this.rpc.call<{ value: T | null }>("task.storage.get", {
      task_run_id: this.taskRunId,
      key,
    });
    return response.value ?? null;
  }

  async set<T>(key: string, value: T): Promise<void> {
    await this.rpc.call("task.storage.set", {
      task_run_id: this.taskRunId,
      key,
      value,
    });
  }

  async delete(key: string): Promise<void> {
    await this.rpc.call("task.storage.delete", {
      task_run_id: this.taskRunId,
      key,
    });
  }
}

export class HostClient implements TaskHostClient {
  constructor(
    private readonly rpc: RpcClient,
    private readonly taskRunId: string,
  ) {}

  async executeTool(toolId: string, input: unknown, timeoutMs?: number): Promise<unknown> {
    const response = await this.rpc.call<{ result: unknown }>(
      "task.host.executeTool",
      {
        task_run_id: this.taskRunId,
        tool_id: toolId,
        input,
        timeout_ms: timeoutMs,
      },
    );
    return response.result;
  }

  async emitEvent(type: string, payload: unknown): Promise<void> {
    this.rpc.notify("task.host.emitEvent", {
      task_run_id: this.taskRunId,
      type,
      payload,
    });
  }
}

export interface TaskContextBundle {
  context: TaskContext;
  progress: ProgressClient;
  checkpoint: CheckpointClient;
}

export function createTaskContext(options: TaskContextOptions): TaskContextBundle {
  const { rpc, taskRunId, taskId, deadline, attempt, maxAttempts, initialCheckpoint, signal } =
    options;

  const logger = new LoggerClient(rpc, taskRunId);
  const progress = new ProgressClient(rpc, taskRunId);
  const checkpoint = new CheckpointClient(rpc, taskRunId, initialCheckpoint);
  const artifacts = new ArtifactClient(rpc, taskRunId);
  const storage = new StorageClient(rpc, taskRunId);
  const host = new HostClient(rpc, taskRunId);

  const context: TaskContext = {
    taskId,
    traceId: taskRunId,
    deadline,
    signal,
    logger,
    progress,
    checkpoint,
    artifacts,
    storage,
    host,
    attempt,
    maxAttempts,
  };

  return { context, progress, checkpoint };
}
