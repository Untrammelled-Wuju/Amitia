export const name = "simple-tool-plugin";

export function apply(ctx) {
  const systemPrompt = ctx.get("amitiaSystemPrompt");
  systemPrompt?.section({
    name: "simple-tool-prompt",
    order: 50,
    text: "Use the simple tool carefully.",
  });
  systemPrompt?.context({
    name: "simple-tool-context",
    order: 100,
    text: "Simple tool context.",
  });
  ctx.amitiaTools.register({
    name: "double",
    description: "Double a number",
    execute(input) {
      return { value: Number(input.value) * 2 };
    },
  });
}
