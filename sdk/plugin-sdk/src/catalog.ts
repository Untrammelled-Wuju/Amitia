import type { LocalizedText } from "./types";

export const PLUGIN_CATALOG_SCHEMA_VERSION = "amitia.plugin.catalog/v1";

export interface PluginCatalogSource {
  readonly id: string;
  readonly type: "official" | "community" | "private" | "local" | "custom";
  readonly url?: string;
  readonly displayName?: string;
  readonly trustLevel?: string;
}

export interface PluginCatalogPublisher {
  readonly id: string;
  readonly displayName?: string;
  readonly trustLevel?: string;
  readonly homepage?: string;
}

export interface PluginCatalogPackage {
  readonly uri: string;
  readonly sha256: string;
  readonly fileName?: string;
  readonly sizeBytes?: number;
  readonly mediaType?: "application/vnd.amitia.plugin+zip" | string;
}

export interface PluginCatalogSignature {
  readonly status: "verified" | "unverified" | "unsigned" | "unknown";
  readonly algorithm?: string;
  readonly keyId?: string;
  readonly value?: string;
  readonly signedAt?: string;
}

export interface PluginCatalogCompatibility {
  readonly minHostVersion?: string;
  readonly maxHostVersion?: string;
  readonly platforms?: readonly string[];
}

export interface PluginCatalogEntry {
  readonly extensionId: string;
  readonly version: string;
  readonly name: string | LocalizedText;
  readonly description?: string | LocalizedText;
  readonly summary?: string;
  readonly package: PluginCatalogPackage;
  readonly publisher?: PluginCatalogPublisher;
  readonly signature?: PluginCatalogSignature;
  readonly trustLevel?: string;
  readonly capabilities?: readonly string[];
  readonly categories?: readonly string[];
  readonly permissions?: readonly string[];
  readonly compatibility?: PluginCatalogCompatibility;
  readonly releaseNotes?: string;
  readonly publishedAt?: string;
  readonly provenance?: Readonly<Record<string, unknown>>;
  readonly metadata?: Readonly<Record<string, unknown>>;
}

export interface PluginCatalog {
  readonly schemaVersion: typeof PLUGIN_CATALOG_SCHEMA_VERSION;
  readonly id: string;
  readonly name?: string;
  readonly source?: PluginCatalogSource;
  readonly generatedAt?: string;
  readonly expiresAt?: string;
  readonly signature?: PluginCatalogSignature;
  readonly entries: readonly PluginCatalogEntry[];
  readonly metadata?: Readonly<Record<string, unknown>>;
}

export function validatePluginCatalog(input: unknown): string[] {
  const errors: string[] = [];
  if (!isRecord(input)) return ["catalog must be an object"];
  if (input.schemaVersion !== PLUGIN_CATALOG_SCHEMA_VERSION) {
    errors.push(`schemaVersion must be ${PLUGIN_CATALOG_SCHEMA_VERSION}`);
  }
  if (typeof input.id !== "string" || !input.id.trim()) {
    errors.push("catalog.id is required");
  }
  if (!Array.isArray(input.entries)) {
    errors.push("catalog.entries must be an array");
    return errors;
  }
  const ids = new Set<string>();
  input.entries.forEach((entry, index) => {
    const path = `entries[${index}]`;
    if (!isRecord(entry)) {
      errors.push(`${path} must be an object`);
      return;
    }
    const extensionID = typeof entry.extensionId === "string" ? entry.extensionId.trim() : "";
    const version = typeof entry.version === "string" ? entry.version.trim() : "";
    if (!extensionID) errors.push(`${path}.extensionId is required`);
    if (!version) errors.push(`${path}.version is required`);
    if (extensionID && version && ids.has(`${extensionID}@${version}`)) {
      errors.push(`${path} duplicates ${extensionID}@${version}`);
    }
    ids.add(`${extensionID}@${version}`);
    if (entry.name === undefined) errors.push(`${path}.name is required`);
    if (!isRecord(entry.package)) {
      errors.push(`${path}.package is required`);
      return;
    }
    if (typeof entry.package.uri !== "string" || !entry.package.uri.trim()) {
      errors.push(`${path}.package.uri is required`);
    }
    if (typeof entry.package.sha256 !== "string" || !/^[a-fA-F0-9]{64}$/.test(entry.package.sha256)) {
      errors.push(`${path}.package.sha256 must be a 64-character hexadecimal digest`);
    }
    if (entry.package.sizeBytes !== undefined && (!Number.isSafeInteger(entry.package.sizeBytes) || Number(entry.package.sizeBytes) < 0)) {
      errors.push(`${path}.package.sizeBytes must be a non-negative safe integer`);
    }
  });
  return errors;
}

export function definePluginCatalog<const T extends PluginCatalog>(catalog: T): T {
  const errors = validatePluginCatalog(catalog);
  if (errors.length > 0) throw new Error(errors.join("; "));
  return catalog;
}

function isRecord(value: unknown): value is Record<string, any> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
