import { AmitiaError, ValidationError, TimeoutError, CancelledError } from "./errors";

export interface TaskInput {
  readonly [key: string]: unknown;
}

export interface TaskResult<O = unknown> {
  readonly success: boolean;
  readonly output?: O;
  readonly error?: { code: string; message: string };
  readonly artifacts?: TaskArtifact[];
  readonly metadata?: Record<string, unknown>;
}

export interface TaskArtifact {
  readonly artifactId: string;
  readonly kind: "file" | "image" | "audio" | "data" | "report";
  readonly mimeType?: string;
  readonly size?: number;
  readonly handle: string;
  readonly metadata?: Record<string, unknown>;
}

export interface TaskProgressReport {
  readonly current?: number;
  readonly total?: number;
  readonly unit?: string;
  readonly message?: string;
  readonly stage?: string;
}

export interface TaskCheckpoint {
  readonly cursor?: number;
  readonly cursorKey?: string;
  readonly data?: unknown;
  readonly savedAt: string;
}

export interface TaskContext {
  readonly taskId: string;
  readonly traceId: string;
  readonly deadline?: number;
  readonly signal: AbortSignal;
  readonly logger: TaskLogger;
  readonly progress: TaskProgressClient;
  readonly checkpoint: TaskCheckpointClient;
  readonly artifacts: TaskArtifactClient;
  readonly storage: TaskStorageClient;
  readonly host: TaskHostClient;
  readonly attempt: number;
  readonly maxAttempts: number;
}

export interface TaskLogger {
  debug(message: string, fields?: Record<string, unknown>): void;
  info(message: string, fields?: Record<string, unknown>): void;
  warn(message: string, fields?: Record<string, unknown>): void;
  error(message: string, fields?: Record<string, unknown>): void;
}

export interface TaskProgressClient {
  report(progress: TaskProgressReport): Promise<void>;
}

export interface TaskCheckpointClient {
  save(checkpoint: TaskCheckpoint): Promise<void>;
  load(): Promise<TaskCheckpoint | null>;
}

export interface TaskArtifactClient {
  saveFile(name: string, content: Uint8Array, options?: TaskArtifactOptions): Promise<TaskArtifact>;
  saveData(name: string, data: unknown, options?: TaskArtifactOptions): Promise<TaskArtifact>;
  list(): Promise<TaskArtifact[]>;
}

export interface TaskArtifactOptions {
  readonly kind?: TaskArtifact["kind"];
  readonly mimeType?: string;
  readonly metadata?: Record<string, unknown>;
}

export interface TaskStorageClient {
  get<T>(key: string): Promise<T | null>;
  set<T>(key: string, value: T): Promise<void>;
  delete(key: string): Promise<void>;
}

export interface TaskHostClient {
  executeTool(toolId: string, input: unknown, timeoutMs?: number): Promise<unknown>;
  emitEvent(type: string, payload: unknown): Promise<void>;
}

export type TaskHandler<I extends TaskInput = TaskInput, O = unknown> = (
  input: I,
  context: TaskContext,
) => Promise<TaskResult<O>> | TaskResult<O>;

export interface TaskDefinition<I extends TaskInput = TaskInput, O = unknown> {
  readonly taskId: string;
  readonly handler: TaskHandler<I, O>;
  readonly timeoutMs?: number;
  readonly maxAttempts?: number;
  readonly idempotent?: boolean;
  readonly inputSchema?: unknown;
  readonly outputSchema?: unknown;
}

const taskRegistry = new Map<string, TaskDefinition>();

export function defineTask<I extends TaskInput = TaskInput, O = unknown>(
  definition: TaskDefinition<I, O>,
): TaskDefinition<I, O> {
  if (!definition.taskId) {
    throw new ValidationError("taskId is required");
  }
  if (typeof definition.handler !== "function") {
    throw new ValidationError("task handler must be a function");
  }
  if (taskRegistry.has(definition.taskId)) {
    throw new ValidationError(`task ${definition.taskId} already defined`);
  }
  taskRegistry.set(definition.taskId, definition as unknown as TaskDefinition);
  return definition;
}

export function getTask(taskId: string): TaskDefinition | undefined {
  return taskRegistry.get(taskId);
}

export function listTasks(): TaskDefinition[] {
  return Array.from(taskRegistry.values());
}

export function clearTasks(): void {
  taskRegistry.clear();
}

export function successTaskResult<O>(output: O, options?: { artifacts?: TaskArtifact[]; metadata?: Record<string, unknown> }): TaskResult<O> {
  return {
    success: true,
    output,
    artifacts: options?.artifacts,
    metadata: options?.metadata,
  };
}

export function failureTaskResult(code: string, message: string): TaskResult<never> {
  return {
    success: false,
    error: { code, message },
  };
}

export function assertTaskSignal(signal: AbortSignal): void {
  if (signal.aborted) {
    throw new CancelledError("task aborted before start");
  }
}

export function assertTaskDeadline(deadline: number | undefined, traceId: string): void {
  if (deadline === undefined) return;
  if (Date.now() >= deadline) {
    throw new TimeoutError(`task ${traceId} deadline exceeded before start`);
  }
}

export function mapTaskError(cause: unknown): AmitiaError {
  if (cause instanceof AmitiaError) return cause;
  const message = cause instanceof Error ? cause.message : String(cause);
  if (/cancel|abort/i.test(message)) return new CancelledError(message);
  if (/timeout|deadline/i.test(message)) return new TimeoutError(message);
  return new ValidationError(message);
}

export interface TaskExecutionOptions {
  readonly traceId?: string;
  readonly timeoutMs?: number;
  readonly deadline?: number;
  readonly attempt?: number;
  readonly maxAttempts?: number;
  readonly signal?: AbortSignal;
  readonly logger?: TaskLogger;
  readonly progress?: TaskProgressClient;
  readonly checkpoint?: TaskCheckpointClient;
  readonly artifacts?: TaskArtifactClient;
  readonly storage?: TaskStorageClient;
  readonly host?: TaskHostClient;
}

export async function runTask<I extends TaskInput, O>(
  definition: TaskDefinition<I, O>,
  input: I,
  options: TaskExecutionOptions,
): Promise<TaskResult<O>> {
  const traceId = options.traceId ?? `task-${Math.random().toString(36).slice(2)}`;
  const signal = options.signal ?? new AbortController().signal;
  const logger: TaskLogger = options.logger ?? noopTaskLogger;
  const context: TaskContext = {
    taskId: definition.taskId,
    traceId,
    deadline: options.deadline,
    signal,
    logger,
    progress: options.progress ?? noopProgress,
    checkpoint: options.checkpoint ?? noopCheckpoint,
    artifacts: options.artifacts ?? noopArtifacts,
    storage: options.storage ?? noopStorage,
    host: options.host ?? noopHost,
    attempt: options.attempt ?? 1,
    maxAttempts: options.maxAttempts ?? definition.maxAttempts ?? 1,
  };
  assertTaskSignal(signal);
  assertTaskDeadline(options.deadline, traceId);
  try {
    return await definition.handler(input, context);
  } catch (cause) {
    const err = mapTaskError(cause);
    return failureTaskResult(err.code, err.message) as TaskResult<O>;
  }
}

const noopTaskLogger: TaskLogger = {
  debug: () => {},
  info: () => {},
  warn: () => {},
  error: () => {},
};

const noopProgress: TaskProgressClient = {
  report: async () => {},
};

const noopCheckpoint: TaskCheckpointClient = {
  save: async () => {},
  load: async () => null,
};

const noopArtifacts: TaskArtifactClient = {
  saveFile: async () => { throw new ValidationError("artifact client not configured"); },
  saveData: async () => { throw new ValidationError("artifact client not configured"); },
  list: async () => [],
};

const noopStorage: TaskStorageClient = {
  get: async () => null,
  set: async () => {},
  delete: async () => {},
};

const noopHost: TaskHostClient = {
  executeTool: async () => { throw new ValidationError("host client not configured"); },
  emitEvent: async () => {},
};
