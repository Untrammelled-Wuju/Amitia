import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXECUTE_CASES,
  EXECUTE_SOURCE,
  EXECUTE_TEST,
  EXIT_EXECUTE,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseExecuteRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoBackgroundWork,
  validateSource
} from '../m06-s03-execute.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  const shared = ['enablement_test.go']
  const tagged = ['enablement_test.go', 'install_metadata_test.go', 'lifecycle_state_test.go', EXECUTE_TEST].sort()
  return { defaultTests: shared, taggedTests: tagged }
}

test('S03 退出码为该步独占', () => {
  assert.equal(EXIT_EXECUTE, 67)
})

test('S03 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestPluginExecute')
  for (const taken of ['TestPluginMetadata', 'TestLifecycle', 'TestQuality', 'TestExactRoute']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of EXECUTE_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S03 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateSource(SOURCE_GUARDS.join('\n')), [])
})

test('S03 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 9) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S03 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 9 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S03 执行路径守卫缺一即报', () => {
  assert.equal(validateSource('package enablement').length, SOURCE_GUARDS.length)
  assert.ok(SOURCE_GUARDS.includes('if cached, ok := results[request.ID]; ok {'))
  assert.ok(SOURCE_GUARDS.includes('_, _ = machine.Apply(context.Background(), EventDeactivate, machine.Epoch()+1)'))
  assert.ok(SOURCE_GUARDS.includes('invoker, ok := handle.(RuntimeInvoker)'))
})

test('S03 断言守卫钉住超时链与回退', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if !errors.Is(err, context.DeadlineExceeded) {'))
  assert.ok(ASSERT_GUARDS.includes('if !runtime.Closed() {'))
  assert.ok(ASSERT_GUARDS.includes('if !errors.Is(err, ErrRequestInFlight) {'))
})

test('S03 十个用例名必须逐个出现', () => {
  const body = EXECUTE_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package enablement')
  assert.equal(problems.length, EXECUTE_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('execute test is missing TestPluginExecute')))
})

test('S03 执行路径不得包含后台协程与休眠', () => {
  const clean = 'func execute() {}'
  assert.deepEqual(validateNoBackgroundWork(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoBackgroundWork(clean + token), [`execute path must not contain ${token}`])
  }
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
})

test('S03 构建标签把执行路径挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_lifecycle')
  const leaked = validateIsolation({ ...isolation(), defaultTests: ['enablement_test.go', EXECUTE_TEST].sort() })
  assert.ok(leaked.some((p) => p.includes(`leaked ${EXECUTE_TEST}`)))
  const missing = validateIsolation({ ...isolation(), taggedTests: ['enablement_test.go'] })
  assert.ok(missing.some((p) => p.includes(`missing ${EXECUTE_TEST}`)))
  const dropped = validateIsolation({ ...isolation(), taggedTests: [EXECUTE_TEST] })
  assert.ok(dropped.some((p) => p.includes('dropped default test enablement_test.go')))
})

test('S03 tagged 集合不得混入生产文件', () => {
  const problems = validateIsolation({ ...isolation(), taggedTests: [...isolation().taggedTests, 'sneaky.go'].sort() })
  assert.ok(problems.some((p) => p.includes('leaked non-test file sneaky.go')))
})

test('S03 行解析只认 m06-s03 空格前缀', () => {
  const log = [
    '--- PASS: TestPluginExecuteColdStartSpawnsOnce',
    '    execute_test.go:1: m06-s03 cold a',
    '    execute_test.go:2: m06-s03-detail noise',
    '    execute_test.go:3: m06-s03 timeout b'
  ].join('\n')
  assert.deepEqual(parseExecuteRows(log), ['cold a', 'timeout b'])
})

test('S03 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S03 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S03 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseExecuteRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=execute package=' + PACKAGE_PATH))
  assert.ok(text.includes('delta=[execute_test.go]'))
  assert.ok(text.includes('passed=10 failed=0 exit=0 race=true'))
})

test('S03 源码文件名固定', () => {
  assert.equal(EXECUTE_SOURCE, 'execute.go')
  assert.equal(EXECUTE_TEST, 'execute_test.go')
})
