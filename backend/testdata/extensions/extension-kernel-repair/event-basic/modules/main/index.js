export function onEvent(event) {
  return { received: true, eventId: event.id, eventType: event.type };
}
