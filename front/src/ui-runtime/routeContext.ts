function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function routePathSegments(fullPath: unknown): string[] {
  const normalized = text(fullPath).split(/[?#]/, 1)[0] ?? "";
  return normalized.split("/").filter(Boolean);
}

function characterIdFromSegments(segments: string[]): string {
  const index = segments.indexOf("characters");
  return index >= 0 ? text(segments[index + 1]) : "";
}

export function resolveRouteIdentity(
  params: Record<string, unknown> | null | undefined,
  fullPath?: string,
): {
  characterId: string;
  conversationId: string;
} {
  const pathMatch = Array.isArray(params?.pathMatch)
    ? params.pathMatch.map((value) => text(value)).filter(Boolean)
    : [];
  const pathSegments = routePathSegments(fullPath);
  return {
    characterId: text(params?.characterId)
      || text(params?.id)
      || characterIdFromSegments(pathMatch)
      || characterIdFromSegments(pathSegments),
    conversationId: text(params?.conversationId)
      || text(params?.conversation)
      || text(params?.conversation_id),
  };
}

export function buildProviderRouteContext(
  fullPath: string,
  params: Record<string, unknown>,
  query: Record<string, unknown>,
  extras: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    route: fullPath,
    routeParams: params,
    routeQuery: query,
    ...resolveRouteIdentity(params, fullPath),
    ...extras,
  };
}
