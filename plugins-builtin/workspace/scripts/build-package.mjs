import { createHash } from "node:crypto";
import {
  copyFileSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function option(name, fallback) {
  const index = process.argv.indexOf(name);
  return index >= 0 ? process.argv[index + 1] || fallback : fallback;
}

const sourceRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = resolve(sourceRoot, "..", "..");
const stagingRoot = join(sourceRoot, ".package-staging");
const manifest = JSON.parse(readFileSync(join(sourceRoot, "amitia-extension.json"), "utf8"));
const outputDir = option("--output", join(repoRoot, "plugin", "workspace"));
const outputFile = option(
  "--file",
  join(outputDir, `amitia-workspace-${manifest.plugin.version}.amitiax`),
);

function browserHash(path) {
  const digest = createHash("sha256").update(readFileSync(path)).digest("base64");
  return `sha256-${digest}`;
}

function prepare() {
  rmSync(stagingRoot, { recursive: true, force: true });
  mkdirSync(join(stagingRoot, "modules", "workspace-runtime", "dist"), { recursive: true });
  mkdirSync(join(stagingRoot, "modules", "workspace-ui", "desktop"), { recursive: true });
  copyFileSync(
    join(sourceRoot, "src", "runtime", "index.mjs"),
    join(stagingRoot, "modules", "workspace-runtime", "dist", "index.js"),
  );
  writeFileSync(
    join(stagingRoot, "modules", "workspace-runtime", "package.json"),
    `${JSON.stringify({ type: "module" }, null, 2)}\n`,
  );
  copyFileSync(
    join(sourceRoot, "ui", "schema", "workspace.json"),
    join(stagingRoot, "modules", "workspace-ui", "desktop", "workspace.json"),
  );
  for (const module of manifest.modules || []) {
    for (const contribution of module.contributions || []) {
      const schemaPath = contribution.spec?.entry?.schema_path;
      if (schemaPath) {
        contribution.spec.entry.content_hash = browserHash(join(stagingRoot, schemaPath));
      }
    }
  }
  writeFileSync(join(stagingRoot, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
}

prepare();
mkdirSync(outputDir, { recursive: true });
const packed = spawnSync(
  process.env.GO_BIN || "go",
  ["run", "./cmd/amitia-ext", "pack", "--dir", stagingRoot, "-o", outputFile],
  { cwd: join(repoRoot, "backend"), stdio: "inherit", env: process.env },
);
rmSync(stagingRoot, { recursive: true, force: true });
if (packed.error) throw packed.error;
if ((packed.status ?? 1) !== 0) process.exit(packed.status ?? 1);
console.log(outputFile);
