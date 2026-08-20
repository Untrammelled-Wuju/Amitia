import type { UIContributionSnapshot } from "@/stores/extensionUI";

const PREFIX = "amitia.ui.snapshot.lkg.v1";

function key(platform: string, deviceId: string, backendNamespace?: string): string {
  const origin = backendNamespace || (typeof window === "undefined" ? "server" : window.location.origin);
  return `${PREFIX}:${encodeURIComponent(origin)}:${platform}:${deviceId || "anonymous"}`;
}

export function saveLastKnownGoodSnapshot(platform: string, deviceId: string, snapshot: UIContributionSnapshot, backendNamespace?: string): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(key(platform, deviceId, backendNamespace), JSON.stringify({ savedAt: Date.now(), snapshot }));
  } catch {
    // Cache failure must never make the UI runtime unavailable.
  }
}

export function loadLastKnownGoodSnapshot(platform: string, deviceId: string, backendNamespace?: string): UIContributionSnapshot | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(key(platform, deviceId, backendNamespace));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as { snapshot?: UIContributionSnapshot };
    return parsed.snapshot ?? null;
  } catch {
    return null;
  }
}
