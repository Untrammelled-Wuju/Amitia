import { Envelope } from './protocol';
import { Client, MessageOption } from './client';

export const METHOD_CHANNEL_PUBLISH = 'channel.publish';
export const METHOD_CHANNEL_SUBSCRIBE = 'channel.subscribe';
export const METHOD_CHANNEL_UNSUBSCRIBE = 'channel.unsubscribe';

export interface ChannelPublishInput {
  channelId: string;
  payload: unknown;
  metadata?: Record<string, unknown>;
}

export interface ChannelSubscribeInput {
  channelId: string;
  cursor?: string;
}

export interface ChannelSubscribeOutput {
  cursor: string;
}

export interface ChannelUnsubscribeInput {
  channelId: string;
}

export async function channelPublish(
  client: Client,
  input: ChannelPublishInput,
  opts: MessageOption[] = []
): Promise<Envelope> {
  const payload: Record<string, unknown> = {
    channelId: input.channelId,
    payload: input.payload,
  };
  if (input.metadata) {
    payload.metadata = input.metadata;
  }
  return client.sendNotification(METHOD_CHANNEL_PUBLISH, payload, ...opts);
}

export async function channelSubscribe(
  client: Client,
  input: ChannelSubscribeInput,
  opts: MessageOption[] = []
): Promise<ChannelSubscribeOutput> {
  const envelope = await client.sendRequest(METHOD_CHANNEL_SUBSCRIBE, input, ...opts);
  return envelope.payload as ChannelSubscribeOutput;
}

export async function channelUnsubscribe(
  client: Client,
  input: ChannelUnsubscribeInput,
  opts: MessageOption[] = []
): Promise<void> {
  await client.sendRequest(METHOD_CHANNEL_UNSUBSCRIBE, input, ...opts);
}
