export const name = "native-missing-plugin";
export const inject = ["nativeMissingService"];

export const applied = [];

export function apply(ctx) {
  applied.push(ctx.nativeMissingService);
}

export function appliedCount() {
  return applied.length;
}
