import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

const coreRoutes = [
  "/channel-center",
  "/import",
  "/storage",
  "/creative-workshop",
  "/creative-workshop/character-cards",
  "/creative-workshop/pet",
  "/creative-workshop/pet/installations",
  "/game-center",
  "/game-center/plugin",
  "/runtime-debug",
  "/decision-viz",
  "/long-running",
  "/workspaces",
  "/kernel",
  "/kernel/wasm",
  "/kernel/hooks",
  "/kernel/trusted-services",
  "/kernel/tasks",
  "/kernel/events",
  "/kernel/schedules",
  "/kernel/desktop",
  "/kernel/updates",
  "/kernel/dev-console",
  "/kernel/dev-mode",
];

describe("core business routes", () => {
  it("owns former hub pages in the core router", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    for (const path of coreRoutes) {
      expect(paths).toContain(path);
      expect(shadowsProtectedRoute(path), path).toBe(true);
    }
  });

  it("loads every restored core page component", async () => {
    for (const path of coreRoutes) {
      const route = builtinBusinessRoutes.find((item) => item.path === path);
      expect(route, path).toBeDefined();
      if (typeof route?.component === "function") {
        const module = await (route.component as () => Promise<{ default?: unknown }>)();
        expect(module.default, path).toBeDefined();
      }
    }
  });
});
