import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { repoRoot } from '../toolchain-preflight.mjs'
import {
  EXIT_TARGET_PROFILE,
  GATES,
  GOLDEN_PATH,
  parseGoTestOutput,
  readMatrix,
  renderMatrix,
  summarizeMatrix,
  validateMatrix
} from '../m02-target-profile.mjs'

function writeTempMatrix(tempRoot, matrix) {
  const target = join(tempRoot, GOLDEN_PATH)
  mkdirSync(join(target, '..'), { recursive: true })
  writeFileSync(target, `${JSON.stringify(matrix)}\n`, 'utf8')
}

const cleanMatrix = {
  schemaVersion: 1,
  providerId: 'com.amitia/emote/emote-runtime/com.amitia/emote/ai',
  cases: [
    {
      name: 'macos-device-fully-granted',
      profile: { platform: 'macos', host: 'device', grantedPermissions: ['tool.invoke'], contractVersion: '0.1.0' },
      decision: 'candidate',
      candidates: ['com.amitia/emote/emote-runtime/com.amitia/emote/ai']
    },
    {
      name: 'macos-device-partial-permissions',
      profile: { platform: 'macos', host: 'device', grantedPermissions: [], contractVersion: '0.1.0' },
      decision: 'rejected',
      candidates: [],
      gate: 'permission',
      detail: 'missing required permissions: tool.invoke'
    }
  ]
}

test('GOLDEN_PATH 指向 plugincontract 下的过滤矩阵金标准', () => {
  assert.equal(GOLDEN_PATH, 'backend/internal/extension/kernel/plugincontract/testdata/binding-filter-matrix.json')
})

test('GATES 固定四道闸且顺序为 placement/platform/permission/contractVersion', () => {
  assert.deepEqual(GATES, ['placement', 'platform', 'permission', 'contractVersion'])
})

test('EXIT_TARGET_PROFILE 与既有退出码不冲突', () => {
  const used = [12, 15, 16, 17, 18, 20, 21, 22, 24, 25, 26]
  assert.equal(EXIT_TARGET_PROFILE, 27)
  assert.equal(used.includes(EXIT_TARGET_PROFILE), false)
})

test('readMatrix 读回临时目录中的矩阵副本', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-target-profile-'))
  writeTempMatrix(dir, cleanMatrix)
  const matrix = readMatrix(dir)
  assert.equal(matrix.providerId, cleanMatrix.providerId)
  assert.equal(matrix.cases.length, 2)
})

test('readMatrix 在文件缺失时报错且错误文案含完整路径', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-target-profile-missing-'))
  assert.throws(() => readMatrix(dir), (error) => error.message.includes(GOLDEN_PATH))
})

test('validateMatrix 接受干净矩阵', () => {
  assert.deepEqual(validateMatrix(cleanMatrix), [])
})

test('validateMatrix 拒绝非对象与非 1 schemaVersion', () => {
  assert.deepEqual(validateMatrix(null), ['matrix is not an object'])
  assert.deepEqual(validateMatrix([]), ['matrix is not an object'])
  const problems = validateMatrix({ ...cleanMatrix, schemaVersion: 2 })
  assert.equal(problems.includes('schemaVersion=2 want 1'), true)
})

test('validateMatrix 拒绝空 providerId 与空 cases', () => {
  const problems = validateMatrix({ ...cleanMatrix, providerId: '  ', cases: [] })
  assert.equal(problems.includes('providerId is empty'), true)
  assert.equal(problems.includes('cases is not a non-empty array'), true)
})

test('validateMatrix 拒绝重复用例名', () => {
  const duped = {
    ...cleanMatrix,
    cases: [cleanMatrix.cases[0], { ...cleanMatrix.cases[1], name: cleanMatrix.cases[0].name }]
  }
  const problems = validateMatrix(duped)
  assert.equal(problems.some((problem) => problem.includes('duplicate name')), true)
})

test('validateMatrix 拒绝候选用例带 gate 或没有候选', () => {
  const problems = validateMatrix({
    ...cleanMatrix,
    cases: [{ ...cleanMatrix.cases[0], gate: 'platform', candidates: [] }]
  })
  assert.equal(problems.includes('cases[0] candidate decision without candidates'), true)
  assert.equal(problems.includes('cases[0] candidate decision carries gate platform'), true)
})

test('validateMatrix 拒绝排除用例仍列候选或缺失 detail', () => {
  const problems = validateMatrix({
    ...cleanMatrix,
    cases: [{ ...cleanMatrix.cases[1], candidates: ['x'], detail: ' ' }]
  })
  assert.equal(problems.includes('cases[0] rejected decision still lists candidates'), true)
  assert.equal(problems.includes('cases[0] detail is empty'), true)
})

test('validateMatrix 拒绝未知 gate 与未知 decision', () => {
  const problems = validateMatrix({
    ...cleanMatrix,
    cases: [{ ...cleanMatrix.cases[1], gate: 'os-version', decision: 'maybe' }]
  })
  assert.equal(problems.some((problem) => problem.startsWith('cases[0] decision=')), true)
  assert.equal(problems.some((problem) => problem.startsWith('cases[0] gate=')), true)
})

test('validateMatrix 要求 profile 带 platform 与 host', () => {
  const problems = validateMatrix({
    ...cleanMatrix,
    cases: [{ ...cleanMatrix.cases[0], profile: { platform: '', host: '' } }]
  })
  assert.equal(problems.includes('cases[0] profile.platform is empty'), true)
  assert.equal(problems.includes('cases[0] profile.host is empty'), true)
})

test('summarizeMatrix 统计候选、排除与各闸命中', () => {
  const summary = summarizeMatrix(cleanMatrix)
  assert.deepEqual(summary, {
    cases: 2,
    candidates: 1,
    rejected: 1,
    gateCounts: { placement: 0, platform: 0, permission: 1, contractVersion: 0 }
  })
})

test('renderMatrix 输出用例总数与四道闸分布', () => {
  const rendered = renderMatrix(cleanMatrix)
  assert.equal(rendered.includes('用例 2 / 通过 1 / 排除 1'), true)
  assert.equal(rendered.includes('placement 0 / platform 0 / permission 1 / contractVersion 0'), true)
  assert.equal(rendered.includes('macos-device-partial-permissions'), true)
  assert.equal(rendered.includes('missing required permissions: tool.invoke'), true)
})

test('parseGoTestOutput 统计 PASS/FAIL 行', () => {
  const counts = parseGoTestOutput('--- PASS: TestA\n--- PASS: TestB\n--- FAIL: TestC\nFAIL\tpkg\t0.2s\n')
  assert.deepEqual(counts, { passed: 2, failed: 1 })
  assert.deepEqual(parseGoTestOutput(undefined), { passed: 0, failed: 0 })
})

test('仓库内真实矩阵通过校验且闸门命中符合 emote 声明', () => {
  const matrix = readMatrix(repoRoot)
  assert.deepEqual(validateMatrix(matrix), [])
  const summary = summarizeMatrix(matrix)
  assert.equal(summary.cases >= 10, true)
  assert.equal(summary.candidates >= 3, true)
  for (const gate of ['placement', 'permission', 'contractVersion']) {
    assert.equal(summary.gateCounts[gate] >= 1, true, `gate ${gate} has no case`)
  }
  assert.equal(summary.gateCounts.platform, 0)
})
