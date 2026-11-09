import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { computeFreezeSourceGateHash } from "../../scripts/lib/freeze-scope.mjs";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const manifestPath = resolve(desktopRoot, "resources/release-runtime/.release-runtime-assets.json");

function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

function loadManifest() {
  if (!existsSync(manifestPath)) {
    throw new Error("release runtime manifest missing; run prepare-release-runtime-assets.mjs before packaging");
  }
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
  if (manifest.schemaVersion !== 2 || !Array.isArray(manifest.entries) || manifest.entries.length === 0) {
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
  if (
    manifest.platform === "windows" &&
    !manifest.entries.some((entry) => entry.path === "powershell/pwsh.exe")
  ) {
    throw new Error("bundled PowerShell runtime is missing from the release manifest");
  }
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

  for (const pluginName of ["plugin-market", "runtime-insights", "workspace", "chat-import", "reminders", "storage", "chat-logs", "memory-hub", "character-hub", "creative-workshop-hub", "game-center-hub"]) {
    const pluginDir = resolve(unpackedDir, "resources", "bundled-plugins", pluginName);
    const pluginPackages = existsSync(pluginDir)
      ? readdirSync(pluginDir).filter((name) => name.endsWith(".amitiax"))
      : [];
    if (pluginPackages.length !== 1) {
      throw new Error(`packaged ${pluginName} package count must be 1, got ${pluginPackages.length}`);
    }
    const packagedPlugin = resolve(pluginDir, pluginPackages[0]);
    const sourcePlugin = resolve(repositoryRoot, "plugin", pluginName, pluginPackages[0]);
    if (!existsSync(sourcePlugin)) throw new Error(`source ${pluginName} package missing: ${pluginPackages[0]}`);
    if (sha256File(packagedPlugin) !== sha256File(sourcePlugin)) {
      throw new Error(`packaged ${pluginName} package SHA mismatch`);
    }
  }

  console.log(`[packaged-runtime] PASS: ${manifest.entries.length} packaged runtime assets match staged hashes`);
}

main().catch((error) => {
  console.error(`[packaged-runtime] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
