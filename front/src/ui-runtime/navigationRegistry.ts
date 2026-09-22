import { computed, type Component } from "vue";
import {
  Avatar,
  Calendar,
  ChatLineRound,
  Collection,
  Connection,
  DataAnalysis,
  DataLine,
  Film,
  Grid,
  List,
  MagicStick,
  Menu,
  Odometer,
  Share,
  StarFilled,
  Timer,
  User,
  UserFilled,
} from "@element-plus/icons-vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { isRuntimeCapabilityAvailable, isRuntimeRouteAvailable, type RuntimeCapabilityName } from "@/runtime/runtime-capabilities";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import { collectRegistryProviders } from "./providerCollection";
import { effectiveExtensionRouteKeys, effectiveProviderRouteKeys, shadowsProtectedRoute } from "./providerRoutes";
import { resolveChannelPresentations } from "./channelPresentation";

export interface UINavigationItem {
  id: string;
  route: string;
  label: string;
  icon: Component;
  order: number;
  group: string;
  groupLabel?: string;
  groupIcon?: Component;
  match?: string[];
  mobile?: boolean;
  extensionId?: string;
  runtimeCapability?: RuntimeCapabilityName;
}

export interface UINavigationGroup {
  id: string;
  label?: string;
  icon?: Component;
  order: number;
  items: UINavigationItem[];
}

const iconMap: Record<string, Component> = {
  chat: ChatLineRound,
  message: ChatLineRound,
  dashboard: Odometer,
  data: DataAnalysis,
  run: DataLine,
  character: UserFilled,
  user: User,
  calendar: Calendar,
  profile: Avatar,
  collection: Collection,
  connection: Connection,
  star: StarFilled,
  memory: Grid,
  list: List,
  film: Film,
  graph: Share,
  timeline: Timer,
  workshop: MagicStick,
  extension: Menu,
};

export function resolveNavigationIcon(name?: string): Component {
  return iconMap[String(name || "extension").toLowerCase()] ?? Menu;
}

const builtinItems: UINavigationItem[] = [
  { id: "character.manage", route: "/character", label: "角色管理", icon: User, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 20, mobile: true, match: ["/character"] },
  { id: "character.reminders", route: "/reminders", label: "日程提醒", icon: Calendar, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 25 },
  { id: "character.profiles", route: "/profiles", label: "用户画像", icon: Avatar, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 30 },
  { id: "character.world-book", route: "/world-book", label: "世界书", icon: Collection, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 35 },
  { id: "memory.manager", route: "/memory-manager", label: "记忆总览", icon: List, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 55, mobile: true, match: ["/memory", "/memory-manager"] },
  { id: "memory.episodic", route: "/episodic", label: "情景记忆", icon: Film, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 60 },
  { id: "memory.graph", route: "/graph", label: "记忆图谱", icon: Share, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 65 },
  { id: "memory.timeline", route: "/memory-timeline", label: "时间线", icon: Timer, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 70 },
  { id: "continuity.threads", route: "/continuity", label: "持续事项", icon: List, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 72, mobile: true },
  { id: "workshop.game-center", route: "/game-center", label: "游戏模式", icon: MagicStick, group: "workshop", order: 80, runtimeCapability: "gameMode" },
  { id: "workshop", route: "/creative-workshop", label: "创意工坊", icon: MagicStick, group: "workshop", order: 85, match: ["/creative-workshop"] },
  { id: "extensions", route: "/extensions", label: "扩展中心", icon: Menu, group: "extensions", order: 90, match: ["/extensions", "/kernel"] },
];

const channelCenterItem: UINavigationItem = {
  id: "channels.messages",
  route: "/channel-messages",
  label: "渠道消息",
  icon: Connection,
  group: "channels",
  groupLabel: "渠道",
  groupIcon: Connection,
  order: 78,
};

const pageTitleOverrides: Record<string, string> = {
  "/onboarding": "引导",
  "/privacy": "隐私说明",
  "/usage-boundary": "使用边界",
  "/storage": "存储清理",
  "/runtime-mode": "运行模式",
  "/runtime-debug": "运行时调试",
  "/chat": "新对话",
  "/settings/overview": "运行概览",
  "/settings/data": "运行数据",
  "/channel-messages": "渠道消息",
  "/continuity": "持续事项",
  "/user-settings": "用户信息",
  "/creative-workshop/character-cards": "角色卡工坊",
  "/creative-workshop/pet": "桌宠",
  "/emotes": "表情包管理",
  "/extensions/mcp": "MCP 服务",
  "/extensions/packages": "扩展包",
  "/extensions/workflows": "工作流",
  "/kernel": "扩展包",
  "/kernel/trusted-services": "可信服务运行时",
  "/kernel/wasm": "WASM 运行时",
  "/kernel/hooks": "Hook 中心",
  "/kernel/tasks": "任务运行时",
  "/kernel/events": "事件中心",
  "/kernel/schedules": "调度中心",
  "/kernel/desktop": "桌面贡献中心",
  "/kernel/updates": "扩展更新中心",
  "/kernel/dev-console": "开发者诊断控制台",
  "/kernel/migrations": "迁移与灰度中心",
  "/kernel/dev-mode": "开发模式中心",
  "/devices": "我的设备",
  "/workspaces": "工作区",
  "/realtime-voice": "实时语音",
  "/long-running": "长期运行维护",
  "/settings/system-logs": "系统运行日志",
  "/settings/prompt-trace": "Prompt Trace",
  "/logs": "归档对话",
};

function finiteNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function absolutePrefixes(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) return undefined;
  const prefixes = value
    .map((item) => String(item).trim())
    .filter((item) => item.startsWith("/") && item !== "/");
  return prefixes.length > 0 ? prefixes : undefined;
}

function activeNavigationProviders(store: ReturnType<typeof useExtensionUIStore>): UIProviderDefinition[] {
  const providers = [
    ...collectRegistryProviders(store, "app.navigation"),
    ...collectRegistryProviders(store, "route.registry"),
  ];
  const seen = new Set<string>();
  return providers
    .filter((provider) => {
      if (seen.has(provider.providerId)) return false;
      seen.add(provider.providerId);
      return true;
    })
    .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0) || a.providerId.localeCompare(b.providerId));
}

/**
 * Resolve additive navigation contributions from every enabled compatible
 * provider. route.registry navigation items are shown only when their route won
 * conflict resolution and is still backed by a live provider from the same extension.
 */
export function collectExtensionNavigationItems(
  store: ReturnType<typeof useExtensionUIStore>,
): UINavigationItem[] {
  const result: UINavigationItem[] = [];
  const seen = new Set<string>();
  const effectiveRouteKeys = effectiveProviderRouteKeys(store);
  const effectiveExtensionRoutes = effectiveExtensionRouteKeys(store);
  for (const provider of activeNavigationProviders(store)) {
    const raw = provider.metadata?.navigationItems;
    if (!Array.isArray(raw)) continue;
    for (const value of raw) {
      if (!value || typeof value !== "object") continue;
      const row = value as Record<string, unknown>;
      const id = String(row.id ?? "").trim();
      const route = String(row.route ?? "").trim();
      const label = String(row.label ?? "").trim();
      const key = `${provider.extensionId}:${id}`;
      if (!id || !route.startsWith("/") || route === "/" || !label) continue;
      if (!isRuntimeRouteAvailable(route)) continue;
      if (provider.capability === "route.registry") {
        if (!effectiveRouteKeys.has(`${provider.providerId}\u0000${route}`)) continue;
      } else if (
        !shadowsProtectedRoute(route) &&
        !effectiveExtensionRoutes.has(`${provider.extensionId}\u0000${route}`)
      ) {
        continue;
      }
      if (!seen.add(key)) continue;
      const group = String(row.group ?? `extension:${provider.extensionId}`).trim() || `extension:${provider.extensionId}`;
      result.push({
        id: key,
        route,
        label,
        icon: resolveNavigationIcon(String(row.icon ?? "extension")),
        order: finiteNumber(row.order, 1000),
        group,
        groupLabel: row.groupLabel ? String(row.groupLabel).trim() : "扩展",
        groupIcon: resolveNavigationIcon(String(row.groupIcon ?? row.icon ?? "extension")),
        match: absolutePrefixes(row.match ?? row.routePrefixes),
        mobile: row.mobile === true || String(row.panel ?? "").trim() === "main",
        extensionId: provider.extensionId,
      });
    }
  }
  return result;
}

function applyIconProvider(store: ReturnType<typeof useExtensionUIStore>, items: UINavigationItem[]): UINavigationItem[] {
  const provider = store.getResolvedProvider("ui.icons");
  if (!provider || !provider.enabled || provider.builtin) return items;
  const aliases = provider.metadata?.iconAliases;
  if (!aliases || typeof aliases !== "object" || Array.isArray(aliases)) return items;
  const map = aliases as Record<string, unknown>;
  return items.map((item) => {
    const name = map[item.id] ?? map[item.group] ?? map.default;
    return name ? { ...item, icon: resolveNavigationIcon(String(name)) } : item;
  });
}

export function resolveBuiltinNavigationItems(store: ReturnType<typeof useExtensionUIStore>): UINavigationItem[] {
  const available = builtinItems.filter(
    (item) => !item.runtimeCapability || isRuntimeCapabilityAvailable(item.runtimeCapability),
  );
  return resolveChannelPresentations(store).length > 0
    ? [...available, channelCenterItem]
    : available;
}

export function useUINavigationRegistry() {
  const store = useExtensionUIStore();
  const items = computed<UINavigationItem[]>(() => {
    const merged = applyIconProvider(store, [...resolveBuiltinNavigationItems(store), ...collectExtensionNavigationItems(store)]);
    return merged.sort((a, b) => a.order - b.order || a.id.localeCompare(b.id));
  });
  const groups = computed<UINavigationGroup[]>(() => {
    const map = new Map<string, UINavigationGroup>();
    let groupOrder = 0;
    for (const item of items.value) {
      const current = map.get(item.group) ?? {
        id: item.group,
        label: item.groupLabel,
        icon: item.groupIcon,
        order: groupOrder++,
        items: [],
      };
      current.items.push(item);
      map.set(item.group, current);
    }
    return [...map.values()].sort((a, b) => a.order - b.order);
  });
  const mobileItems = computed(() => items.value.filter((item) => item.mobile));
  return { items, groups, mobileItems };
}

export function isUINavigationItemActive(path: string, item: UINavigationItem): boolean {
  if (path === item.route || path.startsWith(`${item.route}/`)) return true;
  return (item.match ?? []).some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}

export function resolvePageTitle(path: string, items: UINavigationItem[] = builtinItems): string {
  const override = pageTitleOverrides[path];
  if (override) return override;
  const item = items.find((entry) => isUINavigationItemActive(path, entry));
  return item?.label ?? "AI-Amitia";
}
