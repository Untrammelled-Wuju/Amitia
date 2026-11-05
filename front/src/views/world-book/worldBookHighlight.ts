function escapeHTML(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function escapePattern(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export function highlightWorldBookMatch(text: unknown, pattern: unknown): string {
  const source = typeof text === "string" ? text : "";
  const needle = typeof pattern === "string" ? pattern : "";
  if (!source) return "";
  if (!needle) return escapeHTML(source);

  const matches = [...source.matchAll(new RegExp(escapePattern(needle), "gi"))];
  if (!matches.length) return escapeHTML(source);

  let cursor = 0;
  let output = "";
  for (const match of matches) {
    const index = match.index ?? 0;
    output += escapeHTML(source.slice(cursor, index));
    output += `<mark>${escapeHTML(match[0])}</mark>`;
    cursor = index + match[0].length;
  }
  return output + escapeHTML(source.slice(cursor));
}
