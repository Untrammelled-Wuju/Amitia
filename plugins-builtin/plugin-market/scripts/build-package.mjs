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
import { basename, dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function readOption(name) {
  const index = process.argv.indexOf(name);
  return index >= 0 ? process.argv[index + 1] || "" : "";
}

const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const packageRoot = readOption("--source") || process.env.AMITIA_PLUGIN_MARKET_SOURCE || scriptRoot;
const outputDir = readOption("--output") || process.env.AMITIA_PLUGIN_OUTPUT_DIR || resolve(packageRoot, "..", "..", "plugin", "plugin-market");
const stagingRoot = join(packageRoot, ".package-staging");
const manifestInfo = JSON.parse(readFileSync(join(packageRoot, "amitia-extension.json"), "utf8"));
const outputFile = join(outputDir, `amitia-plugin-market-${manifestInfo.plugin.version}.amitiax`);
const generatedAt = "2026-01-01T00:00:00Z";

function collectFiles(root) {
  const files = [];
  const walk = (current) => {
    for (const entry of readdirSync(current).sort()) {
      const fullPath = join(current, entry);
      if (statSync(fullPath).isDirectory()) walk(fullPath);
      else files.push(fullPath);
    }
  };
  walk(root);
  return files;
}

function sha256(buffer) {
  return createHash("sha256").update(buffer).digest("hex");
}

function browserHash(buffer) {
  return `sha256-${createHash("sha256").update(buffer).digest("base64")}`;
}

function archivePath(file) {
  return relative(stagingRoot, file).replace(/\\/g, "/");
}

function buildIntegrity() {
  const entries = collectFiles(stagingRoot)
    .filter((file) => {
      const name = archivePath(file);
      return name !== "integrity/files.json" &&
        name !== "integrity/content-tree.json" &&
        !name.startsWith("signatures/") &&
        name !== "META-INF/amitia-signature.json";
    })
    .map((file) => {
      const data = readFileSync(file);
      return {
        path: archivePath(file),
        size: data.length,
        hash: sha256(data),
        modified: generatedAt,
      };
    })
    .sort((left, right) => Buffer.compare(Buffer.from(left.path), Buffer.from(right.path)));
  const files = {};
  for (const entry of entries) files[entry.path] = entry;
  writeFileSync(
    join(stagingRoot, "integrity", "files.json"),
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
    join(stagingRoot, "integrity", "content-tree.json"),
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

function writeZip() {
  const localParts = [];
  const centralParts = [];
  let offset = 0;
  for (const file of collectFiles(stagingRoot)) {
    const name = Buffer.from(archivePath(file), "utf8");
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

function mobileSchema(desktop) {
  const value = JSON.parse(JSON.stringify(desktop));
  const queue = [...(value.children || [])];
  while (queue.length > 0) {
    const node = queue.shift();
    if (node?.id === "plugin-market-list" && node.props) node.props.minItemWidth = 260;
    if (Array.isArray(node?.children)) queue.push(...node.children);
  }
  return value;
}

function main() {
  rmSync(stagingRoot, { recursive: true, force: true });
  mkdirSync(join(stagingRoot, "modules", "plugin-market-runtime", "dist"), { recursive: true });
  mkdirSync(join(stagingRoot, "modules", "plugin-market-ui", "desktop"), { recursive: true });
  mkdirSync(join(stagingRoot, "modules", "plugin-market-ui", "mobile"), { recursive: true });
  mkdirSync(join(stagingRoot, "integrity"), { recursive: true });
  mkdirSync(outputDir, { recursive: true });
  for (const name of readdirSync(outputDir)) {
    if (name.endsWith(".amitiax") || name.endsWith(".amitiax.sbom.json")) {
      rmSync(join(outputDir, name), { force: true });
    }
  }

  copyFileSync(
    join(packageRoot, "src", "runtime", "index.mjs"),
    join(stagingRoot, "modules", "plugin-market-runtime", "dist", "index.js"),
  );
  writeFileSync(
    join(stagingRoot, "modules", "plugin-market-runtime", "package.json"),
    `${JSON.stringify({ type: "module" }, null, 2)}\n`,
  );
  const sourceDesktopPath = join(packageRoot, "ui", "schema", "desktop.json");
  const desktopSchema = JSON.parse(readFileSync(sourceDesktopPath, "utf8"));
  writeFileSync(
    join(stagingRoot, "modules", "plugin-market-ui", "desktop", "schema.json"),
    `${JSON.stringify(desktopSchema, null, 2)}\n`,
  );
  copyFileSync(
    join(packageRoot, "ui", "schema", "extensions-hub.json"),
    join(stagingRoot, "modules", "plugin-market-ui", "desktop", "extensions-hub.json"),
  );
  copyFileSync(
    join(packageRoot, "ui", "schema", "kernel-hub.json"),
    join(stagingRoot, "modules", "plugin-market-ui", "desktop", "kernel-hub.json"),
  );
  writeFileSync(
    join(stagingRoot, "modules", "plugin-market-ui", "mobile", "schema.json"),
    `${JSON.stringify(mobileSchema(desktopSchema), null, 2)}\n`,
  );

  const manifest = JSON.parse(readFileSync(join(packageRoot, "amitia-extension.json"), "utf8"));
  for (const module of manifest.modules || []) {
    for (const contribution of module.contributions || []) {
      const schemaPath = contribution.spec?.entry?.schema_path;
      if (schemaPath) {
        contribution.spec.entry.content_hash = browserHash(readFileSync(join(stagingRoot, schemaPath)));
      }
    }
  }
  manifest.integrity.algorithm = "sha256";
  manifest.integrity.contentTreeHash = "";
  writeFileSync(join(stagingRoot, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
  const treeHash = buildIntegrity();
  writeZip();
  rmSync(stagingRoot, { recursive: true, force: true });
  console.log(`package=${basename(outputFile)}`);
  console.log(`treeHash=${treeHash}`);
}

main();
