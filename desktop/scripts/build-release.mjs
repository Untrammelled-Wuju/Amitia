import { existsSync, rmSync } from "node:fs";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import {
  desktopRoot,
  releaseDir,
  verifyPreBuildGates,
  verifyReleaseGateStamp,
  writeReleaseGateStamp,
} from "./release-integrity.mjs";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const cliPath = fileURLToPath(new URL("../node_modules/electron-builder/cli.js", import.meta.url));
const verifyPackagedPath = resolve(__dirname, "verify-packaged-desktop-pet.mjs");
const verifyPackagedCorePath = resolve(__dirname, "verify-packaged-core.mjs");
const buildAmitiaCorePath = resolve(__dirname, "build-amitiacore.mjs");
const generateReleaseReportPath = resolve(__dirname, "generate-release-report.mjs");
const prepareRuntimeAssetsPath = resolve(__dirname, "prepare-release-runtime-assets.mjs");
const verifyPackagedRuntimeAssetsPath = resolve(__dirname, "verify-packaged-runtime-assets.mjs");
const compressionLevel = process.env.ELECTRON_BUILDER_COMPRESSION_LEVEL || "5";

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? desktopRoot,
    stdio: "inherit",
    env: options.env ?? process.env,
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.signal) throw new Error(`${command} terminated by ${result.signal}`);
  if ((result.status ?? 1) !== 0) throw new Error(`${command} exited with code ${result.status ?? 1}`);
}

function runPnpm(args) {
  const npmExecPath = process.env.npm_execpath;
  if (npmExecPath) {
    run(process.execPath, [npmExecPath, ...args]);
    return;
  }
  run(process.platform === "win32" ? "pnpm.cmd" : "pnpm", args);
}

function parseTarget() {
  const args = new Set(process.argv.slice(2));
  if (args.has("--linux") || args.has("--mac")) {
    throw new Error(
      "this release pipeline is Windows-only because bundled Node/Qdrant/SurrealDB assets are Windows artifacts; Linux/macOS release targets are intentionally not exposed",
    );
  }
  if (args.has("--arm64") || process.env.ELECTRON_BUILDER_ARCH === "arm64") {
    throw new Error("Windows arm64 packaging is not frozen yet; supported final target is win-x64");
  }
  return { dirOnly: args.has("--dir"), platform: "win", arch: "x64" };
}

async function main() {
  const target = parseTarget();
  console.log(`[build-release] target=${target.platform}-${target.arch}${target.dirOnly ? " (dir)" : ""}`);

  console.log("[build-release] running mandatory release gate...");
  runPnpm(["run", "release:gate"]);
  await verifyPreBuildGates();

  if (existsSync(releaseDir)) rmSync(releaseDir, { recursive: true, force: true });

  console.log("[build-release] building AmitiaCore from the frozen source...");
  run(process.execPath, [buildAmitiaCorePath], {
    env: { ...process.env, ELECTRON_BUILDER_ARCH: target.arch },
  });

  // Recheck after the Core build. Any source mutation during build invalidates release.
  await verifyPreBuildGates();

  console.log("[build-release] staging frozen sidecars and verifying external runtime assets...");
  run(process.execPath, [prepareRuntimeAssetsPath]);
  await verifyPreBuildGates();

  const builderArgs = [cliPath, "--win", "--x64", "--publish", "never"];
  if (target.dirOnly) builderArgs.push("--dir");

  console.log("[build-release] running electron-builder...");
  run(process.execPath, builderArgs, {
    env: { ...process.env, ELECTRON_BUILDER_COMPRESSION_LEVEL: compressionLevel },
  });

  const unpacked = resolve(releaseDir, "win-unpacked");
  run(process.execPath, [verifyPackagedPath, unpacked]);
  run(process.execPath, [verifyPackagedCorePath, unpacked]);
  run(process.execPath, [verifyPackagedRuntimeAssetsPath, unpacked]);

  // Recheck the frozen source again after packaging.
  await verifyPreBuildGates();

  if (target.dirOnly) {
    console.log("[build-release] PASS: unpacked win-x64 package verified");
    return;
  }

  const stamp = await writeReleaseGateStamp();
  await verifyReleaseGateStamp();
  run(process.execPath, [generateReleaseReportPath]);
  console.log(
    `[build-release] PASS: ${stamp.packageVersion}, source=${stamp.sourceGateSha256.slice(0, 12)}..., core=${stamp.core.sha256.slice(0, 12)}...`,
  );
}

main().catch((error) => {
  console.error(`[build-release] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
