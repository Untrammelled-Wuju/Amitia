import { apiClient } from "@/composables/useApi"
import type { MCPAuditLog, MCPCapabilityName, MCPPendingInteraction, MCPPrompt, MCPResource, MCPResourceTemplate, MCPServer, MCPServerCapability, MCPServerForm, MCPTask, MCPTool } from "./types"

function data<T>(response: any): T {
  return (response.data?.data ?? response.data) as T
}

function payload(form: MCPServerForm) {
  let credential: unknown = undefined
  if (form.credentialText.trim()) {
    credential = form.authType === "bearer_token" ? form.credentialText.trim() : JSON.parse(form.credentialText)
  }
  return {
    name: form.name.trim(),
    displayName: form.displayName.trim(),
    description: form.description.trim(),
    transport: form.transport,
    endpoint: form.transport === "streamable_http" ? form.endpoint.trim() : "",
    command: form.transport === "stdio" ? form.command.trim() : "",
    args: form.transport === "stdio" ? form.argsText.split("\n").map(item => item.trim()).filter(Boolean) : [],
    workDir: form.transport === "stdio" ? form.workDir.trim() : "",
    authType: form.authType,
    enabled: form.enabled,
    source: "manual",
    privateNetworkConfirmed: form.privateNetworkConfirmed,
    ...(credential === undefined ? {} : { credential }),
  }
}

export async function listMCPServers() { return data<MCPServer[]>(await apiClient.get("/api/mcp/servers")) }
export async function createMCPServer(form: MCPServerForm) { return data<MCPServer>(await apiClient.post("/api/mcp/servers", payload(form))) }
export async function updateMCPServer(id: string, form: MCPServerForm) { return data<MCPServer>(await apiClient.put(`/api/mcp/servers/${encodeURIComponent(id)}`, payload(form))) }
export async function deleteMCPServer(id: string) { await apiClient.delete(`/api/mcp/servers/${encodeURIComponent(id)}`) }
export async function connectMCPServer(id: string) { await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/connect`) }
export async function disconnectMCPServer(id: string) { await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/disconnect`) }
export async function reconnectMCPServer(id: string) { await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/reconnect`) }
export async function refreshMCPServer(id: string) { await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/refresh`) }
export async function listMCPTools(id: string) { return data<MCPTool[]>(await apiClient.get(`/api/mcp/servers/${encodeURIComponent(id)}/tools`)) }
export async function listMCPResources(id: string) { return data<{ resources: MCPResource[]; resourceTemplates: MCPResourceTemplate[] }>(await apiClient.get(`/api/mcp/servers/${encodeURIComponent(id)}/resources`)) }
export async function readMCPResource(id: string, uri: string, characterId = "") { return data<any>(await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/resources/read`, { uri, characterId })) }
export async function subscribeMCPResource(id: string, uri: string, subscribed: boolean, characterId = "") { await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/resources/${subscribed ? "subscribe" : "unsubscribe"}`, { uri, characterId }) }
export async function listMCPPrompts(id: string) { return data<MCPPrompt[]>(await apiClient.get(`/api/mcp/servers/${encodeURIComponent(id)}/prompts`)) }
export async function getMCPPrompt(id: string, name: string, argumentsValue: Record<string, string>, characterId = "") { return data<any>(await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/prompts/get`, { name, arguments: argumentsValue, characterId })) }
export async function completeMCPArgument(id: string, promptName: string, argumentName: string, value: string, contextArguments: Record<string, string>, characterId = "") { return data<{ values: string[]; total?: number; hasMore?: boolean }>(await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/completion`, { characterId, ref: { type: "ref/prompt", name: promptName }, argument: { name: argumentName, value }, contextArguments })) }
export async function listMCPLogs(id: string) { return data<MCPAuditLog[]>(await apiClient.get(`/api/mcp/servers/${encodeURIComponent(id)}/logs`)) }
export async function listMCPTasks(id: string) { return data<MCPTask[]>(await apiClient.get(`/api/mcp/servers/${encodeURIComponent(id)}/tasks`)) }
export async function cancelMCPTask(id: string, taskId: string) { await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/tasks/${encodeURIComponent(taskId)}/cancel`) }
export async function setMCPToolEnabled(serverId: string, toolId: string, enabled: boolean) { await apiClient.put(`/api/mcp/servers/${encodeURIComponent(serverId)}/tools/${encodeURIComponent(toolId)}/scope`, { characterId: "", enabled }) }
export async function listMCPCapabilities(id: string) { return data<MCPServerCapability[]>(await apiClient.get(`/api/mcp/servers/${encodeURIComponent(id)}/capabilities`)) }
export async function setMCPCapability(id: string, capability: MCPCapabilityName, enabled: boolean, configuration: Record<string, unknown> = {}) { return data<MCPServerCapability>(await apiClient.put(`/api/mcp/servers/${encodeURIComponent(id)}/capabilities/${capability}`, { enabled, configuration })) }
export async function startMCPOAuth(id: string, resourceUrl: string) {
  const redirectUri = `${window.location.origin}/api/mcp/oauth/callback`
  return data<{ authorizationUrl: string; sessionId: string }>(await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/oauth/start`, { resourceUrl, redirectUri, scopes: [] }))
}
export async function revokeMCPOAuth(id: string) { await apiClient.post(`/api/mcp/servers/${encodeURIComponent(id)}/oauth/revoke`) }
export async function listMCPInteractions() { return data<MCPPendingInteraction[]>(await apiClient.get("/api/mcp/interactions")) }
export async function resolveMCPInteraction(id: string, action: "accept" | "decline" | "cancel", content: Record<string, unknown> = {}) { await apiClient.post(`/api/mcp/interactions/${encodeURIComponent(id)}/resolve`, { action, content }) }
