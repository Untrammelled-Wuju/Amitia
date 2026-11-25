import * as React from "react";
import * as ReactJSXRuntime from "react/jsx-runtime";
import * as ReactDOM from "react-dom";
import { createRoot } from "react-dom/client";
import * as DshPrimitives from "@deepseek-ai/dsh-client-ui-primitives";
import * as DshClientStore from "@deepseek-ai/dsh-client-store";
import * as DshClientSlots from "@deepseek-ai/dsh-client-ui-slots";
import {
  DshClientEventBus,
  DshClientModuleLoader,
  installDshModuleLoader,
  loadDshClientPlugin,
} from "../client-abi/module-loader.mjs";
import { createDshClientJobsController } from "../client-abi/jobs-controller.mjs";
import { createDshClientResourceRegistry } from "../client-abi/resource-registry.mjs";
import { createDshClientRuntimeServices } from "../client-abi/runtime-services.mjs";
import { createDshFileDragActions } from "../client-abi/file-drag.mjs";
import { DSH_SLOT_ALIASES, mapDshSlot as resolveDshSlot } from "../client-abi/slot-map.mjs";
import { acquireDshIconSprite } from "../client-ui/icon-sprite.mjs";

const CORDIS_SHIM = Object.freeze({
  Service: class Service {
    constructor(context, name) {
      this.ctx = context;
      this.name = name;
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
const KNOWN_AMITIA_SLOTS = new Set(Object.values(DSH_SLOT_ALIASES));

function text(value) {
  return value == null ? "" : String(value).trim();
}

function decodeSourceBase64(value) {
  const binary = globalThis.atob(String(value || ""));
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

async function readMobileClientSource(plugin, entry) {
  const chunks = [];
  let offset = 0;
  let total = 0;
  while (true) {
    const value = await invokeCommand({
      action: "client-source",
      pluginId: plugin.pluginId,
      entryId: entry.id,
      offset,
      length: 256 * 1024,
    });
    if (typeof value?.code === "string" && value.dataBase64 === undefined) return value.code;
    const bytes = decodeSourceBase64(value?.dataBase64);
    chunks.push(bytes);
    total += bytes.length;
    if (value?.done) break;
    const nextOffset = Number(value?.nextOffset);
    if (!Number.isSafeInteger(nextOffset) || nextOffset <= offset || nextOffset > Number(value?.totalBytes)) {
      throw new Error(`DSH mobile client source ${plugin.pluginId}#${entry.id} returned invalid offset ${value?.nextOffset}`);
    }
    offset = nextOffset;
  }
  const bytes = new Uint8Array(total);
  let cursor = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, cursor);
    cursor += chunk.length;
  }
  return new TextDecoder().decode(bytes);
}

function rawError(error) {
  if (error instanceof Error) {
    const cause = error.cause ? `\ncause: ${rawError(error.cause)}` : "";
    return `${error.name}: ${error.message}\n${error.stack ?? ""}${cause}`;
  }
  if (error && typeof error === "object") {
    try {
      return JSON.stringify(error, null, 2);
    } catch {
      return String(error);
    }
  }
  return String(error);
}

function compare(left, right) {
  return Number(left ?? 0) - Number(right ?? 0);
}

async function invokeCommand(input) {
  let output = await window.AmitiaHost.invokeAction("command", input);
  for (let depth = 0; depth < 6; depth += 1) {
    if (typeof output === "string") {
      try {
        output = JSON.parse(output);
      } catch {
        break;
      }
    }
    if (!output || typeof output !== "object") break;
    if (output.ok === false) throw new Error(rawError(output.error ?? output));
    if (output.ok === true && output.output !== undefined) {
      output = output.output;
      continue;
    }
    if ((output.status === "failed" || output.status === "error") && output.error != null) {
      throw new Error(rawError(output.error));
    }
    if (
      Object.hasOwn(output, "packages") ||
      Object.hasOwn(output, "package") ||
      Object.hasOwn(output, "code")
    ) {
      break;
    }
    const nested = ["output", "data", "result", "structured"].find((key) => output[key] !== undefined);
    if (!nested) break;
    output = output[nested];
  }
  if (
    output &&
    typeof output === "object" &&
    (output.ok === false || output.status === "failed" || output.status === "error") &&
    output.error != null
  ) {
    throw new Error(rawError(output.error));
  }
  return output;
}

function upperFirst(value) {
  return value ? value[0].toUpperCase() + value.slice(1) : value;
}

function normalizeManagedStore(value, context) {
  if (value == null) return { store: undefined, actions: {}, dispose() {} };
  if (typeof value === "function") {
    value = value(context);
  }
  if (value && value.__amitiaSlotStoreResource === true) {
    return {
      store: value.store,
      actions: value.actions ?? {},
      subscribe: value.subscribe,
      dispose: value.dispose,
    };
  }
  if (value && value.__amitiaManagedSlotStoreHandle === true) {
    const instance = value.create(context.scope ?? "root");
    return {
      store: instance.getSnapshot(),
      actions: instance.actions ?? {},
      subscribe: instance.subscribe.bind(instance),
      getSnapshot: instance.getSnapshot.bind(instance),
      dispose: instance.dispose.bind(instance),
    };
  }
  if (value && typeof value.create === "function" && value.spec && typeof value.spec === "object") {
    const scope = text(context.scope);
    const instance = value.create(scope && scope !== "root" ? scope : undefined);
    return {
      store: instance.getSnapshot(),
      actions: instance.actions ?? {},
      subscribe: instance.subscribe.bind(instance),
      getSnapshot: instance.getSnapshot.bind(instance),
      dispose() {},
    };
  }
  return { store: value, actions: {}, dispose() {} };
}

class MobileDshSlotRegistry {
  definitions = new Map();
  registrations = new Map();
  injections = new Map();
  listeners = new Set();
  sequence = 0;
  revision = 0;

  constructor(context) {
    const target = text(context.slotId);
    for (const slotId of KNOWN_AMITIA_SLOTS) {
      this.definitions.set(slotId, {
        slotId,
        kind: slotId === target ? text(context.slotKind) || "list" : "list",
        layout: slotId === target ? text(context.slotLayout) || "stack" : "stack",
        scope: slotId === target ? text(context.slotScope) || "root" : "root",
      });
    }
    if (target) {
      this.definitions.set(target, {
        slotId: target,
        kind: text(context.slotKind) || this.definitions.get(target)?.kind || "list",
        layout: text(context.slotLayout) || "stack",
        scope: text(context.slotScope) || "root",
      });
    }
  }

  getDefinition(slotId) {
    return this.definitions.get(text(slotId));
  }

  declare(definition) {
    const slotId = text(definition?.slotId);
    if (!slotId) throw new Error("DSH mobile child slot id is required");
    const existing = this.definitions.get(slotId);
    if (existing && existing.parentContributionId && existing.parentContributionId !== definition.parentContributionId) {
      throw new Error(`DSH mobile child slot ${slotId} already belongs to ${existing.parentContributionId}`);
    }
    const active = {
      slotId,
      kind: text(definition.kind) || "list",
      layout: text(definition.layout) || "stack",
      scope: text(definition.scope) || "root",
      supportedKinds: Array.isArray(definition.supportedKinds) ? definition.supportedKinds : ["panel"],
      parentContributionId: text(definition.parentContributionId) || undefined,
    };
    this.definitions.set(slotId, active);
    this.notify();
    return {
      definition: active,
      dispose: () => {
        if (this.definitions.get(slotId) === active) {
          this.definitions.delete(slotId);
          this.registrations.delete(slotId);
          this.notify();
        }
      },
    };
  }

  register(pluginId, slotId, key, component, options = {}) {
    const mapped = text(slotId);
    const contributionKey = text(key);
    if (!this.definitions.has(mapped)) throw new Error(`DSH mobile slot ${mapped} is not declared`);
    if (!contributionKey) throw new Error("DSH mobile slot contribution key is required");
    const group = this.registrations.get(mapped) ?? [];
    if (group.some((item) => item.key === contributionKey)) {
      throw new Error(`DSH mobile slot ${mapped}/${contributionKey} is already registered`);
    }
    const registration = {
      pluginId,
      slotId: mapped,
      key: contributionKey,
      component,
      options: { ...options },
      sequence: ++this.sequence,
    };
    group.push(registration);
    this.registrations.set(mapped, group);
    for (const injection of this.injections.get(mapped) ?? []) {
      Promise.resolve(injection(this.getDefinition(mapped))).catch((error) => {
        console.error("[original-error]", error);
      });
    }
    this.notify();
    let active = true;
    return () => {
      if (!active) return;
      active = false;
      const index = group.indexOf(registration);
      if (index >= 0) group.splice(index, 1);
      if (group.length === 0) this.registrations.delete(mapped);
      this.notify();
    };
  }

  async inject(slotId, callback) {
    const mapped = text(slotId);
    if (!this.definitions.has(mapped)) throw new Error(`DSH mobile slot ${mapped} is not declared`);
    if (typeof callback !== "function") throw new Error("DSH mobile slot injection callback must be a function");
    const group = this.injections.get(mapped) ?? new Set();
    group.add(callback);
    this.injections.set(mapped, group);
    const effect = await callback(this.getDefinition(mapped));
    return async () => {
      group.delete(callback);
      if (group.size === 0) this.injections.delete(mapped);
      if (typeof effect === "function") await effect();
    };
  }

  entriesOfSlot(slotId) {
    return [...(this.registrations.get(text(slotId)) ?? [])];
  }

  subscribe(listener) {
    if (typeof listener !== "function") throw new Error("DSH mobile slot listener must be a function");
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  notify() {
    this.revision += 1;
    for (const listener of [...this.listeners]) listener();
  }

  dispose() {
    this.registrations.clear();
    this.injections.clear();
    this.listeners.clear();
  }
}

class MobileLocaleBridge {
  dictionaries = new Map();
  listeners = new Set();
  locale;

  constructor(locale) {
    this.locale = text(locale) || "zh-CN";
  }

  register(ownerId, namespace, dictionaries) {
    const twoArgumentForm = arguments.length === 2 && namespace && typeof namespace === "object" && !Array.isArray(namespace);
    const name = text(twoArgumentForm ? ownerId : namespace);
    const values = twoArgumentForm ? namespace : dictionaries;
    if (!name) throw new Error("DSH locale namespace is required");
    this.dictionaries.set(name, {
      ownerId: twoArgumentForm ? name : ownerId,
      dictionaries: values ?? {},
    });
    this.notify();
    return () => {
      this.dictionaries.delete(name);
      this.notify();
    };
  }

  setLocale(locale) {
    this.locale = text(locale) || this.locale;
    this.notify();
  }

  getLocale() {
    return this.locale;
  }

  bind(namespace) {
    const name = text(namespace);
    return (key, params = {}) => {
      const entry = this.dictionaries.get(name);
      const language = this.locale.split("-")[0];
      const table = entry?.dictionaries?.[this.locale] ?? entry?.dictionaries?.[language] ?? entry?.dictionaries?.zh ?? entry?.dictionaries?.en ?? {};
      const template = table[text(key)] ?? text(key);
      return Object.entries(params).reduce(
        (value, [name, replacement]) => value.replaceAll(`{${name}}`, String(replacement)),
        template,
      );
    };
  }

  subscribe(listener) {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  notify() {
    for (const listener of [...this.listeners]) listener();
  }

  dispose() {
    this.dictionaries.clear();
    this.listeners.clear();
  }
}

function registerLocalConversation(runtime, moduleId, definition, index) {
  if (runtime) {
    runtime.register(moduleId, definition, index);
  }
  return () => {};
}

function createLocalSlotBridge(pluginId, resources, registry) {
  const disposers = [];
  const bridge = {
    async register(options, component) {
      const slotId = resolveDshSlot(options?.name, (mapped) => registry.getDefinition(mapped));
      const key = text(options?.key) || text(options?.id) || `${slotId}:${disposers.length}`;
      const children = new Map();
      const childDeclarations = Object.entries(options?.children ?? {}).map(([name, raw]) => {
        const spec = raw && typeof raw === "object" ? raw : {};
        const childId = resolveDshSlot(name, (mapped) => registry.getDefinition(mapped) || mapped);
        const declared = registry.declare({
          slotId: childId,
          contractVersion: Number(spec.contractVersion) || 1,
          supportedKinds: Array.isArray(spec.supportedKinds) ? spec.supportedKinds : ["panel"],
          kind: spec.kind || "list",
          layout: spec.layout,
          scope: spec.scope,
          parentContributionId: `${slotId}:${key}`,
        });
        children.set(name, declared);
        return declared;
      });
      const dispose = registry.register(pluginId, slotId, key, component, {
        ordering: Number.isFinite(Number(options?.order ?? options?.ordering)) ? Number(options.order ?? options.ordering) : 0,
        priority: Number.isFinite(Number(options?.priority)) ? Number(options.priority) : 0,
        children: [...children.values()],
        store: options?.store,
        inject: options?.inject,
        cellId: text(options?.id) || undefined,
        entryKey: text(options?.entryKey) || undefined,
        label: options?.label,
        select: options?.select,
        localeNamespace: typeof options?.locale === "string" ? options.locale : undefined,
      });
      const combined = async () => {
        await dispose();
        for (const child of childDeclarations.reverse()) await child.dispose();
      };
      disposers.push(combined);
      return combined;
    },
    async inject(slotId, callback) {
      const mapped = resolveDshSlot(slotId, (id) => registry.getDefinition(id));
      const invoke = (definition) => callback({
        ...definition,
        name: mapped,
      });
      const existing = registry.getDefinition(mapped);
      if (existing) {
        const effect = await invoke(existing);
        const dispose = typeof effect === "function" ? effect : () => {};
        disposers.push(dispose);
        return dispose;
      }
      const dispose = await registry.inject(mapped, invoke);
      disposers.push(dispose);
      return dispose;
    },
    entriesOfSlot(slotId) {
      return registry.entriesOfSlot(resolveDshSlot(slotId, (id) => registry.getDefinition(id)));
    },
    registerFactory(options, factory) {
      if (typeof factory !== "function") throw new Error("DSH mobile slot factory must be a function");
      return this.register(options, factory);
    },
    subscribe(listener) {
      return registry.subscribe(listener);
    },
    async dispose() {
      for (let index = disposers.length - 1; index >= 0; index -= 1) {
        await disposers[index]?.();
      }
      disposers.length = 0;
    },
  };
  return bridge;
}

const volatileClientConfigs = new Map();

function cloneConfigValue(value) {
  if (value === undefined) return {};
  try {
    return structuredClone(value);
  } catch {
    return JSON.parse(JSON.stringify(value));
  }
}

function clientConfigStorageKey(moduleId, namespace) {
  return `amitia:dsh:config:${encodeURIComponent(moduleId)}:${encodeURIComponent(namespace)}`;
}

function readClientConfig(moduleId, namespace) {
  const key = clientConfigStorageKey(moduleId, namespace);
  try {
    const raw = window.localStorage?.getItem(key);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === "object") {
        return {
          value: cloneConfigValue(parsed.value),
          revision: Number.isInteger(parsed.revision) ? parsed.revision : 0,
        };
      }
    }
  } catch {
    const cached = volatileClientConfigs.get(key);
    if (cached) return cloneConfigValue(cached);
  }
  const cached = volatileClientConfigs.get(key);
  return cached ? cloneConfigValue(cached) : { value: {}, revision: 0 };
}

function writeClientConfig(moduleId, namespace, record) {
  const key = clientConfigStorageKey(moduleId, namespace);
  const next = cloneConfigValue(record);
  volatileClientConfigs.set(key, next);
  try {
    window.localStorage?.setItem(key, JSON.stringify(next));
  } catch {
    return;
  }
}

function applyClientConfigOperations(input, operations) {
  let next = cloneConfigValue(input);
  for (const operation of [...operations]) {
    const path = Array.isArray(operation?.path)
      ? operation.path.map((item) => String(item))
      : [];
    if (path.length === 0) throw new Error("DSH config operation path is required");
    if (operation.op !== "set" && operation.op !== "unset") {
      throw new Error(`unsupported DSH config operation ${String(operation.op)}`);
    }
    let parent = next;
    for (let index = 0; index < path.length - 1; index += 1) {
      const segment = path[index];
      if (!parent[segment] || typeof parent[segment] !== "object" || Array.isArray(parent[segment])) {
        parent[segment] = {};
      }
      parent = parent[segment];
    }
    const leaf = path.at(-1);
    if (operation.op === "set") parent[leaf] = cloneConfigValue(operation.value);
    else delete parent[leaf];
  }
  return next;
}

function withClientConfigDefaults(value) {
  const source = value && typeof value === "object" && !Array.isArray(value) ? value : {};
  return new Proxy(source, {
    get(target, property) {
      if (Reflect.has(target, property)) return Reflect.get(target, property);
      const key = String(property);
      if (key === "preference") return "system";
      if (key === "fontSize" || key.endsWith("FontSize")) return 14;
      if (key.endsWith("FontFamily")) return "";
      return undefined;
    },
  });
}

function createMobileConfigForms(moduleId) {
  const forms = new Map();
  const listeners = new Map();
  const remoteListeners = new Map();

  function view(namespace, snapshot) {
    return {
      ns: namespace,
      revision: snapshot.revision,
      base: snapshot.base,
      user: snapshot.user,
      value: snapshot.value,
      schema: {},
    };
  }

  function notifyRemote(event, payload) {
    for (const listener of [...(remoteListeners.get(event) ?? [])]) {
      Promise.resolve(listener(payload)).catch((error) => {
        console.error("[original-error]", error);
      });
    }
  }

  function form(namespace) {
    const key = text(namespace);
    if (!key) throw new Error("DSH config namespace is required");
    const existing = forms.get(key);
    if (existing) return existing;
    const stored = readClientConfig(moduleId, key);
    let snapshot = {
      status: "ready",
      value: withClientConfigDefaults(cloneConfigValue(stored.value)),
      base: cloneConfigValue(stored.value),
      user: cloneConfigValue(stored.value),
      revision: Number.isInteger(stored.revision) ? stored.revision : 0,
      writable: true,
    };
    const subscribers = listeners.get(key) ?? new Set();
    listeners.set(key, subscribers);
    const controller = {
      getSnapshot() {
        return snapshot;
      },
      subscribe(listener) {
        if (typeof listener !== "function") throw new Error("DSH config listener must be a function");
        subscribers.add(listener);
        return () => subscribers.delete(listener);
      },
      async set(field, value) {
        return controller.mutate([{ op: "set", path: [field], value }]);
      },
      async unset(field) {
        return controller.mutate([{ op: "unset", path: [field] }]);
      },
      async mutate(operations, expectedRevision) {
        if (expectedRevision != null && Number(expectedRevision) !== snapshot.revision) return false;
        const value = applyClientConfigOperations(snapshot.value, operations);
        snapshot = {
          ...snapshot,
          value: withClientConfigDefaults(value),
          base: cloneConfigValue(value),
          user: cloneConfigValue(value),
          revision: snapshot.revision + 1,
        };
        writeClientConfig(moduleId, key, {
          value: snapshot.value,
          revision: snapshot.revision,
        });
        for (const subscriber of [...subscribers]) subscriber();
        notifyRemote("settings/document-updated", { namespace: key, revision: snapshot.revision });
        return true;
      },
      dispose() {
        subscribers.clear();
      },
    };
    forms.set(key, controller);
    return controller;
  }

  const settings = {
    async describe() {
      return {
        ok: true,
        value: {
          writable: true,
          namespaces: [...forms.entries()].map(([namespace, controller]) => view(namespace, controller.getSnapshot())),
        },
      };
    },
    async mutate(namespace, operations, revision) {
      const controller = form(namespace);
      const updated = await controller.mutate(operations, revision);
      if (!updated) {
        return {
          ok: false,
          error: {
            code: "SETTINGS_REVISION_CONFLICT",
            message: `DSH config ${namespace} revision ${revision} is stale`,
          },
        };
      }
      return {
        ok: true,
        value: view(namespace, controller.getSnapshot()),
      };
    },
  };

  const baseConfigForms = {
    get(namespace) {
      return form(namespace);
    },
    describe() {
      return {
        getSnapshot() {
          return {
            status: "ready",
            writable: true,
            view: {
              writable: true,
              namespaces: [...forms.entries()].map(([namespace, controller]) => view(namespace, controller.getSnapshot())),
            },
          };
        },
        subscribe(listener) {
          const disposers = [...forms.values()].map((controller) => controller.subscribe(listener));
          return () => {
            for (const dispose of disposers) dispose();
          };
        },
        ensure() {
          return Promise.resolve();
        },
      };
    },
    whileServed(namespaces, register) {
      const required = new Set((namespaces ?? []).map((item) => text(item)).filter(Boolean));
      if (required.size === 0) return register(new Set());
      const served = new Set([...required].filter((namespace) => forms.has(namespace)));
      return served.size ? register(served) : () => {};
    },
    dispose() {
      for (const controller of forms.values()) controller.dispose();
      forms.clear();
      listeners.clear();
      remoteListeners.clear();
    },
  };
  const configForms = new Proxy(baseConfigForms, {
    get(target, property, receiver) {
      if (Reflect.has(target, property)) return Reflect.get(target, property, receiver);
      return typeof property === "string" ? form(property).getSnapshot().value : undefined;
    },
  });

  function onRemote(event, listener) {
    const name = text(event);
    if (!name || typeof listener !== "function") throw new Error("DSH remote listener is invalid");
    const group = remoteListeners.get(name) ?? new Set();
    group.add(listener);
    remoteListeners.set(name, group);
    return () => group.delete(listener);
  }

  return { configForms, settings, onRemote };
}

function createServices(moduleId, resources, registry, locale, conversationDefinitions, eventBus) {
  const noopDisposer = () => {};
  const mobileConfig = createMobileConfigForms(moduleId);
  const runtimeServices = createDshClientRuntimeServices({
    command: (input) => invokeCommand(input),
    navigate: (target) => window.AmitiaHost.invokeAction("navigate", { target }),
    emit: eventBus ? (type, payload) => eventBus.emit(type, payload) : undefined,
    hostActions: {
      toggleSidebar: () => window.AmitiaHost.action("openWorkspaceSidebar"),
    },
  });
  const remote = {
    ...runtimeServices.remote,
    settings: mobileConfig.settings,
    $on: mobileConfig.onRemote,
  };
  const uiConversation = {
    events: {
      register(definition) {
        const index = conversationDefinitions.length;
        conversationDefinitions.push({ moduleId, definition });
        return registerLocalConversation(undefined, moduleId, definition, index);
      },
    },
    binding() {
      return {
        target() {
          return {
            getSnapshot() {
              return { nodes: { turnDataSource() { return []; } } };
            },
          };
        },
      };
    },
  };
  const sidebarRightTabs = {
    definitions: [],
    register(definition) {
      if (!definition || typeof definition !== "object") throw new Error("DSH mobile right tab definition is required");
      this.definitions.push(definition);
      return () => {
        const index = this.definitions.indexOf(definition);
        if (index >= 0) this.definitions.splice(index, 1);
      };
    },
    resolve(address) {
      return this.definitions.find((definition) => {
        if (typeof definition.canOpen === "function") return definition.canOpen(address);
        const patterns = Array.isArray(definition.patterns) ? definition.patterns : [];
        return patterns.some((pattern) => address.startsWith(String(pattern).replaceAll("**", "")));
      });
    },
  };
  const sidebarRight = {
    tabsIn() {
      return [];
    },
    mounted: { getSnapshot() { return true; }, subscribe() { return noopDisposer; } },
    openResource(address, options = {}) {
      const definition = sidebarRightTabs.resolve(address);
      const title = typeof definition?.title === "function" ? definition.title() : definition?.title;
      return window.AmitiaHost.openRightPane({
        address,
        title,
        params: options.params,
      });
    },
  };
  const services = {
    slots: registry,
    locale,
    theme: runtimeServices.theme,
    timer: runtimeServices.timer,
    fileUpload: runtimeServices.fileUpload,
    configForms: mobileConfig.configForms,
    fileDrag: createDshFileDragActions(),
    jobs: createDshClientJobsController((input) => window.AmitiaHost.invokeAction("command", input)),
    remote,
    "remote.commands": remote.commands,
    "remote.pluginManager": remote.pluginManager,
    "remote.pluginInventory": remote.pluginInventory,
    "remote.pluginRegistryProbe": remote.pluginRegistryProbe,
    "remote.directoryPicker": remote.directoryPicker,
    "remote.toolResults": remote.toolResults,
    "remote.session": remote.session,
    "remote.schedule": remote.schedule,
    "remote.credentials": remote.credentials,
    "remote.llm": remote.llm,
    "remote.workspaceFiles": remote.workspaceFiles,
    sessions: runtimeServices.sessions,
    uiConversation,
    resources,
    sidebarRight,
    sidebarRightTabs,
    layout: runtimeServices.layout,
    shortcuts: { catalog: { getSnapshot() { return []; }, subscribe() { return noopDisposer; } } },
    uiWorkspace: runtimeServices.uiWorkspace,
    workspaces: runtimeServices.workspaces,
    remoteAccess: runtimeServices.remoteAccess,
    conversation: runtimeServices.conversation,
  };
  Object.defineProperty(services, "__dispose", {
    value: runtimeServices.dispose,
    enumerable: false,
  });
  return services;
}

async function loadMobileClients(status, registry, locale, conversationDefinitions) {
  const loader = installDshModuleLoader();
  if (!(loader instanceof DshClientModuleLoader)) throw new Error("DSH mobile client module loader is invalid");
  const loaded = [];
  const errors = [];
  for (const plugin of status.packages ?? []) {
    const clientEntries = Array.isArray(plugin.clientEntries) && plugin.clientEntries.length
      ? plugin.clientEntries
      : plugin.clientEntry
        ? [{ id: "root", entry: plugin.clientEntry }]
        : [];
    if (!plugin.enabled || clientEntries.length === 0) continue;
    try {
      const before = new Set(loader.list());
      for (const entry of clientEntries) {
        const source = await readMobileClientSource(plugin, entry);
        const executable = text(source)
          .replace(/\/\/# sourceMappingURL=.*$/gm, "")
          .replace(/\/\*[\s\S]*?\*\//g, "")
          .replace(/\/\/.*$/gm, "")
          .trim();
        if (!executable || /^export\s*\{\s*\}\s*;?$/.test(executable)) continue;
        new Function(`${source}\n//# sourceURL=amitia-dsh-mobile://${encodeURIComponent(plugin.pluginId)}/${encodeURIComponent(entry.id)}`)();
      }
      const moduleIds = loader.list().filter((id) => !before.has(id));
      if (moduleIds.length === 0) throw new Error(`DSH 插件 ${plugin.pluginId} 没有注册新的 client module`);
      for (const moduleId of moduleIds) {
        const resources = createDshClientResourceRegistry();
        const slots = createLocalSlotBridge(moduleId, resources, registry);
        const eventBus = new DshClientEventBus();
        const services = createServices(moduleId, resources, registry, locale, conversationDefinitions, eventBus);
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
            "@deepseek-ai/dsh-client-store": DshClientStore,
            "@deepseek-ai/dsh-client-ui-slots": DshClientSlots,
            "@deepseek-ai/cordis": CORDIS_SHIM,
            "@deepseek-ai/dsh-invariants": INVARIANTS_SHIM,
            "@deepseek-ai/dsh-brand": BRAND_SHIM,
          },
          slots,
          locale,
          eventBus,
          services,
        });
        loaded.push({ runtime, slots, resources, services });
      }
    } catch (error) {
      errors.push(`${plugin.pluginId}: ${rawError(error)}`);
    }
  }
  return {
    errors,
    async dispose() {
      for (let index = loaded.length - 1; index >= 0; index -= 1) {
        const item = loaded[index];
        await item.runtime.dispose();
        await item.slots.dispose();
        await item.services.jobs.dispose();
        await item.resources.dispose?.();
        await item.services.__dispose?.();
      }
      loaded.length = 0;
    },
  };
}

function selectRegistrations(registry, slotId, owner, options = {}) {
  const definition = registry.getDefinition(slotId);
  if (!definition) return [];
  let items = registry.entriesOfSlot(slotId);
  const entryKey = text(options.entryKey);
  const only = text(options.only);
  if (definition.kind === "keyed") {
    items = items.filter((item) => text(item.options.entryKey) === entryKey);
  } else if (definition.kind === "list" && only) {
    items = items.filter((item) => text(item.options.cellId) === only);
  } else if (definition.kind === "chain") {
    items = items.filter((item) => {
      if (typeof item.options.select !== "function") return true;
      const matched = item.options.select(owner);
      if (matched == null) return false;
      item.matched = matched;
      return true;
    });
  } else if (definition.kind === "hoc" && entryKey) {
    items = items.filter((item) => !item.options.entryKey || text(item.options.entryKey) === entryKey);
  }
  return [...items].sort((left, right) => {
    if (definition.kind === "chain") {
      return compare(right.options.priority, left.options.priority) || left.sequence - right.sequence;
    }
    return compare(left.options.ordering, right.options.ordering)
      || compare(left.options.priority, right.options.priority)
      || left.sequence - right.sequence;
  });
}

function MobileContribution({ registration, owner, registry, locale, children }) {
  const definition = registry.getDefinition(registration.slotId);
  const resourceContext = {
    pluginId: registration.pluginId,
    slotId: registration.slotId,
    key: registration.key,
    scope: definition?.scope ?? "root",
    sessionId: text(owner.sessionId || owner.conversationId) || undefined,
  };
  const managedRef = React.useRef(null);
  if (!managedRef.current) {
    managedRef.current = normalizeManagedStore(registration.options.store, resourceContext);
  }
  React.useEffect(() => () => {
    void managedRef.current?.dispose?.();
  }, []);
  const managed = managedRef.current;
  const useStore = (selector = (value) => value) => {
    return React.useSyncExternalStore(
      typeof managed?.subscribe === "function"
        ? (listener) => managed.subscribe(listener)
        : () => () => {},
      () => selector(typeof managed?.getSnapshot === "function" ? managed.getSnapshot() : managed.store),
    );
  };
  const useGlobal = (selector = (value) => value) => selector(owner);
  const useSession = (selector = (value) => value) => selector(owner.conversation ?? owner);
  const injectedRef = React.useRef(null);
  if (!injectedRef.current) {
    if (typeof registration.options.inject === "function") {
      const hasStore = registration.options.store !== undefined;
      const injectArgs = definition?.scope === "root"
        ? (hasStore ? [managed.actions] : [])
        : [resourceContext.sessionId, hasStore ? managed.actions : undefined];
      injectedRef.current = registration.options.inject(...injectArgs) ?? {};
    } else {
      injectedRef.current = {};
    }
  }
  function renderSlot(slotId, childOwner = {}, options = {}) {
    const mapped = resolveDshSlot(slotId, (id) => registry.getDefinition(id));
    return React.createElement(LocalSlotOutlet, {
      slotId: mapped,
      owner: { ...owner, ...childOwner },
      registry,
      locale,
      dispatchKey: options.entryKey,
      dispatchOnly: options.only,
    });
  }
  function SessionProvider(area) {
    const sessionId = text(owner.sessionId || owner.conversationId);
    if (!sessionId) return area.empty?.() ?? null;
    return React.createElement(React.Fragment, null, area.children(sessionId));
  }
  const translator = registration.options.localeNamespace
    ? locale.bind(registration.options.localeNamespace)
    : undefined;
  const actions = new Proxy(managed.actions ?? {}, {
    get(target, property) {
      if (Object.prototype.hasOwnProperty.call(target, property)) return target[property];
      const name = text(property);
      if (!name) return undefined;
      return (input) => window.AmitiaHost.invokeAction(name, input);
    },
  });
  const props = {
    ...owner,
    slotId: registration.slotId,
    pluginId: registration.pluginId,
    contributionId: registration.key,
    sessionId: resourceContext.sessionId,
    matched: registration.matched,
    useStore,
    useGlobal,
    useSession,
    actions,
    uiContext: owner,
    uiActions: actions,
    renderSlot,
    renderSlotChain: renderSlot,
    SessionProvider,
    ...(translator ? { t: translator } : {}),
    ...injectedRef.current,
  };
  const Component = registration.component;
  return React.createElement(Component, props, children);
}

function LocalSlotOutlet({ slotId, owner = {}, registry, locale, dispatchKey, dispatchOnly }) {
  const version = React.useSyncExternalStore(
    registry.subscribe.bind(registry),
    () => registry.revision,
  );
  version;
  const definition = registry.getDefinition(slotId);
  const registrations = selectRegistrations(registry, slotId, owner, {
    entryKey: dispatchKey,
    only: dispatchOnly,
  });
  if (!definition || registrations.length === 0) return null;
  if (definition.kind === "single" || definition.kind === "keyed" || definition.kind === "chain") {
    const registration = registrations[0];
    return React.createElement(MobileContribution, {
      key: registration.key,
      registration,
      owner,
      registry,
      locale,
    });
  }
  if (definition.kind === "hoc") {
    return registrations.reduceRight(
      (children, registration) => React.createElement(MobileContribution, {
        key: registration.key,
        registration,
        owner,
        registry,
        locale,
        children,
      }),
      null,
    );
  }
  return React.createElement(
    React.Fragment,
    null,
    ...registrations.map((registration) => React.createElement(MobileContribution, {
      key: registration.key,
      registration,
      owner,
      registry,
      locale,
    })),
  );
}

export async function mount(root, context = {}) {
  if (!(root instanceof Element)) throw new Error("DSH mobile client UI root element is required");
  if (!window.AmitiaHost || typeof window.AmitiaHost.invokeAction !== "function") {
    throw new Error("AmitiaHost mobile bridge is unavailable");
  }
  const slotId = text(context.slotId);
  if (!slotId) throw new Error("DSH mobile client UI requires slotId");
  const releaseIconSprite = acquireDshIconSprite(DshPrimitives.ICON_SYMBOLS);
  root.replaceChildren();
  const host = document.createElement("div");
  host.className = "dsh-mobile-slot";
  root.append(host);
  const registry = new MobileDshSlotRegistry(context);
  const locale = new MobileLocaleBridge(context.locale);
  const conversationDefinitions = [];
  let status;
  let clients;
  try {
    status = await invokeCommand({ action: "status" });
    clients = await loadMobileClients(status, registry, locale, conversationDefinitions);
  } catch (error) {
    releaseIconSprite();
    root.replaceChildren();
    const pre = document.createElement("pre");
    pre.className = "dsh-mobile-slot__error";
    pre.textContent = rawError(error);
    root.append(pre);
    void window.AmitiaHost.resize(document.documentElement.scrollHeight).catch((failure) => {
      console.error("[original-error]", failure);
    });
    throw error;
  }
  if (clients.errors.length > 0) {
    const error = document.createElement("pre");
    error.className = "dsh-mobile-slot__error";
    error.textContent = clients.errors.join("\n\n");
    root.insertBefore(error, host);
  }
  const reactRoot = createRoot(host);
  const render = () => {
    reactRoot.render(React.createElement(LocalSlotOutlet, {
      slotId,
      owner: context,
      registry,
      locale,
      dispatchKey: context.dispatchKey,
      dispatchOnly: context.dispatchOnly,
    }));
    requestAnimationFrame(() => {
      void window.AmitiaHost.resize(document.documentElement.scrollHeight).catch((failure) => {
        console.error("[original-error]", failure);
      });
    });
  };
  const unsubscribe = registry.subscribe(render);
  const unsubscribeContext = window.AmitiaHost.subscribeContext((next) => {
    context = next && typeof next === "object" ? { ...context, ...next } : context;
    locale.setLocale(context.locale);
    render();
  });
  render();
  requestAnimationFrame(() => {
    void window.AmitiaHost.invokeAction("command", {
      action: "status",
      toolInput: {
        diagnostic: {
          slotId,
          definition: registry.getDefinition(slotId),
          registrations: registry.entriesOfSlot(slotId).map((registration) => ({
            pluginId: registration.pluginId,
            slotId: registration.slotId,
            key: registration.key,
          })),
          text: root.innerText,
          html: root.innerHTML,
          scrollHeight: document.documentElement.scrollHeight,
        },
      },
    }).catch((error) => {
      console.error("[original-error]", rawError(error));
    });
  });
  return {
    async dispose() {
      unsubscribe();
      unsubscribeContext();
      reactRoot.unmount();
      await clients.dispose();
      registry.dispose();
      locale.dispose();
      releaseIconSprite();
      root.replaceChildren();
    },
  };
}
