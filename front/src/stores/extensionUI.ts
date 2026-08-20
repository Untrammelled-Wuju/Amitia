import { defineStore } from "pinia";
import { ref, computed } from "vue";
import type { UIProviderCapability, UIProviderDefinition, UIProfile } from "@/ui-runtime/types";
import {
  fetchUISnapshot,
  fetchContributions,
  createBridgeSession,
  revokeBridgeSession,
  updateUIProfile,
} from "@/api/extension";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";

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
  visible: boolean;
  effective: boolean;
  enabled: boolean;
  runtimeReady: boolean;
  permissions?: string[];
  sandbox?: string;
  entryPath?: string;
  schemaPath?: string;
  actions?: Array<{
    actionId: string;
    title: string;
    icon?: string;
    riskLevel?: string;
  }>;
  hiddenReason?: string;
}

export interface SlotSnapshot {
  slotId: string;
  contractVersion: number;
  layout: string;
  multiplicity: string;
  fallbackPolicy: string;
  contributions: UIContributionSummary[];
  generatedAt: string;
}

export interface UIContributionSnapshot {
  slots: SlotSnapshot[];
  contributions?: UIContributionSummary[];
  generatedAt: string;
  version: number;
  providers?: UIProviderDefinition[];
  profile?: UIProfile;
  resolved?: Partial<Record<UIProviderCapability, UIProviderDefinition>>;
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
  const snapshot = ref<UIContributionSnapshot | null>(null);
  const lastFetchAt = ref<number>(0);
  const loading = ref(false);
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
    return snapshot.value?.resolved?.[capability] ?? null;
  }

  function getProviders(capability?: UIProviderCapability): UIProviderDefinition[] {
    const providers = snapshot.value?.providers ?? [];
    return capability ? providers.filter((provider) => provider.capability === capability) : providers;
  }

  function getVisibleContributions(slotId: string): UIContributionSummary[] {
    const all = getSlotContributions(slotId);
    const pref = layoutPrefs.value[slotId];
    if (!pref) return [...all].sort((a, b) => a.ordering - b.ordering);
    return all
      .filter((c) => !pref.hiddenContributions.includes(c.contributionId))
      .sort((a, b) => {
        const oa = pref.ordering[a.contributionId] ?? a.ordering;
        const ob = pref.ordering[b.contributionId] ?? b.ordering;
        return oa - ob;
      });
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
    try {
      const next = await fetchUISnapshot(resolveHostEnvironment().platform);
      snapshot.value = next;
      lastFetchAt.value = Date.now();
    } catch (e) {
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

  async function loadAllContributions(): Promise<UIContributionSummary[]> {
    return fetchContributions();
  }

  async function selectProvider(capability: UIProviderCapability, providerId?: string): Promise<void> {
    const current = snapshot.value?.profile ?? { profileId: "default", name: "Default", selections: {} };
    const selections = { ...(current.selections ?? {}) };
    if (providerId) selections[capability] = providerId;
    else delete selections[capability];
    const profile = await updateUIProfile({ ...current, selections, updatedAt: Date.now() });
    if (snapshot.value) snapshot.value = { ...snapshot.value, profile };
    await refreshSnapshot(true);
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
