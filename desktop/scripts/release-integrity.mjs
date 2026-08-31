import { createHash } from "node:crypto";
import {
  existsSync,
  readFileSync,
  readdirSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  computeFreezeSourceGateHash,
  verifyFreezeManifest,
} from "../../scripts/lib/freeze-scope.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = fileURLToPath(new URL(".", import.meta.url));
export const desktopRoot = resolve(__dirname, "..");
export const repositoryRoot = resolve(desktopRoot, "..");
export const releaseDir = resolve(desktopRoot, "release");
export const releaseGateStampPath = resolve(releaseDir, ".desktop-pet-release-gate.json");
const coreBuildStampPath = resolve(desktopRoot, "resources/core/.amitiacore-build.json");
const runtimeAssetsManifestPath = resolve(desktopRoot, "resources/core/.release-runtime-assets.json");

export function sha256Buffer(buffer) {
  return createHash("sha256").update(buffer).digest("hex");
}

export function sha256File(filePath) {
  return sha256Buffer(readFileSync(filePath));
}

export async function computeSourceGateHash() {
  return computeFreezeSourceGateHash(repositoryRoot);
}

export function findReleaseArtifacts(baseDir = releaseDir) {
  if (!existsSync(baseDir)) {
    throw new Error(`release directory missing: ${baseDir}`);
  }
  const names = readdirSync(baseDir);
  const exe = names.filter((name) => /^AmitiaSetup-.*-x64\.exe$/.test(name));
  const blockmap = names.filter((name) => /^AmitiaSetup-.*-x64\.exe\.blockmap$/.test(name));
  const yml = names.filter((name) => name === "latest.yml");
  if (exe.length !== 1 || blockmap.length !== 1 || yml.length !== 1) {
    throw new Error(
      `release artifacts must be unique (exe=${exe.length}, blockmap=${blockmap.length}, latest.yml=${yml.length})`,
    );
  }
  return [exe[0], blockmap[0], yml[0]].map((name) => {
    const filePath = resolve(baseDir, name);
    const stats = statSync(filePath);
    return {
      name,
      path: filePath,
      bytes: stats.size,
      sha256: sha256File(filePath),
    };
  });
}

export async function verifyPreBuildGates() {
  const freeze = await verifyFreezeManifest(repositoryRoot);
  return {
    freezeShaVerified: freeze.count,
    sourceGateHash: await computeSourceGateHash(),
  };
}

function readCoreBuildStamp() {
  if (!existsSync(coreBuildStampPath)) {
    throw new Error("AmitiaCore build stamp missing; build Core from the current frozen source before packaging");
  }
  const stamp = JSON.parse(readFileSync(coreBuildStampPath, "utf8"));
  if (!/^[0-9a-f]{64}$/.test(stamp.CORE_BUILD_SHA256 ?? "")) {
    throw new Error("AmitiaCore build stamp has an invalid CORE_BUILD_SHA256");
  }
  return stamp;
}

function readRuntimeAssetsManifest() {
  if (!existsSync(runtimeAssetsManifestPath)) {
    throw new Error("release runtime assets manifest missing; stage and verify runtime assets before packaging");
  }
  const manifest = JSON.parse(readFileSync(runtimeAssetsManifestPath, "utf8"));
  if (manifest.schemaVersion !== 1 || !Array.isArray(manifest.entries) || manifest.entries.length === 0) {
    throw new Error("release runtime assets manifest is invalid");
  }
  for (const entry of manifest.entries) {
    if (typeof entry.path !== "string" || !/^[0-9a-f]{64}$/.test(entry.sha256 ?? "") || !Number.isInteger(entry.bytes)) {
      throw new Error(`invalid runtime asset manifest entry: ${entry?.path ?? "unknown"}`);
    }
  }
  return manifest;
}

export async function writeReleaseGateStamp() {
  const packageJsonPath = resolve(desktopRoot, "package.json");
  const packageJson = JSON.parse(readFileSync(packageJsonPath, "utf8"));
  const artifacts = findReleaseArtifacts();
  const preBuild = await verifyPreBuildGates();
  const coreBuild = readCoreBuildStamp();
  const runtimeAssets = readRuntimeAssetsManifest();
  if (coreBuild.CORE_SOURCE_GATE_SHA256 !== preBuild.sourceGateHash) {
    throw new Error("AmitiaCore was not built from the currently frozen source; rebuild required");
  }
  if (runtimeAssets.sourceGateSha256 !== preBuild.sourceGateHash) {
    throw new Error("runtime assets were not staged from the currently frozen source; restage required");
  }

  const stamp = {
    schemaVersion: 4,
    createdAt: new Date().toISOString(),
    packageVersion: packageJson.version,
    desktopPetRuntimeVersion: packageJson.desktopPetRuntimeVersion,
    desktopPetRuntimeContractVersion: packageJson.desktopPetRuntimeContractVersion,
    packageJsonSha256: sha256File(packageJsonPath),
    sourceGateSha256: preBuild.sourceGateHash,
    freezeShaVerified: preBuild.freezeShaVerified,
    core: {
      sha256: coreBuild.CORE_BUILD_SHA256,
      goVersion: coreBuild.CORE_GO_VERSION,
      commit: coreBuild.CORE_COMMIT,
      sourceGateSha256: coreBuild.CORE_SOURCE_GATE_SHA256,
    },
    runtimeAssets: {
      sourceGateSha256: runtimeAssets.sourceGateSha256,
      entries: runtimeAssets.entries,
    },
    artifacts: artifacts.map(({ name, bytes, sha256 }) => ({ name, bytes, sha256 })),
  };
  writeFileSync(releaseGateStampPath, `${JSON.stringify(stamp, null, 2)}\n`, "utf8");
  return stamp;
}

export async function verifyReleaseGateStamp() {
  if (!existsSync(releaseGateStampPath)) {
    throw new Error("release gate stamp missing; rebuild with pnpm dist:win");
  }
  const stamp = JSON.parse(readFileSync(releaseGateStampPath, "utf8"));
  if (stamp.schemaVersion !== 4) {
    throw new Error(`unsupported release gate stamp schema: ${stamp.schemaVersion}`);
  }

  const packageJsonPath = resolve(desktopRoot, "package.json");
  const packageJson = JSON.parse(readFileSync(packageJsonPath, "utf8"));
  const currentPackageHash = sha256File(packageJsonPath);
  if (currentPackageHash !== stamp.packageJsonSha256) {
    throw new Error("desktop/package.json changed after release gate; rebuild required");
  }
  if (packageJson.version !== stamp.packageVersion) {
    throw new Error("desktop package version changed after release gate; rebuild required");
  }

  const preBuild = await verifyPreBuildGates();
  if (preBuild.sourceGateHash !== stamp.sourceGateSha256) {
    throw new Error("release inputs changed after release gate; rebuild required");
  }

  const runtimeAssets = readRuntimeAssetsManifest();
  if (runtimeAssets.sourceGateSha256 !== stamp.runtimeAssets?.sourceGateSha256) {
    throw new Error("runtime asset source gate differs from release gate stamp");
  }
  if (JSON.stringify(runtimeAssets.entries) !== JSON.stringify(stamp.runtimeAssets?.entries ?? [])) {
    throw new Error("runtime asset manifest changed after release gate; rebuild required");
  }

  const currentArtifacts = findReleaseArtifacts();
  const expected = new Map((stamp.artifacts ?? []).map((item) => [item.name, item]));
  for (const artifact of currentArtifacts) {
    const recorded = expected.get(artifact.name);
    if (!recorded) throw new Error(`artifact not recorded by release gate: ${artifact.name}`);
    if (recorded.bytes !== artifact.bytes || recorded.sha256 !== artifact.sha256) {
      throw new Error(`release artifact changed after gate: ${artifact.name}`);
    }
  }
  if (expected.size !== currentArtifacts.length) {
    throw new Error("release artifact set differs from release gate stamp");
  }

  return { stamp, artifacts: currentArtifacts };
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(__filename)) {
  const args = process.argv.slice(2);
  try {
    if (args.includes("--pre-build")) {
      const result = await verifyPreBuildGates();
      console.log(
        `[release-integrity] PASS: ${result.freezeShaVerified} canonical freeze entries verified; source gate ${result.sourceGateHash.slice(0, 12)}...`,
      );
    } else if (args.includes("--verify")) {
      const result = await verifyReleaseGateStamp();
      console.log(`[release-integrity] PASS: release gate stamp verified for ${result.stamp.packageVersion}`);
    } else if (args.includes("--write-stamp")) {
      const stamp = await writeReleaseGateStamp();
      console.log(`[release-integrity] PASS: release gate stamp written for ${stamp.packageVersion}`);
    } else {
      console.log("Usage: node scripts/release-integrity.mjs [--pre-build|--verify|--write-stamp]");
      process.exitCode = 1;
    }
  } catch (error) {
    console.error(`[release-integrity] FAILED: ${error instanceof Error ? error.message : String(error)}`);
    process.exitCode = 1;
  }
}
