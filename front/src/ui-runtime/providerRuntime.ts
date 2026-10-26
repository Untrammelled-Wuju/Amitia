import { defineAsyncComponent, defineComponent, h, markRaw, onUnmounted, type Component } from "vue";
import { getRuntimeConnection } from "@/runtime/runtime-adapter";
import type { UIProviderDefinition, UIProviderEntry, UIProviderResolveContext } from "./types";

const trustedModuleCache = new Map<string, Component>();
const trustedStyleRefs = new Map<string, { count: number; link: HTMLLinkElement }>();

export function selectProviderEntry(provider: UIProviderDefinition, platform: string): UIProviderEntry | null {
  if (["windows", "macos", "linux"].includes(platform)) {
    if (provider.entries[`electron_${platform}`]) return provider.entries[`electron_${platform}`];
    if (provider.entries.electron_desktop) return provider.entries.electron_desktop;
  }
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

let providerModuleBaseURL = "";

async function resolveProviderModuleBaseURL(): Promise<string> {
  if (providerModuleBaseURL) return providerModuleBaseURL;
  const connection = await getRuntimeConnection();
  providerModuleBaseURL = connection.apiBaseURL.replace(/\/+$/, "");
  return providerModuleBaseURL;
}

async function providerModuleURL(provider: UIProviderDefinition, path: string, integrity?: string, moduleToken?: string): Promise<string> {
  const clean = path.replace(/^\/+/, "");
  const query = new URLSearchParams();
  query.set("generation", String(provider.generation ?? 0));
  if (integrity?.trim()) query.set("integrity", integrity.trim());
  if (moduleToken?.trim()) query.set("accessToken", moduleToken.trim());
  const base = await resolveProviderModuleBaseURL();
  return `${base}/api/public/extensions/ui/provider-module/${encodeURIComponent(provider.providerId)}/${clean}?${query.toString()}`;
}

async function acquireTrustedStyle(provider: UIProviderDefinition, path: string, integrity?: string, moduleToken?: string): Promise<() => void> {
  const url = await providerModuleURL(provider, path, integrity, moduleToken);
  const existing = trustedStyleRefs.get(url);
  if (existing) {
    existing.count += 1;
    return () => releaseTrustedStyle(url);
  }
  const link = document.createElement("link");
  link.rel = "stylesheet";
  link.href = url;
  link.dataset.uiProvider = provider.providerId;
  document.head.append(link);
  trustedStyleRefs.set(url, { count: 1, link });
  return () => releaseTrustedStyle(url);
}

function releaseTrustedStyle(url: string): void {
  const entry = trustedStyleRefs.get(url);
  if (!entry) return;
  entry.count -= 1;
  if (entry.count > 0) return;
  entry.link.remove();
  trustedStyleRefs.delete(url);
}

export function trustedWebModuleExport(
  provider: UIProviderDefinition,
  entry: UIProviderEntry,
  exportName: string,
): Component | null {
  if (!canLoadTrustedWebModule(provider, entry) || !entry.path || !exportName.trim()) return null;
  const name = exportName.trim();
  const key = `${provider.providerId}:${provider.generation ?? 0}:${entry.path}:${entry.contentHash ?? ""}:export:${name}`;
  const cached = trustedModuleCache.get(key);
  if (cached) return cached;
  const component = markRaw(defineAsyncComponent(async () => {
    const mod = await import(/* @vite-ignore */ await providerModuleURL(provider, entry.path!, entry.contentHash, entry.moduleToken));
    const exported = mod[name] ?? mod.default?.[name] ?? mod.components?.[name] ?? mod.icons?.[name];
    if (!exported) throw new Error(`UI provider ${provider.providerId} module does not export ${name}`);
    return exported;
  }));
  trustedModuleCache.set(key, component);
  return component;
}

export function trustedWebModule(provider: UIProviderDefinition, entry: UIProviderEntry): Component | null {
  if (!canLoadTrustedWebModule(provider, entry) || !entry.path) return null;
  const key = `${provider.providerId}:${provider.generation ?? 0}:${entry.path}:${entry.contentHash ?? ""}:${entry.stylePath ?? ""}:${entry.exportName ?? "default"}`;
  const cached = trustedModuleCache.get(key);
  if (cached) return cached;
  const moduleComponent = defineAsyncComponent(async () => {
    const mod = await import(/* @vite-ignore */ await providerModuleURL(provider, entry.path!, entry.contentHash, entry.moduleToken));
    const exported = entry.exportName ? mod[entry.exportName] : (mod.default ?? mod);
    if (!exported) throw new Error(`UI provider ${provider.providerId} module does not export ${entry.exportName ?? "default"}`);
    return exported;
  });
  const component = markRaw(defineComponent({
    name: `TrustedUIProvider-${provider.providerId}`,
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      let release: (() => void) | null = null;
      let disposed = false;
      if (entry.stylePath) {
        void acquireTrustedStyle(provider, entry.stylePath, entry.styleHash, entry.moduleToken).then((releaseStyle) => {
          if (disposed) releaseStyle();
          else release = releaseStyle;
        });
      }
      onUnmounted(() => {
        disposed = true;
        release?.();
      });
      return () => h(
        "div",
        {
          class: "ui-provider-module",
          "data-ui-provider-id": provider.providerId,
          "data-ui-provider-generation": provider.generation ?? 0,
        },
        [h(moduleComponent, attrs, slots)],
      );
    },
  }));
  trustedModuleCache.set(key, component);
  return component;
}

export async function prewarmTrustedWebModule(provider: UIProviderDefinition, entry: UIProviderEntry): Promise<void> {
  if (!canLoadTrustedWebModule(provider, entry) || !entry.path) return;
  if (entry.stylePath && typeof document !== "undefined") {
    const url = await providerModuleURL(provider, entry.stylePath, entry.styleHash, entry.moduleToken);
    if (!document.querySelector(`link[data-ui-provider="${provider.providerId}"][href="${url}"]`)) {
      const link = document.createElement("link");
      link.rel = "preload";
      link.as = "style";
      link.href = url;
      link.dataset.uiProvider = provider.providerId;
      document.head.append(link);
    }
  }
  await import(/* @vite-ignore */ await providerModuleURL(provider, entry.path, entry.contentHash, entry.moduleToken));
}
