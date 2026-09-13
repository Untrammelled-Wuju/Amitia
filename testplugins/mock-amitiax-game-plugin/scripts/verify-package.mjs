import { createHash } from 'node:crypto';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { inflateRawSync } from 'node:zlib';

const __dirname = dirname(fileURLToPath(import.meta.url));
const packageDir = resolve(__dirname, '..', 'dist-package');
const packagePath = join(packageDir, process.env.MOCK_PLUGIN_OUTPUT || 'world-game-plugin.amitiax');
const expectRequiredArtifact = /^(1|true|yes)$/i.test((process.env.MOCK_PLUGIN_REQUIRED_ARTIFACT || '').trim());
const errors = [];
const runtimeModulePrefix = 'modules/world-game-runtime/';
const supportModulePrefix = 'modules/world-game-support/';

function readZipEntries(buffer) {
  const entries = new Map();
  let eocd = -1;
  for (let i = buffer.length - 22; i >= Math.max(0, buffer.length - 65557); i--) {
    if (buffer.readUInt32LE(i) === 0x06054b50) { eocd = i; break; }
  }
  if (eocd < 0) throw new Error('ZIP EOCD not found');
  const count = buffer.readUInt16LE(eocd + 10);
  let cursor = buffer.readUInt32LE(eocd + 16);
  for (let i = 0; i < count; i++) {
    if (buffer.readUInt32LE(cursor) !== 0x02014b50) throw new Error('invalid central directory entry');
    const method = buffer.readUInt16LE(cursor + 10);
    const compressedSize = buffer.readUInt32LE(cursor + 20);
    const nameLen = buffer.readUInt16LE(cursor + 28);
    const extraLen = buffer.readUInt16LE(cursor + 30);
    const commentLen = buffer.readUInt16LE(cursor + 32);
    const localOffset = buffer.readUInt32LE(cursor + 42);
    const name = buffer.subarray(cursor + 46, cursor + 46 + nameLen).toString('utf8');
    if (buffer.readUInt32LE(localOffset) !== 0x04034b50) throw new Error(`invalid local header for ${name}`);
    const localNameLen = buffer.readUInt16LE(localOffset + 26);
    const localExtraLen = buffer.readUInt16LE(localOffset + 28);
    const dataStart = localOffset + 30 + localNameLen + localExtraLen;
    const compressed = buffer.subarray(dataStart, dataStart + compressedSize);
    const data = method === 8 ? inflateRawSync(compressed) : method === 0 ? compressed : (() => { throw new Error(`unsupported method ${method}`); })();
    if (entries.has(name)) throw new Error(`duplicate ZIP entry: ${name}`);
    entries.set(name, data);
    cursor += 46 + nameLen + extraLen + commentLen;
  }
  return entries;
}

function sha256Raw(buffer) {
  return createHash('sha256').update(buffer).digest('hex');
}

function allowedPath(name) {
  return name === 'manifest.json'
    || name === 'integrity/files.json'
    || name === 'integrity/content-tree.json'
    || name.startsWith(runtimeModulePrefix)
    || name.startsWith(supportModulePrefix)
    || name.startsWith('artifacts/');
}

function computeTree(entries, integrityFiles) {
  const canonical = [];
  for (const [name, data] of entries) {
    if (name === 'integrity/files.json' || name === 'integrity/content-tree.json' || name.startsWith('signatures/') || name === 'META-INF/amitia-signature.json') continue;
    const hash = sha256Raw(data);
    const declared = integrityFiles[name];
    if (!declared) errors.push(`integrity/files.json missing ${name}`);
    else {
      if (declared.path !== name) errors.push(`integrity path mismatch for ${name}`);
      if (declared.hash !== hash) errors.push(`integrity hash mismatch for ${name}`);
      if (declared.size !== data.length) errors.push(`integrity size mismatch for ${name}`);
    }
    canonical.push({ path: name, hash });
  }
  canonical.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
  const h = createHash('sha256');
  for (const entry of canonical) {
    h.update(entry.path);
    h.update(Buffer.from([0]));
    h.update(entry.hash);
    h.update(Buffer.from([0]));
  }
  return h.digest('hex');
}

if (!existsSync(packagePath)) {
  console.error('final .amitiax package missing; run npm run package first');
  process.exit(1);
}

for (const legacyArtifact of [
  'amitia-extension.json',
  'checksum.txt',
  'dist',
  'artifacts',
  'node_modules',
  'world-game-plugin.zip',
  'mock-amitiax-game-plugin.zip',
  'mock-amitiax-game-plugin.amitiax',
]) {
  if (existsSync(join(packageDir, legacyArtifact))) {
    errors.push(`legacy package output is forbidden: ${legacyArtifact}`);
  }
}

try {
  const entries = readZipEntries(readFileSync(packagePath));
  for (const name of entries.keys()) if (!allowedPath(name)) errors.push(`unexpected canonical package path: ${name}`);
  for (const required of [
    'manifest.json',
    'integrity/files.json',
    'integrity/content-tree.json',
    `${runtimeModulePrefix}dist/index.js`,
    `${runtimeModulePrefix}node_modules/@amitia/game-plugin-sdk/package.json`,
    `${supportModulePrefix}dist/support.js`,
    `${supportModulePrefix}node_modules/@amitia/game-plugin-sdk/package.json`,
    'artifacts/worldgame-companion.txt',
  ]) {
    if (!entries.has(required)) errors.push(`${required} missing from final package`);
  }

  let manifest;
  let filesDoc;
  let treeDoc;
  try { manifest = JSON.parse(entries.get('manifest.json')?.toString('utf8') || '{}'); } catch { errors.push('manifest.json is invalid JSON'); }
  try { filesDoc = JSON.parse(entries.get('integrity/files.json')?.toString('utf8') || '{}'); } catch { errors.push('integrity/files.json is invalid JSON'); }
  try { treeDoc = JSON.parse(entries.get('integrity/content-tree.json')?.toString('utf8') || '{}'); } catch { errors.push('integrity/content-tree.json is invalid JSON'); }

  if (manifest?.manifestVersion !== 1) errors.push('manifestVersion must be 1');
  if (manifest?.extension?.id !== 'com.amitia/world-game-plugin') errors.push('unexpected extension id');
  const gameContribution = manifest?.modules
    ?.flatMap(module => module?.contributions || [])
    .find(contribution => contribution?.kind === 'gamex');
  const services = gameContribution?.spec?.services || [];
  const companionArtifact = gameContribution?.spec?.artifacts
    ?.find(artifact => artifact?.id === 'worldgame-companion-file');
  if (!manifest?.modules?.some(module => module?.id === 'world-game-support')) errors.push('world-game-support module missing from canonical manifest');
  if (!services.some(service => service?.id === 'world-game-support' && service?.required === true)) errors.push('required world-game-support service missing from canonical game_plugin spec');
  if (!companionArtifact) errors.push('worldgame-companion-file declaration missing from canonical game_plugin spec');
  else if (Boolean(companionArtifact.required) !== expectRequiredArtifact) errors.push(`worldgame-companion-file required=${Boolean(companionArtifact.required)} does not match expected ${expectRequiredArtifact}`);
  if (manifest?.integrity?.algorithm !== 'sha256') errors.push('manifest integrity algorithm must be sha256');
  if (manifest?.integrity?.contentTreeHash !== '') errors.push('manifest contentTreeHash must be empty for canonical tree binding');
  if (filesDoc?.algorithm !== 'sha256' || !filesDoc?.files || typeof filesDoc.files !== 'object') errors.push('invalid integrity/files.json');
  if (treeDoc?.algorithm !== 'sha256' || typeof treeDoc?.treeHash !== 'string' || !treeDoc.treeHash) errors.push('invalid integrity/content-tree.json');

  if (filesDoc?.files && treeDoc?.treeHash) {
    const expectedNames = [...entries.keys()].filter(name => name !== 'integrity/files.json' && name !== 'integrity/content-tree.json' && !name.startsWith('signatures/') && name !== 'META-INF/amitia-signature.json').sort();
    const declaredNames = Object.keys(filesDoc.files).sort();
    if (JSON.stringify(expectedNames) !== JSON.stringify(declaredNames)) errors.push('integrity/files.json does not exactly cover canonical payload files');
    const actualTree = computeTree(entries, filesDoc.files);
    if (actualTree !== treeDoc.treeHash) errors.push(`content tree mismatch: expected ${treeDoc.treeHash}, got ${actualTree}`);
  }
} catch (error) {
  errors.push(error?.stack || String(error));
}

if (errors.length) {
  console.error('Package verification failed:');
  for (const error of errors) console.error(`  - ${error}`);
  process.exit(1);
}

console.log(`Package verification passed: ${packagePath}`);
