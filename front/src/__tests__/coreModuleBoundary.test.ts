import { existsSync, readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

const root = resolve(__dirname, "../../..");

const coreOnlyExtensionIds = [
  "com.amitia/active-task",
  "com.amitia/memory-hub",
  "com.amitia/character-hub",
  "com.amitia/user-space-hub",
  "com.amitia/chat-logs",
  "com.amitia/channels-hub",
  "com.amitia/game-center-hub",
  "com.amitia/chat-import",
  "com.amitia/runtime-insights",
  "com.amitia/workspace",
  "com.amitia/storage",
  "com.amitia/creative-workshop-hub",
];

const coreOnlyPaths = [
  "/characters",
  "/memory",
  "/tasks",
  "/settings",
  "/my-space",
  "/logs",
  "/channel-center",
  "/game-center",
  "/import",
  "/runtime-debug",
  "/decision-viz",
  "/long-running",
  "/workspaces",
  "/storage",
  "/creative-workshop",
  "/kernel",
];

const coreOnlyDirectories = coreOnlyExtensionIds.map(
  (extensionId) => extensionId.split("/")[1],
);

function sourceManifests(root: string): Array<{ id: string; path: string }> {
  const result: Array<{ id: string; path: string }> = [];
  const visit = (directory: string) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (["node_modules", ".package-staging", "dist", "build"].includes(entry.name)) continue;
      const path = resolve(directory, entry.name);
      if (entry.isDirectory()) {
        visit(path);
        continue;
      }
      if (entry.name !== "amitia-extension.json") continue;
      const manifest = JSON.parse(readFileSync(path, "utf8")) as {
        plugin?: { id?: string };
      };
      result.push({ id: String(manifest.plugin?.id ?? ""), path });
    }
  };
  visit(root);
  return result;
}

describe("core module boundaries", () => {
  it("never references explicitly retained core modules in release paths", () => {
    const releaseSources = [
      "scripts/build-marketplace-plugins.mjs",
      "scripts/publish-marketplace-plugins.mjs",
      "desktop/electron-builder.yml",
      "desktop/electron-builder.ci.yml",
    ].map((path) => ({ path, source: readFileSync(resolve(root, path), "utf8") }));
    const contract = JSON.parse(readFileSync(
      resolve(root, "contracts/ui-runtime/v1/provider-route-consistency.json"),
      "utf8",
    )) as { providerRoutes: Array<{ extensionId: string }> };
    const providerIds = new Set(contract.providerRoutes.map((route) => route.extensionId));

    for (const extensionId of coreOnlyExtensionIds) {
      expect(providerIds.has(extensionId), extensionId).toBe(false);
      for (const file of releaseSources) {
        expect(file.source, `${file.path}: ${extensionId}`).not.toContain(extensionId);
      }
    }
    for (const directory of coreOnlyDirectories) {
      for (const file of releaseSources.slice(2)) {
        expect(file.source, `${file.path}: ${directory}`).not.toContain(`../plugin/${directory}`);
        expect(file.source, `${file.path}: ${directory}`).not.toContain(`bundled-plugins/${directory}`);
      }
    }
  });

  it("keeps retained core modules out of every plugin source directory", () => {
    expect(existsSync(resolve(root, "plugins-builtin/plugin-market/amitia-extension.json"))).toBe(true);
    const manifests = [
      ...sourceManifests(resolve(root, "plugins-builtin")),
      ...sourceManifests(resolve(root, "plugins-code")),
    ];
    const builtInManifests = sourceManifests(resolve(root, "plugins-builtin"));
    expect(builtInManifests.map((manifest) => manifest.id)).toEqual(["com.amitia/plugin-market"]);
    const manifestIds = new Set(manifests.map((manifest) => manifest.id));
    for (const extensionId of coreOnlyExtensionIds) {
      expect(manifestIds.has(extensionId), extensionId).toBe(false);
    }
  });

  it("keeps every retained module route in the core router", () => {
    const paths = new Set(builtinBusinessRoutes.map((route) => String(route.path ?? "")));
    for (const path of coreOnlyPaths) expect(paths.has(path), path).toBe(true);
  });

  it("keeps extension injection points on retained core surfaces", () => {
    const surfaces = [
      ["front/src/views/tasks/ActiveTasksView.vue", "ExtensionSlot"],
      ["front/src/views/character-config/CharacterConfigView.vue", "ExtensionSlot"],
      ["front/src/views/user-settings/UserSpaceHost.vue", "ExtensionSlot"],
      ["front/src/views/game-center/GamePluginView.vue", "ExtensionSlot"],
      ["front/src/views/channels/ChannelHostMessagesPanel.vue", "UIProviderHost"],
    ];
    for (const [path, marker] of surfaces) {
      expect(readFileSync(resolve(root, path), "utf8"), path).toContain(marker);
    }
  });
});
