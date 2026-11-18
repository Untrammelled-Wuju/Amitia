function text(value) {
  return value == null ? "" : String(value).trim();
}

function interpolate(template, params) {
  return String(template).replace(/\{(\w+)\}/g, (_match, key) => {
    const value = params?.[key];
    return value === undefined || value === null ? "" : String(value);
  });
}

export function createLocaleService() {
  const dictionaries = new Map();
  const namespaces = new Map();
  const resolve = (namespace, key, params) => {
    const dict = dictionaries.get(namespace) ?? {};
    const raw = dict[key];
    if (typeof raw !== "string") return String(key);
    return params ? interpolate(raw, params) : raw;
  };
  const service = {
    current: "en",
    locales: ["en"],
    fallback: "en",
    register(namespace, dictionary) {
      const name = text(namespace);
      if (!name) throw new Error("locale.register requires a namespace");
      dictionaries.set(name, { ...(dictionaries.get(name) ?? {}), ...(dictionary ?? {}) });
      return () => {};
    },
    has(namespace) {
      return dictionaries.has(text(namespace));
    },
    get(namespace, key, params) {
      return resolve(text(namespace), text(key), params);
    },
    t(namespace, key, params) {
      if (key === undefined) return resolve("", text(namespace), key);
      return resolve(text(namespace), text(key), params);
    },
    namespace(namespace) {
      const name = text(namespace);
      const existing = namespaces.get(name);
      if (existing) return existing;
      const created = {
        name,
        get: (key, params) => resolve(name, text(key), params),
        t: (key, params) => resolve(name, text(key), params),
        has: (key) => typeof (dictionaries.get(name) ?? {})[text(key)] === "string",
        register: (dictionary) => service.register(name, dictionary),
      };
      namespaces.set(name, created);
      return created;
    },
    subscribe() {
      return () => {};
    },
  };
  return service;
}

export function createCordisContext(base = {}) {
  const source = base && typeof base === "object" ? base : {};
  const registered = new Set();
  const listeners = new Map();
  const services = new Map();
  if (!source.locale) services.set("locale", createLocaleService());

  const detach = (off) => {
    if (!registered.delete(off)) return;
    off();
  };

  const ctx = {
    ...source,
    effect(callback) {
      if (typeof callback !== "function") throw new Error("ctx.effect requires a callback");
      const dispose = callback();
      const off = typeof dispose === "function" ? dispose : () => {};
      registered.add(off);
      return () => detach(off);
    },
    on(name, listener, prepend = false) {
      const key = text(name);
      if (!key) throw new Error("ctx.on requires an event name");
      if (typeof listener !== "function") throw new Error("ctx.on requires a listener");
      const entry = { listener, active: true };
      const list = listeners.get(key) ?? [];
      if (prepend) list.unshift(entry);
      else list.push(entry);
      listeners.set(key, list);
      const off = () => {
        if (!entry.active) return;
        entry.active = false;
        const current = listeners.get(key) ?? [];
        const index = current.indexOf(entry);
        if (index >= 0) current.splice(index, 1);
      };
      registered.add(off);
      return off;
    },
    once(name, listener) {
      const off = ctx.on(name, (...args) => {
        off();
        listener(...args);
      });
      return off;
    },
    off(name, listener) {
      const key = text(name);
      const list = listeners.get(key) ?? [];
      for (const entry of [...list]) {
        if (entry.listener !== listener) continue;
        const index = list.indexOf(entry);
        if (index >= 0) list.splice(index, 1);
        entry.active = false;
      }
    },
    emit(name, ...args) {
      const key = text(name);
      if (!key) return;
      for (const entry of [...(listeners.get(key) ?? [])]) {
        if (entry.active) entry.listener(...args);
      }
    },
    get(name) {
      const key = text(name);
      if (!key) return undefined;
      return services.has(key) ? services.get(key) : source[key];
    },
    set(name, value) {
      const key = text(name);
      if (!key) throw new Error("ctx.set requires a service name");
      services.set(key, value);
      return value;
    },
    provide(name, value) {
      return ctx.set(name, value);
    },
    inject(dependencies, callback) {
      if (typeof callback !== "function") throw new Error("ctx.inject requires a callback");
      return ctx.effect(() => callback(ctx));
    },
    dispose() {
      for (const off of [...registered]) detach(off);
      listeners.clear();
      services.clear();
    },
  };
  ctx.scope = { name: "dsh-compatible-host", parent: undefined, dispose() {} };
  return ctx;
}
