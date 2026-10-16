export interface ScheduledRecurrence {
  type: "none" | "daily" | "weekly" | "monthly" | "weekdays" | "interval" | "cron";
  weekdays?: number[];
  intervalSeconds?: number;
  cron?: string;
}

export interface ScheduledTaskInput {
  title: string;
  content?: string;
  characterId: string;
  conversationId?: string;
  targetDeviceId?: string;
  remindAt: string;
  timezone: string;
  recurrence: ScheduledRecurrence;
  endAt?: string;
  maxOccurrences?: number;
  enabled?: boolean;
  systemNotification?: boolean;
  channelTargets?: { channelId: string; peerId: string }[];
  policy?: Record<string, unknown>;
  requestId: string;
}

export interface ScheduledTaskRecord extends Omit<ScheduledTaskInput,"enabled"> {
	enabled: number;
  id: string;
  ownerId: string;
  generation: number;
  state: string;
  executionMode: string;
  paused: boolean;
  saved: boolean;
}

export interface ScheduledTaskHistory {
  items: Record<string, unknown>[];
  total: number;
}

export interface ScheduledTaskTransport {
  call(method: string, input: unknown): Promise<unknown>;
}

export function scheduledTaskTransport(host: ScheduledTaskTransport): ScheduledTaskTransport {
  if (!host || typeof host.call !== "function") throw new Error("统一任务 Host API 传输未接通");
  return { call: (method, input) => host.call(method, input) };
}

export interface ScheduledTaskClient {
  create(input: ScheduledTaskInput): Promise<ScheduledTaskRecord>;
  list(): Promise<{ items: ScheduledTaskRecord[]; total: number }>;
  get(id: string): Promise<ScheduledTaskRecord>;
  update(id: string, generation: number, input: ScheduledTaskInput): Promise<ScheduledTaskRecord>;
  pause(id: string, generation: number): Promise<ScheduledTaskRecord>;
  resume(id: string, generation: number): Promise<ScheduledTaskRecord>;
  cancel(id: string, generation: number): Promise<ScheduledTaskRecord>;
  execute(id: string, generation: number): Promise<{ triggerId: string; status: string }>;
  history(options?: { id?: string; page?: number; pageSize?: number; state?: string }): Promise<ScheduledTaskHistory>;
  capabilities(): Promise<Record<string, unknown>>;
  status(): Promise<Record<string, unknown>>;
  queue(): Promise<Record<string, unknown>>;
  preview(id: string): Promise<Record<string, unknown>>;
  clearBackpressure(): Promise<Record<string, unknown>>;
}

function record(value: unknown): ScheduledTaskRecord {
  if (!value || typeof value !== "object") throw new Error("任务响应为空");
  const result = value as ScheduledTaskRecord;
  if (typeof result.id !== "string" || !result.id || typeof result.ownerId !== "string" || !result.ownerId || !Number.isInteger(result.generation) || result.generation < 1 || result.saved !== true) {
    throw new Error("任务保存未确认");
  }
  return result;
}

export function createScheduledTaskClient(transport: ScheduledTaskTransport, kind: "reminder" | "proactive_task" = "proactive_task"): ScheduledTaskClient {
  const bridge = scheduledTaskTransport(transport);
  const call = (method: string, input: unknown) => bridge.call(`host.${kind}.${method}`, input);
  const change = async (method: string, id: string, generation: number) => record(await call(method, { id, generation }));
  const object = async (method: string, input: unknown = {}) => {
    const response = await call(method, input);
    if (!response || typeof response !== 'object' || Array.isArray(response)) throw new Error('任务响应无效');
    return response as Record<string, unknown>;
  };
  return {
    status: () => object('status'),
    queue: () => object('queue'),
    preview: (id) => object('preview', { id }),
    clearBackpressure: () => object('clear'),
    create: async (input) => record(await call("create", input)),
    get: async (id) => record(await call("get", { id })),
    update: async (id, generation, input) => record(await call("update", { ...input, id, generation })),
    pause: (id, generation) => change("pause", id, generation),
    resume: (id, generation) => change("resume", id, generation),
    cancel: (id, generation) => change("cancel", id, generation),
    list: async () => {
      const response = await call("list", {}) as { items: unknown[]; total: number };
      if (!response || !Array.isArray(response.items) || !Number.isInteger(response.total) || response.total < response.items.length) throw new Error("任务列表响应无效");
      return { items: response.items.map(record), total: response.total };
    },
    execute: async (id, generation) => {
      const response = await call("execute", { id, generation }) as { triggerId: string; status: string };
      if (typeof response?.triggerId !== "string" || !response.triggerId || typeof response.status !== "string" || !response.status) throw new Error("任务执行请求未确认");
      return response;
    },
    history: async (options = {}) => {
      const response = await call("history", options) as ScheduledTaskHistory;
      if (!response || !Array.isArray(response.items) || !Number.isInteger(response.total) || response.total < response.items.length) throw new Error("任务历史响应无效");
      return response;
    },
    capabilities: async () => {
      const response = await call("capabilities", {});
      if (!response || typeof response !== "object" || Array.isArray(response)) throw new Error("任务能力响应无效");
      return response as Record<string, unknown>;
    },
  };
}
