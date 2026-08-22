import { inject, type InjectionKey } from "vue";

export interface ClientContributionRuntimeContext<TStore = unknown, TInjected extends Record<string, unknown> = Record<string, unknown>> {
  readonly pluginId: string;
  readonly slotId: string;
  readonly contributionId: string;
  readonly store: TStore;
  readonly injected: Readonly<TInjected>;
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
