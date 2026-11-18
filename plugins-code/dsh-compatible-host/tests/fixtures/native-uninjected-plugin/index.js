export const name = "native-uninjected-plugin";

export function apply(ctx) {
  return ctx.undeclaredService;
}
