export function normalizeUIRoute(raw: string): string {
  const route = (raw || "/").split(/[?#]/, 1)[0].trim() || "/";
  const prefixed = route.startsWith("/") ? route : `/${route}`;
  return prefixed.length > 1 ? prefixed.replace(/\/+$/, "") : prefixed;
}

function hasPrefix(route: string, prefix: string): boolean {
  return route === prefix || route.startsWith(`${prefix}/`);
}

export interface UISurfaceDefinition {
  id: string;
  routePrefixes: readonly string[];
  exactRoutes: readonly string[];
}

export const UI_SURFACE_DEFINITIONS: readonly UISurfaceDefinition[] = [
  { id: "surface.character.list", routePrefixes: [], exactRoutes: ["/characters"] },
  { id: "surface.character.detail", routePrefixes: ["/characters", "/character"], exactRoutes: [] },
  { id: "surface.memory.manager", routePrefixes: [], exactRoutes: ["/memory-manager"] },
  {
    id: "surface.memory.detail",
    routePrefixes: ["/memory"],
    exactRoutes: ["/memory-timeline", "/episodic", "/graph", "/profiles", "/world-book", "/knowledge"],
  },
  { id: "surface.workshop", routePrefixes: ["/creative-workshop"], exactRoutes: [] },
  { id: "surface.settings.devices", routePrefixes: ["/settings/devices"], exactRoutes: [] },
  { id: "surface.kernel", routePrefixes: ["/kernel"], exactRoutes: [] },
  {
    id: "surface.settings.section",
    routePrefixes: ["/settings"],
    exactRoutes: ["/long-running", "/decision-viz", "/storage"],
  },
  { id: "surface.extension", routePrefixes: ["/extensions", "/extension/page"], exactRoutes: [] },
  { id: "surface.page", routePrefixes: ["/"], exactRoutes: [] },
];

export function resolveUISurfaceDefinition(rawRoute: string): UISurfaceDefinition {
  const route = normalizeUIRoute(rawRoute);
  return UI_SURFACE_DEFINITIONS.find((definition) =>
    definition.exactRoutes.includes(route)
    || definition.routePrefixes.some((prefix) => hasPrefix(route, prefix)),
  ) ?? UI_SURFACE_DEFINITIONS[UI_SURFACE_DEFINITIONS.length - 1]!;
}

export function canonicalUISurfaceId(rawRoute: string): string {
  return resolveUISurfaceDefinition(rawRoute).id;
}
