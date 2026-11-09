import { createHash } from "node:crypto";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { deflateRawSync } from "node:zlib";
import { basename, dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function option(name, fallback = "") {
  const index = process.argv.indexOf(name);
  return index >= 0 ? process.argv[index + 1] || fallback : fallback;
}

function files(root) {
  const result = [];
  const walk = (current) => {
    for (const entry of readdirSync(current).sort()) {
      const path = join(current, entry);
      if (statSync(path).isDirectory()) walk(path);
      else result.push(path);
    }
  };
  walk(root);
  return result;
}

function sha256(data) {
  return createHash("sha256").update(data).digest("hex");
}

function crc32(buffer) {
  let crc = 0xffffffff;
  for (const byte of buffer) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit += 1) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function createZip(root, output) {
  const locals = [];
  const centrals = [];
  let offset = 0;
  for (const file of files(root)) {
    const name = Buffer.from(relative(root, file).replace(/\\/g, "/"));
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
    const localEntry = Buffer.concat([local, name, compressed]);
    locals.push(localEntry);
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
    offset += localEntry.length;
  }
  const central = Buffer.concat(centrals);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(centrals.length, 8);
  end.writeUInt16LE(centrals.length, 10);
  end.writeUInt32LE(central.length, 12);
  end.writeUInt32LE(offset, 16);
  writeFileSync(output, Buffer.concat([...locals, central, end]));
}

const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const packageRoot = resolve(option("--package", process.cwd()));
const manifestPath = join(packageRoot, "amitia-extension.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const moduleDefinition = manifest.modules.find((item) => item.runtime?.adapter === "javascript");
if (!moduleDefinition) throw new Error("javascript module is required");
const staging = join(packageRoot, ".package-staging");
const moduleRoot = join(staging, "modules", moduleDefinition.id, "dist");
const outputRoot = resolve(scriptRoot, "..", "..", "plugin", "agent-identities");
const output = join(outputRoot, `${basename(packageRoot)}-${manifest.plugin.version}.amitiax`);
rmSync(staging, { recursive: true, force: true });
mkdirSync(moduleRoot, { recursive: true });
mkdirSync(outputRoot, { recursive: true });
copyFileSync(join(packageRoot, "src", "index.mjs"), join(moduleRoot, "index.js"));
copyFileSync(join(scriptRoot, "shared", "runtime.mjs"), join(moduleRoot, "common.mjs"));
writeFileSync(join(moduleRoot, "package.json"), `${JSON.stringify({ type: "module" }, null, 2)}\n`);
manifest.integrity.algorithm = "sha256";
manifest.integrity.contentTreeHash = "";
writeFileSync(join(staging, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
const integrityRoot = join(staging, "integrity");
mkdirSync(integrityRoot, { recursive: true });
const entries = files(staging)
  .filter((file) => !relative(staging, file).replace(/\\/g, "/").startsWith("integrity/"))
  .map((file) => {
    const path = relative(staging, file).replace(/\\/g, "/");
    const data = readFileSync(file);
    return { path, size: data.length, hash: sha256(data), modified: "2026-11-03T00:00:00Z" };
  })
  .sort((left, right) => Buffer.compare(Buffer.from(left.path), Buffer.from(right.path)));
const fileMap = {};
for (const entry of entries) fileMap[entry.path] = entry;
writeFileSync(join(integrityRoot, "files.json"), `${JSON.stringify({ algorithm: "sha256", files: fileMap, generatedAt: "2026-11-03T00:00:00Z" }, null, 2)}\n`);
const tree = createHash("sha256");
for (const entry of entries) {
  tree.update(entry.path);
  tree.update(Buffer.from([0]));
  tree.update(entry.hash);
  tree.update(Buffer.from([0]));
}
const treeHash = tree.digest("hex");
writeFileSync(join(integrityRoot, "content-tree.json"), `${JSON.stringify({ algorithm: "sha256", treeHash, generatedAt: "2026-11-03T00:00:00Z" }, null, 2)}\n`);
createZip(staging, output);
rmSync(staging, { recursive: true, force: true });
const sbomPath = `${output}.sbom.json`;
const sbom = {
  spdxVersion: "SPDX-2.3",
  dataLicense: "CC0-1.0",
  SPDXID: "SPDXRef-DOCUMENT",
  name: `${manifest.plugin.id}@${manifest.plugin.version}`,
  documentNamespace: `https://amitia.untrammelled.top/sbom/${encodeURIComponent(manifest.plugin.id)}/${manifest.plugin.version}/${treeHash}`,
  creationInfo: {
    created: "2026-11-03T00:00:00Z",
    creators: ["Tool: Amitia agent identity package builder"],
  },
  packages: [{
    SPDXID: "SPDXRef-Package",
    name: manifest.plugin.id,
    versionInfo: manifest.plugin.version,
    downloadLocation: "NOASSERTION",
    filesAnalyzed: true,
    licenseConcluded: manifest.plugin.license || "NOASSERTION",
    licenseDeclared: manifest.plugin.license || "NOASSERTION",
    checksums: [{ algorithm: "SHA256", checksumValue: treeHash }],
  }],
  files: entries.map((entry, index) => ({
    SPDXID: `SPDXRef-File-${index + 1}`,
    fileName: `./${entry.path}`,
    checksums: [{ algorithm: "SHA256", checksumValue: entry.hash }],
    licenseConcluded: "NOASSERTION",
  })),
  relationships: [
    { spdxElementId: "SPDXRef-DOCUMENT", relationshipType: "DESCRIBES", relatedSpdxElement: "SPDXRef-Package" },
    ...entries.map((_, index) => ({
      spdxElementId: "SPDXRef-Package",
      relationshipType: "CONTAINS",
      relatedSpdxElement: `SPDXRef-File-${index + 1}`,
    })),
  ],
};
writeFileSync(sbomPath, `${JSON.stringify(sbom, null, 2)}\n`);
console.log(`package=${output}`);
console.log(`sbom=${sbomPath}`);
console.log(`treeHash=${treeHash}`);
