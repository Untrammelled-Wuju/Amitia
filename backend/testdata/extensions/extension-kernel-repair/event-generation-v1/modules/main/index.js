export function onEvent(event) {
  return { received: true, generation: 1, eventId: event.id };
}
