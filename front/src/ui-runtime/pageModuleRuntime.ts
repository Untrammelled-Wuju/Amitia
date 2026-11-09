import { defineAsyncComponent, markRaw, type Component } from "vue";
import { getRuntimeConnection } from "@/runtime/runtime-adapter";

const pageModuleCache = new Map<string, Component>();
let pageModuleBaseURL = "";

async function resolvePageModuleBaseURL(): Promise<string> {
  if (pageModuleBaseURL) return pageModuleBaseURL;
  const connection = await getRuntimeConnection();
  pageModuleBaseURL = connection.apiBaseURL.replace(/\/+$/, "");
  return pageModuleBaseURL;
}

export interface PageModuleDescriptor {
  extensionId: string;
  pageId: string;
  generation: number;
  entryPath: string;
  entryHash?: string;
  moduleToken?: string;
}

export async function pageModuleURL(descriptor: PageModuleDescriptor): Promise<string> {
  const extensionId = descriptor.extensionId.trim();
  const pageId = descriptor.pageId.trim();
  const entryPath = descriptor.entryPath.replace(/^\/+/, "").trim();
  const moduleToken = descriptor.moduleToken?.trim() || "";
  if (!extensionId || !pageId || !entryPath || !moduleToken) {
    throw new Error("页面模块缺少扩展、页面、入口或访问令牌");
  }
  const query = new URLSearchParams();
  query.set("generation", String(descriptor.generation));
  if (descriptor.entryHash?.trim()) query.set("integrity", descriptor.entryHash.trim());
  query.set("accessToken", moduleToken);
  const base = await resolvePageModuleBaseURL();
  return `${base}/api/public/extensions/ui/page-module/${encodeURIComponent(extensionId)}/${encodeURIComponent(pageId)}/${entryPath}?${query.toString()}`;
}

export function pageModuleComponent(descriptor: PageModuleDescriptor): Component {
  const key = [
    descriptor.extensionId,
    descriptor.pageId,
    descriptor.generation,
    descriptor.entryPath,
    descriptor.entryHash ?? "",
  ].join(":");
  const cached = pageModuleCache.get(key);
  if (cached) return cached;
  const component = markRaw(defineAsyncComponent(async () => {
    const url = await pageModuleURL(descriptor);
    const loaded = await import(/* @vite-ignore */ url);
    const exported = loaded.default ?? loaded;
    if (!exported) throw new Error(`页面 ${descriptor.extensionId}/${descriptor.pageId} 未导出内容`);
    return exported;
  }));
  pageModuleCache.set(key, component);
  return component;
}
