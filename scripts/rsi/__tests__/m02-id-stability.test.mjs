import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { repoRoot } from '../toolchain-preflight.mjs'
import { GOLDEN_PATH, readGolden, renderListing, summarizeGolden, validateGolden } from '../m02-id-stability.mjs'

function writeTempGolden(tempRoot, snapshot) {
  const target = join(tempRoot, GOLDEN_PATH)
  mkdirSync(join(target, '..'), { recursive: true })
  writeFileSync(target, `${JSON.stringify(snapshot)}\n`, 'utf8')
}

const sampleEntry = {
  source: 'builtin.core',
  providerId: 'builtin.core/core/memory.search',
  serviceId: 'memory.search',
  capabilityId: 'builtin/core/memory.search',
  version: '1.0.0'
}

test('readGolden 读出真实金标准快照', () => {
  const snapshot = readGolden(repoRoot)
  assert.equal(snapshot.schemaVersion, 1)
  assert.equal(snapshot.entries.length, snapshot.count)
  assert.ok(snapshot.entries.length >= 2)
})

test('真实金标准只覆盖内置 core 与随仓库发布的官方插件', () => {
  const snapshot = readGolden(repoRoot)
  assert.deepEqual(validateGolden(snapshot), [])
  const sources = summarizeGolden(snapshot).sources
  assert.ok(sources['builtin.core'] >= 1)
  assert.ok(Object.keys(sources).some((key) => key.startsWith('manifest.')))
  assert.deepEqual(snapshot.scope, ['builtin.core', 'plugins-code/*/amitia-extension.json'])
})

test('readGolden 在文件缺失时给出明确错误', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-id-missing-'))
  assert.throws(() => readGolden(dir), /id stability golden missing/)
})

test('validateGolden 抓出计数不符、重复条目与非法版本', () => {
  const problems = validateGolden({
    schemaVersion: 1,
    scope: ['builtin.core'],
    count: 9,
    entries: [sampleEntry, { ...sampleEntry, version: 'v1' }]
  })
  assert.ok(problems.some((line) => line.includes('count=9')), problems.join('; '))
  assert.ok(problems.some((line) => line.startsWith('duplicate entry')), problems.join('; '))
  assert.ok(problems.some((line) => line.includes('is not dotted numeric')), problems.join('; '))
})

test('validateGolden 抓出空 scope 与非数组 entries', () => {
  assert.ok(validateGolden({ schemaVersion: 1, scope: [], count: 0, entries: [] }).includes('scope is empty'))
  assert.deepEqual(validateGolden({ schemaVersion: 2, scope: [], entries: null }), ['schemaVersion=2 want 1', 'entries is not an array'])
})

test('writeTempGolden 副本能被 readGolden 读回（临时目录隔离）', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-id-temp-'))
  const snapshot = { schemaVersion: 1, scope: ['builtin.core'], count: 1, entries: [sampleEntry] }
  writeTempGolden(dir, snapshot)
  assert.deepEqual(readGolden(dir), snapshot)
  assert.deepEqual(validateGolden(readGolden(dir)), [])
})

test('summarizeGolden 统计来源并保留能力 ID 顺序', () => {
  const summary = summarizeGolden({
    schemaVersion: 1,
    scope: ['builtin.core'],
    count: 2,
    entries: [sampleEntry, { ...sampleEntry, source: 'manifest.uiProvider', capabilityId: 'com.amitia/emote/emote-ui/x', providerId: 'com.amitia/emote/emote-ui/x.provider', serviceId: 'x' }]
  })
  assert.equal(summary.count, 2)
  assert.deepEqual(summary.sources, { 'builtin.core': 1, 'manifest.uiProvider': 1 })
  assert.deepEqual(summary.capabilityIds, ['builtin/core/memory.search', 'com.amitia/emote/emote-ui/x'])
})

test('renderListing 逐条输出来源/能力/版本/provider 四列', () => {
  const listing = renderListing({ entries: [sampleEntry] })
  assert.match(listing, /^golden=backend\/internal\/extension\/kernel\/plugincontract\/testdata\/id-stability\.json entries=1\n/)
  assert.match(listing, /^builtin\.core\tbuiltin\/core\/memory\.search\t1\.0\.0\tbuiltin\.core\/core\/memory\.search$/m)
})
