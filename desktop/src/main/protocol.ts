import { protocol } from "electron";
import { stat as fsStat, readFile } from "node:fs/promises";
import path from "node:path";
import { ALLOWED_MIME_TYPES, lookupMIME, sanitizePath } from "./protocol-types";
import type { ProtocolResourceRequest } from "./protocol-types";

const EXTENSION_SCHEME = "amitia-extension";
const SCHEME_PREFIX = `${EXTENSION_SCHEME}://`;

const EXTENSION_CSP =
  "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'none'; media-src 'self'; object-src 'none'; frame-src 'none'; child-src 'none'; worker-src 'none'; manifest-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; navigate-to 'none'";

const PRELOAD_SCRIPT =
  '(function(){if(window.__amitiaExtensionBridge)return;var bridge={ready:false,pending:[]};window.__amitiaExtensionBridge=bridge;function send(method,payload){try{window.parent.postMessage({type:"amitia.extension.bridge",method:method,payload:payload},"*");}catch(e){}}bridge.send=send;window.addEventListener("message",function(event){if(!event.data||event.data.type!=="amitia.extension.host")return;bridge.pending.push(event.data);});send("ui.ready",{});})();';

const SECURITY_HEADERS: Record<string, string> = {
  "X-Content-Type-Options": "nosniff",
  "X-Frame-Options": "SAMEORIGIN",
  "Cache-Control": "no-store, no-cache, must-revalidate",
};

let registered = false;

export function registerExtensionProtocol(extRoot: string): void {
  if (registered) return;
  registered = true;
  const root = path.resolve(extRoot);

  protocol.handle(EXTENSION_SCHEME, async (request) => {
    const parsed = parseExtensionUrl(request.url);
    if (!parsed || !parsed.extensionId || !parsed.moduleId) {
      return jsonError(400, "invalid_request");
    }

    const cleanPath = sanitizePath(parsed.path);
    if (!cleanPath) {
      return jsonError(403, "invalid_path");
    }

    const baseDir = path.resolve(root, parsed.extensionId);
    const fullPath = path.resolve(baseDir, cleanPath);
    if (fullPath !== baseDir && !fullPath.startsWith(baseDir + path.sep)) {
      return jsonError(403, "path_outside_bundle");
    }

    let stats;
    try {
      stats = await fsStat(fullPath);
    } catch {
      return jsonError(404, "resource_not_found");
    }
    if (stats.isDirectory()) {
      return jsonError(404, "resource_not_found");
    }

    const mimeType = lookupMIME(cleanPath);
    if (!mimeType || !ALLOWED_MIME_TYPES.has(mimeType)) {
      return jsonError(415, "mime_not_allowed");
    }

    try {
      if (mimeType === "text/html") {
        const raw = await readFile(fullPath, "utf-8");
        const html = injectIntoHtml(raw, PRELOAD_SCRIPT, EXTENSION_CSP);
        return new Response(html, {
          status: 200,
          headers: {
            "Content-Type": "text/html; charset=utf-8",
            ...SECURITY_HEADERS,
          },
        });
      }
      const content = await readFile(fullPath);
      return new Response(new Uint8Array(content), {
        status: 200,
        headers: {
          "Content-Type": mimeType,
          ...SECURITY_HEADERS,
        },
      });
    } catch {
      return jsonError(500, "read_failed");
    }
  });
}

function parseExtensionUrl(rawUrl: string): ProtocolResourceRequest | null {
  let rest: string;
  if (rawUrl.startsWith(SCHEME_PREFIX)) {
    rest = rawUrl.slice(SCHEME_PREFIX.length);
  } else {
    const idx = rawUrl.indexOf("://");
    if (idx < 0) return null;
    rest = rawUrl.slice(idx + 3);
  }

  const queryIdx = rest.indexOf("?");
  const hashIdx = rest.indexOf("#");
  let cut = rest.length;
  if (queryIdx >= 0 && queryIdx < cut) cut = queryIdx;
  if (hashIdx >= 0 && hashIdx < cut) cut = hashIdx;
  rest = rest.slice(0, cut).replace(/^\/+/, "");

  const segments = rest.split("/");
  const extensionId = safeDecode(segments[0] ?? "");
  const moduleId = safeDecode(segments[1] ?? "");
  const resourcePath = segments.slice(2).map(safeDecode).join("/");

  if (!isValidSegment(extensionId) || !isValidSegment(moduleId)) {
    return null;
  }
  return { extensionId, moduleId, path: resourcePath };
}

function isValidSegment(segment: string): boolean {
  if (!segment) return false;
  if (
    segment.includes("/") ||
    segment.includes("\\") ||
    segment.includes("\0") ||
    segment === ".." ||
    segment === "."
  ) {
    return false;
  }
  return true;
}

function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function injectIntoHtml(html: string, preload: string, csp: string): string {
  const cspMeta = `<meta http-equiv="Content-Security-Policy" content="${csp}">`;
  const scriptTag = `<script>${preload}</script>`;
  const headIdx = html.lastIndexOf("</head>");
  if (headIdx >= 0) {
    return (
      html.slice(0, headIdx) + cspMeta + "\n" + scriptTag + "\n" + html.slice(headIdx)
    );
  }
  const bodyIdx = html.lastIndexOf("</body>");
  if (bodyIdx >= 0) {
    return (
      html.slice(0, bodyIdx) + cspMeta + "\n" + scriptTag + "\n" + html.slice(bodyIdx)
    );
  }
  return cspMeta + "\n" + scriptTag + "\n" + html;
}

function jsonError(status: number, code: string): Response {
  return new Response(JSON.stringify({ error: code }), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
}
