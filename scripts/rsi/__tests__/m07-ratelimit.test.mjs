import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAG,
  CASES,
  EVIDENCE_ROWS,
  EXIT_RATELIMIT,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  LIMITER_FILE,
  LIMITER_TEST_FILE,
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
  validateSource
} from '../m07-s14-ratelimit.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: 12, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultFiles: ['queue.go', 'service.go'],
    taggedFiles: ['queue.go', 'service.go', LIMITER_FILE].sort(),
    defaultTests: ['queue_test.go'],
    taggedTests: ['queue_test.go', LIMITER_TEST_FILE].sort()
  }
}

test('S14 退出码为该步独占', () => assert.equal(EXIT_RATELIMIT, 86))

test('S14 用例前缀与标签固定', () => {
  assert.equal(RUN_PATTERN, 'TestEventLimiter')
  assert.equal(BUILD_TAG, 'rsi_ratelimit')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/task_runtime')
  for (const name of CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S14 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S14 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 11) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S14 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 11 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S14 关闭态：限流与死信不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['queue.go', LIMITER_FILE].sort() })
  assert.ok(leaked.some((p) => p.includes(`default build leaked ${LIMITER_FILE}`)))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['queue.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged build missing ${LIMITER_FILE}`)))
  const leakedTest = validateIsolation({ ...isolation(), defaultTests: ['queue_test.go', LIMITER_TEST_FILE].sort() })
  assert.ok(leakedTest.some((p) => p.includes(`default tests leaked ${LIMITER_TEST_FILE}`)))
})

test('S14 源码守卫缺一即报', () => {
  assert.deepEqual(validateSource(process.cwd()), [])
  assert.equal(SOURCE_GUARDS.length, 8)
})

test('S14 十二个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  assert.equal(validateCaseNames('package task_runtime').length, CASES.length)
})

test('S14 行解析只认 m07-s14 空格前缀', () => {
  const log = [
    '--- PASS: TestEventLimiterAdmitsWithinLimits',
    '    event_rate_limit_test.go:1: m07-s14 admit a',
    '    event_rate_limit_test.go:2: m07-s14-detail noise',
    '    event_rate_limit_test.go:3: m07-s14 trace b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['admit a', 'trace b'])
})

test('S14 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n=== RUN x\n--- FAIL: C\n'), { passed: 1, failed: 1 })
})

test('S14 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S14 渲染结果包含隔离计数与 12 行', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=3 default_tests=1 tagged_tests=2'))
  assert.ok(text.includes('invocation=ratelimit package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=12 failed=0 exit=0 race=true'))
  assert.equal(parseRows(text).length, 12)
})

test('S14 期望通过数固定', () => {
  assert.equal(EXPECTED_PASSED, 12)
  assert.equal(EXPECTED_PASSED, CASES.length)
  assert.ok(GO_TEST_FLAGS.includes('-race'))
})

test('S14 守卫集合完整', () => {
  assert.ok(SOURCE_GUARDS.every((guard) => typeof guard === 'string' && guard.length > 0))
})
