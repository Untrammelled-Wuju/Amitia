import { createHash } from "node:crypto";
import {
  existsSync,
  readFileSync,
  readdirSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
export const desktopRoot = resolve(__dirname, "..");
export const repositoryRoot = resolve(desktopRoot, "..");
export const releaseDir = resolve(desktopRoot, "release");
export const releaseGateStampPath = resolve(releaseDir, ".desktop-pet-release-gate.json");

const SOURCE_INPUTS = [
  "desktop/package.json",
  "desktop/src",
  "desktop/scripts",
  "backend/internal/desktoppet",
  "backend/cmd/server/router.go",
  "backend/cmd/server/services.go",
  "scripts/audit/verify-desktop-pet-runtime-singletrack.mjs",
  ".github/workflows/desktop-pet.yml",
];

const EXCLUDED_SOURCE_NAMES = new Set([
  ".publish-config.json",
]);

export function sha256Buffer(buffer) {
  return createHash("sha256").update(buffer).digest("hex");
}

export function sha256File(filePath) {
  return sha256Buffer(readFileSync(filePath));
}

function collectFiles(pathValue, result) {
  const stats = statSync(pathValue);
  if (stats.isDirectory()) {
    for (const entry of readdirSync(pathValue, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      if (EXCLUDED_SOURCE_NAMES.has(entry.name)) continue;
      collectFiles(join(pathValue, entry.name), result);
    }
    return;
  }
  if (stats.isFile()) result.push(pathValue);
}

export function computeSourceGateHash() {
  const files = [];
  for (const input of SOURCE_INPUTS) {
    const absolute = resolve(repositoryRoot, input);
    if (!existsSync(absolute)) {
      throw new Error(`release source input missing: ${input}`);
    }
    collectFiles(absolute, files);
  }
  files.sort((a, b) => relative(repositoryRoot, a).localeCompare(relative(repositoryRoot, b)));

  const hash = createHash("sha256");
  for (const filePath of files) {
    const rel = relative(repositoryRoot, filePath).replace(/\\/g, "/");
    hash.update(rel, "utf8");
    hash.update("\0", "utf8");
    hash.update(readFileSync(filePath));
    hash.update("\0", "utf8");
  }
  return hash.digest("hex");
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
    const path = resolve(baseDir, name);
    const stats = statSync(path);
    return {
      name,
      path,
      bytes: stats.size,
      sha256: sha256File(path),
    };
  });
}

export function writeReleaseGateStamp() {
  const packageJsonPath = resolve(desktopRoot, "package.json");
  const packageJson = JSON.parse(readFileSync(packageJsonPath, "utf8"));
  const artifacts = findReleaseArtifacts();
  const stamp = {
    schemaVersion: 1,
    createdAt: new Date().toISOString(),
    packageVersion: packageJson.version,
    desktopPetRuntimeVersion: packageJson.desktopPetRuntimeVersion,
    desktopPetRuntimeContractVersion: packageJson.desktopPetRuntimeContractVersion,
    packageJsonSha256: sha256File(packageJsonPath),
    sourceGateSha256: computeSourceGateHash(),
    artifacts: artifacts.map(({ name, bytes, sha256 }) => ({ name, bytes, sha256 })),
  };
  writeFileSync(releaseGateStampPath, `${JSON.stringify(stamp, null, 2)}\n`, "utf8");
  return stamp;
}

export function verifyReleaseGateStamp() {
  if (!existsSync(releaseGateStampPath)) {
    throw new Error("release gate stamp missing; rebuild with pnpm dist:win");
  }
  const stamp = JSON.parse(readFileSync(releaseGateStampPath, "utf8"));
  if (stamp.schemaVersion !== 1) {
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

  const currentSourceHash = computeSourceGateHash();
  if (currentSourceHash !== stamp.sourceGateSha256) {
    throw new Error("desktop pet release inputs changed after release gate; rebuild required");
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

  return {
    stamp,
    artifacts: currentArtifacts,
  };
}
