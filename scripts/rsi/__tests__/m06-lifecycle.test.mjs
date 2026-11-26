import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_LIFECYCLE,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  LIFECYCLE_CASES,
  LIFECYCLE_SOURCE,
  LIFECYCLE_TEST,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseLifecycleRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoForbidden,
  validateSource
} from '../m06-s01-lifecycle.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return { defaultTests: ['enablement_test.go'], taggedTests: ['enablement_test.go', LIFECYCLE_TEST].sort(), consumers: [] }
}

test('S01 退出码为该步独占', () => {
  assert.equal(EXIT_LIFECYCLE, 65)
})

test('S01 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestLifecycle')
  for (const taken of ['TestQuality', 'TestExactRoute', 'TestSchemaCache', 'TestPreauth']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of LIFECYCLE_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S01 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateSource(SOURCE_GUARDS.join('\n')), [])
})

test('S01 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 9) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S01 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 9 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S01 七个状态与关键迁移守卫缺一即报', () => {
  assert.equal(validateSource('package enablement').length, SOURCE_GUARDS.length)
  for (const guard of ['LifecycleInstalled LifecycleState = "installed"', '{EventStart, LifecycleRunning},', 'if epoch <= m.epoch {', 'if previous, ok := m.applied[epoch]; ok {']) {
    assert.ok(SOURCE_GUARDS.includes(guard))
  }
})

test('S01 断言守卫钉住跳步、倒退、幂等与乱序', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if !strings.Contains(err.Error(), "installed + start") {'))
  assert.ok(ASSERT_GUARDS.includes('if replay.Changed || replay.From != first.From || replay.To != first.To {'))
  assert.ok(ASSERT_GUARDS.includes('if changed.Load() != 1 {'))
})

test('S01 十个用例名必须逐个出现', () => {
  const body = LIFECYCLE_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package enablement')
  assert.equal(problems.length, LIFECYCLE_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('lifecycle test is missing TestLifecycle')))
})

test('S01 禁止环境变量与睡眠式同步', () => {
  const clean = 'time.Now()'
  assert.deepEqual(validateNoForbidden(clean), [])
  assert.deepEqual(validateNoForbidden(clean + 'os.Setenv'), ['lifecycle surface must not contain os.Setenv'])
  assert.deepEqual(validateNoForbidden(clean + 'time.Sleep('), ['lifecycle surface must not contain time.Sleep('])
  assert.deepEqual(validateNoForbidden(clean + 'openai'), ['lifecycle surface must not contain openai'])
})

test('S01 构建标签把状态机挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_lifecycle')
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
  const leaked = validateIsolation({ ...isolation(), defaultTests: ['enablement_test.go', LIFECYCLE_TEST].sort() })
  assert.ok(leaked.some((p) => p.includes('leaked')))
  const missing = validateIsolation({ ...isolation(), taggedTests: ['enablement_test.go'] })
  assert.ok(missing.some((p) => p.includes(`missing ${LIFECYCLE_TEST}`)))
})

test('S01 未接线证明：包外 NewMachine 调用必须为 0', () => {
  const problems = validateIsolation({ ...isolation(), consumers: ['backend/internal/x.go'] })
  assert.deepEqual(problems, ['lifecycle machine is wired before the integration step: backend/internal/x.go'])
})

test('S01 行解析只认 m06-s01 空格前缀', () => {
  const log = [
    '--- PASS: TestLifecycleHappyPath',
    '    lifecycle_state_test.go:1: m06-s01 happy a',
    '    lifecycle_state_test.go:2: m06-s01-detail noise',
    '    lifecycle_state_test.go:3: m06-s01 stale b'
  ].join('\n')
  assert.deepEqual(parseLifecycleRows(log), ['happy a', 'stale b'])
})

test('S01 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S01 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S01 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseLifecycleRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=lifecycle package=' + PACKAGE_PATH))
  assert.ok(text.includes('consumers=0'))
  assert.ok(text.includes('passed=10 failed=0 exit=0 race=true'))
})
