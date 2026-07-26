import { defineStore } from "pinia";
import { ref, computed } from "vue";

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
  generatedAt: string;
  version: number;
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
    recordError,
    clearErrors,
    registerSession,
    unregisterSession,
    updateLayoutPreference,
    clearScope,
    invalidateSnapshot,
  };
});
