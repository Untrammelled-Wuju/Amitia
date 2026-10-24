import { isRuntimeRouteAvailable } from "@/runtime/runtime-capabilities";

export const NAVIGATION_WHITELIST: readonly string[] = [
  "/chat",
  "/dashboard",
  "/creative-workshop",
  "/character",
  "/characters",
  "/settings",
  "/kernel",
  "/extensions",
  "/extension/page/",
  "/emotes",
  "/graph",
  "/continuity",
  "/channel-center",
  "/channels",
  "/logs",
  "/import",
  "/reminders",
  "/storage",
  "/profiles",
  "/my-space",
  "/user-settings",
  "/episodic",
  "/world-book",
  "/knowledge",
  "/decision-viz",
  "/memory-manager",
  "/memory-timeline",
  "/memory",
  "/privacy-scan",
  "/devices",
  "/game-center",
  "/runtime-debug",
  "/workspaces",
  "/long-running",
];

const NAVIGATION_BLACKLIST: readonly string[] = [
  "/onboarding",
  "/privacy",
  "/usage-boundary",
  "/404",
];

export function isNavigationAllowed(target: string): boolean {
  if (!target || typeof target !== "string") return false;
  const path = target.split("?")[0].split("#")[0];
  if (!isRuntimeRouteAvailable(path)) return false;
  for (const blocked of NAVIGATION_BLACKLIST) {
    if (path === blocked || path.startsWith(blocked + "/")) return false;
  }
  for (const allowed of NAVIGATION_WHITELIST) {
    if (path === allowed || path.startsWith(allowed + "/") || path.startsWith(allowed)) {
      return true;
    }
  }
  return false;
}
