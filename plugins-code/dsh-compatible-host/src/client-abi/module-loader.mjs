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

function consumeClientEffect(effect, cleanup, pending) {
  if (effect == null) return;
  if (typeof effect?.then === "function") {
    pending.push(Promise.resolve(effect).then((value) => consumeClientEffect(value, cleanup, pending)));
    return;
  }
  if (typeof effect === "function") {
    cleanup.push(effect);
    return;
  }
  if (typeof effect?.[Symbol.asyncIterator] === "function") {
    pending.push((async () => {
      for await (const item of effect) consumeClientEffect(item, cleanup, pending);
    })());
    return;
  }
  if (typeof effect?.[Symbol.iterator] === "function" && typeof effect !== "string") {
    for (const item of effect) consumeClientEffect(item, cleanup, pending);
    return;
  }
  throw new Error(`DSH client effect must return a disposer or iterable: ${String(effect)}`);
}

export class DshClientModuleLoader {
  records = new Map();

  exportsByModules = new WeakMap();

  materializing = new Set();

  load(record) {
    const id = text(record?.id);
    if (!id) throw new Error("DSH client module id is required");
    if (typeof record?.factory !== "function") throw new Error(`DSH client module ${id} factory is required`);
    this.records.set(id, record);
    return () => {
      if (this.records.get(id) === record) this.records.delete(id);
    };
  }

  has(id) {
    return this.records.has(text(id));
  }

  list() {
    return Array.from(this.records.keys()).sort();
  }

  record(id) {
    return this.records.get(text(id));
  }

  remove(id, record) {
    const key = text(id);
    if (this.records.get(key) === record) this.records.delete(key);
  }

  resolveModuleId(specifier) {
    const name = text(specifier);
    if (this.records.has(name)) return name;
    let matched = "";
    for (const id of this.records.keys()) {
      if (name.startsWith(`${id}/`) && id.length > matched.length) matched = id;
    }
    return matched;
  }

  requireInto(fromId, specifier, modules) {
    const name = text(specifier);
    if (Object.prototype.hasOwnProperty.call(modules, name)) return modules[name];
    const target = this.resolveModuleId(name);
    if (!target) throw new Error(`DSH client module ${fromId} requested unavailable dependency ${name}`);
    const exportsValue = this.materialize(target, modules);
    if (name === target) return exportsValue;
    const subpath = name.slice(target.length + 1);
    if (subpath !== "client") {
      throw new Error(`DSH client module ${fromId} requested unsupported subpath ${name}: only the client entry of ${target} is available`);
    }
    return exportsValue;
  }

  exportsFor(modules) {
    let cache = this.exportsByModules.get(modules);
    if (!cache) {
      cache = new Map();
      this.exportsByModules.set(modules, cache);
    }
    return cache;
  }

  materialize(id, modules) {
    const key = text(id);
    const cache = this.exportsFor(modules);
    if (cache.has(key)) return cache.get(key);
    const record = this.records.get(key);
    if (!record) throw new Error(`DSH client module ${key} is not registered`);
    if (this.materializing.has(key)) throw new Error(`DSH client module ${key} has a circular dependency`);
    this.materializing.add(key);
    try {
      const requireModule = (specifier) => this.requireInto(key, specifier, modules);
      requireModule.asset = (path) => text(path);
      const value = record.factory(requireModule);
      cache.set(key, value);
      return value;
    } finally {
      this.materializing.delete(key);
    }
  }

  instantiate(id, modules = {}) {
    const key = text(id);
    if (!this.records.has(key)) throw new Error(`DSH client module ${key} is not registered`);
    const plugin = this.materialize(key, modules);
    if (!plugin || typeof plugin.apply !== "function") {
      throw new Error(`DSH client module ${key} did not return apply()`);
    }
    return plugin;
  }
}

export function installDshModuleLoader(target = globalThis) {
  if (target.__ModuleLoader__ instanceof DshClientModuleLoader) return target.__ModuleLoader__;
  const loader = new DshClientModuleLoader();
  Object.defineProperty(target, "__ModuleLoader__", {
    configurable: true,
    enumerable: false,
    writable: false,
    value: loader,
  });
  return loader;
}

export class DshClientEventBus {
  listeners = new Map();

  on(type, listener) {
    const name = text(type);
    if (!name) throw new Error("DSH client event type is required");
    if (typeof listener !== "function") throw new Error(`DSH client event ${name} listener must be a function`);
    const group = this.listeners.get(name) ?? new Set();
    group.add(listener);
    this.listeners.set(name, group);
    return () => {
      group.delete(listener);
      if (group.size === 0 && this.listeners.get(name) === group) this.listeners.delete(name);
    };
  }

  async emit(type, payload) {
    for (const listener of Array.from(this.listeners.get(text(type)) ?? [])) {
      await listener(payload);
    }
  }

  async waterfall(type, args, fallback) {
    const name = text(type);
    if (!name) throw new Error("DSH client event type is required");
    if (typeof fallback !== "function") throw new Error(`DSH client event ${name} waterfall fallback must be a function`);
    const listeners = Array.from(this.listeners.get(name) ?? []);
    const run = async (index, values) => {
      if (index >= listeners.length) return await fallback(...values);
      return await listeners[index](...values, (...next) => run(index + 1, next));
    };
    return await run(0, [...(Array.isArray(args) ? args : [])]);
  }
}

export function createDshClientContext(options) {
  const disposers = [];
  const eventBus = options.eventBus ?? new DshClientEventBus();
  const services = options.services ?? {};
  const context = {
    pluginId: options.pluginId,
    slots: options.slots,
    locale: options.locale,
    logger: options.logger ?? {
      debug() {},
      info() {},
      warn() {},
      error() {},
    },
    reflect: {
      provide(name, value) {
        return context.provide(name, value);
      },
      get(name) {
        return context.get(name);
      },
    },
    provide(name, value) {
      const key = text(name);
      if (!key) throw new Error("DSH client service name is required");
      const hadPrevious = Object.prototype.hasOwnProperty.call(context, key);
      const previous = context[key];
      context[key] = value;
      let active = true;
      const dispose = () => {
        if (!active) return;
        active = false;
        if (hadPrevious) context[key] = previous;
        else delete context[key];
      };
      disposers.push(dispose);
      return dispose;
    },
    get(name) {
      return context[text(name)];
    },
    set(name, value) {
      const key = text(name);
      if (!key) throw new Error("DSH client service name is required");
      context[key] = value;
    },
    inject(names, callback) {
      const requested = Array.isArray(names) ? names : [names];
      if (requested.some((name) => context[text(name)] === undefined)) return () => {};
      if (typeof callback !== "function") throw new Error("DSH client ctx.inject requires a callback");
      const result = callback(context);
      if (typeof result === "function") disposers.push(result);
      return result;
    },
    effect(factory) {
      if (typeof factory !== "function") throw new Error("DSH client ctx.effect requires a function");
      const result = factory();
      const cleanup = [];
      const pending = [];
      consumeClientEffect(result, cleanup, pending);
      if (cleanup.length || pending.length) {
        disposers.push(async () => {
          await Promise.all(pending);
          for (let index = cleanup.length - 1; index >= 0; index -= 1) await cleanup[index]?.();
        });
      }
      return result;
    },
    plugin(definition, config) {
      if (typeof definition === "function") {
        if (Function.prototype.toString.call(definition).trimStart().startsWith("class")) {
          const instance = new definition(context, config);
          if (typeof instance?.apply === "function") {
            const result = instance.apply(context, config);
            const cleanup = [];
            const pending = [];
            consumeClientEffect(result, cleanup, pending);
            if (cleanup.length || pending.length) {
              disposers.push(async () => {
                await Promise.all(pending);
                for (let index = cleanup.length - 1; index >= 0; index -= 1) await cleanup[index]?.();
              });
            }
          }
          return instance;
        }
        const result = definition(context, config);
        const cleanup = [];
        const pending = [];
        consumeClientEffect(result, cleanup, pending);
        if (cleanup.length || pending.length) {
          disposers.push(async () => {
            await Promise.all(pending);
            for (let index = cleanup.length - 1; index >= 0; index -= 1) await cleanup[index]?.();
          });
        }
        return result;
      }
      if (!definition || typeof definition !== "object" || typeof definition.apply !== "function") {
        throw new Error("DSH client plugin definition must be a class, function, or apply object");
      }
      const result = definition.apply(context, config);
      const cleanup = [];
      const pending = [];
      consumeClientEffect(result, cleanup, pending);
      if (cleanup.length || pending.length) {
        disposers.push(async () => {
          await Promise.all(pending);
          for (let index = cleanup.length - 1; index >= 0; index -= 1) await cleanup[index]?.();
        });
      }
      return result;
    },
    on(type, listener) {
      const dispose = eventBus.on(type, listener);
      disposers.push(dispose);
      return dispose;
    },
    emit(type, payload) {
      return eventBus.emit(type, payload);
    },
    waterfall(...input) {
      if (input.length < 2) throw new Error("DSH client ctx.waterfall requires an event name and a fallback");
      const name = text(input[0]);
      if (!name) throw new Error("DSH client ctx.waterfall event name is required");
      const fallback = input[input.length - 1];
      if (typeof fallback !== "function") throw new Error(`DSH client ctx.waterfall ${name} fallback must be a function`);
      return eventBus.waterfall(name, input.slice(1, -1), fallback);
    },
  };
  for (const [name, value] of Object.entries(services)) {
    if (context[name] === undefined) context[name] = value;
  }
  const timer = services.timer;
  if (timer) {
    for (const method of ["timeout", "interval", "throttle", "debounce"]) {
      if (context[method] === undefined) context[method] = (...args) => timer[method](...args);
    }
    if (context.sleep === undefined) context.sleep = (delay) => timer.timeout(delay);
  }
  return {
    context,
    eventBus,
    dispose: disposeList(disposers),
  };
}

export async function loadDshClientPlugin(options) {
  const loader = options.loader;
  if (!(loader instanceof DshClientModuleLoader)) throw new Error("DSH client loader is required");
  const plugin = loader.instantiate(options.id, options.modules);
  const inject = Array.isArray(plugin.inject) ? plugin.inject : [];
  const runtime = createDshClientContext({
    pluginId: options.id,
    slots: options.slots,
    locale: options.locale,
    services: options.services,
    logger: options.logger,
    eventBus: options.eventBus,
  });
  for (const service of inject) {
    const name = text(service);
    if (name && runtime.context[name] === undefined) {
      throw new Error(`DSH client plugin ${options.id} requires unavailable service ${name}`);
    }
  }
  await plugin.apply(runtime.context);
  return {
    plugin,
    inject,
    context: runtime.context,
    eventBus: runtime.eventBus,
    dispose: runtime.dispose,
  };
}
