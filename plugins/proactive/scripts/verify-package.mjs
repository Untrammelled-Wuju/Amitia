import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { basename, join } from "node:path";
import { inflateRawSync } from "node:zlib";

const packageFile =
  process.argv[2] ||
  process.env.AMITIA_PROACTIVE_PACKAGE ||
  join("..", "..", "Plugin", "Character", "amitia-\u4e3b\u52a8\u6d88\u606f-1.0.1.amitiax");

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
    throw new Error("package not found");
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
    "modules/proactive-ui/index.html",
    "modules/proactive-ui/app.js",
    "modules/proactive-ui/styles.css",
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
  const permissions = Array.isArray(manifest.permissions) ? manifest.permissions : [];
  if (!permissions.some((permission) => permission.name === "message.send")) {
    throw new Error("message.send permission missing");
  }
  if (!permissions.some((permission) => permission.name === "tool.invoke")) {
    throw new Error("tool.invoke permission missing");
  }
  if (permissions.some((permission) => permission.name === "proactive.dispatch")) {
    throw new Error("legacy proactive.dispatch permission must not be present");
  }
  const runtimeSource = entries.get("modules/proactive-runtime/dist/index.js").toString("utf8");
  if (!runtimeSource.includes("host.conversation.message.send")) {
    throw new Error("conversation message host call missing");
  }
  if (runtimeSource.includes("host.proactive.dispatch")) {
    throw new Error("legacy proactive host call must not be present");
  }
  const uiContribution = manifest.modules
    ?.flatMap((module) => module.contributions || [])
    .find((item) => item.id === "proactive-character-tab");
  if (uiContribution?.spec?.slot?.slot_id !== "character.detail.tab") {
    throw new Error("character detail proactive ui contribution missing");
  }
  if (
    uiContribution?.spec?.entry?.content_hash !==
    `sha256-${createHash("sha256")
      .update(entries.get("modules/proactive-ui/index.html"))
      .digest("base64")}`
  ) {
    throw new Error("proactive ui entry hash mismatch");
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
  console.log(`verified=${basename(packageFile)}`);
}

main();
