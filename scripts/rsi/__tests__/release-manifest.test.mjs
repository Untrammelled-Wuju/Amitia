import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { buildManifest } from '../release-manifest.mjs'

function fixture(overrides = {}) {
  const root = mkdtempSync(resolve(tmpdir(), 'rsi-manifest-'))
  const files = {
    'backend/go.mod': 'module example\n\ngo 1.26.1\n',
    'backend/go.sum': '',
    'front/package.json': JSON.stringify({ version: '1.0.0' }),
    'front/pnpm-lock.yaml': 'lockfileVersion: 9\n',
    'desktop/package.json': JSON.stringify({ version: '1.0.0' }),
    'desktop/pnpm-lock.yaml': 'lockfileVersion: 9\n',
    'mobile_app/pubspec.yaml': 'name: demo\nversion: 1.0.0+7\n',
    'mobile_app/pubspec.lock': '',
    'contracts/ui-runtime/v1/schema.json': JSON.stringify({ title: 'demo' }),
    ...overrides
  }
  for (const [relativePath, contents] of Object.entries(files)) {
    if (contents === null) continue
    const absolute = resolve(root, relativePath)
    mkdirSync(dirname(absolute), { recursive: true })
    writeFileSync(absolute, contents, 'utf8')
  }
  return root
}

test('多端版本不一致时阻断且不产出清单', () => {
  const root = fixture({ 'desktop/package.json': JSON.stringify({ version: '1.0.1' }) })
  const result = buildManifest(root)
  assert.equal(result.ok, false)
  assert.equal(result.manifest, null)
  assert.ok(result.violations.some((item) => item.code === 'VERSION_MISMATCH'))
})

test('缺少平台依赖文件时阻断', () => {
  const root = fixture({ 'backend/go.sum': null })
  const result = buildManifest(root)
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'PLATFORM_FILE_MISSING'))
})

test('契约目录为空时阻断', () => {
  const root = fixture()
  const empty = fixture({ 'contracts/ui-runtime/v1/schema.json': null })
  const result = buildManifest(empty)
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'CONTRACT_EMPTY'))
  assert.equal(buildManifest(root).ok, true)
})

test('版本一致时产出稳定清单哈希', () => {
  const root = fixture()
  const first = buildManifest(root)
  const second = buildManifest(root)
  assert.equal(first.ok, true)
  assert.equal(first.manifest.manifestHash, second.manifest.manifestHash)
  assert.equal(first.manifest.contractCount, 1)
})

test('移动端 build 号不参与版本一致性比较', () => {
  const root = fixture({ 'mobile_app/pubspec.yaml': 'name: demo\nversion: 1.0.0+29\n' })
  const result = buildManifest(root)
  assert.equal(result.ok, true, JSON.stringify(result.violations))
})
