export const name = "sample-dsh-plugin";
export const inject = ["fixtureService"];

let effectDisposed = 0;

export function apply(ctx) {
  ctx.amitiaTools.register({
    name: "echo",
    description: "Echo input",
    execute(input) {
      return {
        value: input.value,
        service: ctx.fixtureService.value,
      };
    },
  });
  ctx.effect(() => () => {
    effectDisposed += 1;
  });
  ctx.on("fixture/ping", (value) => {
    ctx.fixtureService.lastPing = value;
  });
}

export function fixtureState() {
  return { effectDisposed };
}
