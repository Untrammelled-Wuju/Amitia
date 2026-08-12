import { Envelope } from './protocol';
import { Client, MessageOption } from './client';

export const METHOD_STATE_PUBLISH = 'plugin.state.publish';
export const METHOD_STATE_GET = 'plugin.state.get';

export interface StatePublishInput {
  stateId: string;
  payload: unknown;
  version: number;
  metadata?: Record<string, unknown>;
}

export interface StatePublishOutput {
  acked: boolean;
  stateId: string;
}

export interface StateGetInput {
  stateId: string;
}

export interface StateGetOutput {
  stateId: string;
  payload: unknown;
  version: number;
}

export async function publishState(
  client: Client,
  input: StatePublishInput,
  opts: MessageOption[] = []
): Promise<Envelope> {
  const payload: Record<string, unknown> = {
    stateId: input.stateId,
    payload: input.payload,
    version: input.version,
  };
  if (input.metadata) {
    payload.metadata = input.metadata;
  }
  return client.sendNotification(METHOD_STATE_PUBLISH, payload, ...opts);
}

export async function getState(
  client: Client,
  input: StateGetInput,
  opts: MessageOption[] = []
): Promise<StateGetOutput> {
  const envelope = await client.sendRequest(METHOD_STATE_GET, input, ...opts);
  return envelope.payload as StateGetOutput;
}
