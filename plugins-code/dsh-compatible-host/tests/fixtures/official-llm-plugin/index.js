export const name = "official-llm-plugin";
export const inject = ["llm"];

export function apply(ctx) {
  return ctx.llm.listProviders();
}
