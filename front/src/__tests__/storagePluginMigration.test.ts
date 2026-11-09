import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

interface StorageManifest {
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

describe("storage plugin migration", () => {
  it("keeps /storage out of the fixed business router", () => {
    expect(builtinBusinessRoutes.some((route) => route.path === "/storage")).toBe(false);
  });

  it("provides /storage through the route registry", () => {
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/storage/amitia-extension.json"),
      "utf8",
    )) as StorageManifest;
    const routes = manifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? []);
    expect(routes).toContainEqual(expect.objectContaining({
      path: "/storage",
      providerId: "storage-page-provider",
    }));
  });
});
