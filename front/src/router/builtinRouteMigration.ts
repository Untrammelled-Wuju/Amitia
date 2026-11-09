import { builtinBusinessRoutes } from "./builtinRoutes";
import type { UIProviderCapability } from "@/ui-runtime/types";

export interface BuiltinRouteMigration {
  path: string;
  root: string;
  capability: UIProviderCapability;
  priority: number;
}

const migrationByRoot: Record<string, Omit<BuiltinRouteMigration, "path" | "root">> = {
  "/chat": { capability: "conversation.shell", priority: 100 },
  "/attention": { capability: "page.provider", priority: 99 },
  "/call": { capability: "conversation.overlay", priority: 95 },
  "/extensions": { capability: "extension.center", priority: 90 },
  "/extension": { capability: "extension.page", priority: 80 },
  "/kernel": { capability: "settings.section", priority: 60 },
  "/settings": { capability: "settings.shell", priority: 80 },
  "/continuity": { capability: "conversation.sidebar", priority: 65 },
  "/channel-center": { capability: "extension.page", priority: 70 },
  "/my-space": { capability: "settings.shell", priority: 70 },
};

export function routeRoot(path: string): string {
  const segment = String(path || "").split("/").filter(Boolean)[0];
  return segment ? `/${segment}` : "/";
}

export function builtinRouteMigrationPlan(): BuiltinRouteMigration[] {
  const seen = new Set<string>();
  const result: BuiltinRouteMigration[] = [];
  for (const route of builtinBusinessRoutes) {
    const path = String(route.path ?? "");
    if (!path || seen.has(path)) continue;
    const root = routeRoot(path);
    const migration = migrationByRoot[root];
    if (!migration) continue;
    seen.add(path);
    result.push({ path, root, ...migration });
  }
  return result.sort((left, right) => right.priority - left.priority || left.path.localeCompare(right.path));
}

export function unclassifiedBuiltinRoutes(): string[] {
  return builtinBusinessRoutes
    .map((route) => String(route.path ?? ""))
    .filter((path) => path && !migrationByRoot[routeRoot(path)]);
}
