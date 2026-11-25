import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  GOLDEN_PATH,
  parseGoTestOutput,
  readGolden,
  renderListing,
  summarizeGolden,
  validateGolden
} from '../m02-provider-sync.mjs'

function writeTempGolden(tempRoot, golden) {
  const target = join(tempRoot, GOLDEN_PATH)
  mkdirSync(join(target, '..'), { recursive: true })
  writeFileSync(target, `${JSON.stringify(golden)}\n`, 'utf8')
}

const cleanGolden = {
  schemaVersion: 1,
  extensionId: 'com.amitia/emote',
  declaredProviders: 1,
  runtimeProviders: 1,
  gaps: []
}

const blockingGap = {
  kind: 'runtime.undeclared',
  extensionId: 'com.amitia/emote',
  moduleId: 'emote-runtime',
  capabilityId: 'com.amitia/emote/ghost',
  providerId: 'extprov_orphan',
  detail: 'runtime provider extprov_orphan has no manifest declaration for capability com.amitia/emote/ghost'
}

const advisoryGap = {
  kind: 'declared.missing',
  extensionId: 'com.amitia/emote',
  moduleId: 'emote-runtime',
  capabilityId: 'com.amitia/emote/ai',
  detail: 'manifest declares capability com.amitia/emote/ai for module com.amitia/emote/emote-runtime but no runtime provider is registered'
}

test('GOLDEN_PATH 指向 kernel testdata 下的 provider-conformance 金标准', () => {
  assert.equal(GOLDEN_PATH, 'backend/internal/extension/kernel/testdata/provider-conformance.json')
})

test('readGolden 读回临时目录中的金标准副本', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-provider-temp-'))
  writeTempGolden(dir, { ...cleanGolden, declaredProviders: 2, runtimeProviders: 3 })
  const golden = readGolden(dir)
  assert.equal(golden.extensionId, 'com.amitia/emote')
  assert.equal(golden.declaredProviders, 2)
  assert.equal(golden.runtimeProviders, 3)
  assert.deepEqual(golden.gaps, [])
})

test('readGolden 在文件缺失时报错且错误文案含完整路径', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-provider-missing-'))
  assert.throws(() => readGolden(dir), (error) => {
    assert.match(error.message, /provider conformance golden missing/)
    assert.ok(error.message.includes(join(dir, GOLDEN_PATH)), error.message)
    return true
  })
})

test('validateGolden 对合法金标准返回空问题数组', () => {
  assert.deepEqual(validateGolden(cleanGolden), [])
  assert.deepEqual(validateGolden({ ...cleanGolden, gaps: [blockingGap, advisoryGap] }), [])
})

test('validateGolden 抓出 schemaVersion 不符', () => {
  const problems = validateGolden({ ...cleanGolden, schemaVersion: 2 })
  assert.ok(problems.includes('schemaVersion=2 want 1'), problems.join('; '))
})

test('validateGolden 抓出计数为负数或非整数', () => {
  const problems = validateGolden({ ...cleanGolden, declaredProviders: -1, runtimeProviders: 1.5 })
  assert.ok(problems.some((line) => line.includes('declaredProviders=-1')), problems.join('; '))
  assert.ok(problems.some((line) => line.includes('runtimeProviders=1.5')), problems.join('; '))
  assert.ok(problems.every((line) => line.includes('is not a non-negative integer')))
})

test('validateGolden 抓出 gaps 不是数组', () => {
  assert.deepEqual(validateGolden({ ...cleanGolden, gaps: null }), ['gaps is not an array'])
})

test('validateGolden 抓出非法 gap kind', () => {
  const problems = validateGolden({ ...cleanGolden, gaps: [{ ...blockingGap, kind: 'runtime.extra' }] })
  assert.ok(problems.some((line) => line.includes('kind=runtime.extra is not one of runtime.undeclared, declared.missing')), problems.join('; '))
})

test('validateGolden 抓出必填字段为空', () => {
  const problems = validateGolden({
    ...cleanGolden,
    extensionId: '   ',
    gaps: [{ ...blockingGap, capabilityId: '', detail: '' }]
  })
  assert.ok(problems.includes('extensionId is empty'), problems.join('; '))
  assert.ok(problems.includes('detail missing on gap 0'), problems.join('; '))
})

test('validateGolden 拒绝非对象金标准', () => {
  assert.deepEqual(validateGolden(null), ['golden is not an object'])
  assert.deepEqual(validateGolden([]), ['golden is not an object'])
})

test('summarizeGolden 同时统计阻断与提醒', () => {
  const summary = summarizeGolden({ ...cleanGolden, declaredProviders: 2, runtimeProviders: 3, gaps: [blockingGap, advisoryGap] })
  assert.deepEqual(summary, { declaredProviders: 2, runtimeProviders: 3, gapCount: 2, blockingCount: 1, advisoryCount: 1 })
})

test('summarizeGolden 只有提醒项时阻断数为 0', () => {
  const summary = summarizeGolden({ ...cleanGolden, gaps: [advisoryGap, { ...advisoryGap, capabilityId: 'com.amitia/emote/vision' }] })
  assert.equal(summary.gapCount, 2)
  assert.equal(summary.blockingCount, 0)
  assert.equal(summary.advisoryCount, 2)
})

test('summarizeGolden 只有阻断项时提醒数为 0', () => {
  const summary = summarizeGolden({ ...cleanGolden, gaps: [blockingGap] })
  assert.equal(summary.gapCount, 1)
  assert.equal(summary.blockingCount, 1)
  assert.equal(summary.advisoryCount, 0)
})

test('summarizeGolden 在 gaps 缺失时按零差异处理', () => {
  const summary = summarizeGolden({ schemaVersion: 1, extensionId: 'com.amitia/emote' })
  assert.deepEqual(summary, { declaredProviders: 0, runtimeProviders: 0, gapCount: 0, blockingCount: 0, advisoryCount: 0 })
})

test('renderListing 无差异时输出汇总与无差异', () => {
  const listing = renderListing(cleanGolden)
  const lines = listing.trimEnd().split('\n')
  assert.equal(lines.length, 2)
  assert.match(lines[0], /^golden=backend\/internal\/extension\/kernel\/testdata\/provider-conformance\.json 声明 1 \/ 运行 1 \/ 差异 0（阻断 0，提醒 0）$/)
  assert.equal(lines[1], '无差异')
})

test('renderListing 有差异时按 extensionId/moduleId/capabilityId/providerId/kind/detail 对齐分列', () => {
  const listing = renderListing({ ...cleanGolden, runtimeProviders: 2, gaps: [blockingGap, advisoryGap] })
  const lines = listing.trimEnd().split('\n')
  assert.match(lines[0], /差异 2（阻断 1，提醒 1）$/)
  assert.match(lines[1], /^extensionId +moduleId +capabilityId +providerId +kind +detail$/)
  assert.equal(lines[1].indexOf('kind'), lines[2].indexOf('runtime.undeclared'))
  assert.deepEqual(lines[2].split(/\s{2,}/), [
    'com.amitia/emote',
    'emote-runtime',
    'com.amitia/emote/ghost',
    'extprov_orphan',
    'runtime.undeclared',
    blockingGap.detail
  ])
  assert.ok(lines[3].includes(advisoryGap.detail))
  assert.ok(lines[3].includes('declared.missing'))
  assert.doesNotMatch(listing, /\|/)
})

test('parseGoTestOutput 统计 PASS 与 FAIL 行', () => {
  const log = [
    '=== RUN   TestProviderConformanceRealManifestHasNoGaps',
    '--- PASS: TestProviderConformanceRealManifestHasNoGaps (0.01s)',
    '=== RUN   TestProviderConformanceDetectsMissingRuntimeProvider',
    '--- PASS: TestProviderConformanceDetectsMissingRuntimeProvider (0.00s)',
    '=== RUN   TestProviderConformanceMatchesGolden',
    '--- FAIL: TestProviderConformanceMatchesGolden (0.02s)',
    'FAIL',
    'exit status 1'
  ].join('\n')
  assert.deepEqual(parseGoTestOutput(log), { passed: 2, failed: 1 })
})

test('parseGoTestOutput 在无输出或非字符串时返回零计数', () => {
  assert.deepEqual(parseGoTestOutput(''), { passed: 0, failed: 0 })
  assert.deepEqual(parseGoTestOutput(undefined), { passed: 0, failed: 0 })
  assert.deepEqual(parseGoTestOutput('ok  \tgithub.com/u-ai/backend/internal/extension/kernel\t0.032s'), { passed: 0, failed: 0 })
})
