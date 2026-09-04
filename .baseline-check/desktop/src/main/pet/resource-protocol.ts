import { protocol } from "electron";
import { lstat, readFile } from "node:fs/promises";
import { extname, isAbsolute } from "node:path";
import { createHash } from "node:crypto";
import { PET_PROTOCOL_SCHEME } from "../../shared/animation-ipc";
import type { Manifest, IntegrityFileEntry } from "./resource-loader";
import { decodePackagePathFromUrl, resolvePackagePathUnderRoot, tryNormalizePackagePath } from "./package-path";

const MIME_MAP: Record<string, string> = { ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp", ".json": "application/json", ".txt": "text/plain" };
const SECURITY_HEADERS: Record<string, string> = {
  "X-Content-Type-Options": "nosniff", "Cache-Control": "no-store, no-cache, must-revalidate",
  "Access-Control-Allow-Origin": "*", "Access-Control-Allow-Methods": "GET, HEAD, OPTIONS", "Access-Control-Allow-Headers": "*",
  "Cross-Origin-Resource-Policy": "cross-origin",
};
const JSON_HEADERS: Record<string, string> = { "Content-Type": "application/json; charset=utf-8", ...SECURITY_HEADERS };

export type ResourceIndexEntry = IntegrityFileEntry;
export type ResourceIndex = Map<string, ResourceIndexEntry>;
export interface ActiveInstallationInfo { installationId: string; installPath: string; manifest: Manifest; resourceIndex: ResourceIndex; }

let schemeRegistered = false;
try {
  protocol.registerSchemesAsPrivileged([{ scheme: PET_PROTOCOL_SCHEME, privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true } }]);
  schemeRegistered = true;
} catch { schemeRegistered = false; }

export function getMimeFromPath(filePath: string): string { return MIME_MAP[extname(filePath).toLowerCase()] ?? "application/octet-stream"; }

export function buildResourceIndex(manifest: Manifest): ResourceIndex {
  const index: ResourceIndex = new Map();
  const files = manifest.integrity?.files;
  if (!files || !Array.isArray(files)) return index;
  for (const entry of files) {
    if (!entry || typeof entry.path !== "string") continue;
    const canonical = tryNormalizePackagePath(entry.path);
    if (!canonical || index.has(canonical)) continue;
    index.set(canonical, entry);
  }
  return index;
}

function computeSha256(buffer: Buffer): string { return createHash("sha256").update(buffer).digest("hex"); }

export class PetResourceProtocolRegistry {
  private static instance: PetResourceProtocolRegistry | null = null;
  private registered = false;
  private getActiveInstallation: (() => ActiveInstallationInfo | null) | null = null;
  private constructor() {}
  static getInstance(): PetResourceProtocolRegistry {
    if (!PetResourceProtocolRegistry.instance) PetResourceProtocolRegistry.instance = new PetResourceProtocolRegistry();
    return PetResourceProtocolRegistry.instance;
  }
  setActiveInstallationResolver(getActiveInstallation: () => ActiveInstallationInfo | null): void { this.getActiveInstallation = getActiveInstallation; this.ensureRegistered(); }
  clearActiveInstallationResolver(): void { this.getActiveInstallation = null; }

  private ensureRegistered(): void {
    if (this.registered) return;
    this.registered = true;
    protocol.handle(PET_PROTOCOL_SCHEME, async (request) => {
      if (request.method === "OPTIONS") return new Response(null, { status: 204, headers: SECURITY_HEADERS });
      if (request.method !== "GET" && request.method !== "HEAD") return new Response(JSON.stringify({ error: "method_not_allowed" }), { status: 405, headers: JSON_HEADERS });
      const resolver = this.getActiveInstallation;
      const active = resolver?.() ?? null;
      if (!active) return new Response(JSON.stringify({ error: "no_active_installation" }), { status: 503, headers: JSON_HEADERS });
      if (!isAbsolute(active.installPath)) return new Response(JSON.stringify({ error: "invalid_install_root" }), { status: 500, headers: JSON_HEADERS });

      const parsed = parsePetUrl(request.url);
      if (!parsed) return new Response(JSON.stringify({ error: "invalid_url" }), { status: 400, headers: JSON_HEADERS });
      if (parsed.installationId !== active.installationId) return new Response(JSON.stringify({ error: "installation_mismatch" }), { status: 403, headers: JSON_HEADERS });

      const indexEntry = active.resourceIndex.get(parsed.relativePath);
      if (!indexEntry) return new Response(JSON.stringify({ error: "resource_not_declared" }), { status: 403, headers: JSON_HEADERS });

      let fullPath: string;
      try {
        fullPath = resolvePackagePathUnderRoot(active.installPath, parsed.relativePath);
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        const code = (error as NodeJS.ErrnoException | undefined)?.code;
        if (code === "ENOENT") {
          return new Response(JSON.stringify({ error: "resource_not_found" }), { status: 404, headers: JSON_HEADERS });
        }
        if (message.includes("PACKAGE_SYMLINK_FORBIDDEN")) {
          return new Response(JSON.stringify({ error: "symlink_forbidden" }), { status: 403, headers: JSON_HEADERS });
        }
        return new Response(JSON.stringify({ error: "path_outside_install" }), { status: 403, headers: JSON_HEADERS });
      }

      let stats;
      try {
        stats = await lstat(fullPath);
      } catch {
        return new Response(JSON.stringify({ error: "resource_not_found" }), { status: 404, headers: JSON_HEADERS });
      }
      if (!stats.isFile()) return new Response(JSON.stringify({ error: "resource_not_found" }), { status: 404, headers: JSON_HEADERS });
      if (typeof indexEntry.bytes === "number" && stats.size !== indexEntry.bytes) {
        return new Response(JSON.stringify({ error: "size_mismatch", declared: indexEntry.bytes, actual: stats.size }), { status: 403, headers: JSON_HEADERS });
      }

      try {
        const content = await readFile(fullPath);
        const actualHash = computeSha256(content);
        if (actualHash !== indexEntry.sha256) return new Response(JSON.stringify({ error: "integrity_check_failed", declared: indexEntry.sha256, actual: actualHash }), { status: 403, headers: JSON_HEADERS });
        const headers = { "Content-Type": indexEntry.mediaType || getMimeFromPath(parsed.relativePath), ...SECURITY_HEADERS };
        return new Response(request.method === "HEAD" ? null : new Uint8Array(content), { status: 200, headers });
      } catch { return new Response(JSON.stringify({ error: "read_failed" }), { status: 500, headers: JSON_HEADERS }); }
    });
  }
}

export function registerPetProtocol(getActiveInstallation: () => ActiveInstallationInfo | null): void {
  PetResourceProtocolRegistry.getInstance().setActiveInstallationResolver(getActiveInstallation);
}

interface ParsedPetUrl { installationId: string; relativePath: string; }
function parsePetUrl(rawUrl: string): ParsedPetUrl | null {
  const prefix = `${PET_PROTOCOL_SCHEME}://installation/`;
  if (!rawUrl.startsWith(prefix)) return null;
  let rest = rawUrl.slice(prefix.length);
  const queryIdx = rest.indexOf("?"); const hashIdx = rest.indexOf("#");
  let cut = rest.length; if (queryIdx >= 0) cut = Math.min(cut, queryIdx); if (hashIdx >= 0) cut = Math.min(cut, hashIdx); rest = rest.slice(0, cut);
  const slashIdx = rest.indexOf("/");
  if (slashIdx <= 0 || slashIdx === rest.length - 1) return null;
  let installationId: string;
  try { installationId = decodeURIComponent(rest.slice(0, slashIdx)); } catch { return null; }
  if (!installationId || /[\u0000-\u001f\u007f]/.test(installationId)) return null;
  try { return { installationId, relativePath: decodePackagePathFromUrl(rest.slice(slashIdx + 1)) }; }
  catch { return null; }
}

export { schemeRegistered };
