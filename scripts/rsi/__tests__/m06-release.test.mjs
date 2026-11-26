import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_RELEASE,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  RELEASE_CASES,
  RELEASE_SOURCE,
  RELEASE_TEST,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseReleaseRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoLeakyPrimitives,
  validateSource
} from '../m06-s06-release.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultTests: ['enablement_test.go'],
    taggedTests: ['enablement_test.go', 'execute_test.go', 'hot_test.go', 'idle_test.go', 'install_metadata_test.go', 'leak_test.go', 'lifecycle_state_test.go'].sort()
  }
}

test('S06 退出码为该步独占', () => {
  assert.equal(EXIT_RELEASE, 70)
})

test('S06 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestPluginRelease')
  for (const taken of ['TestPluginHot', 'TestPluginIdle', 'TestPluginExecute', 'TestPluginMetadata']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of RELEASE_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S06 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateSource(SOURCE_GUARDS.join('\n')), [])
})

test('S06 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 9) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S06 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 9 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S06 释放路径守卫缺一即报', () => {
  assert.equal(validateSource('package enablement').length, SOURCE_GUARDS.length)
  assert.ok(SOURCE_GUARDS.includes('if active := i.activeCalls[pluginID]; active > 0 {'))
  assert.ok(SOURCE_GUARDS.includes('if len(m.history) > 128 {'))
  assert.ok(SOURCE_GUARDS.includes('if appliedEpoch+128 < m.epoch {'))
})

test('S06 断言守卫钉住 1000 次循环与忙碌拒绝', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if spawner.Starts() != 1000 || spawner.Stops() != 1000 {'))
  assert.ok(ASSERT_GUARDS.includes('if after > baseline+2 {'))
  assert.ok(ASSERT_GUARDS.includes('if len(machine.History()) > 128 {'))
})

test('S06 十个用例名必须逐个出现', () => {
  const body = RELEASE_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package enablement')
  assert.equal(problems.length, RELEASE_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('release test is missing TestPluginRelease')))
})

test('S06 释放路径不得包含协程、recover 与休眠', () => {
  const clean = 'func release() {}'
  assert.deepEqual(validateNoLeakyPrimitives(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoLeakyPrimitives(clean + token), [`release path must not contain ${token}`])
  }
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
})

test('S06 构建标签把泄漏测试挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_lifecycle')
  const leaked = validateIsolation({ ...isolation(), defaultTests: ['enablement_test.go', RELEASE_TEST].sort() })
  assert.ok(leaked.some((p) => p.includes(`leaked ${RELEASE_TEST}`)))
  const missing = validateIsolation({ ...isolation(), taggedTests: ['enablement_test.go'] })
  assert.ok(missing.some((p) => p.includes(`missing ${RELEASE_TEST}`)))
  const dropped = validateIsolation({ ...isolation(), taggedTests: [RELEASE_TEST] })
  assert.ok(dropped.some((p) => p.includes('dropped default test enablement_test.go')))
})

test('S06 tagged 集合不得混入生产文件', () => {
  const problems = validateIsolation({ ...isolation(), taggedTests: [...isolation().taggedTests, 'sneaky.go'].sort() })
  assert.ok(problems.some((p) => p.includes('leaked non-test file sneaky.go')))
})

test('S06 行解析只认 m06-s06 空格前缀', () => {
  const log = [
    '--- PASS: TestPluginReleaseDeactivateClosesOnce',
    '    leak_test.go:1: m06-s06 deactivate a',
    '    leak_test.go:2: m06-s06-detail noise',
    '    leak_test.go:3: m06-s06 busy b'
  ].join('\n')
  assert.deepEqual(parseReleaseRows(log), ['deactivate a', 'busy b'])
})

test('S06 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S06 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S06 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseReleaseRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=release package=' + PACKAGE_PATH))
  assert.ok(text.includes(`delta=[${RELEASE_TEST}]`))
  assert.ok(text.includes('passed=10 failed=0 exit=0 race=true'))
})

test('S06 源码文件名固定', () => {
  assert.equal(RELEASE_SOURCE, 'release.go')
  assert.equal(RELEASE_TEST, 'leak_test.go')
})
