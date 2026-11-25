import { spawnSync } from 'node:child_process'
import { createHash, verify as verifySignatureBytes } from 'node:crypto'
import { createReadStream, existsSync, readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv } from './toolchain-preflight.mjs'

export const SUPPORTED_PLATFORMS = ['darwin-arm64', 'darwin-x64', 'win32-x64', 'linux-x64', 'android-arm64', 'android-armv7']

export async function hashFile(filePath) {
  const absolute = resolve(filePath)
  if (!existsSync(absolute)) {
    throw new Error(`发布包不存在：${absolute}`)
  }
  const sha256 = createHash('sha256')
  const sha512 = createHash('sha512')
  let bytes = 0
  await new Promise((done, fail) => {
    const stream = createReadStream(absolute)
    stream.on('data', (chunk) => {
      bytes += chunk.length
      sha256.update(chunk)
      sha512.update(chunk)
    })
    stream.on('error', fail)
    stream.on('end', done)
  })
  return {
    path: absolute,
    bytes,
    sha256: sha256.digest('hex'),
    sha512Base64: sha512.digest('base64')
  }
}

export function parseLatestYml(text) {
  const version = text.match(/^version:\s*(\S+)\s*$/m)?.[1] ?? null
  const entries = []
  const filesBlock = text.split(/^files:\s*$/m)[1]
  if (filesBlock) {
    const block = filesBlock.split(/^\S/m)[0]
    const items = block.split(/^\s*-\s+/m).slice(1)
    for (const item of items) {
      const url = item.match(/^\s*url:\s*(\S+)\s*$/m)?.[1] ?? null
      const sha512 = item.match(/^\s*sha512:\s*(\S+)\s*$/m)?.[1] ?? null
      const size = item.match(/^\s*size:\s*(\d+)\s*$/m)?.[1] ?? null
      if (!url) continue
      entries.push({ url, sha512, size: size === null ? null : Number.parseInt(size, 10) })
    }
  }
  return { version, files: entries }
}

export function inferPlatform(url) {
  if (/\.blockmap$/i.test(url)) return null
  if (/ia32\.exe$/i.test(url)) return 'win32-ia32'
  if (/arm64\.exe$/i.test(url)) return 'win32-arm64'
  if (/\.exe$/i.test(url)) return 'win32-x64'
  if (/arm64\.dmg$/i.test(url)) return 'darwin-arm64'
  if (/\.dmg$/i.test(url)) return 'darwin-x64'
  if (/armeabi-v7a\.apk$/i.test(url)) return 'android-armv7'
  if (/\.apk$/i.test(url)) return 'android-arm64'
  if (/\.AppImage$/i.test(url)) return 'linux-x64'
  return null
}

export async function verifyLatestYml(releaseDir) {
  const directory = resolve(releaseDir)
  const latestPath = resolve(directory, 'latest.yml')
  if (!existsSync(latestPath)) {
    throw new Error(`缺少 latest.yml：${latestPath}`)
  }
  const latest = parseLatestYml(readFileSync(latestPath, 'utf8'))
  const violations = []
  const packages = []
  if (!latest.version) {
    violations.push({ code: 'LATEST_VERSION_MISSING', message: `${latestPath} 缺少 version 字段` })
  }
  if (latest.files.length === 0) {
    violations.push({ code: 'LATEST_FILES_EMPTY', message: `${latestPath} 未声明任何发布包` })
  }
  for (const entry of latest.files) {
    const filePath = resolve(directory, entry.url)
    if (!existsSync(filePath)) {
      violations.push({ code: 'PACKAGE_MISSING', message: `latest.yml 声明的发布包不存在：${filePath}` })
      continue
    }
    const platform = inferPlatform(entry.url)
    const auxiliary = /\.blockmap$/i.test(entry.url)
    if (!auxiliary && !platform) {
      violations.push({
        code: 'PLATFORM_UNKNOWN',
        message: `无法从发布包文件名识别平台：${entry.url}（允许清单 ${SUPPORTED_PLATFORMS.join(', ')}）`
      })
    } else if (platform && !SUPPORTED_PLATFORMS.includes(platform)) {
      violations.push({
        code: 'PLATFORM_UNSUPPORTED',
        message: `平台 ${platform} 不在允许清单 ${SUPPORTED_PLATFORMS.join(', ')}：${entry.url}`
      })
    }
    const actual = await hashFile(filePath)
    if (entry.size !== null && actual.bytes !== entry.size) {
      violations.push({
        code: 'PACKAGE_SIZE_MISMATCH',
        message: `${filePath} 实际 ${actual.bytes} 字节，latest.yml 声明 ${entry.size} 字节`
      })
    }
    if (entry.sha512 !== null && actual.sha512Base64 !== entry.sha512) {
      violations.push({
        code: 'PACKAGE_HASH_MISMATCH',
        message: `${filePath} sha512 实际 ${actual.sha512Base64}，latest.yml 声明 ${entry.sha512}`
      })
    }
    packages.push({ file: filePath, platform, declaredSha512: entry.sha512, declaredSize: entry.size, ...actual })
  }
  return { latest, packages, violations }
}

function resolveApksigner(explicit) {
  if (explicit) return resolve(explicit)
  const found = spawnSync('sh', ['-c', 'command -v apksigner'], { encoding: 'utf8', env: cleanEnv() })
  const fromPath = found.status === 0 ? found.stdout.trim() : ''
  return fromPath || null
}

export function verifyApkSignature(apkPath, options = {}) {
  const apk = resolve(apkPath)
  if (!existsSync(apk)) {
    return { ok: false, violations: [{ code: 'APK_MISSING', message: `APK 不存在：${apk}` }], signers: [] }
  }
  const apksigner = resolveApksigner(options.apksignerPath)
  if (!apksigner || !existsSync(apksigner)) {
    return {
      ok: false,
      violations: [
        {
          code: 'APKSIGNER_MISSING',
          message: `未找到 apksigner，无法校验 APK 签名：${apk}（请通过 --apksigner 指定 Android build-tools 中的 apksigner${apksigner ? `，给定路径不存在：${apksigner}` : ''}）`
        }
      ],
      signers: []
    }
  }
  const result = spawnSync(apksigner, ['verify', '--print-certs', apk], { encoding: 'utf8', env: cleanEnv() })
  if (result.error) {
    throw new Error(`执行 apksigner 失败：${result.error.message}`, { cause: result.error })
  }
  const signers = `${result.stdout}`
    .split('\n')
    .filter((line) => /certificate DN:/i.test(line))
    .map((line) => line.trim())
  if (result.status !== 0) {
    return {
      ok: false,
      violations: [
        {
          code: 'APK_SIGNATURE_INVALID',
          message: `apksigner verify 退出码 ${result.status}\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`
        }
      ],
      signers,
      apksigner
    }
  }
  return { ok: true, violations: [], signers, apksigner, stdout: result.stdout.trim() }
}

export function verifyEd25519Signature({ file, signaturePath, signatureBase64, publicKeyPath }) {
  const absolute = resolve(file)
  if (!existsSync(absolute)) {
    return { ok: false, violations: [{ code: 'SIGNED_FILE_MISSING', message: `待验签文件不存在：${absolute}` }] }
  }
  if (!publicKeyPath || !existsSync(resolve(publicKeyPath))) {
    return { ok: false, violations: [{ code: 'PUBLIC_KEY_MISSING', message: `公钥不存在：${publicKeyPath}` }] }
  }
  let signature = null
  if (signatureBase64) signature = Buffer.from(signatureBase64, 'base64')
  else if (signaturePath && existsSync(resolve(signaturePath))) signature = readFileSync(resolve(signaturePath))
  if (!signature || signature.length === 0) {
    return { ok: false, violations: [{ code: 'SIGNATURE_MISSING', message: `签名不存在：${signaturePath ?? '未提供 signatureBase64'}` }] }
  }
  const publicKey = readFileSync(resolve(publicKeyPath), 'utf8')
  const valid = verifySignatureBytes(null, readFileSync(absolute), publicKey, signature)
  return valid
    ? { ok: true, violations: [], algorithm: 'ed25519', file: absolute }
    : { ok: false, violations: [{ code: 'SIGNATURE_INVALID', message: `Ed25519 验签失败：${absolute}` }] }
}

export async function verifyPackageReceipts(receipts, options = {}) {
  const violations = []
  const packages = []
  for (const receipt of receipts) {
    if (!SUPPORTED_PLATFORMS.includes(receipt.platform)) {
      violations.push({
        code: 'PLATFORM_UNSUPPORTED',
        message: `平台 ${receipt.platform} 不在允许清单 ${SUPPORTED_PLATFORMS.join(', ')}`
      })
      continue
    }
    if (!receipt.file || !existsSync(resolve(receipt.file))) {
      violations.push({ code: 'PACKAGE_MISSING', message: `发布包不存在：${receipt.file}` })
      continue
    }
    const actual = await hashFile(receipt.file)
    if (receipt.sha256 && receipt.sha256 !== actual.sha256) {
      violations.push({
        code: 'PACKAGE_HASH_MISMATCH',
        message: `${actual.path} sha256 实际 ${actual.sha256}，清单声明 ${receipt.sha256}`
      })
    }
    if (receipt.size && receipt.size !== actual.bytes) {
      violations.push({
        code: 'PACKAGE_SIZE_MISMATCH',
        message: `${actual.path} 实际 ${actual.bytes} 字节，清单声明 ${receipt.size} 字节`
      })
    }
    if (receipt.signature) {
      const signature = verifyEd25519Signature({
        file: actual.path,
        signaturePath: receipt.signature.signaturePath,
        signatureBase64: receipt.signature.signatureBase64,
        publicKeyPath: receipt.signature.publicKeyPath ?? options.publicKeyPath
      })
      violations.push(...signature.violations)
    }
    packages.push({ platform: receipt.platform, declaredSha256: receipt.sha256 ?? null, ...actual, signed: Boolean(receipt.signature) })
  }
  return { packages, violations }
}

export function apkFileInfo(apkPath) {
  const absolute = resolve(apkPath)
  if (!existsSync(absolute)) return null
  const stat = statSync(absolute)
  return { path: absolute, bytes: stat.size, mtime: stat.mtime.toISOString() }
}
