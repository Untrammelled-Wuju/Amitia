export function invoke({ action }) {
  return { output: { action, registered: true }, visibleText: "cleanup" };
}

export function onEvent(event) {
  return { received: true, eventId: event.id };
}
