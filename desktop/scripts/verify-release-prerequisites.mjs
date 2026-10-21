import { existsSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import { basename, extname, join, resolve, sep } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const backendRoot = resolve(repositoryRoot, "backend");
const frontRoot = resolve(repositoryRoot, "front");

const baseRef = (process.env.AMITIA_RELEASE_BASE_REF || "").trim();
const explicitGoPackages = splitList(process.env.AMITIA_RELEASE_GO_TEST_PACKAGES);
const explicitFrontTests = splitList(process.env.AMITIA_RELEASE_FRONT_TEST_FILES);
const fullGoTests = process.env.AMITIA_RELEASE_FULL_GO_TESTS === "1";
const fullFrontTests = process.env.AMITIA_RELEASE_FULL_FRONT_TESTS === "1";
const skipFrontTests = process.env.AMITIA_RELEASE_SKIP_FRONT_TESTS === "1";
const forcedRaceChecks = process.env.AMITIA_RELEASE_RACE_CHECKS === "1";

function splitList(value) {
  return (value || "")
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

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

function gitBinary() {
  if (process.env.GIT_BIN) return process.env.GIT_BIN;
  if (process.platform !== "win32") return "git";
  const configured = "C:\\Code\\Git\\Git\\bin\\git.exe";
  return existsSync(configured) ? configured : "git.exe";
}

function testNodeBinary() {
  if (process.env.AMITIA_TEST_NODE) return process.env.AMITIA_TEST_NODE;
  const bundled = resolve(desktopRoot, "resources", "core", "node", process.platform === "win32" ? "node.exe" : "node");
  return existsSync(bundled) ? bundled : "";
}

function runGoTest(goBin, packages, options = {}) {
  const extraArgs = options.extraArgs ?? [];
  const result = spawnSync(goBin, ["test", ...extraArgs, ...packages, "-count=1", "-json"], {
    cwd: options.cwd ?? backendRoot,
    env: options.env ?? process.env,
    encoding: "utf8",
    maxBuffer: 512 * 1024 * 1024,
    shell: false,
  });
  if (result.error) throw result.error;
  const stdout = result.stdout ?? "";
  const failures = [];
  const passed = new Set();
  for (const line of stdout.split(/\r?\n/)) {
    if (!line.startsWith("{")) continue;
    let event;
    try {
      event = JSON.parse(line);
    } catch {
      continue;
    }
    if (event.Action === "fail") {
      const name = event.Test ? `${event.Package}/${event.Test}` : event.Package;
      if (name && !failures.includes(name)) failures.push(name);
    } else if (event.Action === "pass" && !event.Test && event.Package) {
      passed.add(event.Package);
    }
  }
  for (const name of [...passed].sort()) console.log(`ok  ${name}`);
  if (failures.length > 0) {
    const logPath = resolve(os.tmpdir(), "amitia-release-gotest.log");
    writeFileSync(logPath, stdout, "utf8");
    throw new Error(`go test reported failures: ${failures.join(", ")} (full log: ${logPath})`);
  }
  if (passed.size === 0) {
    const logPath = resolve(os.tmpdir(), "amitia-release-gotest.log");
    writeFileSync(logPath, stdout, "utf8");
    throw new Error(`go test produced no package result (exit ${result.status ?? 1}, full log: ${logPath})`);
  }
  console.log(`[release-prerequisites] go test: ${passed.size} package(s) passed`);
}

function gitLines(args, options = {}) {
  const result = spawnSync(gitBinary(), ["-C", repositoryRoot, ...args], { encoding: "utf8" });
  if (result.error || result.status !== 0) return null;
  const lines = result.stdout.split(/\r?\n/).filter((line) => line.trim().length > 0);
  return options.trim === false ? lines : lines.map((line) => line.trim());
}

function changedFiles() {
  const files = new Set();
  const status = gitLines(["status", "--porcelain", "--untracked-files=all"], { trim: false });
  if (status === null) {
    console.log("[release-prerequisites] warning: cannot read working tree status");
  } else {
    for (const line of status) {
      const raw = line.slice(3).trim().replace(/^"|"$/g, "");
      files.add(raw.includes(" -> ") ? raw.split(" -> ").pop().trim() : raw);
    }
  }
  if (baseRef) {
    const diff = gitLines(["diff", "--name-only", `${baseRef}...HEAD`]);
    if (diff === null) {
      console.log(`[release-prerequisites] warning: cannot diff against ${baseRef}`);
    } else {
      for (const file of diff) files.add(file);
    }
  }
  return [...files];
}

function changedGoPackages(files) {
  const packages = new Set();
  let moduleFileChanged = false;
  for (const file of files) {
    if (file === "backend/go.mod" || file === "backend/go.sum") {
      moduleFileChanged = true;
      continue;
    }
    if (!file.startsWith("backend/") || extname(file) !== ".go") continue;
    const directory = resolve(backendRoot, file.slice("backend/".length), "..");
    if (directory !== backendRoot && !directory.startsWith(backendRoot + sep)) continue;
    const relative = directory.slice(backendRoot.length + 1).split(sep).join("/");
    packages.add(relative ? `./${relative}` : ".");
  }
  if (moduleFileChanged) {
    console.log("[release-prerequisites] notice: backend/go.mod or go.sum changed; Go test scope still derives from changed Go files");
  }
  return [...packages].sort();
}

function runnableGoPackages(packages, goBin) {
  return packages.filter((pkg) => {
    const result = spawnSync(goBin, ["list", "-e", "-f", "{{if .Error}}{{.Error.Err}}{{else}}{{.Name}}{{end}}", pkg], {
      cwd: backendRoot,
      encoding: "utf8",
      shell: false,
    });
    const output = `${result.stdout || ""}${result.stderr || ""}`.trim();
    if (result.status !== 0) return false;
    if (!output || output.startsWith("no Go files") || output.includes("build constraints exclude all Go files")) {
      console.log(`[release-prerequisites] backend: skipping non-runnable package ${pkg}`);
      return false;
    }
    return true;
  });
}

function frontTestCandidates() {
  const root = join(frontRoot, "src");
  const found = [];
  const walk = (directory) => {
    let entries = [];
    try {
      entries = readdirSync(directory, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      const full = join(directory, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (/\.(test|spec)\.tsx?$/.test(entry.name)) found.push(full);
    }
  };
  if (existsSync(root)) walk(root);
  return found;
}

function changedFrontTests(files) {
  const candidates = frontTestCandidates();
  const targets = new Set();
  for (const file of files) {
    if (!file.startsWith("front/")) continue;
    const name = basename(file);
    if (/\.(test|spec)\.tsx?$/.test(name)) {
      const match = candidates.find((candidate) => basename(candidate) === name);
      if (match) targets.add(match);
      continue;
    }
    if (!/\.tsx?$/.test(name)) continue;
    const stem = name.replace(/\.tsx?$/, "");
    for (const candidate of candidates) {
      const candidateName = basename(candidate);
      if (
        candidateName === `${stem}.test.ts` ||
        candidateName === `${stem}.test.tsx` ||
        candidateName === `${stem}.spec.ts` ||
        candidateName === `${stem}.spec.tsx`
      ) {
        targets.add(candidate);
      }
    }
  }
  return [...targets].sort();
}

function main() {
  const goBin = process.env.GO_BIN || (process.platform === "win32" ? "go.exe" : "go");
  const testNode = testNodeBinary();
  const backendTestEnv = testNode ? { ...process.env, AMITIA_TEST_NODE: testNode } : process.env;
  const files = changedFiles();
  const derivedGoPackages = explicitGoPackages.length > 0
    ? explicitGoPackages
    : fullGoTests
      ? ["./..."]
      : changedGoPackages(files);
  const goPackages = runnableGoPackages(derivedGoPackages, process.env.GO_BIN || (process.platform === "win32" ? "go.exe" : "go"));
  const frontTests = explicitFrontTests.length > 0
    ? explicitFrontTests
    : fullFrontTests
      ? []
      : changedFrontTests(files);

  console.log(`[release-prerequisites] changed files: ${files.length}`);
  console.log(`[release-prerequisites] backend test node: ${testNode || "(not found)"}`);
  console.log(`[release-prerequisites] backend test scope: ${goPackages.length > 0 ? goPackages.join(" ") : "(none)"}`);
  console.log(
    `[release-prerequisites] front test scope: ${
      fullFrontTests
        ? "(full)"
        : frontTests.length > 0
          ? frontTests.map((file) => file.slice(frontRoot.length + 1)).join(" ")
          : "(none)"
    }`,
  );

  if (process.env.AMITIA_RELEASE_PLAN_ONLY === "1") {
    console.log("[release-prerequisites] PLAN ONLY: no command executed");
    return;
  }

  console.log("[release-prerequisites] backend: go mod verify");
  run(goBin, ["mod", "verify"], { cwd: backendRoot });

  console.log("[release-prerequisites] backend: go vet ./...");
  run(goBin, ["vet", "./..."], { cwd: backendRoot });

  if (goPackages.length > 0) {
    console.log(`[release-prerequisites] backend: targeted go test ${goPackages.join(", ")}`);
    runGoTest(goBin, goPackages, { cwd: backendRoot, env: backendTestEnv });
  } else {
    console.log(
      "[release-prerequisites] backend: no changed Go packages; skipping go test (set AMITIA_RELEASE_GO_TEST_PACKAGES or AMITIA_RELEASE_FULL_GO_TESTS=1 to override)",
    );
  }

  if (forcedRaceChecks || fullGoTests || goPackages.some((value) => value.includes("internal/desktoppet"))) {
    console.log("[release-prerequisites] backend: focused desktoppet race checks");
    runGoTest(goBin, ["./internal/desktoppet/installation/...", "./internal/desktoppet/processing/..."], {
      cwd: backendRoot,
      env: backendTestEnv,
      extraArgs: [
        "-race",
        "-timeout=20m",
        "-run",
        "Test.*(Concurrent|Parallel|Atomic|CAS|Lease|Claim|Cancel|Recover|Recovery|Idempotent|Lock|Serialized)",
      ],
    });
  }

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

  if (!skipFrontTests) {
    if (fullFrontTests) {
      console.log("[release-prerequisites] front: full vitest run");
      runPnpm(["--dir", frontRoot, "test"]);
    } else if (explicitFrontTests.length > 0) {
      console.log(`[release-prerequisites] front: targeted vitest run (${explicitFrontTests.length} file(s))`);
      runPnpm(["--dir", frontRoot, "exec", "vitest", "run", ...explicitFrontTests]);
    } else {
      console.log(
        `[release-prerequisites] front: vitest skipped (set AMITIA_RELEASE_FRONT_TEST_FILES or AMITIA_RELEASE_FULL_FRONT_TESTS=1 to run it)${
          frontTests.length > 0 ? `; derived candidates: ${frontTests.map((file) => file.slice(frontRoot.length + 1)).join(" ")}` : ""
        }`,
      );
    }
  } else {
    console.log("[release-prerequisites] front: vitest skipped by AMITIA_RELEASE_SKIP_FRONT_TESTS=1");
  }

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
