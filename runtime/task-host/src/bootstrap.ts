import type { TaskInput, TaskResult, TaskCheckpoint } from "@amitia/plugin-sdk";
import { StdioRpcTransport, RpcClient } from "./rpc.js";
import { createTaskContext, type TaskContextBundle } from "./task-context.js";
import { loadTaskHandler } from "./task-loader.js";
import { processResult } from "./result.js";
import { gracefulShutdown } from "./shutdown.js";

export interface RuntimeConfig {
  generation: number;
  instanceId: string;
  extensionId: string;
  moduleId: string;
  nonce: string;
  hostApiVersion: string;
  definitionHash: string;
  taskRunId: string;
  taskEntry: string;
  entryHash: string;
  taskInput: TaskInput;
  taskDeadline: number;
  taskAttempt: number;
  taskMaxAttempts: number;
  taskCheckpoint: TaskCheckpoint | null;
  workspacePath: string;
}

interface HostWelcomeParams {
  session_id: string;
  session_token: string;
  limits: Record<string, unknown>;
  expires_at: string;
}

interface TaskExecuteParams {
  task_run_id: string;
  entry: string;
  input: TaskInput;
  checkpoint: TaskCheckpoint | null;
  deadline: number;
  attempt: number;
  max_attempts: number;
}

export function readRuntimeConfig(): RuntimeConfig {
  const generation = Number(process.env.AMITIA_GENERATION ?? "1");
  if (!Number.isSafeInteger(generation) || generation < 1) {
    throw new Error("任务运行代次无效");
  }
  const instanceId = process.env.AMITIA_INSTANCE_ID ?? "";
  const extensionId = process.env.AMITIA_EXTENSION_ID ?? "";
  const moduleId = process.env.AMITIA_MODULE_ID ?? "";
  const nonce = process.env.AMITIA_NONCE ?? "";
  const hostApiVersion = process.env.AMITIA_HOST_API_VERSION ?? "1.0.0";
  const definitionHash = process.env.AMITIA_DEFINITION_HASH ?? "";
  const taskRunId = process.env.AMITIA_TASK_RUN_ID ?? "";
  const taskEntry = process.env.AMITIA_TASK_ENTRY ?? "";
  const entryHash = process.env.AMITIA_ENTRY_HASH ?? "";
  const taskInputRaw = process.env.AMITIA_TASK_INPUT ?? "{}";
  const taskDeadlineStr = process.env.AMITIA_TASK_DEADLINE ?? "0";
  const taskAttemptStr = process.env.AMITIA_TASK_ATTEMPT ?? "1";
  const taskMaxAttemptsStr = process.env.AMITIA_TASK_MAX_ATTEMPTS ?? "1";
  const taskCheckpointRaw = process.env.AMITIA_TASK_CHECKPOINT ?? "";
  const workspacePath = process.env.AMITIA_WORKSPACE_PATH ?? "";

  let taskInput: TaskInput;
  try {
    taskInput = JSON.parse(taskInputRaw) as TaskInput;
  } catch {
    taskInput = {} as TaskInput;
  }

  const taskDeadline = parseInt(taskDeadlineStr, 10) || 0;
  const taskAttempt = parseInt(taskAttemptStr, 10) || 1;
  const taskMaxAttempts = parseInt(taskMaxAttemptsStr, 10) || 1;

  let taskCheckpoint: TaskCheckpoint | null = null;
  if (taskCheckpointRaw) {
    try {
      taskCheckpoint = JSON.parse(taskCheckpointRaw) as TaskCheckpoint;
    } catch {
      taskCheckpoint = null;
    }
  }

  return {
    generation,
    instanceId,
    extensionId,
    moduleId,
    nonce,
    hostApiVersion,
    definitionHash,
    taskRunId,
    taskEntry,
    entryHash,
    taskInput,
    taskDeadline,
    taskAttempt,
    taskMaxAttempts,
    taskCheckpoint,
    workspacePath,
  };
}

export async function bootstrap(): Promise<void> {
  const config = readRuntimeConfig();

  const transport = new StdioRpcTransport();
  transport.start();
  const rpc = new RpcClient(transport);

  const abortController = new AbortController();
  let shuttingDown = false;
  let pausing = false;
  let contextBundle: TaskContextBundle | null = null;

  const initiateShutdown = async (reason: string) => {
    if (shuttingDown) return;
    shuttingDown = true;
    if (!abortController.signal.aborted) {
      abortController.abort(reason);
    }
    await gracefulShutdown({
      rpc,
      taskRunId: config.taskRunId,
      reason,
      flushProgress: contextBundle ? () => contextBundle!.progress.flush() : undefined,
    });
    process.exit(0);
  };

  rpc.onNotification("runtime.shutdown", (params) => {
    const reason =
      (params as { reason?: string })?.reason ?? "shutdown";
    void initiateShutdown(reason);
  });

  rpc.onNotification("task.cancel", (params) => {
    const reason =
      (params as { reason?: string })?.reason ?? "cancelled";
    abortController.abort(reason);
  });

  rpc.onNotification("task.pause", (params) => {
    if (shuttingDown || pausing) return;
    if (!contextBundle) return;
    const p = params as { task_run_id?: string; timeout_ms?: number };
    const timeoutMs = typeof p.timeout_ms === "number" ? p.timeout_ms : 30000;
    pausing = true;
    void handlePause(rpc, contextBundle, p.task_run_id ?? config.taskRunId, timeoutMs, initiateShutdown).finally(() => { pausing = false; });
  });

  transport.onClose(() => {
    void initiateShutdown("stdin closed");
  });

  rpc.notify("runtime.hello", {
    protocol_version: "2.0",
    runtime_type: "task",
    instance_id: config.instanceId,
    generation: config.generation,
    definition_hash: config.definitionHash,
    nonce: config.nonce,
    features: ["checkpoint", "cancellation", "streaming", "cooperative_pause", "entry_pin", "checkpoint_ack"],
  });

  const welcomeRaw = await rpc.onceNotification("host.welcome", 30000);
  const welcome = welcomeRaw as HostWelcomeParams;

  rpc.notify("runtime.ready", {
    session_id: welcome.session_id,
  });

  const executeParams = await waitForTaskExecute(rpc, 120000);

  const taskRunId = executeParams.task_run_id || config.taskRunId;
  const entry = executeParams.entry || config.taskEntry;
  const input = (executeParams.input ?? config.taskInput) as TaskInput;
  const checkpoint = (executeParams.checkpoint ?? config.taskCheckpoint) as TaskCheckpoint | null;
  const deadline = executeParams.deadline || config.taskDeadline;
  const attempt = executeParams.attempt || config.taskAttempt;
  const maxAttempts = executeParams.max_attempts || config.taskMaxAttempts;

  let deadlineTimer: ReturnType<typeof setTimeout> | null = null;
  if (deadline > 0) {
    const remaining = deadline - Date.now();
    if (remaining <= 0) {
      abortController.abort("deadline exceeded");
    } else {
      deadlineTimer = setTimeout(() => {
        abortController.abort("deadline exceeded");
      }, remaining);
    }
  }

  contextBundle = createTaskContext({
    rpc,
    taskRunId,
    taskId: taskRunId,
    deadline: deadline > 0 ? deadline : undefined,
    attempt,
    maxAttempts,
    initialCheckpoint: checkpoint,
    signal: abortController.signal,
  });

  let result: TaskResult;

  try {
    const handler = await loadTaskHandler(entry, config.entryHash);
    result = await handler(input, contextBundle.context);
  } catch (error: unknown) {
    if (abortController.signal.aborted) {
      result = {
        success: false,
        error: {
          code: "cancelled",
          message: abortController.signal.reason || "task cancelled",
        },
      };
    } else {
      result = {
        success: false,
        error: {
          code: error instanceof Error ? error.name : "execution_error",
          message: error instanceof Error ? error.message : String(error),
        },
      };
    }
  }

  if (deadlineTimer !== null) {
    clearTimeout(deadlineTimer);
  }

  if (shuttingDown) return;

  contextBundle.progress.flush();

  const finishedPayload = await processResult(result, rpc, taskRunId);
  rpc.notify("task.finished", finishedPayload);

  await new Promise<void>((resolve) => setTimeout(resolve, 200));
  process.exit(result.success ? 0 : 1);
}

function waitForTaskExecute(rpc: RpcClient, timeoutMs: number): Promise<TaskExecuteParams> {
  return new Promise<TaskExecuteParams>((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error("timeout waiting for task.execute request"));
    }, timeoutMs);

    rpc.onRequest("task.execute", async (params) => {
      clearTimeout(timer);
      resolve(params as TaskExecuteParams);
      return { accepted: true };
    });
  });
}

async function handlePause(
  rpc: RpcClient,
  contextBundle: TaskContextBundle,
  taskRunId: string,
  timeoutMs: number,
  initiateShutdown: (reason: string) => Promise<void>,
): Promise<void> {
  const cpClient = contextBundle.checkpoint;
  if (!cpClient) {
    rpc.notify("task.pause_ack", {
      task_run_id: taskRunId,
      checkpoint_version: 0,
      checkpoint_hash: "",
      paused: false,
    });
    return;
  }

  let expired = false;
  const timer = setTimeout(() => {
    expired = true;
    rpc.notify("task.pause_ack", {
      task_run_id: taskRunId,
      checkpoint_version: 0,
      checkpoint_hash: "",
      paused: false,
    });
  }, timeoutMs);

  try {
    await cpClient.waitForConfirmation();
    const current = cpClient.getCurrent();
    if (current) {
      await cpClient.save({
        ...current,
        savedAt: new Date().toISOString(),
      });
    }

    clearTimeout(timer);
    if (expired) return;

    const finalCp = cpClient.getCurrent();
    const version = (finalCp?.cursor as number) ?? 0;

    rpc.notify("task.pause_ack", {
      task_run_id: taskRunId,
      checkpoint_version: version,
      checkpoint_hash: "",
      paused: true,
    });

    await initiateShutdown("paused");
  } catch {
    clearTimeout(timer);
    if (expired) return;
    rpc.notify("task.pause_ack", {
      task_run_id: taskRunId,
      checkpoint_version: 0,
      checkpoint_hash: "",
      paused: false,
    });
  }
}
