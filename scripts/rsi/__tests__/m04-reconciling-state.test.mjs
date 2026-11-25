import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  ASSERT_GUARDS,
  EVIDENCE_ROWS,
  EXIT_RECONCILING_STATE,
  EXPECTED_PASSED,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  STATE_GUARDS,
  WRITE_TOKENS,
  parseGoListFiles,
  parseGoTestOutput,
  parseStateRows,
  renderEvidence,
  validateAssertSource,
  validateEvidence,
  validateStateSource,
  validateWriteFree
} from '../m04-reconciling-state.mjs'

const LOG = [
  '=== RUN   TestReconcilingStateAbsent',
  '    reconciling_state_test.go:61: m04-s09 state=absent ledger=none reconciliation=none auto=false next=none',
  '--- PASS: TestReconcilingStateAbsent (0.02s)',
  ...EVIDENCE_ROWS.slice(1, 8).map((row, index) => `    reconciling_state_test.go:${100 + index}: m04-s09 ${row}`),
  '--- PASS: TestReconcilingStateRest (0.03s)',
  '    reconciling_state_test.go:258: m04-s09 ' + EVIDENCE_ROWS[8],
  '--- PASS: TestReconcilingStateInvalidAndCancelled (0.01s)',
  'PASS',
  'ok  \tgithub.com/u-ai/backend/internal/devicemesh/executionjournal\t1.930s'
].join('\n')

const SOURCE = STATE_GUARDS.join('\n')
const TEST_SOURCE = ASSERT_GUARDS.join('\n')

function evidence(overrides = {}) {
  return {
    goFiles: [
      'effect_token.go',
      'inspection.go',
      'ledger.go',
      'readiness.go',
      'reconciling_state.go',
      'reconciliation.go',
      'run_guard.go',
      'store.go',
      'task_inspection.go'
    ],
    testFiles: ['reconciling_state_test.go'],
    rows: [...EVIDENCE_ROWS],
    source: SOURCE,
    testSource: TEST_SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('S09 门禁常量已冻结', () => {
  assert.equal(EXIT_RECONCILING_STATE, 49)
  assert.equal(PACKAGE_PATH, './internal/devicemesh/executionjournal')
  assert.equal(RUN_PATTERN, 'TestReconcilingState')
  assert.equal(EXPECTED_PASSED, 9)
  assert.deepEqual(REQUIRED_SOURCES, ['reconciling_state.go'])
  assert.deepEqual(REQUIRED_TESTS, ['reconciling_state_test.go'])
  assert.equal(EVIDENCE_ROWS.length, 9)
  assert.equal(STATE_GUARDS.length + ASSERT_GUARDS.length, 14)
})

test('S09 只读面被源码守卫覆盖', () => {
  assert.deepEqual(validateStateSource(SOURCE), [])
  assert.deepEqual(validateWriteFree(SOURCE), [])
  assert.deepEqual(validateAssertSource(TEST_SOURCE), [])
})

test('S09 源码守卫缺一条即报出原文', () => {
  const problems = validateStateSource(SOURCE.replace(STATE_GUARDS[4], ''))
  assert.equal(problems.length, 1)
  assert.match(problems[0], /reconciling state source is missing func \(l \*InvocationLedger\) DescribeReconciliation/)
})

test('S09 断言守卫缺一条即报出原文', () => {
  const problems = validateAssertSource(TEST_SOURCE.replace(ASSERT_GUARDS[2], ''))
  assert.deepEqual(problems, ['reconciling state test is missing if before != after {'])
})

test('S09 只读守卫拒写语句', () => {
  for (const token of WRITE_TOKENS) {
    const problems = validateWriteFree(`${SOURCE}\n${token} something`)
    assert.deepEqual(problems, [`reconciling state source must stay read-only but contains ${token}`])
  }
})

test('S09 证据行按顺序解析且去掉前缀', () => {
  const rows = parseStateRows(LOG)
  assert.deepEqual(rows, EVIDENCE_ROWS)
})

test('S09 缺行必须判失败', () => {
  const rows = EVIDENCE_ROWS.filter((row) => !row.startsWith('state=blocked'))
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((item) => item.includes('evidence rows=8 want 9')))
  assert.ok(problems.some((item) => item.includes('state=blocked')))
})

test('S09 同一行打印两次必须判失败', () => {
  const rows = [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((item) => item.includes('must print exactly once (2)')))
})

test('S09 行内容被改必须判失败', () => {
  const rows = EVIDENCE_ROWS.map((row) => (row.includes('auto=true') ? row.replace('auto=true', 'auto=false') : row))
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((item) => item.includes('not_delivered auto=true')))
})

test('S09 未知结果被改成可自动补发必须判失败', () => {
  const rows = EVIDENCE_ROWS.map((row) => (row.startsWith('state=ambiguous ledger=unknown') ? row.replace('auto=false', 'auto=true') : row))
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((item) => item.includes('state=ambiguous ledger=unknown reconciliation=none auto=false')))
})

test('S09 只读证明缺失必须判失败', () => {
  const rows = EVIDENCE_ROWS.filter((row) => !row.includes('readonly=true'))
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((item) => item.includes('readonly=true')))
})

test('S09 冲突行被抹掉必须判失败', () => {
  const rows = EVIDENCE_ROWS.filter((row) => !row.includes('conflict=true'))
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((item) => item.includes('conflict=true')))
})

test('S09 源码文件必须出现在默认构建', () => {
  const goFiles = evidence().goFiles.filter((file) => file !== 'reconciling_state.go')
  const problems = validateEvidence(evidence({ goFiles }))
  assert.deepEqual(problems, ['default build is missing reconciling_state.go'])
})

test('S09 默认构建混入测试文件必须判失败', () => {
  const goFiles = [...evidence().goFiles, 'reconciling_state_test.go']
  const problems = validateEvidence(evidence({ goFiles }))
  assert.ok(problems.includes('default build leaked test files'))
})

test('S09 测试文件缺失必须判失败', () => {
  const problems = validateEvidence(evidence({ testFiles: [] }))
  assert.deepEqual(problems, ['test build is missing reconciling_state_test.go'])
})

test('S09 用例数、失败数、退出码与竞态都必须匹配', () => {
  assert.ok(validateEvidence(evidence({ passed: 8 })).includes('passed=8 want 9'))
  assert.ok(validateEvidence(evidence({ failed: 1 })).includes('failed=1 want 0'))
  assert.ok(validateEvidence(evidence({ exitCode: 1 })).includes('go test exit=1 want 0'))
  assert.ok(validateEvidence(evidence({ race: false })).includes('go test must run with -race'))
})

test('S09 完整证据必须零问题', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('S09 渲染证据先打印口径再逐行打印原文', () => {
  const rendered = renderEvidence(evidence())
  const lines = rendered.trimEnd().split('\n')
  assert.match(lines[0], /rows=9 guards=14/)
  assert.match(lines[1], /sources=1\/1 testExcluded=true/)
  assert.deepEqual(lines.slice(2, 11), EVIDENCE_ROWS.map((row) => `m04-s09 ${row}`))
  assert.equal(lines[11], 'passed=9 failed=0 exit=0 race=true')
})

test('S09 go 测试输出统计只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 3, failed: 0 })
  assert.deepEqual(parseGoTestOutput(`${LOG}\n--- FAIL: TestReconcilingStateAbsent (0.01s)`), { passed: 3, failed: 1 })
})

test('S09 go list 解析拒绝非数组输出', () => {
  assert.deepEqual(parseGoListFiles('[a.go b.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('a.go b.go'), [])
})
