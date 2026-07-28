export function onEvent(event) {
  return { received: true, generation: 2, eventId: event.id };
}
