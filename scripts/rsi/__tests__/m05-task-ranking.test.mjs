import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  EVIDENCE_ROWS,
  EXIT_TASK_RANKING,
  EXPECTED_PASSED,
  FILTER_BYPASS_TOKENS,
  INTEGRATION_GUARDS,
  POLICY_GUARDS,
  RUN_PATTERN,
  STRATEGY_GUARDS,
  parseRankingRows,
  renderEvidence,
  validateAssertSource,
  validateEvidence,
  validateFixture,
  validateIntegrationSource,
  validateNoFilterBypass,
  validatePolicySource,
  validateStrategySource
} from '../m05-s04-ranking.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function goFiles() {
  return { goFiles: ['ranking.go', 'task_selector.go'], testFiles: ['ranking_test.go'] }
}

function fixture(overrides = {}) {
  return JSON.stringify({
    schemaVersion: 1,
    weights: { lexical: 1, domain: 1, usage: 1, explicit: 1 },
    tools: [{ id: 'a' }, { id: 'b' }, { id: 'c' }, { id: 'd' }],
    queries: [
      { text: 'explicit', mode: 'explicit', top1: 'a' },
      { text: 'explicit2', mode: 'explicit', top1: 'b' },
      { text: 'confusable', mode: 'confusable', top1: 'c' }
    ],
    ...overrides
  })
}

test('S04 退出码为该步独占', () => {
  assert.equal(EXIT_TASK_RANKING, 56)
})

test('S04 用例前缀不与既有前缀撞车', () => {
  for (const taken of ['TestTaskGate', 'TestCapabilityRevision', 'TestCapabilityDescriptor', 'TestJournal']) {
    assert.equal(RUN_PATTERN.includes(taken), false)
  }
  assert.equal(RUN_PATTERN, 'TestRanking')
})

test('S04 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence({ ...goFiles(), ...rowSet() }), [])
})

test('S04 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[4]] }).some((p) => p.includes('exactly once')))
})

test('S04 未带 race 或退出非零时拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), exitCode: 2 }).some((p) => p.includes('exit=2')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), passed: 8 }).some((p) => p.includes('passed=8')))
})

test('S04 缺源文件或缺测试文件会被点名', () => {
  const missing = validateEvidence({ ...goFiles(), ...rowSet(), goFiles: ['ranking.go'], testFiles: [] })
  assert.ok(missing.some((p) => p.includes('task_selector.go')))
  assert.ok(missing.some((p) => p.includes('ranking_test.go')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), goFiles: ['ranking.go', 'task_selector.go', 'ranking_test.go'] }).some((p) => p.includes('leaked test files')))
})

test('S04 策略接口守卫缺一即报', () => {
  assert.deepEqual(validateStrategySource(STRATEGY_GUARDS.join('\n')), [])
  const problems = validateStrategySource('package capability')
  assert.equal(problems.length, STRATEGY_GUARDS.length)
  assert.ok(problems.every((p) => p.startsWith('ranking strategy source is missing ')))
})

test('S04 策略快照必须可回退且带摘要', () => {
  assert.deepEqual(validatePolicySource(POLICY_GUARDS.join('\n')), [])
  assert.ok(POLICY_GUARDS.includes('p.active, p.previous = p.previous, p.active'))
  assert.ok(POLICY_GUARDS.some((guard) => guard.includes('ErrRankingSnapshotDrift')))
})

test('S04 排序策略不得重写硬过滤', () => {
  const clean = 'func (h *HybridStrategy) Rank() {'
  assert.deepEqual(validateNoFilterBypass(clean), [])
  for (const token of FILTER_BYPASS_TOKENS) {
    assert.deepEqual(validateNoFilterBypass(clean + token), [`ranking strategy must not re-implement the hard filter but contains ${token}`])
  }
})

test('S04 选择器必须保留关闭时的旧路径', () => {
  assert.deepEqual(validateIntegrationSource(INTEGRATION_GUARDS.join('\n')), [])
  const problems = validateIntegrationSource('if s.ranking != nil {')
  assert.ok(problems.length > 0)
  assert.ok(INTEGRATION_GUARDS.includes('sort.SliceStable(report.Selected, func(i, j int) bool {'))
})

test('S04 断言守卫缺一即报', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.equal(validateAssertSource('').length, ASSERT_GUARDS.length)
})

test('S04 fixture 校验能放过合法样例并点名各类缺陷', () => {
  assert.deepEqual(validateFixture(fixture()), [])
  assert.ok(validateFixture('{').some((p) => p.includes('not valid JSON')))
  assert.ok(validateFixture(fixture({ schemaVersion: 2 })).some((p) => p.includes('schemaVersion=2')))
  assert.ok(validateFixture(fixture({ tools: [{ id: 'a' }] })).some((p) => p.includes('tools=1')))
  assert.ok(validateFixture(fixture({ queries: [] })).some((p) => p.includes('queries=0')))
  const unknown = fixture({ queries: [{ text: 'x', mode: 'explicit', top1: 'zzz' }] })
  assert.ok(validateFixture(unknown).some((p) => p.includes('unknown tool zzz')))
  const zero = fixture({ weights: { lexical: 0, domain: 0, usage: 0, explicit: 0 } })
  assert.ok(validateFixture(zero).some((p) => p.includes('must not all be zero')))
  const negative = fixture({ weights: { lexical: -1, domain: 1, usage: 1, explicit: 1 } })
  assert.ok(validateFixture(negative).some((p) => p.includes('finite and non-negative')))
})

test('S04 行解析只认 m05-s04 前缀', () => {
  const log = '--- PASS: X\n    x_test.go:1: m05-s04 a\nnoise m05-s03 b\n    x_test.go:2: m05-s04 c\n'
  assert.deepEqual(parseRankingRows(log), ['a', 'c'])
})

test('S04 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS })
  assert.deepEqual(parseRankingRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('passed=9 failed=0 exit=0 race=true'))
})
