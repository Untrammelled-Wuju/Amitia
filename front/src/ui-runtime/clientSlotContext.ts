import { computed, inject, unref, type ComputedRef, type InjectionKey } from "vue";
import type { UIPluginContext } from "@/ui-runtime/uiContext";

export interface ClientContributionRuntimeContext<TStore = unknown, TInjected extends Record<string, unknown> = Record<string, unknown>> {
  readonly pluginId: string;
  readonly slotId: string;
  readonly contributionId: string;
  readonly uiContext: UIPluginContext | ComputedRef<UIPluginContext>;
  readonly store?: TStore;
  readonly injected?: Readonly<TInjected>;
  readonly parent: ClientContributionRuntimeContext | null;
}

export const CLIENT_SLOT_RUNTIME_CONTEXT = Symbol("amitia-client-slot-runtime") as InjectionKey<ClientContributionRuntimeContext>;

export function useClientSlotRuntimeContext<TStore = unknown, TInjected extends Record<string, unknown> = Record<string, unknown>>() {
  return inject(CLIENT_SLOT_RUNTIME_CONTEXT, null) as ClientContributionRuntimeContext<TStore, TInjected> | null;
}

export function useClientSlotStore<TStore = unknown>(): TStore | undefined {
  return useClientSlotRuntimeContext<TStore>()?.store;
}

export function useClientSlotInjected<TInjected extends Record<string, unknown> = Record<string, unknown>>(): Readonly<TInjected> | undefined {
  return useClientSlotRuntimeContext<unknown, TInjected>()?.injected;
}

export function useParentClientSlotRuntimeContext(): ClientContributionRuntimeContext | null {
  return useClientSlotRuntimeContext()?.parent ?? null;
}

export function useClientUIContext(): UIPluginContext {
  const context = useClientSlotRuntimeContext()?.uiContext;
  if (!context) throw new Error("UI plugin context is unavailable");
  return unref(context);
}

export function useClientUIContextSelector<T>(
  selector: (context: UIPluginContext) => T,
  equality: (previous: T, next: T) => boolean = Object.is,
): ComputedRef<T> {
  let initialized = false;
  let selected: T;
  return computed(() => {
    const next = selector(useClientUIContext());
    if (initialized && equality(selected, next)) return selected;
    initialized = true;
    selected = next;
    return next;
  });
}
