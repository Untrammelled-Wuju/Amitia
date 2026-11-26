import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BENCH_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_QUALITY,
  EXPECTED_PASSED,
  FORBIDDEN_IMPORTS,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  QUALITY_CASES,
  QUALITY_SOURCE,
  RUN_PATTERN,
  parseGoListFiles,
  parseGoTestOutput,
  parseQualityRows,
  renderEvidence,
  validateAssertSource,
  validateBenchSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoModelAnalysis,
  validateNoProductionWiring
} from '../m05-s12-quality.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  const shared = ['detail_budget_test.go', 'exact_route_test.go']
  return { defaultTests: [...shared], taggedTests: [...shared, QUALITY_SOURCE].sort() }
}

test('S12 退出码为该步独占', () => {
  assert.equal(EXIT_QUALITY, 64)
})

test('S12 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestQuality')
  for (const taken of ['TestExactRoute', 'TestSchemaCache', 'TestPreauth', 'TestScale']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of QUALITY_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S12 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateBenchSource(BENCH_GUARDS.join('\n')), [])
})

test('S12 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[4]] }).some((p) => p.includes('exactly once')))
})

test('S12 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 8 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S12 固定任务集规模必须是 12/288/12/24/12', () => {
  assert.ok(BENCH_GUARDS.includes('qualityCapabilities = 12'))
  assert.ok(BENCH_GUARDS.includes('qualityNoise        = 288'))
  assert.ok(BENCH_GUARDS.includes('qualityExactTasks   = 12'))
  assert.ok(BENCH_GUARDS.includes('qualityLexicalTasks = 24'))
  assert.ok(BENCH_GUARDS.includes('qualityAdversarial  = 12'))
  assert.deepEqual(validateBenchSource('package capability_test').length, BENCH_GUARDS.length)
})

test('S12 必须复用 S08 门槛而不是自造阈值', () => {
  assert.ok(BENCH_GUARDS.includes('retrievalcutover.DefaultThresholds()'))
  assert.ok(BENCH_GUARDS.includes('retrievalcutover.Metrics{'))
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
  assert.ok(BUILD_TAG.includes('rsi_cutover'))
})

test('S12 断言守卫钉住召回、载荷、泄漏、延迟与阻断', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if top1 < 0.95 || top5 < 0.98 {'))
  assert.ok(ASSERT_GUARDS.includes('if reduction < 10 {'))
  assert.ok(ASSERT_GUARDS.includes('if unit.LegacyDisabled() {'))
})

test('S12 九个用例名必须逐个出现', () => {
  const body = QUALITY_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package capability_test')
  assert.equal(problems.length, QUALITY_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('quality test is missing TestQuality')))
})

test('S12 基准不得依赖模型分析', () => {
  const clean = 'import "context"'
  assert.deepEqual(validateNoModelAnalysis(clean), [])
  for (const token of FORBIDDEN_IMPORTS) {
    assert.deepEqual(validateNoModelAnalysis(clean + token), [`quality benchmark must not reference ${token}`])
  }
})

test('S12 基准不得改环境或逐条注册', () => {
  const clean = 'registry.BatchRegister(ctx, defs)'
  assert.deepEqual(validateNoProductionWiring(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoProductionWiring(clean + token), [`quality benchmark must not contain ${token}`])
  }
})

test('S12 构建标签把基准挡在默认集合外', () => {
  assert.equal(QUALITY_SOURCE, 'quality_bench_test.go')
  const leaked = validateIsolation({ ...isolation(), defaultTests: [...isolation().defaultTests, QUALITY_SOURCE].sort() })
  assert.ok(leaked.some((p) => p.includes('leaked')))
  const missing = validateIsolation({ ...isolation(), taggedTests: [...isolation().defaultTests] })
  assert.ok(missing.some((p) => p.includes('missing quality_bench_test.go')))
  const extra = validateIsolation({ ...isolation(), taggedTests: [...isolation().taggedTests, 'zzz_test.go'] })
  assert.ok(extra.some((p) => p.includes('want')))
})

test('S12 行解析只认 m05-s12 空格前缀', () => {
  const log = [
    '--- PASS: TestQualityFixedTaskSetRecall',
    '    quality_bench_test.go:1: m05-s12 recall a',
    '    quality_bench_test.go:2: m05-s12-detail latency noise',
    '    quality_bench_test.go:3: m05-s12 gate b'
  ].join('\n')
  assert.deepEqual(parseQualityRows(log), ['recall a', 'gate b'])
})

test('S12 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S12 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b_test.go a_test.go]'), ['a_test.go', 'b_test.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S12 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseQualityRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=quality package=' + PACKAGE_PATH))
  assert.ok(text.includes('tags="' + BUILD_TAG + '"'))
  assert.ok(text.includes('passed=9 failed=0 exit=0 race=true'))
})
