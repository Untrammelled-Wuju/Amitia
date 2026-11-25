import assert from 'node:assert/strict'
import { appendFileSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import test from 'node:test'
import { buildPermit, evaluateGroup, findGroup, loadGroups } from '../release-gate.mjs'
import { sha256File } from '../evidence-manifest.mjs'

function logFile(contents = 'ok\n') {
  const root = mkdtempSync(resolve(tmpdir(), 'rsi-gate-'))
  const path = resolve(root, 'run.log')
  writeFileSync(path, contents, 'utf8')
  return path
}

function receipt(id, logPath, overrides = {}) {
  return {
    id,
    status: 'PASSED',
    sourceRevision: 'abc123',
    changedFiles: ['scripts/rsi/release-gate.mjs'],
    testCommand: 'node --test scripts/rsi/__tests__/',
    exitCode: 0,
    testLogPath: logPath,
    testLogSha256: sha256File(logPath),
    isolationProof: '隔离临时目录，无生产数据',
    reviewer: 'qa',
    finishedAt: '2026-11-22T00:00:00Z',
    ...overrides
  }
}

const groups = { digest: 'groups-digest', groups: [{ id: 'RG-TEST', name: '测试组', steps: ['A', 'B'] }] }
const group = findGroup(groups, 'RG-TEST')

test('全部前置 PASSED 时产生发布许可且摘要稳定', () => {
  const manifest = { steps: [receipt('A', logFile()), receipt('B', logFile())] }
  const first = evaluateGroup(manifest, group, groups)
  const second = evaluateGroup(manifest, group, groups)
  assert.equal(first.ok, true, JSON.stringify(first.violations))
  assert.equal(first.gateDigest, second.gateDigest)
  const permit = buildPermit(first, JSON.stringify(manifest), groups)
  assert.equal(permit.verdict, 'APPROVED')
  assert.equal(permit.steps.length, 2)
})

test('缺少前置工作卡时拒绝发布许可', () => {
  const manifest = { steps: [receipt('A', logFile())] }
  const result = evaluateGroup(manifest, group, groups)
  assert.equal(result.ok, false)
  assert.deepEqual(result.missing, ['B'])
})

test('前置工作卡未通过时拒绝发布许可', () => {
  const manifest = { steps: [receipt('A', logFile()), receipt('B', logFile(), { status: 'IN_PROGRESS' })] }
  const result = evaluateGroup(manifest, group, groups)
  assert.equal(result.ok, false)
  assert.deepEqual(result.failed, ['B'])
})

test('前置工作卡的测试日志被篡改时拒绝发布许可', () => {
  const path = logFile()
  const manifest = { steps: [receipt('A', path), receipt('B', logFile())] }
  appendFileSync(path, 'tampered\n', 'utf8')
  const result = evaluateGroup(manifest, group, groups)
  assert.equal(result.ok, false)
  assert.ok(result.violations.some((item) => item.code === 'LOG_TAMPERED' && item.message.includes('A')))
})

test('默认发布组定义覆盖 M01 十二步', () => {
  const loaded = loadGroups()
  const m01 = findGroup(loaded, 'RG-M01')
  assert.equal(m01.steps.length, 12)
  assert.equal(m01.steps[0], 'M01-S01')
  assert.equal(m01.steps[11], 'M01-S12')
})
