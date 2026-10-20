export interface CitationSnapshotEntry {
  marker: string;
  kind: "knowledge" | "web";
  index: number;
  evidenceId?: string;
  refId?: string;
  sourceId?: string;
  sourceRevision?: number;
  unitId?: string;
  title?: string;
  url?: string;
  locator?: Record<string, unknown>;
  excerpt?: string;
  contentHash?: string;
  checksum?: string;
}

const citationNumericLocatorKeys = ["page", "row", "sheet", "paragraph", "section", "index"] as const;
const citationLocatorAliases: Record<(typeof citationNumericLocatorKeys)[number], readonly string[]> = {
  page: ["page", "pageNumber", "page_number"],
  row: ["row", "rowStart", "row_start", "rowEnd", "row_end"],
  sheet: ["sheet", "sheetName", "sheet_name"],
  paragraph: ["paragraph", "paragraphIndex", "paragraph_index"],
  section: ["section"],
  index: ["index"],
};
const citationTextLocatorKeys = ["kind", "field"] as const;
const sensitiveCitationQueryKeys = ["token", "secret", "password", "passwd", "apikey", "api_key", "auth", "signature", "credential", "session", "access_key", "code"];

export function normalizeCitationURL(raw: unknown): string | undefined {
  if (typeof raw !== "string" || !raw.trim() || Array.from(raw).length > 2048 || /[\r\n\t\\]/.test(raw)) return undefined;
  try {
    const parsed = new URL(raw.trim());
    if (!["https:", "http:"].includes(parsed.protocol) || !parsed.hostname || parsed.username || parsed.password) return undefined;
    const keys: string[] = [];
    parsed.searchParams.forEach((_value, key) => keys.push(key));
    for (const key of keys) {
      if (sensitiveCitationQueryKeys.some((part) => key.toLowerCase().includes(part))) parsed.searchParams.delete(key);
    }
    return parsed.toString();
  } catch {
    return undefined;
  }
}

export function normalizeCitationLocator(raw: unknown): Record<string, unknown> | undefined {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return undefined;
  const record = raw as Record<string, unknown>;
  const locator: Record<string, unknown> = {};
  for (const key of citationNumericLocatorKeys) {
    const alias = citationLocatorAliases[key].find((name) => Object.prototype.hasOwnProperty.call(record, name));
    const rawValue = alias ? record[alias] : undefined;
    if (key === "sheet" && alias !== "sheet" && typeof rawValue === "string" && safeCitationLocatorLabel(rawValue)) {
      locator[key] = rawValue.trim();
      continue;
    }
    const value = typeof rawValue === "string" && /^\d+$/.test(rawValue.trim()) ? Number(rawValue) : rawValue;
    if (typeof value === "number" && Number.isSafeInteger(value) && value >= 0 && value <= 1000000000) locator[key] = value;
    else if (key === "sheet" && typeof rawValue === "string" && safeCitationLocatorLabel(rawValue)) locator[key] = rawValue.trim();
  }
  for (const key of citationTextLocatorKeys) {
    const value = record[key];
    if (typeof value === "string" && safeCitationLocatorLabel(value)) locator[key] = value.trim();
  }
  return Object.keys(locator).length ? locator : undefined;
}

function safeCitationLocatorLabel(value: string): boolean {
  return !!value.trim() && Array.from(value.trim()).length <= 128 && !/[\/\\:\x00-\x1F\x7F-\x9F]/.test(value);
}

export function normalizeCitationSnapshot(raw: unknown): CitationSnapshotEntry[] {
  if (!Array.isArray(raw)) return [];
  const result: CitationSnapshotEntry[] = [];
  const seen = new Set<string>();
  for (const value of raw.slice(0, 96)) {
    if (result.length >= 24) break;
    if (!value || typeof value !== "object" || Array.isArray(value)) continue;
    const entry = value as Record<string, unknown>;
    const kind = entry.kind;
    const index = entry.index;
    if (kind !== "knowledge" && kind !== "web") continue;
    if (typeof index !== "number" || !Number.isSafeInteger(index) || index < 1 || index > 999999) continue;
    const marker = kind === "knowledge" ? `[K${index}]` : `[${index}]`;
    if (entry.marker !== marker || seen.has(marker)) continue;
    seen.add(marker);
    const citation: CitationSnapshotEntry = { marker, kind, index };
    const fields = {
      evidenceId: 256, refId: 256, sourceId: 256, unitId: 256,
      title: 200, url: 2048, excerpt: 1200, contentHash: 128, checksum: 128,
    } as const;
    for (const [key, limit] of Object.entries(fields)) {
      const text = entry[key];
      if (typeof text === "string") (citation as unknown as Record<string, unknown>)[key] = Array.from(text.trim()).slice(0, limit).join("");
    }
    if (typeof entry.sourceRevision === "number" && Number.isSafeInteger(entry.sourceRevision) && entry.sourceRevision >= 0) {
      citation.sourceRevision = entry.sourceRevision;
    }
    citation.url = normalizeCitationURL(entry.url);
    citation.locator = normalizeCitationLocator(entry.locator);
    result.push(citation);
  }
  return result;
}
