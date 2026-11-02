export interface RecentLogLine {
  file?: string;
  line?: string;
  time?: string;
}

export function formatRecentLogText(logs: RecentLogLine[]): string {
  const groups = new Map<string, string[]>();
  for (const log of logs) {
    const file = log.file?.trim() || "runtime.log";
    const line = typeof log.line === "string" ? log.line : String(log.line ?? "");
    const lines = groups.get(file) ?? [];
    lines.push(line);
    groups.set(file, lines);
  }
  return Array.from(groups, ([file, lines]) => {
    return `==> ${file} <==\n${lines.join("\n")}`;
  }).join("\n\n");
}
