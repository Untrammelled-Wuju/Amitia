import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import {
  builtinRouteMigrationPlan,
  routeRoot,
  unclassifiedBuiltinRoutes,
} from "@/router/builtinRouteMigration";

describe("builtin route ownership", () => {
  it("classifies every built-in business route", () => {
    expect(unclassifiedBuiltinRoutes()).toEqual([]);
    expect(builtinRouteMigrationPlan().length).toBeGreaterThan(0);
  });

  it("maps nested routes to their top-level namespace", () => {
    expect(routeRoot("/settings/model/llm")).toBe("/settings");
    expect(routeRoot("/character/example/voice")).toBe("/character");
    expect(routeRoot("/")).toBe("/");
  });

  it("keeps core modules in the fixed router", () => {
    const paths = builtinBusinessRoutes.map((item) => String(item.path ?? ""));
    expect(paths).toEqual(expect.arrayContaining([
      "/characters",
      "/characters/teams",
      "/character",
      "/character/:id",
      "/character/:id/:tab(life-rules|voice|psyche)",
      "/world-book",
      "/memory",
      "/memory-manager",
      "/memory-timeline",
      "/episodic",
      "/graph",
      "/knowledge",
      "/profiles",
      "/tasks",
      "/reminders",
      "/continuity",
      "/extensions",
      "/settings",
      "/my-space",
      "/logs",
    ]));
    for (const path of ["/extensions/market", "/extensions/packages"]) {
      expect(paths).not.toContain(path);
    }
  });

  it("does not package the retired core modules as plugins", () => {
    const buildScript = readFileSync(resolve(__dirname, "../../../scripts/build-marketplace-plugins.mjs"), "utf8");
    for (const extensionID of ["com.amitia/active-task", "com.amitia/memory-hub", "com.amitia/character-hub"]) {
      expect(buildScript).not.toContain(extensionID);
    }
  });

  it("keeps runtime tools in the core router", () => {
    const paths = builtinBusinessRoutes.map((item) => String(item.path ?? ""));
    expect(paths).toEqual(expect.arrayContaining([
      "/runtime-debug",
      "/decision-viz",
      "/long-running",
      "/kernel/wasm",
      "/kernel/hooks",
      "/kernel/trusted-services",
    ]));
  });
});
