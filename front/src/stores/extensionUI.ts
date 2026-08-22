import { defineStore } from "pinia";
import { ref, computed } from "vue";
import type { UIProviderCapability, UIProviderDefinition, UIProfile, UIProfileScopeKind, UIProviderResolveContext } from "@/ui-runtime/types";
import {
  fetchUISnapshot,
  fetchContributions,
  fetchUIProfile,
  createBridgeSession,
  revokeBridgeSession,
  updateUIProfile,
  deleteUIProfileOverride,
} from "@/api/extension";
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";
import { resolveUIClientInfo } from "@/ui-runtime/clientInfo";
import { loadLastKnownGoodSnapshot, saveLastKnownGoodSnapshot } from "@/ui-runtime/snapshotCache";
import { isProviderCompatible } from "@/ui-runtime/providerRuntime";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import { getRuntimeConnection } from "@/runtime/runtime-adapter";
import { useSessionStore } from "@/stores/session-store";

export interface UIContributionSummary {
  contributionId: string;
  extensionId: string;
  moduleId: string;
  kind: string;
  slotId: string;
  contractVersion: number;
  generation: number;
  title: string;
  description?: string;
  icon?: string;
  ordering: number;
  priority?: number;
  visible: boolean;
  effective: boolean;
  enabled: boolean;
  runtimeReady: boolean;
  permissions?: string[];
  sandbox?: string;
  entryPath?: string;
  schemaPath?: string;
  dataContract?: Record<string, unknown>;
  actions?: Array<{
    actionId: string;
    title: string;
    icon?: string;
    riskLevel?: string;
  }>;
  hiddenReason?: string;
  visibility?: {
    requiredContext?: string[];
    platforms?: string[];
    messageTypes?: string[];
    conditions?: Array<{ field: string; operator: string; value?: unknown }>;
    userSetting?: string;
  };
}

export interface SlotSnapshot {
  slotId: string;
  contractVersion: number;
  supportedKinds?: string[];
  layout: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal" | "hidden";
  multiplicity: "single" | "multiple" | "ordered_multiple" | "replaceable_single" | "exclusive";
  fallbackPolicy: "none" | "skeleton" | "empty" | "default";
  description?: string;
  platforms?: string[];
  orderingPolicy?: string;
  failurePolicy?: string;
  ownerExtension?: string;
  parentSlotId?: string;
  dynamic?: boolean;
  scope?: "root" | "session-maybe" | "session";
  declarationEpoch?: number;
  performanceBudget?: {
    firstPaint: number;
    bundleSize: number;
    memoryBytes: number;
    messageRate: number;
    updateFrequency: number;
  };
  contributions: UIContributionSummary[];
  generatedAt: string;
}

export interface UIContributionSnapshot {
  slots: SlotSnapshot[];
  contributions?: UIContributionSummary[];
  pendingContributions?: UIContributionSummary[];
  generatedAt: string;
  version: number;
  providers?: UIProviderDefinition[];
  profile?: UIProfile;
  profileLayers?: UIProfile[];
  resolved?: Partial<Record<UIProviderCapability, UIProviderDefinition>>;
  providerContext?: UIProviderResolveContext;
  providerVersion?: number;
}

export interface SlotError {
  contributionId: string;
  slotId: string;
  message: string;
  timestamp: number;
  recoverable: boolean;
}

export interface SlotSession {
  contributionId: string;
  sessionId: string;
  createdAt: number;
  lastActivity: number;
}

export interface LayoutPreference {
  slotId: string;
  hiddenContributions: string[];
  ordering: Record<string, number>;
  collapsed?: boolean;
}

export const useExtensionUIStore = defineStore("extensionUI", () => {
  const sessionStore = useSessionStore();
  const snapshot = ref<UIContributionSnapshot | null>(null);
  const lastFetchAt = ref<number>(0);
  const loading = ref(false);
  const usingLastKnownGood = ref(false);
  const profileScope = ref<UIProfileScopeKind>("user");
  const scopeProfile = ref<UIProfile | null>(null);
  const scopeExists = ref(false);
  const errors = ref<SlotError[]>([]);
  const sessions = ref<Map<string, SlotSession>>(new Map());
  const layoutPrefs = ref<Record<string, LayoutPreference>>({});

  const slotsById = computed(() => {
    const map = new Map<string, SlotSnapshot>();
    if (snapshot.value) {
      for (const slot of snapshot.value.slots) {
        map.set(slot.slotId, slot);
      }
    }
    return map;
  });

  function setSnapshot(next: UIContributionSnapshot) {
    snapshot.value = next;
    lastFetchAt.value = Date.now();
  }

  function getSlotContributions(slotId: string): UIContributionSummary[] {
    const slot = slotsById.value.get(slotId);
    if (!slot) return [];
    return slot.contributions.filter((c) => c.visible && c.effective && c.enabled && c.runtimeReady);
  }

  function getContributionById(contributionId: string): UIContributionSummary | null {
    if (!snapshot.value) return null;
    const catalogMatch = snapshot.value.contributions?.find((item) => item.contributionId === contributionId);
    if (catalogMatch) return catalogMatch;
    for (const slot of snapshot.value.slots) {
      const match = slot.contributions.find((item) => item.contributionId === contributionId);
      if (match) return match;
    }
    return null;
  }

  function getResolvedProvider(capability: UIProviderCapability): UIProviderDefinition | null {
    const current = snapshot.value;
    if (!current) return null;
    const platform = resolveHostEnvironment().platform;
    const providers = (current.providers ?? []).filter((provider) => provider.capability === capability);
    const byId = new Map(providers.map((provider) => [provider.providerId, provider]));

    // Re-resolve the server selection locally because a cached LKG snapshot can
    // transition from an online device context to offline. In that case the
    // previously resolved device provider is no longer compatible, but its
    // declared cloud fallback can still be rendered safely.
    const resolveFrom = (initialId?: string): UIProviderDefinition | null => {
      let id = initialId?.trim() ?? "";
      const seen = new Set<string>();
      while (id && !seen.has(id)) {
        seen.add(id);
        const provider = byId.get(id);
        if (!provider) return null;
        if (isProviderCompatible(provider, current.providerContext, platform)) return provider;
        id = provider.fallbackProviderId?.trim() ?? "";
      }
      return null;
    };

    const resolved = current.resolved?.[capability];
    const fromServer = resolveFrom(resolved?.providerId);
    if (fromServer) return fromServer;

    const selected = current.profile?.selections?.[capability];
    const fromProfile = resolveFrom(selected);
    if (fromProfile) return fromProfile;

    return providers.find((provider) =>
      provider.builtin && isProviderCompatible(provider, current.providerContext, platform),
    ) ?? null;
  }

  function getProviders(capability?: UIProviderCapability): UIProviderDefinition[] {
    const providers = snapshot.value?.providers ?? [];
    return capability ? providers.filter((provider) => provider.capability === capability) : providers;
  }

  function getVisibleContributions(slotId: string, context?: Record<string, unknown>): UIContributionSummary[] {
    const all = getSlotContributions(slotId);
    const pref = layoutPrefs.value[slotId];
    const filtered = context ? all.filter((contribution) => matchesVisibility(contribution, context)) : all;
    if (!pref) return [...filtered].sort((a, b) => a.ordering - b.ordering);
    return filtered
      .filter((c) => !pref.hiddenContributions.includes(c.contributionId))
      .sort((a, b) => {
        const oa = pref.ordering[a.contributionId] ?? a.ordering;
        const ob = pref.ordering[b.contributionId] ?? b.ordering;
        return oa - ob;
      });
  }

  function matchesVisibility(contribution: UIContributionSummary, context: Record<string, unknown>): boolean {
    const rule = contribution.visibility;
    if (!rule) return true;
    const platform = String(context.platform ?? resolveHostEnvironment().platform);
    if (rule.platforms?.length && !rule.platforms.includes(platform)) return false;
    const messageType = String(context.messageType ?? context.type ?? "");
    if (rule.messageTypes?.length && !rule.messageTypes.includes(messageType)) return false;
    if (rule.requiredContext?.some((key) => lookupContext(context, key) === undefined)) return false;
    if (rule.conditions?.some((condition) => !matchesCondition(context, condition))) return false;
    return true;
  }

  function lookupContext(context: Record<string, unknown>, path: string): unknown {
    if (!path) return undefined;
    let current: unknown = context;
    for (const segment of path.split(".")) {
      if (!current || typeof current !== "object" || !(segment in (current as Record<string, unknown>))) return undefined;
      current = (current as Record<string, unknown>)[segment];
    }
    return current;
  }

  function matchesCondition(
    context: Record<string, unknown>,
    condition: { field: string; operator: string; value?: unknown },
  ): boolean {
    const actual = lookupContext(context, condition.field);
    switch (condition.operator) {
      case "==":
      case "eq":
        return actual === condition.value;
      case "!=":
      case "ne":
        return actual !== condition.value;
      case "in":
        return Array.isArray(condition.value) && condition.value.includes(actual);
      case "not_in":
        return Array.isArray(condition.value) && !condition.value.includes(actual);
      case "not_null":
        return actual !== undefined && actual !== null;
      case "is_null":
        return actual === undefined || actual === null;
      case "contains":
        return typeof actual === "string" && typeof condition.value === "string" && actual.includes(condition.value);
      default:
        return false;
    }
  }

  function recordError(error: SlotError) {
    errors.value = [error, ...errors.value].slice(0, 100);
  }

  function clearErrors(contributionId?: string) {
    if (!contributionId) {
      errors.value = [];
      return;
    }
    errors.value = errors.value.filter((e) => e.contributionId !== contributionId);
  }

  function registerSession(session: SlotSession) {
    sessions.value.set(session.contributionId, session);
    sessions.value = new Map(sessions.value);
  }

  function unregisterSession(contributionId: string) {
    sessions.value.delete(contributionId);
    sessions.value = new Map(sessions.value);
  }

  function updateLayoutPreference(slotId: string, pref: Partial<LayoutPreference>) {
    const existing = layoutPrefs.value[slotId] || {
      slotId,
      hiddenContributions: [],
      ordering: {},
      collapsed: false,
    };
    layoutPrefs.value[slotId] = { ...existing, ...pref };
  }

  function clearScope(scopeKey: string) {
    for (const [key, session] of sessions.value.entries()) {
      if (key.includes(scopeKey)) {
        sessions.value.delete(key);
      }
    }
    sessions.value = new Map(sessions.value);
  }

  function invalidateSnapshot() {
    snapshot.value = null;
  }

  async function refreshSnapshot(force = false): Promise<void> {
    if (loading.value) return;
    if (!force && snapshot.value && Date.now() - lastFetchAt.value < 30_000) return;
    loading.value = true;
    const platform = resolveHostEnvironment().platform;
    const deviceId = await resolveUIHostDeviceId();
    const backendNamespace = await getRuntimeConnection().then((connection) => connection.apiBaseURL).catch(() => window.location.origin);
    const cacheNamespace = `${backendNamespace}|user=${sessionStore.state.value.userId || "anonymous"}`;
    try {
      const next = await fetchUISnapshot(platform, deviceId);
      snapshot.value = next;
      lastFetchAt.value = Date.now();
      usingLastKnownGood.value = false;
      saveLastKnownGoodSnapshot(platform, deviceId, next, cacheNamespace);
    } catch (e) {
      const cached = loadLastKnownGoodSnapshot(platform, deviceId, cacheNamespace);
      if (cached) {
        const client = await resolveUIClientInfo();
        snapshot.value = {
          ...cached,
          providerContext: {
            ...(cached.providerContext ?? { platform }),
            platform,
            deviceId,
            architecture: client.architecture,
            appVersion: client.appVersion,
            deviceOnline: false,
            runtimeVersion: undefined,
            deviceCapabilities: [],
          },
        };
        lastFetchAt.value = Date.now();
        usingLastKnownGood.value = true;
      }
      recordError({
        contributionId: "",
        slotId: "",
        message: e instanceof Error ? e.message : String(e),
        timestamp: Date.now(),
        recoverable: true,
      });
    } finally {
      loading.value = false;
    }
  }

  async function loadProfileScope(scope: UIProfileScopeKind = profileScope.value): Promise<void> {
    const platform = resolveHostEnvironment().platform;
    const deviceId = await resolveUIHostDeviceId();
    const envelope = await fetchUIProfile({ platform, deviceId, scope });
    profileScope.value = scope;
    scopeProfile.value = envelope.scopeProfile;
    scopeExists.value = envelope.scopeExists;
    if (snapshot.value) {
      snapshot.value = {
        ...snapshot.value,
        profile: envelope.profile,
        profileLayers: envelope.layers,
        providerContext: envelope.context,
      };
    }
  }

  async function loadAllContributions(): Promise<UIContributionSummary[]> {
    return fetchContributions();
  }

  async function selectProvider(
    capability: UIProviderCapability,
    providerId?: string,
    scope: UIProfileScopeKind = profileScope.value,
  ): Promise<void> {
    const platform = resolveHostEnvironment().platform;
    const deviceId = await resolveUIHostDeviceId();
    // Always edit the exact layer, never the merged effective profile.
    const envelope = await fetchUIProfile({ platform, deviceId, scope });
    const current = envelope.scopeProfile;
    const selections = { ...(current.selections ?? {}) };
    if (providerId) selections[capability] = providerId;
    else delete selections[capability];
    try {
      const saved = await updateUIProfile(
        { ...current, selections, revision: current.revision ?? 0, updatedAt: Date.now() },
        { platform, deviceId, scope },
      );
      profileScope.value = scope;
      scopeProfile.value = saved;
      scopeExists.value = true;
    } catch (error) {
      // Refresh the optimistic revision before surfacing a conflict to the user.
      await loadProfileScope(scope).catch(() => {});
      throw error;
    }
    await refreshSnapshot(true);
    await loadProfileScope(scope);
  }

  async function resetProfileScope(scope: UIProfileScopeKind = profileScope.value): Promise<void> {
    if (scope === "global") throw new Error("全局 UI Profile 不能删除，只能由管理员修改");
    const platform = resolveHostEnvironment().platform;
    const deviceId = await resolveUIHostDeviceId();
    const envelope = await fetchUIProfile({ platform, deviceId, scope });
    await deleteUIProfileOverride({ platform, deviceId, scope, revision: envelope.scopeProfile.revision ?? 0 });
    scopeProfile.value = null;
    scopeExists.value = false;
    await refreshSnapshot(true);
    await loadProfileScope(scope);
  }

  async function startSession(params: {
    contributionId: string;
    origin?: string;
    grantedScopes?: string[];
    grantedPerms?: string[];
  }): Promise<string> {
    const res = await createBridgeSession(params);
    const key = params.contributionId;
    registerSession({
      contributionId: key,
      sessionId: res.sessionId,
      createdAt: Date.now(),
      lastActivity: Date.now(),
    });
    return res.sessionId;
  }

  async function endSession(sessionId: string, contributionId: string): Promise<void> {
    await revokeBridgeSession(sessionId);
    unregisterSession(contributionId);
  }

  async function notifyExtensionChanged(changeType: "install" | "uninstall" | "enable" | "disable" | "update"): Promise<void> {
    if (loading.value) return;
    snapshot.value = null;
    lastFetchAt.value = 0;
    await refreshSnapshot(true);
  }

  function dispatchExtensionChanged(changeType: "install" | "uninstall" | "enable" | "disable" | "update"): void {
    if (typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("amitia:extension-state-changed", { detail: { changeType } }));
    }
  }

  function setupExtensionChangeListener(): () => void {
    const handler = () => {
      void notifyExtensionChanged("update");
    };
    window.addEventListener("amitia:extension-state-changed", handler);
    return () => window.removeEventListener("amitia:extension-state-changed", handler);
  }

  return {
    snapshot,
    loading,
    usingLastKnownGood,
    profileScope,
    scopeProfile,
    scopeExists,
    errors,
    sessions,
    layoutPrefs,
    lastFetchAt,
    slotsById,
    setSnapshot,
    getSlotContributions,
    getVisibleContributions,
    getContributionById,
    getResolvedProvider,
    getProviders,
    selectProvider,
    loadProfileScope,
    resetProfileScope,
    recordError,
    clearErrors,
    registerSession,
    unregisterSession,
    updateLayoutPreference,
    clearScope,
    invalidateSnapshot,
    refreshSnapshot,
    loadAllContributions,
    startSession,
    endSession,
    notifyExtensionChanged,
    dispatchExtensionChanged,
    setupExtensionChangeListener,
  };
});
