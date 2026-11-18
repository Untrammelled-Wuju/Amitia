export const ROOT_AGENT_NAME = "lead";

export function assertSubagentMaxDepth(value) {
  if (value === undefined || value === "provider-managed") return;
  if (!Number.isSafeInteger(value) || value < 0) throw new Error("subagent maxDepth must be a non-negative safe integer");
}

export function delegationDepthOf(agent) {
  return Number(agent?.delegationDepth) || 0;
}

export function parentAgentOptionsForDelegation(agent) {
  return agent?.options ?? {};
}
