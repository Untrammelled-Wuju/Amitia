export const name = "official-agent-create-plugin";
export const inject = ["agents", "spillStore"];

export function apply(ctx) {
  const handles = new Map();
  ctx.amitiaTools.register({
    name: "official_agent_create_probe",
    description: "Create an agent through the official agents registry",
    async execute(input) {
      if (handles.has(input.sessionId)) throw new Error(`Agent ${input.sessionId} is already owned by this plugin`);
      const handle = await ctx.agents.create({
        sessionId: input.sessionId,
        meta: input.cwd === undefined ? {} : { cwd: input.cwd },
      });
      handles.set(input.sessionId, handle);
      return {
        agentId: handle.agent.id,
        live: ctx.agents.list().map((agent) => agent.id),
      };
    },
  });
  ctx.amitiaTools.register({
    name: "official_agent_dispose_probe",
    description: "Dispose an agent created through the official agents registry",
    async execute(input) {
      const handle = handles.get(input.sessionId);
      if (!handle) throw new Error(`Agent ${input.sessionId} is not owned by this plugin`);
      await handle.dispose();
      handles.delete(input.sessionId);
      return {
        agentId: input.sessionId,
        disposed: true,
        live: ctx.agents.list().map((agent) => agent.id),
      };
    },
  });
  ctx.amitiaTools.register({
    name: "official_spill_save_probe",
    description: "Persist text through the official spill store",
    async execute(input) {
      const saved = await ctx.spillStore.saveText({
        owner: { sessionId: input.sessionId },
        suggestedName: input.suggestedName,
        content: input.text,
      });
      return {
        locator: String(saved.locator),
        bytes: saved.bytes,
        retrievalHint: saved.retrievalHint,
      };
    },
  });
}
