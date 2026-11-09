import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

function pluginMarketRoutes(): string[] {
  const manifest = JSON.parse(readFileSync(
    resolve(__dirname, "../../../plugins-builtin/plugin-market/amitia-extension.json"),
    "utf8",
  ));
  return manifest.modules
    .flatMap((module: any) => module.contributions)
    .flatMap((contribution: any) => contribution.spec?.metadata?.routes ?? [])
    .map((item: any) => item.path);
}

describe("extension center baseline", () => {
  it("keeps the extension center root in core and marketplace pages in the plugin", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    expect(paths).toContain("/extensions");
    expect(pluginMarketRoutes()).toEqual(expect.arrayContaining([
      "/extensions/market",
      "/extensions/packages",
      "/extensions/workflows",
      "/extensions/mcp",
      "/extensions/agent-skills",
    ]));
  });

  it("keeps kernel and runtime insight pages in core", () => {
    const paths = builtinBusinessRoutes.map((route) => String(route.path ?? ""));
    expect(paths).toEqual(expect.arrayContaining([
      "/kernel",
      "/kernel/wasm",
      "/kernel/hooks",
      "/kernel/trusted-services",
      "/runtime-debug",
      "/decision-viz",
      "/long-running",
    ]));
  });
});
