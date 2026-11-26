import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_HOST,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  GATES,
  HOST_CASES,
  HOST_TEST,
  LIFECYCLE_FILES,
  PACKAGE_FILE,
  PACKAGE_PATH,
  RUN_PATTERN,
  parseGoListFiles,
  parseGoTestOutput,
  parseHostRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateOffState
} from '../m06-s08-host.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function offState() {
  return {
    defaultFiles: ['support.go'],
    taggedFiles: ['support.go', ...LIFECYCLE_FILES].sort(),
    defaultTests: ['supervisor_test.go'],
    taggedTests: ['host_isolation_test.go', 'supervisor_test.go'].sort()
  }
}

test('S08 退出码为该步独占', () => {
  assert.equal(EXIT_HOST, 72)
})

test('S08 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestHostIsolation')
  for (const taken of ['TestPluginBaseline', 'TestPluginRelease', 'TestPluginHot', 'TestPluginIdle']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of HOST_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S08 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateOffState(offState()), [])
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
})

test('S08 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S08 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 7 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S08 关闭态：默认构建不得含生命周期文件', () => {
  const leaked = validateOffState({ ...offState(), defaultFiles: ['install.go', ...LIFECYCLE_FILES].sort() })
  assert.ok(leaked.some((p) => p.includes('default build leaked lifecycle.go')))
  const missing = validateOffState({ ...offState(), taggedFiles: ['support.go'] })
  assert.ok(missing.some((p) => p.includes('missing lifecycle.go')))
})

test('S08 关闭态：宿主机测试只在 tags 下出现', () => {
  const missing = validateOffState({ ...offState(), taggedTests: ['supervisor_test.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged supervisor tests missing ${HOST_TEST}`)))
  const leaked = validateOffState({ ...offState(), defaultTests: ['supervisor_test.go', HOST_TEST].sort() })
  assert.ok(leaked.some((p) => p.includes(`default supervisor tests leaked ${HOST_TEST}`)))
})

test('S08 断言守卫钉住强杀隔离与旧流程', () => {
  assert.equal(validateAssertSource('package runtime_supervisor').length, ASSERT_GUARDS.length)
  assert.ok(ASSERT_GUARDS.includes('if after.Actual != ActualStopped {'))
  assert.ok(ASSERT_GUARDS.includes('if instance.Actual != ActualReady || instance.Health != HealthHealthy {'))
  assert.ok(ASSERT_GUARDS.includes('if result.Error != nil || result.Status != "success" {'))
})

test('S08 八个用例名必须逐个出现', () => {
  const body = HOST_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package runtime_supervisor')
  assert.equal(problems.length, HOST_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('host test is missing TestHostIsolation')))
})

test('S08 接入包必须覆盖 8 个门禁与 8 步', () => {
  assert.equal(GATES.length, 8)
  assert.ok(GATES.includes('HOST_ISOLATION'))
  assert.ok(GATES.includes('COLD_BASELINE'))
  assert.equal(LIFECYCLE_FILES.length, 6)
  assert.equal(PACKAGE_FILE, 'artifacts/rsi/m06-integration-package.json')
})

test('S08 行解析只认 m06-s08 空格前缀', () => {
  const log = [
    '--- PASS: TestHostIsolationThreeRuntimeTypes',
    '    host_isolation_test.go:1: m06-s08 isolation a',
    '    host_isolation_test.go:2: m06-s08-detail noise',
    '    host_isolation_test.go:3: m06-s08 kill b'
  ].join('\n')
  assert.deepEqual(parseHostRows(log), ['isolation a', 'kill b'])
})

test('S08 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S08 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S08 渲染结果包含关闭态与开关', () => {
  const text = renderEvidence({ ...rowSet(), defaultLifecycle: 0, taggedLifecycle: 6 })
  assert.ok(text.includes(`offstate: default_lifecycle_files=0 tagged_lifecycle_files=6 switch=${BUILD_TAG}`))
  assert.ok(text.includes('invocation=host package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=8 failed=0 exit=0 race=true'))
  assert.deepEqual(parseHostRows(text), EVIDENCE_ROWS)
})
