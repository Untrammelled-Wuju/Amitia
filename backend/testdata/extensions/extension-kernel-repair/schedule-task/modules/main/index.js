export function onTrigger(context) {
  return { triggered: true, target: "task", scheduleId: context.scheduleId };
}
