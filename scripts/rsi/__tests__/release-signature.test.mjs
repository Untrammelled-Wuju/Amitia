import assert from 'node:assert/strict'
import { createHash, generateKeyPairSync, sign } from 'node:crypto'
import { chmodSync, mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import test from 'node:test'
import {
  hashFile,
  inferPlatform,
  parseLatestYml,
  verifyApkSignature,
  verifyEd25519Signature,
  verifyLatestYml,
  verifyPackageReceipts
} from '../release-signature.mjs'

function workspace() {
  return mkdtempSync(resolve(tmpdir(), 'rsi-signature-'))
}

test('latest.yml 解析出版本、文件、sha512 与大小', () => {
  const parsed = parseLatestYml(
    ['version: 1.2.3', 'files:', '  - url: app.exe', '    sha512: abc', '    size: 12', 'path: app.exe'].join('\n')
  )
  assert.equal(parsed.version, '1.2.3')
  assert.deepEqual(parsed.files, [{ url: 'app.exe', sha512: 'abc', size: 12 }])
})

test('发布包哈希与大小一致时通过，篡改后阻断并保留原始值', async () => {
  const root = workspace()
  const file = resolve(root, 'AmitiaSetup.exe')
  writeFileSync(file, 'payload')
  const actual = await hashFile(file)
  writeFileSync(
    resolve(root, 'latest.yml'),
    ['version: 1.0.0', 'files:', '  - url: AmitiaSetup.exe', `    sha512: ${actual.sha512Base64}`, `    size: ${actual.bytes}`].join('\n')
  )
  const ok = await verifyLatestYml(root)
  assert.deepEqual(ok.violations, [])
  assert.equal(ok.packages.length, 1)

  writeFileSync(file, 'payload-tampered')
  const tampered = await verifyLatestYml(root)
  assert.ok(tampered.violations.some((item) => item.code === 'PACKAGE_HASH_MISMATCH'))
  assert.ok(tampered.violations.some((item) => item.code === 'PACKAGE_SIZE_MISMATCH'))
})

test('latest.yml 声明不存在的包时阻断', async () => {
  const root = workspace()
  writeFileSync(resolve(root, 'latest.yml'), ['version: 1.0.0', 'files:', '  - url: missing.exe', '    sha512: x', '    size: 1'].join('\n'))
  const result = await verifyLatestYml(root)
  assert.ok(result.violations.some((item) => item.code === 'PACKAGE_MISSING'))
})

test('发布包平台可识别，未知平台阻断清单', async () => {
  assert.equal(inferPlatform('Amitia-26.2.0-x64.exe'), 'win32-x64')
  assert.equal(inferPlatform('Amitia-26.2.0-arm64.dmg'), 'darwin-arm64')
  assert.equal(inferPlatform('Amitia-26.2.0.AppImage'), 'linux-x64')
  assert.equal(inferPlatform('Amitia-26.2.0-x64.exe.blockmap'), null)

  const root = workspace()
  const file = resolve(root, 'package.bin')
  writeFileSync(file, 'payload')
  const actual = await hashFile(file)
  writeFileSync(
    resolve(root, 'latest.yml'),
    ['version: 1.0.0', 'files:', '  - url: package.bin', `    sha512: ${actual.sha512Base64}`, `    size: ${actual.bytes}`].join('\n')
  )
  const result = await verifyLatestYml(root)
  assert.ok(result.violations.some((item) => item.code === 'PLATFORM_UNKNOWN'))
})

test('Ed25519 验签通过，内容被改写后失败', () => {
  const root = workspace()
  const file = resolve(root, 'plugin.amitiax')
  writeFileSync(file, 'package-bytes')
  const { publicKey, privateKey } = generateKeyPairSync('ed25519')
  writeFileSync(resolve(root, 'pub.pem'), publicKey.export({ type: 'spki', format: 'pem' }))
  writeFileSync(resolve(root, 'sig.bin'), sign(null, Buffer.from('package-bytes'), privateKey))

  const ok = verifyEd25519Signature({ file, signaturePath: resolve(root, 'sig.bin'), publicKeyPath: resolve(root, 'pub.pem') })
  assert.equal(ok.ok, true)

  writeFileSync(file, 'package-bytes-tampered')
  const bad = verifyEd25519Signature({ file, signaturePath: resolve(root, 'sig.bin'), publicKeyPath: resolve(root, 'pub.pem') })
  assert.equal(bad.ok, false)
  assert.equal(bad.violations[0].code, 'SIGNATURE_INVALID')
})

test('缺少 apksigner 时按失败关闭阻断，不跳过签名校验', () => {
  const root = workspace()
  const apk = resolve(root, 'app-release.apk')
  writeFileSync(apk, 'apk')
  const result = verifyApkSignature(apk, { apksignerPath: resolve(root, 'no-apksigner') })
  assert.equal(result.ok, false)
  assert.equal(result.violations[0].code, 'APKSIGNER_MISSING')
})

test('apksigner 退出码非零时透传原始输出', () => {
  const root = workspace()
  const apk = resolve(root, 'app-release.apk')
  writeFileSync(apk, 'apk')
  const stub = resolve(root, 'apksigner')
  writeFileSync(stub, '#!/bin/sh\necho "ERROR: missing META-INF" >&2\nexit 1\n')
  chmodSync(stub, 0o755)
  const result = verifyApkSignature(apk, { apksignerPath: stub })
  assert.equal(result.ok, false)
  assert.equal(result.violations[0].code, 'APK_SIGNATURE_INVALID')
  assert.match(result.violations[0].message, /missing META-INF/)
})

test('apksigner 成功时解析签名者证书', () => {
  const root = workspace()
  const apk = resolve(root, 'app-release.apk')
  writeFileSync(apk, 'apk')
  const stub = resolve(root, 'apksigner')
  writeFileSync(stub, '#!/bin/sh\necho "Signer #1 certificate DN: CN=Amitia"\nexit 0\n')
  chmodSync(stub, 0o755)
  const result = verifyApkSignature(apk, { apksignerPath: stub })
  assert.equal(result.ok, true)
  assert.deepEqual(result.signers, ['Signer #1 certificate DN: CN=Amitia'])
})

test('未知平台或哈希不符的发布回执被阻断', async () => {
  const root = workspace()
  const file = resolve(root, 'artifact.pkg')
  writeFileSync(file, 'artifact')
  const actual = await hashFile(file)
  const unsupported = await verifyPackageReceipts([{ platform: 'solaris-sparc', file }])
  assert.ok(unsupported.violations.some((item) => item.code === 'PLATFORM_UNSUPPORTED'))
  const mismatch = await verifyPackageReceipts([{ platform: 'android-arm64', file, sha256: createHash('sha256').update('other').digest('hex') }])
  assert.ok(mismatch.violations.some((item) => item.code === 'PACKAGE_HASH_MISMATCH'))
  const ok = await verifyPackageReceipts([{ platform: 'android-arm64', file, sha256: actual.sha256, size: actual.bytes }])
  assert.deepEqual(ok.violations, [])
  assert.equal(ok.packages[0].platform, 'android-arm64')
})

test('发布目录不存在 latest.yml 时抛出带路径的错误', async () => {
  const root = workspace()
  mkdirSync(resolve(root, 'release'), { recursive: true })
  await assert.rejects(() => verifyLatestYml(resolve(root, 'release')), /缺少 latest.yml/)
})
