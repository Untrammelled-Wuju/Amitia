import { Client, MessageOption } from './client';

export const METHOD_HOST_INVOKE = 'host.invoke';

export const HOST_API_SIDE_EFFECT_READ = 'read';
export const HOST_API_SIDE_EFFECT_WRITE = 'write';
export const HOST_API_SIDE_EFFECT_EXECUTE = 'execute';
export const HOST_API_SIDE_EFFECT_NOTIFY = 'notify';

export const HOST_API_STATUS_SUCCESS = 'success';
export const HOST_API_STATUS_FAILED = 'failed';

export interface HostAPIInvokeInput {
  method: string;
  version?: number;
  input: unknown;
  serviceId?: string;
  sideEffect?: string;
  requestId?: string;
  timeoutMs?: number;
}

export interface HostAPIError {
  code: string;
  message: string;
  detail?: string;
}

export interface HostAPIInvokeResult {
  status: string;
  output?: unknown;
  method?: string;
  durationMs?: number;
  error?: HostAPIError;
}

export interface HostAPIQueryCapsInput {
  method: string;
}

export interface HostAPIQueryCapsResult {
  method: string;
  registered: boolean;
  permissionId?: string;
  rateLimitMax?: number;
  timeoutMs?: number;
  sideEffect?: string;
  versions?: number[];
}

export interface HostAPIRateLimitStatusResult {
  limit: number;
  remaining: number;
  resetAt: number;
}

export async function invokeHostAPI(
  client: Client,
  input: HostAPIInvokeInput,
  opts: MessageOption[] = []
): Promise<HostAPIInvokeResult> {
  const envelope = await client.sendReservedRequest(METHOD_HOST_INVOKE, input, ...opts);
  return envelope.payload as HostAPIInvokeResult;
}

export async function queryHostAPICapabilities(
  client: Client,
  input: HostAPIQueryCapsInput,
  opts: MessageOption[] = []
): Promise<HostAPIQueryCapsResult> {
  const envelope = await client.sendReservedRequest(METHOD_HOST_INVOKE, {
    method: 'host_api.query_capabilities',
    input: { method: input.method },
  }, ...opts);
  return envelope.payload as HostAPIQueryCapsResult;
}

export async function queryHostAPIRateLimit(
  client: Client,
  method: string,
  opts: MessageOption[] = []
): Promise<HostAPIRateLimitStatusResult> {
  const envelope = await client.sendReservedRequest(METHOD_HOST_INVOKE, {
    method: 'host_api.rate_limit_status',
    input: { method },
  }, ...opts);
  return envelope.payload as HostAPIRateLimitStatusResult;
}
