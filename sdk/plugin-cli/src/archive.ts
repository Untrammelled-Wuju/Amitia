import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as path from "node:path";
import * as zlib from "node:zlib";
import type { AmitiaxManifestV2 } from "@amitia/plugin-sdk";

export interface ArchiveEntry {
  path: string;
  data: Buffer;
}

export interface PackageInspection {
  manifest: AmitiaxManifestV2;
  files: string[];
  treeHash: string;
}

interface IntegrityEntry {
  path: string;
  size: number;
  hash: string;
}

export function buildPackage(projectDir: string, manifestPath: string, outputPath: string): PackageInspection {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8")) as AmitiaxManifestV2;
  const entries = collectProjectEntries(projectDir, manifestPath, manifest);
  const integrityEntries = entries.map((entry) => ({
    path: entry.path,
    size: entry.data.length,
    hash: sha256(entry.data),
  }));
  const treeHash = computeTreeHash(integrityEntries);
  const generatedAt = new Date().toISOString();
  const filesDocument = Buffer.from(JSON.stringify({
    algorithm: "sha256",
    files: Object.fromEntries(integrityEntries.map((entry) => [entry.path, entry])),
    generatedAt,
  }));
  const treeDocument = Buffer.from(JSON.stringify({ algorithm: "sha256", treeHash, generatedAt }));
  entries.push({ path: "integrity/files.json", data: filesDocument });
  entries.push({ path: "integrity/content-tree.json", data: treeDocument });
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, createZip(entries));
  return { manifest, files: entries.map((entry) => entry.path), treeHash };
}

export function inspectPackage(packagePath: string): PackageInspection {
  const entries = readZip(fs.readFileSync(packagePath));
  const manifestData = entries.get("manifest.json");
  const filesData = entries.get("integrity/files.json");
  const treeData = entries.get("integrity/content-tree.json");
  if (!manifestData || !filesData || !treeData) {
    throw new Error("package is missing manifest or integrity metadata");
  }
  const manifest = JSON.parse(manifestData.toString("utf8")) as AmitiaxManifestV2;
  const filesDocument = JSON.parse(filesData.toString("utf8")) as { files: Record<string, IntegrityEntry> };
  const treeDocument = JSON.parse(treeData.toString("utf8")) as { treeHash: string };
  const verified: IntegrityEntry[] = [];
  for (const [name, data] of entries) {
    if (name === "integrity/files.json" || name === "integrity/content-tree.json") continue;
    const expected = filesDocument.files[name];
    if (!expected) throw new Error(`${name} is missing from integrity metadata`);
    if (expected.size !== data.length) throw new Error(`${name} size mismatch`);
    if (expected.hash !== sha256(data)) throw new Error(`${name} hash mismatch`);
    verified.push({ path: name, size: data.length, hash: expected.hash });
  }
  const treeHash = computeTreeHash(verified);
  if (treeHash !== treeDocument.treeHash) throw new Error("content tree hash mismatch");
  if (manifest.integrity.contentTreeHash && manifest.integrity.contentTreeHash !== treeHash) {
    throw new Error("manifest content tree hash mismatch");
  }
  return { manifest, files: Array.from(entries.keys()).sort(), treeHash };
}

function collectProjectEntries(projectDir: string, manifestPath: string, manifest: AmitiaxManifestV2): ArchiveEntry[] {
  const entries: ArchiveEntry[] = [{ path: "manifest.json", data: fs.readFileSync(manifestPath) }];
  for (const directory of ["modules", "resources", "assets", "migrations", "licenses", "docs", "signatures"]) {
    const absolute = path.join(projectDir, directory);
    if (fs.existsSync(absolute)) collectDirectory(absolute, directory, entries);
  }
  for (const module of manifest.modules) {
    const prefix = `modules/${module.id}/`;
    if (entries.some((entry) => entry.path.startsWith(prefix))) continue;
    const entryPoint = module.runtime?.entryPoint;
    if (!entryPoint) continue;
    const candidates = [path.join(projectDir, "dist", `${module.id}.js`), path.join(projectDir, "dist", "index.js")];
    const source = candidates.find((candidate) => fs.existsSync(candidate));
    if (!source) throw new Error(`module ${module.id} has no packaged files or build output`);
    entries.push({ path: `${prefix}${normalizeEntryPath(entryPoint)}`, data: fs.readFileSync(source) });
  }
  return entries.sort((a, b) => a.path.localeCompare(b.path));
}

function collectDirectory(root: string, archiveRoot: string, entries: ArchiveEntry[]): void {
  for (const item of fs.readdirSync(root, { withFileTypes: true })) {
    const absolute = path.join(root, item.name);
    const relative = `${archiveRoot}/${item.name}`.replaceAll("\\", "/");
    if (item.isDirectory()) collectDirectory(absolute, relative, entries);
    if (item.isFile()) entries.push({ path: relative, data: fs.readFileSync(absolute) });
  }
}

function normalizeEntryPath(value: string): string {
  const normalized = value.replaceAll("\\", "/").replace(/^\.\//, "");
  if (!normalized || normalized.includes("..") || normalized.startsWith("/")) throw new Error("invalid module entry path");
  return normalized;
}

function sha256(data: Buffer): string {
  return crypto.createHash("sha256").update(data).digest("hex");
}

function computeTreeHash(entries: IntegrityEntry[]): string {
  const hash = crypto.createHash("sha256");
  for (const entry of [...entries].sort((a, b) => a.path.localeCompare(b.path))) {
    hash.update(entry.path);
    hash.update(Buffer.from([0]));
    hash.update(entry.hash);
    hash.update(Buffer.from([0]));
  }
  return hash.digest("hex");
}

function createZip(entries: ArchiveEntry[]): Buffer {
  const localParts: Buffer[] = [];
  const centralParts: Buffer[] = [];
  let offset = 0;
  for (const entry of entries) {
    const name = Buffer.from(entry.path.replaceAll("\\", "/"));
    const crc = crc32(entry.data);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(0, 6);
    local.writeUInt16LE(0, 8);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(entry.data.length, 18);
    local.writeUInt32LE(entry.data.length, 22);
    local.writeUInt16LE(name.length, 26);
    localParts.push(local, name, entry.data);
    const central = Buffer.alloc(46);
    central.writeUInt32LE(0x02014b50, 0);
    central.writeUInt16LE(20, 4);
    central.writeUInt16LE(20, 6);
    central.writeUInt16LE(0, 8);
    central.writeUInt16LE(0, 10);
    central.writeUInt32LE(crc, 16);
    central.writeUInt32LE(entry.data.length, 20);
    central.writeUInt32LE(entry.data.length, 24);
    central.writeUInt16LE(name.length, 28);
    central.writeUInt32LE(offset, 42);
    centralParts.push(central, name);
    offset += local.length + name.length + entry.data.length;
  }
  const centralData = Buffer.concat(centralParts);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(entries.length, 8);
  end.writeUInt16LE(entries.length, 10);
  end.writeUInt32LE(centralData.length, 12);
  end.writeUInt32LE(offset, 16);
  return Buffer.concat([...localParts, centralData, end]);
}

function readZip(data: Buffer): Map<string, Buffer> {
  const endOffset = findSignature(data, 0x06054b50);
  if (endOffset < 0) throw new Error("invalid ZIP end record");
  const count = data.readUInt16LE(endOffset + 10);
  let cursor = data.readUInt32LE(endOffset + 16);
  const entries = new Map<string, Buffer>();
  for (let index = 0; index < count; index++) {
    if (data.readUInt32LE(cursor) !== 0x02014b50) throw new Error("invalid ZIP central directory");
    const method = data.readUInt16LE(cursor + 10);
    const compressedSize = data.readUInt32LE(cursor + 20);
    const nameLength = data.readUInt16LE(cursor + 28);
    const extraLength = data.readUInt16LE(cursor + 30);
    const commentLength = data.readUInt16LE(cursor + 32);
    const localOffset = data.readUInt32LE(cursor + 42);
    const name = data.subarray(cursor + 46, cursor + 46 + nameLength).toString("utf8");
    if (name.includes("..") || name.startsWith("/") || name.includes("\\")) throw new Error(`unsafe ZIP path: ${name}`);
    if (data.readUInt32LE(localOffset) !== 0x04034b50) throw new Error("invalid ZIP local entry");
    const localNameLength = data.readUInt16LE(localOffset + 26);
    const localExtraLength = data.readUInt16LE(localOffset + 28);
    const start = localOffset + 30 + localNameLength + localExtraLength;
    const compressed = data.subarray(start, start + compressedSize);
    const content = method === 0 ? Buffer.from(compressed) : method === 8 ? zlib.inflateRawSync(compressed) : undefined;
    if (!content) throw new Error(`unsupported ZIP method: ${method}`);
    entries.set(name, content);
    cursor += 46 + nameLength + extraLength + commentLength;
  }
  return entries;
}

function findSignature(data: Buffer, signature: number): number {
  for (let offset = data.length - 22; offset >= Math.max(0, data.length - 65557); offset--) {
    if (data.readUInt32LE(offset) === signature) return offset;
  }
  return -1;
}

const crcTable = Array.from({ length: 256 }, (_, index) => {
  let value = index;
  for (let bit = 0; bit < 8; bit++) value = (value & 1) !== 0 ? 0xedb88320 ^ (value >>> 1) : value >>> 1;
  return value >>> 0;
});

function crc32(data: Buffer): number {
  let value = 0xffffffff;
  for (const byte of data) value = (value >>> 8) ^ (crcTable[(value ^ byte) & 0xff] ?? 0);
  return (value ^ 0xffffffff) >>> 0;
}
