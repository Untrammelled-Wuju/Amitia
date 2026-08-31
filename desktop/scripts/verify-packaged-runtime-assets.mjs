import { createHash } from "node:crypto";
import { existsSync, readFileSync, statSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { computeFreezeSourceGateHash } from "../../scripts/lib/freeze-scope.mjs";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const manifestPath = resolve(desktopRoot, "resources/core/.release-runtime-assets.json");

function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

function loadManifest() {
  if (!existsSync(manifestPath)) {
    throw new Error("release runtime manifest missing; run prepare-release-runtime-assets.mjs before packaging");
  }
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
  if (manifest.schemaVersion !== 1 || !Array.isArray(manifest.entries) || manifest.entries.length === 0) {
    throw new Error("release runtime manifest is invalid");
  }
  return manifest;
}

async function main() {
  const unpackedDir = process.argv[2] ? resolve(process.argv[2]) : null;
  if (!unpackedDir || !existsSync(unpackedDir)) {
    throw new Error("usage: node verify-packaged-runtime-assets.mjs <unpacked-dir>");
  }

  const manifest = loadManifest();
  const currentSourceGate = await computeFreezeSourceGateHash(repositoryRoot);
  if (manifest.sourceGateSha256 !== currentSourceGate) {
    throw new Error("release runtime assets were staged from a different frozen source; restage required");
  }

  for (const entry of manifest.entries) {
    const packagedPath = resolve(unpackedDir, "resources", entry.path);
    if (!existsSync(packagedPath)) throw new Error(`packaged runtime asset missing: ${entry.path}`);
    const stats = statSync(packagedPath);
    if (!stats.isFile()) throw new Error(`packaged runtime asset is not a file: ${entry.path}`);
    if (stats.size !== entry.bytes) throw new Error(`packaged runtime asset size mismatch: ${entry.path}`);
    if (sha256File(packagedPath) !== entry.sha256) {
      throw new Error(`packaged runtime asset SHA mismatch: ${entry.path}`);
    }
  }

  console.log(`[packaged-runtime] PASS: ${manifest.entries.length} packaged runtime assets match staged hashes`);
}

main().catch((error) => {
  console.error(`[packaged-runtime] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
