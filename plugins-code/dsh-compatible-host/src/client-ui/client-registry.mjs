const registryKey = "__amitiaDshClientRegistry";

function registry() {
  if (!(globalThis[registryKey] instanceof Map)) globalThis[registryKey] = new Map();
  return globalThis[registryKey];
}

function requirePluginId(pluginId) {
  const id = String(pluginId ?? "").trim();
  if (!id) throw new Error("DSH 客户端实例化需要 pluginId");
  return id;
}

export function acquireDshClient(pluginId, create) {
  const id = requirePluginId(pluginId);
  if (typeof create !== "function") throw new Error(`DSH 客户端 ${id} 缺少实例化函数`);
  const entries = registry();
  let record = entries.get(id);
  if (!record) {
    record = { id, refCount: 0, instance: null, pending: null, disposed: false };
    record.pending = Promise.resolve()
      .then(() => create())
      .then((instance) => {
        record.instance = instance;
        return instance;
      })
      .catch((error) => {
        if (entries.get(id) === record) entries.delete(id);
        throw error;
      });
    record.pending.catch(() => {});
    entries.set(id, record);
  }
  record.refCount += 1;
  let released = false;
  return {
    pluginId: id,
    async ready() {
      return await record.pending;
    },
    async release() {
      if (released) return;
      released = true;
      record.refCount -= 1;
      if (record.refCount > 0) return;
      if (entries.get(id) === record) entries.delete(id);
      if (record.disposed) return;
      record.disposed = true;
      let instance;
      try {
        instance = await record.pending;
      } catch {
        return;
      }
      await instance?.dispose?.();
    },
  };
}

export async function invalidateDshClient(pluginId) {
  const id = requirePluginId(pluginId);
  const entries = registry();
  const record = entries.get(id);
  if (!record) return;
  entries.delete(id);
  if (record.disposed) return;
  record.disposed = true;
  let instance;
  try {
    instance = await record.pending;
  } catch {
    return;
  }
  await instance?.dispose?.();
}

export function dshClientHandleCount() {
  let total = 0;
  for (const record of registry().values()) total += record.refCount;
  return total;
}
