import { Envelope } from './protocol';
import { Client, MessageOption } from './client';

export const METHOD_CHANNEL_PUBLISH = 'channel.publish';

export interface ChannelPublishInput {
  channelId: string;
  payload: unknown;
  metadata?: Record<string, unknown>;
}

/**
 * Publishes from the plugin service to a channel declared in the plugin host
 * manifest and negotiated during hello. Host-to-plugin subscriptions are not
 * part of host protocol v1.
 */
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
  return client.sendReservedNotification(METHOD_CHANNEL_PUBLISH, payload, ...opts);
}
