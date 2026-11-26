import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_PREAUTH,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  ORDER_ANCHORS,
  PACKAGE_PATH,
  PREAUTH_CASES,
  PREAUTH_SOURCE,
  RUN_PATTERN,
  parseGoListFiles,
  parseGoTestOutput,
  parsePreauthRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoForbiddenCalls,
  validateOrder
} from '../m05-s09-preauth.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function selectorSource() {
  return ORDER_ANCHORS.join('\n')
}

function isolation() {
  const shared = ['detail_budget_test.go', 'task_selector_test.go']
  return { defaultTests: [...shared], taggedTests: [...shared, PREAUTH_SOURCE].sort() }
}

test('S09 退出码为该步独占', () => {
  assert.equal(EXIT_PREAUTH, 61)
})

test('S09 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestPreauth')
  for (const taken of ['TestCutover', 'TestScale', 'TestAgentContext', 'TestDetail']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of PREAUTH_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S09 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateOrder(selectorSource()), [])
})

test('S09 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S09 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 8 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S09 顺序断言缺任一锚点即报', () => {
  for (const anchor of ORDER_ANCHORS) {
    const problems = validateOrder(selectorSource().replace(anchor, ''))
    assert.ok(problems.some((p) => p.includes(anchor)), anchor)
  }
})

test('S09 顺序错位会被点名', () => {
  const swapped = [
    ORDER_ANCHORS[0],
    ORDER_ANCHORS[1],
    ORDER_ANCHORS[5],
    ORDER_ANCHORS[6],
    ORDER_ANCHORS[2],
    ORDER_ANCHORS[3],
    ORDER_ANCHORS[4],
    ORDER_ANCHORS[7],
    ORDER_ANCHORS[8]
  ].join('\n')
  const problems = validateOrder(swapped)
  assert.ok(problems.some((p) => p.includes('selector order broken')))
})

test('S09 排序调用必须恰好一次', () => {
  const doubled = selectorSource() + '\nranked, snapshot, err := s.ranking.Rank(ctx, report.Profile, query, candidates)\n'
  assert.ok(validateOrder(doubled).some((p) => p.includes('exactly once')))
})

test('S09 断言守卫钉住 fail-closed 与无回退', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if !errors.Is(err, failure) {'))
  assert.ok(ASSERT_GUARDS.includes('if len(report.Selected) != 0 {'))
  assert.ok(ASSERT_GUARDS.includes('if len(report.Rejected) != 1 || report.Rejected[0].ToolID != denied.ID {'))
})

test('S09 九个用例名必须逐个出现', () => {
  const body = PREAUTH_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package capability_test')
  assert.equal(problems.length, PREAUTH_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('preauth test is missing TestPreauth')))
})

test('S09 禁止逐条注册与整表枚举', () => {
  const clean = 'registry.BatchRegister(ctx, defs)'
  assert.deepEqual(validateNoForbiddenCalls(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoForbiddenCalls(clean + token), [`preauth surface must not contain ${token}`])
  }
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
  assert.ok(GO_TEST_FLAGS.includes('-race'))
})

test('S09 构建标签把预检测试挡在默认集合外', () => {
  assert.equal(BUILD_TAG, 'rsi_preauth')
  const leaked = validateIsolation({ ...isolation(), defaultTests: [...isolation().defaultTests, PREAUTH_SOURCE].sort() })
  assert.ok(leaked.some((p) => p.includes('leaked')))
  const missing = validateIsolation({ ...isolation(), taggedTests: [...isolation().defaultTests] })
  assert.ok(missing.some((p) => p.includes('missing preauth_test.go')))
  const extra = validateIsolation({ ...isolation(), taggedTests: [...isolation().taggedTests, 'zzz_test.go'] })
  assert.ok(extra.some((p) => p.includes('want')))
})

test('S09 行解析只认 m05-s09 空格前缀', () => {
  const log = [
    '--- PASS: TestPreauthDeniedNeverReachesRanking',
    '    preauth_test.go:1: m05-s09 order a',
    '    preauth_test.go:2: m05-s09-detail noise',
    '    preauth_test.go:3: m05-s09 cancel b'
  ].join('\n')
  assert.deepEqual(parsePreauthRows(log), ['order a', 'cancel b'])
})

test('S09 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S09 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b_test.go a_test.go]'), ['a_test.go', 'b_test.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S09 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parsePreauthRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=preauth package=' + PACKAGE_PATH))
  assert.ok(text.includes('delta=[preauth_test.go]'))
  assert.ok(text.includes('passed=9 failed=0 exit=0 race=true'))
})
