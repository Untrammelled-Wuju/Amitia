export type AppSectionId =
  | "home"
  | "character"
  | "memory"
  | "tasks"
  | "continuity"
  | "game"
  | "workshop"
  | "extensions"
  | "channels"
  | "account"
  | "settings";

export interface AppSectionEntry {
  route: string;
  label: string;
}

export interface AppSectionDefinition {
  id: Exclude<AppSectionId, "home" | "channels" | "account" | "settings">;
  route: string;
  label: string;
  entries: AppSectionEntry[];
}

export const appSections: AppSectionDefinition[] = [
  {
    id: "character",
    route: "/characters",
    label: "角色",
    entries: [],
  },
  {
    id: "memory",
    route: "/memory",
    label: "记忆",
    entries: [],
  },
  {
    id: "tasks",
    route: "/reminders",
    label: "任务",
    entries: [],
  },
  {
    id: "workshop",
    route: "/creative-workshop",
    label: "创意工坊",
    entries: [],
  },
  {
    id: "extensions",
    route: "/extensions",
    label: "扩展中心",
    entries: [],
  },
];

export function resolveAppSection(target: string): AppSectionId | null {
  const path = target.split(/[?#]/)[0];
  if (path === "/chat" || path === "/logs") return "home";
  if (path === "/settings" || path.startsWith("/settings/")) return "settings";
  if (path === "/my-space" || path.startsWith("/my-space/")) return "account";
  if (path === "/channel-center" || path.startsWith("/channels/")) return "channels";
  if (path === "/reminders" || path === "/continuity" || /^\/characters\/[^/]+\/active-task$/.test(path)) return "tasks";
  if (path === "/game-center" || path.startsWith("/game-center/")) return "game";
  if (path === "/characters" || path === "/character" || path.startsWith("/character/") || path === "/characters/teams" || path === "/world-book") return "character";
  if (path === "/memory" || path.startsWith("/memory/") || path === "/memory-manager" || path === "/memory-timeline" || path === "/knowledge" || path === "/episodic" || path === "/graph" || path === "/profiles") return "memory";
  if (path === "/creative-workshop" || path.startsWith("/creative-workshop/")) return "workshop";
  if (path === "/extensions" || path.startsWith("/extensions/") || path === "/kernel" || path.startsWith("/kernel/") || path.startsWith("/extension/page/")) return "extensions";
  return null;
}

export function appSection(id: AppSectionId): AppSectionDefinition | undefined {
  return appSections.find(section => section.id === id);
}
