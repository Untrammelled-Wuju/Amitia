export function normalizeStylePageId(value: unknown): string {
  const text = String(value ?? "").trim();
  if (!text) return "root";
  if (!text.includes("/") && !text.includes("\\") && !text.includes("?")) {
    return text.split(".").map(normalizeStylePageSegment).join(".");
  }
  let path = text;
  let search = new URLSearchParams();
  try {
    const url = /^[a-z][a-z0-9+.-]*:\/\//i.test(text)
      ? new URL(text)
      : new URL(text, "https://amitia.local");
    path = url.pathname;
    search = url.searchParams;
  } catch (error) {
  console.error("[original-error]", error);
path = text.split(/[?#]/, 1)[0] ?? text;
  }
  path = path.replace(/\\/g, "/");
  const segments = path
    .split("/")
    .map((segment) => segment.trim())
    .filter(Boolean);
  if (segments.length === 0) return "root";
  if (
    segments.length >= 3 &&
    segments[0].toLowerCase() === "extension" &&
    segments[1].toLowerCase() === "page"
  ) {
    const page = segments.slice(2).map(normalizeStylePageSegment).join(".");
    const extensionId = search.get("extensionId")?.trim();
    return extensionId
      ? `plugin.${normalizeStylePageSegment(extensionId)}.${page}`
      : `plugin.${page}`;
  }
  return segments.map(normalizeStylePageSegment).join(".");
}

export function stylePageMatches(left: unknown, right: unknown): boolean {
  const leftText = String(left ?? "").trim();
  const rightText = String(right ?? "").trim();
  if (!leftText || !rightText) return !leftText && !rightText;
  return normalizeStylePageId(leftText) === normalizeStylePageId(rightText);
}

function normalizeStylePageSegment(value: string): string {
  const normalized = value.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, "-");
  return normalized || "root";
}
