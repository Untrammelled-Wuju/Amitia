export function normalizeUIRoute(raw: string): string {
  const route = (raw || "/").split(/[?#]/, 1)[0].trim() || "/";
  const prefixed = route.startsWith("/") ? route : `/${route}`;
  return prefixed.length > 1 ? prefixed.replace(/\/+$/, "") : prefixed;
}

function hasPrefix(route: string, prefix: string): boolean {
  return route === prefix || route.startsWith(`${prefix}/`);
}

export function canonicalUISurfaceId(rawRoute: string): string {
  const route = normalizeUIRoute(rawRoute);
  if (route === "/characters") return "surface.character.list";
  if (hasPrefix(route, "/characters") || hasPrefix(route, "/character")) return "surface.character.detail";
  if (route === "/memory-manager") return "surface.memory.manager";
  if (hasPrefix(route, "/memory") || ["/memory-timeline", "/episodic", "/graph", "/profiles", "/world-book", "/knowledge"].includes(route)) {
    return "surface.memory.detail";
  }
  if (hasPrefix(route, "/creative-workshop")) return "surface.workshop";
  if (hasPrefix(route, "/settings/devices")) return "surface.settings.devices";
  if (hasPrefix(route, "/kernel")) return "surface.kernel";
  if (hasPrefix(route, "/settings") || ["/long-running", "/decision-viz", "/storage"].includes(route)) return "surface.settings.section";
  if (hasPrefix(route, "/extensions") || hasPrefix(route, "/extension/page")) return "surface.extension";
  return "surface.page";
}
