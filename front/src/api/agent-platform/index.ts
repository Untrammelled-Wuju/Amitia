import { apiClient } from "@/composables/useApi";

const BASE = "/api/agent-platform";
const PLATFORM_SESSION_KEY = "amitia.agent-platform.session";

export interface PlatformSessionSnapshot {
  token: string;
  user: Record<string, any>;
  memberships: Record<string, any>[];
  expiresAt: string;
}

export function platformSession(): PlatformSessionSnapshot | null {
  if (typeof window === "undefined") return null;
  try {
    const value = window.localStorage.getItem(PLATFORM_SESSION_KEY);
    return value ? JSON.parse(value) : null;
  } catch {
    return null;
  }
}

function storePlatformSession(value: PlatformSessionSnapshot | null) {
  if (typeof window === "undefined") return;
  if (value) window.localStorage.setItem(PLATFORM_SESSION_KEY, JSON.stringify(value));
  else window.localStorage.removeItem(PLATFORM_SESSION_KEY);
}

apiClient.interceptors.request.use((config) => {
  const path = String(config.url || "");
  if (!path.startsWith(BASE)) return config;
  const session = platformSession();
  if (session?.token) {
    config.headers.set("X-Amitia-Platform-Session", session.token);
  }
  return config;
});

export async function loginPlatformUser(email: string, password: string): Promise<PlatformSessionSnapshot> {
  const response = await apiClient.post(`${BASE}/auth/login`, { email, password });
  const payload = response.data as Record<string, any>;
  const session = {
    token: String(payload.token || ""),
    user: payload.user || {},
    memberships: payload.membership || [],
    expiresAt: String(payload.expiresAt || ""),
  };
  if (!session.token) throw new Error("平台登录未返回会话令牌");
  storePlatformSession(session);
  return session;
}

export async function logoutPlatformUser(): Promise<void> {
  try {
    await apiClient.post(`${BASE}/auth/logout`, {});
  } finally {
    storePlatformSession(null);
  }
}

export async function acceptPlatformInvitation(payload: {
  token: string;
  displayName: string;
  password: string;
}): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/users/invitations/accept`, payload));
}

export interface AgentProfile {
  id: string;
  spaceId: string;
  characterId: string;
  name: string;
  description: string;
  archetype: string;
  status: string;
  ownerId: string;
  teamId: string;
  currentRevisionId: string;
  currentReleaseId: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

export interface AgentRevision {
  id: string;
  revision: number;
  configJson: string;
  status: string;
  testStatus: string;
  testEvidence: string;
  changeNote: string;
  createdAt: string;
}

export interface AgentRelease {
  id: string;
  revisionId: string;
  version: string;
  status: string;
  releaseNotes: string;
  createdAt: string;
}

export interface AgentDetail {
  profile: AgentProfile;
  revision: AgentRevision;
  release?: AgentRelease;
  history: AgentRevision[];
}

export interface Team {
  id: string;
  name: string;
  description: string;
  leadAgentId: string;
  workspaceId: string;
  workspaceRootUri: string;
  sharedMemory: boolean;
  sharedWorkspace: boolean;
  status: string;
  revision: number;
}

export interface ApprovalRecord {
  id: string;
  agentId: string;
  actionSummary: string;
  resourceSummary: string;
  toolName: string;
  riskLevel: string;
  status: string;
  revision: number;
  expiresAt: string;
}

export interface ReconciliationCase {
  id: string;
  agentId: string;
  reason: string;
  originalError: string;
  status: string;
  revision: number;
}

export interface RuntimeInstance {
  id: string;
  agentId: string;
  provider: string;
  state: string;
  rootPath: string;
  endpoint: string;
  cpuQuota: number;
  memoryBytes: number;
  revision: number;
}

export interface RuntimeExecResult {
  runtimeId: string;
  provider: string;
  output: string;
  exitCode: number;
  durationMs: number;
}

export interface ActivityEvent {
  id: string;
  actorId: string;
  actorType: string;
  agentId: string;
  teamId: string;
  kind: string;
  state: string;
  title: string;
  detail: string;
  waitingReason: string;
  costMicroUsd: number;
  durationMs: number;
  occurredAt: string;
}

export interface MarketplaceCategory {
  id: string;
  name: string;
  description: string;
  icon: string;
  sortOrder: number;
}

export interface MarketplaceListing {
  id: string;
  extensionId: string;
  categoryId: string;
  publisherId: string;
  displayName: string;
  summary: string;
  description: string;
  latestVersion: string;
  trustLevel: string;
  securityScore: number;
  installCount: number;
  ratingAverage: number;
  status: string;
}

export interface MarketplaceListingView {
  listing: MarketplaceListing;
  category: MarketplaceCategory;
  versions: Array<{ id: string; version: string; status: string }>;
  installable: boolean;
}

export interface OverviewData {
  agents: AgentProfile[];
  activity: ActivityEvent[];
  summary: {
    running: number;
    waiting: number;
    failed: number;
    completed: number;
    costMicroUsd: number;
    averageDurationMs: number;
  };
  pendingApprovals: ApprovalRecord[];
  reconciliations: ReconciliationCase[];
}

function data<T>(response: { data: T }): T {
  return response.data;
}

function collection(value: unknown, field: string): unknown[] {
  if (value == null) return [];
  if (!Array.isArray(value)) {
    throw new Error(`Agent Platform 字段 ${field} 需要数组，实际收到 ${JSON.stringify(value)}`);
  }
  return value;
}

function collectionData<T>(response: { data: { data: unknown } }, field: string): T[] {
  return collection(data(response), field) as T[];
}

function overviewData(value: unknown): OverviewData {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`Agent Platform overview 返回值无效: ${JSON.stringify(value)}`);
  }
  const raw = value as Record<string, unknown>;
  if (!raw.summary || typeof raw.summary !== "object" || Array.isArray(raw.summary)) {
    throw new Error(`Agent Platform 字段 overview.summary 需要对象，实际收到 ${JSON.stringify(raw.summary)}`);
  }
  return {
    agents: collection(raw.agents, "overview.agents") as AgentProfile[],
    activity: collection(raw.activity, "overview.activity") as ActivityEvent[],
    summary: raw.summary as OverviewData["summary"],
    pendingApprovals: collection(raw.pendingApprovals, "overview.pendingApprovals") as ApprovalRecord[],
    reconciliations: collection(raw.reconciliations, "overview.reconciliations") as ReconciliationCase[],
  };
}

export async function getAgentPlatformBootstrap() {
  return data(await apiClient.get(`${BASE}/bootstrap`));
}

export async function getAgentPlatformOverview(): Promise<OverviewData> {
  return overviewData(data(await apiClient.get(`${BASE}/overview`)));
}

export async function listAgents(): Promise<AgentProfile[]> {
  return collectionData<AgentProfile>(await apiClient.get(`${BASE}/agents`, { params: { includeDisabled: true } }), "agents");
}

export async function getAgent(id: string): Promise<AgentDetail> {
  return data(await apiClient.get(`${BASE}/agents/${encodeURIComponent(id)}`));
}

export async function createAgent(payload: Record<string, unknown>): Promise<AgentDetail> {
  return data(await apiClient.post(`${BASE}/agents`, payload));
}

export async function updateAgent(id: string, payload: Record<string, unknown>): Promise<AgentDetail> {
  return data(await apiClient.put(`${BASE}/agents/${encodeURIComponent(id)}`, payload));
}

export async function listAgentReleases(id: string): Promise<AgentRelease[]> {
  return collectionData<AgentRelease>(await apiClient.get(`${BASE}/agents/${encodeURIComponent(id)}/releases`), "agentReleases");
}

export async function diffAgentRevisions(id: string, from: number, to: number): Promise<Record<string, unknown>> {
  return data(await apiClient.get(`${BASE}/agents/${encodeURIComponent(id)}/diff`, { params: { from, to } }));
}

export async function testAgent(id: string, revisionId: string): Promise<AgentRevision> {
  return data(await apiClient.post(`${BASE}/agents/${encodeURIComponent(id)}/test`, { revisionId }));
}

export async function publishAgent(id: string, payload: Record<string, unknown>): Promise<AgentRelease> {
  return data(await apiClient.post(`${BASE}/agents/${encodeURIComponent(id)}/publish`, payload));
}

export async function rollbackAgent(id: string, payload: Record<string, unknown>): Promise<AgentDetail> {
  return data(await apiClient.post(`${BASE}/agents/${encodeURIComponent(id)}/rollback`, payload));
}

export async function disableAgent(id: string, expectedRevision: number): Promise<void> {
  await apiClient.post(`${BASE}/agents/${encodeURIComponent(id)}/disable`, { expectedRevision });
}

export async function exportAgent(id: string, version = ""): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/agents/${encodeURIComponent(id)}/export`, { version }));
}

export async function exportAgentArchive(id: string, version = ""): Promise<{ blob: Blob; fileName: string }> {
  const response = await apiClient.post(`${BASE}/agents/${encodeURIComponent(id)}/export-archive`, { version }, { responseType: "blob" });
  const disposition = String(response.headers["content-disposition"] || "");
  const match = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(disposition);
  return {
    blob: response.data as Blob,
    fileName: match ? decodeURIComponent(match[1]) : "agent.amitia-agent.zip",
  };
}

export async function importAgentPackage(payload: Record<string, unknown>): Promise<AgentDetail> {
  return data(await apiClient.post(`${BASE}/agents/import`, payload));
}

export async function importAgentArchive(file: File): Promise<AgentDetail> {
  const form = new FormData();
  form.append("package", file);
  return data(await apiClient.post(`${BASE}/agents/import-archive`, form));
}

export async function getAgentAutonomy(id: string): Promise<Record<string, any>> {
  return data(await apiClient.get(`${BASE}/agents/${encodeURIComponent(id)}/autonomy`));
}

export async function saveAgentAutonomy(id: string, payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.put(`${BASE}/agents/${encodeURIComponent(id)}/autonomy`, payload));
}

export async function listTeams(): Promise<Team[]> {
  return collectionData<Team>(await apiClient.get(`${BASE}/teams`), "teams");
}

export async function listTeamTemplates(): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${BASE}/teams/templates`), "teamTemplates");
}

export async function createTeamFromTemplate(payload: Record<string, unknown>): Promise<Team> {
  return data(await apiClient.post(`${BASE}/teams/from-template`, payload));
}

export async function routeTeamMessage(teamId: string, payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/teams/${encodeURIComponent(teamId)}/route`, payload));
}

export async function listPlatformUsers(): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${BASE}/users`), "platformUsers");
}

export async function invitePlatformUser(payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/users/invitations`, payload));
}

export async function createTeam(payload: Record<string, unknown>): Promise<Team> {
  return data(await apiClient.post(`${BASE}/teams`, payload));
}

export async function getTeam(id: string): Promise<Record<string, unknown>> {
  return data(await apiClient.get(`${BASE}/teams/${encodeURIComponent(id)}`));
}

export async function addTeamMember(teamId: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/teams/${encodeURIComponent(teamId)}/members`, payload));
}

export async function createTeamContext(teamId: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/teams/${encodeURIComponent(teamId)}/contexts`, payload));
}

export async function listApprovals(status = "pending"): Promise<ApprovalRecord[]> {
  return collectionData<ApprovalRecord>(await apiClient.get(`${BASE}/approvals`, { params: { status } }), "approvals");
}

export async function decideApproval(id: string, payload: Record<string, unknown>): Promise<ApprovalRecord> {
  return data(await apiClient.post(`${BASE}/approvals/${encodeURIComponent(id)}/decide`, payload));
}

export async function listReconciliations(status = "open"): Promise<ReconciliationCase[]> {
  return collectionData<ReconciliationCase>(await apiClient.get(`${BASE}/reconciliations`, { params: { status } }), "reconciliations");
}

export async function listCredentialGrants(agentId: string): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${BASE}/credentials`, { params: { agentId } }), "credentialGrants");
}

export async function createCredentialGrant(payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/credentials`, payload));
}

export async function revokeCredentialGrant(id: string): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/credentials/${encodeURIComponent(id)}/revoke`, {}));
}

export async function listNetworkPolicies(agentId = ""): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${BASE}/network-policies`, { params: { agentId } }), "networkPolicies");
}

export async function createNetworkPolicy(payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/network-policies`, payload));
}

export async function updateNetworkPolicy(id: string, payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.put(`${BASE}/network-policies/${encodeURIComponent(id)}`, payload));
}

export async function listBudgetPolicies(scopeType = "", scopeId = ""): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${BASE}/budgets`, { params: { scopeType, scopeId } }), "budgetPolicies");
}

export async function createBudgetPolicy(payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/budgets`, payload));
}

export async function resolveReconciliation(id: string, payload: Record<string, unknown>): Promise<ReconciliationCase> {
  return data(await apiClient.post(`${BASE}/reconciliations/${encodeURIComponent(id)}/resolve`, payload));
}

export async function listRuntimes(agentId = ""): Promise<RuntimeInstance[]> {
  return collectionData<RuntimeInstance>(await apiClient.get(`${BASE}/runtimes`, { params: { agentId } }), "runtimes");
}

export async function createRuntime(payload: Record<string, unknown>): Promise<RuntimeInstance> {
  return data(await apiClient.post(`${BASE}/runtimes`, payload));
}

export async function registerRemoteRuntime(payload: { runtime: Record<string, unknown>; endpoint: string }): Promise<RuntimeInstance> {
  return data(await apiClient.post(`${BASE}/runtimes/remote`, payload));
}

export async function runtimeAction(id: string, action: string, payload: Record<string, unknown> = {}): Promise<RuntimeInstance> {
  return data(await apiClient.post(`${BASE}/runtimes/${encodeURIComponent(id)}/${action}`, payload));
}

export async function execRuntime(id: string, payload: { command: string; workingDir?: string; timeoutSeconds?: number }): Promise<RuntimeExecResult> {
  return data(await apiClient.post(`${BASE}/runtimes/${encodeURIComponent(id)}/exec`, payload));
}

export async function snapshotRuntime(id: string, name = ""): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/runtimes/${encodeURIComponent(id)}/snapshots`, { name }));
}

export async function createHostControl(agentId: string, capabilities: string[]): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/host-control`, {
    agentId,
    capabilities,
    permission: {},
    expiresInSeconds: 1800,
  }));
}

export async function approveHostControl(id: string): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/host-control/${encodeURIComponent(id)}/approve`, {}));
}

export async function endHostControl(id: string): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/host-control/${encodeURIComponent(id)}/end`, {}));
}

export async function executeHostControl(id: string, action: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/host-control/${encodeURIComponent(id)}/actions`, action));
}

export async function listActivity(params: Record<string, unknown> = {}): Promise<{ items: ActivityEvent[]; summary: OverviewData["summary"] }> {
  const value = data<{ items: unknown; summary: OverviewData["summary"] }>(await apiClient.get(`${BASE}/activity`, { params }));
  return {
    ...value,
    items: collection(value?.items, "activity.items") as ActivityEvent[],
  };
}

export async function listMarketCategories(): Promise<MarketplaceCategory[]> {
  return collectionData<MarketplaceCategory>(await apiClient.get(`${BASE}/market/categories`), "marketCategories");
}

export async function listMarketListings(params: Record<string, unknown> = {}): Promise<MarketplaceListingView[]> {
  return collectionData<MarketplaceListingView>(await apiClient.get(`${BASE}/market/listings`, { params }), "marketListings");
}

export async function installMarketListing(id: string, version = ""): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/market/listings/${encodeURIComponent(id)}/install`, { version }));
}

export async function listEvaluationSuites(agentId = ""): Promise<Record<string, unknown>[]> {
  return collectionData<Record<string, unknown>>(await apiClient.get(`${BASE}/evaluations/suites`, { params: { agentId } }), "evaluationSuites");
}

export async function createEvaluationSuite(payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/evaluations/suites`, payload));
}

export async function createEvaluationCase(suiteId: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/evaluations/suites/${encodeURIComponent(suiteId)}/cases`, payload));
}

export async function runEvaluation(suiteId: string, payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  return data(await apiClient.post(`${BASE}/evaluations/suites/${encodeURIComponent(suiteId)}/run`, payload));
}

export async function listA2ACards(): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${BASE}/a2a/cards`), "a2aCards");
}

export async function registerA2ACard(payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/a2a/cards`, payload));
}

export async function listA2ATasks(agentId = ""): Promise<Record<string, any>[]> {
  return collectionData<Record<string, any>>(await apiClient.get(`${BASE}/a2a/tasks`, { params: { agentId } }), "a2aTasks");
}

export async function submitA2ATask(payload: Record<string, unknown>): Promise<Record<string, any>> {
  return data(await apiClient.post(`${BASE}/a2a/tasks`, payload));
}
