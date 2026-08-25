import { Envelope } from './protocol';
import { HandlerRegistry } from './handler';
import { Client, MessageOption } from './client';

export const METHOD_CHANNEL_PUBLISH = 'channel.publish';
export const METHOD_CHANNEL_DELIVER = 'channel.deliver';

export interface ChannelPublishInput {
  channelId: string;
  payload: unknown;
  metadata?: Record<string, unknown>;
}

/**
 * Publishes from the plugin service to a channel declared in the plugin host
 * manifest and negotiated during hello. Host-to-plugin delivery is available on channels declared host_to_plugin or
 * bidirectional.
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

export interface ChannelDelivery {
  channelId: string;
  payload: unknown;
  metadata?: Record<string, unknown>;
}

export type ChannelDeliveryHandler = (delivery: ChannelDelivery, envelope: Envelope) => Promise<void> | void;

/** Registers the canonical GameHost host-to-plugin channel notification. */
export function registerChannelDeliveryHandler(registry: HandlerRegistry, handler: ChannelDeliveryHandler): void {
  registry.registerNotification(METHOD_CHANNEL_DELIVER, async (notification) => {
    const delivery = notification.payload as ChannelDelivery | undefined;
    if (!delivery || typeof delivery.channelId !== 'string' || !delivery.channelId) {
      throw new Error('channel delivery missing channelId');
    }
    await handler(delivery, notification);
  });
}
