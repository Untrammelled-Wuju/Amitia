import assert from 'node:assert/strict'
import { appendFileSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import test from 'node:test'
import { gateVerdict, receiptFor, tamperDetected } from '../ci-gate.mjs'

function logFile(contents) {
  const root = mkdtempSync(resolve(tmpdir(), 'rsi-ci-'))
  const path = resolve(root, 'run.log')
  writeFileSync(path, contents, 'utf8')
  return path
}

function check(overrides = {}) {
  return { id: 'CI-RSI-UNIT', command: 'node --test', logPath: logFile('ok'), ok: true, ...overrides }
}

test('回执按检查结果标记状态并写入日志哈希', () => {
  const passed = receiptFor(check(), { revision: 'abc123', finishedAt: '2026-11-22T00:00:00Z' })
  assert.equal(passed.status, 'PASSED')
  assert.equal(passed.exitCode, 0)
  assert.match(passed.testLogSha256, /^[0-9a-f]{64}$/)

  const failed = receiptFor(check({ ok: false }), { revision: 'abc123', finishedAt: '2026-11-22T00:00:00Z' })
  assert.equal(failed.status, 'FAILED')
  assert.equal(failed.exitCode, 1)
})

test('日志被追加内容后必须判定为篡改', () => {
  const path = logFile('suite ok\n')
  const receipt = receiptFor(
    { id: 'CI-RSI-UNIT', command: 'node --test', logPath: path, ok: true },
    { revision: 'abc123', finishedAt: '2026-11-22T00:00:00Z' }
  )
  assert.equal(tamperDetected(receipt, path), false)
  appendFileSync(path, 'appended\n', 'utf8')
  assert.equal(tamperDetected(receipt, path), true)
})

test('任一步非 PASSED 即整体拒绝放行', () => {
  const receipts = [
    { id: 'A', status: 'PASSED' },
    { id: 'B', status: 'FAILED' }
  ]
  const verdict = gateVerdict(receipts)
  assert.equal(verdict.ok, false)
  assert.deepEqual(verdict.failed, ['B'])
  assert.equal(gateVerdict([{ id: 'A', status: 'PASSED' }]).ok, true)
})
