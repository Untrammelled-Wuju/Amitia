function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

export function resolveRouteIdentity(params: Record<string, unknown> | null | undefined): {
  characterId: string;
  conversationId: string;
} {
  return {
    characterId: text(params?.characterId) || text(params?.id),
    conversationId: text(params?.conversationId),
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
    ...resolveRouteIdentity(params),
    ...extras,
  };
}
