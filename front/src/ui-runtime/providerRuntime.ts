import { defineAsyncComponent, markRaw, type Component } from "vue";
import type { UIProviderDefinition, UIProviderEntry } from "./types";

const trustedModuleCache = new Map<string, Component>();

export function selectProviderEntry(provider: UIProviderDefinition, platform: string): UIProviderEntry | null {
  if (provider.entries[platform]) return provider.entries[platform];
  if (["android", "ios"].includes(platform) && provider.entries.mobile) return provider.entries.mobile;
  if (["windows", "macos", "linux"].includes(platform) && provider.entries.desktop) return provider.entries.desktop;
  if (provider.entries.web && platform === "web") return provider.entries.web;
  return provider.entries["*"] ?? null;
}

export function canLoadTrustedWebModule(provider: UIProviderDefinition, entry: UIProviderEntry): boolean {
  if (entry.type !== "web_module" || !entry.path) return false;
  return ["system", "official", "trusted", "user_trusted", "verified"].includes((provider.trustLevel ?? "").toLowerCase());
}

function providerModuleURL(provider: UIProviderDefinition, path: string): string {
  const clean = path.replace(/^\/+/, "");
  return `/api/extensions/ui/provider-module/${encodeURIComponent(provider.providerId)}/${clean}`;
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
