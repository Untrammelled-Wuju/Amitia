import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_IDLE,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  IDLE_CASES,
  IDLE_SOURCE,
  IDLE_TEST,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseIdleRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoTimers,
  validateSource
} from '../m06-s04-idle.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultTests: ['enablement_test.go'],
    taggedTests: ['enablement_test.go', 'execute_test.go', 'idle_test.go', 'install_metadata_test.go', 'lifecycle_state_test.go'].sort()
  }
}

test('S04 退出码为该步独占', () => {
  assert.equal(EXIT_IDLE, 68)
})

test('S04 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestPluginIdle')
  for (const taken of ['TestPluginExecute', 'TestPluginMetadata', 'TestLifecycle', 'TestQuality']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of IDLE_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S04 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
})

test('S04 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 9) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S04 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 9 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S04 空闲回收守卫缺一即报', () => {
  assert.equal(validateSource('package enablement').length, SOURCE_GUARDS.length)
  assert.ok(SOURCE_GUARDS.includes('if now.Sub(lastUsed) < r.policy.TTL {'))
  assert.ok(SOURCE_GUARDS.includes('if err := r.drainer.Drain(ctx, instanceID, r.policy.DrainTimeout); err != nil {'))
  assert.ok(SOURCE_GUARDS.includes('func (i *Installer) WithActivity(activity func(string)) *Installer {'))
})

test('S04 断言守卫钉住 TTL 边界与释放后重启', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if fixture.runtime.StopCount() != 1 {'))
  assert.ok(ASSERT_GUARDS.includes('if fixture.spawner.Calls() != 2 {'))
  assert.ok(ASSERT_GUARDS.includes('if len(report.Released) != 0 || fixture.installer.Snapshot().Processes != 1 {'))
})

test('S04 十个用例名必须逐个出现', () => {
  const body = IDLE_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package enablement')
  assert.equal(problems.length, IDLE_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('idle test is missing TestPluginIdle')))
})

test('S04 回收器不得自建未清理的计时器或协程', () => {
  const clean = 'func sweep() {}'
  assert.deepEqual(validateNoTimers(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoTimers(clean + token), [`idle reaper must not contain ${token}`])
  }
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
})

test('S04 构建标签把回收器挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_lifecycle')
  const leaked = validateIsolation({ ...isolation(), defaultTests: ['enablement_test.go', IDLE_TEST].sort() })
  assert.ok(leaked.some((p) => p.includes(`leaked ${IDLE_TEST}`)))
  const missing = validateIsolation({ ...isolation(), taggedTests: ['enablement_test.go'] })
  assert.ok(missing.some((p) => p.includes(`missing ${IDLE_TEST}`)))
  const dropped = validateIsolation({ ...isolation(), taggedTests: [IDLE_TEST] })
  assert.ok(dropped.some((p) => p.includes('dropped default test enablement_test.go')))
})

test('S04 tagged 集合不得混入生产文件', () => {
  const problems = validateIsolation({ ...isolation(), taggedTests: [...isolation().taggedTests, 'sneaky.go'].sort() })
  assert.ok(problems.some((p) => p.includes('leaked non-test file sneaky.go')))
})

test('S04 行解析只认 m06-s04 空格前缀', () => {
  const log = [
    '--- PASS: TestPluginIdleReleasesAfterTTL',
    '    idle_test.go:1: m06-s04 release a',
    '    idle_test.go:2: m06-s04-detail noise',
    '    idle_test.go:3: m06-s04 cancel b'
  ].join('\n')
  assert.deepEqual(parseIdleRows(log), ['release a', 'cancel b'])
})

test('S04 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S04 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S04 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseIdleRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=idle package=' + PACKAGE_PATH))
  assert.ok(text.includes(`delta=[${IDLE_TEST}]`))
  assert.ok(text.includes('passed=10 failed=0 exit=0 race=true'))
})

test('S04 源码文件名固定', () => {
  assert.equal(IDLE_SOURCE, 'idle.go')
  assert.equal(IDLE_TEST, 'idle_test.go')
})
