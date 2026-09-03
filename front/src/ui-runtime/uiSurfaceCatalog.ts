type RouteAliasRule = {
  canonicalPrefix: string;
  aliasPrefix: string;
  exact?: boolean;
};

const ROUTE_ALIAS_RULES: RouteAliasRule[] = [
  { canonicalPrefix: "/characters", aliasPrefix: "/character" },
  { canonicalPrefix: "/workshop", aliasPrefix: "/creative-workshop" },
  { canonicalPrefix: "/channels/wechat", aliasPrefix: "/wechat", exact: true },
  { canonicalPrefix: "/channels/qq", aliasPrefix: "/qq", exact: true },
  { canonicalPrefix: "/settings/devices", aliasPrefix: "/devices" },
  { canonicalPrefix: "/developer/kernel", aliasPrefix: "/kernel" },
  { canonicalPrefix: "/memory/manager", aliasPrefix: "/memory-manager", exact: true },
  { canonicalPrefix: "/memory/timeline", aliasPrefix: "/memory-timeline", exact: true },
  { canonicalPrefix: "/memory/episodic", aliasPrefix: "/episodic", exact: true },
  { canonicalPrefix: "/memory/graph", aliasPrefix: "/graph", exact: true },
  { canonicalPrefix: "/memory/profiles", aliasPrefix: "/profiles", exact: true },
  { canonicalPrefix: "/memory/world-book", aliasPrefix: "/world-book", exact: true },
  { canonicalPrefix: "/chat-logs", aliasPrefix: "/logs", exact: true },
  { canonicalPrefix: "/chat-import", aliasPrefix: "/import", exact: true },
  { canonicalPrefix: "/settings/runtime-mode", aliasPrefix: "/runtime-mode", exact: true },
  { canonicalPrefix: "/settings/long-running", aliasPrefix: "/long-running", exact: true },
  { canonicalPrefix: "/settings/decision-viz", aliasPrefix: "/decision-viz", exact: true },
  { canonicalPrefix: "/settings/privacy-scan", aliasPrefix: "/privacy-scan", exact: true },
  { canonicalPrefix: "/settings/storage", aliasPrefix: "/storage", exact: true },
  { canonicalPrefix: "/settings/user", aliasPrefix: "/user-settings", exact: true },
];

export function normalizeUIRoute(raw: string): string {
  const route = (raw || "/").split(/[?#]/, 1)[0].trim() || "/";
  const prefixed = route.startsWith("/") ? route : `/${route}`;
  return prefixed.length > 1 ? prefixed.replace(/\/+$/, "") : prefixed;
}

function matchesPrefix(route: string, prefix: string, exact = false): boolean {
  return exact ? route === prefix : route === prefix || route.startsWith(`${prefix}/`);
}

function swapPrefix(route: string, from: string, to: string, exact = false): string {
  return exact ? to : `${to}${route.slice(from.length)}`;
}

export function uiRouteAliases(rawRoute: string): string[] {
  const route = normalizeUIRoute(rawRoute);
  const aliases = new Set<string>([route]);
  for (const rule of ROUTE_ALIAS_RULES) {
    if (matchesPrefix(route, rule.canonicalPrefix, rule.exact)) {
      aliases.add(swapPrefix(route, rule.canonicalPrefix, rule.aliasPrefix, rule.exact));
    }
    if (matchesPrefix(route, rule.aliasPrefix, rule.exact)) {
      aliases.add(swapPrefix(route, rule.aliasPrefix, rule.canonicalPrefix, rule.exact));
    }
  }
  return [...aliases];
}

export function canonicalUISurfaceId(rawRoute: string): string {
  const aliases = uiRouteAliases(rawRoute);
  const hasPrefix = (prefix: string) => aliases.some((route) => route === prefix || route.startsWith(`${prefix}/`));
  const hasExact = (...paths: string[]) => paths.some((path) => aliases.includes(path));

  if (hasExact("/characters", "/character")) return "surface.character.list";
  if (hasPrefix("/characters") || hasPrefix("/character")) return "surface.character.detail";
  if (hasExact("/memory/manager", "/memory-manager")) return "surface.memory.manager";
  if (hasPrefix("/memory") || hasExact("/memory-timeline", "/episodic", "/graph", "/profiles", "/world-book")) {
    return "surface.memory.detail";
  }
  if (hasPrefix("/workshop") || hasPrefix("/creative-workshop")) return "surface.workshop";
  if (hasExact("/channels/wechat", "/wechat")) return "surface.channel.wechat";
  if (hasExact("/channels/qq", "/qq")) return "surface.channel.qq";
  if (hasPrefix("/settings/devices") || hasPrefix("/devices")) return "surface.settings.devices";
  if (hasPrefix("/developer/kernel") || hasPrefix("/kernel")) return "surface.kernel";
  if (hasPrefix("/settings")) return "surface.settings.section";
  if (hasPrefix("/extensions") || hasPrefix("/extension/page")) return "surface.extension";
  return "surface.page";
}
