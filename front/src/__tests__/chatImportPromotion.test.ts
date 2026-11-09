import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { isDeviceLocalApiPath } from "@/runtime/runtime-adapter";

interface ChatImportManifest {
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

describe("chat import plugin migration", () => {
  it("keeps the import page out of the host business router", () => {
    expect(builtinBusinessRoutes.some((route) => route.path === "/import")).toBe(false);
  });

  it("provides /import through the chat-import plugin route registry", () => {
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/chat-import/amitia-extension.json"),
      "utf8",
    )) as ChatImportManifest;
    const routes = manifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? []);
    expect(routes).toContainEqual(expect.objectContaining({
      path: "/import",
      providerId: "chat-import-page-provider",
    }));
  });

  it("retains device-local staging behind the core runtime", () => {
    expect(isDeviceLocalApiPath("/api/imports/confirm")).toBe(true);
  });
});
