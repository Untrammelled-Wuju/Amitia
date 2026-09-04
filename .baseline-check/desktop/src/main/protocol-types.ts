export interface ProtocolResourceRequest {
  extensionId: string;
  moduleId: string;
  path: string;
}

export interface ProtocolResourceResponse {
  content: Buffer;
  mimeType: string;
  headers: Record<string, string>;
}

export const ALLOWED_MIME_TYPES = new Set<string>([
  "text/html",
  "text/css",
  "text/javascript",
  "application/javascript",
  "application/json",
  "image/png",
  "image/jpeg",
  "image/webp",
  "image/svg+xml",
  "font/woff2",
  "application/wasm",
]);

export function lookupMIME(filePath: string): string {
  const lower = filePath.toLowerCase();
  const dot = lower.lastIndexOf(".");
  const ext = dot >= 0 ? lower.slice(dot + 1) : "";
  switch (ext) {
    case "html":
    case "htm":
      return "text/html";
    case "css":
      return "text/css";
    case "js":
    case "mjs":
      return "text/javascript";
    case "json":
      return "application/json";
    case "png":
      return "image/png";
    case "jpg":
    case "jpeg":
      return "image/jpeg";
    case "webp":
      return "image/webp";
    case "svg":
      return "image/svg+xml";
    case "woff2":
      return "font/woff2";
    case "wasm":
      return "application/wasm";
    default:
      return "";
  }
}

export function sanitizePath(requested: string): string | null {
  const input = requested ?? "";
  if (input.length > 1024) {
    return null;
  }
  if (input.includes("\0")) {
    return null;
  }
  const lower = input.toLowerCase();
  if (
    lower.includes("%2e") ||
    lower.includes("%5c") ||
    lower.includes("%2f") ||
    lower.includes("%00")
  ) {
    return null;
  }
  if (input.includes("\\")) {
    return null;
  }
  const normalized = input.replace(/\/+/g, "/");
  const trimmed = normalized.replace(/^\/+/, "").replace(/\/+$/, "");
  if (/^[a-zA-Z]:/.test(trimmed)) {
    return null;
  }
  const rawSegments = trimmed.split("/").filter((seg) => seg.length > 0);
  for (const seg of rawSegments) {
    if (seg === "..") {
      return null;
    }
  }
  const cleanSegments: string[] = [];
  for (const seg of rawSegments) {
    if (seg === ".") {
      continue;
    }
    let decoded: string;
    try {
      decoded = decodeURIComponent(seg);
    } catch {
      decoded = seg;
    }
    if (
      decoded.includes("/") ||
      decoded.includes("\\") ||
      decoded.includes("\0") ||
      decoded === ".."
    ) {
      return null;
    }
    cleanSegments.push(decoded);
  }
  const clean = cleanSegments.join("/");
  if (clean.includes("..")) {
    return null;
  }
  return clean || "index.html";
}
