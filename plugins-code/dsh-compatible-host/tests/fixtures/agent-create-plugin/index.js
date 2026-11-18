export const inject = ["amitiaAgents"];

export function apply(ctx) {
  const handles = new Map();
  ctx.amitiaTools.register({
    name: "agent_create_probe",
    async execute(input) {
      if (handles.has(input.sessionId)) throw new Error(`Agent ${input.sessionId} is already owned`);
      const handle = await ctx.amitiaAgents.create({ sessionId: input.sessionId });
      handles.set(input.sessionId, handle);
      return {
        agentId: handle.agent.id,
        live: ctx.amitiaAgents.list().some((agent) => agent.id === input.sessionId),
      };
    },
  });
  ctx.amitiaTools.register({
    name: "agent_dispose_probe",
    async execute(input) {
      const handle = handles.get(input.sessionId);
      if (!handle) throw new Error(`Agent ${input.sessionId} is not owned by this plugin`);
      await handle.dispose();
      handles.delete(input.sessionId);
      return {
        agentId: input.sessionId,
        disposed: true,
        live: ctx.amitiaAgents.list().some((agent) => agent.id === input.sessionId),
      };
    },
  });
  ctx.amitiaTools.register({
    name: "agent_resume_probe",
    async execute(input) {
      const handle = await ctx.amitiaAgents.resume({ resumeSessionId: input.sessionId });
      handles.set(input.sessionId, handle);
      return {
        agentId: handle.agent.id,
        live: ctx.amitiaAgents.list().some((agent) => agent.id === input.sessionId),
      };
    },
  });
  ctx.amitiaTools.register({
    name: "agent_inject_probe",
    async execute(input) {
      const handle = handles.get(input.sessionId);
      if (!handle) throw new Error(`Agent ${input.sessionId} is not owned by this plugin`);
      await handle.agent.inject({
        content: [{ type: "text", text: input.text }],
      });
      return {
        agentId: input.sessionId,
        injected: true,
      };
    },
  });
}
