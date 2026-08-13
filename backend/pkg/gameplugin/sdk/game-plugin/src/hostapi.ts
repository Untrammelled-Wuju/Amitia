import { Client, MessageOption } from './client';

export const METHOD_HOST_API_INVOKE = 'host_api.invoke';
export const METHOD_HOST_API_QUERY_CAPS = 'host_api.query_capabilities';
export const METHOD_HOST_API_RATE_LIMIT = 'host_api.rate_limit_status';

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
  const envelope = await client.sendRequest(METHOD_HOST_API_INVOKE, input, ...opts);
  return envelope.payload as HostAPIInvokeResult;
}

export async function queryHostAPICapabilities(
  client: Client,
  input: HostAPIQueryCapsInput,
  opts: MessageOption[] = []
): Promise<HostAPIQueryCapsResult> {
  const envelope = await client.sendRequest(METHOD_HOST_API_QUERY_CAPS, input, ...opts);
  return envelope.payload as HostAPIQueryCapsResult;
}

export async function queryHostAPIRateLimit(
  client: Client,
  method: string,
  opts: MessageOption[] = []
): Promise<HostAPIRateLimitStatusResult> {
  const envelope = await client.sendRequest(METHOD_HOST_API_RATE_LIMIT, { method }, ...opts);
  return envelope.payload as HostAPIRateLimitStatusResult;
}
