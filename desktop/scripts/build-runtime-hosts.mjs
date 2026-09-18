import { existsSync } from "node:fs";
import { resolve } from "node:path";
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
}
