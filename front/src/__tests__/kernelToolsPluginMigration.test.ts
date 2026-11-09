import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

const kernelPaths = [
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

describe("kernel tools plugin migration", () => {
  it("keeps kernel detail pages out of the fixed business router", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    for (const path of kernelPaths) {
      expect(paths).not.toContain(path);
    }
  });

  it("provides every kernel detail page through runtime insights", () => {
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/runtime-insights/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{
        contributions: Array<{
          spec?: {
            entry?: { runtime_id?: string };
            metadata?: { routes?: Array<{ path: string; providerId: string }> };
          };
        }>;
      }>;
    };
    const contributions = manifest.modules.flatMap((module) => module.contributions);
    const routes = contributions.flatMap((item) => item.spec?.metadata?.routes ?? []);
    const runtimeIds = contributions.map((item) => item.spec?.entry?.runtime_id).filter(Boolean);

    for (const path of kernelPaths) {
      expect(routes.map((route) => route.path)).toContain(path);
    }
    expect(runtimeIds).toEqual(expect.arrayContaining([
      "host.kernel.wasm",
      "host.kernel.hooks",
      "host.kernel.trusted-services",
      "host.kernel.tasks",
      "host.kernel.events",
      "host.kernel.schedules",
      "host.kernel.desktop",
      "host.kernel.updates",
      "host.kernel.dev-console",
      "host.kernel.dev-mode",
    ]));
  });
});
