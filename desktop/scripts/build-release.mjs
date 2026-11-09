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
const buildMarketplacePluginsPath = resolve(__dirname, "../../scripts/build-marketplace-plugins.mjs");
const verifyPluginMarketPackagePath = resolve(__dirname, "../../plugins-builtin/plugin-market/scripts/verify-package.mjs");
const testPluginMarketRuntimePath = resolve(__dirname, "../../plugins-builtin/plugin-market/scripts/test-runtime.mjs");
const testRuntimeInsightsPath = resolve(__dirname, "../../plugins-builtin/runtime-insights/scripts/test-runtime.mjs");
const testWorkspaceRuntimePath = resolve(__dirname, "../../plugins-builtin/workspace/scripts/test-runtime.mjs");
const testChatImportRuntimePath = resolve(__dirname, "../../plugins-builtin/chat-import/scripts/test-runtime.mjs");
const testStorageRuntimePath = resolve(__dirname, "../../plugins-builtin/storage/scripts/test-runtime.mjs");
const testChatLogsRuntimePath = resolve(__dirname, "../../plugins-builtin/chat-logs/scripts/test-runtime.mjs");
const testCreativeWorkshopHubRuntimePath = resolve(__dirname, "../../plugins-builtin/creative-workshop-hub/scripts/test-runtime.mjs");
const testGameCenterHubRuntimePath = resolve(__dirname, "../../plugins-builtin/game-center-hub/scripts/test-runtime.mjs");
const testChannelsHubRuntimePath = resolve(__dirname, "../../plugins-builtin/channels-hub/scripts/test-runtime.mjs");
const testUserSpaceHubRuntimePath = resolve(__dirname, "../../plugins-builtin/user-space-hub/scripts/test-runtime.mjs");
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

function ensureUpdateMetadata(platform) {
  const names = readdirSync(releaseDir);
  if (platform === "mac") {
    if (names.includes("latest-mac.yml")) return;
    throw new Error("electron-builder did not produce macOS update metadata");
  }
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

  console.log("[build-release] building bundled plugins...");
  for (const extensionID of ["com.amitia/plugin-market", "com.amitia/runtime-insights", "com.amitia/workspace", "com.amitia/chat-import", "com.amitia/storage", "com.amitia/chat-logs", "com.amitia/creative-workshop-hub", "com.amitia/game-center-hub", "com.amitia/channels-hub", "com.amitia/user-space-hub"]) {
    run(process.execPath, [buildMarketplacePluginsPath, "--build"], {
      env: { ...process.env, AMITIA_PLUGIN_FILTER: extensionID },
    });
  }
  run(process.execPath, [testPluginMarketRuntimePath]);
  run(process.execPath, [testRuntimeInsightsPath]);
  run(process.execPath, [testWorkspaceRuntimePath]);
  run(process.execPath, [testChatImportRuntimePath]);
  run(process.execPath, [testStorageRuntimePath]);
  run(process.execPath, [testChatLogsRuntimePath]);
  run(process.execPath, [testCreativeWorkshopHubRuntimePath]);
  run(process.execPath, [testGameCenterHubRuntimePath]);
  run(process.execPath, [testChannelsHubRuntimePath]);
  run(process.execPath, [testUserSpaceHubRuntimePath]);
  run(process.execPath, [verifyPluginMarketPackagePath]);

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

  if (!target.dirOnly) ensureUpdateMetadata(target.platform);

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

  const stamp = await writeReleaseGateStamp(target.platform);
  await verifyReleaseGateStamp(target.platform);
  run(process.execPath, [generateReleaseReportPath]);
  console.log(
    `[build-release] PASS: ${stamp.packageVersion}, source=${stamp.sourceGateSha256.slice(0, 12)}..., core=${stamp.core.sha256.slice(0, 12)}...`,
  );
}

main().catch((error) => {
  console.error(`[build-release] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
