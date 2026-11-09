export const CORE_ROUTE_PATHS = [
  "/web-access",
  "/onboarding",
  "/privacy",
  "/usage-boundary",
  "/attention",
  "/404",
] as const;

export const CORE_ROUTE_ROOTS = new Set(
  CORE_ROUTE_PATHS.map((path) => `/${path.split("/").filter(Boolean)[0]}`),
);
