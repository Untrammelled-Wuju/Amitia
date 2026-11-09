import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import {
  builtinRouteMigrationPlan,
  routeRoot,
  unclassifiedBuiltinRoutes,
} from "@/router/builtinRouteMigration";

describe("builtin route migration plan", () => {
  it("classifies every built-in business route", () => {
    expect(unclassifiedBuiltinRoutes()).toEqual([]);
    expect(builtinRouteMigrationPlan().length).toBeGreaterThan(20);
  });

  it("maps nested routes to their top-level provider namespace", () => {
    expect(routeRoot("/settings/model/llm")).toBe("/settings");
    expect(routeRoot("/character/example/voice")).toBe("/character");
    expect(routeRoot("/")).toBe("/");
  });

  it("keeps migrated runtime pages out of the host business router", () => {
    const plan = builtinRouteMigrationPlan();
    const chat = plan.find((item) => item.root === "/chat");
    const paths = builtinBusinessRoutes.map((item) => String(item.path ?? ""));
    expect(chat?.priority).toBeGreaterThan(0);
    expect(paths).not.toContain("/runtime-debug");
    expect(paths).not.toContain("/decision-viz");
    expect(paths).not.toContain("/long-running");
    expect(paths).not.toContain("/workspaces");
    expect(paths).not.toContain("/import");
    expect(paths).not.toContain("/reminders");
    expect(paths).not.toContain("/storage");
    expect(paths).not.toContain("/logs");
    expect(paths).not.toContain("/profiles");
    expect(paths).not.toContain("/episodic");
    expect(paths).not.toContain("/graph");
    expect(paths).not.toContain("/knowledge");
    expect(paths).not.toContain("/memory-manager");
    expect(paths).not.toContain("/memory-timeline");
    expect(paths).not.toContain("/memory");
    expect(paths).not.toContain("/characters");
    expect(paths).not.toContain("/characters/teams");
    expect(paths).not.toContain("/character");
    expect(paths).not.toContain("/character/:id");
    expect(paths).not.toContain("/character/:id/life-rules");
    expect(paths).not.toContain("/character/:id/voice");
    expect(paths).not.toContain("/character/:id/psyche");
    expect(paths).not.toContain("/world-book");
    expect(paths).not.toContain("/creative-workshop");
    expect(paths).not.toContain("/game-center");
    expect(paths).not.toContain("/creative-workshop/pet");
    expect(paths).not.toContain("/privacy-scan");

    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/runtime-insights/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const routes = manifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(routes).toEqual(expect.arrayContaining(["/runtime-debug", "/decision-viz", "/long-running"]));

    const workspaceManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/workspace/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const workspaceRoutes = workspaceManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(workspaceRoutes).toContain("/workspaces");

    const chatImportManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/chat-import/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const chatImportRoutes = chatImportManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(chatImportRoutes).toContain("/import");

    const remindersManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/reminders/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const remindersRoutes = remindersManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(remindersRoutes).toContain("/reminders");

    const storageManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/storage/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const storageRoutes = storageManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(storageRoutes).toContain("/storage");

    const chatLogsManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/chat-logs/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const chatLogsRoutes = chatLogsManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(chatLogsRoutes).toContain("/logs");

    const memoryHubManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/memory-hub/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const memoryHubRoutes = memoryHubManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(memoryHubRoutes).toEqual(expect.arrayContaining([
      "/memory",
      "/memory-manager",
      "/memory-timeline",
      "/episodic",
      "/graph",
      "/knowledge",
      "/profiles",
    ]));

    const characterHubManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/character-hub/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const characterHubRoutes = characterHubManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(characterHubRoutes).toEqual(expect.arrayContaining([
      "/characters",
      "/characters/teams",
      "/character",
      "/character/:id",
      "/character/:id/life-rules",
      "/character/:id/voice",
      "/character/:id/psyche",
      "/world-book",
    ]));

    const creativeWorkshopManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/creative-workshop-hub/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const creativeWorkshopRoutes = creativeWorkshopManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(creativeWorkshopRoutes).toEqual(expect.arrayContaining(["/creative-workshop", "/creative-workshop/pet"]));

    const gameCenterManifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/game-center-hub/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }> } } }> }>;
    };
    const gameCenterRoutes = gameCenterManifest.modules
      .flatMap((module) => module.contributions)
      .flatMap((contribution) => contribution.spec?.metadata?.routes ?? [])
      .map((route) => route.path);
    expect(gameCenterRoutes).toContain("/game-center");
  });
});
