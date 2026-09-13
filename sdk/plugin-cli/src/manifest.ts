import type { AmitiaxManifestV1 } from "@amitia/plugin-sdk";

export type { AmitiaxManifestV1 };

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
  }
  if (!manifest.integrity?.algorithm) errors.push("integrity.algorithm is required");
  return errors;
}
