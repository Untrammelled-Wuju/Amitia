export function onTrigger(context) {
  return { triggered: true, target: "tool", scheduleId: context.scheduleId };
}
