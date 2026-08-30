import { existsSync, readFileSync, writeFileSync, rmSync, copyFileSync } from "node:fs";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const coreDir = resolve(desktopRoot, "resources/core");
const outputPath = resolve(coreDir, "AmitiaCore.exe");
const stampPath = resolve(coreDir, ".amitiacore-build.json");

function sha256File(filePath) {
  const data = readFileSync(filePath);
  return createHash("sha256").update(data).digest("hex");
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? repositoryRoot,
    stdio: "inherit",
    env: options.env ?? process.env,
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.symbol) throw new Error(`${command} terminated by ${result.signal}`);
  if ((result.status ?? 1) !== 0) {
    throw new Error(`${command} exited with code ${result.status ?? 1}`);
  }
}

function main() {
  const goBin = process.env.GO_BIN || "C:\\Code\\Go\\bin\\go.exe";
  const backendDir = resolve(repositoryRoot, "backend");

  console.log("[build-amitiacore] Removing existing AmitiaCore.exe...");
  if (existsSync(outputPath)) {
    rmSync(outputPath, { force: true });
  }
  if (existsSync(stampPath)) {
    rmSync(stampPath, { force: true });
  }

  console.log("[build-amitiacore] Building Go backend server.exe...");
  run(goBin, ["build", "-o", outputPath, "./cmd/server"], { cwd: backendDir });

  if (!existsSync(outputPath)) {
    throw new Error("[build-amitiacore] Build failed: AmitiaCore.exe not produced");
  }

  const buildSha256 = sha256File(outputPath);
  const goVersionResult = spawnSync(goBin, ["version"], { encoding: "utf8", shell: false });
  const goVersion = goVersionResult.stdout?.trim() || "unknown";

  const commitResult = spawnSync("git", ["rev-parse", "--short", "HEAD"], {
    cwd: repositoryRoot,
    encoding: "utf8",
    shell: false,
  });
  const commit = commitResult.stdout?.trim() || "unknown";

  const stamp = {
    CORE_BUILD_SHA256: buildSha256,
    CORE_BUILD_TIMESTAMP: new Date().toISOString(),
    CORE_GO_VERSION: goVersion,
    CORE_COMMIT: commit,
  };

  writeFileSync(stampPath, JSON.stringify(stamp, null, 2) + "\n", "utf8");

  console.log("[build-amitiacore] AmitiaCore.exe built successfully");
  console.log(`  SHA256: ${buildSha256}`);
  console.log(`  Go version: ${goVersion}`);
  console.log(`  Commit: ${commit}`);
  console.log(`  Stamp written: ${stampPath}`);
}

try {
  main();
} catch (error) {
  console.error(`[build-amitiacore] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
}
