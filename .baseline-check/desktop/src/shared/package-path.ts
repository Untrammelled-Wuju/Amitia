const MAX_PACKAGE_PATH_BYTES = 512;
const MAX_PACKAGE_SEGMENT_BYTES = 255;
const utf8Encoder = new TextEncoder();

const WINDOWS_RESERVED_NAMES = new Set([
  "con", "prn", "aux", "nul",
  "com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
  "lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
]);
const WINDOWS_FORBIDDEN_CHAR = /[<>:"|?*]/;
const CONTROL_CHAR = /[\u0000-\u001f\u007f]/;

export function normalizePackagePath(raw: string): string {
  if (typeof raw !== "string" || raw.length === 0) throw new Error("PACKAGE_PATH_EMPTY");
  if (hasUnpairedSurrogate(raw)) throw new Error(`PACKAGE_PATH_INVALID_UNICODE: ${raw}`);
  if (raw !== raw.normalize("NFC")) throw new Error(`PACKAGE_PATH_NOT_NFC: ${raw}`);
  if (utf8ByteLength(raw) > MAX_PACKAGE_PATH_BYTES) throw new Error(`PACKAGE_PATH_TOO_LONG: ${raw}`);
  if (raw.startsWith("/")) throw new Error(`PACKAGE_PATH_ABSOLUTE: ${raw}`);
  if (raw.includes("\\")) throw new Error(`PACKAGE_PATH_BACKSLASH: ${raw}`);
  if (CONTROL_CHAR.test(raw)) throw new Error(`PACKAGE_PATH_CONTROL_CHAR: ${raw}`);

  const segments = raw.split("/");
  for (const segment of segments) validatePackagePathSegment(segment);
  const canonical = segments.join("/");
  if (canonical !== raw) throw new Error(`PACKAGE_PATH_NOT_CANONICAL: ${raw}`);
  return canonical;
}

export function caseFoldPackagePath(raw: string): string {
  // Go's strings.ToLower uses Unicode simple lowercase mapping per code point.
  // JavaScript lowercasing has one unconditional multi-code-point expansion
  // (U+0130 -> "i\u0307"), so lower each scalar independently and keep the
  // first scalar to reproduce Go's simple mapping exactly.
  return Array.from(normalizePackagePath(raw), (char) => {
    const lowered = Array.from(char.toLowerCase());
    return lowered[0] ?? char;
  }).join("");
}

export function tryNormalizePackagePath(raw: string): string | null {
  try { return normalizePackagePath(raw); } catch { return null; }
}

export function encodePackagePathForUrl(raw: string): string {
  return normalizePackagePath(raw).split("/").map((segment) => encodeURIComponent(segment)).join("/");
}

export function decodePackagePathFromUrl(encodedPath: string): string {
  if (typeof encodedPath !== "string" || encodedPath.length === 0) throw new Error("PACKAGE_URL_PATH_EMPTY");
  const decoded = encodedPath.split("/").map((segment) => {
    let value: string;
    try { value = decodeURIComponent(segment); }
    catch { throw new Error(`PACKAGE_URL_PATH_ENCODING_INVALID: ${segment}`); }
    if (value.includes("/") || value.includes("\\")) throw new Error(`PACKAGE_URL_PATH_ENCODED_SEPARATOR: ${segment}`);
    return value;
  });
  return normalizePackagePath(decoded.join("/"));
}

export function resolveActionResourcePackagePath(actionConfigPath: string, resourcePath: string): string {
  const config = normalizePackagePath(actionConfigPath);
  const resource = normalizePackagePath(resourcePath);
  const slash = config.lastIndexOf("/");
  const configDir = slash >= 0 ? config.slice(0, slash) : "";
  return normalizePackagePath(configDir ? `${configDir}/${resource}` : resource);
}

function validatePackagePathSegment(segment: string): void {
  if (segment.length === 0) throw new Error("PACKAGE_PATH_EMPTY_SEGMENT");
  if (segment === "." || segment === "..") throw new Error(`PACKAGE_PATH_DOT_SEGMENT: ${segment}`);
  if (utf8ByteLength(segment) > MAX_PACKAGE_SEGMENT_BYTES) throw new Error(`PACKAGE_PATH_SEGMENT_TOO_LONG: ${segment}`);
  if (WINDOWS_FORBIDDEN_CHAR.test(segment)) throw new Error(`PACKAGE_PATH_WINDOWS_FORBIDDEN_CHAR: ${segment}`);
  if (segment.endsWith(".") || segment.endsWith(" ")) throw new Error(`PACKAGE_PATH_WINDOWS_TRAILING_DOT_SPACE: ${segment}`);
  const dot = segment.indexOf(".");
  const base = (dot >= 0 ? segment.slice(0, dot) : segment).toLowerCase();
  if (WINDOWS_RESERVED_NAMES.has(base)) throw new Error(`PACKAGE_PATH_WINDOWS_RESERVED_NAME: ${segment}`);
}

function utf8ByteLength(value: string): number { return utf8Encoder.encode(value).length; }
function hasUnpairedSurrogate(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return true;
      index += 1;
    } else if (code >= 0xdc00 && code <= 0xdfff) return true;
  }
  return false;
}
