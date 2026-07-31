import { protocol } from "electron";
import { stat as fsStat, readFile } from "node:fs/promises";
import { realpathSync } from "node:fs";
import { join, normalize, relative, isAbsolute, extname } from "node:path";
import { PET_PROTOCOL_SCHEME } from "../../shared/animation-ipc";
import type { Manifest } from "./resource-loader";

const MIME_MAP: Record<string, string> = {
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".bmp": "image/bmp",
  ".json": "application/json",
  ".txt": "text/plain",
};

const SECURITY_HEADERS: Record<string, string> = {
  "X-Content-Type-Options": "nosniff",
  "Cache-Control": "no-store, no-cache, must-revalidate",
};

export interface ActiveInstallationInfo {
  installationId: string;
  installPath: string;
  manifest: Manifest;
}

let schemeRegistered = false;

try {
  protocol.registerSchemesAsPrivileged([
    {
      scheme: PET_PROTOCOL_SCHEME,
      privileges: {
        standard: true,
        secure: true,
        supportFetchAPI: true,
      },
    },
  ]);
  schemeRegistered = true;
} catch {
  schemeRegistered = false;
}

function getMimeFromPath(filePath: string): string {
  const ext = extname(filePath).toLowerCase();
  return MIME_MAP[ext] ?? "application/octet-stream";
}

function isUnsafeRelativePath(relativePath: string): boolean {
  if (!relativePath) return false;
  if (isAbsolute(relativePath)) return true;
  if (relativePath.includes("\0")) return true;
  const lower = relativePath.toLowerCase();
  if (lower.includes("%2e") || lower.includes("%5c") || lower.includes("%2f") || lower.includes("%00")) {
    return true;
  }
  const normalized = normalize(relativePath);
  const rel = normalized.replace(/\\/g, "/");
  if (rel.startsWith("..") || rel.includes("/../") || rel === "..") {
    return true;
  }
  return false;
}

function isPathWithinInstall(installPath: string, fullPath: string): boolean {
  try {
    const realInstall = realpathSync(installPath);
    const realFull = realpathSync(fullPath);
    const rel = relative(realInstall, realFull);
    return !rel.startsWith("..") && !isAbsolute(rel);
  } catch {
    return false;
  }
}

export class PetResourceProtocolRegistry {
  private static instance: PetResourceProtocolRegistry | null = null;
  private registered = false;
  private getActiveInstallation: (() => ActiveInstallationInfo | null) | null = null;

  private constructor() {}

  static getInstance(): PetResourceProtocolRegistry {
    if (!PetResourceProtocolRegistry.instance) {
      PetResourceProtocolRegistry.instance = new PetResourceProtocolRegistry();
    }
    return PetResourceProtocolRegistry.instance;
  }

  setActiveInstallationResolver(
    getActiveInstallation: () => ActiveInstallationInfo | null,
  ): void {
    this.getActiveInstallation = getActiveInstallation;
    this.ensureRegistered();
  }

  clearActiveInstallationResolver(): void {
    this.getActiveInstallation = null;
  }

  private ensureRegistered(): void {
    if (this.registered) return;
    this.registered = true;
    protocol.handle(PET_PROTOCOL_SCHEME, async (request) => {
      const resolver = this.getActiveInstallation;
      if (!resolver) {
        return new Response(JSON.stringify({ error: "no_active_installation" }), {
          status: 503,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }

      const active = resolver();
      if (!active) {
        return new Response(JSON.stringify({ error: "no_active_installation" }), {
          status: 503,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }

      const parsed = parsePetUrl(request.url);
      if (!parsed) {
        return new Response(JSON.stringify({ error: "invalid_url" }), {
          status: 400,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }

      if (parsed.installationId !== active.installationId) {
        return new Response(JSON.stringify({ error: "installation_mismatch" }), {
          status: 403,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }

      if (isUnsafeRelativePath(parsed.relativePath)) {
        return new Response(JSON.stringify({ error: "unsafe_path" }), {
          status: 403,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }

      const fullPath = join(active.installPath, parsed.relativePath);
      if (!isPathWithinInstall(active.installPath, fullPath)) {
        return new Response(JSON.stringify({ error: "path_outside_install" }), {
          status: 403,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }

      let stats;
      try {
        stats = await fsStat(fullPath);
      } catch {
        return new Response(JSON.stringify({ error: "resource_not_found" }), {
          status: 404,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }

      if (stats.isDirectory()) {
        return new Response(JSON.stringify({ error: "resource_not_found" }), {
          status: 404,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }

      try {
        const content = await readFile(fullPath);
        const mimeType = getMimeFromPath(fullPath);
        return new Response(new Uint8Array(content), {
          status: 200,
          headers: {
            "Content-Type": mimeType,
            ...SECURITY_HEADERS,
          },
        });
      } catch {
        return new Response(JSON.stringify({ error: "read_failed" }), {
          status: 500,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        });
      }
    });
  }
}

export function registerPetProtocol(
  getActiveInstallation: () => ActiveInstallationInfo | null,
): void {
  PetResourceProtocolRegistry.getInstance().setActiveInstallationResolver(getActiveInstallation);
}

interface ParsedPetUrl {
  installationId: string;
  relativePath: string;
}

function parsePetUrl(rawUrl: string): ParsedPetUrl | null {
  let rest: string;
  const prefix = `${PET_PROTOCOL_SCHEME}://`;
  if (rawUrl.startsWith(prefix)) {
    rest = rawUrl.slice(prefix.length);
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
  rest = rest.slice(0, cut);

  rest = rest.replace(/^\/+/, "");

  const installationPrefix = "installation/";
  if (rest.toLowerCase().startsWith(installationPrefix)) {
    rest = rest.slice(installationPrefix.length);
  }

  const slashIdx = rest.indexOf("/");
  if (slashIdx < 0) {
    return { installationId: decodeSegment(rest), relativePath: "" };
  }

  const installationId = decodeSegment(rest.slice(0, slashIdx));
  let relativePath = rest.slice(slashIdx + 1);

  relativePath = relativePath.replace(/\\/g, "/");

  if (!installationId) return null;

  return { installationId, relativePath };
}

function decodeSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export { schemeRegistered };
