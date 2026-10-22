import { copyFileSync, existsSync, readdirSync, rmSync, statSync } from "node:fs";
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
const buildRuntimeHostsPath = resolve(__dirname, "build-runtime-hosts.mjs");
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
  const selected = [
    ["--win", "win"],
    ["--mac", "mac"],
    ["--linux", "linux"],
  ].filter(([flag]) => args.has(flag));
  if (selected.length !== 1) {
    throw new Error("select exactly one release platform: --win, --mac, or --linux");
  }
  const platform = selected[0][1];
  const arch = args.has("--arm64") || process.env.ELECTRON_BUILDER_ARCH === "arm64" ? "arm64" : "x64";
  return {
    dirOnly: args.has("--dir"),
    platform,
    arch,
    runtimePlatform: platform === "win" ? "windows" : platform === "mac" ? "macos" : "linux",
    goos: platform === "win" ? "windows" : platform === "mac" ? "darwin" : "linux",
    goarch: arch === "arm64" ? "arm64" : "amd64",
  };
}

function resolvePackagedContents(platform, arch) {
  if (platform === "win") return resolve(releaseDir, "win-unpacked");
  if (platform === "linux") {
    const candidates = [
      resolve(releaseDir, `linux${arch === "arm64" ? "-arm64" : ""}-unpacked`),
      resolve(releaseDir, "linux-unpacked"),
    ];
    const found = candidates.find((candidate) => existsSync(candidate));
    if (!found) throw new Error(`packaged Linux output missing: ${candidates.join(", ")}`);
    return found;
  }
  const macRoot = resolve(releaseDir, `mac${arch === "arm64" ? "-arm64" : ""}`);
  if (!existsSync(macRoot)) throw new Error(`packaged macOS output missing: ${macRoot}`);
  const appName = readdirSync(macRoot).find((name) => name.endsWith(".app"));
  if (!appName) throw new Error(`packaged macOS app missing under ${macRoot}`);
  const contents = resolve(macRoot, appName, "Contents");
  if (!statSync(contents).isDirectory()) throw new Error(`packaged macOS Contents missing: ${contents}`);
  return contents;
}

function ensureLatestUpdateMetadata() {
  const names = readdirSync(releaseDir);
  if (names.includes("latest.yml")) return;
  const channelMetadata = names.find((name) => /^(alpha|beta|rc)\.yml$/.test(name));
  if (!channelMetadata) {
    throw new Error("electron-builder did not produce update metadata for the requested release");
  }
  copyFileSync(resolve(releaseDir, channelMetadata), resolve(releaseDir, "latest.yml"));
}

async function main() {
  const target = parseTarget();
  console.log(`[build-release] target=${target.platform}-${target.arch}${target.dirOnly ? " (dir)" : ""}`);

  if (existsSync(releaseDir)) rmSync(releaseDir, { recursive: true, force: true });

  console.log("[build-release] running mandatory release gate...");
  runPnpm(["run", "release:gate"]);
  await verifyPreBuildGates();

  console.log("[build-release] building shared runtime hosts...");
  run(process.execPath, [buildRuntimeHostsPath]);

  console.log("[build-release] building AmitiaCore from the frozen source...");
  run(process.execPath, [buildAmitiaCorePath], {
    env: {
      ...process.env,
      ELECTRON_BUILDER_ARCH: target.arch,
      AMITIA_CORE_GOOS: target.goos,
      AMITIA_CORE_GOARCH: target.goarch,
    },
  });

  // Recheck after the Core build. Any source mutation during build invalidates release.
  await verifyPreBuildGates();

  console.log("[build-release] verifying external runtime assets...");
  run(process.execPath, [prepareRuntimeAssetsPath], {
    env: {
      ...process.env,
      AMITIA_RELEASE_PLATFORM: target.runtimePlatform,
      AMITIA_RELEASE_ARCH: target.arch,
    },
  });
  await verifyPreBuildGates();

  const builderArgs = [cliPath, `--${target.platform}`, `--${target.arch}`, "--publish", "never"];
  if (target.dirOnly) builderArgs.push("--dir");
  builderArgs.push(`--config.directories.output=${releaseDir}`);

  console.log("[build-release] running electron-builder...");
  run(process.execPath, builderArgs, {
    env: { ...process.env, ELECTRON_BUILDER_COMPRESSION_LEVEL: compressionLevel },
  });

  if (!target.dirOnly) ensureLatestUpdateMetadata();

  const unpacked = resolvePackagedContents(target.platform, target.arch);
  run(process.execPath, [verifyPackagedPath, unpacked]);
  run(process.execPath, [verifyPackagedCorePath, unpacked]);
  run(process.execPath, [verifyPackagedRuntimeAssetsPath, unpacked]);

  // Recheck the frozen source again after packaging.
  await verifyPreBuildGates();

  if (target.dirOnly) {
    console.log(`[build-release] PASS: unpacked ${target.platform}-${target.arch} package verified`);
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
