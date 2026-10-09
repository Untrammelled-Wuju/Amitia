import { ref, watch, type Ref } from "vue";
import { apiClient } from "@/composables/useApi";
import {
  getApiBaseURLForPath,
  getBackendAuthHeaders,
} from "@/runtime/runtime-adapter";

interface MediaTicketResponse {
  url?: string;
  expiresAt?: string;
}

interface CachedMediaTicket {
  url: string;
  expiresAt: number;
}

const mediaTicketCache = new Map<string, CachedMediaTicket>();
const mediaBlobCache = new Map<string, string>();
if (typeof window !== "undefined") {
  window.addEventListener("amitia:runtime-connection-changed", () => {
    mediaTicketCache.clear();
    for (const url of mediaBlobCache.values()) URL.revokeObjectURL(url);
    mediaBlobCache.clear();
  });
}

function artifactIdFromRaw(raw: string): string {
  const value = String(raw || "").trim();
  const resourceMatch = /^amitia:\/\/artifacts\/([^/?#]+)$/i.exec(value);
  if (resourceMatch) return decodeURIComponent(resourceMatch[1]);
  const contentMatch = /\/api\/artifacts\/v1\/([^/?#]+)\/content(?:[?#]|$)/i.exec(value);
  if (contentMatch) return decodeURIComponent(contentMatch[1]);
  return "";
}

async function getMediaTicket(artifactId: string): Promise<string> {
  const base = await getApiBaseURLForPath("/api/artifacts/v1");
  const key = `${base}:${artifactId}`;
  if (new URL(base).pathname.includes("/internal/device-mesh/provider")) {
    const cachedBlob = mediaBlobCache.get(key);
    if (cachedBlob) return cachedBlob;
    const response = await apiClient.get<Blob>(
      `/api/artifacts/v1/${encodeURIComponent(artifactId)}/content`,
      { responseType: "blob" },
    );
    const url = URL.createObjectURL(response.data);
    mediaBlobCache.set(key, url);
    return url;
  }
  const cached = mediaTicketCache.get(key);
  if (cached && cached.expiresAt > Date.now() + 30_000) return cached.url;
  const response = await apiClient.get<MediaTicketResponse>(
    `/api/artifacts/v1/${encodeURIComponent(artifactId)}/media-ticket`,
  );
  const payload = response.data as MediaTicketResponse;
  const url = String(payload?.url || "").trim();
  if (!url) throw new Error("后端未返回媒体访问地址");
  const expiresAt = Date.parse(String(payload?.expiresAt || ""));
  const absolute = /^https?:\/\//i.test(url) ? url : `${base.replace(/\/+$/, "")}/${url.replace(/^\/+/, "")}`;
  mediaTicketCache.set(key, {
    url: absolute,
    expiresAt: Number.isFinite(expiresAt) ? expiresAt : Date.now() + 55 * 60_000,
  });
  return absolute;
}

export function invalidateConversationMedia(raw: string) {
  const id = artifactIdFromRaw(raw);
  for (const key of mediaTicketCache.keys()) {
    if (key.endsWith(`:${id}`)) mediaTicketCache.delete(key);
  }
  for (const [key, url] of mediaBlobCache) {
    if (key.endsWith(`:${id}`)) {
      URL.revokeObjectURL(url);
      mediaBlobCache.delete(key);
    }
  }
}

export async function loadConversationText(raw: string): Promise<{ source: string; url: string }> {
  const url = await resolveConversationMediaUrl(raw);
  const response = await fetch(url, { credentials: "omit" });
  if (!response.ok) throw new Error(`文件加载失败 (${response.status})`);
  const source = await response.text();
  if (source.length > 5 * 1024 * 1024) throw new Error("文件过大，无法预览，请下载查看");
  return { source, url };
}

export async function resolveEmbeddedConversationMedia(raw: string): Promise<string> {
  const url = await resolveConversationMediaUrl(raw);
  if (url.startsWith("data:")) return url;
  const response = await fetch(url, { credentials: "omit" });
  if (!response.ok) throw new Error(`图片加载失败 (${response.status})`);
  const blob = await response.blob();
  if (blob.size > 25 * 1024 * 1024) throw new Error("图片过大，无法内嵌预览");
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(new Error("图片读取失败"));
    reader.readAsDataURL(blob);
  });
}

async function absoluteMediaUrl(path: string): Promise<string> {
  const value = String(path || "").trim();
  if (!value) return "";
  if (/^(data|blob|file):/i.test(value) || /^https?:\/\//i.test(value)) return value;
  if (!value.startsWith("/")) return value;
  const base = await getApiBaseURLForPath(value);
  return new URL(value, `${base.replace(/\/+$/, "")}/`).toString();
}

export async function resolveConversationMediaUrl(raw: string): Promise<string> {
  const value = String(raw || "").trim();
  if (!value) return "";
  const artifactId = artifactIdFromRaw(value);
  if (artifactId) {
    return absoluteMediaUrl(await getMediaTicket(artifactId));
  }
  return absoluteMediaUrl(value);
}

export async function resolveConversationDownloadUrl(raw: string): Promise<string> {
  const resolved = await resolveConversationMediaUrl(raw);
  if (!resolved) return "";
  const url = new URL(resolved);
  if (!["http:", "https:"].includes(url.protocol)) return resolved;
  url.searchParams.set("download", "1");
  return url.toString();
}

function extensionFromUrl(value: string): string {
  try {
    const pathname = new URL(value).pathname;
    const match = /\.([A-Za-z0-9]{1,10})$/.exec(pathname);
    return match ? `.${match[1].toLowerCase()}` : "";
  } catch {
    return "";
  }
}

function extensionFromMimeType(value: string): string {
  switch (String(value || "").toLowerCase()) {
    case "image/jpeg":
      return ".jpg";
    case "image/png":
      return ".png";
    case "image/gif":
      return ".gif";
    case "image/webp":
      return ".webp";
    case "video/mp4":
      return ".mp4";
    case "video/quicktime":
      return ".mov";
    case "audio/mpeg":
      return ".mp3";
    case "audio/wav":
      return ".wav";
    case "audio/ogg":
      return ".ogg";
    case "audio/mp4":
      return ".m4a";
    case "application/pdf":
      return ".pdf";
    case "application/zip":
      return ".zip";
    default:
      return "";
  }
}

function safeFileName(
  value: string,
  fallback: string,
  url: string,
  mimeType = "",
): string {
  let name = String(value || "")
    .replace(/[\u0000-\u001f<>:"/\\|?*]/g, "-")
    .trim()
    .slice(0, 180);
  if (!name) name = fallback;
  if (!/\.[A-Za-z0-9]{1,10}$/.test(name)) {
    name += extensionFromUrl(url) || extensionFromMimeType(mimeType) || ".bin";
  }
  return name;
}

async function saveBlob(blob: Blob, fileName: string): Promise<void> {
  const picker = (window as any).showSaveFilePicker;
  if (typeof picker === "function") {
    const handle = await picker({ suggestedName: fileName });
    const writable = await handle.createWritable();
    await writable.write(blob);
    await writable.close();
    return;
  }
  const objectUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectUrl;
  anchor.download = fileName;
  anchor.click();
  URL.revokeObjectURL(objectUrl);
}

export async function downloadConversationMedia(
  raw: string,
  fileName: string,
  mimeType = "",
): Promise<boolean> {
  const resolved = await resolveConversationDownloadUrl(raw);
  if (!resolved) throw new Error("附件地址为空");
  const suggestedName = safeFileName(
    fileName,
    "attachment",
    resolved,
    mimeType,
  );
  if (/^(data|blob):/i.test(resolved)) {
    const response = await fetch(resolved);
    await saveBlob(await response.blob(), suggestedName);
    return true;
  }
  const backendBase = await getApiBaseURLForPath("/api/");
  const isBackendRequest =
    new URL(resolved).origin === new URL(backendBase).origin;
  if (window.amitiaDesktop?.saveConversationAttachment) {
    let headers: Record<string, string> = {};
    if (isBackendRequest) {
      headers = await getBackendAuthHeaders("business");
    }
    const result = await window.amitiaDesktop.saveConversationAttachment({
      url: resolved,
      suggestedName,
      headers,
    });
    return result.saved === true;
  }
  if (isBackendRequest) {
    const response = await apiClient.get<Blob>(resolved, {
      responseType: "blob",
    });
    await saveBlob(response.data, suggestedName);
    return true;
  }
  const response = await fetch(resolved, { credentials: "include" });
  if (!response.ok) throw new Error(`附件下载失败 (${response.status})`);
  await saveBlob(await response.blob(), suggestedName);
  return true;
}

export function useResolvedConversationMediaUrl(
  source: Ref<string | undefined>,
) {
  const resolved = ref("");
  const loading = ref(false);
  const error = ref("");
  let generation = 0;
  watch(
    source,
    async (value) => {
      const current = ++generation;
      const raw = String(value || "").trim();
      if (!raw) {
        resolved.value = "";
        loading.value = false;
        error.value = "";
        return;
      }
      loading.value = true;
      error.value = "";
      try {
        const next = await resolveConversationMediaUrl(raw);
        if (current !== generation) return;
        resolved.value = next;
      } catch (reason) {
        if (current !== generation) return;
        resolved.value = "";
        error.value = reason instanceof Error ? reason.message : "媒体加载失败";
      } finally {
        if (current === generation) loading.value = false;
      }
    },
    { immediate: true },
  );
  return { resolved, loading, error };
}
