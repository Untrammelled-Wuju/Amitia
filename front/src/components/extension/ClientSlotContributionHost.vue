<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  Fragment,
  h,
  inject,
  onScopeDispose,
  provide,
  readonly,
  shallowRef,
  watch,
  type ComputedRef,
  type VNode,
} from "vue";
import type { ClientObservable } from "@/ui-runtime/slotContract";
import { browserClientPluginRuntime, type ClientContributionInstance, type ClientSlotContribution } from "@/ui-runtime/clientPluginRuntime";
import { CLIENT_SLOT_RUNTIME_CONTEXT } from "@/ui-runtime/clientSlotContext";
import type { UIPluginContext } from "@/ui-runtime/uiContext";
import { useChatStore } from "@/stores/chat";
import { useAppStore } from "@/stores/app";
import { openContextMenu, type ContextMenuAction } from "@/components/context-menu/contextMenuStore";

const props = defineProps<{
  contribution: ClientSlotContribution;
  context: Record<string, unknown>;
  matched?: unknown;
  hookContext?: unknown;
  hocChildren?: VNode;
}>();

const RenderVNode = (props: { node: VNode }) => props.node;

// Dynamic import avoids a static module cycle: ExtensionSlot -> Host -> outlet.
const AuthorizedSlotOutlet = defineAsyncComponent(() => import("./ExtensionSlot.vue"));
const parent = inject(CLIENT_SLOT_RUNTIME_CONTEXT, null);
const chatStore = useChatStore();
const appStore = useAppStore();

function currentSessionId(): string | undefined {
  const direct = props.context.sessionId ?? props.context.conversationId;
  if (typeof direct === "string" && direct.trim()) return direct.trim();
  const conversation = props.context.conversation;
  if (conversation && typeof conversation === "object") {
    const id = (conversation as { id?: unknown }).id;
    if (typeof id === "string" && id.trim()) return id.trim();
  }
  // Root-scoped entries can declare session children. Their render occurrence
  // may not itself carry a conversation object, so SessionProvider must fall
  // back to the framework-owned active conversation selection.
  const active = chatStore.currentConversationId;
  return typeof active === "string" && active.trim() ? active.trim() : undefined;
}

const sessionId = computed(currentSessionId);
const uiContext = computed(() => props.context as unknown as UIPluginContext);
const instance = computed<ClientContributionInstance>(() =>
  browserClientPluginRuntime.slots.acquireContributionInstance(props.contribution.contributionId, sessionId.value),
);

function useExternalSelector<T, Selected = T>(
  source: () => T,
  subscribe: () => ((listener: () => void) => () => void) | undefined,
  selector?: (value: T) => Selected,
  equality?: (previous: Selected, next: Selected) => boolean,
): ComputedRef<Selected> {
  const version = shallowRef(0);
  let unsubscribe: (() => void) | undefined;
  const bind = () => {
    unsubscribe?.();
    unsubscribe = subscribe()?.(() => { version.value += 1; });
  };
  bind();
  const stop = watch(instance, bind);
  onScopeDispose(() => {
    stop();
    unsubscribe?.();
    unsubscribe = undefined;
  });
  let initialized = false;
  let selectedValue: Selected;
  return computed(() => {
    version.value;
    const value = source();
    const next = selector ? selector(value) : value as unknown as Selected;
    if (initialized && equality?.(selectedValue, next)) return selectedValue;
    initialized = true;
    selectedValue = next;
    return next;
  });
}

function useStore<Selected = unknown>(
  selector?: (store: unknown) => Selected,
  equality?: (previous: Selected, next: Selected) => boolean,
): ComputedRef<Selected> {
  return useExternalSelector(
    () => instance.value.getSnapshot(),
    () => instance.value.subscribe,
    selector,
    equality,
  );
}

function isObservable(value: unknown): value is ClientObservable<unknown> {
  return !!value
    && typeof value === "object"
    && typeof (value as { getSnapshot?: unknown }).getSnapshot === "function"
    && typeof (value as { subscribe?: unknown }).subscribe === "function";
}

function upperFirst(value: string): string {
  return value ? value[0]!.toUpperCase() + value.slice(1) : value;
}

function bindInjectedFace(
  face: Readonly<Record<string, unknown>>,
  hookContext?: unknown,
): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(face)) {
    if (key !== "hooks") result[key] = value;
  }
  const hooks = face.hooks;
  if (hooks && typeof hooks === "object") {
    for (const [name, source] of Object.entries(hooks as Record<string, unknown>)) {
      if (isObservable(source)) {
        result[`use${upperFirst(name)}`] = <Selected = unknown>(
          selector?: (value: unknown) => Selected,
          equality?: (previous: Selected, next: Selected) => boolean,
        ) => useExternalSelector(
          () => source.getSnapshot(),
          () => source.subscribe.bind(source),
          selector,
          equality,
        );
        continue;
      }
      // Slot-level contextual hooks are pure factories: binding receives the
      // standard kit + opaque render-occurrence hookContext and returns the
      // actual hook. Registrant inject hooks stay observable-only.
      if (typeof source === "function") {
        const hook = source({
          slotId: props.contribution.slotId,
          pluginId: props.contribution.pluginId,
          contributionId: props.contribution.contributionId,
          sessionId: sessionId.value,
          useSession,
          useGlobal,
        }, hookContext);
        if (typeof hook !== "function") {
          throw new Error(`slot hook factory ${name} must return a function`);
        }
        result[`use${upperFirst(name)}`] = hook;
      }
    }
  }
  return result;
}

function renderAuthorizedSlot(
  slotId: string,
  owner: Record<string, unknown> = {},
  options: { entryKey?: string; only?: string; fallback?: unknown; hookContext?: unknown; chain?: boolean; overlay?: boolean } = {},
): VNode {
  browserClientPluginRuntime.slots.assertRenderAuthority(props.contribution.contributionId, slotId);
  const definition = browserClientPluginRuntime.slots.getDefinition(slotId);
  if (!definition) throw new Error(`authorized child slot ${slotId} no longer exists`);
  if (options.chain && definition.kind !== "chain") {
    throw new Error(`child slot ${slotId} is ${definition.kind ?? "list"}, not chain`);
  }
  if (!options.chain && definition.kind === "chain") {
    throw new Error(`child slot ${slotId} is chain; use renderSlotChain()`);
  }
  const slots = options.fallback === undefined ? undefined : { default: () => options.fallback };
  return h(AuthorizedSlotOutlet, {
    slotId,
    context: { ...props.context, ...owner },
    dispatchKey: options.entryKey,
    dispatchOnly: options.only,
    hookContext: options.hookContext,
    chainOverlay: options.chain === true && options.overlay === true,
    fallback: options.fallback === undefined ? undefined : "default",
    authorizedBy: props.contribution.contributionId,
    bare: true,
  }, slots);
}

function renderSlot(slotId: string, owner: Record<string, unknown> = {}, options?: { entryKey?: string; only?: string; fallback?: unknown; hookContext?: unknown }): VNode {
  return renderAuthorizedSlot(slotId, owner, options);
}

function renderSlotChain(slotId: string, owner: Record<string, unknown> = {}, options?: { fallback?: unknown; overlay?: boolean; hookContext?: unknown }): VNode {
  return renderAuthorizedSlot(slotId, owner, { ...options, chain: true });
}

function useContextMenu() {
  return {
    getTriggerProps(request: unknown) {
      return {
        onContextmenu(event: MouseEvent) {
          if (!event || typeof event.preventDefault !== 'function') return;
          event.preventDefault();
          event.stopPropagation();
          const descriptor = typeof request === 'function'
            ? (request as () => unknown)()
            : request;
          const input = descriptor && typeof descriptor === 'object'
            ? descriptor as Record<string, unknown>
            : {};
          const value = input.value && typeof input.value === 'object'
            ? input.value as Record<string, unknown>
            : {};
          const kind = String(input.kind || '');
          const items: ContextMenuAction[] = [];
          if (kind === 'file') {
            const path = String(value.path || input.path || '').trim();
            const ownerSession = String(value.sessionId || input.sessionId || sessionId.value || '').trim();
            if (path) {
              items.push(
                {
                  id: 'copy-file-path',
                  label: '复制文件路径',
                  action: async () => {
                    await navigator.clipboard.writeText(path);
                  },
                },
                {
                  id: 'open-file-workbench',
                  label: '在工作台打开文件',
                  action: () => {
                    window.dispatchEvent(new CustomEvent('amitia:workbench:open', {
                      detail: {
                        protocolVersion: 1,
                        conversationId: ownerSession,
                        kind: 'file',
                        resourceUri: path,
                        workspaceUri: '',
                        title: path.split('/').pop() || path,
                      },
                    }));
                  },
                },
              );
            }
          } else if (kind === 'url') {
            const url = String(value.url || input.url || '').trim();
            if (url) {
              items.push(
                {
                  id: 'open-url',
                  label: '在新窗口打开链接',
                  action: () => {
                    window.open(url, '_blank', 'noopener,noreferrer');
                  },
                },
                {
                  id: 'copy-url',
                  label: '复制链接地址',
                  action: async () => {
                    await navigator.clipboard.writeText(url);
                  },
                },
              );
            }
          }
          if (!items.length) {
            const serialized = JSON.stringify(descriptor ?? null);
            items.push({
              id: 'copy-context',
              label: '复制上下文',
              action: async () => {
                await navigator.clipboard.writeText(serialized);
              },
            });
          }
          openContextMenu({
            x: event.clientX,
            y: event.clientY,
            title: kind === 'file' ? '文件操作' : kind === 'url' ? '链接操作' : '上下文操作',
            items,
          });
        },
      };
    },
  };
}

function sessionSnapshot(): Record<string, unknown> | undefined {
  const conversation = props.context.conversation;
  if (conversation && typeof conversation === "object") return conversation as Record<string, unknown>;
  const id = sessionId.value;
  if (!id) return undefined;
  const stored = appStore.conversations.find((item) => item.id === id);
  if (stored) return stored as unknown as Record<string, unknown>;
  return { ...props.context, id };
}

function useComputedSelector<T, Selected = T>(
  source: () => T,
  selector?: (value: T) => Selected,
  equality?: (previous: Selected, next: Selected) => boolean,
): ComputedRef<Selected> {
  let initialized = false;
  let selectedValue: Selected;
  return computed(() => {
    const value = source();
    const next = selector ? selector(value) : value as unknown as Selected;
    if (initialized && equality?.(selectedValue, next)) return selectedValue;
    initialized = true;
    selectedValue = next;
    return next;
  });
}

function useSession<Selected = Record<string, unknown> | undefined>(
  selector?: (session: Record<string, unknown> | undefined) => Selected,
  equality?: (previous: Selected, next: Selected) => boolean,
): ComputedRef<Selected> {
  return useComputedSelector(sessionSnapshot, selector, equality);
}

function useGlobal<Selected = Record<string, unknown>>(
  selector?: (global: Record<string, unknown>) => Selected,
  equality?: (previous: Selected, next: Selected) => boolean,
): ComputedRef<Selected> {
  return useComputedSelector(() => (props.context.global && typeof props.context.global === "object")
    ? props.context.global as Record<string, unknown>
    : props.context, selector, equality);
}

function SessionProvider(area: { children: (sessionId: string) => unknown; empty?: () => unknown }): unknown {
  const id = sessionId.value;
  if (!id) return area.empty?.() ?? null;
  // Keying the session body forces a clean subtree boundary when the active
  // session changes, while framework-owned slot stores remain session-persistent.
  return h(Fragment, { key: `slot-session:${id}` }, [area.children(id)] as any);
}

const commonInjectedFace = computed(() => bindInjectedFace(
  browserClientPluginRuntime.slots.getDefinition(props.contribution.slotId)?.commonInject ?? {},
  props.hookContext,
));
const injectedFace = computed(() => bindInjectedFace(instance.value.injected));
const localeTranslator = computed(() => {
  const runtime = browserClientPluginRuntime.locale;
  runtime.revision.value;
  const namespace = props.contribution.localeNamespace;
  if (!namespace) return undefined;
  if (runtime.has(namespace)) return runtime.bind(namespace);
  console.error(
    `[extension] locale namespace ${namespace} is not registered for ${props.contribution.contributionId}`,
    new Error(`locale namespace ${namespace} is not registered`),
  );
  return (key: string, params?: Record<string, unknown>) => {
    let value = key;
    for (const [name, raw] of Object.entries(params ?? {})) {
      value = value.replaceAll(`{${name}}`, String(raw));
    }
    return value;
  };
});
const componentProps = computed<Record<string, unknown>>(() => ({
  // Registration ownerDefaults are defaults only; render occurrence owner props win.
  ...(props.contribution.props ?? {}),
  ...props.context,
  ...commonInjectedFace.value,
  ...injectedFace.value,
  slotId: props.contribution.slotId,
  pluginId: props.contribution.pluginId,
  contributionId: props.contribution.contributionId,
  sessionId: sessionId.value,
  uiContext: uiContext.value,
  useStore,
  actions: instance.value.actions,
  useSession,
  useGlobal,
  SessionProvider,
  renderSlot,
  renderSlotChain,
  useContextMenu,
  ...(localeTranslator.value ? { t: localeTranslator.value } : {}),
  ...(props.matched !== undefined ? { matched: props.matched } : {}),
}));

provide(CLIENT_SLOT_RUNTIME_CONTEXT, {
  pluginId: props.contribution.pluginId,
  slotId: props.contribution.slotId,
  contributionId: props.contribution.contributionId,
  uiContext,
  parent,
});
</script>

<template>
  <component
    :is="contribution.component"
    v-bind="componentProps"
  >
    <RenderVNode v-if="hocChildren" :node="hocChildren" />
  </component>
</template>
