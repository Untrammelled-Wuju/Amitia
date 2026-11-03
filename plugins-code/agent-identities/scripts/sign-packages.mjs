import { existsSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";

const repositoryRoot = resolve(process.argv[3] || ".");
const packageRoot = resolve(process.argv[2] || "plugin/agent-identities");
const keyPath = resolve(
  repositoryRoot,
  process.env.AMITIA_PLUGIN_SIGNING_KEY ||
    "AmitiaData/security/plugin-publisher-private.key",
);
if (!existsSync(keyPath)) {
  throw new Error(`plugin signing key not found: ${keyPath}`);
}
for (const name of readdirSync(packageRoot).filter((item) => item.endsWith(".amitiax")).sort()) {
  const result = spawnSync(
    process.env.GO_BIN || "go",
    [
      "run",
      "./cmd/amitia-ext",
      "sign",
      join(packageRoot, name),
      "--key",
      keyPath,
      "--publisher",
      "com.amitia",
      "--channel",
      "stable",
    ],
    {
      cwd: resolve(repositoryRoot, "backend"),
      stdio: "inherit",
      shell: false,
      env: process.env,
    },
  );
  if (result.error) throw result.error;
  if ((result.status ?? 1) !== 0) {
    throw new Error(`signing failed for ${name}: ${result.status}`);
  }
}
