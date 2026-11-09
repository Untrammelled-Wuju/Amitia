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

describe("task maintenance plugin contract", () => {
  it("declares maintenance actions and the long-running route", () => {
    const manifest = JSON.parse(readFileSync(resolve(root, "amitia-extension.json"), "utf8"));
    const tool = manifest.modules[0].contributions.find((item: any) => item.id === "runtime-insights-command");
    expect(tool.spec.inputSchema.properties.action.enum).toEqual(expect.arrayContaining([
      "maintenance.status",
      "maintenance.config",
      "maintenance.update",
      "maintenance.cleanup-temp",
      "maintenance.rotate-logs",
      "maintenance.check-db",
    ]));
    const routes = manifest.modules[0].contributions
      .flatMap((item: any) => item.spec?.metadata?.routes ?? []);
    expect(routes).toContainEqual(expect.objectContaining({
      id: "long-running",
      path: "/long-running",
    }));
  });

  it("loads runtime state and submits the authoritative configuration revisions", () => {
    const document = JSON.parse(readFileSync(resolve(root, "ui/schema/long-running.json"), "utf8")) as SchemaUIDocument;
    expect(document.lifecycle?.onMount?.map((action) => action.input?.action)).toEqual([
      "maintenance.overview",
      "maintenance.config",
    ]);
    const save = findNode(document.children ?? [], "long-running-save-config");
    expect(save?.actions?.[0]?.input).toMatchObject({
      action: "maintenance.update",
      payload: {
        maxTasks: "$form.maxTasks",
        timeout: {
          disabled: "$form.timeoutDisabled",
          seconds: "$form.timeoutSeconds",
        },
        expectedRevision: "$state.maintenanceConfig.revision",
        expectedTimeoutRevision: "$state.maintenanceConfig.timeoutRevision",
      },
    });
  });
});
