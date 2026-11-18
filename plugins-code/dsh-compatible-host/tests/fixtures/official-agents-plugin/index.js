export const name = "official-agents-plugin";
export const inject = ["agents"];

export function apply(ctx) {
  ctx.amitiaTools.register({
    name: "official_agents_probe",
    description: "Report the official agents registry surface",
    execute() {
      return { setFactory: typeof ctx.agents.setFactory };
    },
  });
}
