import { describe, expect, it } from "vitest";
import {
  PLUGIN_CATALOG_SCHEMA_VERSION,
  definePluginCatalog,
  validatePluginCatalog,
  type PluginCatalog,
} from "./catalog";
import { defineCustomContribution } from "./manifest";

describe("plugin catalog", () => {
  it("accepts fixed package identity and provenance", () => {
    const catalog: PluginCatalog = {
      schemaVersion: PLUGIN_CATALOG_SCHEMA_VERSION,
      id: "com.example.market",
      source: { id: "com.example.market", type: "community" },
      entries: [{
        extensionId: "com.example/tools",
        version: "1.0.0",
        name: "Tools",
        package: {
          uri: "https://plugins.example.com/tools.amitiax",
          sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
        },
        publisher: { id: "com.example" },
      }],
    };
    expect(validatePluginCatalog(catalog)).toEqual([]);
    expect(definePluginCatalog(catalog)).toBe(catalog);
  });

  it("rejects duplicate package identities and mutable hashes", () => {
    const catalog = {
      schemaVersion: PLUGIN_CATALOG_SCHEMA_VERSION,
      id: "catalog",
      entries: [{
        extensionId: "com.example/tools",
        version: "1.0.0",
        name: "Tools",
        package: { uri: "https://plugins.example.com/tools.amitiax", sha256: "main" },
      }, {
        extensionId: "com.example/tools",
        version: "1.0.0",
        name: "Tools",
        package: { uri: "https://plugins.example.com/tools.amitiax", sha256: "main" },
      }],
    };
    const errors = validatePluginCatalog(catalog);
    expect(errors.some((item) => item.includes("duplicates"))).toBe(true);
    expect(errors.filter((item) => item.includes("sha256"))).toHaveLength(2);
  });

  it("types custom contribution contracts without host enum changes", () => {
    const contribution = defineCustomContribution({
      id: "page",
      kind: "com.example.market/page",
      adapter: "ui",
      contractVersion: 1,
      schema: { type: "object" },
      name: { default: "Page" },
      spec: { kind: "schema_page", slot: { slotId: "root" } },
    });
    expect(contribution.kind).toBe("com.example.market/page");
    expect(contribution.spec.slot).toEqual({ slotId: "root" });
  });
});
