import { TARGET_DSH_VERSION } from "./abi.mjs";

export const STATE_KEY = "dsh-compatible-host-state";

function text(value) {
  return value == null ? "" : String(value).trim();
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

export function defaultState() {
  return {
    version: 1,
    targetDshVersion: TARGET_DSH_VERSION,
    packages: [],
    updatedAt: new Date(0).toISOString(),
  };
}

export function normalizePackage(value) {
  const source = value && typeof value === "object" ? value : {};
  const pluginId = text(source.pluginId);
  if (!pluginId) throw new Error("DSH 插件缺少 pluginId");
  const version = text(source.version);
  if (!version) throw new Error(`DSH 插件 ${pluginId} 缺少 version`);
  const entry = text(source.entry);
  if (!entry) throw new Error(`DSH 插件 ${pluginId} 缺少 entry`);
  return {
    pluginId,
    version,
    entry,
    entries: Array.isArray(source.entries)
      ? source.entries.map((item) => ({
          id: text(item?.id) || "root",
          entry: text(item?.entry),
        })).filter((item) => item.entry)
      : [{ id: "root", entry }],
    name: text(source.name) || pluginId,
    source: text(source.source),
    clientEntry: text(source.clientEntry),
    clientEntries: Array.isArray(source.clientEntries)
      ? source.clientEntries.map((item) => ({
          id: text(item?.id) || "root",
          entry: text(item?.entry),
        })).filter((item) => item.entry)
      : text(source.clientEntry)
        ? [{ id: "root", entry: text(source.clientEntry) }]
        : [],
    installRoot: text(source.installRoot),
    packageDir: text(source.packageDir),
    dsh: source.dsh && typeof source.dsh === "object" ? clone(source.dsh) : {},
    enabled: source.enabled === true,
    installedAt: text(source.installedAt) || new Date().toISOString(),
    mappedContributions: Array.isArray(source.mappedContributions)
      ? source.mappedContributions.map((item) => clone(item))
      : [],
  };
}

export function normalizeState(value) {
  const source = value && typeof value === "object" ? value : {};
  const packages = Array.isArray(source.packages)
    ? source.packages.map((item) => {
        try {
          return normalizePackage(item);
        } catch (error) {
          throw new Error(`DSH 容器状态包含无效插件: ${error instanceof Error ? error.message : String(error)}`, { cause: error });
        }
      })
    : [];
  const seen = new Set();
  for (const item of packages) {
    if (seen.has(item.pluginId)) throw new Error(`DSH 插件 ${item.pluginId} 重复登记`);
    seen.add(item.pluginId);
  }
  return {
    version: 1,
    targetDshVersion: text(source.targetDshVersion) || TARGET_DSH_VERSION,
    packages,
    updatedAt: text(source.updatedAt) || new Date(0).toISOString(),
  };
}

export function upsertPackage(state, value) {
  const next = clone(state);
  const item = normalizePackage(value);
  const index = next.packages.findIndex((entry) => entry.pluginId === item.pluginId);
  if (index >= 0) {
    item.installedAt = next.packages[index].installedAt;
    next.packages[index] = item;
  } else {
    next.packages.push(item);
  }
  return next;
}

export function setPackageEnabled(state, pluginId, enabled) {
  const id = text(pluginId);
  if (!id) throw new Error("pluginId 不能为空");
  const next = clone(state);
  const item = next.packages.find((entry) => entry.pluginId === id);
  if (!item) throw new Error(`DSH 插件 ${id} 未登记`);
  item.enabled = Boolean(enabled);
  return next;
}

export function removePackage(state, pluginId) {
  const id = text(pluginId);
  if (!id) throw new Error("pluginId 不能为空");
  const next = clone(state);
  const before = next.packages.length;
  next.packages = next.packages.filter((entry) => entry.pluginId !== id);
  if (next.packages.length === before) throw new Error(`DSH 插件 ${id} 未登记`);
  return next;
}
