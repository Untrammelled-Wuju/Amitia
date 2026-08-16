import { Envelope } from './protocol';
import { Client, MessageOption } from './client';

export const METHOD_EVENT_PUBLISH = 'plugin.event.publish';

export interface EventPublishInput {
  channelId: string;
  eventId: string;
  payload: unknown;
  metadata?: Record<string, unknown>;
}

export async function publishEvent(
  client: Client,
  input: EventPublishInput,
  opts: MessageOption[] = []
): Promise<Envelope> {
  const payload: Record<string, unknown> = {
    channelId: input.channelId,
    eventId: input.eventId,
    payload: input.payload,
  };
  if (input.metadata) {
    payload.metadata = input.metadata;
  }
  return client.sendReservedNotification(METHOD_EVENT_PUBLISH, payload, ...opts);
}
