import { createHash } from 'node:crypto';
import {
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { deflateRawSync, inflateRawSync } from 'node:zlib';

const here = dirname(fileURLToPath(import.meta.url));
const workspace = resolve(process.env.GITHUB_WORKSPACE || resolve(here, '../../..'));
const pluginDir = join(workspace, 'testplugins', 'game-plugin-demo', 'go');
const sourceManifestPath = join(pluginDir, 'manifest.json');
const outputDir = join(pluginDir, 'dist-package');
const stagingRoot = join(pluginDir, '.package-staging-e2e');
const runtimeModuleId = 'mock-go-runtime';
const executableName = process.platform === 'win32' ? 'mock-game-plugin.exe' : 'mock-game-plugin';
const generatedAt = '2026-08-26T00:00:00Z';

function runGoBuild(outputPath) {
  const result = spawnSync('go', ['build', '-trimpath', '-o', outputPath, './cmd/mock-game-plugin'], {
    cwd: pluginDir,
    env: { ...process.env, CGO_ENABLED: '0' },
    stdio: 'inherit',
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`go build native fixture failed with exit code ${result.status}`);
}

function collectFiles(root) {
  const files = [];
  const walk = current => {
    for (const entry of readdirSync(current).sort()) {
      const full = join(current, entry);
      if (statSync(full).isDirectory()) walk(full);
      else files.push(full);
    }
  };
  walk(root);
  return files;
}

function sha256Raw(data) {
  return createHash('sha256').update(data).digest('hex');
}

function canonicalPath(root, file) {
  return relative(root, file).replace(/\\/g, '/');
}

function buildIntegrityDocuments(staging) {
  const integrityDir = join(staging, 'integrity');
  mkdirSync(integrityDir, { recursive: true });
  const entries = collectFiles(staging)
    .filter(file => {
      const name = canonicalPath(staging, file);
      return name !== 'integrity/files.json' && name !== 'integrity/content-tree.json';
    })
    .map(file => {
      const data = readFileSync(file);
      const path = canonicalPath(staging, file);
      return { path, size: data.length, hash: sha256Raw(data), modified: generatedAt };
    })
    .sort((a, b) => a.path.localeCompare(b.path));

  const files = Object.fromEntries(entries.map(entry => [entry.path, entry]));
  writeFileSync(join(integrityDir, 'files.json'), `${JSON.stringify({ algorithm: 'sha256', files, generatedAt }, null, 2)}\n`);

  const tree = createHash('sha256');
  for (const entry of entries) {
    tree.update(entry.path);
    tree.update(Buffer.from([0]));
    tree.update(entry.hash);
    tree.update(Buffer.from([0]));
  }
  const treeHash = tree.digest('hex');
  writeFileSync(join(integrityDir, 'content-tree.json'), `${JSON.stringify({ algorithm: 'sha256', treeHash, generatedAt }, null, 2)}\n`);
  return treeHash;
}

function crc32(buffer) {
  let crc = 0xffffffff;
  for (const byte of buffer) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function createZip(staging, outputFile) {
  const locals = [];
  const central = [];
  let offset = 0;
  for (const file of collectFiles(staging)) {
    const archivePath = canonicalPath(staging, file);
    const content = readFileSync(file);
    const name = Buffer.from(archivePath, 'utf8');
    const compressed = deflateRawSync(content, { level: 9 });
    const crc = crc32(content);

    const localHeader = Buffer.alloc(30);
    localHeader.writeUInt32LE(0x04034b50, 0);
    localHeader.writeUInt16LE(20, 4);
    localHeader.writeUInt16LE(0, 6);
    localHeader.writeUInt16LE(8, 8);
    localHeader.writeUInt32LE(crc, 14);
    localHeader.writeUInt32LE(compressed.length, 18);
    localHeader.writeUInt32LE(content.length, 22);
    localHeader.writeUInt16LE(name.length, 26);
    const local = Buffer.concat([localHeader, name, compressed]);
    locals.push(local);

    const centralHeader = Buffer.alloc(46);
    centralHeader.writeUInt32LE(0x02014b50, 0);
    centralHeader.writeUInt16LE(20, 4);
    centralHeader.writeUInt16LE(20, 6);
    centralHeader.writeUInt16LE(0, 8);
    centralHeader.writeUInt16LE(8, 10);
    centralHeader.writeUInt32LE(crc, 16);
    centralHeader.writeUInt32LE(compressed.length, 20);
    centralHeader.writeUInt32LE(content.length, 24);
    centralHeader.writeUInt16LE(name.length, 28);
    centralHeader.writeUInt32LE(offset, 42);
    central.push(Buffer.concat([centralHeader, name]));
    offset += local.length;
  }

  const centralStart = offset;
  const centralSize = central.reduce((sum, item) => sum + item.length, 0);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(central.length, 8);
  end.writeUInt16LE(central.length, 10);
  end.writeUInt32LE(centralSize, 12);
  end.writeUInt32LE(centralStart, 16);
  writeFileSync(outputFile, Buffer.concat([...locals, ...central, end]));
}

function readZipEntries(buffer) {
  let eocd = -1;
  for (let i = buffer.length - 22; i >= Math.max(0, buffer.length - 65557); i--) {
    if (buffer.readUInt32LE(i) === 0x06054b50) { eocd = i; break; }
  }
  if (eocd < 0) throw new Error('native fixture ZIP EOCD not found');
  const count = buffer.readUInt16LE(eocd + 10);
  let cursor = buffer.readUInt32LE(eocd + 16);
  const entries = new Map();
  for (let i = 0; i < count; i++) {
    if (buffer.readUInt32LE(cursor) !== 0x02014b50) throw new Error('invalid native fixture central directory');
    const method = buffer.readUInt16LE(cursor + 10);
    const compressedSize = buffer.readUInt32LE(cursor + 20);
    const nameLen = buffer.readUInt16LE(cursor + 28);
    const extraLen = buffer.readUInt16LE(cursor + 30);
    const commentLen = buffer.readUInt16LE(cursor + 32);
    const localOffset = buffer.readUInt32LE(cursor + 42);
    const name = buffer.subarray(cursor + 46, cursor + 46 + nameLen).toString('utf8');
    if (entries.has(name)) throw new Error(`duplicate native fixture ZIP entry: ${name}`);
    if (buffer.readUInt32LE(localOffset) !== 0x04034b50) throw new Error(`invalid local header for ${name}`);
    const localNameLen = buffer.readUInt16LE(localOffset + 26);
    const localExtraLen = buffer.readUInt16LE(localOffset + 28);
    const dataStart = localOffset + 30 + localNameLen + localExtraLen;
    const compressed = buffer.subarray(dataStart, dataStart + compressedSize);
    entries.set(name, method === 8 ? inflateRawSync(compressed) : compressed);
    cursor += 46 + nameLen + extraLen + commentLen;
  }
  return entries;
}

function verifyPackage(outputFile, expectedVersion, expectedTreeHash) {
  const entries = readZipEntries(readFileSync(outputFile));
  const required = [
    'manifest.json',
    'integrity/files.json',
    'integrity/content-tree.json',
    `modules/${runtimeModuleId}/${executableName}`,
  ];
  for (const path of required) if (!entries.has(path)) throw new Error(`native fixture missing ${path}`);
  for (const name of entries.keys()) {
    if (!required.includes(name)) throw new Error(`unexpected native fixture package path: ${name}`);
  }

  const manifest = JSON.parse(entries.get('manifest.json').toString('utf8'));
  const filesDoc = JSON.parse(entries.get('integrity/files.json').toString('utf8'));
  const treeDoc = JSON.parse(entries.get('integrity/content-tree.json').toString('utf8'));
  if (manifest.extension?.id !== 'com.example/mock-game-plugin-go') throw new Error('native fixture extension id mismatch');
  if (manifest.extension?.version !== expectedVersion) throw new Error('native fixture version mismatch');
  if (manifest.modules?.[0]?.type !== 'service' || manifest.modules?.[0]?.runtime?.type !== 'service') {
    throw new Error('native fixture must exercise service module/runtime');
  }
  if (manifest.modules?.[0]?.runtime?.entryPoint !== executableName) throw new Error('native fixture entryPoint mismatch');
  if (treeDoc.treeHash !== expectedTreeHash) throw new Error('native fixture content tree mismatch');

  const payloadNames = [...entries.keys()].filter(name => !name.startsWith('integrity/')).sort();
  if (JSON.stringify(Object.keys(filesDoc.files || {}).sort()) !== JSON.stringify(payloadNames)) {
    throw new Error('native fixture integrity coverage mismatch');
  }
  const tree = createHash('sha256');
  for (const name of payloadNames) {
    const data = entries.get(name);
    const declared = filesDoc.files[name];
    const hash = sha256Raw(data);
    if (!declared || declared.hash !== hash || declared.size !== data.length) throw new Error(`native fixture integrity mismatch for ${name}`);
    tree.update(name);
    tree.update(Buffer.from([0]));
    tree.update(hash);
    tree.update(Buffer.from([0]));
  }
  if (tree.digest('hex') !== expectedTreeHash) throw new Error('native fixture recomputed tree mismatch');
}

function buildFixture(version, fileName) {
  const staging = join(stagingRoot, version.replace(/[^0-9A-Za-z.-]/g, '_'));
  rmSync(staging, { recursive: true, force: true });
  mkdirSync(join(staging, 'modules', runtimeModuleId), { recursive: true });

  const executablePath = join(staging, 'modules', runtimeModuleId, executableName);
  runGoBuild(executablePath);

  const manifest = JSON.parse(readFileSync(sourceManifestPath, 'utf8'));
  manifest.extension.version = version;
  manifest.compatibility.platforms = [process.platform === 'darwin' ? 'macos' : process.platform];
  manifest.modules[0].runtime.entryPoint = executableName;
  manifest.integrity = { algorithm: 'sha256', contentTreeHash: '' };
  writeFileSync(join(staging, 'manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`);

  const treeHash = buildIntegrityDocuments(staging);
  const outputFile = join(outputDir, fileName);
  createZip(staging, outputFile);
  verifyPackage(outputFile, version, treeHash);
  process.stdout.write(`Native service fixture ready: ${outputFile}\n`);
}

mkdirSync(outputDir, { recursive: true });
rmSync(stagingRoot, { recursive: true, force: true });
try {
  buildFixture('0.3.0', 'mock-game-plugin-go-v1.amitiax');
  buildFixture('0.4.0', 'mock-game-plugin-go-v2.amitiax');
} finally {
  rmSync(stagingRoot, { recursive: true, force: true });
}
