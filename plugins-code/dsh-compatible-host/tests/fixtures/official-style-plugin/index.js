import { defineTool } from "@deepseek-ai/dsh-tools";

export const inject = ["amitiaSessionProjections"];

export async function apply(ctx) {
  await ctx.amitiaSessionProjections.register({
    key: "todo",
    init: () => null,
    apply: (state) => state,
    stateVersion: 1,
  });
  ctx.amitiaTools.register(defineTool({
    name: "todo_write",
    description: "Update the task list",
    parameters: {
      todos: {
        type: "array",
        required: true,
        items: { type: "string" },
      },
    },
    output: {
      schema: { type: "json" },
      render(args, value) {
        return { text: `count=${value.count}`, content: [] };
      },
    },
    execute(input) {
      return { count: input.todos.length };
    },
  }));
}
