import { Client, MessageOption } from './client';

export const METHOD_SERVICE_REGISTER = 'service.register';
export const METHOD_SERVICE_UNREGISTER = 'service.unregister';

export interface ServiceRegisterInput {
  serviceId: string;
  capabilities?: string[];
  metadata?: Record<string, unknown>;
}

export interface ServiceRegisterOutput {
  serviceId: string;
  token: string;
}

export interface ServiceUnregisterInput {
  serviceId: string;
}

export async function registerService(
  client: Client,
  input: ServiceRegisterInput,
  opts: MessageOption[] = []
): Promise<ServiceRegisterOutput> {
  const payload: Record<string, unknown> = {
    serviceId: input.serviceId,
  };
  if (input.capabilities) {
    payload.capabilities = input.capabilities;
  }
  if (input.metadata) {
    payload.metadata = input.metadata;
  }
  const envelope = await client.sendRequest(METHOD_SERVICE_REGISTER, payload, ...opts);
  return envelope.payload as ServiceRegisterOutput;
}

export async function unregisterService(
  client: Client,
  input: ServiceUnregisterInput,
  opts: MessageOption[] = []
): Promise<void> {
  await client.sendRequest(METHOD_SERVICE_UNREGISTER, input, ...opts);
}
