export const name = "storage-consumer-plugin";
export const inject = ["amitiaTestStorage"];

export function apply(ctx) {
  globalThis.__dshStorageConsumerProbe = {
    provider: ctx.amitiaTestStorage?.provider ?? "",
  };
}
