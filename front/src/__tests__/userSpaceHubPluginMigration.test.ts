import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

describe("user space hub plugin migration", () => {
  it("keeps settings and account pages out of the host business router", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    expect(paths).not.toContain("/settings");
    expect(paths).not.toContain("/settings/system");
    expect(paths).not.toContain("/settings/model");
    expect(paths).not.toContain("/my-space");
    expect(paths).not.toContain("/my-space/profile");
  });

  it("retains only the provider repair route under the settings namespace", () => {
    const route = builtinBusinessRoutes.find((item) => item.path === "/settings/ui-providers");
    expect(route?.meta?.allowsExtensionChildren).toBe(true);
    expect(builtinBusinessRoutes.filter((item) => String(item.path ?? "").startsWith("/settings"))).toHaveLength(1);
    expect(shadowsProtectedRoute("/settings/ui-providers")).toBe(true);
    expect(shadowsProtectedRoute("/settings/:section")).toBe(false);
    expect(shadowsProtectedRoute("/my-space/:section")).toBe(false);
  });

  it("provides settings and account surfaces through one plugin host runtime", () => {
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/user-space-hub/amitia-extension.json"),
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
    const routes = contributions
      .flatMap((item) => item.spec?.metadata?.routes ?? [])
      .filter((item): item is { path: string; providerId: string } => typeof item === "object" && item !== null);
    expect(routes.map((item) => item.path)).toEqual(expect.arrayContaining([
      "/settings/:section",
      "/settings/model/:modelType",
      "/my-space/:section",
    ]));
    expect(routes.every((item) => ["user-space-page-provider"].includes(item.providerId))).toBe(true);
    expect(contributions.some((item) => item.spec?.entry?.runtime_id === "host.user-space")).toBe(true);
  });
});
