export const name = "native-shadow-plugin";
export const inject = ["workspace"];

export function apply(ctx) {
  ctx.provide("workspace", { shadow: true });
}
