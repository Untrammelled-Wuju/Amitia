import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { dirname, resolve, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'
import { apkFileInfo, hashFile, verifyApkSignature, verifyLatestYml, verifyPackageReceipts } from './release-signature.mjs'

const PLATFORM_FILES = [
  'backend/go.mod',
  'backend/go.sum',
  'front/package.json',
  'front/pnpm-lock.yaml',
  'desktop/package.json',
  'desktop/pnpm-lock.yaml',
  'mobile_app/pubspec.yaml',
  'mobile_app/pubspec.lock'
]

const VERSION_SOURCES = [
  { platform: 'front', path: 'front/package.json', kind: 'json' },
  { platform: 'desktop', path: 'desktop/package.json', kind: 'json' },
  { platform: 'mobile', path: 'mobile_app/pubspec.yaml', kind: 'pubspec' }
]

function sha256(buffer) {
  return createHash('sha256').update(buffer).digest('hex')
}

function readJson(absolute) {
  return JSON.parse(readFileSync(absolute, 'utf8'))
}

export function readVersions(root = repoRoot) {
  return VERSION_SOURCES.map((source) => {
    const absolute = resolve(root, source.path)
    if (!existsSync(absolute)) {
      throw new Error(`缺少版本来源文件：${source.path}`)
    }
    const raw = readFileSync(absolute, 'utf8')
    if (source.kind === 'json') {
      const parsed = JSON.parse(raw)
      return { platform: source.platform, path: source.path, version: parsed.version ?? null }
    }
    const matched = raw.match(/^version:\s*(\S+)/m)
    const version = matched ? matched[1].split('+')[0] : null
    return { platform: source.platform, path: source.path, version }
  })
}

function collectFiles(root, relativeDir) {
  const absoluteDir = resolve(root, relativeDir)
  if (!existsSync(absoluteDir)) return []
  const collected = []
  const walk = (current) => {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const child = resolve(current, entry.name)
      if (entry.isDirectory()) walk(child)
      else if (entry.isFile()) collected.push(child)
    }
  }
  walk(absoluteDir)
  return collected.sort()
}

export function buildManifest(root = repoRoot) {
  const violations = []

  const versions = readVersions(root)
  const missingVersions = versions.filter((item) => !item.version).map((item) => item.path)
  if (missingVersions.length > 0) {
    violations.push({ code: 'VERSION_MISSING', message: `版本字段缺失：${missingVersions.join(', ')}` })
  }
  const distinct = [...new Set(versions.map((item) => item.version).filter(Boolean))]
  if (distinct.length > 1) {
    violations.push({
      code: 'VERSION_MISMATCH',
      message: `多端版本不一致（单版本策略要求完全一致）：${versions.map((item) => `${item.platform}=${item.version}`).join(', ')}`
    })
  }

  const platformFiles = PLATFORM_FILES.map((relativePath) => {
    const absolute = resolve(root, relativePath)
    if (!existsSync(absolute)) {
      violations.push({ code: 'PLATFORM_FILE_MISSING', message: `缺少平台依赖文件：${relativePath}` })
      return null
    }
    const buffer = readFileSync(absolute)
    return { path: relativePath, bytes: buffer.length, sha256: sha256(buffer) }
  }).filter(Boolean)

  const contractFiles = collectFiles(root, 'contracts').map((absolute) => {
    const buffer = readFileSync(absolute)
    if (absolute.endsWith('.json')) {
      try {
        JSON.parse(buffer.toString('utf8'))
      } catch (error) {
        throw new Error(`契约文件 JSON 解析失败 ${relative(root, absolute)}：${error.message}`, { cause: error })
      }
    }
    return { path: relative(root, absolute), bytes: buffer.length, sha256: sha256(buffer) }
  })
  if (contractFiles.length === 0) {
    violations.push({ code: 'CONTRACT_EMPTY', message: 'contracts 目录缺失或为空，无法生成契约哈希清单' })
  }

  const payload = {
    schemaVersion: 1,
    versions,
    platformFiles,
    contractFiles,
    contractCount: contractFiles.length
  }

  if (violations.length > 0) {
    return { ok: false, violations, manifest: null }
  }

  return { ok: true, violations: [], manifest: { ...payload, manifestHash: sha256(JSON.stringify(payload)) } }
}

export async function buildReleaseManifest(root, options = {}) {
  const base = buildManifest(root)
  const violations = [...base.violations]
  const packages = []
  let apkSignature = null
  let latestYml = null

  if (options.releaseDir) {
    const verified = await verifyLatestYml(options.releaseDir)
    violations.push(...verified.violations)
    packages.push(...verified.packages)
    latestYml = { version: verified.latest.version, files: verified.latest.files }
    if (verified.latest.version && base.manifest) {
      const mismatch = base.manifest.versions.filter((item) => item.version !== verified.latest.version)
      if (mismatch.length > 0) {
        violations.push({
          code: 'RELEASE_VERSION_MISMATCH',
          message: `latest.yml 版本 ${verified.latest.version} 与源码版本不一致：${mismatch.map((item) => `${item.platform}=${item.version}`).join(', ')}`
        })
      }
    }
  }

  if (options.apk) {
    const signature = verifyApkSignature(options.apk, { apksignerPath: options.apksignerPath })
    violations.push(...signature.violations)
    const info = apkFileInfo(options.apk)
    if (info) packages.push({ platform: 'android-arm64', ...info, ...(await hashFile(options.apk)) })
    apkSignature = { ok: signature.ok, signers: signature.signers ?? [], apksigner: signature.apksigner ?? null }
  }

  if (options.receipts && options.receipts.length > 0) {
    const verified = await verifyPackageReceipts(options.receipts, options)
    violations.push(...verified.violations)
    packages.push(...verified.packages)
  }

  const uniquePackages = []
  for (const item of packages) {
    const duplicateFile = uniquePackages.find((entry) => entry.file === item.file && entry.sha256 === item.sha256)
    if (duplicateFile) continue
    const samePlatform = uniquePackages.find((entry) => entry.platform === item.platform)
    if (samePlatform) {
      violations.push({
        code: 'DUPLICATE_PLATFORM',
        message: `平台 ${item.platform} 声明了多个不同发布包：${samePlatform.file} / ${item.file}`
      })
      continue
    }
    uniquePackages.push(item)
  }

  if (violations.length > 0) {
    return { ok: false, violations, manifest: null }
  }

  const payload = { ...base.manifest, latestYml, packages: uniquePackages, apkSignature }
  return { ok: true, violations: [], manifest: { ...payload, manifestHash: sha256(JSON.stringify(payload)) } }
}

function parseArgs(argv) {
  const options = { out: 'artifacts/rsi/release-manifest.json', receipts: [] }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === '--out') options.out = argv[index + 1]
    else if (arg === '--release-dir') options.releaseDir = argv[index + 1]
    else if (arg === '--apk') options.apk = argv[index + 1]
    else if (arg === '--apksigner') options.apksignerPath = argv[index + 1]
    else if (arg === '--receipt') options.receipts.push(...JSON.parse(readFileSync(argv[index + 1], 'utf8')).packages ?? [])
  }
  return options
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  const result = await buildReleaseManifest(repoRoot, options)
  if (!result.ok) {
    process.stderr.write(`发布清单生成被阻断：\n${result.violations.map((item) => `- [${item.code}] ${item.message}`).join('\n')}\n`)
    process.exit(5)
  }
  const target = resolve(repoRoot, options.out)
  mkdirSync(dirname(target), { recursive: true })
  writeFileSync(target, `${JSON.stringify(result.manifest, null, 2)}\n`, 'utf8')
  process.stdout.write(
    `${JSON.stringify(
      {
        out: target,
        manifestHash: result.manifest.manifestHash,
        versions: result.manifest.versions,
        contracts: result.manifest.contractCount,
        packages: result.manifest.packages.map((item) => ({ platform: item.platform, bytes: item.bytes, sha256: item.sha256 })),
        apkSignature: result.manifest.apkSignature
      },
      null,
      2
    )}\n`
  )
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    process.stderr.write(`发布清单生成失败：${error.message}\n${error.stack ?? ''}\n`)
    process.exit(5)
  })
}
