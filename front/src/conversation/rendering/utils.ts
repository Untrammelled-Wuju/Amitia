export async function copyText(value: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    return false;
  }
}

export function formatBytes(value?: number): string {
  if (!value || value <= 0) return "";
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index += 1;
  }
  return `${size >= 10 || index === 0 ? size.toFixed(0) : size.toFixed(1)} ${units[index]}`;
}

export function formatDuration(value?: number): string {
  if (!value || value <= 0) return "00:00";
  const seconds = value > 1000 ? Math.round(value / 1000) : Math.round(value);
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`;
}

export function isSafeLink(url?: string): boolean {
  const value = String(url ?? "").trim();
  if (!value) return false;
  const lower = value.toLowerCase();
  return (
    lower.startsWith("http://") ||
    lower.startsWith("https://") ||
    lower.startsWith("mailto:") ||
    lower.startsWith("amitia://")
  );
}

export function isSafeMediaUrl(url?: string): boolean {
  const value = String(url ?? "").trim();
  if (!value) return false;
  const lower = value.toLowerCase();
  return (
    value.startsWith("/") ||
    lower.startsWith("http://") ||
    lower.startsWith("https://") ||
    lower.startsWith("blob:") ||
    lower.startsWith("data:image/") ||
    lower.startsWith("data:audio/") ||
    lower.startsWith("data:video/")
  );
}

export function openSafeLink(url?: string): void {
  if (!isSafeLink(url)) return;
  window.open(String(url), "_blank", "noopener,noreferrer");
}

export function prettyJson(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function normalizedLanguage(value?: string): string {
  const language = String(value ?? "").trim().toLowerCase();
  return language || "text";
}

