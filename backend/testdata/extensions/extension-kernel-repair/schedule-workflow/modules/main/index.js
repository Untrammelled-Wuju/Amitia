export function onTrigger(context) {
  return { triggered: true, target: "workflow", scheduleId: context.scheduleId };
}
