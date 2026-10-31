import { createHash } from "node:crypto";
import { createReadStream, createWriteStream, existsSync, mkdirSync, readFileSync, readdirSync, renameSync, rmSync, statSync, copyFileSync, writeFileSync } from "node:fs";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join, resolve } from "node:path";
import { pipeline } from "node:stream/promises";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const lockPath = join(root, "config", "local-model-runtimes.lock.json");
const args = new Map();
for (let index = 2; index < process.argv.length; index += 1) {
  const value = process.argv[index];
  if (value.startsWith("--")) args.set(value.slice(2), process.argv[index + 1]?.startsWith("--") ? "" : process.argv[++index] ?? "");
}
const target = args.get("target") || defaultTarget();
const destination = resolve(root, args.get("dest") || "desktop/resources/release-runtime/llama");
const cacheDir = resolve(root, "AmitiaData", "runtime-assets-cache");
const lock = JSON.parse(readFileSync(lockPath, "utf8"));
const entry = lock.targets[target];

function defaultTarget() {
  const os = process.platform === "win32" ? "windows" : process.platform === "darwin" ? "macos" : "linux";
  const arch = process.arch === "arm64" ? "arm64" : "x64";
  return `${os}-${arch}`;
}

async function sha256(path) {
  const hash = createHash("sha256");
  await pipeline(createReadStream(path), hash);
  return hash.digest("hex");
}

async function download(url, targetPath) {
  const response = await fetch(url, { redirect: "follow" });
  if (!response.ok || !response.body) throw new Error(`download failed: ${response.status} ${url}`);
  mkdirSync(cacheDir, { recursive: true });
  await pipeline(response.body, createWriteStream(targetPath));
}

function extract(archive, dest) {
  const result = spawnSync("tar", ["-xf", archive, "-C", dest], { stdio: "inherit" });
  if (result.error) throw result.error;
  if ((result.status ?? 1) !== 0) throw new Error(`tar exited with ${result.status}`);
}

function collectRuntimeFiles(source, output) {
  for (const name of readdirSync(source)) {
    const path = join(source, name);
    const info = statSync(path);
    if (info.isDirectory()) {
      collectRuntimeFiles(path, output);
      continue;
    }
    if (/^(llama-server)(\.exe)?$/.test(name) || /\.(dylib|dll)$/.test(name) || /\.so(?:\.|$)/.test(name)) output.push(path);
  }
}

async function main() {
  if (!entry) throw new Error(`unsupported local model runtime target: ${target}`);
  mkdirSync(cacheDir, { recursive: true });
  const archive = join(cacheDir, `${target}-${basename(entry.url)}`);
  if (existsSync(archive) && await sha256(archive) !== entry.sha256) rmSync(archive, { force: true });
  if (!existsSync(archive)) {
    const partial = `${archive}.partial`;
    rmSync(partial, { force: true });
    await download(entry.url, partial);
    const downloaded = await sha256(partial);
    if (downloaded !== entry.sha256) {
      rmSync(partial, { force: true });
      throw new Error(`runtime checksum mismatch: ${downloaded}`);
    }
    renameSync(partial, archive);
  }
  const actual = await sha256(archive);
  if (actual !== entry.sha256) {
    rmSync(archive, { force: true });
    throw new Error(`runtime checksum mismatch: ${actual}`);
  }
  const staging = await mkdtemp(join(tmpdir(), `amitia-llama-${target}-`));
  try {
    extract(archive, staging);
    rmSync(destination, { recursive: true, force: true });
    mkdirSync(destination, { recursive: true });
    const files = [];
    collectRuntimeFiles(staging, files);
    if (!files.some((file) => /^llama-server(\.exe)?$/.test(basename(file)))) throw new Error("llama-server missing from archive");
    const names = new Set();
    for (const file of files) {
      const name = basename(file);
      if (names.has(name)) throw new Error(`duplicate llama runtime file: ${name}`);
      names.add(name);
      copyFileSync(file, join(destination, name));
    }
    const manifest = {
      provider: lock.provider,
      version: lock.version,
      target,
      source: lock.source,
      license: lock.license,
      archiveSha256: entry.sha256,
      files: [...names].sort(),
    };
    writeFileSync(join(destination, "runtime-manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
    console.log(`[local-model-runtime] ${target} -> ${destination}`);
  } finally {
    rmSync(staging, { recursive: true, force: true });
  }
}

main().catch((error) => {
  console.error(`[local-model-runtime] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
