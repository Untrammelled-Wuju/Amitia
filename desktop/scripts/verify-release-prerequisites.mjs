import { existsSync, rmSync } from "node:fs";
import os from "node:os";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const backendRoot = resolve(repositoryRoot, "backend");
const frontRoot = resolve(repositoryRoot, "front");

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? repositoryRoot,
    stdio: "inherit",
    env: options.env ?? process.env,
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.signal) throw new Error(`${command} terminated by ${result.signal}`);
  if ((result.status ?? 1) !== 0) throw new Error(`${command} exited with code ${result.status ?? 1}`);
}

function runPnpm(args, cwd = repositoryRoot) {
  const npmExecPath = process.env.npm_execpath;
  if (npmExecPath) {
    run(process.execPath, [npmExecPath, ...args], { cwd });
    return;
  }
  run(process.platform === "win32" ? "pnpm.cmd" : "pnpm", args, { cwd });
}

function main() {
  const goBin = process.env.GO_BIN || (process.platform === "win32" ? "go.exe" : "go");

  console.log("[release-prerequisites] backend: go mod verify");
  run(goBin, ["mod", "verify"], { cwd: backendRoot });

  console.log("[release-prerequisites] backend: go vet ./...");
  run(goBin, ["vet", "./..."], { cwd: backendRoot });

  console.log("[release-prerequisites] backend: go test ./...");
  run(goBin, ["test", "./...", "-count=1"], { cwd: backendRoot });

  console.log("[release-prerequisites] backend: go test -race ./internal/desktoppet/...");
  run(goBin, ["test", "-race", "./internal/desktoppet/...", "-count=1"], { cwd: backendRoot });

  console.log("[release-prerequisites] backend: go build ./cmd/server");
  const buildOutput = resolve(os.tmpdir(), process.platform === "win32" ? "amitia-release-gate-server.exe" : "amitia-release-gate-server");
  rmSync(buildOutput, { force: true });
  try {
    run(goBin, ["build", "-o", buildOutput, "./cmd/server"], { cwd: backendRoot });
  } finally {
    rmSync(buildOutput, { force: true });
  }

  if (!existsSync(resolve(frontRoot, "node_modules"))) {
    throw new Error("front/node_modules is missing; run `pnpm --dir front install --frozen-lockfile` before release:gate");
  }

  console.log("[release-prerequisites] front: typecheck");
  runPnpm(["--dir", frontRoot, "typecheck"]);

  console.log("[release-prerequisites] front: test");
  runPnpm(["--dir", frontRoot, "test"]);

  console.log("[release-prerequisites] front: build");
  runPnpm(["--dir", frontRoot, "build"]);

  console.log("[release-prerequisites] PASS");
}

try {
  main();
} catch (error) {
  console.error(`[release-prerequisites] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
}
