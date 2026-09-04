import { createHash } from "node:crypto";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { dirname, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { computeFreezeSourceGateHash } from "../../scripts/lib/freeze-scope.mjs";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const resourcesRoot = resolve(desktopRoot, "resources");
const coreRoot = resolve(resourcesRoot, "core");
const manifestPath = resolve(coreRoot, ".release-runtime-assets.json");

const sourceCopies = [
  {
    source: resolve(repositoryRoot, "backend/sidecar/bundle.mjs"),
    destination: resolve(coreRoot, "sidecar/bundle.mjs"),
  },
  {
    source: resolve(repositoryRoot, "backend/sidecar/launcher.mjs"),
    destination: resolve(coreRoot, "sidecar/launcher.mjs"),
  },
  {
    source: resolve(repositoryRoot, "backend/qq-sidecar/bundle.mjs"),
    destination: resolve(coreRoot, "qq-sidecar/bundle.mjs"),
  },
  {
    source: resolve(repositoryRoot, "backend/qq-sidecar/launcher.mjs"),
    destination: resolve(coreRoot, "qq-sidecar/launcher.mjs"),
  },
];

function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

function assertRegularNonEmptyFile(filePath, label) {
  if (!existsSync(filePath)) throw new Error(`${label} missing: ${filePath}`);
  const stats = statSync(filePath);
  if (!stats.isFile()) throw new Error(`${label} is not a regular file: ${filePath}`);
  if (stats.size <= 0) throw new Error(`${label} is empty: ${filePath}`);
  return stats;
}

function copyFrozenSidecars() {
  for (const item of sourceCopies) {
    assertRegularNonEmptyFile(item.source, "frozen sidecar source");
    mkdirSync(dirname(item.destination), { recursive: true });
    copyFileSync(item.source, item.destination);
    if (sha256File(item.source) !== sha256File(item.destination)) {
      throw new Error(`sidecar staging SHA mismatch: ${relative(repositoryRoot, item.source)}`);
    }
  }
}

function listFilesRecursive(root) {
  if (!existsSync(root)) return [];
  const output = [];
  const visit = (current) => {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const absolute = resolve(current, entry.name);
      if (entry.isDirectory()) visit(absolute);
      else if (entry.isFile()) output.push(absolute);
    }
  };
  visit(root);
  return output.sort((a, b) => a.localeCompare(b));
}

function packagePath(absolutePath) {
  return relative(resourcesRoot, absolutePath).split(sep).join("/");
}

function collectRuntimeFiles() {
  const coreExe = resolve(coreRoot, "AmitiaCore.exe");
  assertRegularNonEmptyFile(coreExe, "AmitiaCore");

  const nodeCandidates = [
    resolve(coreRoot, "node/node.exe.zip"),
    resolve(coreRoot, "node/node.zip"),
  ].filter((filePath) => existsSync(filePath));
  if (nodeCandidates.length === 0) {
    throw new Error("Node runtime archive missing: provide resources/core/node/node.exe.zip or node.zip");
  }
  for (const filePath of nodeCandidates) assertRegularNonEmptyFile(filePath, "Node runtime archive");

  const qdrantZip = resolve(resourcesRoot, "qdrant/qdrant.zip");
  const qdrantConfig = resolve(resourcesRoot, "qdrant/config/config.yaml");
  const surrealZip = resolve(resourcesRoot, "surrealdb/surreal.zip");
  assertRegularNonEmptyFile(qdrantZip, "Qdrant runtime archive");
  assertRegularNonEmptyFile(qdrantConfig, "Qdrant config");
  assertRegularNonEmptyFile(surrealZip, "SurrealDB runtime archive");

  const requiredSourceCopies = sourceCopies.map((item) => item.destination);
  for (const filePath of requiredSourceCopies) assertRegularNonEmptyFile(filePath, "staged sidecar");

  const files = new Set([
    coreExe,
    ...nodeCandidates,
    ...requiredSourceCopies,
    qdrantZip,
    qdrantConfig,
    surrealZip,
    ...listFilesRecursive(resolve(resourcesRoot, "bridge")),
    ...listFilesRecursive(resolve(resourcesRoot, "migrations")),
    ...listFilesRecursive(resolve(resourcesRoot, "config-template")),
  ]);

  // Never let generated provenance files become package inputs.
  files.delete(manifestPath);
  files.delete(resolve(coreRoot, ".amitiacore-build.json"));

  return [...files].sort((a, b) => packagePath(a).localeCompare(packagePath(b)));
}

export async function prepareReleaseRuntimeAssets() {
  copyFrozenSidecars();
  const sourceGateSha256 = await computeFreezeSourceGateHash(repositoryRoot);
  const files = collectRuntimeFiles();
  const sourceCopyDestinations = new Set(sourceCopies.map((item) => item.destination));
  const entries = files.map((absolute) => {
    const stats = statSync(absolute);
    let provenance = "external-runtime";
    if (absolute === resolve(coreRoot, "AmitiaCore.exe")) provenance = "built-core";
    if (sourceCopyDestinations.has(absolute)) provenance = "frozen-source-copy";
    if (
      absolute.startsWith(resolve(resourcesRoot, "bridge") + sep) ||
      absolute.startsWith(resolve(resourcesRoot, "migrations") + sep) ||
      absolute.startsWith(resolve(resourcesRoot, "config-template") + sep) ||
      absolute === resolve(resourcesRoot, "qdrant/config/config.yaml")
    ) {
      provenance = "frozen-source-resource";
    }
    return {
      path: packagePath(absolute),
      bytes: stats.size,
      sha256: sha256File(absolute),
      provenance,
    };
  });

  const manifest = {
    schemaVersion: 1,
    createdAt: new Date().toISOString(),
    sourceGateSha256,
    entries,
  };
  mkdirSync(coreRoot, { recursive: true });
  writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
  return manifest;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const manifest = await prepareReleaseRuntimeAssets();
    console.log(
      `[release-runtime] PASS: ${manifest.entries.length} runtime files staged; source gate ${manifest.sourceGateSha256.slice(0, 12)}...`,
    );
  } catch (error) {
    console.error(`[release-runtime] FAILED: ${error instanceof Error ? error.message : String(error)}`);
    process.exit(1);
  }
}
