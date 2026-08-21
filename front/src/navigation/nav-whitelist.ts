export const NAVIGATION_WHITELIST: readonly string[] = [
  "/chat",
  "/dashboard",
  "/creative-workshop",
  "/character",
  "/settings",
  "/kernel",
  "/extensions",
  "/extension/page/",
  "/emotes",
  "/graph",
  "/logs",
  "/import",
  "/reminders",
  "/runtime-mode",
  "/storage",
  "/profiles",
  "/user-settings",
  "/episodic",
  "/world-book",
  "/decision-viz",
  "/memory-manager",
  "/memory-timeline",
  "/privacy-scan",
  "/devices",
  "/game-center",
  "/qq",
  "/wechat",
  "/runtime-debug",
  "/workspaces",
  "/asr",
  "/realtime-voice",
  "/long-running",
];

const NAVIGATION_BLACKLIST: readonly string[] = [
  "/login",
  "/setup",
  "/onboarding",
  "/privacy",
  "/usage-boundary",
  "/404",
];

export function isNavigationAllowed(target: string): boolean {
  if (!target || typeof target !== "string") return false;
  const path = target.split("?")[0].split("#")[0];
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
