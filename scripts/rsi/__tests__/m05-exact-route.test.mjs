import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  EVIDENCE_ROWS,
  EXACT_CASES,
  EXIT_EXACT_ROUTE,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  ROUTE_SOURCE,
  ROUTE_TEST,
  RUN_PATTERN,
  SELECTOR_GUARDS,
  SOURCE_GUARDS,
  parseExactRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoCatalogScan,
  validateRouteSource,
  validateSelectorSource
} from '../m05-s11-exact-route.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return { files: ['exact_route.go', 'task_selector.go'], tests: ['exact_route_test.go'], consumers: [] }
}

function selectorSource() {
  return SELECTOR_GUARDS[0] + '\n' + SELECTOR_GUARDS[1] + '\n' + SELECTOR_GUARDS[2] + '\nlowered := strings.ToLower(query)\n'
}

test('S11 退出码为该步独占', () => {
  assert.equal(EXIT_EXACT_ROUTE, 63)
})

test('S11 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestExactRoute')
  for (const taken of ['TestSchemaCache', 'TestPreauth', 'TestScale', 'TestCutover']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of EXACT_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S11 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateSelectorSource(selectorSource()), [])
})

test('S11 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S11 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 8 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S11 精确索引源码守卫缺一即报', () => {
  assert.deepEqual(validateRouteSource(SOURCE_GUARDS.join('\n')), [])
  assert.equal(validateRouteSource('package capability').length, SOURCE_GUARDS.length)
  assert.ok(SOURCE_GUARDS.includes('tool, ok := s.registry.Get(ctx, id)'))
  assert.ok(SOURCE_GUARDS.includes('return strings.Join(strings.Fields(trimmed), " ")'))
})

test('S11 精确分支必须早于词法扫描', () => {
  const wrong = 'lowered := strings.ToLower(query)\n' + selectorSource()
  assert.ok(validateSelectorSource(wrong).some((p) => p.includes('before the lexical scan')))
  assert.ok(validateSelectorSource('package capability').length >= SELECTOR_GUARDS.length)
})

test('S11 精确路径不得枚举目录或调用排序', () => {
  const clean = 'tool, ok := s.registry.Get(ctx, id)'
  assert.deepEqual(validateNoCatalogScan(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoCatalogScan(clean + token), [`exact route index must not contain ${token}`])
  }
  assert.ok(GO_TEST_FLAGS.includes('-race'))
  assert.equal(GO_TEST_FLAGS.includes('-tags'), false)
})

test('S11 断言守卫钉住零排序器调用与回退', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if recorder.calls != 0 {'))
  assert.ok(ASSERT_GUARDS.includes('if recorder.calls != 3 {'))
  assert.ok(ASSERT_GUARDS.includes('if index.Hits() != 0 || index.Misses() != 3 {'))
})

test('S11 九个用例名必须逐个出现', () => {
  const body = EXACT_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package capability_test')
  assert.equal(problems.length, EXACT_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('exact route test is missing TestExactRoute')))
})

test('S11 未接线证明：包外消费者必须为 0', () => {
  assert.deepEqual(validateIsolation({ ...isolation(), consumers: ['backend/internal/x.go'] }), ['exact routes are wired before the integration step: backend/internal/x.go'])
  assert.ok(validateIsolation({ ...isolation(), files: [] }).some((p) => p.includes(ROUTE_SOURCE)))
  assert.ok(validateIsolation({ ...isolation(), tests: [] }).some((p) => p.includes(ROUTE_TEST)))
})

test('S11 行解析只认 m05-s11 空格前缀', () => {
  const log = [
    '--- PASS: TestExactRouteOpensPluginCenterWithoutRanking',
    '    exact_route_test.go:1: m05-s11 hit a',
    '    exact_route_test.go:2: m05-s11-detail noise',
    '    exact_route_test.go:3: m05-s11 fallback b'
  ].join('\n')
  assert.deepEqual(parseExactRows(log), ['hit a', 'fallback b'])
})

test('S11 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S11 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S11 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseExactRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=exact-route package=' + PACKAGE_PATH))
  assert.ok(text.includes('consumers=0'))
  assert.ok(text.includes('passed=9 failed=0 exit=0 race=true'))
})
