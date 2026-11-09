import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { inflateRawSync } from "node:zlib";

const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const manifestInfo = JSON.parse(readFileSync(join(scriptRoot, "amitia-extension.json"), "utf8"));
const packageFile = process.argv[2] ||
  process.env.AMITIA_PLUGIN_MARKET_PACKAGE ||
  resolve(scriptRoot, "..", "..", "plugin", "plugin-market", `amitia-plugin-market-${manifestInfo.plugin.version}.amitiax`);

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
  for (const path of [
    "manifest.json",
    "integrity/files.json",
    "integrity/content-tree.json",
    "modules/plugin-market-runtime/package.json",
    "modules/plugin-market-runtime/dist/index.js",
    "modules/plugin-market-ui/desktop/schema.json",
    "modules/plugin-market-ui/mobile/schema.json",
  ]) {
    if (!entries.has(path)) throw new Error(`missing package path: ${path}`);
  }
  if (manifest.plugin?.id !== "com.amitia/plugin-market") throw new Error("unexpected extension id");
  if (manifest.plugin?.metadata?.["system.defaultInstall"] !== true) throw new Error("default install marker missing");
  if (manifest.plugin?.metadata?.["system.defaultEnabled"] !== true) throw new Error("default enabled marker missing");
  const command = manifest.modules
    ?.flatMap((module) => module.contributions || [])
    .find((item) => item.id === "plugin-market-command");
  if (!command?.spec?.inputSchema?.properties?.action?.enum?.includes("uninstall")) {
    throw new Error("plugin market command must support uninstall");
  }
  if (
    command?.spec?.inputSchema?.properties?.source?.type !== "object" ||
    command.spec.inputSchema.properties.source.additionalProperties !== false
  ) {
    throw new Error("plugin market command must declare the source provenance object");
  }
  for (const contributionId of ["plugin-market-desktop-page", "plugin-market-mobile-page"]) {
    const contribution = manifest.modules
      ?.flatMap((module) => module.contributions || [])
      .find((item) => item.id === contributionId);
    const schemaPath = contribution?.spec?.entry?.schema_path;
    if (!schemaPath) throw new Error(`${contributionId} schema missing`);
    if (contribution.spec.entry.content_hash !== browserHash(entries.get(schemaPath))) {
      throw new Error(`${contributionId} content hash mismatch`);
    }
  }
  const desktop = JSON.parse(entries.get("modules/plugin-market-ui/desktop/schema.json").toString("utf8"));
  const mobile = JSON.parse(entries.get("modules/plugin-market-ui/mobile/schema.json").toString("utf8"));
  if (JSON.stringify(desktop) === JSON.stringify(mobile)) throw new Error("desktop and mobile schema must differ");
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
    .sort((left, right) => Buffer.compare(Buffer.from(left.path), Buffer.from(right.path)));
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
