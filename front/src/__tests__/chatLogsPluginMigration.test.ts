import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

interface ChatLogsManifest {
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

describe("chat logs plugin migration", () => {
  it("keeps /logs out of the fixed business router", () => {
    expect(builtinBusinessRoutes.some((route) => route.path === "/logs")).toBe(false);
  });

  it("provides /logs through the route registry", () => {
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/chat-logs/amitia-extension.json"),
      "utf8",
    )) as ChatLogsManifest;
    const routes = manifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? []);
    expect(routes).toContainEqual(expect.objectContaining({
      path: "/logs",
      providerId: "chat-logs-page-provider",
    }));
  });
});
