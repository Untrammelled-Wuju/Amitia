import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import type { SchemaUIDocument, SchemaUINode } from "@/components/extension/schema-ui-utils";

const root = resolve(__dirname, "../../../plugins-builtin/runtime-insights");

function findNode(nodes: SchemaUINode[], id: string): SchemaUINode | undefined {
  for (const node of nodes) {
    if (node.id === id) return node;
    const nested = findNode(node.children ?? [], id);
    if (nested) return nested;
  }
  return undefined;
}

describe("runtime maintenance actions", () => {
  it("routes every manual action through the maintenance host API", () => {
    const document = JSON.parse(readFileSync(resolve(root, "ui/schema/long-running.json"), "utf8")) as SchemaUIDocument;
    const expected = {
      "long-running-cleanup": "maintenance.cleanup-temp",
      "long-running-rotate": "maintenance.rotate-logs",
      "long-running-db": "maintenance.check-db",
    };
    for (const [nodeID, action] of Object.entries(expected)) {
      expect(findNode(document.children ?? [], nodeID)?.actions?.[0]?.input).toMatchObject({ action });
    }
  });

  it("contains no direct administrator HTTP endpoint", () => {
    const source = readFileSync(resolve(root, "ui/schema/long-running.json"), "utf8");
    expect(source).not.toContain("/api/runtime/");
  });
});
