export const name = "amitia-llm-plugin";
export const inject = ["amitiaLlm"];

export let pending;

export function apply(ctx) {
  pending = ctx.amitiaLlm.invoke({ provider: "deepseek", model: "deepseek-chat" });
}
