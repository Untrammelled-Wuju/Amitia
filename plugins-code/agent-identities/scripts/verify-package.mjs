import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { inflateRawSync } from "node:zlib";

function entries(buffer) {
  let end = -1;
  for (let index = buffer.length - 22; index >= 0; index -= 1) {
    if (buffer.readUInt32LE(index) === 0x06054b50) {
      end = index;
      break;
    }
  }
  if (end < 0) throw new Error("ZIP end record not found");
  const count = buffer.readUInt16LE(end + 10);
  let cursor = buffer.readUInt32LE(end + 16);
  const result = new Map();
  for (let index = 0; index < count; index += 1) {
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
    result.set(name, method === 8 ? inflateRawSync(compressed) : compressed);
    cursor += 46 + nameLength + extraLength + commentLength;
  }
  return result;
}

const path = process.argv[2];
if (!path || !existsSync(path)) throw new Error("package not found");
const archive = entries(readFileSync(path));
const manifest = JSON.parse(archive.get("manifest.json").toString("utf8"));
const files = JSON.parse(archive.get("integrity/files.json").toString("utf8"));
for (const [name, data] of archive) {
  if (name.startsWith("integrity/") || name === "META-INF/amitia-signature.json") continue;
  const record = files.files[name];
  const hash = createHash("sha256").update(data).digest("hex");
  if (!record || record.hash !== hash || record.size !== data.length) {
    throw new Error(`integrity mismatch: ${name}`);
  }
}
if (!manifest.plugin?.id || !manifest.modules?.length) throw new Error("manifest invalid");
console.log(`verified=${path}`);
