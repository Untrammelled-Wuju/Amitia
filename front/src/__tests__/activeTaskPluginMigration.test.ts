import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

interface ActiveTaskManifest {
  modules: Array<{
    contributions: Array<{
      spec?: {
        metadata?: {
          routes?: Array<{ path: string; providerId: string }>;
        };
      };
    }>;
  }>;
}

describe("active task plugin migration", () => {
  it("keeps /reminders out of the fixed business router", () => {
    expect(builtinBusinessRoutes.some((route) => route.path === "/reminders")).toBe(false);
  });

  it("provides the merged task page through the active task route registry", () => {
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/active-task/amitia-extension.json"),
      "utf8",
    )) as ActiveTaskManifest;
    const routes = manifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? []);
    expect(routes).toContainEqual(expect.objectContaining({
      path: "/reminders",
      providerId: "active-task-page-provider",
    }));
  });
});
