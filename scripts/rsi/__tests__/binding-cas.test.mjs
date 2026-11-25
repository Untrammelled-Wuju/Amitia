import assert from 'node:assert/strict'
import { existsSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import test from 'node:test'
import {
  BindingError,
  casSwitch,
  markUnknown,
  readLedger,
  readState,
  rollback,
  writePlan
} from '../binding-cas.mjs'

function workspace() {
  return mkdtempSync(resolve(tmpdir(), 'rsi-cas-'))
}

function seed(root, revision = 1) {
  const statePath = resolve(root, 'state.json')
  writeFileSync(statePath, JSON.stringify({ revision, binding: { providerId: 'stable', version: '1.0.0' } }), 'utf8')
  return statePath
}

test('不可变发布计划同内容幂等、不同内容冲突', () => {
  const root = workspace()
  const plan = { planId: 'p1', module: 'M01', binding: { providerId: 'next' } }
  const first = writePlan(root, plan)
  assert.equal(first.created, true)
  const again = writePlan(root, plan)
  assert.equal(again.created, false)
  assert.equal(again.digest, first.digest)
  assert.throws(
    () => writePlan(root, { ...plan, binding: { providerId: 'other' } }),
    (error) => error instanceof BindingError && error.code === 'IMMUTABLE_CONFLICT'
  )
})

test('CAS 版本冲突时失败且不改写状态', () => {
  const root = workspace()
  const statePath = seed(root, 2)
  const before = readState(statePath)
  try {
    casSwitch({ statePath, expectedRevision: 1, nextBinding: { providerId: 'next' } })
    assert.fail('应当抛出 CAS_CONFLICT')
  } catch (error) {
    assert.equal(error.code, 'CAS_CONFLICT')
    assert.equal(error.details.actual, 2)
    assert.equal(error.details.expected, 1)
  }
  assert.deepEqual(readState(statePath), before)
})

test('CAS 切换成功写入新修订并记录账本', () => {
  const root = workspace()
  const statePath = seed(root, 1)
  const ledgerPath = resolve(root, 'ledger.jsonl')
  const receipt = casSwitch({ statePath, expectedRevision: 1, nextBinding: { providerId: 'next' }, ledgerPath })
  assert.equal(receipt.revision, 2)
  assert.equal(readState(statePath).binding.providerId, 'next')
  const ledger = readLedger(ledgerPath)
  assert.equal(ledger.length, 1)
  assert.equal(ledger[0].event, 'switch')
  assert.equal(ledger[0].fromRevision, 1)
})

test('回滚恢复前一绑定，过期回执拒绝盲目回退', () => {
  const root = workspace()
  const statePath = seed(root, 1)
  const ledgerPath = resolve(root, 'ledger.jsonl')
  const receipt = casSwitch({ statePath, expectedRevision: 1, nextBinding: { providerId: 'next' }, ledgerPath })
  const restored = rollback({ statePath, receipt, ledgerPath })
  assert.equal(restored.revision, 3)
  assert.equal(restored.binding.providerId, 'stable')

  try {
    rollback({ statePath, receipt, ledgerPath })
    assert.fail('应当抛出 ROLLBACK_STALE')
  } catch (error) {
    assert.equal(error.code, 'ROLLBACK_STALE')
    assert.equal(error.details.receiptRevision, 2)
    assert.equal(error.details.actual, 3)
  }
  assert.equal(readLedger(ledgerPath).filter((item) => item.event === 'rollback').length, 1)
})

test('未知外部副作用只记录不重放', () => {
  const root = workspace()
  const ledgerPath = resolve(root, 'ledger.jsonl')
  const error = new BindingError('PROVIDER_ACTIVATION_FAILED', '激活失败', { cause: new Error('exit 1') })
  const record = markUnknown({ ledgerPath, receipt: { revision: 2 }, error })
  assert.equal(record.replay, false)
  assert.equal(record.code, 'PROVIDER_ACTIVATION_FAILED')
  assert.match(record.cause, /exit 1/)
  assert.equal(readLedger(ledgerPath).length, 1)
  assert.equal(existsSync(resolve(root, 'state.json')), false)
})
