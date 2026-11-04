import { apiClient } from "@/composables/useApi";

export interface AgentRun {
  id: string;
  spaceId: string;
  conversationId: string;
  parentRunId: string;
  coordinationId: string;
  assignmentId: string;
  parentInteractionId: string;
  childInteractionId: string;
  characterId: string;
  workerRole: string;
  objective: string;
  status: string;
  depth: number;
  createdAt: string;
  updatedAt: string;
}

export interface AgentRunMessage {
  id: string;
  spaceId: string;
  conversationId: string;
  senderRunId: string;
  recipientRunId: string;
  kind: string;
  content: string;
  status: string;
  correlationId: string;
  createdAt: string;
  readAt?: string | null;
}

function unwrap<T>(response: { data?: { data?: T } & T }): T {
  return response.data?.data ?? response.data as T;
}

export async function listAgentRuns(conversationId: string): Promise<AgentRun[]> {
  const response = await apiClient.get("/api/roles/runs", { params: { conversationId } });
  const data = unwrap<{ runs?: AgentRun[] }>(response);
  return Array.isArray(data?.runs) ? data.runs : [];
}

export async function listAgentRunMessages(runId: string): Promise<AgentRunMessage[]> {
  const response = await apiClient.get(`/api/roles/runs/${encodeURIComponent(runId)}/messages`);
  const data = unwrap<{ messages?: AgentRunMessage[] }>(response);
  return Array.isArray(data?.messages) ? data.messages : [];
}

export async function sendAgentRunMessage(runId: string, content: string): Promise<AgentRunMessage> {
  const response = await apiClient.post(`/api/roles/runs/${encodeURIComponent(runId)}/messages`, { content });
  return unwrap<AgentRunMessage>(response);
}
