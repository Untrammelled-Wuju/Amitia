import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { resolveBuiltinNavigationItems } from "@/ui-runtime/navigationRegistry";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { createPinia, setActivePinia } from "pinia";

describe("channels hub plugin migration", () => {
  it("keeps the channel center out of fixed host routes and navigation", () => {
    setActivePinia(createPinia());
    expect(builtinBusinessRoutes.some((route) => route.path === "/channel-center")).toBe(false);
    expect(resolveBuiltinNavigationItems(useExtensionUIStore()).some((item) => item.route === "/channel-center")).toBe(false);
  });

  it("provides the channel center through the fixed channels hub plugin", () => {
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/channels-hub/amitia-extension.json"),
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
    expect(routes).toContainEqual(expect.objectContaining({
      path: "/channel-center",
      providerId: "channels-hub-page-provider",
    }));
    expect(contributions.some((item) => item.spec?.entry?.runtime_id === "host.channels.hub")).toBe(true);
  });
});
