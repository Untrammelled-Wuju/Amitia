import type { AmitiaxManifestV1, CustomManifestContribution } from "@amitia/plugin-sdk";
import { defineCustomContribution } from "@amitia/plugin-sdk";

export type { AmitiaxManifestV1, CustomManifestContribution };
export { defineCustomContribution };

const contributionAdapters = new Set([
  "tool",
  "agent_skill",
  "workflow",
  "mcp_server",
  "hook",
  "event",
  "schedule",
  "ui",
  "ui_provider",
  "ui_slot",
  "background",
  "provider",
  "game",
  "desktop_pet",
]);

const builtInContributionKinds = new Set([
  "tool",
  "agent_skill",
  "workflow",
  "mcp_server",
  "hook",
  "event_subscription",
  "schedule",
  "background_task",
  "ui_page",
  "ui_panel",
  "ui_chat",
  "ui_context_action",
  "ui_desktop",
  "ui_provider",
  "ui_slot",
  "provider",
  "gamex",
  "petx",
]);

export function validateManifest(manifest: AmitiaxManifestV1): string[] {
  const errors: string[] = [];
  if (manifest.manifestVersion !== 1) errors.push("manifestVersion must be 1");
  if (!manifest.extension?.id) errors.push("extension.id is required");
  if (!manifest.publisher?.id) errors.push("publisher.id is required");
  if (!manifest.extension?.name?.default) errors.push("extension.name.default is required");
  if (!manifest.extension?.version) errors.push("extension.version is required");
  if (!manifest.modules || manifest.modules.length === 0) errors.push("at least one module is required");
  for (const module of manifest.modules ?? []) {
    if (!module.id) errors.push("module.id is required");
    if (!module.type) errors.push(`module ${module.id}: type is required`);
    if (!module.name?.default) errors.push(`module ${module.id}: name.default is required`);
    if (module.runtime && !module.runtime.type) errors.push(`module ${module.id}: runtime.type is required`);
    const access = module.runtime?.hostAccess;
    if (access?.filesystem?.mode && !["none", "read", "read_write"].includes(access.filesystem.mode)) {
      errors.push(`module ${module.id}: runtime.hostAccess.filesystem.mode is invalid`);
    }
    if (access?.network?.mode && !["none", "host"].includes(access.network.mode)) {
      errors.push(`module ${module.id}: runtime.hostAccess.network.mode is invalid`);
    }
    if (access?.process?.mode && !["none", "direct"].includes(access.process.mode)) {
      errors.push(`module ${module.id}: runtime.hostAccess.process.mode is invalid`);
    }
    if (access?.process?.mode === "direct" && (!access.process.maxProcesses || access.process.maxProcesses < 1)) {
      errors.push(`module ${module.id}: direct process access requires maxProcesses`);
    }
    for (const contribution of module.contributions ?? []) {
      if (!builtInContributionKinds.has(contribution.kind)) {
        if (!contribution.adapter) {
          errors.push(`module ${module.id}: contribution ${contribution.id} requires adapter for custom kind ${contribution.kind}`);
        }
        if (!contribution.contractVersion || contribution.contractVersion < 1) {
          errors.push(`module ${module.id}: contribution ${contribution.id} requires contractVersion for custom kind ${contribution.kind}`);
        }
        if (!contribution.schema || typeof contribution.schema !== "object") {
          errors.push(`module ${module.id}: contribution ${contribution.id} requires schema for custom kind ${contribution.kind}`);
        }
      }
      if (contribution.adapter && !contributionAdapters.has(contribution.adapter)) {
        errors.push(`module ${module.id}: contribution ${contribution.id} uses unknown adapter ${contribution.adapter}`);
      }
    }
  }
  if (!manifest.integrity?.algorithm) errors.push("integrity.algorithm is required");
  return errors;
}
