export function apply(ctx) {
  ctx.amitiaTools.register({
    name: "search",
    execute(input) {
      return { entry: "root", value: input.value };
    },
  });
}
