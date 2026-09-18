import { createHash } from "node:crypto";
import {
  copyFileSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { deflateRawSync } from "node:zlib";
import { basename, join, relative } from "node:path";

function readOption(name) {
  const index = process.argv.indexOf(name);
  if (index < 0) return "";
  return process.argv[index + 1] || "";
}

const packageRoot = readOption("--source") || process.env.AMITIA_PROACTIVE_SOURCE || ".";
const stagingRoot = join(packageRoot, ".package-staging");
const outputDir =
  readOption("--output") ||
  process.env.AMITIA_PLUGIN_OUTPUT_DIR ||
  join("..", "..", "Plugin", "Character");
const outputFile = join(outputDir, "amitia-\u4e3b\u52a8\u6d88\u606f-1.0.1.amitiax");
const moduleID = "proactive-runtime";
const moduleRoot = join(stagingRoot, "modules", moduleID);
const generatedAt = "2026-01-01T00:00:00Z";

function collectFiles(root) {
  const files = [];
  const walk = (current) => {
    for (const entry of readdirSync(current).sort()) {
      const fullPath = join(current, entry);
      if (statSync(fullPath).isDirectory()) {
        walk(fullPath);
      } else {
        files.push(fullPath);
      }
    }
  };
  walk(root);
  return files;
}

function sha256(buffer) {
  return createHash("sha256").update(buffer).digest("hex");
}

function packagePath(file) {
  return relative(stagingRoot, file).replace(/\\/g, "/");
}

function buildIntegrity() {
  const integrityRoot = join(stagingRoot, "integrity");
  mkdirSync(integrityRoot, { recursive: true });
  const entries = collectFiles(stagingRoot)
    .filter((file) => {
      const name = packagePath(file);
      return name !== "integrity/files.json" &&
        name !== "integrity/content-tree.json" &&
        !name.startsWith("signatures/") &&
        name !== "META-INF/amitia-signature.json";
    })
    .map((file) => {
      const data = readFileSync(file);
      return {
        path: packagePath(file),
        size: data.length,
        hash: sha256(data),
        modified: generatedAt,
      };
    })
    .sort((left, right) => left.path.localeCompare(right.path));

  const files = {};
  for (const entry of entries) {
    files[entry.path] = entry;
  }
  writeFileSync(
    join(integrityRoot, "files.json"),
    `${JSON.stringify({ algorithm: "sha256", files, generatedAt }, null, 2)}\n`,
  );

  const tree = createHash("sha256");
  for (const entry of entries) {
    tree.update(entry.path);
    tree.update(Buffer.from([0]));
    tree.update(entry.hash);
    tree.update(Buffer.from([0]));
  }
  const treeHash = tree.digest("hex");
  writeFileSync(
    join(integrityRoot, "content-tree.json"),
    `${JSON.stringify({ algorithm: "sha256", treeHash, generatedAt }, null, 2)}\n`,
  );
  return treeHash;
}

function crc32(buffer) {
  let crc = 0xffffffff;
  for (const byte of buffer) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit += 1) {
      crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function createZip() {
  const localParts = [];
  const centralParts = [];
  let offset = 0;

  for (const file of collectFiles(stagingRoot)) {
    const archivePath = packagePath(file);
    const name = Buffer.from(archivePath, "utf8");
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
    localParts.push(localEntry);

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
    centralParts.push(Buffer.concat([central, name]));
    offset += localEntry.length;
  }

  const centralDirectory = Buffer.concat(centralParts);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(centralParts.length, 8);
  end.writeUInt16LE(centralParts.length, 10);
  end.writeUInt32LE(centralDirectory.length, 12);
  end.writeUInt32LE(offset, 16);
  writeFileSync(outputFile, Buffer.concat([...localParts, centralDirectory, end]));
}

function main() {
  rmSync(stagingRoot, { recursive: true, force: true });
  mkdirSync(moduleRoot, { recursive: true });
  mkdirSync(outputDir, { recursive: true });
  rmSync(outputFile, { force: true });

  mkdirSync(join(moduleRoot, "dist"), { recursive: true });
  mkdirSync(join(stagingRoot, "modules", "proactive-ui"), { recursive: true });
  copyFileSync(
    join(packageRoot, "src", "index.mjs"),
    join(moduleRoot, "dist", "index.js"),
  );
  for (const file of ["index.html", "app.js", "styles.css"]) {
    copyFileSync(
      join(packageRoot, "src", "ui", file),
      join(stagingRoot, "modules", "proactive-ui", file),
    );
  }
  writeFileSync(
    join(moduleRoot, "package.json"),
    `${JSON.stringify({ type: "module" }, null, 2)}\n`,
  );

  const manifest = JSON.parse(
    readFileSync(join(packageRoot, "amitia-extension.json"), "utf8"),
  );
  const uiContribution = manifest.modules
    .flatMap((module) => module.contributions || [])
    .find((item) => item.id === "proactive-character-tab");
  if (!uiContribution) throw new Error("proactive ui contribution missing");
  uiContribution.spec.entry.content_hash = `sha256-${createHash("sha256")
    .update(readFileSync(join(stagingRoot, "modules", "proactive-ui", "index.html")))
    .digest("base64")}`;
  manifest.integrity.algorithm = "sha256";
  manifest.integrity.contentTreeHash = "";
  writeFileSync(
    join(stagingRoot, "manifest.json"),
    `${JSON.stringify(manifest, null, 2)}\n`,
  );

  const treeHash = buildIntegrity();
  createZip();
  rmSync(stagingRoot, { recursive: true, force: true });
  console.log(`package=${basename(outputFile)}`);
  console.log(`treeHash=${treeHash}`);
}

main();
