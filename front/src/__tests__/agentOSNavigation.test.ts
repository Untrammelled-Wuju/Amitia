import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { appSection, resolveAppSection } from "@/navigation/appSections";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

describe("Agent page removal", () => {
  it("does not expose a standalone Agent section or route", () => {
    expect(resolveAppSection("/agents")).toBeNull();
    expect(appSection("character")?.entries.map((entry) => entry.route)).not.toContain("/agents");
    expect(builtinBusinessRoutes.map((route) => route.path)).not.toContain("/agents");
  });

  it("keeps the extensions hub as a core shell without hardcoding the marketplace page", () => {
    const paths = builtinBusinessRoutes.map((route) => route.path);
    expect(paths).not.toContain("/extensions/market");
    expect(paths).not.toContain("/marketplace");
    expect(appSection("extensions")?.entries.map((entry) => entry.route)).not.toContain("/extensions/market");
    const extensionsHub = builtinBusinessRoutes.find((route) => route.path === "/extensions");
    expect(extensionsHub?.meta?.allowsExtensionChildren).toBe(true);
    const manifest = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-builtin/plugin-market/amitia-extension.json"), "utf8"));
    const routes = manifest.modules.flatMap((module: any) => module.contributions)
      .flatMap((contribution: any) => contribution.spec?.metadata?.routes ?? [])
      .map((item: any) => item.path);
    expect(routes).toEqual(expect.arrayContaining(["/extensions", "/kernel"]));
  });
});
