
export function apply(ctx) {
  ctx.amitiaTools.register({
    name: "agent_echo",
    execute(input, execution) {
      if (!execution.agent) throw new Error("agent context is required");
      execution.agent.session.append("agent/echo", input, { ignorable: true });
      return {
        eventCount: execution.agent.session.events.length,
        inboxCount: execution.agent.inbox.nextTurn.length,
      };
    },
  });
}
