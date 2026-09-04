import { defineAsyncComponent, markRaw, type Component } from "vue";
import type { UIProviderDefinition, UIProviderEntry, UIProviderResolveContext } from "./types";

const trustedModuleCache = new Map<string, Component>();

export function selectProviderEntry(provider: UIProviderDefinition, platform: string): UIProviderEntry | null {
  if (provider.entries[platform]) return provider.entries[platform];
  if (["android", "ios"].includes(platform) && provider.entries.mobile) return provider.entries.mobile;
  if (["windows", "macos", "linux"].includes(platform) && provider.entries.desktop) return provider.entries.desktop;
  if (provider.entries.web && platform === "web") return provider.entries.web;
  return provider.entries["*"] ?? null;
}


function hasFold(values: string[] | undefined, wanted: string): boolean {
  return (values ?? []).some((value) => value.trim().toLowerCase() === wanted.trim().toLowerCase());
}

function compareVersion(a: string, b: string): number {
  const parse = (raw: string): number[] => {
    const normalized = raw.trim().replace(/^v/i, "").split("-", 1)[0] ?? "";
    const parts = normalized.split(".");
    return Array.from({ length: 4 }, (_, index) => {
      const value = Number.parseInt(parts[index] ?? "0", 10);
      return Number.isFinite(value) ? value : 0;
    });
  };
  const left = parse(a);
  const right = parse(b);
  for (let i = 0; i < left.length; i += 1) {
    if (left[i] < right[i]) return -1;
    if (left[i] > right[i]) return 1;
  }
  return 0;
}

export function isProviderCompatible(provider: UIProviderDefinition, context: UIProviderResolveContext | undefined, platform: string): boolean {
  if (!provider.enabled || !selectProviderEntry(provider, platform)) return false;
  const placement = provider.placement ?? (provider.builtin ? "any" : "cloud");
  const ctx = context ?? { platform };
  if (placement === "device" && !ctx.localRuntime) {
    if (!ctx.deviceId || !ctx.deviceOnline) return false;
  }
  const req = provider.deviceRequirements;
  if (!req) return true;
  if (req.platforms?.length && !hasFold(req.platforms, platform)) return false;
  if (req.architectures?.length) {
    if (!ctx.architecture || !hasFold(req.architectures, ctx.architecture)) return false;
  }
  if (req.minAppVersion) {
    if (!ctx.appVersion || compareVersion(ctx.appVersion, req.minAppVersion) < 0) return false;
  }
  if (req.minRuntimeVersion) {
    if (!ctx.runtimeVersion || compareVersion(ctx.runtimeVersion, req.minRuntimeVersion) < 0) return false;
  }
  for (const feature of req.requiredFeatures ?? []) {
    if (!hasFold(ctx.deviceCapabilities, feature)) return false;
  }
  return true;
}

export function canLoadTrustedWebModule(provider: UIProviderDefinition, entry: UIProviderEntry): boolean {
  if (entry.type !== "web_module" || !entry.path) return false;
  return ["system", "official", "trusted", "user_trusted", "verified"].includes((provider.trustLevel ?? "").toLowerCase());
}

function providerModuleURL(provider: UIProviderDefinition, path: string): string {
  const clean = path.replace(/^\/+/, "");
  return `/api/extensions/ui/provider-module/${encodeURIComponent(provider.providerId)}/${clean}`;
}

export function trustedWebModuleExport(
  provider: UIProviderDefinition,
  entry: UIProviderEntry,
  exportName: string,
): Component | null {
  if (!canLoadTrustedWebModule(provider, entry) || !entry.path || !exportName.trim()) return null;
  const name = exportName.trim();
  const key = `${provider.providerId}:${provider.generation ?? 0}:${entry.path}:export:${name}`;
  const cached = trustedModuleCache.get(key);
  if (cached) return cached;
  const component = markRaw(defineAsyncComponent(async () => {
    const mod = await import(/* @vite-ignore */ providerModuleURL(provider, entry.path!));
    const exported = mod[name] ?? mod.default?.[name] ?? mod.components?.[name] ?? mod.icons?.[name];
    if (!exported) throw new Error(`UI provider ${provider.providerId} module does not export ${name}`);
    return exported;
  }));
  trustedModuleCache.set(key, component);
  return component;
}

export function trustedWebModule(provider: UIProviderDefinition, entry: UIProviderEntry): Component | null {
  if (!canLoadTrustedWebModule(provider, entry) || !entry.path) return null;
  const key = `${provider.providerId}:${provider.generation ?? 0}:${entry.path}:${entry.exportName ?? "default"}`;
  const cached = trustedModuleCache.get(key);
  if (cached) return cached;
  const component = markRaw(defineAsyncComponent(async () => {
    const mod = await import(/* @vite-ignore */ providerModuleURL(provider, entry.path!));
    const exported = entry.exportName ? mod[entry.exportName] : (mod.default ?? mod);
    if (!exported) throw new Error(`UI provider ${provider.providerId} module does not export ${entry.exportName ?? "default"}`);
    return exported;
  }));
  trustedModuleCache.set(key, component);
  return component;
}
