import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAG,
  CASES,
  EVIDENCE_ROWS,
  EXIT_GAP,
  EXPECTED_PASSED,
  GAP_FILE,
  GAP_TEST_FILE,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateReuse,
  validateSource
} from '../m08-s01-gap.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: 12, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultFiles: ['definition.go', 'registry.go'],
    taggedFiles: ['definition.go', 'registry.go', GAP_FILE].sort(),
    defaultTests: ['registry_test.go'],
    taggedTests: ['registry_test.go', GAP_TEST_FILE].sort()
  }
}

test('S01 退出码为该步独占', () => assert.equal(EXIT_GAP, 87))

test('S01 用例前缀与标签固定', () => {
  assert.equal(RUN_PATTERN, 'TestGapReport')
  assert.equal(BUILD_TAG, 'rsi_gap')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/capability')
  for (const name of CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S01 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S01 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 11) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S01 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 11 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S01 关闭态：缺口报告不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['definition.go', GAP_FILE].sort() })
  assert.ok(leaked.some((p) => p.includes(`default build leaked ${GAP_FILE}`)))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['definition.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged build missing ${GAP_FILE}`)))
  const leakedTest = validateIsolation({ ...isolation(), defaultTests: ['registry_test.go', GAP_TEST_FILE].sort() })
  assert.ok(leakedTest.some((p) => p.includes(`default tests leaked ${GAP_TEST_FILE}`)))
})

test('S01 必须复用选择器与原因码', () => {
  assert.deepEqual(validateReuse(process.cwd()), [])
})

test('S01 源码守卫缺一即报', () => {
  assert.deepEqual(validateSource(process.cwd()), [])
  assert.equal(SOURCE_GUARDS.length, 8)
})

test('S01 十二个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  assert.equal(validateCaseNames('package capability').length, CASES.length)
})

test('S01 行解析只认 m08-s01 空格前缀', () => {
  const log = [
    '--- PASS: TestGapReportExistingCapability',
    '    gap_report_test.go:1: m08-s01 existing a',
    '    gap_report_test.go:2: m08-s01-detail noise',
    '    gap_report_test.go:3: m08-s01 missing b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['existing a', 'missing b'])
})

test('S01 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n=== RUN x\n--- FAIL: C\n'), { passed: 1, failed: 1 })
})

test('S01 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S01 渲染结果包含隔离计数与 12 行', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=3 default_tests=1 tagged_tests=2'))
  assert.ok(text.includes('invocation=gap package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=12 failed=0 exit=0 race=true'))
  assert.equal(parseRows(text).length, 12)
})

test('S01 期望通过数固定', () => {
  assert.equal(EXPECTED_PASSED, 12)
  assert.equal(EXPECTED_PASSED, CASES.length)
  assert.ok(GO_TEST_FLAGS.includes('-race'))
})
