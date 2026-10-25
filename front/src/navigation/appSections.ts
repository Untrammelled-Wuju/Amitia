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
    entries: [
      { route: "/character", label: "角色管理" },
      { route: "/profiles", label: "用户画像" },
      { route: "/world-book", label: "世界书" },
    ],
  },
  {
    id: "memory",
    route: "/memory",
    label: "记忆",
    entries: [
      { route: "/knowledge", label: "知识库" },
      { route: "/memory-manager", label: "记忆总览" },
      { route: "/episodic", label: "情景记忆" },
      { route: "/graph", label: "记忆图谱" },
      { route: "/memory-timeline", label: "时间线" },
    ],
  },
  {
    id: "workshop",
    route: "/creative-workshop",
    label: "创意工坊",
    entries: [
      { route: "/creative-workshop/character-cards", label: "角色卡工坊" },
      { route: "/creative-workshop/pet/installations", label: "桌宠" },
    ],
  },
  {
    id: "extensions",
    route: "/extensions",
    label: "扩展中心",
    entries: [
      { route: "/extensions/packages", label: "扩展包" },
      { route: "/extensions/workflows", label: "工作流" },
      { route: "/extensions/mcp", label: "MCP 服务" },
      { route: "/extensions/agent-skills", label: "Agent Skills" },
    ],
  },
];

export function resolveAppSection(target: string): AppSectionId | null {
  const path = target.split(/[?#]/)[0];
  if (path === "/chat" || path === "/logs") return "home";
  if (path === "/settings" || path.startsWith("/settings/")) return "settings";
  if (path === "/my-space" || path.startsWith("/my-space/")) return "account";
  if (path === "/channel-center" || path.startsWith("/channels/")) return "channels";
  if (path === "/reminders" || /^\/characters\/[^/]+\/active-task$/.test(path)) return "tasks";
  if (path === "/continuity") return "continuity";
  if (path === "/game-center" || path.startsWith("/game-center/")) return "game";
  if (path === "/characters" || path === "/character" || path.startsWith("/character/") || path === "/profiles" || path === "/world-book") return "character";
  if (path === "/memory" || path.startsWith("/memory/") || path === "/memory-manager" || path === "/memory-timeline" || path === "/knowledge" || path === "/episodic" || path === "/graph") return "memory";
  if (path === "/creative-workshop" || path.startsWith("/creative-workshop/")) return "workshop";
  if (path === "/extensions" || path.startsWith("/extensions/") || path === "/kernel" || path.startsWith("/kernel/") || path.startsWith("/extension/page/")) return "extensions";
  return null;
}

export function appSection(id: AppSectionId): AppSectionDefinition | undefined {
  return appSections.find(section => section.id === id);
}
