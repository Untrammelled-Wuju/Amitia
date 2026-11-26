import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BASELINE_FIELDS,
  BUILD_TAG,
  CUTOVER_CASES,
  CUTOVER_GUARDS,
  DEFAULT_SOURCES,
  EVIDENCE_ROWS,
  EXIT_CUTOVER,
  EXPECTED_PASSED,
  FORBIDDEN_IMPORTS,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  RUN_PATTERN,
  TAGGED_SOURCES,
  TAGGED_TESTS,
  parseCutoverRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateAssertSource,
  validateBaseline,
  validateCaseNames,
  validateEvidence,
  validateGateSource,
  validateIsolation,
  validateNoLegacyEnumerations,
  validateNoPromptImports
} from '../m05-s08-cutover.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return { defaultFiles: [...DEFAULT_SOURCES], taggedFiles: [...TAGGED_SOURCES], taggedTests: [...TAGGED_TESTS] }
}

test('S08 退出码为该步独占', () => {
  assert.equal(EXIT_CUTOVER, 60)
})

test('S08 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestCutover')
  for (const taken of ['TestScale', 'TestAgentContext', 'TestDetail', 'TestTaskGate']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of CUTOVER_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S08 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateBaseline(BASELINE_FIELDS), [])
})

test('S08 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 11) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[4]] }).some((p) => p.includes('exactly once')))
})

test('S08 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: EXPECTED_PASSED - 1 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S08 构建标签把门槛单元挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_cutover')
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
  assert.ok(GO_TEST_FLAGS.includes('-race'))
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['doc.go', 'gate.go'] })
  assert.ok(leaked.some((p) => p.includes('default build files')))
  const noTests = validateIsolation({ ...isolation(), taggedTests: [] })
  assert.ok(noTests.some((p) => p.includes('tagged tests')))
})

test('S08 基准字段必须逐个钉死', () => {
  assert.deepEqual(validateBaseline({ ...BASELINE_FIELDS }), [])
  for (const field of Object.keys(BASELINE_FIELDS)) {
    const drifted = { ...BASELINE_FIELDS, [field]: BASELINE_FIELDS[field] + 1 }
    assert.deepEqual(validateBaseline(drifted), [`baseline.${field}=${BASELINE_FIELDS[field] + 1} want ${BASELINE_FIELDS[field]}`])
  }
  assert.equal(BASELINE_FIELDS.schema_tokens, 1096)
  assert.equal(BASELINE_FIELDS.schema_reads, 8)
})

test('S08 门槛守卫钉住召回、权限、延迟与预算', () => {
  assert.deepEqual(validateGateSource(CUTOVER_GUARDS.join('\n')), [])
  assert.equal(validateGateSource('package retrievalcutover').length, CUTOVER_GUARDS.length)
  assert.ok(CUTOVER_GUARDS.includes('MinTop1Recall:      0.95,'))
  assert.ok(CUTOVER_GUARDS.includes('MaxPermissionLeaks: 0,'))
  assert.ok(CUTOVER_GUARDS.includes('MaxP99Latency:      500 * time.Millisecond,'))
  assert.ok(CUTOVER_GUARDS.includes('return fmt.Errorf("%w: %d failing metrics: %v", ErrThresholdsNotMet, len(u.verdict.Failures), u.verdict.Failures)'))
})

test('S08 断言守卫钉住「未达阈值不得关闭旧装载」', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if err := unit.DisableLegacy(); !errors.Is(err, retrievalcutover.ErrThresholdsNotMet) {'))
  assert.ok(ASSERT_GUARDS.includes('if unit.LegacyDisabled() {'))
  assert.ok(ASSERT_GUARDS.includes('if failures := baseline.Compare(metrics); len(failures) != 0 {'))
})

test('S08 八个用例名必须逐个出现', () => {
  const body = CUTOVER_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package retrievalcutover_test')
  assert.equal(problems.length, CUTOVER_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('cutover test is missing TestCutover')))
})

test('S08 门槛单元不得触碰生产 Prompt', () => {
  const clean = 'import "context"'
  assert.deepEqual(validateNoPromptImports(clean), [])
  for (const token of FORBIDDEN_IMPORTS) {
    assert.deepEqual(validateNoPromptImports(clean + token), [`cutover gate must not import ${token}`])
  }
})

test('S08 门槛单元不得逐条注册或整表枚举', () => {
  const clean = 'registry.BatchRegister(ctx, defs)'
  assert.deepEqual(validateNoLegacyEnumerations(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoLegacyEnumerations(clean + token), [`cutover surface must not call ${token}`])
  }
})

test('S08 行解析只认 m05-s08 空格前缀', () => {
  const log = [
    '--- PASS: TestCutoverMeasuredMetricsPass',
    '    gate_test.go:1: m05-s08 metrics a',
    '    gate_test.go:2: m05-s08-detail latency p50_us=1',
    '    gate_test.go:3: m05-s08 verdict b',
    '    gate_test.go:4: m05-s08-detail noise'
  ].join('\n')
  assert.deepEqual(parseCutoverRows(log), ['metrics a', 'verdict b'])
})

test('S08 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S08 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[gate.go doc.go]'), ['doc.go', 'gate.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S08 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseCutoverRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=cutover package=' + PACKAGE_PATH))
  assert.ok(text.includes('rows=12 guards=' + (CUTOVER_GUARDS.length + ASSERT_GUARDS.length + CUTOVER_CASES.length)))
  assert.ok(text.includes('passed=8 failed=0 exit=0 race=true'))
})
