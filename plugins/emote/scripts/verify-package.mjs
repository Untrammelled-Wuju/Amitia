import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { basename, join } from "node:path";
import { inflateRawSync } from "node:zlib";

const manifestSource = JSON.parse(readFileSync(join(".", "amitia-extension.json"), "utf8"));
const packageFile = process.argv[2]
  || process.env.AMITIA_EMOTE_PACKAGE
  || join("..", "..", "Plugin", "Emote", `amitia-emote-${manifestSource.extension.version}.amitiax`);

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
    const data = method === 8 ? inflateRawSync(compressed) : method === 0 ? compressed : (() => { throw new Error(`unsupported ZIP method ${method}`); })();
    entries.set(name, data);
    cursor += 46 + nameLength + extraLength + commentLength;
  }
  return entries;
}

function sha256(buffer) {
  return createHash("sha256").update(buffer).digest("hex");
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
    "modules/emote-runtime/package.json",
    "modules/emote-runtime/dist/index.js",
    "modules/emote-ui/package.json",
    "modules/emote-ui/ui/index.html",
    "modules/emote-ui/ui/index.js",
    "modules/emote-ui/ui/styles.css",
    "modules/emote-ui/ui/composer.html",
    "modules/emote-ui/ui/composer.js",
    "modules/emote-ui/ui/composer.css",
    "modules/emote-ui/ui/message.html",
    "modules/emote-ui/ui/message.js",
    "modules/emote-ui/ui/message.css",
  ];
  for (const path of required) {
    if (!entries.has(path)) throw new Error(`missing package path: ${path}`);
  }
  if (manifest.extension?.id !== "com.amitia/emote") throw new Error("unexpected extension id");
  const contributions = manifest.modules?.flatMap((module) => module.contributions || []) || [];
  if (!contributions.some((item) => item.spec?.metadata?.["amitia.message.outputs"] === true)) {
    throw new Error("message output provider contribution missing");
  }
  if (!contributions.some((item) => item.spec?.entry?.path === "modules/emote-ui/ui/composer.html")) {
    throw new Error("composer contribution missing");
  }
  for (const file of ["index.html", "composer.html", "message.html"]) {
    const html = entries.get(`modules/emote-ui/ui/${file}`).toString("utf8");
    if (/<style[\s>]/i.test(html) || /<script(?![^>]*\bsrc=)[^>]*>/i.test(html)) {
      throw new Error(`inline CSP resource found: ${file}`);
    }
  }
  const payload = [...entries.entries()].filter(([name]) =>
    name !== "integrity/files.json"
    && name !== "integrity/content-tree.json"
    && !name.startsWith("signatures/")
    && name !== "META-INF/amitia-signature.json"
  );
  for (const [name, data] of payload) {
    const declared = files.files[name];
    if (!declared || declared.hash !== sha256(data) || declared.size !== data.length) {
      throw new Error(`integrity mismatch: ${name}`);
    }
  }
  const canonical = payload.map(([path, data]) => ({ path, hash: sha256(data) })).sort((left, right) => left.path.localeCompare(right.path));
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
