import { execFileSync } from 'node:child_process';
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { createHash } from 'node:crypto';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { deflateRawSync } from 'node:zlib';

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, '..');
const distDir = join(projectRoot, 'dist');
const packageOutputDir = join(projectRoot, 'dist-package');
const packageStagingDir = join(projectRoot, '.package-staging');
const outputFile = join(packageOutputDir, process.env.MOCK_PLUGIN_OUTPUT || 'world-game-plugin.amitiax');
const packageVersionOverride = (process.env.MOCK_PLUGIN_VERSION || '').trim();
const requireCompanionArtifact = /^(1|true|yes)$/i.test((process.env.MOCK_PLUGIN_REQUIRED_ARTIFACT || '').trim());
const runtimeModuleId = 'world-game-runtime';
const supportModuleId = 'world-game-support';
const runtimeModuleRoot = join(packageStagingDir, 'modules', runtimeModuleId);
const supportModuleRoot = join(packageStagingDir, 'modules', supportModuleId);
const generatedAt = '2026-01-01T00:00:00Z';

function clean() {
  if (existsSync(distDir)) rmSync(distDir, { recursive: true, force: true });
  if (existsSync(packageStagingDir)) rmSync(packageStagingDir, { recursive: true, force: true });
  mkdirSync(packageOutputDir, { recursive: true });
  if (existsSync(outputFile)) rmSync(outputFile, { force: true });

}

function build() {
  console.log('Building TypeScript...');
  try {
    execFileSync(process.execPath, [join(projectRoot, 'node_modules', 'typescript', 'bin', 'tsc'), '-p', projectRoot], {
      cwd: projectRoot,
      stdio: 'inherit',
    });
  } catch {
    console.log('tsc reported type errors; emitted JS output is present, continuing package build.');
  }
}

function copyRecursively(src, dest) {
  const stat = statSync(src);
  if (stat.isDirectory()) {
    mkdirSync(dest, { recursive: true });
    for (const entry of readdirSync(src)) copyRecursively(join(src, entry), join(dest, entry));
    return;
  }
  mkdirSync(dirname(dest), { recursive: true });
  copyFileSync(src, dest);
}

function collectFiles(dir) {
  const out = [];
  const walk = current => {
    for (const entry of readdirSync(current).sort()) {
      const full = join(current, entry);
      if (statSync(full).isDirectory()) walk(full);
      else out.push(full);
    }
  };
  walk(dir);
  return out;
}

function sha256Raw(buffer) {
  return createHash('sha256').update(buffer).digest('hex');
}

function canonicalPackagePath(file) {
  return relative(packageStagingDir, file).replace(/\\/g, '/');
}

function createPayloadLayout() {
  mkdirSync(runtimeModuleRoot, { recursive: true });
  copyRecursively(distDir, join(runtimeModuleRoot, 'dist'));

  // Keep runtime dependencies inside the module directory. Node resolves
  // modules/<moduleId>/dist/index.js -> modules/<moduleId>/node_modules/...,
  // while the canonical .amitiax package exposes no ambient repository files.
  const allowedRuntimePackages = ['@amitia/game-plugin-sdk'];
  for (const pkg of allowedRuntimePackages) {
    const installed = join(projectRoot, 'node_modules', ...pkg.split('/'));
    const destination = join(runtimeModuleRoot, 'node_modules', ...pkg.split('/'));
    if (existsSync(installed)) {
      copyRecursively(installed, destination);
      continue;
    }

    // The fixture intentionally vendors the public SDK tarball so canonical
    // package construction does not depend on a developer workstation having
    // already populated node_modules. CI still runs npm ci and therefore uses
    // the normal installed dependency path.
    if (pkg !== '@amitia/game-plugin-sdk') throw new Error(`required runtime dependency is missing: ${pkg}`);
    const archive = join(projectRoot, 'vendor', 'amitia-game-plugin-sdk-0.1.0.tgz');
    if (!existsSync(archive)) throw new Error(`required runtime dependency is missing: ${pkg}`);
    const extractRoot = join(packageStagingDir, '.vendor-sdk-extract');
    rmSync(extractRoot, { recursive: true, force: true });
    mkdirSync(extractRoot, { recursive: true });
    execFileSync('tar', ['-xzf', archive, '-C', extractRoot], { stdio: 'inherit' });
    const extracted = join(extractRoot, 'package');
    const metadata = JSON.parse(readFileSync(join(extracted, 'package.json'), 'utf8'));
    if (metadata.name !== pkg) throw new Error(`vendored runtime dependency identity mismatch: expected ${pkg}, got ${metadata.name || '<missing>'}`);
    copyRecursively(extracted, destination);
    rmSync(extractRoot, { recursive: true, force: true });
  }

  // Stage an independent second process module. The manifest deliberately
  // declares the main runtime before this dependency so the real topology
  // planner, rather than manifest order, is responsible for startup ordering.
  copyRecursively(runtimeModuleRoot, supportModuleRoot);

  const artifactSource = join(projectRoot, 'artifacts');
  if (existsSync(artifactSource)) {
    copyRecursively(artifactSource, join(packageStagingDir, 'artifacts'));
  }

  const manifest = JSON.parse(readFileSync(join(projectRoot, 'amitia-extension.json'), 'utf8'));
  if (packageVersionOverride) manifest.extension.version = packageVersionOverride;
  if (requireCompanionArtifact) {
    const gameContribution = manifest.modules
      ?.flatMap(module => module.contributions || [])
      .find(contribution => contribution.kind === 'gamex');
    const artifact = gameContribution?.spec?.artifacts
      ?.find(item => item.id === 'worldgame-companion-file');
    if (!artifact) throw new Error('required-artifact fixture needs mock-companion-file in canonical source manifest');
    artifact.required = true;
  }
  manifest.integrity ??= {};
  manifest.integrity.algorithm = 'sha256';

  // The canonical tree includes manifest.json itself, so embedding the final
  // tree hash into that same file would be circular. The backend package parser
  // binds an empty manifest hash to integrity/content-tree.json before semantic
  // validation, then verifies the complete tree independently.
  manifest.integrity.contentTreeHash = '';
  writeFileSync(join(packageStagingDir, 'manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`);
}

function buildIntegrityDocuments() {
  const integrityDir = join(packageStagingDir, 'integrity');
  mkdirSync(integrityDir, { recursive: true });

  const payloadFiles = collectFiles(packageStagingDir).filter(file => {
    const name = canonicalPackagePath(file);
    return name !== 'integrity/files.json' && name !== 'integrity/content-tree.json' && !name.startsWith('signatures/') && name !== 'META-INF/amitia-signature.json';
  });

  const entries = payloadFiles.map(file => {
    const data = readFileSync(file);
    const path = canonicalPackagePath(file);
    return {
      path,
      size: data.length,
      hash: sha256Raw(data),
      modified: generatedAt,
    };
  }).sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));

  const files = {};
  for (const entry of entries) files[entry.path] = entry;
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

function crc32(buf) {
  let crc = 0xffffffff;
  for (let i = 0; i < buf.length; i++) {
    crc ^= buf[i];
    for (let j = 0; j < 8; j++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function createZip() {
  console.log('Creating canonical .amitiax extension package...');
  const localHeaders = [];
  const centralDirectory = [];
  let offset = 0;

  function addFile(filePath, archivePath) {
    const content = readFileSync(filePath);
    const nameBuffer = Buffer.from(archivePath, 'utf8');
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
    localHeader.writeUInt16LE(nameBuffer.length, 26);
    const localEntry = Buffer.concat([localHeader, nameBuffer, compressed]);
    localHeaders.push(localEntry);

    const centralHeader = Buffer.alloc(46);
    centralHeader.writeUInt32LE(0x02014b50, 0);
    centralHeader.writeUInt16LE(20, 4);
    centralHeader.writeUInt16LE(20, 6);
    centralHeader.writeUInt16LE(0, 8);
    centralHeader.writeUInt16LE(8, 10);
    centralHeader.writeUInt32LE(crc, 16);
    centralHeader.writeUInt32LE(compressed.length, 20);
    centralHeader.writeUInt32LE(content.length, 24);
    centralHeader.writeUInt16LE(nameBuffer.length, 28);
    centralHeader.writeUInt32LE(offset, 42);
    centralDirectory.push(Buffer.concat([centralHeader, nameBuffer]));
    offset += localEntry.length;
  }

  for (const file of collectFiles(packageStagingDir)) addFile(file, canonicalPackagePath(file));

  const centralStart = offset;
  const centralSize = centralDirectory.reduce((sum, entry) => sum + entry.length, 0);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(centralDirectory.length, 8);
  end.writeUInt16LE(centralDirectory.length, 10);
  end.writeUInt32LE(centralSize, 12);
  end.writeUInt32LE(centralStart, 16);
  writeFileSync(outputFile, Buffer.concat([...localHeaders, ...centralDirectory, end]));
  console.log(`Package created: ${outputFile}`);
}

clean();
build();
createPayloadLayout();
const treeHash = buildIntegrityDocuments();
createZip();
rmSync(packageStagingDir, { recursive: true, force: true });
console.log(`Canonical content tree: ${treeHash}`);
