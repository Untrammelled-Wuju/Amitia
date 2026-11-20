import * as React from "react";
import * as ReactJSXRuntime from "react/jsx-runtime";
import * as ReactDOM from "react-dom";
import { createRoot } from "react-dom/client";
import * as DshPrimitives from "@deepseek-ai/dsh-client-ui-primitives";
import * as DshClientStore from "@deepseek-ai/dsh-client-store";
import * as DshClientSlots from "@deepseek-ai/dsh-client-ui-slots";
import * as DshDockkit from "@deepseek-ai/dsh-client-ui-dockkit";
import * as Zod from "zod";
import * as TanStackReactVirtual from "@tanstack/react-virtual";
import {
  DshClientEventBus,
  DshClientModuleLoader,
  installDshModuleLoader,
  loadDshClientPlugin,
} from "../client-abi/module-loader.mjs";
import { createDshClientJobsController } from "../client-abi/jobs-controller.mjs";
import { createDshClientResourceRegistry } from "../client-abi/resource-registry.mjs";
import { registerDshConversationDefinition } from "../client-abi/conversation-adapter.mjs";
import { REACT_ADAPTER_ID, registerReactAdapter } from "../client-abi/react-adapter.mjs";
import { hasDshSlot, mapDshSlot as resolveDshSlot } from "../client-abi/slot-map.mjs";
import { createDshClientRuntimeServices } from "../client-abi/runtime-services.mjs";
import { createDshConfigForms } from "../client-abi/config-forms.mjs";
import { createDshFileDragActions } from "../client-abi/file-drag.mjs";
import { createDshSettingsSchema } from "../client-abi/settings-schema.mjs";
import { createConversationData, registerConversationData } from "./conversation-data.mjs";

Zod.config({ jitless: true });

const reactAdapterId = `${REACT_ADAPTER_ID}-${Math.random().toString(36).slice(2)}`;

const CORDIS_SHIM = Object.freeze({
  Service: class Service {
    constructor(context, name) {
      this.ctx = context;
      this.name = name;
      if (context && typeof context.provide === "function" && text(name)) context.provide(text(name), this);
    }
  },
  Context: class Context {},
});
const INVARIANTS_SHIM = Object.freeze({
  invariant(condition, message) {
    if (!condition) throw new Error(String(message || "invariant failed"));
  },
});
const BRAND_SHIM = Object.freeze({
  brandString(value) {
    return value;
  },
  brandNumber(value) {
    return value;
  },
  brandBoolean(value) {
    return value;
  },
});
const CRYPTO_SHIM = Object.freeze({
  randomUUID() {
    return globalThis.crypto.randomUUID();
  },
});

const style = document.createElement("style");
style.dataset.plugin = "com.amitia/dsh-compatible-host";
style.textContent = `
.dsh-host-page{display:flex;flex-direction:column;gap:14px;padding:16px;color:var(--el-text-color-primary)}
.dsh-host-page__title{font-size:18px;font-weight:650}
.dsh-host-page__row{display:flex;align-items:center;flex-wrap:wrap;gap:8px}
.dsh-host-page__input{min-width:320px;flex:1;padding:8px 10px;border:1px solid var(--surface-border);border-radius:6px;background:var(--el-bg-color);color:inherit}
.dsh-host-page__button{padding:7px 11px;border:1px solid var(--surface-border);border-radius:6px;background:transparent;color:inherit;cursor:pointer}
.dsh-host-page__button--primary{background:var(--el-color-primary);border-color:var(--el-color-primary);color:#fff}
.dsh-host-page__button--danger{color:var(--el-color-danger)}
.dsh-host-page__table{width:100%;border-collapse:collapse;font-size:12px}
.dsh-host-page__table th,.dsh-host-page__table td{padding:8px;border-bottom:1px solid var(--surface-border);text-align:left;vertical-align:top}
.dsh-host-page__actions{display:flex;gap:6px;flex-wrap:wrap}
.dsh-host-page__error{padding:10px;border:1px solid color-mix(in srgb,var(--el-color-danger) 35%,transparent);border-radius:6px;background:var(--el-color-danger-light-9);color:var(--el-color-danger);white-space:pre-wrap;font-family:monospace}
.dsh-react-mount{display:contents}
.dsh-vue-slot-bridge{display:contents}
`;
document.head.append(style);

function text(value) {
  return value == null ? "" : String(value).trim();
}

export async function clientRuntimeModuleURL(plugin, entry) {
  const pageModuleURL = new URL(import.meta.url);
  const marker = "/page-module/";
  const markerIndex = pageModuleURL.pathname.indexOf(marker);
  if (markerIndex < 0) {
    throw new Error(`Amitia page module URL is not a runtime module resource: ${pageModuleURL.href}`);
  }
  const parts = pageModuleURL.pathname.slice(markerIndex + marker.length).split("/");
  if (parts.length < 2) throw new Error(`Amitia page module URL has no extension and page identity: ${pageModuleURL.href}`);
  const extensionID = decodeURIComponent(parts[0]);
  const pageID = decodeURIComponent(parts[1]);
  const desktop = globalThis.window?.amitiaDesktop;
  let baseURL = globalThis.location.origin;
  let headers = { "Content-Type": "application/json" };
  let credentials = "include";
  if (typeof desktop?.getRuntimeEndpoint === "function") {
    const endpoint = await desktop.getRuntimeEndpoint();
    const origin = text(endpoint?.httpBaseURL);
    if (!origin) throw new Error("Amitia desktop runtime endpoint is unavailable");
    baseURL = origin;
    credentials = "omit";
    if (typeof desktop.getBackendAuthHeaders === "function") {
      Object.assign(headers, await desktop.getBackendAuthHeaders("local"));
    }
  }
  const response = await fetch(new URL("/api/extensions/ui/runtime-module-ticket", baseURL), {
    method: "POST",
    credentials,
    headers,
    body: JSON.stringify({
      extensionId: extensionID,
      pageId: pageID,
      pluginId: plugin.pluginId,
      entryId: entry.id,
    }),
  });
  if (!response.ok) {
    throw new Error(`Amitia runtime module ticket failed: ${response.status} ${await response.text()}`);
  }
  const ticket = await response.json();
  const path = text(ticket?.path || ticket?.data?.path);
  if (!path.startsWith("/api/public/extensions/ui/runtime-module/")) {
    throw new Error(`Amitia runtime module ticket returned invalid path: ${JSON.stringify(ticket)}`);
  }
  return new URL(path, baseURL).href;
}

function createDshSnapshotHandle(initial) {
  let snapshot = Object.freeze(initial);
  const listeners = new Set();
  return {
    getSnapshot() {
      return snapshot;
    },
    subscribe(listener) {
      if (typeof listener !== "function") throw new Error("DSH snapshot listener must be a function");
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    set(next) {
      snapshot = Object.freeze(next);
      for (const listener of [...listeners]) listener();
    },
    dispose() {
      listeners.clear();
    },
  };
}

async function resolveSlotInjectEffect(effect) {
  const disposers = [];
  try {
    const resolved = await effect;
    if (resolved == null) return disposers;
    if (typeof resolved === "function") {
      disposers.push(resolved);
      return disposers;
    }
    if (typeof resolved[Symbol.asyncIterator] !== "function" && typeof resolved[Symbol.iterator] !== "function") {
      throw new Error(`DSH slot inject effect yielded ${typeof resolved}, expected a disposer`);
    }
    for await (const item of resolved) {
      disposers.push(...await resolveSlotInjectEffect(await item));
    }
    return disposers;
  } catch (error) {
    for (let index = disposers.length - 1; index >= 0; index -= 1) {
      await disposers[index]?.();
    }
    throw error;
  }
}

export function rawError(error, seen = new WeakSet()) {
  if (!error || typeof error !== "object") return String(error);
  if (seen.has(error)) return "[Circular]";
  seen.add(error);
  if (error instanceof Error) {
    const parts = [`${error.name}: ${error.message}`, error.stack ?? ""];
    for (const key of Reflect.ownKeys(error)) {
      if (key === "name" || key === "message" || key === "stack") continue;
      try {
        parts.push(`${String(key)}: ${rawError(error[key], seen)}`);
      } catch (failure) {
        parts.push(`${String(key)}: ${String(failure)}`);
      }
    }
    return parts.filter(Boolean).join("\n");
  }
  if (Array.isArray(error)) return `[${error.map((value) => rawError(value, seen)).join(", ")}]`;
  const fields = [];
  for (const key of Reflect.ownKeys(error)) {
    try {
      fields.push(`${String(key)}: ${rawError(error[key], seen)}`);
    } catch (failure) {
      fields.push(`${String(key)}: ${String(failure)}`);
    }
  }
  return `{ ${fields.join(", ")} }`;
}

function ensureReactAdapter() {
  registerReactAdapter(globalThis.amitiaClientPlugins?.foreignComponents, reactAdapterId);
}

function VueSlotReactBridge(props) {
  const element = React.useRef(null);
  const renderVNode = React.useRef(props.renderVNode);
  renderVNode.current = props.renderVNode;
  React.useEffect(() => {
    const Vue = globalThis.__AMITIA_VUE__;
    const container = element.current;
    if (!container || !Vue) return undefined;
    Vue.render(renderVNode.current(), container);
    return () => {
      Vue.render(null, container);
    };
  }, [props.version]);
  return React.createElement("div", {
    ref: element,
    className: "dsh-vue-slot-bridge",
  });
}

function runtimeContributionInjected(componentProps) {
  const runtime = globalThis.amitiaClientPlugins?.slots;
  const contributionId = text(componentProps?.amitiaContributionId || componentProps?.contributionId);
  if (!runtime || !contributionId) return {};
  const contribution = runtime.contributionsById?.get(contributionId);
  if (!contribution) return {};
  const sessionId = text(componentProps?.amitiaSessionId || componentProps?.sessionId);
  const scopeKey = contribution.scope === "root"
    ? "root"
    : contribution.scope === "session"
      ? sessionId
      : sessionId || "root";
  return contribution.instances?.get(scopeKey)?.injected ?? {};
}

function authorizeChildRender(contributionId, dshSlotId) {
  if (!contributionId) return;
  const runtime = globalThis.amitiaClientPlugins?.slots;
  if (!runtime || typeof runtime.authorizeChildSlot !== "function") return;
  let mapped = text(dshSlotId);
  try {
    mapped = mapDshSlot(dshSlotId);
  } catch (error) {
    void error;
  }
  if (!mapped) return;
  runtime.authorizeChildSlot(contributionId, mapped);
}

function hookName(name) {
  const value = text(name);
  if (!value) throw new Error("DSH hook compartment member name is required");
  return `use${value.charAt(0).toUpperCase()}${value.slice(1)}`;
}

const DSH_EMPTY_LIST = Object.freeze([]);
const DSH_EMPTY_SESSION_SNAPSHOT = Object.freeze({
  id: "",
  items: DSH_EMPTY_LIST,
  status: new Map(),
  ready: false,
  blank: false,
  awaitingFirstTurn: false,
  running: false,
  promptAttempted: false,
  pendingSubmissions: DSH_EMPTY_LIST,
  openState: "closed",
  subagent: null,
  removed: false,
  promptError: null,
});
const DSH_EMPTY_CONVERSATION_SNAPSHOT = Object.freeze({
  views: Object.freeze({ get: () => undefined, grouped: () => undefined }),
  activeTargets: new Set(),
});
const DSH_EMPTY_NODE_SOURCE = Object.freeze({ getSnapshot: () => undefined, subscribe: () => () => {} });
const DSH_EMPTY_LIST_SOURCE = Object.freeze({ getSnapshot: () => DSH_EMPTY_LIST, subscribe: () => () => {} });
const DSH_EMPTY_BOTTOM_SOURCE = Object.freeze({ getSnapshot: () => false, subscribe: () => () => {} });
const DSH_EMPTY_CHAT_SNAPSHOT = Object.freeze({
  order: DSH_EMPTY_LIST,
  nodes: Object.freeze({
    get: () => undefined,
    source: () => DSH_EMPTY_NODE_SOURCE,
    bottomSource: () => DSH_EMPTY_BOTTOM_SOURCE,
    turnDataSource: () => DSH_EMPTY_LIST_SOURCE,
    processSource: () => DSH_EMPTY_NODE_SOURCE,
    values: () => DSH_EMPTY_LIST,
  }),
  locations: Object.freeze({ getTurn: () => DSH_EMPTY_LIST, getStep: () => DSH_EMPTY_LIST }),
  navigation: Object.freeze({ items: () => DSH_EMPTY_LIST }),
  timeline: Object.freeze({ turnOrder: DSH_EMPTY_LIST, turns: new Map() }),
  legacy: Object.freeze({
    nodes: DSH_EMPTY_LIST,
    turnTimings: new Map(),
    turnEnds: new Map(),
    partial: null,
    runningCalls: DSH_EMPTY_LIST,
  }),
});

function emptyChatSnapshot() {
  return DSH_EMPTY_CHAT_SNAPSHOT;
}

let dshActiveSessionId = "";

export function setDshActiveSession(sessionId) {
  dshActiveSessionId = text(sessionId) || "";
}

const dshSlotGeneration = (() => {
  let value = 0;
  const listeners = new Set();
  return {
    getSnapshot: () => value,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    bump: () => {
      value += 1;
      for (const listener of [...listeners]) listener();
    },
  };
})();

globalThis.__dshBumpSlots = () => dshSlotGeneration.bump();

function DshReactRuntimeBridge(props) {
  const sourceProps = props.componentProps ?? props;
  const resources = props.resources ?? sourceProps.resources;
  const componentProps = {
    ...runtimeContributionInjected(sourceProps),
    ...sourceProps,
  };
  if (componentProps.sessionId === undefined && dshActiveSessionId) {
    componentProps.sessionId = dshActiveSessionId;
  }
  if (typeof componentProps.useSessionAccess !== "function") {
    componentProps.useSessionAccess = (sessionId, capability) => {
      void sessionId;
      void capability;
      return true;
    };
  }
  if (typeof componentProps.useAccess !== "function") {
    componentProps.useAccess = (capability) => {
      void capability;
      return true;
    };
  }
  if (typeof componentProps.useWorkspaceAccess !== "function") {
    componentProps.useWorkspaceAccess = (capability) => {
      void capability;
      return true;
    };
  }
  const renderingContributionId = text(componentProps.amitiaContributionId);
  const originalRenderers = {};
  for (const method of ["renderSlot", "renderSlotChain", "renderFactorySlot"]) {
    const baseRenderer = componentProps[method];
    if (typeof baseRenderer !== "function") continue;
    originalRenderers[method] = baseRenderer;
    componentProps[method] = (slotId, owner, options) => {
      if (method === "renderFactorySlot") {
        const target = resolveFactorySlotTarget(slotId);
        if (!target) return null;
        authorizeChildRender(renderingContributionId, target);
        const definition = globalThis.amitiaClientPlugins?.slots?.getDefinition(target);
        return renderMappedSlot(slotId, owner, options, definition?.kind === "chain", target);
      }
      authorizeChildRender(renderingContributionId, slotId);
      return renderMappedSlot(slotId, owner, options, method === "renderSlotChain");
    };
  }
  const commonInjectFace = dshCommonInjects.get(text(componentProps.slotId));
  if (commonInjectFace) {
    componentProps.hooks = { ...(commonInjectFace.hooks ?? {}), ...(componentProps.hooks ?? {}) };
    if (commonInjectFace.keyedHooks) {
      componentProps.keyedHooks = { ...commonInjectFace.keyedHooks, ...(componentProps.keyedHooks ?? {}) };
    }
  }
  const selectSnapshot = (value, selector) => typeof selector === "function" ? selector(value) : value;
  const useInjectedHook = (source) => React.useSyncExternalStore(
    typeof source?.subscribe === "function" ? source.subscribe.bind(source) : () => () => {},
    typeof source?.getSnapshot === "function" ? source.getSnapshot.bind(source) : () => source,
  );
  const injectedHookProps = {};
  const slotHookContext = componentProps.hookContext;
  const hookFactoryStandard = {
    sessionId: text(componentProps.amitiaSessionId || componentProps.sessionId || globalThis.__dshCurrentSessionId) || undefined,
    slotId: componentProps.slotId,
    pluginId: componentProps.pluginId,
    contributionId: componentProps.amitiaContributionId,
  };
  for (const [name, source] of Object.entries(componentProps.hooks ?? {})) {
    const key = hookName(name);
    const observable = source
      && typeof source === "object"
      && typeof source.getSnapshot === "function"
      && typeof source.subscribe === "function";
    if (typeof componentProps[key] === "function" && !observable) continue;
    if (typeof source === "function" && !observable) {
      const hook = source(hookFactoryStandard, slotHookContext);
      if (typeof hook !== "function") {
        throw new Error(`DSH slot hook factory ${name} returned ${typeof hook}, expected a Hook`);
      }
      injectedHookProps[key] = hook;
      continue;
    }
    injectedHookProps[key] = (selector) => selectSnapshot(useInjectedHook(source), selector);
  }
  for (const [name, resolver] of Object.entries(componentProps.keyedHooks ?? {})) {
    const key = hookName(name);
    if (typeof componentProps[key] === "function" || typeof injectedHookProps[key] === "function") continue;
    injectedHookProps[key] = (sourceKey, selector) => {
      const source = typeof resolver === "function" ? resolver(text(sourceKey)) : undefined;
      const value = useInjectedHook(source);
      return typeof selector === "function" ? selector(value) : value;
    };
  }
  const standardHookProps = {};
  const componentSessionId = text(componentProps.amitiaSessionId || componentProps.sessionId || globalThis.__dshCurrentSessionId);
  const sessionSources = componentSessionId ? dshSessionSources.binding(componentSessionId).getSnapshot() : undefined;
  for (const [name, source] of Object.entries(sessionSources?.hooks ?? {})) {
    standardHookProps[hookName(name)] = (selector) => selectSnapshot(useInjectedHook(source), selector);
  }
  for (const [name, resolver] of Object.entries(sessionSources?.keyedHooks ?? {})) {
    standardHookProps[hookName(name)] = (sourceKey, selector) => {
      const source = typeof resolver === "function" ? resolver(text(sourceKey)) : undefined;
      const value = useInjectedHook(source);
      return typeof selector === "function" ? selector(value) : value;
    };
  }
  const conversationData = componentSessionId ? conversationDataForSession(componentSessionId) : null;
  const objectProp = (value) => (value && typeof value === "object" && !Array.isArray(value) ? value : null);
  const chatSource = objectProp(componentProps.chat) ?? conversationData?.chat ?? null;
  const sessionSource = objectProp(componentProps.session) ?? conversationData?.session ?? null;
  const conversationSource = objectProp(componentProps.conversation) ?? conversationData?.conversation ?? null;
  const dshStore = dshStoreInstanceFor(text(componentProps.amitiaContributionId), componentSessionId);
  const useStore = (selector) => {
    const state = useInjectedHook(dshStore) ?? {};
    return typeof selector === "function" ? selector(state) : state;
  };
  const pickHook = (name, fallback) => standardHookProps[name] ?? injectedHookProps[name] ?? fallback;
  const useChat = pickHook("useChat", (selector) => selectSnapshot(
    useInjectedHook(chatSource) ?? conversationData?.chat.getSnapshot() ?? emptyChatSnapshot(),
    selector,
  ));
  const useSession = pickHook("useSession", (selector) => selectSnapshot({
    ...DSH_EMPTY_SESSION_SNAPSHOT,
    ...(useInjectedHook(sessionSource) ?? conversationData?.session.getSnapshot() ?? {}),
  }, selector));
  const useConversation = pickHook("useConversation", (selector) => selectSnapshot(
    useInjectedHook(conversationSource) ?? conversationData?.conversation.getSnapshot() ?? DSH_EMPTY_CONVERSATION_SNAPSHOT,
    selector,
  ));
  const useContextMenu = pickHook("useContextMenu", () => ({
    getTriggerProps: () => () => undefined,
  }));
  const normalizeSlotOwner = (input) => {
    if (!input || typeof input !== "object" || Array.isArray(input)) return input;
    const normalized = { ...input };
    if (normalized.session && typeof normalized.session === "object") {
      const id = text(normalized.session.sessionId || normalized.session.id);
      if (id) normalized.session = id;
      else delete normalized.session;
    }
    if (!text(normalized.sessionId) && typeof normalized.session === "string") normalized.sessionId = normalized.session;
    return normalized;
  };
  const renderMappedSlot = (slotId, owner, options, chain = false, mappedOverride = "") => {
    const mapped = mappedOverride || tryMapDshSlot(slotId);
    if (!mapped) return null;
    authorizeChildRender(renderingContributionId, mapped);
    const renderer = originalRenderers[chain ? "renderSlotChain" : "renderSlot"]
      ?? componentProps[chain ? "renderSlotChain" : "renderSlot"];
    if (typeof renderer !== "function") {
      throw new Error(`Amitia ${chain ? "chain " : ""}slot renderer is unavailable for ${slotId}`);
    }
    const normalizedOwner = normalizeSlotOwner(owner);
    const resolvedOwnerProps = (() => {
      const resolvedSessionId = text(normalizedOwner?.sessionId || normalizedOwner?.session?.sessionId);
      if (!resolvedSessionId) return {};
      const resolved = dshSessionSources.binding(resolvedSessionId).getSnapshot();
      return resolved && resolved.props && typeof resolved.props === "object" ? resolved.props : {};
    })();
    const ownerWithAccess = normalizedOwner && typeof normalizedOwner === "object" && !Array.isArray(normalizedOwner)
      ? {
        ...resolvedOwnerProps,
        useAccess: componentProps.useAccess,
        useSessionAccess: componentProps.useSessionAccess,
        useWorkspaceAccess: componentProps.useWorkspaceAccess,
        inputActions: resolvedOwnerProps.inputActions ?? normalizedOwner.inputActions ?? createInputActions(),
        ...(options && typeof options === "object" && options.hookContext !== undefined ? { hookContext: options.hookContext } : {}),
        ...normalizedOwner,
      }
      : normalizedOwner;
    const rawSlotOptions = options && typeof options === "object" ? options : {};
    const originIds = [];
    for (const [contributionId, originSlot] of dshContributionOrigins) {
      if (originSlot === slotId) originIds.push(contributionId);
    }
    const slotOptions = originIds.length > 0 && !rawSlotOptions.only && !rawSlotOptions.entryKey
      ? { ...rawSlotOptions, only: originIds }
      : rawSlotOptions;
    const slotFallback = slotOptions.fallback ?? slotOptions.children;
    const slotRuntime = globalThis.amitiaClientPlugins?.slots;
    const slotContributionCount = slotRuntime && typeof slotRuntime.listContributions === "function"
      ? slotRuntime.listContributions(mapped).length
      : 0;
    const slotDeclaredFallback = slotOptions.fallback ?? slotOptions.children;
    if (slotContributionCount === 0 && slotDeclaredFallback !== undefined && slotDeclaredFallback !== null) {
      const debug = globalThis.__dshSlotDebug ?? (globalThis.__dshSlotDebug = []);
      if (debug.length < 60) debug.push({ slot: mapped, hasChildren: slotOptions.children !== undefined, hasFallback: slotOptions.fallback !== undefined, type: typeof slotDeclaredFallback });
      return slotDeclaredFallback;
    }
    const hostDefinition = globalThis.amitiaClientPlugins?.slots?.getDefinition(mapped);
    const vueOptions = (() => {
      const base = slotOptions && typeof slotOptions === "object" ? { ...slotOptions } : {};
      const dispatchEntryKey = text(base.entryKey);
      if (dispatchEntryKey && !text(base.only) && hostDefinition && hostDefinition.kind !== "keyed") {
        base.only = dispatchEntryKey;
      }
      const candidate = base.fallback ?? base.children;
      if (!isReactElement(candidate)) {
        delete base.children;
        return base;
      }
      const Vue = globalThis.__AMITIA_VUE__;
      if (!Vue) return base;
      const next = { ...base, fallback: Vue.h(reactElementOutletComponent(), { element: candidate }) };
      delete next.children;
      return next;
    })();
    const effectiveChain = chain && hostDefinition?.kind === "chain";
    const chainRenderer = effectiveChain ? renderer : (originalRenderers.renderSlot ?? componentProps.renderSlot ?? renderer);
    const rendered = chainRenderer(mapped, ownerWithAccess, vueOptions);
    const emptyRendered = rendered === undefined || rendered === null || rendered === false;
    const fallbackRendered = slotFallback === undefined || slotFallback === null || slotFallback === false ? undefined : slotFallback;
    if (emptyRendered && fallbackRendered !== undefined) return fallbackRendered;
    if (rendered && typeof rendered === "object" && rendered.__v_isVNode) {
      return React.createElement(VueSlotReactBridge, {
        renderVNode: () => rendered,
        version: rendered,
      });
    }
    return rendered;
  };
  const renderSlot = (slotId, owner = {}, options = {}) =>
    renderMappedSlot(slotId, owner, options);
  const renderSlotChain = (slotId, owner = {}, options = {}) =>
    renderMappedSlot(slotId, owner, options, true);
  const renderFactorySlot = (slotId, owner = {}, options = {}) => {
    const normalizedOwner = owner && typeof owner === "object" && !Array.isArray(owner)
      ? {
        ...owner,
        ...(owner.facts && typeof owner.facts === "object"
          ? { facts: { sessionLabels: [], skillNames: [], ...owner.facts } }
          : { facts: { sessionLabels: [], skillNames: [] } }),
      }
      : owner;
    owner = normalizedOwner;
    const mapped = resolveFactorySlotTarget(slotId);
    if (!mapped) return null;
    const definition = globalThis.amitiaClientPlugins?.slots?.getDefinition(mapped);
    const factoryId = dshFactoryContributionIds.get(text(slotId));
    const withDispatch = factoryId && definition?.kind !== "keyed" && !text(options?.only) && !text(options?.entryKey)
      ? { ...(options && typeof options === "object" ? options : {}), only: factoryId }
      : options;
    const stampedOwner = factoryId && owner && typeof owner === "object" && !Array.isArray(owner)
      ? { ...owner, __dshFactory: factoryId }
      : owner;
    return renderMappedSlot(slotId, stampedOwner, withDispatch, definition?.kind === "chain", mapped);
  };
  const useResource = (address) => {
    const value = text(address);
    const snapshot = React.useSyncExternalStore(
      (listener) => resources.subscribe(value, listener),
      () => resources.getSnapshot(value),
    );
    React.useEffect(() => resources.watch(value), [resources, value]);
    return snapshot;
  };
  const useTabInfo = () => ({
    tab: {
      navigation: {
        address: componentProps.address,
        params: componentProps.params,
      },
      title: componentProps.rightPane?.title,
    },
  });
  const useProjection = () => undefined;
  const useRemoteAccess = () => false;
  const defaultInputSnapshot = {
    draft: "",
    attachmentIds: [],
    draftRev: 0,
    phase: "plain",
    queue: [],
    attachments: [],
    command: undefined,
    quotes: [],
    occurrences: [],
    pendingSubmissions: [],
    persistDraft: () => {},
    requestDraftInitialization: () => "preserved",
    setDraft: () => {},
    dispose: () => {},
  };
  const fallbackInput = (selector) => selectSnapshot(
    componentProps.input && typeof componentProps.input === "object"
      ? { ...defaultInputSnapshot, ...componentProps.input }
      : defaultInputSnapshot,
    selector,
  );
  const useInput = (selector) => {
    const source = standardHookProps.useInput ?? injectedHookProps.useInput;
    if (typeof source !== "function") return fallbackInput(selector);
    const state = source((value) => (value && typeof value === "object" ? value : defaultInputSnapshot));
    return typeof selector === "function" ? selector(state) : state;
  };
  const statusMap = (value) => (value instanceof Map ? value : new Map());
  const useSessionStatus = (selector) => selectSnapshot(statusMap(componentProps.session?.status), selector);
  const useLiveCatalog = (name, fallback, selector) => {
    const source = globalThis.__dshLiveCatalogs?.[name];
    const isSource = Boolean(source) && typeof source?.getSnapshot === "function" && typeof source?.subscribe === "function";
    const value = React.useSyncExternalStore(
      isSource ? source.subscribe : () => () => {},
      isSource ? source.getSnapshot : () => fallback,
    );
    return typeof selector === "function" ? selector(value) : value;
  };
  const useSessions = (selector) => useLiveCatalog("sessions", componentProps.sessions ?? { items: [], ready: false }, selector);
  const useWorkspaces = (selector) => useLiveCatalog("workspaces", componentProps.workspaces ?? { items: [], selectedId: null, ready: false }, selector);
  if (!componentProps.workspaces || typeof componentProps.workspaces !== "object") {
    componentProps.workspaces = { items: [], selectedId: null, ready: false };
  }
  if (!componentProps.sessions || typeof componentProps.sessions !== "object") {
    componentProps.sessions = { items: [], byId: {}, ready: false };
  }
  const sessionsBridge = componentProps.sessions;
  if (!sessionsBridge.byId || typeof sessionsBridge.byId !== "object" || Array.isArray(sessionsBridge.byId)) {
    sessionsBridge.byId = {};
  }
  if (typeof sessionsBridge.refreshProjections !== "function") {
    sessionsBridge.refreshProjections = async (sessionId) => {
      void sessionId;
      return [];
    };
  }
  if (typeof sessionsBridge.list !== "object" || sessionsBridge.list === null) {
    sessionsBridge.list = { getSnapshot: () => ({ items: sessionsBridge.items ?? [], byId: sessionsBridge.byId ?? {}, ready: false }), subscribe: () => () => {} };
  }
  componentProps.session = {
    ...DSH_EMPTY_SESSION_SNAPSHOT,
    ...componentProps.session,
    status: statusMap(componentProps.session?.status),
  };
  componentProps.conversation = componentProps.conversation && typeof componentProps.conversation === "object"
    ? { activeTargets: new Set(), ...componentProps.conversation }
    : { activeTargets: new Set() };
  if (!componentProps.navigation || typeof componentProps.navigation !== "object") {
    componentProps.navigation = {};
  }
  if (typeof componentProps.navigation.forkSession !== "function") {
    componentProps.navigation.forkSession = async (sessionId) => {
      throw new Error(`DSH forkSession 尚未接入宿主会话服务: ${sessionId ?? ""}`);
    };
  }
  const factoryLocalComponentKit = () => ({
    ...componentProps,
    ...injectedHookProps,
    ...standardHookProps,
    renderSlot,
    renderSlotChain,
    renderFactorySlot,
    useFactorySlot,
    useResource,
    useTabInfo,
    useProjection,
    useRemoteAccess,
    useContextMenu,
    useStore,
    actions: dshStore?.actions ?? componentProps.actions,
    useChat,
    useInput,
    useSession,
    useConversation,
    useSessionStatus,
    useSessions,
    useWorkspaces,
    usePanelInfo,
    useRenameRequest: injectedHookProps.useRenameRequest ?? ((selector) => selectSnapshot(null, selector)),
    useArchiveRequest: injectedHookProps.useArchiveRequest ?? ((selector) => selectSnapshot(null, selector)),
  });
  const useFactorySlot = (slotName, fallback) => {
    const factory = componentProps?.factorySlots?.[text(slotName)];
    const Local = typeof factory === "function" ? factory : fallback;
    return React.useMemo(() => {
      if (typeof Local !== "function") return Local;
      return function BoundFactoryLocalComponent(localProps) {
        return React.createElement(Local, { ...factoryLocalComponentKit(), ...(localProps ?? {}) });
      };
    }, [Local]);
  };
  const usePanelInfo = (selector) => selectSnapshot(componentProps.panel, selector);
  return React.createElement(props.component, {
    ...componentProps,
    ...injectedHookProps,
    ...standardHookProps,
    useRenameRequest: injectedHookProps.useRenameRequest ?? ((selector) => selectSnapshot(null, selector)),
    useArchiveRequest: injectedHookProps.useArchiveRequest ?? ((selector) => selectSnapshot(null, selector)),
    renderSlot,
    renderSlotChain,
    renderFactorySlot,
    useFactorySlot,
    useResource,
    useTabInfo,
    useProjection,
    useRemoteAccess,
    useContextMenu,
    useStore,
    actions: dshStore?.actions ?? componentProps.actions,
    useChat,
    useInput,
    useSession,
    useConversation,
    useSessionStatus,
    useSessions,
    useWorkspaces,
    usePanelInfo,
  });
}

function createReactVueComponent(pluginId, reactComponent, resources, contributionId) {
  const Vue = globalThis.__AMITIA_VUE__;
  if (!Vue) throw new Error("Amitia Vue runtime is unavailable");
  return Vue.defineComponent({
    name: `DshReactBridge_${pluginId.replace(/[^a-zA-Z0-9_]/g, "_")}`,
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      const element = Vue.ref(null);
      const slotVersion = Vue.ref(0);
      let mounted = null;
      let active = true;
      const reactProps = () => {
        const current = {
          ...attrs,
          component: reactComponent,
          resources,
          amitiaContributionId: contributionId,
          amitiaSessionId: text(attrs.sessionId),
        };
        if (slots.default) {
          current.children = React.createElement(VueSlotReactBridge, {
            renderVNode: () => slots.default?.(),
            version: slotVersion.value,
          });
        }
        return current;
      };
      const syncHitArea = () => {
        const host = element.value;
        if (!host) return;
        host.style.pointerEvents = host.childElementCount === 0 ? "none" : "";
      };
      const mount = async () => {
        if (!element.value) throw new Error("DSH React bridge has no mount element");
        mounted = await globalThis.amitiaClientPlugins.foreignComponents.mount(reactAdapterId, {
          element: element.value,
          props: {
            component: DshReactRuntimeBridge,
            componentProps: reactProps(),
            resources,
          },
          context: attrs.uiContext,
          emit() {},
        });
      };
      const remount = async () => {
        if (!active || !element.value) return;
        try {
          await mounted?.unmount?.();
        } catch (error) {
          console.error("[original-error]", rawError(error));
        }
        mounted = null;
        if (!active) return;
        try {
          await mount();
          syncHitArea();
        } catch (error) {
          console.error("[original-error]", error);
        }
      };
      let unsubscribeGeneration = null;
      Vue.onMounted(() => {
        void mount().then(() => {
          syncHitArea();
          unsubscribeGeneration = dshSlotGeneration.subscribe(() => {
            void remount();
          });
        }).catch((error) => {
          console.error("[original-error]", error);
        });
      });
      Vue.onUpdated(() => {
        slotVersion.value += 1;
        if (!active || !mounted?.update) return;
        void mounted.update({
          element: element.value,
          props: {
            component: DshReactRuntimeBridge,
            componentProps: reactProps(),
            resources,
          },
          context: attrs.uiContext,
          emit() {},
        }).then(() => {
          syncHitArea();
        }).catch((error) => {
          console.error("[original-error]", rawError(error));
        });
      });
      Vue.onBeforeUnmount(() => {
        active = false;
        unsubscribeGeneration?.();
        unsubscribeGeneration = null;
        void mounted?.unmount().catch((error) => {
          console.error("[original-error]", rawError(error));
        });
        mounted = null;
      });
      return () => Vue.h("div", { ref: element, class: "dsh-react-mount" });
    },
  });
}

function isReactElement(value) {
  return Boolean(value) && typeof value === "object" && typeof value.$$typeof === "symbol";
}

let reactElementOutlet = null;

function reactElementOutletComponent() {
  if (reactElementOutlet) return reactElementOutlet;
  const Vue = globalThis.__AMITIA_VUE__;
  if (!Vue) throw new Error("Amitia Vue runtime is unavailable");
  const passthrough = (props) => props.element;
  reactElementOutlet = Vue.defineComponent({
    name: "DshReactElementOutlet",
    inheritAttrs: false,
    props: { element: { type: null, required: true } },
    setup(props) {
      const host = Vue.ref(null);
      let mounted = null;
      let active = true;
      const payload = () => ({
        element: host.value,
        props: { component: passthrough, componentProps: { element: props.element } },
        emit() {},
      });
      Vue.onMounted(() => {
        void globalThis.amitiaClientPlugins.foreignComponents.mount(reactAdapterId, payload())
          .then((instance) => {
            if (!active) return instance.unmount();
            mounted = instance;
            return undefined;
          })
          .catch((error) => {
            console.error("[original-error]", rawError(error));
          });
      });
      Vue.onUpdated(() => {
        if (!active || typeof mounted?.update !== "function") return;
        void mounted.update(payload()).catch((error) => {
          console.error("[original-error]", rawError(error));
        });
      });
      Vue.onBeforeUnmount(() => {
        active = false;
        const instance = mounted;
        mounted = null;
        void instance?.unmount().catch((error) => {
          console.error("[original-error]", rawError(error));
        });
      });
      return () => Vue.h("div", { ref: host, class: "dsh-react-mount" });
    },
  });
  return reactElementOutlet;
}

function mapDshSlot(slotId) {
  return resolveDshSlot(slotId, (mapped) => Boolean(globalThis.amitiaClientPlugins?.slots.getDefinition(mapped)));
}

function tryMapDshSlot(slotId) {
  try {
    return mapDshSlot(slotId);
  } catch (error) {
    console.error(`[dsh-ui] unsupported DSH slot ${text(slotId)}`, error);
    return undefined;
  }
}

function tryMapDshChildSlot(slotId) {
  const id = text(slotId);
  if (!id) return undefined;
  if (factorySlotAliases().has(id)) return undefined;
  if (!hasDshSlot(id)) {
    const reported = globalThis.__dshUnmappedChildSlots ?? (globalThis.__dshUnmappedChildSlots = new Set());
    if (!reported.has(id)) {
      reported.add(id);
      console.warn(`[dsh-ui] child slot ${id} is not in the 0.2.2-rc.2 catalog and stays bridge-local`);
    }
    return undefined;
  }
  try {
    return mapDshSlot(id);
  } catch (error) {
    const reported = globalThis.__dshUnmappedChildSlots ?? (globalThis.__dshUnmappedChildSlots = new Set());
    if (!reported.has(id)) {
      reported.add(id);
      console.warn(`[dsh-ui] child slot ${id} has no Amitia host slot and stays bridge-local`, error);
    }
    return undefined;
  }
}

function factorySurfaceId(slotId) {
  const encoded = Array.from(text(slotId), (character) => /[a-z0-9]/i.test(character)
    ? character
    : `_${character.codePointAt(0).toString(16)}`).join("");
  return `ui.dsh.factory.${encoded || "root"}`;
}

function factorySlotAliases() {
  if (!(globalThis.__dshFactorySlotAliases instanceof Map)) globalThis.__dshFactorySlotAliases = new Map();
  return globalThis.__dshFactorySlotAliases;
}

function resolveFactorySlotTarget(slotId) {
  const id = text(slotId);
  if (!id) return undefined;
  const alias = factorySlotAliases().get(id);
  if (alias) return alias;
  if (!hasDshSlot(id)) return undefined;
  try {
    return mapDshSlot(id);
  } catch (error) {
    void error;
    return undefined;
  }
}

function storedEntries(contributions) {
  return contributions.map((contribution) => ({
    component: contribution.component,
    options: {
      key: contribution.key,
      id: contribution.cellId ?? contribution.key,
      order: contribution.ordering,
      label: contribution.label,
      priority: contribution.priority,
    },
    registrant: contribution.pluginId,
  }));
}

const DSH_PAGE_SHELL_SLOTS = new Set([
  "main.conversation",
  "conversation.content",
  "conversation.session",
  "conversation.session.header",
  "conversation.header",
  "conversation.view",
  "conversation.chat.flow",
  "conversation.composer",
  "conversation.composer.bar",
  "conversation.composer.dock",
  "conversation.input.attachments",
  "conversation.input.dock",
  "conversation.input.left",
  "conversation.input.model",
  "conversation.input.model.label",
  "conversation.input.model.options",
  "conversation.input.overlay",
  "conversation.input.permission",
  "conversation.input.right",
  "conversation.user.text",
  "conversation.user.text.node",
  "conversation.chat.user.content",
  "conversation.chat.reasoning.content",
]);

const DSH_CONVERSATION_NODE_SLOT = "conversation.chat.node";
const DSH_TEXT_NODE_KEY_PATTERN = /(?:^|[.:/_-])(?:user|assistant|reasoning|text)(?:$|[.:/_-])/i;

function isDshConversationNodeSlot(slotId) {
  return text(slotId) === DSH_CONVERSATION_NODE_SLOT;
}

function isDshTextConversationNodeKey(key) {
  const value = text(key);
  if (!value) return false;
  return DSH_TEXT_NODE_KEY_PATTERN.test(value);
}

function nodeFieldText(value) {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function nodeContentText(value) {
  if (Array.isArray(value)) return value.map((item) => nodeContentText(item)).filter(Boolean).join("");
  if (value && typeof value === "object") return nodeFieldText(value.display) || nodeFieldText(value.text) || nodeFieldText(value.value);
  return nodeFieldText(value);
}

function conversationNodeText(node) {
  if (!node || typeof node !== "object") return "";
  const events = Array.isArray(node.events) ? node.events : [];
  const lastEvent = events.length > 0 ? events[events.length - 1] : undefined;
  const payloads = [node.payload, lastEvent?.payload, node.event?.data, node.event?.payload];
  const candidates = [node.title];
  for (const payload of payloads) {
    if (!payload || typeof payload !== "object") continue;
    candidates.push(payload.display, payload.text, payload.content, payload.summary, payload.message, payload.label, payload.title);
  }
  for (const candidate of candidates) {
    const value = nodeContentText(candidate);
    if (value.trim()) return value;
  }
  return "";
}

export function adaptConversationNodeOwner(owner) {
  const source = owner && typeof owner === "object" ? owner : {};
  const node = source.conversationNode && typeof source.conversationNode === "object" ? source.conversationNode : null;
  const display = conversationNodeText(node);
  return {
    ...source,
    eventType: text(source.eventType) || text(node?.eventType),
    conversationId: text(source.conversationId) || text(node?.conversationId),
    nodeType: text(source.nodeType) || text(node?.nodeType),
    conversationNode: node,
    node,
    event: node?.event ?? null,
    content: Object.freeze({ display, text: display, value: display }),
  };
}

export function createConversationNodeOwnerAdapter(component) {
  return function DshConversationNodeOwnerAdapter(owner) {
    return React.createElement(component, adaptConversationNodeOwner(owner));
  };
}

function isDshPageShellSlot(slotId) {
  return DSH_PAGE_SHELL_SLOTS.has(text(slotId));
}

const dshAdmittedContributions = new Map();
const dshSingleSlotOwners = new Map();
const DSH_SINGLE_ADMIT_SLOTS = new Set(["chat.conversation.header"]);
const dshContributionOrigins = new Map();
const dshSlotListeners = new Map();
const dshCommonInjects = new Map();
const dshFactoryContributionIds = new Map();



function createSlotBridge(pluginId, resources) {
  const runtime = globalThis.amitiaClientPlugins?.slots;
  if (!runtime) throw new Error("Amitia client slot runtime is unavailable");
  const disposers = [];
  const rootProviders = [];
  const tryMapSlot = (slotId) => tryMapDshSlot(slotId);
  const notifySlot = (slotId) => {
    const registry = globalThis.__dshSlotNotifiers ?? (globalThis.__dshSlotNotifiers = new Map());
    registry.set(slotId, notifySlot);
    const listeners = dshSlotListeners.get(slotId);
    if (!listeners) return;
    for (const listener of [...listeners]) {
      try {
        listener();
      } catch (error) {
        console.error("[original-error]", error);
      }
    }
  };
  const injectDeclOf = (options) => {
    const declared = options?.inject;
    if (typeof declared !== "function") return declared;
    return (context) => declared(text(context?.sessionId) || undefined, context?.actions ?? {});
  };
  const registerMapped = (options, component, slotId) => {
    const key = text(options?.key) || text(options?.id) || `${slotId}:${disposers.length}`;
    const contributionId = `client:${pluginId}:${slotId}:${key}`;
    const admissionKey = `${slotId}::${key}`;
    const admitted = dshAdmittedContributions.get(admissionKey);
    if (admitted && admitted !== contributionId) {
      return async () => {};
    }
    const hostKind = globalThis.amitiaClientPlugins?.slots?.getDefinition(slotId)?.kind;
    if (hostKind === "single" || DSH_SINGLE_ADMIT_SLOTS.has(slotId)) {
      const owner = dshSingleSlotOwners.get(slotId);
      if (owner && owner !== contributionId) {
        return async () => {};
      }
      dshSingleSlotOwners.set(slotId, contributionId);
    }
    dshAdmittedContributions.set(admissionKey, contributionId);
    dshContributionOrigins.set(contributionId, slotId);
    const vueComponent = createReactVueComponent(pluginId, component, resources, contributionId);
    const children = Object.entries(options?.children ?? {}).flatMap(([name, raw]) => {
      const spec = raw && typeof raw === "object" ? raw : {};
      const childSlotId = tryMapDshChildSlot(name);
      if (!childSlotId || childSlotId === slotId) return [];
      if (typeof runtime.acceptsChildDeclaration !== "function") {
        throw new Error("Amitia client slot runtime does not support child declaration acceptance");
      }
      if (!runtime.acceptsChildDeclaration(slotId, childSlotId)) return [];
      if (spec.inject && typeof spec.inject === "object") dshCommonInjects.set(childSlotId, spec.inject);
      const childDefinition = runtime.getDefinition(childSlotId);
      if (childDefinition) {
        console.warn(
          `[dsh-ui] child slot ${childSlotId} is predeclared by the host: dsh=${text(spec.kind) || "-"}/${text(spec.scope) || "-"} host=${childDefinition.kind}/${childDefinition.scope ?? "root"}`,
        );
        return [];
      }
      const parentScope = runtime.getDefinition(slotId)?.scope ?? "root";
      const requestedScope = spec.scope ?? parentScope;
      const scope = parentScope === "session" || requestedScope === "root" ? parentScope : requestedScope;
      return [{
        slotId: childSlotId,
        contractVersion: Number(spec.contractVersion) || 1,
        supportedKinds: Array.isArray(spec.supportedKinds) ? spec.supportedKinds : ["panel"],
        kind: spec.kind || "list",
        layout: spec.layout,
        fallbackPolicy: spec.fallbackPolicy,
        scope,
        allowRootChild: spec.allowRootChild ?? (scope === "root"),
        description: spec.description,
        commonInject: spec.inject,
      }];
    });
    const entryKey = text(options?.entryKey || options?.key || options?.id) || undefined;
    const cellId = text(options?.id || options?.key) || undefined;
    const dispose = runtime.register(pluginId, slotId, key, vueComponent, {
      ordering: Number.isFinite(Number(options?.order ?? options?.ordering)) ? Number(options.order ?? options.ordering) : 0,
      priority: Number.isFinite(Number(options?.priority)) ? Number(options.priority) : 0,
      children,
      store: options?.store,
      inject: injectDeclOf(options),
      cellId,
      entryKey,
      label: options?.label,
      select: options?.select ?? ((owner) => owner),
      localeNamespace: typeof options?.locale === "string" ? options.locale : undefined,
    });
    if (options?.store) dshStoreHandles.set(contributionId, options.store);
    notifySlot(slotId);
    const release = async () => {
      await dispose();
      if (dshAdmittedContributions.get(admissionKey) === contributionId) {
        dshAdmittedContributions.delete(admissionKey);
      }
      dshContributionOrigins.delete(contributionId);
      if (dshSingleSlotOwners.get(slotId) === contributionId) {
        dshSingleSlotOwners.delete(slotId);
      }
      releaseDshStore(contributionId);
      notifySlot(slotId);
    };
    disposers.push(release);
    return release;
  };
  const registerConversationNode = (options, component, key) => {
    const nodeRuntime = globalThis.amitiaClientPlugins;
    if (typeof nodeRuntime?.registerConversationNode !== "function") {
      throw new Error(
        `Amitia conversation node runtime is unavailable for DSH slot ${DSH_CONVERSATION_NODE_SLOT} key ${key}：window.amitiaClientPlugins.registerConversationNode 不存在`,
      );
    }
    const slotId = tryMapSlot(DSH_CONVERSATION_NODE_SLOT);
    if (!slotId) {
      throw new Error(`DSH slot ${DSH_CONVERSATION_NODE_SLOT} has no Amitia conversation node target`);
    }
    const contributionId = `client:${pluginId}:${slotId}:${key}`;
    const vueComponent = createReactVueComponent(
      pluginId,
      createConversationNodeOwnerAdapter(component),
      resources,
      contributionId,
    );
    const ordering = Number.isFinite(Number(options?.order ?? options?.ordering))
      ? Number(options.order ?? options.ordering)
      : 0;
    const definition = {
      maxEvents: 64,
      match(event) {
        const eventType = text(event?.eventType) || text(event?.type);
        if (eventType !== key) return null;
        const eventId = text(event?.id) || text(event?.sequence) || "current";
        return { contextId: `dsh-node:${key}:${eventId}`, phase: "update", data: event?.payload };
      },
      create(event) {
        return {
          eventType: text(event?.eventType) || text(event?.type),
          payload: event?.payload && typeof event.payload === "object" ? event.payload : {},
          timestamp: text(event?.timestamp),
        };
      },
      update(state, event) {
        const payload = event?.payload && typeof event.payload === "object" ? event.payload : state.payload;
        return {
          eventType: text(event?.eventType) || text(event?.type) || state.eventType,
          payload,
          timestamp: text(event?.timestamp) || state.timestamp,
        };
      },
      project(context) {
        const state = context?.state && typeof context.state === "object" ? context.state : {};
        const payload = state.payload && typeof state.payload === "object" ? state.payload : {};
        const title = text(payload.title) || text(payload.name);
        return { key, nodeType: key, payload, ...(title ? { title } : {}) };
      },
    };
    const dispose = nodeRuntime.registerConversationNode(
      pluginId,
      key,
      vueComponent,
      definition,
      { id: text(options?.id) || key, key, ordering },
    );
    notifySlot(slotId);
    const release = async () => {
      await dispose?.();
      notifySlot(slotId);
    };
    disposers.push(release);
    return release;
  };
  return {
    provideRoot(source) {
      if (!source || typeof source !== "object") throw new Error("DSH root slot source is required");
      rootProviders.push(source);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        const index = rootProviders.indexOf(source);
        if (index >= 0) rootProviders.splice(index, 1);
      };
    },
    root() {
      return Object.assign({}, ...rootProviders);
    },
    register(options, component) {
      const slotName = text(options?.name);
      if (isDshConversationNodeSlot(slotName)) {
        const nodeKey = text(options?.key) || text(options?.id);
        if (!nodeKey || isDshTextConversationNodeKey(nodeKey)) return null;
        return registerConversationNode(options, component, nodeKey);
      }
      if (isDshPageShellSlot(slotName)) return null;
      const mappedSlot = tryMapSlot(slotName);
      if (!mappedSlot) return null;
      return registerMapped(options, component, mappedSlot);
    },
    inject(slotId, callback) {
      if (isDshPageShellSlot(slotId)) return () => {};
      const mapped = tryMapSlot(slotId);
      if (!mapped) return () => {};
      let active = true;
      let registered;
      const attach = (definition) => resolveSlotInjectEffect(callback({
        ...definition,
        name: mapped,
      })).then(async (disposeList) => {
        const cleanup = async () => {
          for (let index = disposeList.length - 1; index >= 0; index -= 1) await disposeList[index]?.();
        };
        if (!active) return cleanup();
        registered = cleanup;
        return undefined;
      });
      const existing = typeof runtime.getDefinition === "function" ? runtime.getDefinition(mapped) : undefined;
      const pending = existing
        ? attach(existing)
        : runtime.inject(mapped, (definition) => attach(definition)).then((dispose) => {
          if (!active) return dispose();
          registered = dispose;
          return undefined;
        });
      const dispose = () => {
        if (!active) return;
        active = false;
        return registered ? registered() : pending;
      };
      disposers.push(dispose);
      return dispose;
    },
    entriesOfSlot(slotId) {
      const mapped = tryMapDshSlot(slotId);
      return mapped ? storedEntries(runtime.listContributions(mapped)) : [];
    },
    entries(slotId) {
      const mapped = tryMapDshSlot(slotId);
      return mapped ? storedEntries(runtime.listContributions(mapped)) : [];
    },
    async registerFactory(options, factory) {
      if (!factory || (typeof factory !== "function" && typeof factory !== "object")) throw new Error("DSH slot factory must be a React component");
      const dshSlotId = text(options?.name);
      if (!dshSlotId) throw new Error("DSH slot factory name is required");
      if (hasDshSlot(dshSlotId)) {
        const mappedOfficial = mapDshSlot(dshSlotId);
        if (globalThis.amitiaClientPlugins.slots.getDefinition(mappedOfficial)) {
          const factoryId = `dsh-factory:${dshSlotId}`;
          factorySlotAliases().set(dshSlotId, mappedOfficial);
          dshFactoryContributionIds.set(dshSlotId, factoryId);
          const dispose = this.register({
            ...options,
            id: factoryId,
            key: factoryId,
            select: (owner) => {
              const requested = owner && typeof owner === "object" ? text(owner.__dshFactory) : "";
              return !requested || requested === factoryId ? owner : null;
            },
          }, factory);
          return async () => {
            await dispose();
            if (dshFactoryContributionIds.get(dshSlotId) === factoryId) dshFactoryContributionIds.delete(dshSlotId);
            if (factorySlotAliases().get(dshSlotId) === mappedOfficial) factorySlotAliases().delete(dshSlotId);
          };
        }
      }
      const mapped = factorySurfaceId(dshSlotId);
      factorySlotAliases().set(dshSlotId, mapped);
      let declaration;
      if (!globalThis.amitiaClientPlugins.slots.getDefinition(mapped)) {
        declaration = await globalThis.amitiaClientPlugins.slots.declare({
          slotId: mapped,
          contractVersion: 1,
          supportedKinds: ["panel"],
          kind: "single",
          layout: "stack",
          fallbackPolicy: "none",
          description: `DSH Slot Factory ${dshSlotId}`,
          scope: options?.scope === "session" ? "session" : "root",
        });
      }
      const dispose = registerMapped(options, factory, mapped);
      return async () => {
        await dispose();
        await declaration?.dispose();
        if (factorySlotAliases().get(dshSlotId) === mapped) factorySlotAliases().delete(dshSlotId);
      };
    },
    subscribe(slotId, listener) {
      const mapped = tryMapSlot(slotId);
      if (!mapped) return () => {};
      if (typeof listener !== "function") throw new Error("DSH slot listener must be a function");
      let listeners = dshSlotListeners.get(mapped);
      if (!listeners) {
        listeners = new Set();
        dshSlotListeners.set(mapped, listeners);
      }
      listeners.add(listener);
      void Promise.resolve().then(() => {
        if (!listeners.has(listener)) return;
        try {
          listener();
        } catch (error) {
          console.error("[original-error]", error);
        }
      });
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0) dshSlotListeners.delete(mapped);
      };
    },
    async dispose() {
      for (let index = disposers.length - 1; index >= 0; index -= 1) {
        await disposers[index]?.();
      }
      disposers.length = 0;
    },
  };
}

function createInputService() {
  return {
    draft: "",
    attachments: [],
    command: undefined,
    quotes: [],
    occurrences: [],
    pendingSubmissions: [],
    persistDraft: () => {},
    requestDraftInitialization: () => "preserved",
    setDraft: () => {},
    dispose: () => {},
    subscribe: () => () => {},
    getSnapshot() {
      return this;
    },
  };
}

function createInputActions() {
  return Object.freeze({
    persistDraft: () => {},
    setDraft: () => {},
    clearDraft: () => {},
    completeViewRequest: () => {},
    submit: async () => ({ accepted: false }),
    stop: async () => {},
    queue: async () => ({ accepted: false }),
    steer: async () => ({ accepted: false }),
    addAttachments: () => {},
    removeAttachment: () => {},
  });
}

function createSessionSourceRegistry() {
  const descriptors = [];
  const handles = new Map();
  const bindingFactories = [];
  const resolveSources = (sessionId) => {
    const binding = bindingFactories.length > 0 && sessionId ? bindingFactories[0](sessionId) : undefined;
    const sources = [];
    for (const descriptor of descriptors) {
      const resolved = descriptor.resolve(binding ?? { sessionId });
      if (resolved) sources.push(resolved);
    }
    return Object.freeze({
      sessionId: sessionId || undefined,
      hooks: Object.freeze(Object.assign({}, ...sources.map((source) => source.hooks || {}))),
      keyedHooks: Object.freeze(Object.assign({}, ...sources.map((source) => source.keyedHooks || {}))),
      props: Object.freeze(Object.assign({}, ...sources.map((source) => source.props || {}))),
    });
  };
  const refresh = () => {
    for (const handle of [...handles.values()]) handle.refresh();
  };
  const binding = (reference) => {
    const sessionId = text(reference?.sessionId || reference?.session?.sessionId || reference?.id || reference);
    const key = sessionId || "";
    const existing = handles.get(key);
    if (existing) return existing;
    let snapshot = resolveSources(key);
    const listeners = new Set();
    const handle = {
      sessionId: key || undefined,
      getSnapshot: () => snapshot,
      subscribe(listener) {
        if (typeof listener !== "function") throw new Error("DSH session source listener must be a function");
        listeners.add(listener);
        return () => { listeners.delete(listener); };
      },
      refresh() {
        snapshot = resolveSources(key);
        for (const listener of [...listeners]) {
          try {
            listener();
          } catch (error) {
            console.error("[original-error]", error);
          }
        }
      },
      dispose() {
        listeners.clear();
        if (handles.get(key) === handle) handles.delete(key);
      },
    };
    handles.set(key, handle);
    return handle;
  };
  return {
    provide(descriptor) {
      if (!descriptor || typeof descriptor.resolve !== "function") {
        throw new Error("DSH uiSession.provide requires a resolve function");
      }
      descriptors.push(descriptor);
      refresh();
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        const index = descriptors.indexOf(descriptor);
        if (index >= 0) descriptors.splice(index, 1);
        refresh();
      };
    },
    registerBinding(factory) {
      if (typeof factory !== "function") throw new Error("DSH session binding factory must be a function");
      bindingFactories.push(factory);
      refresh();
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        const index = bindingFactories.indexOf(factory);
        if (index >= 0) bindingFactories.splice(index, 1);
        refresh();
      };
    },
    binding,
    resolve(reference) {
      return binding(reference).getSnapshot();
    },
    refresh,
    dispose() {
      descriptors.length = 0;
      bindingFactories.length = 0;
      for (const handle of [...handles.values()]) handle.dispose();
    },
  };
}

const dshSessionSources = createSessionSourceRegistry();
const conversationDataBySession = new Map();
const dshStoreHandles = new Map();
const dshStoreInstances = new Map();

function dshStoreInstanceFor(contributionId, sessionId) {
  const key = text(contributionId);
  if (!key) return undefined;
  const handle = dshStoreHandles.get(key);
  if (!handle || typeof handle.create !== "function") return undefined;
  const scopeKey = text(sessionId) || "root";
  const instanceKey = `${key}:${scopeKey}`;
  const cached = dshStoreInstances.get(instanceKey);
  if (cached) return cached;
  const instance = handle.create(scopeKey);
  dshStoreInstances.set(instanceKey, instance);
  return instance;
}

function releaseDshStore(contributionId) {
  dshStoreHandles.delete(contributionId);
  for (const [instanceKey, instance] of [...dshStoreInstances]) {
    if (!instanceKey.startsWith(`${contributionId}:`)) continue;
    dshStoreInstances.delete(instanceKey);
    instance?.dispose?.();
  }
}

function conversationDataForSession(sessionId) {
  const id = text(sessionId);
  if (!id) return null;
  const cached = conversationDataBySession.get(id);
  if (cached) return cached;
  const data = createConversationData({
    conversationId: id,
    command: (input) => globalThis.__dshHostCommand(input),
  });
  conversationDataBySession.set(id, data);
  registerConversationData(id, data);
  void data.refresh().catch((error) => console.error("[original-error]", error));
  return data;
}

function createConversationInteractions() {
  const empty = Object.freeze([]);
  return Object.freeze({
    getSnapshot: () => empty,
    subscribe: () => () => {},
    dispatch: async () => undefined,
  });
}

const dshConversationInteractions = createConversationInteractions();

function conversationBindingForSession(sessionId) {
  const data = conversationDataForSession(sessionId);
  if (!data) {
    throw new Error(`DSH conversation binding requires a session id: ${JSON.stringify(sessionId ?? null)}`);
  }
  return Object.freeze({
    sessionId: data.conversationId,
    snapshot: data.conversation,
    interactions: dshConversationInteractions,
    activate(viewId) {
      data.activate(viewId);
    },
    target(name) {
      return data.target(name);
    },
  });
}

function createLocaleBridge() {
  const runtime = globalThis.amitiaClientPlugins?.locale;
  if (!runtime) throw new Error("Amitia client locale runtime is unavailable");
  return {
    register(namespace, dictionaries) {
      return runtime.register("com.amitia/dsh-compatible-host", namespace, dictionaries);
    },
    setCurrent(locale) {
      runtime.setLocale(locale);
    },
    current() {
      return runtime.getLocale();
    },
    bind(namespace) {
      if (typeof runtime.bind !== "function") throw new Error("Amitia client locale bind is unavailable");
      return runtime.bind(namespace);
    },
    subscribe() {
      return () => {};
    },
  };
}

function base64ToBytes(value) {
  const binary = atob(String(value ?? ""));
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

async function importClientEntryFromHost(pluginId, entry) {
  const command = globalThis.__dshHostCommand;
  if (typeof command !== "function") throw new Error("DSH 客户端模块加载需要宿主命令桥");
  const chunks = [];
  let total = 0;
  let offset = 0;
  for (let page = 0; page < 256; page += 1) {
    const chunk = await command({ action: "client-source", pluginId, entryId: entry.id, offset, length: 262144 });
    const bytes = base64ToBytes(chunk?.dataBase64);
    chunks.push(bytes);
    total += bytes.length;
    if (chunk?.done === true) break;
    if (Number(chunk?.nextOffset) <= offset) throw new Error(`DSH client-source 分页未推进：${pluginId}/${entry.id}`);
    offset = Number(chunk.nextOffset);
  }
  const source = new Uint8Array(total);
  let cursor = 0;
  for (const bytes of chunks) {
    source.set(bytes, cursor);
    cursor += bytes.length;
  }
  const url = URL.createObjectURL(new Blob([source], { type: "text/javascript" }));
  try {
    await import(url);
  } finally {
    URL.revokeObjectURL(url);
  }
}

async function importClientEntry(plugin, entry) {
  let url = "";
  try {
    url = await clientRuntimeModuleURL(plugin, entry);
  } catch (error) {
    if (!/page module URL/.test(String(error?.message ?? error))) throw error;
    await importClientEntryFromHost(plugin.pluginId, entry);
    return "client-source";
  }
  try {
    await import(url);
    return "page-module";
  } catch (error) {
    const message = String(error?.message ?? error);
    if (!/runtime_module_not_found|Failed to fetch dynamically imported module|401|404/.test(message)) throw error;
    await importClientEntryFromHost(plugin.pluginId, entry);
    return "client-source";
  }
}

function createClientCommandUi() {
  const items = new Map();
  const decorations = new Map();
  const normalizeName = (command) => {
    const name = String(command?.name ?? "").trim().replace(/^\//, "");
    if (name === "") throw new Error("DSH commandUi.register 需要 name");
    return name;
  };
  const rows = (list) => list.map((command) => ({
    name: command.name,
    label: typeof command.label === "function" ? command.label() : (command.label ?? command.name),
    ...(command.icon ? { icon: command.icon } : {}),
    ...(command.description ? { description: command.description } : {}),
    ...(command.available === false ? { available: false } : {}),
  }));
  return {
    register(command) {
      const name = normalizeName(command);
      if (items.has(name)) throw new Error(`DSH commandUi 命令 ${name} 已经注册`);
      items.set(name, { ...command, name });
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (items.get(name)?.name === name) items.delete(name);
      };
    },
    unregister(name) {
      return items.delete(String(name ?? "").trim().replace(/^\//, ""));
    },
    list() {
      return rows([...items.values()]);
    },
    decorate(decoration) {
      const name = String(decoration?.name ?? "").trim().replace(/^\//, "");
      if (name === "") throw new Error("DSH commandUi.decorate 需要 name");
      if (decorations.has(name)) throw new Error(`DSH commandUi 装饰 ${name} 已经注册`);
      decorations.set(name, decoration);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (decorations.get(name) === decoration) decorations.delete(name);
      };
    },
    decorations() {
      return [...decorations.values()];
    },
    async candidates(_session, request) {
      const prefix = String(request?.query ?? "").trim().replace(/^\//, "").toLowerCase();
      return rows([...items.values()].filter((command) => command.name.toLowerCase().startsWith(prefix)));
    },
  };
}

function createClientCommandRegistry() {
  const items = new Map();
  const requireCommand = (command) => {
    const name = String(command?.name ?? "").trim().replace(/^\//, "");
    if (name === "") throw new Error("DSH 客户端命令需要 name");
    if (typeof command?.handler !== "function" && typeof command?.run !== "function") {
      throw new Error(`DSH 客户端命令 ${name} 需要 handler 或 run 函数`);
    }
    return name;
  };
  return {
    register(command) {
      const name = requireCommand(command);
      if (items.has(name)) throw new Error(`DSH 客户端命令 ${name} 已经注册`);
      items.set(name, command);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (items.get(name) === command) items.delete(name);
      };
    },
    unregister(name) {
      return items.delete(String(name ?? "").trim().replace(/^\//, ""));
    },
    list() {
      return [...items.values()];
    },
    candidates(query) {
      const prefix = String(query ?? "").trim().replace(/^\//, "").toLowerCase();
      return [...items.values()].filter((command) => String(command.name ?? "").toLowerCase().startsWith(prefix));
    },
    async execute(name, input) {
      const key = String(name ?? "").trim().replace(/^\//, "");
      const command = items.get(key);
      if (!command) throw new Error(`DSH 客户端命令 ${key} 不存在`);
      const run = typeof command.handler === "function" ? command.handler : command.run;
      return run.call(command, input);
    },
  };
}

function appendScriptAwait(src) {
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = src;
    script.async = false;
    script.addEventListener("load", () => resolve());
    script.addEventListener("error", (event) => reject(new Error(`DSH 宿主注入脚本加载失败 ${src}: ${String(event?.message ?? "error")}`)));
    document.head.appendChild(script);
  });
}

export async function applyWebIndexInjections() {
  const command = globalThis.__dshHostCommand;
  if (typeof command !== "function") return;
  const injection = await command({ action: "web-index-injection" });
  const origin = typeof injection?.origin === "string" ? injection.origin : "";
  const html = typeof injection?.html === "string" ? injection.html : "";
  if (origin !== "") globalThis.__DSH_WEB_ORIGIN__ = origin;
  if (origin !== "") {
    const paths = injection?.paths ?? {};
    const exactPaths = Array.isArray(paths.exact) ? paths.exact.map(String) : [];
    const prefixPaths = Array.isArray(paths.prefixes) ? paths.prefixes.map(String) : [];
    const bridgeCode = `(() => {
  const base = ${JSON.stringify(origin)};
  const exactPaths = ${JSON.stringify(exactPaths)};
  const prefixPaths = ${JSON.stringify(prefixPaths)};
  const owned = (pathname) => exactPaths.includes(pathname) || prefixPaths.some((prefix) => pathname === prefix || pathname.startsWith(prefix.endsWith("/") ? prefix : prefix + "/"));
  const rewrite = (value) => (typeof value === "string" && value.startsWith("/") && owned(value.split("?")[0]) ? base + value : value);
  const nativeFetch = globalThis.fetch;
  if (typeof nativeFetch === "function") {
    globalThis.fetch = function (input, init) {
      try {
        if (typeof input === "string") input = rewrite(input);
        else if (input instanceof Request && input.url) {
          const url = new URL(input.url);
          if (url.origin === location.origin && owned(url.pathname)) input = new Request(base + url.pathname + url.search, input);
        }
      } catch (error) { console.error("[original-error]", error); }
      return nativeFetch.call(this, input, init);
    };
  }
  const open = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function (method, url, ...rest) {
    try { url = rewrite(url); } catch (error) { console.error("[original-error]", error); }
    return open.call(this, method, url, ...rest);
  };
})();`;
    const servedBridge = await command({ action: "web-inline-script", code: bridgeCode });
    if (typeof servedBridge?.url !== "string" || servedBridge.url === "") {
      throw new Error("DSH 宿主请求改写脚本没有取得可加载地址");
    }
    await appendScriptAwait(servedBridge.url.startsWith("/") ? `${origin}${servedBridge.url}` : servedBridge.url);
  }
  if (html.trim() === "") return;
  const parsed = new DOMParser().parseFromString(html, "text/html");
  for (const node of parsed.head.querySelectorAll("script")) {
    if (node.src) {
      await appendScriptAwait(new URL(node.src, origin || location.origin).toString());
      continue;
    }
    const code = node.textContent ?? "";
    if (code.trim() === "") continue;
    const served = await command({ action: "web-inline-script", code });
    if (typeof served?.url !== "string" || served.url === "") {
      throw new Error("DSH 宿主索引注入的内联脚本没有取得可加载地址");
    }
    await appendScriptAwait(served.url.startsWith("/") ? `${origin || location.origin}${served.url}` : served.url);
  }
  for (const node of parsed.head.querySelectorAll("link[rel=stylesheet], style")) {
    document.head.appendChild(node.cloneNode(true));
  }
}

export async function instantiateClient(plugin, options = {}) {
  const clientEntries = Array.isArray(plugin.clientEntries) && plugin.clientEntries.length
    ? plugin.clientEntries
    : plugin.clientEntry
      ? [{ id: "root", entry: plugin.clientEntry }]
      : [];
  const runtimeEntries = clientEntries.filter((entry) => !String(entry.entry || "").includes("/types/"));
  if (runtimeEntries.length === 0) return null;
  const loader = installDshModuleLoader();
  if (!(loader instanceof DshClientModuleLoader)) throw new Error("DSH client module loader is invalid");
  const dependencies = Array.isArray(plugin.clientDependencies) ? plugin.clientDependencies : [];
  for (const dependency of dependencies) {
    const dependencyEntries = Array.isArray(dependency.clientEntries) && dependency.clientEntries.length
      ? dependency.clientEntries
      : dependency.clientEntry ? [{ id: "root", entry: dependency.clientEntry }] : [];
    if (dependencyEntries.length === 0) continue;
    if (dependency.typeOnly === true) {
      if (!loader.has(dependency.pluginId)) loader.load({ id: dependency.pluginId, factory: () => ({}) });
      continue;
    }
    const identity = { pluginId: dependency.pluginId, clientEntries: dependencyEntries, clientEntry: dependency.clientEntry };
    for (const entry of dependencyEntries) {
      await importClientEntry(identity, entry);
    }
  }
  try {
    const hostCommand = globalThis.__dshHostCommand;
    if (typeof hostCommand !== "function") throw new Error("DSH 宿主命令桥不可用，无法读取插件语言包");
    const payload = await hostCommand({ action: "client-locale", pluginId: plugin.pluginId });
    const locales = payload?.locales && typeof payload.locales === "object" ? payload.locales : {};
    const languages = Object.keys(locales);
    if (languages.length > 0) {
      const namespaces = new Set();
      for (const language of languages) {
        for (const name of Object.keys(locales[language] ?? {})) namespaces.add(name);
      }
      const localeBridge = createLocaleBridge();
      for (const name of namespaces) {
        const dictionaries = {};
        for (const language of languages) {
          const value = locales[language]?.[name];
          if (value && typeof value === "object") dictionaries[language] = value;
        }
        localeBridge.register(name, dictionaries);
      }
    }
  } catch (error) {
    console.error(`[dsh-shell] ${plugin.pluginId} 语言包注册失败\n[original-error] ${rawError(error)}`);
  }
  const before = new Map(loader.list().map((id) => [id, loader.record(id)]));
  for (const entry of runtimeEntries) {
    if (typeof options.loadClientSource === "function") {
      await options.loadClientSource(plugin, entry);
    } else {
      await importClientEntry(plugin, entry);
    }
  }
  const moduleIds = loader.list().filter((id) => !before.has(id) || loader.record(id) !== before.get(id));
  if (moduleIds.length === 0) {
    console.warn(`[dsh-shell] ${plugin.pluginId} 未注册新的 client module，视为已装载`);
    return { async dispose() {} };
  }
  const clientCommands = createClientCommandRegistry();
  const clientCommandUi = createClientCommandUi();
  ensureReactAdapter();
  const moduleRecords = moduleIds.map((id) => ({ id, record: loader.record(id) }));
  const loadedModules = [];
  try {
    for (const moduleId of moduleIds) {
      const resources = createDshClientResourceRegistry();
      const slots = createSlotBridge(moduleId, resources);
      const locale = createLocaleBridge();
      const jobs = createDshClientJobsController((input) => globalThis.__dshHostCommand(input));
      const clientConfig = createDshConfigForms(moduleId);
      const settingsSchema = createDshSettingsSchema();
      const fileDrag = createDshFileDragActions();
      const Vue = globalThis.__AMITIA_VUE__;
      if (!Vue?.defineComponent) throw new Error("Amitia Vue runtime is unavailable");
      const conversationProjectionNode = Vue.defineComponent({
        name: "DshConversationProjectionNode",
        render() {
          return null;
        },
      });
      let conversationDefinitionIndex = 0;
      const conversationNodeDefinitions = new Map();
      const conversationNodeRuntime = {
        register(key, component, definition) {
          if (conversationNodeDefinitions.has(key)) {
            throw new Error(`DSH conversation definition ${key} is already registered`);
          }
          const record = { key, component, definition };
          conversationNodeDefinitions.set(key, record);
          let active = true;
          return () => {
            if (!active) return;
            active = false;
            if (conversationNodeDefinitions.get(key) === record) conversationNodeDefinitions.delete(key);
          };
        },
        entries() {
          return Array.from(conversationNodeDefinitions.values());
        },
      };
      const noopDisposer = () => {};
      const eventBus = new DshClientEventBus();
      const runtimeServices = createDshClientRuntimeServices({
        command: (input) => globalThis.__dshHostCommand(input),
        navigate: (target) => globalThis.__dshUiNavigate(target),
        emit: (type, payload) => eventBus.emit(type, payload),
        hostActions: {
          ...globalThis.__dshUiActions,
          openRightPane: () => {
            const runtime = globalThis.amitiaClientPlugins?.rightPane;
            if (!runtime) throw new Error("Amitia right pane runtime is unavailable");
            return runtime.open("dsh-resource://guide/root", {
              title: "Guide",
              sessionId: globalThis.__dshCurrentSessionId,
              params: { tabKind: "guide" },
            });
          },
          closeRightPane: () => globalThis.amitiaClientPlugins?.rightPane?.close(),
          selectDirectory: () => globalThis.window?.amitiaDesktop?.selectWorkspaceDirectory?.(),
          listDirectory: (path) => {
            if (typeof globalThis.window?.amitiaDesktop?.listWorkspaceDirectory !== "function") {
              throw new Error("Amitia desktop directory listing bridge is unavailable");
            }
            return globalThis.window.amitiaDesktop.listWorkspaceDirectory(path);
          },
          createDirectory: (path, name) => {
            if (typeof globalThis.window?.amitiaDesktop?.createWorkspaceDirectory !== "function") {
              throw new Error("Amitia desktop directory creation bridge is unavailable");
            }
            return globalThis.window.amitiaDesktop.createWorkspaceDirectory(path, name);
          },
        },
      });
      runtimeServices.remote.settings = clientConfig.settings;
      const disposeSessionBindingFactory = dshSessionSources.registerBinding(
        (sessionId) => runtimeServices.sessions.binding(sessionId),
      );
      runtimeServices.remote.$on = clientConfig.onRemote;
      const registeredConversationViews = [];
      const registeredConversationGroups = [];
      const uiConversation = {
        events: {
          register(definition) {
            const index = conversationDefinitionIndex++;
            return registerDshConversationDefinition(conversationNodeRuntime, moduleId, definition, index, conversationProjectionNode);
          },
          registerFallback(definition) {
            return this.register(definition);
          },
        },
        views: {
          register(definition) {
            if (!definition || typeof definition !== "object") throw new Error("DSH Conversation view definition is required");
            registeredConversationViews.push(definition);
            let active = true;
            return () => {
              if (!active) return;
              active = false;
              const index = registeredConversationViews.indexOf(definition);
              if (index >= 0) registeredConversationViews.splice(index, 1);
            };
          },
          entries() {
            return registeredConversationViews.slice();
          },
          subscribe() {
            return noopDisposer;
          },
        },
        groups: {
          register(definition) {
            if (!definition || typeof definition !== "object") throw new Error("DSH Conversation group definition is required");
            registeredConversationGroups.push(definition);
            let active = true;
            return () => {
              if (!active) return;
              active = false;
              const index = registeredConversationGroups.indexOf(definition);
              if (index >= 0) registeredConversationGroups.splice(index, 1);
            };
          },
          entries() {
            return registeredConversationGroups.slice();
          },
        },
        interactions: dshConversationInteractions,
        binding(reference) {
          return conversationBindingForSession(reference?.sessionId || reference?.session?.sessionId || reference?.id || reference);
        },
        imageUrl() {
          throw new Error("DSH conversation image address resolution is not bridged to Amitia attachments yet");
        },
        peekImageUrl() {
          return undefined;
        },
      };
      const sidebarRightTabDefinitions = [];
      const sidebarRightTabListeners = new Set();
      function notifySidebarRightTabs() {
        for (const listener of [...sidebarRightTabListeners]) listener();
      }
      const sidebarRightTabs = {
        register(definition) {
          if (!definition || typeof definition !== "object") throw new Error("DSH sidebar right tab definition is required");
          sidebarRightTabDefinitions.push(definition);
          notifySidebarRightTabs();
          let active = true;
          return () => {
            if (!active) return;
            active = false;
            const index = sidebarRightTabDefinitions.indexOf(definition);
            if (index >= 0) sidebarRightTabDefinitions.splice(index, 1);
            notifySidebarRightTabs();
          };
        },
        entries() {
          return sidebarRightTabDefinitions.slice();
        },
        candidates(address) {
          const value = text(address);
          return sidebarRightTabDefinitions.filter((definition) => {
            if (typeof definition.canOpen === "function") return definition.canOpen(address);
            const patterns = Array.isArray(definition.patterns) ? definition.patterns : [];
            return patterns.some((pattern) => value.startsWith(String(pattern).replaceAll("**", "")));
          });
        },
        subscribe(listener) {
          if (typeof listener !== "function") throw new Error("DSH sidebarRightTabs listener must be a function");
          sidebarRightTabListeners.add(listener);
          return () => sidebarRightTabListeners.delete(listener);
        },
        resolve(address) {
          return this.candidates(address)[0];
        },
        get(key) {
          const value = text(key);
          if (!value) return undefined;
          return sidebarRightTabDefinitions.find((definition) => text(definition?.kind) === value)
            ?? sidebarRightTabDefinitions.find((definition) => text(definition?.id ?? definition?.key) === value);
        },
        has(key) {
          return this.get(key) !== undefined;
        },
        values() {
          return [...sidebarRightTabDefinitions];
        },
      };
      const sidebarRightMounted = createDshSnapshotHandle(true);
      const rightPaneSource = globalThis.amitiaClientPlugins?.rightPane;
      const currentSession = createDshSnapshotHandle({
        key: text(globalThis.__dshCurrentSessionId) || undefined,
      });
      const syncCurrentSession = () => {
        const sessionId = text(rightPaneSource?.getSnapshot?.().sessionId || globalThis.__dshCurrentSessionId);
        currentSession.set({ key: sessionId || undefined });
      };
      const stopCurrentSession = rightPaneSource?.subscribe?.(syncCurrentSession);
      syncCurrentSession();
      const contextMenuActive = createDshSnapshotHandle({
        sessionId: text(globalThis.__dshCurrentSessionId) || undefined,
        paneId: undefined,
        tab: undefined,
      });
      const contextMenuRegistrations = new Map();
      const contextMenus = {
        active: contextMenuActive,
        register(definition) {
          const id = text(definition?.id);
          if (!id) throw new Error("DSH context menu id is required");
          contextMenuRegistrations.set(id, definition);
          let active = true;
          return () => {
            if (!active) return;
            active = false;
            if (contextMenuRegistrations.get(id) === definition) contextMenuRegistrations.delete(id);
          };
        },
      };
      const occurrenceHandles = new Map();
      const sidebarRight = {
        tabsIn() {
          return sidebarRightTabs.entries();
        },
        mounted: sidebarRightMounted,
        tabDomain: {
          occurrence(sessionId, occurrence = {}) {
            const tabId = text(occurrence.id || occurrence.tabId) || "guide";
            const key = `${text(sessionId)}:${tabId}`;
            if (!occurrenceHandles.has(key)) {
              occurrenceHandles.set(key, {
                navigation: createDshSnapshotHandle({
                  address: `dsh-resource://${tabId}/session/${encodeURIComponent(text(sessionId))}`,
                  params: {},
                }),
              });
            }
            return occurrenceHandles.get(key);
          },
        },
        commandTarget() {
          const current = rightPaneSource?.getSnapshot?.();
          const sessionId = text(current?.sessionId || globalThis.__dshCurrentSessionId);
          if (!sessionId) return undefined;
          return {
            sessionId,
            tabId: current?.address,
            paneId: "right",
            host: "dock",
          };
        },
        isExpanded() {
          return Boolean(rightPaneSource?.getSnapshot?.().open);
        },
        toggleExpanded() {
          if (!rightPaneSource) throw new Error("Amitia right pane runtime is unavailable");
          if (rightPaneSource.getSnapshot().open) return rightPaneSource.close();
          return globalThis.__dshUiActions?.openRightPane?.({
            track: true,
            fullscreen: false,
          });
        },
        openTabFromTarget(kind, target = {}) {
          const id = text(kind) || "guide";
          const sessionId = text(target.sessionId || globalThis.__dshCurrentSessionId);
          if (!sessionId) throw new Error("DSH right tab Session id is required");
          if (!rightPaneSource) throw new Error("Amitia right pane runtime is unavailable");
          return rightPaneSource.open(`dsh-resource://${id}/session/${encodeURIComponent(sessionId)}`, {
            sessionId,
            params: { tabKind: id },
          });
        },
        openTab(kind, options = {}) {
          return this.openTabFromTarget(kind, {
            sessionId: options.sessionId,
          });
        },
        openResource(address, options = {}) {
          if (!rightPaneSource) throw new Error("Amitia right pane runtime is unavailable");
          const definition = sidebarRightTabs.resolve(address);
          const title = typeof definition?.title === "function" ? definition.title() : definition?.title;
          return rightPaneSource.open(address, {
            title,
            params: options.params,
            sessionId: options.sessionId || text(globalThis.__dshCurrentSessionId),
          });
        },
      };
      const sessionStatus = createDshSnapshotHandle(new Map());
      const connectionState = createDshSnapshotHandle("connected");
      const pendingInteractions = new Map();
      const publishPendingInteractions = () => {
        const grouped = new Map();
        for (const [key, record] of pendingInteractions) {
          const sessionId = text(record.interaction?.sessionId || record.interaction?.conversationId);
          if (!sessionId) continue;
          const existing = grouped.get(sessionId) || {};
          if (!existing.pendingInteraction || record.precedence >= existing.precedence) {
            grouped.set(sessionId, {
              pendingInteraction: record.interaction,
              precedence: record.precedence,
              status: existing.status,
            });
          }
        }
        sessionStatus.set(grouped);
      };
      const uiSession = {
        adapter: {
          current: currentSession,
        },
        sessionStatus,
        provide(descriptor) {
          return dshSessionSources.provide(descriptor);
        },
        resolve(reference) {
          return dshSessionSources.resolve(reference);
        },
        registerPendingInteraction(precedence) {
          if (typeof precedence !== "function") throw new Error("DSH pending interaction precedence must be a function");
          return (interaction, delegate) => {
            const key = `${text(interaction?.id || interaction?.sessionId)}:${requestId("pending")}`;
            pendingInteractions.set(key, { interaction, delegate, precedence: Number(precedence(interaction)) || 0 });
            publishPendingInteractions();
            let active = true;
            return async () => {
              if (!active) return;
              active = false;
              const record = pendingInteractions.get(key);
              pendingInteractions.delete(key);
              publishPendingInteractions();
              if (record?.delegate) await record.delegate();
            };
          };
        },
        bindingSource(reference) {
          return dshSessionSources.binding(reference);
        },
        whenAccess(_sessionId, _action, callback) {
          return runtimeServices.remoteAccess.whenAccess(_action, callback);
        },
      };
      const disposeSidebarFacades = () => {
        disposeSessionBindingFactory();
        stopCurrentSession?.();
        sidebarRightMounted.dispose();
        currentSession.dispose();
        sessionStatus.dispose();
        connectionState.dispose();
        pendingInteractions.clear();
        registeredConversationViews.length = 0;
        registeredConversationGroups.length = 0;
        conversationNodeDefinitions.clear();
        contextMenuActive.dispose();
        shortcuts.catalog.dispose();
        window.removeEventListener("keydown", shortcutKeydown);
        shortcutInputListeners.clear();
        fixedShortcuts.clear();
        for (const occurrence of occurrenceHandles.values()) occurrence.navigation.dispose();
        occurrenceHandles.clear();
        contextMenuRegistrations.clear();
      };
      const fixedShortcuts = new Map();
      const shortcutInputListeners = new Set();
      const shortcutLabels = new Map([
        ["Escape", "Esc"],
        ["ArrowUp", "↑"],
        ["ArrowDown", "↓"],
        ["ArrowLeft", "←"],
        ["ArrowRight", "→"],
        ["Enter", "Enter"],
        ["Slash", "/"],
        ["Backslash", "\\"],
        ["Space", "Space"],
      ]);
      const shortcutLabel = (binding) => {
        const modifiers = Array.isArray(binding?.modifiers)
          ? binding.modifiers.map((item) => text(item) === "primary" ? "Ctrl" : text(item))
          : [];
        const key = shortcutLabels.get(text(binding?.code)) || text(binding?.code);
        return [...modifiers, key].filter(Boolean).join("+");
      };
      const shortcuts = {
        catalog: createDshSnapshotHandle([]),
        register(definition) {
          return shortcuts.registerFixed(definition);
        },
        registerFixed(definition) {
          const id = text(definition?.id);
          if (!id) throw new Error("DSH shortcut id is required");
          fixedShortcuts.set(id, definition);
          shortcuts.catalog.set([...fixedShortcuts.values()]);
          let active = true;
          return () => {
            if (!active) return;
            active = false;
            if (fixedShortcuts.get(id) === definition) fixedShortcuts.delete(id);
            shortcuts.catalog.set([...fixedShortcuts.values()]);
          };
        },
        describeBinding(binding) {
          const key = shortcutLabel(binding);
          return { keys: key ? [key] : [] };
        },
        observeFixedInput(listener) {
          if (typeof listener !== "function") throw new Error("DSH fixed shortcut listener must be a function");
          shortcutInputListeners.add(listener);
          return () => shortcutInputListeners.delete(listener);
        },
        async closeWindow() {},
      };
      const shortcutKeydown = (event) => {
        for (const listener of [...shortcutInputListeners]) {
          let consumed = false;
          listener({
            type: "key",
            gesture: {
              code: event.code,
              key: event.key,
              repeat: event.repeat,
              composing: event.isComposing,
              defaultPrevented: event.defaultPrevented,
              control: event.ctrlKey,
              alt: event.altKey,
              shift: event.shiftKey,
              meta: event.metaKey,
            },
            context: {
              modal: document.querySelector("[role='dialog'], dialog[open]"),
              region: text(event.target?.closest?.("[data-conversation-region]")?.getAttribute("data-conversation-region")),
              target: event.target,
            },
            consume() {
              consumed = true;
              event.preventDefault();
            },
          });
          if (consumed) break;
        }
      };
      window.addEventListener("keydown", shortcutKeydown);
      const runtime = await loadDshClientPlugin({
        loader,
        id: moduleId,
        modules: {
          react: React,
          "react/jsx-runtime": ReactJSXRuntime,
          "react/jsx-dev-runtime": ReactJSXRuntime,
          "react-dom": ReactDOM,
          "react-dom/client": { createRoot },
          "@deepseek-ai/dsh-client-ui-primitives": DshPrimitives,
          "@deepseek-ai/dsh-client-ui-dockkit": DshDockkit,
          "@deepseek-ai/dsh-client-store": DshClientStore,
          "@deepseek-ai/dsh-client-ui-slots": DshClientSlots,
          "@deepseek-ai/cordis": CORDIS_SHIM,
          "@deepseek-ai/dsh-invariants": INVARIANTS_SHIM,
          "@deepseek-ai/dsh-brand": BRAND_SHIM,
          "@deepseek-ai/dsh-util-crypto": CRYPTO_SHIM,
          zod: Zod,
          "@tanstack/react-virtual": TanStackReactVirtual,
        },
        slots,
        locale,
        eventBus,
        services: {
          slots,
          commands: clientCommands,
          commandUi: clientCommandUi,
          input: createInputService(),
          locale,
          theme: runtimeServices.theme,
          timer: runtimeServices.timer,
          fileUpload: runtimeServices.fileUpload,
          jobs,
          remote: runtimeServices.remote,
          "remote.commands": runtimeServices.remote.commands,
          "remote.job": runtimeServices.remote.job,
          "remote.pluginManager": runtimeServices.remote.pluginManager,
          "remote.pluginInventory": runtimeServices.remote.pluginInventory,
          "remote.pluginRegistryProbe": runtimeServices.remote.pluginRegistryProbe,
          "remote.directoryPicker": runtimeServices.remote.directoryPicker,
          "remote.toolResults": runtimeServices.remote.toolResults,
          "remote.session": runtimeServices.remote.session,
          "remote.schedule": runtimeServices.remote.schedule,
          "remote.credentials": runtimeServices.remote.credentials,
          "remote.llm": runtimeServices.remote.llm,
          "remote.workspaceFiles": runtimeServices.remote.workspaceFiles,
          "remote.settings": runtimeServices.remote.settings,
          sessions: runtimeServices.sessions,
          configForms: clientConfig.configForms,
          settingsSchema,
          fileDrag,
          uiConversation,
          connection: { state: connectionState },
          resources,
          sidebarRight,
          sidebarRightTabs,
          contextMenus,
          uiSession,
          layout: runtimeServices.layout,
          shortcuts,
          uiWorkspace: runtimeServices.uiWorkspace,
          workspaces: runtimeServices.workspaces,
          remoteAccess: runtimeServices.remoteAccess,
          conversation: runtimeServices.conversation,
        },
      });
      loadedModules.push({ runtime, slots, jobs, runtimeServices, clientConfig, disposeSidebarFacades });
    }
    return {
      async dispose() {
        for (let index = loadedModules.length - 1; index >= 0; index -= 1) {
          await loadedModules[index].runtime.dispose();
          await loadedModules[index].slots.dispose();
          await loadedModules[index].jobs.dispose();
          await loadedModules[index].runtimeServices.dispose();
          loadedModules[index].clientConfig.dispose();
          loadedModules[index].disposeSidebarFacades();
        }
        loadedModules.length = 0;
        for (const entry of moduleRecords) loader.remove(entry.id, entry.record);
      },
    };
  } catch (error) {
    for (let index = loadedModules.length - 1; index >= 0; index -= 1) {
      await loadedModules[index].runtime.dispose();
      await loadedModules[index].slots.dispose();
      await loadedModules[index].jobs.dispose();
      await loadedModules[index].runtimeServices.dispose();
      loadedModules[index].clientConfig.dispose();
      loadedModules[index].disposeSidebarFacades();
    }
    for (const entry of moduleRecords) loader.remove(entry.id, entry.record);
    throw error;
  }
}

export default {
  name: "DshCompatibleHostPage",
  props: {
    uiContext: { type: Object, default: () => ({}) },
    uiActions: { type: Object, default: () => ({}) },
    uiNavigate: { type: Function, default: null },
  },
  setup(props) {
    const Vue = globalThis.__AMITIA_VUE__;
    if (!Vue) throw new Error("Amitia Vue runtime is unavailable");
    const status = Vue.ref({ packages: [] });
    const spec = Vue.ref("");
    const pluginId = Vue.ref("");
    const error = Vue.ref("");
    const busy = Vue.ref(false);
    const activeClients = new Map();
    const syncCurrentSession = () => {
      const context = props.uiContext && typeof props.uiContext === "object" ? props.uiContext : {};
      const conversation = context.conversation && typeof context.conversation === "object" ? context.conversation : {};
      globalThis.__dshCurrentSessionId = text(context.conversationId || conversation.id) || undefined;
      globalThis.__dshCurrentCharacterId = text(context.characterId || conversation.characterId) || undefined;
    };
    Vue.watch?.(() => props.uiContext, syncCurrentSession, { deep: true, immediate: true });
    syncCurrentSession();

    async function command(input) {
      if (typeof props.uiActions?.command !== "function") throw new Error("DSH host command action is unavailable");
      return await props.uiActions.command(input);
    }

    async function loadEnabledClients() {
      for (const plugin of status.value.packages ?? []) {
        const hasClient = Array.isArray(plugin.clientEntries) && plugin.clientEntries.length > 0 || Boolean(plugin.clientEntry);
        if (!plugin.enabled || !hasClient || activeClients.has(plugin.pluginId)) continue;
        activeClients.set(plugin.pluginId, await instantiateClient(plugin));
      }
    }

    async function refresh() {
      error.value = "";
      try {
        status.value = await command({ action: "status" });
        await loadEnabledClients();
      } catch (failure) {
        error.value = rawError(failure);
      }
    }

    async function run(task) {
      busy.value = true;
      error.value = "";
      try {
        await task();
        status.value = await command({ action: "status" });
        await loadEnabledClients();
      } catch (failure) {
        error.value = rawError(failure);
      } finally {
        busy.value = false;
      }
    }

    async function install() {
      await run(async () => {
        await command({ action: "install", spec: spec.value });
        spec.value = "";
      });
    }

    async function setEnabled(id, enabled) {
      await run(async () => {
        await command({ action: "set-enabled", pluginId: id, enabled });
        if (!enabled) {
          const client = activeClients.get(id);
          activeClients.delete(id);
          await client?.dispose();
        }
      });
    }

    async function remove(id) {
      await run(async () => {
        const client = activeClients.get(id);
        activeClients.delete(id);
        await client?.dispose();
        await command({ action: "remove", pluginId: id });
      });
    }

    globalThis.__dshHostCommand = command;
    globalThis.__dshUiActions = props.uiActions;
    globalThis.__dshUiNavigate = props.uiNavigate;
    Vue.onMounted(refresh);
    Vue.onBeforeUnmount(() => {
      for (const client of activeClients.values()) void client?.dispose();
      activeClients.clear();
      delete globalThis.__dshHostCommand;
      delete globalThis.__dshUiActions;
      delete globalThis.__dshUiNavigate;
      delete globalThis.__dshCurrentSessionId;
      delete globalThis.__dshCurrentCharacterId;
    });

    return () => Vue.h("section", { class: "dsh-host-page" }, [
      Vue.h("div", { class: "dsh-host-page__title" }, "DeepSeek Harness 插件兼容宿主"),
      Vue.h("div", { class: "dsh-host-page__row" }, [
        Vue.h("button", {
          class: "dsh-host-page__button",
          onClick: async () => {
            if (typeof props.uiNavigate !== "function") {
              error.value = rawError(new Error("DSH 会话页面导航不可用：uiNavigate 未由宿主提供"));
              return;
            }
            try {
              await props.uiNavigate("/chat");
            } catch (cause) {
              error.value = rawError(cause);
            }
          },
        }, "打开官方会话页面"),
      ]),
      Vue.h("div", { class: "dsh-host-page__row" }, [
        Vue.h("input", {
          class: "dsh-host-page__input",
          value: spec.value,
          placeholder: "npm 包、Git 地址或本地 tarball",
          onInput: (event) => { spec.value = event.target.value; },
        }),
        Vue.h("button", {
          class: "dsh-host-page__button dsh-host-page__button--primary",
          disabled: busy.value,
          onClick: install,
        }, "安装 DSH 插件"),
        Vue.h("button", {
          class: "dsh-host-page__button",
          disabled: busy.value,
          onClick: refresh,
        }, "刷新状态"),
      ]),
      error.value ? Vue.h("pre", { class: "dsh-host-page__error" }, error.value) : null,
      Vue.h("div", { class: "dsh-host-page__row" }, [
        Vue.h("input", {
          class: "dsh-host-page__input",
          value: pluginId.value,
          placeholder: "已安装 DSH 插件 ID",
          onInput: (event) => { pluginId.value = event.target.value; },
        }),
      ]),
      Vue.h("table", { class: "dsh-host-page__table" }, [
        Vue.h("thead", null, [
          Vue.h("tr", null, [
            Vue.h("th", null, "插件"),
            Vue.h("th", null, "版本"),
            Vue.h("th", null, "状态"),
            Vue.h("th", null, "入口"),
            Vue.h("th", null, "操作"),
          ]),
        ]),
        Vue.h("tbody", null, (status.value.packages ?? []).map((plugin) => Vue.h("tr", { key: plugin.pluginId }, [
          Vue.h("td", null, [
            Vue.h("strong", null, plugin.name),
            Vue.h("div", null, plugin.pluginId),
          ]),
          Vue.h("td", null, plugin.version),
          Vue.h("td", null, plugin.enabled ? "已启用" : "已停用"),
          Vue.h("td", null, [
            Vue.h("div", null, `host: ${plugin.entry}`),
            Vue.h("div", null, `client: ${plugin.clientEntry || "无"}`),
          ]),
          Vue.h("td", null, [
            Vue.h("div", { class: "dsh-host-page__actions" }, [
              Vue.h("button", {
                class: "dsh-host-page__button",
                disabled: busy.value,
                onClick: () => setEnabled(plugin.pluginId, !plugin.enabled),
              }, plugin.enabled ? "停用" : "启用"),
              Vue.h("button", {
                class: "dsh-host-page__button dsh-host-page__button--danger",
                disabled: busy.value,
                onClick: () => remove(plugin.pluginId),
              }, "移除"),
            ]),
          ]),
        ]))),
      ]),
    ]);
  },
};
