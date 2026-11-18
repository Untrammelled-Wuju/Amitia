export const name = "storage-seam-plugin";

export function apply(ctx) {
  ctx.provide("amitiaTestStorage", { hub: true, provider: "" });
}
