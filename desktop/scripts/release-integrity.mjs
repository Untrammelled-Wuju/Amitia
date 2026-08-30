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
  "backend",
  "desktop/src",
  "desktop/scripts",
  "desktop/package.json",
  "desktop/pnpm-lock.yaml",
  "desktop/pnpm-workspace.yaml",
  "desktop/electron-builder.yml",
  "desktop/patches",
  "desktop/resources/config-template",
  "front/src",
  "front/package.json",
  "front/pnpm-lock.yaml",
  "mobile_app/lib",
  "mobile_app/android/app/src/main/kotlin/com/amitia/amitia_app/nativeprovider",
  "mobile_app/pubspec.yaml",
  "scripts",
  ".github/workflows",
  ".tool-versions",
  "DESKTOP_PET_FINALIZATION_SHA256SUMS.txt",
];

const EXCLUDED_SOURCE_NAMES = new Set([
  ".publish-config.json",
  "node_modules",
  "dist",
  "build",
  "dist-types",
  ".dart_tool",
  ".gradle",
  "__pycache__",
  ".cache",
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
      if (entry.name === "node_modules" || entry.name === "dist" || entry.name === "build") continue;
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

export function verifyPreBuildGates() {
  const freezeShaPath = resolve(repositoryRoot, "DESKTOP_PET_FINALIZATION_SHA256SUMS.txt");
  if (!existsSync(freezeShaPath)) {
    throw new Error("DESKTOP_PET_FINALIZATION_SHA256SUMS.txt missing");
  }

  const freezeLines = readFileSync(freezeShaPath, "utf8").split("\n");
  let ok = 0;
  let fail = 0;
  for (const line of freezeLines) {
    if (line.trim() === "") continue;
    const expectedHash = line.substring(0, 64);
    const relativePath = line.substring(66);
    const fullPath = resolve(repositoryRoot, relativePath);
    if (!existsSync(fullPath)) {
      throw new Error(`freeze file missing: ${relativePath}`);
    }
    const actualHash = sha256File(fullPath);
    if (actualHash !== expectedHash) {
      throw new Error(`freeze SHA mismatch: ${relativePath}`);
    }
    ok++;
  }

  return {
    freezeShaVerified: ok,
    sourceGateHash: computeSourceGateHash(),
  };
}

export function writeReleaseGateStamp() {
  const packageJsonPath = resolve(desktopRoot, "package.json");
  const packageJson = JSON.parse(readFileSync(packageJsonPath, "utf8"));
  const artifacts = findReleaseArtifacts();
  const preBuild = verifyPreBuildGates();
  const stamp = {
    schemaVersion: 2,
    createdAt: new Date().toISOString(),
    packageVersion: packageJson.version,
    desktopPetRuntimeVersion: packageJson.desktopPetRuntimeVersion,
    desktopPetRuntimeContractVersion: packageJson.desktopPetRuntimeContractVersion,
    packageJsonSha256: sha256File(packageJsonPath),
    sourceGateSha256: preBuild.sourceGateHash,
    freezeShaVerified: preBuild.freezeShaVerified,
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
  if (stamp.schemaVersion !== 1 && stamp.schemaVersion !== 2) {
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

if (import.meta.url === `file://${process.argv[1]}`) {
  const args = process.argv.slice(2);
  if (args.includes("--pre-build")) {
    const result = verifyPreBuildGates();
    console.log(`[release-integrity] Pre-build gates passed: freeze SHA verified (${result.freezeShaVerified} files), source gate hash computed`);
    process.exit(0);
  } else if (args.includes("--verify")) {
    const result = verifyReleaseGateStamp();
    console.log(`[release-integrity] Release gate stamp verified: ${result.stamp.packageVersion}`);
    process.exit(0);
  } else if (args.includes("--write-stamp")) {
    const stamp = writeReleaseGateStamp();
    console.log(`[release-integrity] Release gate stamp written: ${stamp.packageVersion}`);
    process.exit(0);
  } else {
    console.log("Usage: node scripts/release-integrity.mjs [--pre-build|--verify|--write-stamp]");
    process.exit(1);
  }
}
