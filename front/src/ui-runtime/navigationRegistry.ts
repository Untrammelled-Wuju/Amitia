import { computed, type Component } from "vue";
import {
  Avatar,
  Calendar,
  ChatDotSquare,
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
import type { UIProviderDefinition } from "@/ui-runtime/types";

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
  qq: ChatDotSquare,
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
  { id: "overview.run", route: "/dashboard/run", label: "运行概览", icon: DataLine, group: "overview", groupLabel: "概览", groupIcon: Odometer, order: 10 },
  { id: "overview.data", route: "/dashboard/data", label: "运行数据", icon: DataAnalysis, group: "overview", groupLabel: "概览", groupIcon: Odometer, order: 20 },
  { id: "chat", route: "/chat", label: "聊天", icon: ChatLineRound, group: "chat", order: 30, mobile: true },
  { id: "character.manage", route: "/character", label: "角色管理", icon: User, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 10, mobile: true, match: ["/character"] },
  { id: "character.reminders", route: "/reminders", label: "日程提醒", icon: Calendar, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 20 },
  { id: "character.profiles", route: "/profiles", label: "用户画像", icon: Avatar, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 30 },
  { id: "character.world-book", route: "/world-book", label: "世界书", icon: Collection, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 40 },
  { id: "character.wechat", route: "/wechat", label: "微信连接", icon: Connection, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 50 },
  { id: "character.qq", route: "/qq", label: "QQ 连接", icon: ChatDotSquare, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 60 },
  { id: "character.emotes", route: "/emotes", label: "表情包管理", icon: StarFilled, group: "character", groupLabel: "角色", groupIcon: UserFilled, order: 70 },
  { id: "memory.manager", route: "/memory-manager", label: "记忆总览", icon: List, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 10, mobile: true, match: ["/memory", "/memory-manager"] },
  { id: "memory.episodic", route: "/episodic", label: "情景记忆", icon: Film, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 20 },
  { id: "memory.graph", route: "/graph", label: "记忆图谱", icon: Share, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 30 },
  { id: "memory.timeline", route: "/memory-timeline", label: "时间线", icon: Timer, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 40 },
  { id: "memory.logs", route: "/logs", label: "聊天记录", icon: ChatLineRound, group: "memory", groupLabel: "记忆", groupIcon: Grid, order: 50 },
  { id: "workshop", route: "/creative-workshop", label: "创意工坊", icon: MagicStick, group: "workshop", order: 60, match: ["/creative-workshop"] },
  { id: "extensions", route: "/extensions", label: "扩展中心", icon: Menu, group: "extensions", order: 70, match: ["/extensions", "/kernel"] },
  { id: "tools.game-center", route: "/game-center", label: "游戏中心", icon: MagicStick, group: "tools", groupLabel: "工具", groupIcon: Menu, order: 5 },
  { id: "tools.asr", route: "/asr", label: "语音识别", icon: ChatLineRound, group: "tools", groupLabel: "工具", groupIcon: Menu, order: 10 },
  { id: "tools.realtime-voice", route: "/realtime-voice", label: "实时语音", icon: Connection, group: "tools", groupLabel: "工具", groupIcon: Menu, order: 20 },
  { id: "tools.long-running", route: "/long-running", label: "长期运行维护", icon: Timer, group: "tools", groupLabel: "工具", groupIcon: Menu, order: 30 },
];

function activeNavigationProviders(store: ReturnType<typeof useExtensionUIStore>) {
  const providers = [
    store.getResolvedProvider("app.navigation"),
    store.getResolvedProvider("route.registry"),
  ];
  const seen = new Set<string>();
  return providers.filter((provider): provider is UIProviderDefinition => {
    if (!provider || !provider.enabled || provider.builtin || seen.has(provider.providerId)) return false;
    seen.add(provider.providerId);
    return true;
  });
}

function readExtensionItems(store: ReturnType<typeof useExtensionUIStore>): UINavigationItem[] {
  const result: UINavigationItem[] = [];
  const seen = new Set<string>();
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
      if (!id || !route.startsWith("/") || !label || !seen.add(key)) continue;
      result.push({
        id: key,
        route,
        label,
        icon: resolveNavigationIcon(String(row.icon ?? "extension")),
        order: Number(row.order ?? 1000),
        group: String(row.group ?? `extension:${provider.extensionId}`),
        groupLabel: row.groupLabel ? String(row.groupLabel) : "扩展",
        groupIcon: resolveNavigationIcon(String(row.groupIcon ?? row.icon ?? "extension")),
        match: Array.isArray(row.match) ? row.match.map(String) : undefined,
        mobile: row.mobile === true,
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

export function useUINavigationRegistry() {
  const store = useExtensionUIStore();
  const items = computed<UINavigationItem[]>(() => {
    const merged = applyIconProvider(store, [...builtinItems, ...readExtensionItems(store)]);
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
