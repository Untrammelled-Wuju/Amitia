import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync, existsSync } from "node:fs";
import { deflateRawSync } from "node:zlib";
import { basename, dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const root = resolve(dirname(fileURLToPath(import.meta.url)));
const outputDir = resolve(process.argv[2] || join(root, "..", "..", "..", "plugin"));
const manifestPath = join(root, "amitia-extension.json");
const staging = join(root, ".package-staging");
const generatedAt = new Date().toISOString();

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd || root,
    env: { ...process.env, ...(options.env || {}) },
    stdio: "inherit",
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${command} ${args.join(" ")} failed with exit code ${result.status}`);
}
function files(dir) { const out = []; const walk = (p) => { for (const n of readdirSync(p).sort()) { const f = join(p, n); if (statSync(f).isDirectory()) walk(f); else out.push(f); } }; walk(dir); return out; }
function rel(file) { return relative(staging, file).replace(/\\/g, "/"); }
function sha256(data) { return createHash("sha256").update(data).digest("hex"); }
function browserHash(data) { return `sha256-${createHash("sha256").update(data).digest("base64")}`; }
function crc32(buffer) { let crc = 0xffffffff; for (const byte of buffer) { crc ^= byte; for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1)); } return (crc ^ 0xffffffff) >>> 0; }
function zip(output) {
  const locals = [], centrals = []; let offset = 0;
  for (const f of files(staging)) {
    const ap = rel(f), name = Buffer.from(ap), content = readFileSync(f), compressed = deflateRawSync(content, { level: 9 }), crc = crc32(content);
    const local = Buffer.alloc(30); local.writeUInt32LE(0x04034b50, 0); local.writeUInt16LE(20, 4); local.writeUInt16LE(8, 8); local.writeUInt32LE(crc, 14); local.writeUInt32LE(compressed.length, 18); local.writeUInt32LE(content.length, 22); local.writeUInt16LE(name.length, 26);
    const item = Buffer.concat([local, name, compressed]); locals.push(item);
    const central = Buffer.alloc(46); central.writeUInt32LE(0x02014b50, 0); central.writeUInt16LE(20, 4); central.writeUInt16LE(20, 6); central.writeUInt16LE(8, 10); central.writeUInt32LE(crc, 16); central.writeUInt32LE(compressed.length, 20); central.writeUInt32LE(content.length, 24); central.writeUInt16LE(name.length, 28); central.writeUInt32LE(offset, 42);
    centrals.push(Buffer.concat([central, name])); offset += item.length;
  }
  const cd = Buffer.concat(centrals), end = Buffer.alloc(22); end.writeUInt32LE(0x06054b50, 0); end.writeUInt16LE(centrals.length, 8); end.writeUInt16LE(centrals.length, 10); end.writeUInt32LE(cd.length, 12); end.writeUInt32LE(offset, 16);
  writeFileSync(output, Buffer.concat([...locals, cd, end]));
}

function buildNative() {
  const agentRoot = join(root, "native-agent");
  const binRoot = join(root, "native", "bin");
  mkdirSync(join(binRoot, "windows-x64"), { recursive: true });
  mkdirSync(join(binRoot, "linux-x64"), { recursive: true });

  run("go", ["test", "./..."], { cwd: agentRoot });
  run("go", ["build", "-trimpath", "-ldflags=-s -w -buildid=", "-o", join(binRoot, "windows-x64", "amitia-wechat-agent.exe"), "./cmd/agent"], {
    cwd: agentRoot,
    env: { GOOS: "windows", GOARCH: "amd64", CGO_ENABLED: "0" },
  });
  run("go", ["build", "-trimpath", "-ldflags=-s -w -buildid=", "-o", join(binRoot, "linux-x64", "amitia-wechat-agent"), "./cmd/agent"], {
    cwd: agentRoot,
    env: { GOOS: "linux", GOARCH: "amd64", CGO_ENABLED: "0" },
  });

  if (process.platform === "linux") {
    run("g++", ["-std=c++17", "-shared", "-fPIC", "-O2", "-s", "-Wl,--build-id=none", "-pthread", "-o", join(binRoot, "linux-x64", "libamitia_wechat_preload.so"), join(root, "native", "linux-hook", "amitia_wechat_preload.cpp")]);
  } else if (!existsSync(join(binRoot, "linux-x64", "libamitia_wechat_preload.so"))) {
    throw new Error("Linux preload companion is missing. Build release packages on Linux or provide the prebuilt verified .so.");
  }
  run(process.execPath, ["--check", join(root, "runtime", "service.mjs")]);
  run(process.execPath, ["--check", join(root, "ui", "desktop", "app.js")]);
}

function runReleasePreflight() {
  const tests = [
    "service-security-e2e.mjs",
    "native-service-e2e.mjs",
    "native-version-gate-e2e.mjs",
    "service-e2e.mjs",
  ];
  for (const test of tests) run(process.execPath, [join(root, "tests", test)]);
}

function verifyChannelStatusOwnership() {
  const source = readFileSync(join(root, "ui", "desktop", "app.js"), "utf8");
  if (source.includes("setInterval(")) {
    throw new Error("channel UI must not poll channel status");
  }
}

function updateNativeHashes(manifest) {
  for (const mod of manifest.modules || []) {
    for (const companion of mod.runtime?.nativeCompanions || []) {
      const suffix = String(companion.path || "").replace(/^native\//, "");
      const source = join(root, "native", "bin", suffix);
      if (!existsSync(source) || !statSync(source).isFile()) throw new Error(`native build output missing: ${source}`);
      companion.sha256 = sha256(readFileSync(source));
    }
  }
  writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");
}

runReleasePreflight();
verifyChannelStatusOwnership();
buildNative();
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
updateNativeHashes(manifest);

rmSync(staging, { recursive: true, force: true });
mkdirSync(join(staging, "modules", "wechat-personal-channel-service"), { recursive: true });
mkdirSync(join(staging, "modules", "wechat-personal-channel-ui", "desktop"), { recursive: true });
mkdirSync(join(staging, "assets", "docs"), { recursive: true });
for (const name of ["launcher.mjs", "service.mjs"]) copyFileSync(join(root, "runtime", name), join(staging, "modules", "wechat-personal-channel-service", name));
for (const [src, dst] of [
  [join(root, "native", "bin", "windows-x64", "amitia-wechat-agent.exe"), join(staging, "modules", "wechat-personal-channel-service", "native", "windows-x64", "amitia-wechat-agent.exe")],
  [join(root, "native", "bin", "linux-x64", "amitia-wechat-agent"), join(staging, "modules", "wechat-personal-channel-service", "native", "linux-x64", "amitia-wechat-agent")],
  [join(root, "native", "bin", "linux-x64", "libamitia_wechat_preload.so"), join(staging, "modules", "wechat-personal-channel-service", "native", "linux-x64", "libamitia_wechat_preload.so")],
]) { mkdirSync(dirname(dst), { recursive: true }); copyFileSync(src, dst); }
for (const name of ["index.html", "app.js", "styles.css"]) copyFileSync(join(root, "ui", "desktop", name), join(staging, "modules", "wechat-personal-channel-ui", "desktop", name));
for (const doc of ["NATIVE_DRIVER_CONTRACT.md", "INTEGRATION_REPORT.md"]) {
  const src = doc === "INTEGRATION_REPORT.md" ? join(root, doc) : join(root, "docs", doc);
  if (existsSync(src)) copyFileSync(src, join(staging, "assets", "docs", doc));
}
writeFileSync(join(staging, "modules", "wechat-personal-channel-ui", "package.json"), JSON.stringify({ type: "module" }, null, 2) + "\n");

for (const mod of manifest.modules || []) for (const c of mod.contributions || []) {
  const p = c.spec?.entry?.path;
  if (p) c.spec.entry.content_hash = browserHash(readFileSync(join(staging, p)));
}
for (const mod of manifest.modules || []) for (const companion of mod.runtime?.nativeCompanions || []) {
  const file = join(staging, "modules", mod.id, companion.path);
  if (!existsSync(file) || !statSync(file).isFile()) throw new Error(`native companion missing: ${mod.id}/${companion.path}`);
  const actual = sha256(readFileSync(file));
  if (actual !== companion.sha256) throw new Error(`native companion hash mismatch: ${mod.id}/${companion.path}`);
}
manifest.integrity.algorithm = "sha256";
manifest.integrity.contentTreeHash = "";
writeFileSync(join(staging, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
mkdirSync(join(staging, "integrity"), { recursive: true });
const entries = files(staging).filter((f) => !rel(f).startsWith("integrity/")).map((f) => { const b = readFileSync(f); return { path: rel(f), size: b.length, hash: sha256(b), modified: generatedAt }; }).sort((a, b) => a.path.localeCompare(b.path));
const map = {}; for (const e of entries) map[e.path] = e;
writeFileSync(join(staging, "integrity", "files.json"), JSON.stringify({ algorithm: "sha256", files: map, generatedAt }, null, 2) + "\n");
const h = createHash("sha256"); for (const e of entries) { h.update(e.path); h.update(Buffer.from([0])); h.update(e.hash); h.update(Buffer.from([0])); }
const treeHash = h.digest("hex");
writeFileSync(join(staging, "integrity", "content-tree.json"), JSON.stringify({ algorithm: "sha256", treeHash, generatedAt }, null, 2) + "\n");

mkdirSync(outputDir, { recursive: true });
const output = join(outputDir, `${manifest.extension.id.replace("/", ".")}-${manifest.extension.version}.amitiax`);
zip(output);
rmSync(staging, { recursive: true, force: true });
console.log(`package=${basename(output)}`);
console.log(`treeHash=${treeHash}`);
