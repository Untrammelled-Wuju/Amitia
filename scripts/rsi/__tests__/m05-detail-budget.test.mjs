import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  CURSOR_GUARDS,
  ENUMERATION_TOKENS,
  EVIDENCE_ROWS,
  EXIT_DETAIL_BUDGET,
  EXPECTED_PASSED,
  PAGE_GUARDS,
  RUN_PATTERN,
  TRIM_GUARDS,
  parseDetailRows,
  renderEvidence,
  validateAssertSource,
  validateCursorSource,
  validateEvidence,
  validateNoRegistryEnumeration,
  validatePageSource,
  validateTrimSource
} from '../m05-s05-detail-budget.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function goFiles() {
  return { goFiles: ['detail_budget.go', 'exposure.go'], testFiles: ['detail_budget_test.go'] }
}

test('S05 退出码为该步独占', () => {
  assert.equal(EXIT_DETAIL_BUDGET, 57)
})

test('S05 用例前缀不与既有前缀撞车', () => {
  for (const taken of ['TestDetailx', 'TestRanking', 'TestTaskGate', 'TestCapabilityRevision']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  assert.equal(RUN_PATTERN, 'TestDetail')
})

test('S05 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence({ ...goFiles(), ...rowSet() }), [])
})

test('S05 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 9) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[2]] }).some((p) => p.includes('exactly once')))
})

test('S05 未带 race 或退出非零时拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), exitCode: 2 }).some((p) => p.includes('exit=2')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), passed: 9 }).some((p) => p.includes('passed=9')))
})

test('S05 缺源文件或缺测试文件会被点名', () => {
  const missing = validateEvidence({ ...goFiles(), ...rowSet(), goFiles: ['detail_budget.go'], testFiles: [] })
  assert.ok(missing.some((p) => p.includes('exposure.go')))
  assert.ok(missing.some((p) => p.includes('detail_budget_test.go')))
  const leaked = validateEvidence({ ...goFiles(), ...rowSet(), goFiles: ['detail_budget.go', 'exposure.go', 'detail_budget_test.go'] })
  assert.ok(leaked.some((p) => p.includes('leaked test files')))
})

test('S05 裁剪守卫缺一即报', () => {
  assert.deepEqual(validateTrimSource(TRIM_GUARDS.join('\n')), [])
  const problems = validateTrimSource('package capability')
  assert.equal(problems.length, TRIM_GUARDS.length)
  assert.ok(problems.every((p) => p.startsWith('detail trim source is missing ')))
})

test('S05 required 属性保留与拒绝溢出都有守卫', () => {
  assert.ok(TRIM_GUARDS.includes('if !requiredSet[name] {'))
  assert.ok(TRIM_GUARDS.some((guard) => guard.includes('ErrDetailBudgetExhausted, required')))
  assert.ok(TRIM_GUARDS.includes('if report.OriginalTokens <= budget {'))
})

test('S05 游标守卫覆盖版本、摘要与偏移', () => {
  assert.deepEqual(validateCursorSource(CURSOR_GUARDS.join('\n')), [])
  assert.ok(CURSOR_GUARDS.includes('if decoded.Version != detailCursorVersion {'))
  assert.ok(CURSOR_GUARDS.includes('if decoded.Digest != digest {'))
  assert.ok(CURSOR_GUARDS.includes('if decoded.Offset < 0 {'))
})

test('S05 详情页守卫覆盖 Top-K 上限与预算模式', () => {
  assert.deepEqual(validatePageSource(PAGE_GUARDS.join('\n')), [])
  assert.ok(PAGE_GUARDS.includes('if limit > MaxDetailTopK {'))
  assert.ok(PAGE_GUARDS.includes('if budget == 0 {'))
  assert.ok(PAGE_GUARDS.includes('page.SchemaReads = m.schemaStore.Reads() - readsBefore'))
})

test('S05 详情页不得整表枚举注册表', () => {
  const clean = 'func (m *ToolExposureManager) GetModelToolDetails() {'
  assert.deepEqual(validateNoRegistryEnumeration(clean), [])
  for (const token of ENUMERATION_TOKENS) {
    assert.deepEqual(validateNoRegistryEnumeration(clean + token), [`detail page must not enumerate the registry but contains ${token}`])
  }
})

test('S05 断言守卫缺一即报', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.equal(validateAssertSource('').length, ASSERT_GUARDS.length)
  assert.ok(ASSERT_GUARDS.some((guard) => guard.includes('tokens[100] != tokens[1000]')))
})

test('S05 行解析只认 m05-s05 前缀', () => {
  const log = '--- PASS: X\n    x_test.go:1: m05-s05 a\nnoise m05-s04 b\n    x_test.go:2: m05-s05 c\n'
  assert.deepEqual(parseDetailRows(log), ['a', 'c'])
})

test('S05 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS })
  assert.deepEqual(parseDetailRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('passed=10 failed=0 exit=0 race=true'))
})
