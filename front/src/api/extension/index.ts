import { apiClient } from "@/composables/useApi";
import { uiClientQueryParams } from "@/ui-runtime/clientInfo";
import type { UIContributionSummary, UIContributionSnapshot, SlotSnapshot } from "@/stores/extensionUI";
import type { UIProviderDefinition, UIProfile, UIProviderCapability, UIProfileEnvelope, UIProfileScopeKind } from "@/ui-runtime/types";
import type {
  BackendUIContributionDefinition,
  BackendUIContributionSnapshot,
  BackendSlotSnapshotEntry,
  BackendBridgeSessionResponse,
  BackendBridgeResponse,
  BackendOpenPageResult,
  BackendPageSessionStatus,
} from "./types";

export type * from "./types";

function transformContribution(def: BackendUIContributionDefinition): UIContributionSummary {
  const perms = (def.permissions ?? []).map((p) => p.name ?? p.permission).filter((p): p is string => !!p);
  const actions = (def.actions ?? []).map((a) => ({
    actionId: a.action_id,
    title: a.title?.default ?? a.action_id,
    icon: a.icon,
    riskLevel: a.risk_level,
  }));
  return {
    contributionId: def.contribution_id,
    extensionId: def.extension_id,
    moduleId: def.module_id,
    kind: def.kind,
    slotId: def.slot?.slot_id ?? "",
    contractVersion: def.contract_version,
    generation: def.integrity?.generation ?? 0,
    title: def.display?.title?.default ?? def.contribution_id,
    description: def.display?.description?.default,
    icon: def.display?.icon,
    ordering: def.ordering?.priority ?? 0,
    priority: def.ordering?.priority ?? 0,
    cellId: def.dispatch?.cell_id?.trim() || def.ordering?.sort_key?.trim() || def.contribution_id,
    entryKey: def.dispatch?.entry_key?.trim() || def.ordering?.sort_key?.trim() || def.contribution_id,
    matched: def.dispatch?.matched,
    visible: true,
    effective: true,
    enabled: true,
    runtimeReady: true,
    permissions: perms,
    sandbox: def.sandbox?.type,
    entryPath: def.entry?.path,
    schemaPath: def.entry?.schema_path,
    dataContract: def.data_contract && typeof def.data_contract === "object"
      ? def.data_contract as Record<string, unknown>
      : undefined,
    actions,
    visibility: def.visibility ? {
      requiredContext: def.visibility.required_context,
      platforms: def.visibility.platforms,
      messageTypes: def.visibility.message_types,
      conditions: def.visibility.conditions,
      userSetting: def.visibility.user_setting,
    } : undefined,
  };
}

function transformSlotSnapshot(entry: BackendSlotSnapshotEntry): SlotSnapshot {
  return {
    slotId: entry.slotId,
    contractVersion: entry.contractVersion ?? 1,
    supportedKinds: entry.supportedKinds ?? [],
    kind: entry.kind,
    layout: entry.layout ?? "stack",
    multiplicity: entry.multiplicity ?? "ordered_multiple",
    fallbackPolicy: entry.fallbackPolicy ?? "empty",
    description: entry.description,
    platforms: entry.platform,
    orderingPolicy: entry.orderingPolicy,
    failurePolicy: entry.failurePolicy,
    ownerExtension: entry.ownerExtension,
    parentSlotId: entry.parentSlotId,
    dynamic: entry.dynamic,
    scope: entry.scope,
    declarationEpoch: entry.declarationEpoch,
    performanceBudget: entry.performanceBudget,
    contributions: entry.contributions.map(transformContribution),
    generatedAt: new Date().toISOString(),
  };
}

function transformSnapshot(raw: BackendUIContributionSnapshot): UIContributionSnapshot {
  return {
    slots: raw.slots.map(transformSlotSnapshot),
    contributions: (raw.contributions ?? []).map(transformContribution),
    pendingContributions: (raw.pendingContributions ?? []).map(transformContribution),
    generatedAt: raw.timestamp,
    version: 1,
    providers: (raw.providers ?? []) as UIProviderDefinition[],
    profile: raw.profile as UIProfile | undefined,
    profileLayers: (raw.profileLayers ?? []) as UIProfile[],
    resolved: (raw.resolved ?? {}) as Partial<Record<UIProviderCapability, UIProviderDefinition>>,
    providerContext: raw.providerContext as UIContributionSnapshot["providerContext"],
    providerVersion: raw.providerVersion ?? 1,
  };
}

export async function fetchUISnapshot(platform = "web", deviceId = ""): Promise<UIContributionSnapshot> {
  const res = await apiClient.get<BackendUIContributionSnapshot>("/api/extensions/ui/snapshot", {
    params: { platform, ...(deviceId ? { deviceId } : {}), ...(await uiClientQueryParams()) },
  });
  return transformSnapshot(res.data);
}

export async function fetchSlots() {
  const res = await apiClient.get<{ slots: unknown[] }>("/api/extensions/ui/slots");
  return res.data.slots;
}

export async function fetchContributions(): Promise<UIContributionSummary[]> {
  const res = await apiClient.get<{ contributions: BackendUIContributionDefinition[] }>(
    "/api/extensions/ui/contributions",
  );
  return res.data.contributions.map(transformContribution);
}


export interface ConversationUIEventRecord {
  eventId: string;
  sequence: number;
  occurredAt: string;
  payload: Record<string, unknown>;
}

export async function fetchConversationUIEvents(
  conversationId: string,
  limit = 1000,
  offset = 0,
): Promise<{ items: ConversationUIEventRecord[]; total: number; offset: number }> {
  const res = await apiClient.get<{ items: ConversationUIEventRecord[]; total: number; offset: number }>(
    `/api/extensions/events/conversation-ui/${encodeURIComponent(conversationId)}`,
    { params: { limit, offset } },
  );
  return res.data;
}

export async function fetchConversationUIEventsBeforeSequence(
  conversationId: string,
  beforeSequence = 0,
  limit = 1000,
): Promise<{ items: ConversationUIEventRecord[]; total: number; beforeSequence: number }> {
  const res = await apiClient.get<{ items: ConversationUIEventRecord[]; total: number; beforeSequence: number }>(
    `/api/extensions/events/conversation-ui/${encodeURIComponent(conversationId)}`,
    { params: { limit, beforeSequence } },
  );
  return res.data;
}

export interface ClientRuntimeSessionState {
  userId?: string;
  conversationId: string;
  revision: number;
  packages: Array<{
    id: string;
    versions: Array<Record<string, unknown>>;
    approvedVersions?: string[];
    approveFutureVersions?: boolean;
    activeVersion?: string;
    currentPackageId?: string;
    targetVersion?: string;
    nextPackageId?: string;
    transitionState?: string;
    transitionMode?: string;
    runId?: string;
    pluginRunId?: string;
    lastError?: string;
    lastTransitionAt?: string;
    running?: boolean;
  }>;
}

export async function fetchClientRuntimeSessionState(conversationId: string): Promise<ClientRuntimeSessionState> {
  const res = await apiClient.get<ClientRuntimeSessionState>(
    "/api/extensions/ui/client-runtime-state",
    { params: { conversationId } },
  );
  return res.data;
}

export async function acknowledgeClientRuntimeSessionState(
  conversationId: string,
  revision: number,
): Promise<ClientRuntimeSessionState> {
  const res = await apiClient.post<ClientRuntimeSessionState>(
    "/api/extensions/ui/client-runtime-session-ack",
    { conversationId, revision },
  );
  return res.data;
}

export async function fetchExtensionContributions(extensionId: string): Promise<UIContributionSummary[]> {
  const res = await apiClient.get<{ contributions: BackendUIContributionDefinition[] }>(
    `/api/extensions/ui/by-extension`,
    { params: { extensionId } },
  );
  return res.data.contributions.map(transformContribution);
}

export async function createBridgeSession(params: {
  contributionId: string;
  origin?: string;
  grantedScopes?: string[];
  grantedPerms?: string[];
  lifetimeSeconds?: number;
}): Promise<BackendBridgeSessionResponse> {
  const res = await apiClient.post<BackendBridgeSessionResponse>("/api/extensions/ui/sessions", {
    contributionId: params.contributionId,
    origin: params.origin ?? "web",
    grantedScopes: params.grantedScopes ?? [],
    grantedPerms: params.grantedPerms ?? [],
    lifetimeSeconds: params.lifetimeSeconds ?? 3600,
  });
  return res.data;
}

export async function revokeBridgeSession(sessionId: string): Promise<void> {
  await apiClient.delete(`/api/extensions/ui/sessions/${encodeURIComponent(sessionId)}`);
}

export async function invokeBridgeMethod(sessionId: string, params: {
  method: string;
  contributionId: string;
  origin?: string;
  contractVersion?: number;
  payload?: unknown;
}): Promise<BackendBridgeResponse> {
  const res = await apiClient.post<BackendBridgeResponse>(
    `/api/extensions/ui/sessions/${encodeURIComponent(sessionId)}/bridge`,
    {
      method: params.method,
      contributionId: params.contributionId,
      origin: params.origin ?? "web",
      contractVersion: params.contractVersion ?? 1,
      payload: params.payload ?? {},
    },
  );
  return res.data;
}

export async function openExtensionPage(extensionId: string, pageId: string, params?: {
  params?: Record<string, unknown>;
  scopeSnapshot?: string;
  deepLinkOrigin?: string;
}): Promise<BackendOpenPageResult> {
  const res = await apiClient.post<BackendOpenPageResult>(
    `/api/extensions/ui/open-page`,
    {
      extensionId,
      pageId,
      params: params?.params ?? {},
      scopeSnapshot: params?.scopeSnapshot ?? "",
      deepLinkOrigin: params?.deepLinkOrigin ?? "",
    },
  );
  return res.data;
}

export async function pollPageSessionStatus(sessionId: string): Promise<BackendPageSessionStatus> {
  const res = await apiClient.get<BackendPageSessionStatus>(
    `/api/extensions/ui/page-sessions/${encodeURIComponent(sessionId)}/status`,
  );
  return res.data;
}

export async function closePageSession(sessionId: string): Promise<void> {
  await apiClient.delete(`/api/extensions/ui/page-sessions/${encodeURIComponent(sessionId)}`);
}

export async function fetchSchemaDocument(extensionId: string, contributionId: string): Promise<unknown> {
  const res = await apiClient.get<{ document: unknown }>(
    `/api/extensions/ui/schema/${encodeURIComponent(extensionId)}/${encodeURIComponent(contributionId)}`,
  );
  return res.data.document;
}

export async function fetchUIProviders(): Promise<UIProviderDefinition[]> {
  const res = await apiClient.get<{ providers: UIProviderDefinition[] }>("/api/extensions/ui/providers");
  return res.data.providers ?? [];
}

export async function fetchUIProfile(params: { platform?: string; deviceId?: string; scope?: UIProfileScopeKind } = {}): Promise<UIProfileEnvelope> {
  const res = await apiClient.get<UIProfileEnvelope>("/api/extensions/ui/profile", {
    params: { ...params, ...(await uiClientQueryParams()) },
  });
  return res.data;
}

export async function updateUIProfile(
  profile: UIProfile,
  params: { platform?: string; deviceId?: string; scope?: UIProfileScopeKind } = {},
): Promise<UIProfile> {
  const res = await apiClient.put<UIProfile>("/api/extensions/ui/profile", profile, {
    params: { ...params, ...(await uiClientQueryParams()) },
  });
  return res.data;
}

export async function deleteUIProfileOverride(params: { platform?: string; deviceId?: string; scope: UIProfileScopeKind; revision?: number }): Promise<void> {
  await apiClient.delete("/api/extensions/ui/profile", {
    params: { ...params, ...(await uiClientQueryParams()) },
  });
}

export async function resolveUIProvider(capability: UIProviderCapability, platform: string, deviceId = ""): Promise<UIProviderDefinition | null> {
  const res = await apiClient.get<{ provider?: UIProviderDefinition }>("/api/extensions/ui/providers/resolve", {
    params: { capability, platform, ...(deviceId ? { deviceId } : {}), ...(await uiClientQueryParams()) },
  });
  return res.data.provider ?? null;
}
