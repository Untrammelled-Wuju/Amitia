import { protocol } from "electron";
import { readFile, stat } from "node:fs/promises";
import path from "node:path";
import { getCoreEndpoint } from "./core-endpoint";

export const APP_PROTOCOL_SCHEME = "app";
export const APP_PROTOCOL_HOST = "amitia";
export const APP_PROTOCOL_ORIGIN = `${APP_PROTOCOL_SCHEME}://${APP_PROTOCOL_HOST}`;

const MIME_TYPES: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".webp": "image/webp",
  ".gif": "image/gif",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".ttf": "font/ttf",
  ".otf": "font/otf",
  ".wasm": "application/wasm",
  ".map": "application/json; charset=utf-8",
};

const SECURITY_HEADERS: Record<string, string> = {
  "X-Content-Type-Options": "nosniff",
  "Cache-Control": "no-cache",
};

const CORE_RESOURCE_PREFIX = "/api/extension/webui/resource/";
const CORE_RESOURCE_RESPONSE_HEADERS = [
  "content-type",
  "content-security-policy",
  "cache-control",
  "x-content-type-options",
  "referrer-policy",
];

let schemeRegistered = false;
let handlerRegistered = false;

try {
  protocol.registerSchemesAsPrivileged([
    {
      scheme: APP_PROTOCOL_SCHEME,
      privileges: {
        standard: true,
        secure: true,
        supportFetchAPI: true,
        corsEnabled: true,
        stream: true,
      },
    },
  ]);
  schemeRegistered = true;
} catch (error) {
  console.error("[original-error]", error);
schemeRegistered = false;
}

export function registerAppProtocol(rendererRoot: string): void {
  if (handlerRegistered) return;
  if (!schemeRegistered) {
    throw new Error("app protocol scheme was not registered before app readiness");
  }

  const root = path.resolve(rendererRoot);
  handlerRegistered = true;

  protocol.handle(APP_PROTOCOL_SCHEME, async (request) => {
    let parsed: URL;
    try {
      parsed = new URL(request.url);
    } catch (error) {
  console.error("[original-error]", error);
return textResponse(400, error instanceof Error ? error.message : String(error));
    }

    if (parsed.hostname !== APP_PROTOCOL_HOST) {
      return textResponse(403, "invalid app host");
    }
    if (request.method !== "GET" && request.method !== "HEAD") {
      return textResponse(405, "method not allowed");
    }

    if (parsed.pathname.startsWith(CORE_RESOURCE_PREFIX)) {
      return proxyCoreResource(parsed, request.method);
    }

    const relativePath = normalizeRequestPath(parsed.pathname);
    if (relativePath === null) {
      return textResponse(403, "invalid app path");
    }

    let fullPath = path.resolve(root, relativePath || "index.html");
    if (!isUnderRoot(root, fullPath)) {
      return textResponse(403, "path outside renderer root");
    }

    try {
      const info = await stat(fullPath);
      if (info.isDirectory()) {
        fullPath = path.join(fullPath, "index.html");
      }
    } catch (error) {
  console.error("[original-error]", error);
// Hash routing never reaches the protocol handler. For an accidental
      // history-mode route, fall back to the SPA entry only when the request
      // does not look like a static asset.
      if (path.extname(relativePath) === "") {
        fullPath = path.join(root, "index.html");
      } else {
        return textResponse(404, "resource not found");
      }
    }

    if (!isUnderRoot(root, fullPath)) {
      return textResponse(403, "path outside renderer root");
    }

    try {
      const content = await readFile(fullPath);
      const contentType =
        MIME_TYPES[path.extname(fullPath).toLowerCase()] ??
        "application/octet-stream";
      return new Response(
        request.method === "HEAD" ? null : new Uint8Array(content),
        {
          status: 200,
          headers: {
            "Content-Type": contentType,
            ...SECURITY_HEADERS,
          },
        },
      );
    } catch (error) {
  console.error("[original-error]", error);
return textResponse(404, "resource not found");
    }
  });
}

async function proxyCoreResource(parsed: URL, method: string): Promise<Response> {
  const baseURL = getCoreEndpoint()?.httpBaseURL || "";
  if (!baseURL) {
    return textResponse(503, "core is not ready");
  }
  let upstream: Response;
  try {
    upstream = await fetch(`${baseURL}${parsed.pathname}${parsed.search}`, {
      method,
      redirect: "error",
    });
  } catch (error) {
  console.error("[original-error]", error);
return textResponse(502, error instanceof Error ? error.message : String(error));
  }
  const headers = new Headers();
  for (const name of CORE_RESOURCE_RESPONSE_HEADERS) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new Response(method === "HEAD" ? null : upstream.body, {
    status: upstream.status,
    headers,
  });
}

function normalizeRequestPath(pathname: string): string | null {
  let decoded: string;
  try {
    decoded = decodeURIComponent(pathname);
  } catch (error) {
  console.error("[original-error]", error);
return null;
  }
  const normalized = decoded.replace(/^\/+/, "").replace(/\\/g, "/");
  const segments = normalized.split("/");
  if (segments.some((segment) => segment === ".." || segment.includes("\0"))) {
    return null;
  }
  return normalized;
}

function isUnderRoot(root: string, candidate: string): boolean {
  return candidate === root || candidate.startsWith(root + path.sep);
}

function textResponse(status: number, message: string): Response {
  return new Response(message, {
    status,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      ...SECURITY_HEADERS,
    },
  });
}
