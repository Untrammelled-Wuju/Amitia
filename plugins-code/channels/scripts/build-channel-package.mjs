import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { buildPackage, inspectPackage } from "../../../sdk/plugin-cli/dist/archive.js";

function browserHash(filePath) {
  const data = fs.readFileSync(filePath);
  return `sha256-${createHash("sha256").update(data).digest("base64")}`;
}

function updateContributionHashes(sourceDir, manifest) {
  for (const module of manifest.modules ?? []) {
    for (const contribution of module.contributions ?? []) {
      const entry = contribution.spec?.entry;
      if (!entry) continue;
      const relativePath = entry.path ?? entry.schema_path;
      if (!relativePath) continue;
      const absolutePath = path.join(sourceDir, relativePath);
      if (!fs.existsSync(absolutePath)) {
        throw new Error(`entry file not found: ${relativePath}`);
      }
      entry.content_hash = browserHash(absolutePath);
    }
  }
}

export function buildChannelPackage(sourceDir, outputDir) {
  const manifestPath = path.join(sourceDir, "manifest.json");
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  updateContributionHashes(sourceDir, manifest);
  manifest.integrity = {
    ...(manifest.integrity ?? {}),
    algorithm: "sha256",
    contentTreeHash: "",
  };
  fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);

  const packageName = `${manifest.extension.id.replaceAll("/", ".")}-${manifest.extension.version}.amitiax`;
  const outputPath = path.join(outputDir, packageName);
  const temporaryPath = `${outputPath}.tmp`;
  fs.mkdirSync(outputDir, { recursive: true });
  fs.rmSync(temporaryPath, { force: true });
  const inspection = buildPackage(sourceDir, manifestPath, temporaryPath);
  inspectPackage(temporaryPath);
  fs.rmSync(outputPath, { force: true });
  fs.renameSync(temporaryPath, outputPath);
  return {
    packagePath: outputPath,
    treeHash: inspection.treeHash,
    files: inspection.files,
  };
}
