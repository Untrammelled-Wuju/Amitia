import { ref, computed, type Ref } from "vue";
import { useExtensionUIStore, type UIContributionSummary } from "@/stores/extensionUI";
import { useTheme } from "@/composables/useTheme";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import { browserClientPluginRuntime } from "@/ui-runtime/clientPluginRuntime";

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
  scope: "root" | "session-maybe" | "session";
  sessionId?: string;
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
  const contextRef = options.context ?? ref<Record<string, unknown>>({});
  const { resolvedMode } = useTheme();

  const slotSnapshot = computed(() => store.slotsById.get(slotId) ?? null);
  const clientSlotDefinition = computed(() => {
    browserClientPluginRuntime.slots.revision.value;
    return browserClientPluginRuntime.slots.getDefinition(slotId) ?? null;
  });
  const contributions = computed<UIContributionSummary[]>(() => {
    const env = resolveHostEnvironment();
    return store.getVisibleContributions(slotId, {
      ...contextRef.value,
      platform: env.platform,
      host: env.host,
      os: env.os,
    });
  });
  const fallback = computed(() => {
    if (options.fallback) return options.fallback;
    return slotSnapshot.value?.fallbackPolicy ?? clientSlotDefinition.value?.fallbackPolicy ?? "empty";
  });
  const layout = computed(() => options.layout ?? slotSnapshot.value?.layout ?? clientSlotDefinition.value?.layout ?? "stack");
  const scope = computed<"root" | "session-maybe" | "session">(() =>
    slotSnapshot.value?.scope ?? clientSlotDefinition.value?.scope ?? "root"
  );
  const sessionId = computed(() => {
    const value = contextRef.value.sessionId ?? contextRef.value.conversationId;
    return typeof value === "string" ? value.trim() : "";
  });
  const scopeReady = computed(() => scope.value !== "session" || sessionId.value.length > 0);
  const isEmpty = computed(() => contributions.value.length === 0);
  const hasError = computed(() => store.errors.some((e) => e.slotId === slotId));
  const slotErrors = computed(() => store.errors.filter((e) => e.slotId === slotId));

  function buildContext(): SlotContext {
    const env = resolveHostEnvironment();
    const surface = options.surface?.value ?? {
      role: "main" as const,
      width: 0,
      height: 0,
      breakpoint: "xs" as const,
    };
    const capabilities = Array.isArray(contextRef.value.capabilities)
      ? (contextRef.value.capabilities as unknown[]).filter((item): item is string => typeof item === "string")
      : [];
    return {
      slotId,
      contractVersion: slotSnapshot.value?.contractVersion ?? clientSlotDefinition.value?.contractVersion ?? 1,
      platform: env.platform,
      theme: { mode: resolvedMode.value, density: surface.breakpoint === "xs" || surface.breakpoint === "sm" ? "compact" : "comfortable" },
      locale: detectLocale(),
      data: { ...contextRef.value },
      host: env.host,
      os: env.os,
      surface,
      capabilities,
      scope: scope.value,
      ...(sessionId.value ? { sessionId: sessionId.value } : {}),
    };
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
    clientSlotDefinition,
    contributions,
    scope,
    sessionId,
    scopeReady,
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
