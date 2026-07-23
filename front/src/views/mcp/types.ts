export type MCPTransport = "streamable_http" | "stdio";
export type MCPAuthType =
  | "none"
  | "oauth"
  | "bearer_token"
  | "custom_headers"
  | "stdio_env";

export interface MCPServer {
  id: string;
  name: string;
  displayName: string;
  description: string;
  transport: MCPTransport;
  endpoint: string;
  command: string;
  args: string;
  workDir: string;
  protocolVersion: string;
  serverInfo: string;
  capabilities: string;
  instructions: string;
  authType: MCPAuthType;
  enabled: number;
  status: string;
  source: string;
  lastConnectedAt: string;
  lastErrorCode: string;
  lastErrorMessage: string;
  createdAt: string;
  updatedAt: string;
  privateNetworkConfirmed: boolean;
}

export interface MCPTool {
  id: string;
  serverId: string;
  remoteName: string;
  skillId: string;
  title: string;
  description: string;
  inputSchema: string;
  outputSchema: string;
  annotations: string;
  capabilityHints: string;
  riskLevel: string;
  enabled: number;
  hash: string;
}

export interface MCPResource {
  id: string;
  uri: string;
  name: string;
  title: string;
  description: string;
  mimeType: string;
  enabled: number;
}
export interface MCPResourceTemplate {
  id: string;
  uriTemplate: string;
  name: string;
  title: string;
  description: string;
  mimeType: string;
  enabled: number;
}
export interface MCPPrompt {
  id: string;
  remoteName: string;
  title: string;
  description: string;
  arguments: string;
  enabled: number;
}
export interface MCPAuditLog {
  id: string;
  operation: string;
  toolName: string;
  status: string;
  durationMs: number;
  errorCode: string;
  summary: string;
  createdAt: string;
}
export interface MCPTask {
  id: string;
  serverId: string;
  remoteTaskId: string;
  status: string;
  statusMessage: string;
  result: string;
  expiresAt: string;
  lastUpdatedAt: string;
}
export type MCPCapabilityName = "roots" | "sampling" | "elicitation" | "tasks";
export interface MCPServerCapability {
  id?: string;
  serverId: string;
  capability: MCPCapabilityName;
  configuration: string;
  enabled: number;
  createdAt?: string;
  updatedAt?: string;
}
export interface MCPSamplingConfiguration {
  maxTokens: number;
  timeoutSeconds: number;
  maxConcurrent: number;
  toolsEnabled: boolean;
}
export interface MCPTasksConfiguration {
  maxConcurrent: number;
  maxTTLSeconds: number;
}
export interface MCPPendingInteraction {
  id: string;
  serverId: string;
  serverName: string;
  kind: "sampling" | "sampling_result" | "elicitation";
  request: Record<string, any>;
  createdAt: string;
  expiresAt: string;
}

export interface MCPServerForm {
  name: string;
  displayName: string;
  description: string;
  transport: MCPTransport;
  endpoint: string;
  command: string;
  argsText: string;
  workDir: string;
  authType: MCPAuthType;
  enabled: boolean;
  credentialText: string;
  privateNetworkConfirmed: boolean;
}
