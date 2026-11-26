import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_SCALE_BENCH,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  RUN_PATTERN,
  SCALE_CASES,
  SCALE_GUARDS,
  SCALE_SOURCES,
  parseGoListFiles,
  parseGoTestOutput,
  parseScaleRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoSequentialRegistration,
  validateScaleSource
} from '../m05-s07-scale-bench.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  const shared = ['detail_budget_test.go', 'task_selector_test.go']
  return {
    defaultTests: [...shared],
    taggedTests: [...shared, SCALE_SOURCES[0]].sort()
  }
}

test('S07 退出码为该步独占', () => {
  assert.equal(EXIT_SCALE_BENCH, 59)
})

test('S07 用例前缀与既有前缀不撞车', () => {
  assert.equal(RUN_PATTERN, 'TestScale')
  for (const taken of ['TestTaskGate', 'TestRanking', 'TestDetail', 'TestAgentContext']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of SCALE_CASES) {
    assert.ok(name.startsWith(RUN_PATTERN))
  }
})

test('S07 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S07 证据行缺失或重复都会被拦下', () => {
  const short = EVIDENCE_ROWS.slice(0, 5)
  assert.ok(validateEvidence({ ...rowSet(), rows: short }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[2]] }).some((p) => p.includes('exactly once')))
})

test('S07 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: EXPECTED_PASSED - 1 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S07 构建标签把规模测试挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_scale')
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
  assert.ok(GO_TEST_FLAGS.includes('-race'))
  const leaked = validateIsolation({ ...isolation(), defaultTests: [...isolation().defaultTests, SCALE_SOURCES[0]].sort() })
  assert.ok(leaked.some((p) => p.includes('leaked scale tests')))
  const missing = validateIsolation({ ...isolation(), taggedTests: [...isolation().defaultTests] })
  assert.ok(missing.some((p) => p.includes('missing scale_bench_test.go')))
  const extra = validateIsolation({ ...isolation(), taggedTests: [...isolation().defaultTests] })
  assert.ok(extra.some((p) => p.includes('want')))
})

test('S07 规模常量与批量装载守卫缺一即报', () => {
  assert.deepEqual(validateScaleSource(SCALE_GUARDS.join('\n')), [])
  assert.equal(validateScaleSource('package capability_test').length, SCALE_GUARDS.length)
  for (const guard of ['scalePlugins      = 2000', 'scaleSlots        = 5', 'scaleOperations   = 200', 'scaleQualityCases = 20']) {
    assert.ok(SCALE_GUARDS.includes(guard))
  }
  assert.ok(SCALE_GUARDS.includes('registry.BatchRegister(context.Background(), defs)'))
})

test('S07 断言守卫钉住尾部延迟、误注入与预算', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if store.Reads() != 0 {'))
  assert.ok(ASSERT_GUARDS.includes('if p99 > p50*8+8*time.Millisecond {'))
  assert.ok(ASSERT_GUARDS.includes('if injected != 0 {'))
  assert.ok(ASSERT_GUARDS.includes('if !equal {'))
  assert.ok(ASSERT_GUARDS.includes('if !taskGateHasReason(rejected.Reasons, capability.ReasonPermissionMissing) {'))
})

test('S07 六个用例名必须逐个出现', () => {
  const body = SCALE_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package capability_test')
  assert.equal(problems.length, SCALE_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('scale benchmark is missing TestScale')))
})

test('S07 禁止逐条注册与整表枚举', () => {
  const clean = 'registry.BatchRegister(context.Background(), defs)'
  assert.deepEqual(validateNoSequentialRegistration(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoSequentialRegistration(clean + token), [`scale benchmark must batch-load the registry but contains ${token}`])
  }
  assert.ok(FORBIDDEN_TOKENS.includes('registry.Register(ctx'))
  assert.ok(FORBIDDEN_TOKENS.includes('GetModelTools(ctx'))
})

test('S07 行解析只认 m05-s07 空格前缀', () => {
  const log = [
    '--- PASS: TestScaleColdIndexBuild',
    '    scale_bench_test.go:1: m05-s07 cold a',
    '    scale_bench_test.go:2: m05-s07-detail hot p50_us=1',
    '    scale_bench_test.go:3: m05-s07 hot b',
    '    scale_bench_test.go:4: m05-s07-detail noise'
  ].join('\n')
  assert.deepEqual(parseScaleRows(log), ['cold a', 'hot b'])
})

test('S07 测试计数只认 PASS/FAIL 行', () => {
  const log = '--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'
  assert.deepEqual(parseGoTestOutput(log), { passed: 2, failed: 1 })
})

test('S07 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[a.go b.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S07 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseScaleRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=scale-bench package=' + PACKAGE_PATH))
  assert.ok(text.includes('rows=6 guards=' + (SCALE_GUARDS.length + ASSERT_GUARDS.length + SCALE_CASES.length)))
  assert.ok(text.includes('passed=6 failed=0 exit=0 race=true'))
})
