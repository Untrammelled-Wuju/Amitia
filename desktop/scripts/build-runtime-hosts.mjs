import { cpSync, existsSync, mkdirSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const nodeBinary = process.platform === "win32"
  ? resolve(repositoryRoot, "backend", "node", "node.exe")
  : process.env.AMITIA_NODE_BIN || process.execPath;

if (!existsSync(nodeBinary)) {
  throw new Error(`Node binary not found: ${nodeBinary}`);
}

const sdkRoot = resolve(repositoryRoot, "sdk", "plugin-sdk");
const sdkProject = resolve(sdkRoot, "tsconfig.json");
const sdkTsc = [
  resolve(sdkRoot, "node_modules", "typescript", "bin", "tsc"),
  resolve(desktopRoot, "node_modules", "typescript", "bin", "tsc"),
].find((candidate) => existsSync(candidate));
if (!sdkTsc) {
  throw new Error("TypeScript compiler not found for plugin SDK");
}
const sdkBuild = spawnSync(nodeBinary, [sdkTsc, "--project", sdkProject], {
  cwd: sdkRoot,
  stdio: "inherit",
  shell: false,
});
if (sdkBuild.error) throw sdkBuild.error;
if (sdkBuild.status !== 0) {
  throw new Error(`plugin SDK build failed with exit code ${sdkBuild.status}`);
}
const sdkFixImports = spawnSync(nodeBinary, [resolve(sdkRoot, "scripts", "fix-esm-imports.mjs")], {
  cwd: sdkRoot,
  stdio: "inherit",
  shell: false,
});
if (sdkFixImports.error) throw sdkFixImports.error;
if (sdkFixImports.status !== 0) {
  throw new Error(`plugin SDK import repair failed with exit code ${sdkFixImports.status}`);
}
const sdkDist = resolve(sdkRoot, "dist");

for (const host of ["plugin-host", "task-host"]) {
  const project = resolve(repositoryRoot, "runtime", host, "tsconfig.json");
  const tsc = [
    resolve(repositoryRoot, "runtime", host, "node_modules", "typescript", "bin", "tsc"),
    resolve(desktopRoot, "node_modules", "typescript", "bin", "tsc"),
  ].find((candidate) => existsSync(candidate));
  if (!tsc) {
    throw new Error(`TypeScript compiler not found for ${host}`);
  }
  const result = spawnSync(nodeBinary, [tsc, "--project", project], {
    cwd: repositoryRoot,
    stdio: "inherit",
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`${host} build failed with exit code ${result.status}`);
  }
  const sourceDist = resolve(repositoryRoot, "runtime", host, "dist");
  if (host === "plugin-host") {
    const bundledSdk = resolve(sourceDist, "sdk", "plugin-sdk");
    rmSync(bundledSdk, { recursive: true, force: true });
    mkdirSync(bundledSdk, { recursive: true });
    cpSync(sdkDist, bundledSdk, { recursive: true });
    writeFileSync(resolve(bundledSdk, "package.json"), JSON.stringify({ type: "module" }) + "\n");
  }
  const targetDist = resolve(repositoryRoot, "AmitiaData", host, "dist");
  const suffix = `${process.pid}-${Date.now()}`;
  const tempDist = `${targetDist}.tmp-${suffix}`;
  const backupDist = `${targetDist}.backup-${suffix}`;
  rmSync(tempDist, { recursive: true, force: true });
  rmSync(backupDist, { recursive: true, force: true });
  mkdirSync(tempDist, { recursive: true });
  cpSync(sourceDist, tempDist, { recursive: true });
  let previousMoved = false;
  try {
    if (existsSync(targetDist)) {
      renameSync(targetDist, backupDist);
      previousMoved = true;
    }
    renameSync(tempDist, targetDist);
  } catch (error) {
    if (previousMoved && !existsSync(targetDist) && existsSync(backupDist)) renameSync(backupDist, targetDist);
    throw error;
  } finally {
    rmSync(tempDist, { recursive: true, force: true });
    if (existsSync(targetDist)) rmSync(backupDist, { recursive: true, force: true });
  }
}
