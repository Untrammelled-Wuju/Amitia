import { rmSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import {
  desktopRoot,
  releaseDir,
  writeReleaseGateStamp,
} from "./release-integrity.mjs";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const cliPath = fileURLToPath(new URL("../node_modules/electron-builder/cli.js", import.meta.url));
const verifyPackagedPath = resolve(__dirname, "verify-packaged-desktop-pet.mjs");
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
  if ((result.status ?? 1) !== 0) {
    throw new Error(`${command} exited with code ${result.status ?? 1}`);
  }
}

function runPnpm(args) {
  const npmExecPath = process.env.npm_execpath;
  if (npmExecPath) {
    run(process.execPath, [npmExecPath, ...args]);
    return;
  }
  run(process.platform === "win32" ? "pnpm.cmd" : "pnpm", args);
}

function main() {
  console.log("[build-release] running mandatory desktop pet release gate...");
  runPnpm(["run", "release:gate"]);

  // Never package over an old release directory. This prevents stale installers
  // or latest.yml files from being accidentally selected for publishing.
  if (existsSync(releaseDir)) {
    rmSync(releaseDir, { recursive: true, force: true });
  }

  const builderArgs = [
    cliPath,
    "--win",
    "--x64",
    "--publish",
    "never",
    ...process.argv.slice(2),
  ];
  console.log("[build-release] building Windows package...");
  run(process.execPath, builderArgs, {
    env: {
      ...process.env,
      ELECTRON_BUILDER_COMPRESSION_LEVEL: compressionLevel,
    },
  });

  const unpacked = resolve(releaseDir, "win-unpacked");
  console.log("[build-release] verifying packaged desktop pet...");
  run(process.execPath, [verifyPackagedPath, unpacked]);

  const stamp = writeReleaseGateStamp();
  console.log(
    `[build-release] PASSED: release gate stamp written for ${stamp.packageVersion} (${stamp.sourceGateSha256.slice(0, 12)}...)`,
  );
}

try {
  main();
} catch (error) {
  console.error(`[build-release] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
}
