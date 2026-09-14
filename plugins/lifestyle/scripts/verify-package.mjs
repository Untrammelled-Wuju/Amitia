import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { inflateRawSync } from "node:zlib";

const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const manifestInfo = JSON.parse(readFileSync(join(scriptRoot, "amitia-extension.json"), "utf8"));
const packageFile =
  process.argv[2] ||
  process.env.AMITIA_LIFESTYLE_PACKAGE ||
  resolve(scriptRoot, "..", "..", "Plugin", "Character", `amitia-lifestyle-${manifestInfo.extension.version}.amitiax`);

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
    if (buffer.readUInt32LE(cursor) !== 0x02014b50) throw new Error("invalid ZIP central directory");
    const method = buffer.readUInt16LE(cursor + 10);
    const compressedSize = buffer.readUInt32LE(cursor + 20);
    const nameLength = buffer.readUInt16LE(cursor + 28);
    const extraLength = buffer.readUInt16LE(cursor + 30);
    const commentLength = buffer.readUInt16LE(cursor + 32);
    const localOffset = buffer.readUInt32LE(cursor + 42);
    const name = buffer.subarray(cursor + 46, cursor + 46 + nameLength).toString("utf8");
    const localNameLength = buffer.readUInt16LE(localOffset + 26);
    const localExtraLength = buffer.readUInt16LE(localOffset + 28);
    const start = localOffset + 30 + localNameLength + localExtraLength;
    const compressed = buffer.subarray(start, start + compressedSize);
    const data = method === 8 ? inflateRawSync(compressed) : method === 0 ? compressed : null;
    if (!data) throw new Error(`unsupported ZIP method ${method}`);
    entries.set(name, data);
    cursor += 46 + nameLength + extraLength + commentLength;
  }
  return entries;
}

function sha256(buffer) {
  return createHash("sha256").update(buffer).digest("hex");
}

function browserHash(buffer) {
  return `sha256-${createHash("sha256").update(buffer).digest("base64")}`;
}

function main() {
  if (!existsSync(packageFile)) throw new Error("package not found");
  const entries = readEntries(readFileSync(packageFile));
  const manifest = JSON.parse(entries.get("manifest.json").toString("utf8"));
  const files = JSON.parse(entries.get("integrity/files.json").toString("utf8"));
  const tree = JSON.parse(entries.get("integrity/content-tree.json").toString("utf8"));
  const required = [
    "manifest.json",
    "integrity/files.json",
    "integrity/content-tree.json",
    "modules/lifestyle-runtime/package.json",
    "modules/lifestyle-runtime/dist/index.js",
    "modules/lifestyle-ui/index.html",
    "modules/lifestyle-ui/app.js",
    "modules/lifestyle-ui/styles.css",
  ];
  for (const path of required) {
    if (!entries.has(path)) throw new Error(`missing package path: ${path}`);
  }
  if (manifest.extension?.id !== "com.amitia/lifestyle") throw new Error("unexpected extension id");
  const uiContribution = manifest.modules
    ?.flatMap((module) => module.contributions || [])
    .find((item) => item.kind === "ui_page");
  if (uiContribution?.spec?.slot?.slot_id !== "character.detail.tab") {
    throw new Error("character detail ui contribution missing");
  }
  if (uiContribution?.spec?.entry?.content_hash !== browserHash(entries.get("modules/lifestyle-ui/index.html"))) {
    throw new Error("ui entry hash mismatch");
  }
  const payload = [...entries.entries()].filter(([name]) =>
    name !== "integrity/files.json" &&
    name !== "integrity/content-tree.json" &&
    !name.startsWith("signatures/") &&
    name !== "META-INF/amitia-signature.json"
  );
  for (const [name, data] of payload) {
    const declared = files.files[name];
    if (!declared || declared.hash !== sha256(data) || declared.size !== data.length) {
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
  if (digest.digest("hex") !== tree.treeHash) throw new Error("content tree hash mismatch");
  console.log(`verified=${basename(packageFile)}`);
}

main();
