import { mkdir } from "node:fs/promises";
import { basename, dirname, join, resolve } from "node:path";
import { TREE_DIRECTORY_NAME, resolvePluginDataRoot } from "./package-manager.mjs";

const providerDataDirectories = new Map([
  ["@deepseek-ai/dsh-storage-json", "storage"],
  ["@deepseek-ai/dsh-session-persistence-jsonl", "sessions"],
  ["@deepseek-ai/dsh-spill-local", "spill"],
]);

const seamConfigs = new Map([
  ["@deepseek-ai/dsh-storage-domain", { backend: "json" }],
  ["@deepseek-ai/dsh-session-projection-cache", { writeEveryEvents: 200, writeIntervalMs: 5000 }],
  ["@deepseek-ai/dsh-compaction-basic", {
    auto: true,
    summarizationProvider: "amitia",
    summarizationModel: "deepseek-chat",
  }],
  ["@deepseek-ai/dsh-tool-fs-search", { sampleOverCapGlobResults: false }],
]);

export function pluginDataRootFromPackageDir(packageDir) {
  let current = resolve(String(packageDir || ""));
  while (current && current !== dirname(current)) {
    if (basename(current) === TREE_DIRECTORY_NAME) return dirname(current);
    current = dirname(current);
  }
  return "";
}

export function providerDataDirectory(pluginId, pluginDataRoot) {
  const directoryName = providerDataDirectories.get(String(pluginId || "").trim());
  if (!directoryName) return "";
  return join(String(pluginDataRoot), "dsh-data", directoryName);
}

export async function providerConfig(pluginId, pluginDataRoot) {
  const id = String(pluginId || "").trim();
  const seamConfig = seamConfigs.get(id);
  if (seamConfig) return { ...seamConfig };
  const directoryName = providerDataDirectories.get(id);
  if (!directoryName) return {};
  const root = pluginDataRoot ? String(pluginDataRoot) : resolvePluginDataRoot();
  const directory = join(root, "dsh-data", directoryName);
  await mkdir(directory, { recursive: true });
  return { root: directory };
}
