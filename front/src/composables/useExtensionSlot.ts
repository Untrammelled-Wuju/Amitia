import { ref, computed, type Ref } from "vue";
import { useExtensionUIStore, type UIContributionSummary } from "@/stores/extensionUI";
import { useTheme } from "@/composables/useTheme";

export type ExtensionSurfaceRole = "header" | "status" | "sidebar" | "message" | "composer" | "main" | "overlay";

export interface ExtensionSurfaceContext {
  role: ExtensionSurfaceRole;
  width: number;
  height: number;
  breakpoint: "xs" | "sm" | "md" | "lg" | "xl";
}

export interface SlotContext {
  slotId: string;
  contractVersion: number;
  platform: "windows" | "macos" | "linux" | "web";
  theme: { mode: "light" | "dark"; density: "comfortable" | "compact" };
  locale: string;
  data: Record<string, unknown>;
  host: "web" | "desktop";
  os: "windows" | "macos" | "linux" | "unknown";
  surface: ExtensionSurfaceContext;
  capabilities: string[];
}

export interface UseExtensionSlotOptions {
  slotId: string;
  context?: Ref<Record<string, unknown>>;
  fallback?: "none" | "skeleton" | "empty" | "default";
  layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal";
  surface?: Ref<ExtensionSurfaceContext>;
}

export function useExtensionSlot(options: UseExtensionSlotOptions) {
  const store = useExtensionUIStore();
  const { slotId } = options;
  const contextRef = options.context ?? ref({});
  const { resolvedMode } = useTheme();

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
    const os = detectOS();
    const host = isDesktopShell() ? "desktop" : "web";
    const surface = options.surface?.value ?? {
      role: "main" as const,
      width: 0,
      height: 0,
      breakpoint: "xs" as const,
    };
    const capabilities = Array.isArray(contextRef.value.capabilities)
      ? contextRef.value.capabilities.filter((item): item is string => typeof item === "string")
      : [];
    return {
      slotId,
      contractVersion: slotSnapshot.value?.contractVersion ?? 1,
      platform: os === "unknown" ? "web" : os,
      theme: { mode: resolvedMode.value, density: surface.breakpoint === "xs" || surface.breakpoint === "sm" ? "compact" : "comfortable" },
      locale: detectLocale(),
      data: { ...contextRef.value },
      host,
      os,
      surface,
      capabilities,
    };
  }

  function detectOS(): "windows" | "macos" | "linux" | "unknown" {
    if (typeof navigator === "undefined") return "unknown";
    const ua = navigator.userAgent.toLowerCase();
    if (ua.includes("win")) return "windows";
    if (ua.includes("mac")) return "macos";
    if (ua.includes("linux")) return "linux";
    return "unknown";
  }

  function isDesktopShell() {
    return typeof window !== "undefined" && !!window.amitiaDesktop;
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
