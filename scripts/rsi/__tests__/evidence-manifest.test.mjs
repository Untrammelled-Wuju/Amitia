import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import test from 'node:test'
import { validateEvidence, validatePromotion } from '../evidence-manifest.mjs'

function logFile(contents) {
  const dir = mkdtempSync(resolve(tmpdir(), 'rsi-evidence-'))
  const absolute = resolve(dir, 'step.log')
  writeFileSync(absolute, contents, 'utf8')
  return { absolute, sha256: createHash('sha256').update(contents).digest('hex') }
}

function step(overrides = {}) {
  const log = logFile('ok  package  0.5s\nEXIT=0\n')
  return {
    id: 'M01-S01',
    status: 'PASSED',
    testCommand: 'node scripts/rsi/baseline.mjs',
    exitCode: 0,
    testLogPath: log.absolute,
    testLogSha256: log.sha256,
    isolationProof: 'artifacts/rsi/worktrees/M01，无 AmitiaData 用户数据',
    reviewer: 'self-audit',
    finishedAt: '2026-11-22T11:40:00Z',
    ...overrides
  }
}

const taskIds = new Set(['M01-S01', 'M01-S02'])

test('合法证据通过', () => {
  const result = validateEvidence({ steps: [step(), step({ id: 'M01-S02' })] }, { taskIds })
  assert.equal(result.ok, true, JSON.stringify(result.violations))
})

test('缺少测试日志收据不能算 PASSED', () => {
  const result = validateEvidence({ steps: [step({ testLogPath: null })] }, { taskIds })
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'LOG_PATH_MISSING'))
})

test('篡改测试日志会被哈希比对发现', () => {
  const target = step()
  writeFileSync(target.testLogPath, 'ok  package  0.5s\nEXIT=0\n伪造追加\n', 'utf8')
  const result = validateEvidence({ steps: [target] }, { taskIds })
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'LOG_TAMPERED'))
})

test('非零退出码不能标记 PASSED', () => {
  const result = validateEvidence({ steps: [step({ exitCode: 1 })] }, { taskIds })
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'NONZERO_EXIT'))
})

test('缺少隔离证明不能标记 PASSED', () => {
  const result = validateEvidence({ steps: [step({ isolationProof: ' ' })] }, { taskIds })
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'NO_ISOLATION_PROOF'))
})

test('冻结集合外的步骤 ID 被拒绝', () => {
  const result = validateEvidence({ steps: [step({ id: 'M99-S99' })] }, { taskIds })
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'UNKNOWN_STEP'))
})

test('重复步骤 ID 被拒绝', () => {
  const result = validateEvidence({ steps: [step(), step()] }, { taskIds })
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'DUPLICATE_STEP'))
})

test('BLOCKED 步骤只要求 id 与 status', () => {
  const blocked = { id: 'M01-S02', status: 'BLOCKED', testCommand: null, exitCode: null, testLogPath: null, testLogSha256: null, isolationProof: '等待授权', reviewer: null, finishedAt: null }
  const result = validateEvidence({ steps: [step(), blocked] }, { taskIds })
  assert.equal(result.ok, true, JSON.stringify(result.violations))
  assert.equal(result.passed, 1)
})

test('发布前置未通过时拒绝晋升', () => {
  const manifest = { steps: [step(), step({ id: 'M01-S02', status: 'IN_PROGRESS', testLogPath: null, testLogSha256: null, testCommand: null, exitCode: null, isolationProof: null, reviewer: null, finishedAt: null })] }
  const result = validatePromotion(manifest, ['M01-S01', 'M01-S02'], { taskIds })
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'PREREQUISITE_NOT_PASSED'))
  assert.ok(validatePromotion({ steps: [step(), step({ id: 'M01-S02' })] }, ['M01-S01'], { taskIds }).ok)
})
