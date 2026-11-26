import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAG,
  CASES,
  EVIDENCE_ROWS,
  EXIT_BACKPRESSURE,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  QUEUE_FILE,
  QUEUE_TEST_FILE,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateCaseNames,
  validateEvidence,
  validateHardBound,
  validateIsolation,
  validateSource
} from '../m07-s03-backpressure.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultFiles: ['queue.go', 'service.go'],
    taggedFiles: ['queue.go', 'service.go', QUEUE_FILE].sort(),
    defaultTests: ['queue_test.go'],
    taggedTests: ['queue_test.go', QUEUE_TEST_FILE].sort()
  }
}

test('S03 退出码为该步独占', () => {
  assert.equal(EXIT_BACKPRESSURE, 75)
})

test('S03 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestBoundedQueue')
  for (const taken of ['TestScoped', 'TestGovernance', 'TestHostIsolation', 'TestE2E']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S03 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S03 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 13) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S03 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 13 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S03 关闭态：队列与测试不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['queue.go', QUEUE_FILE].sort() })
  assert.ok(leaked.some((p) => p.includes(`default build leaked ${QUEUE_FILE}`)))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['queue.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged build missing ${QUEUE_FILE}`)))
  const leakedTest = validateIsolation({ ...isolation(), defaultTests: ['queue_test.go', QUEUE_TEST_FILE].sort() })
  assert.ok(leakedTest.some((p) => p.includes(`default tests leaked ${QUEUE_TEST_FILE}`)))
})

test('S03 标签构建必须恰好新增一个生产文件', () => {
  const extra = validateIsolation({ ...isolation(), taggedFiles: ['queue.go', 'service.go', QUEUE_FILE, 'extra.go'].sort() })
  assert.ok(extra.some((p) => p.includes('tagged files=')))
})

test('S03 源码守卫缺一即报', () => {
  assert.deepEqual(validateSource(process.cwd()), [])
  assert.equal(SOURCE_GUARDS.length, 8)
})

test('S03 硬上界不得复制旧队列的优先级旁路', () => {
  assert.deepEqual(validateHardBound(process.cwd()), [])
})

test('S03 十四个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package task_runtime')
  assert.equal(problems.length, CASES.length)
  assert.ok(problems.every((p) => p.startsWith('bounded queue test is missing TestBoundedQueue')))
})

test('S03 行解析只认 m07-s03 空格前缀', () => {
  const log = [
    '--- PASS: TestBoundedQueueCapacityBound',
    '    bounded_queue_test.go:1: m07-s03 capacity a',
    '    bounded_queue_test.go:2: m07-s03-detail noise',
    '    bounded_queue_test.go:3: m07-s03 preempt b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['capacity a', 'preempt b'])
})

test('S03 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S03 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S03 标签与包路径固定', () => {
  assert.equal(BUILD_TAG, 'rsi_backpressure')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/task_runtime')
  assert.ok(GO_TEST_FLAGS.includes('-race'))
  assert.ok(GO_TEST_FLAGS.includes('rsi_backpressure'))
})

test('S03 渲染结果包含隔离计数与 13 行证据', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=3 default_tests=1 tagged_tests=2'))
  assert.ok(text.includes('invocation=backpressure package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=14 failed=0 exit=0 race=true'))
  assert.deepEqual(parseRows(text), EVIDENCE_ROWS)
})
