import { computed, type Component } from "vue";
import {
  Avatar,
  Calendar,
  ChatLineRound,
  Collection,
  DataAnalysis,
  DataLine,
  Extensions,
  Film,
  List,
  MagicStick,
  Odometer,
  Share,
  StarFilled,
  Timer,
} from "@element-plus/icons-vue";
import BrainIcon from "@/components/icons/BrainIcon.vue";
import ChannelIcon from "@/components/icons/ChannelIcon.vue";
import CharacterIcon from "@/components/icons/CharacterIcon.vue";
import ContinuityIcon from "@/components/icons/ContinuityIcon.vue";
import GamepadIcon from "@/components/icons/GamepadIcon.vue";
import HomeIcon from "@/components/icons/HomeIcon.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { isRuntimeCapabilityAvailable, isRuntimeRouteAvailable, type RuntimeCapabilityName } from "@/runtime/runtime-capabilities";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import { collectRegistryProviders } from "./providerCollection";
import {
  collectEffectiveProviderRoutes,
  effectiveExtensionRouteKeys,
  effectiveProviderRouteKeys,
  shadowsProtectedRoute,
} from "./providerRoutes";
import { resolveChannelPresentations } from "./channelPresentation";

export interface UINavigationItem {
  id: string;
  route: string;
  label: string;
  placement: "primary" | "settings" | "profile" | "workspace" | "contextual";
  description?: string;
  icon: Component;
  order: number;
  group: string;
  groupLabel?: string;
  groupIcon?: Component;
  match?: string[];
  mobile?: boolean;
  profile?: boolean;
  tabLabel?: string;
  extensionId?: string;
  runtimeCapability?: RuntimeCapabilityName;
  containerId?: string;
  sidebarProviderId?: string;
  sidebarItems?: UINavigationSidebarItem[];
}

export interface UINavigationSidebarItem {
  label: string;
  route: string;
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
  character: CharacterIcon,
  user: CharacterIcon,
  calendar: Calendar,
  profile: Avatar,
  collection: Collection,
  connection: ChannelIcon,
  channel: ChannelIcon,
  star: StarFilled,
  memory: BrainIcon,
  continuity: ContinuityIcon,
  list: List,
  film: Film,
  graph: Share,
  timeline: Timer,
  workshop: MagicStick,
  extension: Extensions,
};

export function resolveNavigationIcon(name?: string): Component {
  return iconMap[String(name || "extension").toLowerCase()] ?? Extensions;
}

const coreNavigationItems: UINavigationItem[] = [
  { id: "home", route: "/chat", label: "主页", placement: "primary", description: "开始新对话，或继续最近的聊天。", icon: HomeIcon, group: "home", order: 0, mobile: true, match: ["/chat"] },
  { id: "character", route: "/characters", label: "角色", placement: "primary", description: "管理角色、团队、世界书和角色能力。", icon: CharacterIcon, group: "character", order: 20, profile: true, match: ["/character", "/characters", "/world-book"] },
  { id: "memory", route: "/memory", label: "记忆", placement: "primary", description: "管理记忆、知识库、图谱和时间线。", icon: BrainIcon, group: "memory", order: 50, mobile: true, profile: true, tabLabel: "记忆", match: ["/memory", "/knowledge", "/episodic", "/graph", "/profiles"] },
  { id: "tasks", route: "/tasks", label: "任务", placement: "primary", description: "管理主动任务、定时提醒和持续事项。", icon: Calendar, group: "tasks", order: 70, mobile: true, profile: true, tabLabel: "任务", match: ["/tasks", "/reminders", "/continuity"] },
  { id: "game-center", route: "/game-center", label: "游戏模式", placement: "primary", description: "管理游戏插件、运行环境和游戏内容。", icon: GamepadIcon, group: "game", order: 78, match: ["/game-center"] },
  { id: "creative-workshop", route: "/creative-workshop", label: "创意工坊", placement: "primary", description: "管理角色卡、工作流和桌宠内容。", icon: MagicStick, group: "workshop", order: 85, match: ["/creative-workshop", "/workshop"] },
];

const corePageTitles: Record<string, string> = {
  "/onboarding": "引导",
  "/privacy": "隐私说明",
  "/usage-boundary": "使用边界",
  "/chat": "新对话",
  "/attention": "待处理",
  "/settings/ui-providers": "界面提供者修复",
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

function resolveNavigationPlacement(row: Record<string, unknown>, route: string, group: string): UINavigationItem["placement"] {
  const explicit = String(row.placement ?? row.navigationPlacement ?? "").trim().toLowerCase();
  if (explicit === "primary" || explicit === "settings" || explicit === "profile" || explicit === "workspace" || explicit === "contextual") {
    return explicit;
  }
  if (group === "settings") return "settings";
  if (route === "/settings" || route.startsWith("/settings/") || route === "/logs" || route.startsWith("/logs/")) return "profile";
  if (group === "workspace" || route === "/workspaces" || route.startsWith("/workspaces/")) return "workspace";
  if (
    row.profile === true ||
    row.profileHub === true ||
    row.mobile === true ||
    String(row.panel ?? "").trim() === "main" ||
    String(row.tabLabel ?? "").trim()
  ) return "primary";
  return "contextual";
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

function hasEffectiveRoute(effectiveRoutes: Set<string>, ownerId: string, route: string): boolean {
  const exact = `${ownerId}\u0000${route}`;
  if (effectiveRoutes.has(exact)) return true;
  const childPrefix = `${exact}/`;
  return [...effectiveRoutes].some((key) => key.startsWith(childPrefix));
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
        if (!hasEffectiveRoute(effectiveRouteKeys, provider.providerId, route)) continue;
      } else if (
        !shadowsProtectedRoute(route) &&
        !hasEffectiveRoute(effectiveExtensionRoutes, provider.extensionId, route)
      ) {
        continue;
      }
      if (!seen.add(key)) continue;
      const group = String(row.group ?? `extension:${provider.extensionId}`).trim() || `extension:${provider.extensionId}`;
      result.push({
        id: key,
        route,
        label,
        placement: resolveNavigationPlacement(row, route, group),
        description: String(row.description ?? row.summary ?? "").trim(),
        icon: resolveNavigationIcon(String(row.icon ?? "extension")),
        order: finiteNumber(row.order, 1000),
        group,
        groupLabel: row.groupLabel ? String(row.groupLabel).trim() : "扩展",
        groupIcon: resolveNavigationIcon(String(row.groupIcon ?? row.icon ?? "extension")),
        match: absolutePrefixes(row.match ?? row.routePrefixes),
        mobile: row.mobile === true || String(row.panel ?? "").trim() === "main",
        profile: row.profile === true || row.profileHub === true,
        tabLabel: String(row.tabLabel ?? "").trim() || undefined,
        extensionId: provider.extensionId,
        containerId: String(row.containerId ?? "").trim(),
        sidebarProviderId: String(row.sidebarProviderId ?? "").trim(),
        sidebarItems: Array.isArray(row.sidebarItems)
          ? row.sidebarItems
            .filter((item): item is Record<string, unknown> => Boolean(item && typeof item === "object" && !Array.isArray(item)))
            .map((item) => ({ label: String(item.label ?? "").trim(), route: String(item.route ?? "").trim() }))
            .filter((item) => item.label && item.route.startsWith("/"))
          : undefined,
      });
    }
  }
  return result;
}

export interface ChannelPluginNavigationItem {
  id: string;
  route: string;
  label: string;
  channelId: string;
  extensionId: string;
}

export function resolveChannelPluginNavigationItems(
  store: ReturnType<typeof useExtensionUIStore>,
  extensionItems: UINavigationItem[],
): ChannelPluginNavigationItem[] {
  const items: ChannelPluginNavigationItem[] = [];
  const providerRoutes = collectEffectiveProviderRoutes(store);
  for (const presentation of resolveChannelPresentations(store)) {
    const routeItem = extensionItems.find((item) =>
      item.extensionId === presentation.extensionId && item.route.startsWith("/channels/"),
    );
    const providerRoute = providerRoutes.find((item) =>
      item.extensionId === presentation.extensionId && item.path.startsWith("/channels/"),
    );
    const route = routeItem?.route || providerRoute?.path;
    if (!route) continue;
    items.push({
      id: `channel:${presentation.extensionId}:${presentation.channelId}`,
      route,
      label: presentation.displayName || routeItem?.label || providerRoute?.title || presentation.channelId,
      channelId: presentation.channelId,
      extensionId: presentation.extensionId,
    });
  }
  return items;
}

export function collectChannelPluginNavigationItems(
  store: ReturnType<typeof useExtensionUIStore>,
): ChannelPluginNavigationItem[] {
  return resolveChannelPluginNavigationItems(store, collectExtensionNavigationItems(store));
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
  return coreNavigationItems.filter(
    (item) => !item.runtimeCapability || isRuntimeCapabilityAvailable(item.runtimeCapability),
  );
}

export function resolveMobileNavigationItems(items: UINavigationItem[]): UINavigationItem[] {
  return items.filter(
    (item) => item.mobile && (item.placement === "primary" || item.placement === "profile"),
  );
}

function matchesRoutePattern(path: string, pattern: string): boolean {
  if (path === pattern || pattern === "*" || pattern === "/*") return true;
  if (pattern.endsWith("/*")) {
    const prefix = pattern.slice(0, -2);
    return path === prefix || path.startsWith(`${prefix}/`);
  }
  const pathParts = path.split("/").filter(Boolean);
  const patternParts = pattern.split("/").filter(Boolean);
  if (pathParts.length !== patternParts.length) return false;
  return patternParts.every((part, index) => part.startsWith(":") || part === "*" || part === pathParts[index]);
}

export function resolveProviderRouteTitle(
  store: ReturnType<typeof useExtensionUIStore>,
  path: string,
): string | null {
  const matches = collectEffectiveProviderRoutes(store)
    .filter((route) => route.title && matchesRoutePattern(path, route.path))
    .sort((a, b) => b.path.length - a.path.length || b.priority - a.priority);
  return matches[0]?.title?.trim() || null;
}

export function useUINavigationRegistry() {
  const store = useExtensionUIStore();
  const items = computed<UINavigationItem[]>(() => {
    const extensionItems = collectExtensionNavigationItems(store);
    const channelPluginRoutes = new Set(
      resolveChannelPluginNavigationItems(store, extensionItems).map((item) => item.route),
    );
    const visibleExtensionItems = extensionItems.filter((item) =>
      !item.route.startsWith("/channels/") && !channelPluginRoutes.has(item.route),
    );
    const merged = applyIconProvider(store, [...resolveBuiltinNavigationItems(store), ...visibleExtensionItems]);
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
  const primaryItems = computed(() => items.value.filter((item) => item.placement === "primary"));
  const settingsItems = computed(() => items.value.filter((item) => item.placement === "settings"));
  const mobileItems = computed(() => resolveMobileNavigationItems(items.value));
  const profileItems = computed(() => items.value.filter((item) => item.profile));
  const titleForRoute = (path: string) =>
    resolveProviderRouteTitle(store, path) || resolvePageTitle(path, items.value);
  return { items, primaryItems, settingsItems, groups, mobileItems, profileItems, titleForRoute };
}

export function isUINavigationItemActive(path: string, item: UINavigationItem): boolean {
  if (path === item.route || path.startsWith(`${item.route}/`)) return true;
  return (item.match ?? []).some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}

export function resolvePageTitle(path: string, items: UINavigationItem[] = coreNavigationItems): string {
  const override = corePageTitles[path];
  if (override) return override;
  const item = items.find((entry) => isUINavigationItemActive(path, entry));
  return item?.label ?? "AI-Amitia";
}
