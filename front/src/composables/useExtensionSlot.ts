import { ref, computed, type Ref } from "vue";
import { useExtensionUIStore, type UIContributionSummary } from "@/stores/extensionUI";

export interface SlotContext {
  slotId: string;
  contractVersion: number;
  platform: "windows" | "macos" | "linux" | "web";
  theme: { mode: "light" | "dark"; density: "comfortable" | "compact" };
  locale: string;
  data: Record<string, unknown>;
}

export interface UseExtensionSlotOptions {
  slotId: string;
  context?: Ref<Record<string, unknown>>;
  fallback?: "none" | "skeleton" | "empty" | "default";
  layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal";
}

export function useExtensionSlot(options: UseExtensionSlotOptions) {
  const store = useExtensionUIStore();
  const { slotId } = options;
  const contextRef = options.context ?? ref({});

  const slotSnapshot = computed(() => store.slotsById.get(slotId) ?? null);
  const contributions = computed<UIContributionSummary[]>(() => store.getVisibleContributions(slotId));
  const fallback = computed(() => {
    if (options.fallback) return options.fallback;
    return slotSnapshot.value?.fallbackPolicy ?? "empty";
  });
  const layout = computed(() => options.layout ?? slotSnapshot.value?.layout ?? "stack");
  const isEmpty = computed(() => contributions.value.length === 0);
  const hasError = computed(() => store.errors.some((e) => e.slotId === slotId));
  const slotErrors = computed(() => store.errors.filter((e) => e.slotId === slotId));

  function buildContext(): SlotContext {
    const theme = { mode: "light" as const, density: "comfortable" as const };
    return {
      slotId,
      contractVersion: slotSnapshot.value?.contractVersion ?? 1,
      platform: detectPlatform(),
      theme,
      locale: detectLocale(),
      data: { ...contextRef.value },
    };
  }

  function detectPlatform(): "windows" | "macos" | "linux" | "web" {
    if (typeof navigator === "undefined") return "web";
    const ua = navigator.userAgent.toLowerCase();
    if (ua.includes("win")) return "windows";
    if (ua.includes("mac")) return "macos";
    if (ua.includes("linux")) return "linux";
    return "web";
  }

  function detectLocale(): string {
    if (typeof navigator === "undefined") return "en";
    return navigator.language || "en";
  }

  function reportError(contributionId: string, message: string, recoverable = true) {
    store.recordError({
      contributionId,
      slotId,
      message,
      timestamp: Date.now(),
      recoverable,
    });
  }

  function clearErrors(contributionId?: string) {
    store.clearErrors(contributionId);
  }

  function hideContribution(contributionId: string) {
    const existing = store.layoutPrefs[slotId] || {
      slotId,
      hiddenContributions: [],
      ordering: {},
      collapsed: false,
    };
    if (!existing.hiddenContributions.includes(contributionId)) {
      existing.hiddenContributions.push(contributionId);
    }
    store.updateLayoutPreference(slotId, existing);
  }

  function reorderContribution(contributionId: string, newOrder: number) {
    const existing = store.layoutPrefs[slotId] || {
      slotId,
      hiddenContributions: [],
      ordering: {},
      collapsed: false,
    };
    existing.ordering[contributionId] = newOrder;
    store.updateLayoutPreference(slotId, existing);
  }

  return {
    slotId,
    slotSnapshot,
    contributions,
    fallback,
    layout,
    isEmpty,
    hasError,
    slotErrors,
    buildContext,
    reportError,
    clearErrors,
    hideContribution,
    reorderContribution,
  };
}
