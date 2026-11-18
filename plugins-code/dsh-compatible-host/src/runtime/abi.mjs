import { DSH_CATALOG_SUMMARY } from "./catalog-summary.generated.mjs";

export const TARGET_DSH_VERSION = "0.2.2-rc.2";

export const TARGET_DSH_CLIENT_UI_PRIMITIVES_VERSION = "0.2.2-rc.2";

export const DSH_ABI_CAPABILITIES = Object.freeze([
  "host.apply",
  "host.effect",
  "host.events",
  "host.tools",
  "host.system-prompt.section",
  "host.system-prompt.context",
  "host.system-prompt.tools",
  "host.system-prompt.variable",
  "host.agents",
  "host.sessions",
  "host.session-projections",
  "host.user-questions",
  "host.working-directory",
  "host.fs",
  "host.jobs",
  "host.subagents",
  "host.mcp-client",
  "host.mcp-resources",
  "client.module-loader",
  "client.react",
  "client.slots",
  "client.locale",
  "client.store",
  "client.inject",
  "client.children",
  "client.portal",
  "client.remote",
  "client.sessions",
  "client.resources",
  "client.ui-conversation",
  "client.workspace-panels",
]);

export const DSH_CAPABILITY_STATUS = Object.freeze({
  "host.apply": "implemented",
  "host.effect": "implemented",
  "host.events": "implemented",
  "host.tools": "implemented",
  "host.system-prompt.section": "implemented",
  "host.system-prompt.context": "implemented",
  "host.system-prompt.tools": "implemented",
  "host.system-prompt.variable": "implemented",
  "host.agents": "partial",
  "host.sessions": "partial",
  "host.session-projections": "implemented",
  "host.user-questions": "partial",
  "host.working-directory": "partial",
  "host.fs": "partial",
  "host.jobs": "implemented",
  "host.subagents": "partial",
  "host.mcp-client": "implemented",
  "host.mcp-resources": "implemented",
  "client.module-loader": "implemented",
  "client.react": "implemented",
  "client.slots": "implemented",
  "client.locale": "implemented",
  "client.store": "partial",
  "client.inject": "partial",
  "client.children": "implemented",
  "client.portal": "implemented",
  "client.remote": "partial",
  "client.sessions": "partial",
  "client.resources": "implemented",
  "client.ui-conversation": "partial",
  "client.workspace-panels": "partial",
});

export function compatibilitySummary() {
  const statuses = Object.values(DSH_CAPABILITY_STATUS);
  return {
    capabilityCount: DSH_ABI_CAPABILITIES.length,
    implemented: statuses.filter((status) => status === "implemented").length,
    partial: statuses.filter((status) => status === "partial").length,
    unavailable: statuses.filter((status) => status === "unavailable").length,
    complete: statuses.every((status) => status === "implemented"),
    catalog: DSH_CATALOG_SUMMARY,
  };
}
