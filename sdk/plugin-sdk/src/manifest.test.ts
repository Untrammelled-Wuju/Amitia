import { describe, expect, it } from "vitest";
import { PLUGIN_SCHEMA_VERSION, validateManifest, type PluginManifest } from "./manifest";

function validManifest(): PluginManifest {
  return {
    schemaVersion: PLUGIN_SCHEMA_VERSION,
    plugin: {
      id: "com.example/plugin",
      name: { default: "Plugin" },
      description: { default: "Plugin" },
      version: "1.0.0",
      publisher: { id: "com.example", displayName: "Example" },
    },
    compatibility: { minHostVersion: "0.1.0" },
    modules: [
      {
        id: "main",
        name: { default: "Main" },
        runtime: { adapter: "javascript", entry: "dist/index.js" },
        services: {
          provides: [
            {
              serviceId: "example.service",
              version: "1.0.0",
              scopes: ["global"],
              placement: "device",
            },
          ],
          requires: [{ serviceId: "example.dependency", versionRange: "^1.0.0" }],
        },
        dynamic: {
          allowedAdapters: ["tool"],
          maxTools: 4,
        },
        contributions: [
          {
            id: "command",
            kind: "tool",
            adapter: "tool",
            contractVersion: 1,
            name: { default: "Command" },
            spec: { toolId: "example.command" },
          },
        ],
      },
    ],
    integrity: { algorithm: "sha256", contentTreeHash: "a".repeat(64) },
  };
}

describe("validateManifest", () => {
  it("accepts the unified plugin contract", () => {
    expect(validateManifest(validManifest())).toEqual([]);
  });

  it("requires an explicit contribution adapter", () => {
    const manifest = validManifest();
    const contribution = manifest.modules[0]?.contributions?.[0];
    if (!contribution) throw new Error("missing fixture contribution");
    delete (contribution as { adapter?: string }).adapter;
    expect(validateManifest(manifest)).toContain("module main contribution command: adapter is required");
  });

  it("requires a runtime adapter instead of a module type", () => {
    const manifest = validManifest();
    const runtime = manifest.modules[0]?.runtime;
    if (!runtime) throw new Error("missing fixture runtime");
    delete (runtime as { adapter?: string }).adapter;
    expect(validateManifest(manifest)).toContain("module main: runtime.adapter is required");
  });

  it("rejects invalid dynamic limits", () => {
    const manifest = validManifest();
    if (manifest.modules[0]?.dynamic) manifest.modules[0].dynamic.maxTools = -1;
    expect(validateManifest(manifest)).toContain("module main: dynamic.maxTools must be a non-negative integer");
  });
});
