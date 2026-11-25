import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  EVIDENCE_ROWS,
  EXIT_RECOVERY,
  EXPECTED_PASSED,
  ASSERT_GUARDS,
  RECOVERY_GUARDS,
  RUN_PATTERN,
  parseRecoveryRows,
  renderEvidence,
  validateEvidence,
  validateInspectReadOnly,
  validateRecoverySource
} from '../m04-s12-recovery.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function goFiles() {
  return { goFiles: ['ledger.go', 'recovery.go'], testFiles: ['recovery_test.go'] }
}

test('S12 退出码为该步独占', () => {
  assert.equal(EXIT_RECOVERY, 52)
})

test('S12 用例前缀不与已占用前缀撞车', () => {
  for (const taken of ['TestLedger', 'TestEffect', 'TestJournalIdentity', 'TestJournalToken', 'TestJournalProof']) {
    assert.equal(taken.startsWith(RUN_PATTERN), false)
    assert.equal(RUN_PATTERN.startsWith(taken), false)
  }
  assert.equal(RUN_PATTERN, 'TestJournalRecovery')
})

test('S12 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence({ ...goFiles(), ...rowSet() }), [])
})

test('S12 证据行缺失或重复都会被拦下', () => {
  const problems = validateEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS.slice(1) })
  assert.ok(problems.some((p) => p.includes('evidence rows')))
  const duplicated = validateEvidence({ ...goFiles(), ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] })
  assert.ok(duplicated.some((p) => p.includes('exactly once')))
})

test('S12 未带 race 或退出非零时拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), passed: 7 }).some((p) => p.includes('passed=7')))
})

test('S12 源文件缺项会被点名', () => {
  assert.deepEqual(validateRecoverySource(RECOVERY_GUARDS.join('\n')), [])
  const problems = validateRecoverySource('package executionjournal')
  assert.ok(problems.length >= RECOVERY_GUARDS.length)
  assert.ok(problems.every((p) => p.startsWith('recovery source is missing ')))
})

test('S12 恢复检索必须保持只读', () => {
  const clean = 'func (l *InvocationLedger) InspectRecovery(ctx context.Context, scope LedgerScope, idempotencyKey string) (RecoveryRecord, bool, error) {\n\treturn l.recovery(ctx, "", key)\n}\n'
  assert.deepEqual(validateInspectReadOnly(clean), [])
  const dirty = clean + 'func (l *InvocationLedger) Other() { db.Exec("DELETE FROM kernel_invocation_recovery") }\n'
  assert.deepEqual(validateInspectReadOnly(dirty), [])
  const inBody = 'func (l *InvocationLedger) InspectRecovery(ctx context.Context) error {\n\tdb.Exec("UPDATE kernel_invocation_recovery SET decision=?","reconcile")\n\treturn nil\n}\n'
  assert.equal(validateInspectReadOnly(inBody).length, 1)
})

test('S12 断言守卫覆盖重启不重复副作用', () => {
  assert.ok(ASSERT_GUARDS.includes('if executions.Load() != 0 {'))
  assert.ok(ASSERT_GUARDS.includes('if !errors.Is(err, executionjournal.ErrRecoveryRequired) {'))
})

test('S12 行解析只认 m04-s12 前缀并按出现顺序返回', () => {
  const log = `--- PASS: X\n    recovery_test.go:1: m04-s12 a\nnoise m04-s11 b\n    recovery_test.go:2: m04-s12 c\n`
  assert.deepEqual(parseRecoveryRows(log), ['a', 'c'])
})

test('S12 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS })
  assert.deepEqual(parseRecoveryRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('passed=8 failed=0 exit=0 race=true'))
})
