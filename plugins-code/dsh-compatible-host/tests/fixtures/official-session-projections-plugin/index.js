export const name = "official-session-projections-plugin";
export const inject = ["sessionProjections"];

export function apply(ctx) {
  return ctx.sessionProjections.list();
}
