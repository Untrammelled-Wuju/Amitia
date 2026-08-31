import { createHash } from "node:crypto";
import { existsSync, readFileSync, writeFileSync, rmSync, mkdirSync } from "node:fs";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { computeFreezeSourceGateHash } from "../../scripts/lib/freeze-scope.mjs";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const coreDir = resolve(desktopRoot, "resources/core");
const outputPath = resolve(coreDir, "AmitiaCore.exe");
const stampPath = resolve(coreDir, ".amitiacore-build.json");

function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? repositoryRoot,
    stdio: options.stdio ?? "inherit",
    encoding: options.encoding,
    env: options.env ?? process.env,
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.signal) throw new Error(`${command} terminated by ${result.signal}`);
  if ((result.status ?? 1) !== 0) {
    throw new Error(`${command} exited with code ${result.status ?? 1}`);
  }
  return result;
}

function resolveGoBin() {
  if (process.env.GO_BIN) return process.env.GO_BIN;
  return process.platform === "win32" ? "go.exe" : "go";
}

async function main() {
  const goBin = resolveGoBin();
  const backendDir = resolve(repositoryRoot, "backend");

  const sourceGateBefore = await computeFreezeSourceGateHash(repositoryRoot);
  console.log(`[build-amitiacore] Source gate: ${sourceGateBefore}`);

  mkdirSync(coreDir, { recursive: true });
  if (existsSync(outputPath)) rmSync(outputPath, { force: true });
  if (existsSync(stampPath)) rmSync(stampPath, { force: true });

  const goEnv = {
    ...process.env,
    GOOS: "windows",
    GOARCH: process.env.ELECTRON_BUILDER_ARCH === "arm64" ? "arm64" : "amd64",
    CGO_ENABLED: process.env.CGO_ENABLED ?? "0",
  };

  console.log(`[build-amitiacore] Building Windows ${goEnv.GOARCH} Core from ./cmd/server...`);
  run(goBin, ["build", "-trimpath", "-o", outputPath, "./cmd/server"], {
    cwd: backendDir,
    env: goEnv,
  });

  if (!existsSync(outputPath)) {
    throw new Error("AmitiaCore.exe was not produced");
  }

  const sourceGateAfter = await computeFreezeSourceGateHash(repositoryRoot);
  if (sourceGateAfter !== sourceGateBefore) {
    rmSync(outputPath, { force: true });
    throw new Error("frozen source changed while AmitiaCore was being built");
  }

  const goVersionResult = run(goBin, ["version"], { stdio: "pipe", encoding: "utf8" });
  const goVersion = goVersionResult.stdout?.trim() || "unknown";

  let commit = "unknown";
  const commitResult = spawnSync("git", ["rev-parse", "--short", "HEAD"], {
    cwd: repositoryRoot,
    encoding: "utf8",
    shell: false,
  });
  if (commitResult.status === 0 && commitResult.stdout?.trim()) {
    commit = commitResult.stdout.trim();
  }

  const stamp = {
    schemaVersion: 2,
    CORE_BUILD_SHA256: sha256File(outputPath),
    CORE_SOURCE_GATE_SHA256: sourceGateBefore,
    CORE_BUILD_TIMESTAMP: new Date().toISOString(),
    CORE_GO_VERSION: goVersion,
    CORE_COMMIT: commit,
    CORE_GOOS: goEnv.GOOS,
    CORE_GOARCH: goEnv.GOARCH,
    CORE_CGO_ENABLED: goEnv.CGO_ENABLED,
  };

  writeFileSync(stampPath, `${JSON.stringify(stamp, null, 2)}\n`, "utf8");
  console.log(`[build-amitiacore] PASS: ${stamp.CORE_BUILD_SHA256}`);
}

main().catch((error) => {
  console.error(`[build-amitiacore] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
