import type { Router } from "vue-router";
import { isRuntimeRouteAvailable } from "@/runtime/runtime-capabilities";

const NAVIGATION_BLACKLIST: readonly string[] = [
  "/web-access",
  "/onboarding",
  "/privacy",
  "/usage-boundary",
  "/404",
];

export function isNavigationAllowed(target: string, router?: Router): boolean {
  if (!target || typeof target !== "string") return false;
  const path = target.split("?")[0].split("#")[0];
  if (!path.startsWith("/") || path.startsWith("//") || !isRuntimeRouteAvailable(path)) return false;
  for (const blocked of NAVIGATION_BLACKLIST) {
    if (path === blocked || path.startsWith(blocked + "/")) return false;
  }
  if (!router) return true;
  const resolved = router.resolve(path);
  const name = typeof resolved.name === "string" ? resolved.name : "";
  return resolved.matched.length > 0 && name !== "notFound" && name !== "catchAll";
}
