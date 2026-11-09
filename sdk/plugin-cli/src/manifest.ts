import {
  validateManifest as validateSdkManifest,
  type CustomManifestContribution,
  type PluginManifest,
} from "@amitia/plugin-sdk";

export type { CustomManifestContribution, PluginManifest };

export function validateManifest(manifest: PluginManifest): string[] {
  return validateSdkManifest(manifest);
}
