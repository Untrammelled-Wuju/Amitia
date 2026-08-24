import { Client, MessageOption } from './client';

export const METHOD_HOST_INVOKE = 'host.invoke';

export const HOST_API_STATUS_SUCCESS = 'success';

export interface HostAPIInvokeInput {
  method: string;
  version?: number;
  input: unknown;
  timeoutMs?: number;
}

export interface HostAPIInvokeResult {
  status: string;
  output: unknown;
  method: string;
  durationMs: number;
}

export async function invokeHostAPI(
  client: Client,
  input: HostAPIInvokeInput,
  opts: MessageOption[] = []
): Promise<HostAPIInvokeResult> {
  const envelope = await client.sendReservedRequest(METHOD_HOST_INVOKE, input, ...opts);
  return envelope.payload as HostAPIInvokeResult;
}
