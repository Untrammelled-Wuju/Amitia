import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { join, dirname } from "node:path";

const require = createRequire(import.meta.url);
const __dirname = dirname(fileURLToPath(import.meta.url));
const runtimeRoot = join(__dirname, "../..");

const nodeVersion = process.version;
const nodeVersionRaw = process.version;
const platform = process.platform;
const arch = process.arch;
const napiVersion = Number(process.versions.napi);
const execPath = process.execPath;

const npmPackagePath = join(runtimeRoot, "node/lib/node_modules/npm/package.json");
let npmVersion = "";
try {
  const npmPkg = JSON.parse(readFileSync(npmPackagePath, "utf8"));
  npmVersion = npmPkg.version || "";
} catch (e) {
  npmVersion = "";
}

const fs = require("node:fs");
const nodeBinPath = join(runtimeRoot, "node/bin/node");
const npmCliPath = join(runtimeRoot, "node/lib/node_modules/npm/bin/npm-cli.js");
const npxCliPath = join(runtimeRoot, "node/lib/node_modules/npm/bin/npx-cli.js");

const runtimeRootValid = !!(fs.existsSync(nodeBinPath) && fs.existsSync(npmCliPath) && fs.existsSync(npxCliPath));
const packageManagementAvailable = !!(fs.existsSync(npmCliPath) && fs.existsSync(npxCliPath));

if (nodeVersion !== "v24.19.0") {
  process.stderr.write("AMITIA_NODE_ERROR code=30 key=node_version_mismatch\n");
  process.exit(30);
}
if (platform !== "linux") {
  process.stderr.write("AMITIA_NODE_ERROR code=32 key=platform_mismatch\n");
  process.exit(32);
}
if (arch !== "arm64") {
  process.stderr.write("AMITIA_NODE_ERROR code=33 key=arch_mismatch\n");
  process.exit(33);
}
if (npmVersion !== "11.17.0") {
  process.stderr.write("AMITIA_NODE_ERROR code=31 key=npm_version_mismatch\n");
  process.exit(31);
}
if (!runtimeRootValid) {
  process.stderr.write("AMITIA_NODE_ERROR code=10 key=runtime_layout_invalid\n");
  process.exit(10);
}

const output = JSON.stringify({
  schemaVersion: 1,
  nodeVersion: nodeVersion,
  nodeVersionRaw: nodeVersionRaw,
  npmVersion: npmVersion,
  napiVersion: napiVersion,
  platform: platform,
  architecture: arch,
  execPath: execPath,
  runtimeRootValid: runtimeRootValid,
  packageManagementAvailable: packageManagementAvailable
});

process.stdout.write(output + "\n");
process.exit(0);
