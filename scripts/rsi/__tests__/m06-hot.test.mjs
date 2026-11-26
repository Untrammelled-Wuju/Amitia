import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_HOT,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  HOT_CASES,
  HOT_SOURCE,
  HOT_TEST,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseHotRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoBypass,
  validateSource
} from '../m06-s05-hot.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultTests: ['enablement_test.go'],
    taggedTests: ['enablement_test.go', 'execute_test.go', 'hot_test.go', 'idle_test.go', 'install_metadata_test.go', 'lifecycle_state_test.go'].sort()
  }
}

test('S05 退出码为该步独占', () => {
  assert.equal(EXIT_HOT, 69)
})

test('S05 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestPluginHot')
  for (const taken of ['TestPluginIdle', 'TestPluginExecute', 'TestPluginMetadata', 'TestLifecycle']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of HOT_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S05 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateSource(SOURCE_GUARDS.join('\n')), [])
})

test('S05 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 11) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S05 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 11 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S05 三重核验守卫缺一即报', () => {
  assert.equal(validateSource('package enablement').length, SOURCE_GUARDS.length)
  assert.ok(SOURCE_GUARDS.includes('if !h.policy.AllowedCategories[request.Category] {'))
  assert.ok(SOURCE_GUARDS.includes('if !request.Granted {'))
  assert.ok(SOURCE_GUARDS.includes('if request.Profile.Source != ProfileSourceHost {'))
  assert.ok(SOURCE_GUARDS.includes('if h.policy.MaxHot > 0 && len(h.hot) >= h.policy.MaxHot {'))
})

test('S05 断言守卫钉住拒绝理由与可查性', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if !ok || reason != "authorization_missing" {'))
  assert.ok(ASSERT_GUARDS.includes('if !strings.Contains(err.Error(), "category_not_allowed") {'))
  assert.ok(ASSERT_GUARDS.includes('if wins.Load() != 1 || fixture.residency.Len() != 1 {'))
})

test('S05 十二个用例名必须逐个出现', () => {
  const body = HOT_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package enablement')
  assert.equal(problems.length, HOT_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('hot test is missing TestPluginHot')))
})

test('S05 常驻路径不得包含协程或环境开关', () => {
  const clean = 'func acquire() {}'
  assert.deepEqual(validateNoBypass(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoBypass(clean + token), [`hot path must not contain ${token}`])
  }
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
})

test('S05 构建标签把常驻策略挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_lifecycle')
  const leaked = validateIsolation({ ...isolation(), defaultTests: ['enablement_test.go', HOT_TEST].sort() })
  assert.ok(leaked.some((p) => p.includes(`leaked ${HOT_TEST}`)))
  const missing = validateIsolation({ ...isolation(), taggedTests: ['enablement_test.go'] })
  assert.ok(missing.some((p) => p.includes(`missing ${HOT_TEST}`)))
  const dropped = validateIsolation({ ...isolation(), taggedTests: [HOT_TEST] })
  assert.ok(dropped.some((p) => p.includes('dropped default test enablement_test.go')))
})

test('S05 tagged 集合不得混入生产文件', () => {
  const problems = validateIsolation({ ...isolation(), taggedTests: [...isolation().taggedTests, 'sneaky.go'].sort() })
  assert.ok(problems.some((p) => p.includes('leaked non-test file sneaky.go')))
})

test('S05 行解析只认 m06-s05 空格前缀', () => {
  const log = [
    '--- PASS: TestPluginHotAuthorizedCategoryResides',
    '    hot_test.go:1: m06-s05 resident a',
    '    hot_test.go:2: m06-s05-detail noise',
    '    hot_test.go:3: m06-s05 denied b'
  ].join('\n')
  assert.deepEqual(parseHotRows(log), ['resident a', 'denied b'])
})

test('S05 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S05 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S05 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseHotRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=hot package=' + PACKAGE_PATH))
  assert.ok(text.includes(`delta=[${HOT_TEST}]`))
  assert.ok(text.includes('passed=12 failed=0 exit=0 race=true'))
})

test('S05 源码文件名固定', () => {
  assert.equal(HOT_SOURCE, 'hot.go')
  assert.equal(HOT_TEST, 'hot_test.go')
})
