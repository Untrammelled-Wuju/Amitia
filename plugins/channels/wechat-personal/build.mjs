import { createHash } from "node:crypto";
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
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

function files(dir) {
  const out = [];
  const walk = (current) => {
    for (const name of readdirSync(current).sort()) {
      const file = join(current, name);
      if (statSync(file).isDirectory()) walk(file);
      else out.push(file);
    }
  };
  walk(dir);
  return out;
}

function rel(file) {
  return relative(staging, file).replace(/\\/g, "/");
}

function sha256(data) {
  return createHash("sha256").update(data).digest("hex");
}

function browserHash(data) {
  return `sha256-${createHash("sha256").update(data).digest("base64")}`;
}

function crc32(buffer) {
  let crc = 0xffffffff;
  for (const byte of buffer) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function zip(output) {
  const locals = [];
  const centrals = [];
  let offset = 0;
  for (const file of files(staging)) {
    const archivePath = rel(file);
    const name = Buffer.from(archivePath);
    const content = readFileSync(file);
    const compressed = deflateRawSync(content, { level: 9 });
    const crc = crc32(content);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(8, 8);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(compressed.length, 18);
    local.writeUInt32LE(content.length, 22);
    local.writeUInt16LE(name.length, 26);
    const item = Buffer.concat([local, name, compressed]);
    locals.push(item);
    const central = Buffer.alloc(46);
    central.writeUInt32LE(0x02014b50, 0);
    central.writeUInt16LE(20, 4);
    central.writeUInt16LE(20, 6);
    central.writeUInt16LE(8, 10);
    central.writeUInt32LE(crc, 16);
    central.writeUInt32LE(compressed.length, 20);
    central.writeUInt32LE(content.length, 24);
    central.writeUInt16LE(name.length, 28);
    central.writeUInt32LE(offset, 42);
    centrals.push(Buffer.concat([central, name]));
    offset += item.length;
  }
  const centralDirectory = Buffer.concat(centrals);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(centrals.length, 8);
  end.writeUInt16LE(centrals.length, 10);
  end.writeUInt32LE(centralDirectory.length, 12);
  end.writeUInt32LE(offset, 16);
  writeFileSync(output, Buffer.concat([...locals, centralDirectory, end]));
}

function copyDirectory(source, target) {
  if (!existsSync(source) || !statSync(source).isDirectory()) throw new Error(`directory missing: ${source}`);
  mkdirSync(target, { recursive: true });
  for (const name of readdirSync(source).sort()) {
    const sourcePath = join(source, name);
    const targetPath = join(target, name);
    if (statSync(sourcePath).isDirectory()) copyDirectory(sourcePath, targetPath);
    else copyFileSync(sourcePath, targetPath);
  }
}

function copyVendorRuntime(target) {
  const source = join(root, "vendor", "qrcode");
  const qrcodeTarget = join(target, "qrcode");
  const pngjsTarget = join(qrcodeTarget, "node_modules", "pngjs");
  const dijkstraTarget = join(qrcodeTarget, "node_modules", "dijkstrajs");
  copyDirectory(join(source, "lib"), join(qrcodeTarget, "lib"));
  copyFileSync(join(source, "package.json"), join(qrcodeTarget, "package.json"));
  copyFileSync(join(source, "license"), join(qrcodeTarget, "license"));
  copyDirectory(join(source, "node_modules", "pngjs", "lib"), join(pngjsTarget, "lib"));
  copyFileSync(join(source, "node_modules", "pngjs", "package.json"), join(pngjsTarget, "package.json"));
  copyFileSync(join(source, "node_modules", "pngjs", "LICENSE"), join(pngjsTarget, "LICENSE"));
  mkdirSync(dijkstraTarget, { recursive: true });
  copyFileSync(join(source, "node_modules", "dijkstrajs", "dijkstra.js"), join(dijkstraTarget, "dijkstra.js"));
  copyFileSync(join(source, "node_modules", "dijkstrajs", "package.json"), join(dijkstraTarget, "package.json"));
  copyFileSync(join(source, "node_modules", "dijkstrajs", "LICENSE.md"), join(dijkstraTarget, "LICENSE.md"));
}

function verifyIndependentRuntime(manifest) {
  const service = readFileSync(join(root, "runtime", "service.mjs"), "utf8");
  const forbidden = ["child_process", "AMITIA_NATIVE_COMPANIONS", "nativeCompanions", "Weixin.exe", "InstallPath"];
  for (const token of forbidden) {
    if (service.includes(token)) throw new Error(`personal WeChat runtime still depends on ${token}`);
  }
  for (const mod of manifest.modules || []) {
    if (mod.runtime?.nativeCompanions) throw new Error(`native companion remains declared on ${mod.id}`);
  }
  const required = ["ilink/bot/get_bot_qrcode", "ilink/bot/getupdates", "ilink/bot/sendmessage"];
  for (const endpoint of required) {
    if (!service.includes(endpoint)) throw new Error(`iLink endpoint missing from runtime: ${endpoint}`);
  }
}

function verifyChannelStatusOwnership() {
  const source = readFileSync(join(root, "ui", "desktop", "app.js"), "utf8");
  if (source.includes("setInterval(")) throw new Error("channel UI must not poll channel status");
}

function runReleasePreflight() {
  for (const test of ["service-security-e2e.mjs", "service-e2e.mjs"]) {
    run(process.execPath, [join(root, "tests", test)]);
  }
}

runReleasePreflight();
verifyChannelStatusOwnership();
run(process.execPath, ["--check", join(root, "runtime", "service.mjs")]);
run(process.execPath, ["--check", join(root, "ui", "desktop", "app.js")]);

const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
verifyIndependentRuntime(manifest);

rmSync(staging, { recursive: true, force: true });
mkdirSync(join(staging, "modules", "wechat-personal-channel-service"), { recursive: true });
mkdirSync(join(staging, "modules", "wechat-personal-channel-ui", "desktop"), { recursive: true });
mkdirSync(join(staging, "assets", "docs"), { recursive: true });
for (const name of ["launcher.mjs", "service.mjs"]) {
  copyFileSync(join(root, "runtime", name), join(staging, "modules", "wechat-personal-channel-service", name));
}
copyVendorRuntime(join(staging, "modules", "wechat-personal-channel-service", "vendor"));
for (const name of ["index.html", "app.js", "styles.css"]) {
  copyFileSync(join(root, "ui", "desktop", name), join(staging, "modules", "wechat-personal-channel-ui", "desktop", name));
}
for (const doc of ["README.md", "INTEGRATION_REPORT.md"]) {
  const source = join(root, doc);
  if (existsSync(source)) copyFileSync(source, join(staging, "assets", "docs", doc));
}
writeFileSync(join(staging, "modules", "wechat-personal-channel-ui", "package.json"), JSON.stringify({ type: "module" }, null, 2) + "\n");

for (const mod of manifest.modules || []) {
  for (const contribution of mod.contributions || []) {
    const entryPath = contribution.spec?.entry?.path;
    if (entryPath) contribution.spec.entry.content_hash = browserHash(readFileSync(join(staging, entryPath)));
  }
}
manifest.integrity.algorithm = "sha256";
manifest.integrity.contentTreeHash = "";
writeFileSync(join(staging, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
mkdirSync(join(staging, "integrity"), { recursive: true });
const entries = files(staging)
  .filter((file) => !rel(file).startsWith("integrity/"))
  .map((file) => {
    const buffer = readFileSync(file);
    return { path: rel(file), size: buffer.length, hash: sha256(buffer), modified: generatedAt };
  })
  .sort((left, right) => Buffer.compare(Buffer.from(left.path, "utf8"), Buffer.from(right.path, "utf8")));
const map = {};
for (const entry of entries) map[entry.path] = entry;
writeFileSync(join(staging, "integrity", "files.json"), JSON.stringify({ algorithm: "sha256", files: map, generatedAt }, null, 2) + "\n");
const hash = createHash("sha256");
for (const entry of entries) {
  hash.update(entry.path);
  hash.update(Buffer.from([0]));
  hash.update(entry.hash);
  hash.update(Buffer.from([0]));
}
const treeHash = hash.digest("hex");
writeFileSync(join(staging, "integrity", "content-tree.json"), JSON.stringify({ algorithm: "sha256", treeHash, generatedAt }, null, 2) + "\n");

mkdirSync(outputDir, { recursive: true });
const output = join(outputDir, `${manifest.extension.id.replace("/", ".")}-${manifest.extension.version}.amitiax`);
zip(output);
rmSync(staging, { recursive: true, force: true });
console.log(`package=${basename(output)}`);
console.log(`treeHash=${treeHash}`);
