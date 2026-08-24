import { Envelope } from './protocol';
import { Client, MessageOption } from './client';
import type { PluginEvent } from './game';

export const METHOD_AGENT_EVENT_PUBLISH = 'plugin.event.publish';
export const PLUGIN_AGENT_EVENT_ID = 'plugin.agent_event';

/**
 * Publishes a plugin-defined event as an Agent wake-up hint. This reserved
 * method is not a generic event bus. Generic plugin event/state traffic must
 * use channelPublish on a channel declared in the GameHost manifest.
 */
export async function publishAgentEvent(
  client: Client,
  event: PluginEvent,
  metadata: Record<string, unknown> = {},
  opts: MessageOption[] = []
): Promise<Envelope> {
  return client.sendReservedNotification(METHOD_AGENT_EVENT_PUBLISH, {
    eventId: PLUGIN_AGENT_EVENT_ID,
    payload: event,
    metadata,
  }, ...opts);
}
