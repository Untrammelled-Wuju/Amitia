import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { inflateRawSync } from "node:zlib";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..", "..", "..");
const packageFile = join(
  projectRoot,
  "Plugin",
  "Proactive",
  "amitia-proactive-1.0.0.amitiax",
);

function readEntries(buffer) {
  const entries = new Map();
  let endOffset = -1;
  for (let offset = buffer.length - 22; offset >= 0; offset -= 1) {
    if (buffer.readUInt32LE(offset) === 0x06054b50) {
      endOffset = offset;
      break;
    }
  }
  if (endOffset < 0) throw new Error("ZIP end record not found");

  const count = buffer.readUInt16LE(endOffset + 10);
  let cursor = buffer.readUInt32LE(endOffset + 16);
  for (let index = 0; index < count; index += 1) {
    if (buffer.readUInt32LE(cursor) !== 0x02014b50) {
      throw new Error("invalid ZIP central directory");
    }
    const method = buffer.readUInt16LE(cursor + 10);
    const compressedSize = buffer.readUInt32LE(cursor + 20);
    const nameLength = buffer.readUInt16LE(cursor + 28);
    const extraLength = buffer.readUInt16LE(cursor + 30);
    const commentLength = buffer.readUInt16LE(cursor + 32);
    const localOffset = buffer.readUInt32LE(cursor + 42);
    const name = buffer
      .subarray(cursor + 46, cursor + 46 + nameLength)
      .toString("utf8");
    const localNameLength = buffer.readUInt16LE(localOffset + 26);
    const localExtraLength = buffer.readUInt16LE(localOffset + 28);
    const start = localOffset + 30 + localNameLength + localExtraLength;
    const compressed = buffer.subarray(start, start + compressedSize);
    const data = method === 8
      ? inflateRawSync(compressed)
      : method === 0
        ? compressed
        : (() => {
            throw new Error(`unsupported ZIP method ${method}`);
          })();
    entries.set(name, data);
    cursor += 46 + nameLength + extraLength + commentLength;
  }
  return entries;
}

function sha256(buffer) {
  return createHash("sha256").update(buffer).digest("hex");
}

function main() {
  if (!existsSync(packageFile)) {
    throw new Error(`package not found: ${packageFile}`);
  }
  const entries = readEntries(readFileSync(packageFile));
  const manifest = JSON.parse(entries.get("manifest.json").toString("utf8"));
  const files = JSON.parse(entries.get("integrity/files.json").toString("utf8"));
  const tree = JSON.parse(entries.get("integrity/content-tree.json").toString("utf8"));

  const required = [
    "manifest.json",
    "integrity/files.json",
    "integrity/content-tree.json",
    "modules/proactive-runtime/package.json",
    "modules/proactive-runtime/dist/index.js",
  ];
  for (const path of required) {
    if (!entries.has(path)) throw new Error(`missing package path: ${path}`);
  }
  if (manifest.manifestVersion !== 1) {
    throw new Error("manifestVersion must be 1");
  }
  if (manifest.extension?.id !== "com.amitia/proactive") {
    throw new Error("unexpected extension id");
  }
  const contribution = manifest.modules
    ?.flatMap((module) => module.contributions || [])
    .find((item) => item.kind === "ui_page");
  if (contribution?.spec?.entry?.runtime_id !== "host.character.proactive") {
    throw new Error("proactive host runtime contribution missing");
  }
  if (files.algorithm !== "sha256" || tree.algorithm !== "sha256") {
    throw new Error("invalid integrity algorithm");
  }

  const payload = [...entries.entries()].filter(([name]) =>
    name !== "integrity/files.json" &&
    name !== "integrity/content-tree.json" &&
    !name.startsWith("signatures/") &&
    name !== "META-INF/amitia-signature.json"
  );
  for (const [name, data] of payload) {
    const declared = files.files[name];
    if (!declared) throw new Error(`integrity entry missing: ${name}`);
    if (declared.hash !== sha256(data) || declared.size !== data.length) {
      throw new Error(`integrity mismatch: ${name}`);
    }
  }

  const canonical = payload
    .map(([path, data]) => ({ path, hash: sha256(data) }))
    .sort((left, right) => left.path.localeCompare(right.path));
  const digest = createHash("sha256");
  for (const entry of canonical) {
    digest.update(entry.path);
    digest.update(Buffer.from([0]));
    digest.update(entry.hash);
    digest.update(Buffer.from([0]));
  }
  if (digest.digest("hex") !== tree.treeHash) {
    throw new Error("content tree hash mismatch");
  }
  console.log(`verified=${packageFile}`);
}

main();
