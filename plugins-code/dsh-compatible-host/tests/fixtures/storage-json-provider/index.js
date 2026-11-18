export const name = "storage-json-provider";
export const inject = ["amitiaTestStorage"];

export function apply(ctx, config) {
  globalThis.__dshStorageJsonProbe = { root: config?.root ?? "", sealed: true };
  ctx.amitiaTestStorage.provider = "json";
}
