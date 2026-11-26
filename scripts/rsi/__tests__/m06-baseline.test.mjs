import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BASELINE_CASES,
  BASELINE_TEST,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_BASELINE,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  RUN_PATTERN,
  parseBaselineRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation
} from '../m06-s07-baseline.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultTests: ['enablement_test.go'],
    taggedTests: ['baseline_test.go', 'enablement_test.go', 'execute_test.go', 'hot_test.go', 'idle_test.go', 'install_metadata_test.go', 'leak_test.go', 'lifecycle_state_test.go'].sort()
  }
}

test('S07 退出码为该步独占', () => {
  assert.equal(EXIT_BASELINE, 71)
})

test('S07 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestPluginBaseline')
  for (const taken of ['TestPluginRelease', 'TestPluginHot', 'TestPluginIdle', 'TestPluginExecute']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of BASELINE_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S07 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
})

test('S07 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 9) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S07 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 8 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S07 断言守卫钉住冷态、线性与显式激活', () => {
  assert.equal(validateAssertSource('package enablement').length, ASSERT_GUARDS.length)
  assert.ok(ASSERT_GUARDS.includes('if sample.Goroutines > 2 {'))
  assert.ok(ASSERT_GUARDS.includes('if ratio > 2.5 {'))
  assert.ok(ASSERT_GUARDS.includes('if snapshot.Processes != 5 || spawner.Starts() != 5 {'))
})

test('S07 九个用例名必须逐个出现', () => {
  const body = BASELINE_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package enablement')
  assert.equal(problems.length, BASELINE_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('baseline test is missing TestPluginBaseline')))
})

test('S07 构建标签把基线测试挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_lifecycle')
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
  const leaked = validateIsolation({ ...isolation(), defaultTests: ['enablement_test.go', BASELINE_TEST].sort() })
  assert.ok(leaked.some((p) => p.includes(`leaked ${BASELINE_TEST}`)))
  const missing = validateIsolation({ ...isolation(), taggedTests: ['enablement_test.go'] })
  assert.ok(missing.some((p) => p.includes(`missing ${BASELINE_TEST}`)))
})

test('S07 tagged 集合不得混入生产文件', () => {
  const problems = validateIsolation({ ...isolation(), taggedTests: [...isolation().taggedTests, 'sneaky.go'].sort() })
  assert.ok(problems.some((p) => p.includes('leaked non-test file sneaky.go')))
})

test('S07 行解析只认 m06-s07 空格前缀', () => {
  const log = [
    '--- PASS: TestPluginBaselineThousand',
    '    baseline_test.go:1: m06-s07 thousand a',
    '    baseline_test.go:2: m06-s07-detail noise',
    '    baseline_test.go:3: m06-s07 zero b'
  ].join('\n')
  assert.deepEqual(parseBaselineRows(log), ['thousand a', 'zero b'])
})

test('S07 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S07 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S07 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseBaselineRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=baseline package=' + PACKAGE_PATH))
  assert.ok(text.includes(`delta=[${BASELINE_TEST}]`))
  assert.ok(text.includes('passed=9 failed=0 exit=0 race=true'))
})

test('S07 证据行包含 0/1000/2000 三档与真机信息', () => {
  assert.ok(EVIDENCE_ROWS.includes('zero plugins=0 processes=0 workers=0 spawn_calls=0'))
  assert.ok(EVIDENCE_ROWS.includes('thousand installed=1000 processes=0 workers=0 spawn_calls=0'))
  assert.ok(EVIDENCE_ROWS.includes('two_thousand installed=2000 processes=0 workers=0 spawn_calls=0'))
  assert.ok(EVIDENCE_ROWS.includes('machine reported=true'))
})
