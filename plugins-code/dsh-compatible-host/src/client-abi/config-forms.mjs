function text(value) {
  return value == null ? "" : String(value).trim();
}

const volatileConfigs = new Map();

function cloneConfigValue(value) {
  if (value === undefined) return {};
  try {
    return structuredClone(value);
  } catch {
    return JSON.parse(JSON.stringify(value));
  }
}

function storageKey(moduleId, namespace) {
  return `amitia:dsh:config:${encodeURIComponent(moduleId)}:${encodeURIComponent(namespace)}`;
}

function readConfig(moduleId, namespace) {
  const key = storageKey(moduleId, namespace);
  try {
    const raw = globalThis.localStorage?.getItem(key);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === "object") {
        return {
          value: cloneConfigValue(parsed.value),
          revision: Number.isInteger(parsed.revision) ? parsed.revision : 0,
        };
      }
    }
  } catch (error) {
    console.error("[original-error]", error);
  }
  const cached = volatileConfigs.get(key);
  return cached ? cloneConfigValue(cached) : { value: {}, revision: 0 };
}

function writeConfig(moduleId, namespace, record) {
  const key = storageKey(moduleId, namespace);
  const next = cloneConfigValue(record);
  volatileConfigs.set(key, next);
  try {
    globalThis.localStorage?.setItem(key, JSON.stringify(next));
  } catch (error) {
    console.error("[original-error]", error);
  }
}

function applyOperations(input, operations) {
  let next = cloneConfigValue(input);
  for (const operation of [...operations]) {
    const segments = Array.isArray(operation?.path)
      ? operation.path.map((item) => String(item))
      : [];
    if (segments.length === 0) throw new Error("DSH config operation path is required");
    if (operation.op !== "set" && operation.op !== "unset") {
      throw new Error(`unsupported DSH config operation ${String(operation.op)}`);
    }
    let parent = next;
    for (let index = 0; index < segments.length - 1; index += 1) {
      const segment = segments[index];
      if (!parent[segment] || typeof parent[segment] !== "object" || Array.isArray(parent[segment])) {
        parent[segment] = {};
      }
      parent = parent[segment];
    }
    const leaf = segments.at(-1);
    if (operation.op === "set") parent[leaf] = cloneConfigValue(operation.value);
    else delete parent[leaf];
  }
  return next;
}

function withDefaults(namespace, value) {
  const source = value && typeof value === "object" && !Array.isArray(value) ? value : {};
  return new Proxy(source, {
    get(target, property) {
      if (Reflect.has(target, property)) return Reflect.get(target, property);
      const key = String(property);
      if (namespace === "developerTools" && key === "enabled") return false;
      if (key === "preference") return "system";
      if (key === "fontSize" || key.endsWith("FontSize")) return 14;
      if (key.endsWith("FontFamily")) return "";
      return undefined;
    },
  });
}

export function createDshConfigForms(moduleId) {
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
    const stored = readConfig(moduleId, key);
    let snapshot = {
      status: "ready",
      value: withDefaults(key, cloneConfigValue(stored.value)),
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
        const value = applyOperations(snapshot.value, operations);
        snapshot = {
          ...snapshot,
          value: withDefaults(key, value),
          base: cloneConfigValue(value),
          user: cloneConfigValue(value),
          revision: snapshot.revision + 1,
        };
        writeConfig(moduleId, key, {
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

  function namespaceForm(namespace) {
    const controller = form(namespace);
    return new Proxy({}, {
      get(_target, property) {
        if (typeof property !== "string") return undefined;
        return {
          getSnapshot() {
            return controller.getSnapshot().value[property];
          },
          subscribe(listener) {
            return controller.subscribe(listener);
          },
        };
      },
    });
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

  const base = {
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
  const configForms = new Proxy(base, {
    get(target, property, receiver) {
      if (Reflect.has(target, property)) return Reflect.get(target, property, receiver);
      return typeof property === "string" ? namespaceForm(property) : undefined;
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
  return { configForms, settings, onRemote, dispose: base.dispose };
}
