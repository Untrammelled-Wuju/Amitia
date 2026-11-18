import { registerHooks } from "node:module";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { inspect } from "node:util";
import { Context } from "./vendor/cordis.mjs";
import {
  DSH_HOST_ERROR_CODES,
  classifyDshPluginError,
  createDshHostError,
  tagDshHostError,
} from "./errors.mjs";

function text(value) {
  return value == null ? "" : String(value).trim();
}

function disposeList(disposers) {
  return async () => {
    for (let index = disposers.length - 1; index >= 0; index -= 1) {
      await disposers[index]?.();
    }
    disposers.length = 0;
  };
}

export class DshEventBus {
  listeners = new Map();

  on(type, listener) {
    const name = text(type);
    if (!name) throw new Error("DSH 事件类型不能为空");
    if (typeof listener !== "function") throw new Error(`DSH 事件 ${name} 的监听器必须是函数`);
    const group = this.listeners.get(name) ?? new Set();
    group.add(listener);
    this.listeners.set(name, group);
    return () => {
      group.delete(listener);
      if (group.size === 0 && this.listeners.get(name) === group) this.listeners.delete(name);
    };
  }

  async emit(type, ...args) {
    for (const listener of Array.from(this.listeners.get(text(type)) ?? [])) {
      await listener(...args);
    }
  }

  async waterfall(type, ...args) {
    const ordered = Array.from(this.listeners.get(text(type)) ?? []);
    const dispatch = async (index, ...current) => {
      const listener = ordered[index];
      if (!listener) return current[0];
      return await listener(...current, (...next) => dispatch(index + 1, ...(next.length ? next : current)));
    };
    return await dispatch(0, ...args);
  }

  async bail(type, ...args) {
    for (const listener of Array.from(this.listeners.get(text(type)) ?? [])) {
      const result = await listener(...args);
      if (result !== undefined) return result;
    }
    return undefined;
  }
}

function injectNames(inject) {
  if (Array.isArray(inject)) return inject.map(text).filter(Boolean);
  if (inject && typeof inject === "object") return Object.keys(inject).map(text).filter(Boolean);
  return [];
}

function assertActive(fiber, inject, context) {
  if (fiber?.state === 2) return fiber;
  const names = injectNames(inject);
  const missing = names.filter((name) => context.get(name) === undefined);
  const unrealized = missing.length ? missing : names;
  throw createDshHostError(
    DSH_HOST_ERROR_CODES.SERVICE_MISSING,
    `DSH 插件依赖的服务 ${unrealized.join(", ")} 不可用`,
  );
}

const vendorCordisUrl = new URL("./vendor/cordis.mjs", import.meta.url).href;
const vendorCosmokitUrl = new URL("./vendor/cosmokit.mjs", import.meta.url).href;

const compatModules = new Map([
  ["@deepseek-ai/cordis", vendorCordisUrl],
  ["@deepseek-ai/cosmokit", vendorCosmokitUrl],
  ["@deepseek-ai/dsh-tools", new URL("./compat/dsh-tools.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-fs", new URL("./compat/dsh-fs.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-llm", new URL("./compat/dsh-llm.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-subagent", new URL("./compat/dsh-subagent.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-schedule", new URL("./compat/dsh-schedule.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-timeout", new URL("./compat/dsh-timeout.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-subprocess", new URL("./compat/dsh-subprocess.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-attachment", new URL("./compat/dsh-attachment.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-scope", new URL("./compat/dsh-scope.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-session", new URL("./compat/dsh-session.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-system-prompt", new URL("./compat/dsh-system-prompt.mjs", import.meta.url).href],
  ["@deepseek-ai/dsh-user-questions", new URL("./compat/dsh-user-questions.mjs", import.meta.url).href],
]);
let importHooksInstalled = false;

function installDshImportHooks() {
  if (importHooksInstalled) return;
  registerHooks({
    resolve(specifier, context, nextResolve) {
      const url = compatModules.get(specifier);
      if (!url) return nextResolve(specifier, context);
      try {
        return nextResolve(specifier, context);
      } catch (error) {
        if (error?.code !== "ERR_MODULE_NOT_FOUND") throw error;
        return { url, shortCircuit: true };
      }
    },
  });
  importHooksInstalled = true;
}

installDshImportHooks();

const reservedServiceNames = new Map([
  ["events", "amitiaEvents"],
  ["runtime", "amitiaRuntime"],
  ["sessions", "amitiaSessions"],
  ["agents", "amitiaAgents"],
  ["sessionProjections", "amitiaSessionProjections"],
  ["llm", "amitiaLlm"],
  ["tools", "amitiaTools"],
  ["systemPrompt", "amitiaSystemPrompt"],
  ["approval", "amitiaApproval"],
  ["commands", "amitiaCommands"],
]);

function registerHostServices(context, services) {
  for (const [name, value] of Object.entries(services)) {
    if (value === undefined) continue;
    const target = reservedServiceNames.get(name) ?? name;
    try {
      context.provide(target, value);
    } catch (error) {
      tagDshHostError(error, DSH_HOST_ERROR_CODES.SERVICE_CONFLICT);
      throw error;
    }
  }
}

export function dshCompatSpecifiers() {
  return Array.from(compatModules.keys());
}

function normalizeLogger(logger) {
  return {
    debug: typeof logger?.debug === "function" ? logger.debug.bind(logger) : () => {},
    info: typeof logger?.info === "function" ? logger.info.bind(logger) : () => {},
    warn: typeof logger?.warn === "function" ? logger.warn.bind(logger) : () => {},
    error: typeof logger?.error === "function" ? logger.error.bind(logger) : () => {},
  };
}

function createToolsFacade(options) {
  const { pluginId, tools } = options;
  return {
    register(definition) {
      if (!definition || typeof definition !== "object") throw new Error("DSH 工具定义必须是对象");
      const name = text(definition.name);
      if (!name) throw new Error("DSH 工具缺少 name");
      if (typeof definition.execute !== "function") throw new Error(`DSH 工具 ${name} 缺少 execute`);
      const entry = { ...definition, name, pluginId };
      tools.push(entry);
      return () => {
        const index = tools.indexOf(entry);
        if (index >= 0) tools.splice(index, 1);
      };
    },
    get(name, scope) {
      return tools.find((tool) => tool.name === text(name) && (scope === undefined || tool.scope === scope));
    },
    list() {
      return tools.slice();
    },
  };
}

function createContextEventBus(context) {
  return {
    on(type, listener) {
      return context.on(type, listener);
    },
    emit(type, ...args) {
      return context.emit(type, ...args);
    },
    waterfall(type, ...args) {
      return context.waterfall(type, ...args);
    },
    bail(type, ...args) {
      return context.bail(type, ...args);
    },
  };
}

function isDeclarationOnlyModule(loaded) {
  if (typeof loaded?.apply === "function") return false;
  if (text(loaded?.name)) return false;
  const candidate = loaded?.default;
  if (typeof candidate !== "function") return false;
  const prototype = candidate.prototype;
  if (!prototype) return true;
  for (const key of Object.getOwnPropertyNames(prototype)) {
    if (key === "constructor") continue;
    const descriptor = Object.getOwnPropertyDescriptor(prototype, key);
    if (!descriptor) continue;
    if (typeof descriptor.value === "function") return false;
    if (typeof descriptor.get === "function") return false;
  }
  return true;
}

export function normalizeHostPlugin(loaded, pluginId) {
  const candidate = loaded?.default && (typeof loaded.default === "function" || typeof loaded.default?.apply === "function")
    ? loaded.default
    : loaded;
  if (typeof candidate === "function") {
    const Config = candidate.Config ?? candidate.schema;
    if (Config !== undefined && candidate.Config === undefined) candidate.Config = Config;
    return candidate;
  }
  if (candidate && typeof candidate.apply === "function" && candidate[Symbol.toStringTag] === "Module") {
    return {
      name: text(candidate.name) || text(pluginId),
      inject: candidate.inject,
      apply: (...args) => candidate.apply.call(candidate, ...args),
      ...(candidate.Config !== undefined ? { Config: candidate.Config } : {}),
    };
  }
  return candidate;
}

export function createDshHostContext(options = {}) {
  const context = new Context();
  const eventBus = options.eventBus ?? createContextEventBus(context);
  const logger = normalizeLogger(options.logger);
  registerHostServices(context, options.services ?? {});
  context.logger.exporter({
    export(message) {
      const args = Array.isArray(message?.args) ? message.args : [message];
      logger.info(...args);
    },
  });
  return {
    context,
    eventBus,
    services: options.services ?? {},
    dispose: async () => {
      await context.fiber?.dispose?.();
    },
  };
}

export async function loadDshHostPlugin(options) {
  installDshImportHooks();
  const packageDir = text(options.packageDir);
  const entry = text(options.entry);
  if (!packageDir) throw new Error("DSH 插件缺少 packageDir");
  if (!entry) throw new Error("DSH 插件缺少 host entry");
  const entryPath = join(packageDir, entry);
  const loaded = await import(pathToFileURL(entryPath).href);
  if (isDeclarationOnlyModule(loaded)) {
    return {
      plugin: null,
      declarationOnly: true,
      inject: [],
      context: options.context ?? null,
      eventBus: options.eventBus,
      tools: [],
      dispose: async () => {},
    };
  }
  const exported = normalizeHostPlugin(loaded, options.pluginId);
  const inject = Array.isArray(exported?.inject)
    ? exported.inject
    : Array.isArray(exported?.constructor?.inject)
      ? exported.constructor.inject
      : Array.isArray(loaded.inject)
        ? loaded.inject
        : [];
  const owned = options.context ? null : createDshHostContext({
    pluginId: options.pluginId,
    services: options.services,
    logger: options.logger,
    eventBus: options.eventBus,
  });
  const root = options.context ?? owned.context;
  const tools = [];
  const isolated = root.isolate(`tools:${text(options.pluginId)}`);
  const entryContext = isolated.extend({
    pluginId: text(options.pluginId),
    scope: { extensionId: text(options.pluginId) },
    amitiaTools: createToolsFacade({ pluginId: text(options.pluginId), tools }),
  });
  const fiber = entryContext.plugin(exported, options.config ?? {});
  try {
    await fiber.await();
  } catch (error) {
    throw classifyDshPluginError(error);
  }
  const runtime = {
    plugin: fiber,
    inject,
    context: entryContext,
    root,
    eventBus: owned?.eventBus ?? options.eventBus ?? createContextEventBus(root),
    tools,
    dispose: async () => {
      const disposers = [];
      if (typeof fiber?.dispose === "function") disposers.push(() => fiber.dispose());
      if (owned) disposers.push(() => owned.dispose());
      disposers.push(() => {
        tools.length = 0;
      });
      await disposeList(disposers)();
    },
  };
  if (options.deferInjectCheck !== true) assertActive(fiber, inject, entryContext);
  return runtime;
}

export async function assertDshHostPluginActive(runtime) {
  if (runtime?.declarationOnly === true) return runtime;
  await runtime.plugin.await();
  assertActive(runtime.plugin, runtime.inject, runtime.context);
  return runtime;
}

export async function invokeDshTool(pluginRuntime, name, input, execution = {}) {
  const toolName = text(name);
  if (!toolName) throw new Error("DSH 工具名不能为空");
  const tool = pluginRuntime.tools.find((item) => item.name === toolName);
  if (!tool) throw new Error(`DSH 工具 ${toolName} 不存在或未启用`);
  const context = {
    signal: execution.signal ?? new AbortController().signal,
    agent: execution.agent,
    callId: execution.callId,
  };
  const session = context.agent?.session;
  if (session) {
    await session.hydrated;
    if (session.persistenceError) throw session.persistenceError;
  }
  context.signal.throwIfAborted();
  try {
    const result = await tool.execute(input ?? {}, context);
    const callId = text(execution.callId);
    if (callId && typeof session?.append === "function") {
      let liveResult;
      try {
        liveResult = result == null ? null : JSON.parse(JSON.stringify(result));
      } catch (cause) {
        throw new Error(`DSH tool ${toolName} returned a non-JSON result`, { cause });
      }
      const events = Array.isArray(session.events) ? session.events : [];
      const lastEvent = events.at(-1);
      session.append("tool/result", {
        turn: Number(lastEvent?.data?.turn) || 0,
        step: Number(lastEvent?.data?.step) || 0,
        message: {
          id: `tool-result:${callId}`,
          role: "tool",
          source: { kind: "tool" },
          content: [{ type: "text", text: typeof result === "string" ? result : JSON.stringify(result ?? null) }],
          toolCallId: callId,
        },
        liveResult,
      });
    }
    if (typeof session?.whenPersisted === "function") {
      await session.whenPersisted();
    }
    return result;
  } catch (error) {
    if (typeof session?.whenPersisted === "function") {
      try {
        await session.whenPersisted();
      } catch (persistenceError) {
        if (persistenceError !== error) {
          const originals = [error, persistenceError]
            .map(original => inspect(original, { depth: null, maxArrayLength: null, maxStringLength: null }))
            .join("\n");
          throw new AggregateError([error, persistenceError], `DSH tool execution and session persistence failed\n${originals}`, { cause: error });
        }
      }
    }
    throw error;
  }
}
