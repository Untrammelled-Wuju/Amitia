import { promises as fs, rmSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import os from "node:os";
import {
  collectFreezeEntries,
  parseFreezeManifest,
  serializeFreezeEntries,
  verifyFreezeManifest,
} from "./lib/freeze-scope.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const workspaceRoot = path.resolve(scriptDir, "..");

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? workspaceRoot,
    stdio: options.stdio ?? "inherit",
    encoding: options.encoding,
    env: options.env ?? process.env,
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.signal) throw new Error(`${command} terminated by ${result.signal}`);
  if ((result.status ?? 1) !== 0) throw new Error(`${command} exited with code ${result.status ?? 1}`);
  return result;
}

function runPnpm(args, cwd) {
  const npmExecPath = process.env.npm_execpath;
  if (npmExecPath) {
    run(process.execPath, [npmExecPath, ...args], { cwd });
    return;
  }
  const command = process.platform === "win32" ? "pnpm.cmd" : "pnpm";
  const probe = spawnSync(command, ["--version"], { cwd, stdio: "ignore", shell: false });
  if (!probe.error && probe.status === 0) {
    run(command, args, { cwd });
    return;
  }
  run(process.platform === "win32" ? "corepack.cmd" : "corepack", ["pnpm", ...args], { cwd });
}

async function findExtractedRoot(extractDir) {
  const entries = await fs.readdir(extractDir, { withFileTypes: true });
  const directories = entries.filter((entry) => entry.isDirectory());
  if (directories.length !== 1) {
    throw new Error(`archive must contain exactly one top-level directory; found ${directories.length}`);
  }
  return path.join(extractDir, directories[0].name);
}

function compareEntrySets(workspaceEntries, archiveEntries) {
  const workspaceMap = new Map(workspaceEntries.map((entry) => [entry.relativePath, entry.hash]));
  const archiveMap = new Map(archiveEntries.map((entry) => [entry.relativePath, entry.hash]));
  const missing = [];
  const extra = [];
  const changed = [];

  for (const [relativePath, hash] of workspaceMap) {
    if (!archiveMap.has(relativePath)) missing.push(relativePath);
    else if (archiveMap.get(relativePath) !== hash) changed.push(relativePath);
  }
  for (const relativePath of archiveMap.keys()) {
    if (!workspaceMap.has(relativePath)) extra.push(relativePath);
  }
  return { missing, extra, changed, matched: workspaceMap.size - missing.length - changed.length };
}

async function verifyManifestFileMatches(root, entries) {
  const text = await fs.readFile(path.join(root, "DESKTOP_PET_FINALIZATION_SHA256SUMS.txt"), "utf8");
  const parsed = parseFreezeManifest(text);
  if (serializeFreezeEntries(parsed) !== serializeFreezeEntries(entries)) {
    throw new Error(`freeze manifest in ${root} does not exactly match its canonical scope`);
  }
}

function runArchiveSelfGates(archiveRoot) {
  const node = process.execPath;
  const commands = [
    ["scripts/build-freeze-scope.mjs", "--verify"],
    ["desktop/scripts/release-integrity.mjs", "--pre-build"],
    ["scripts/audit/verify-source-hygiene.mjs"],
    ["scripts/audit/verify-desktop-pet-runtime-singletrack.mjs"],
    ["desktop/scripts/verify-pet-player-singleton.mjs"],
    ["desktop/scripts/verify-desktop-pet-runtime-lifecycle-semantics.mjs"],
    ["desktop/scripts/verify-desktop-pet-finalization.mjs"],
    ["desktop/scripts/verify-cloud-device-execution-finalization.mjs"],
    ["desktop/scripts/verify-desktop-pet-device-agent-freeze-gate.mjs"],
  ];
  for (const args of commands) {
    run(node, args, { cwd: archiveRoot, env: { ...process.env, DESKTOP_PET_ARCHIVE_ROOT: "", AMITIA_ARCHIVE_SELF_VERIFY: "1" } });
  }
}

function runCleanBuild(archiveRoot) {
  const goBin = process.env.GO_BIN || (process.platform === "win32" ? "go.exe" : "go");
  const backendRoot = path.join(archiveRoot, "backend");
  const frontRoot = path.join(archiveRoot, "front");
  const desktopRoot = path.join(archiveRoot, "desktop");

  console.log("[archive] clean backend build/test gate...");
  run(goBin, ["mod", "verify"], { cwd: backendRoot });
  run(goBin, ["vet", "./..."], { cwd: backendRoot });
  run(goBin, ["test", "./...", "-count=1"], { cwd: backendRoot });
  const buildOutput = path.join(os.tmpdir(), process.platform === "win32" ? "amitia-archive-server.exe" : "amitia-archive-server");
  try {
    run(goBin, ["build", "-o", buildOutput, "./cmd/server"], { cwd: backendRoot });
  } finally {
    rmSync(buildOutput, { force: true });
  }

  console.log("[archive] clean front install/build gate...");
  runPnpm(["install", "--frozen-lockfile"], frontRoot);
  runPnpm(["typecheck"], frontRoot);
  runPnpm(["test"], frontRoot);
  runPnpm(["build"], frontRoot);

  console.log("[archive] clean desktop install/build gate...");
  runPnpm(["install", "--frozen-lockfile"], desktopRoot);
  runPnpm(["typecheck"], desktopRoot);
  runPnpm(["test"], desktopRoot);
  runPnpm(["build"], desktopRoot);
}

async function main() {
  const args = process.argv.slice(2);
  const archiveArg = args.find((arg) => !arg.startsWith("--"));
  if (!archiveArg) {
    throw new Error("Usage: node scripts/verify-source-archive.mjs <archive.tar.gz> [--clean-build]");
  }
  const cleanBuild = args.includes("--clean-build");
  const archivePath = path.resolve(archiveArg);

  console.log(`[archive] workspace=${workspaceRoot}`);
  console.log(`[archive] archive=${archivePath}`);

  const workspaceVerification = await verifyFreezeManifest(workspaceRoot);
  const workspaceEntries = workspaceVerification.entries;
  await verifyManifestFileMatches(workspaceRoot, workspaceEntries);

  const extractDir = await fs.mkdtemp(path.join(os.tmpdir(), "amitia-archive-verify-"));
  try {
    run("tar", ["-xzf", archivePath, "-C", extractDir]);
    const archiveRoot = await findExtractedRoot(extractDir);
    console.log(`[archive] extractedRoot=${archiveRoot}`);

    const archiveEntries = await collectFreezeEntries(archiveRoot);
    await verifyManifestFileMatches(archiveRoot, archiveEntries);
    const comparison = compareEntrySets(workspaceEntries, archiveEntries);

    console.log(`[archive] workspace freeze files=${workspaceEntries.length}`);
    console.log(`[archive] archive freeze files=${archiveEntries.length}`);
    console.log(`[archive] SHA matched=${comparison.matched}/${workspaceEntries.length}`);

    if (comparison.missing.length || comparison.extra.length || comparison.changed.length) {
      for (const item of comparison.missing) console.error(`MISSING_IN_ARCHIVE: ${item}`);
      for (const item of comparison.extra) console.error(`EXTRA_IN_ARCHIVE_SCOPE: ${item}`);
      for (const item of comparison.changed) console.error(`SHA_CHANGED_IN_ARCHIVE: ${item}`);
      throw new Error(
        `workspace/archive frozen source mismatch (missing=${comparison.missing.length}, extra=${comparison.extra.length}, changed=${comparison.changed.length})`,
      );
    }

    console.log("[archive] running archive-local self gates...");
    runArchiveSelfGates(archiveRoot);

    if (cleanBuild) runCleanBuild(archiveRoot);

    console.log(`[archive] PASS: ${workspaceEntries.length} frozen files are identical and archive-local gates passed${cleanBuild ? "; clean build passed" : ""}`);
  } finally {
    await fs.rm(extractDir, { recursive: true, force: true }).catch(() => {});
  }
}

main().catch((error) => {
  console.error(`[archive] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
