import { Envelope } from './protocol';
import { Client, MessageOption } from './client';
import type { PluginEvent } from './game';

export const METHOD_EVENT_PUBLISH = 'plugin.event.publish';
export const PLUGIN_AGENT_EVENT_ID = 'plugin.agent_event';

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

export async function publishAgentEvent(
  client: Client,
  event: PluginEvent,
  metadata: Record<string, unknown> = {},
  opts: MessageOption[] = []
): Promise<Envelope> {
  return publishEvent(client, {
    channelId: event.sessionId ?? '',
    eventId: PLUGIN_AGENT_EVENT_ID,
    payload: event,
    metadata,
  }, opts);
}
