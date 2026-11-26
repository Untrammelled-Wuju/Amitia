import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAGS,
  CASES,
  EVIDENCE_ROWS,
  EXIT_STRESS,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOURCE_GUARDS,
  STRESS_FILE,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateSource,
  validateUserPath
} from '../m07-s07-stress.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: 1, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultFiles: ['queue.go', 'service.go'],
    taggedFiles: ['queue.go', 'service.go', 'bounded_queue.go'].sort(),
    defaultTests: ['queue_test.go'],
    taggedTests: ['queue_test.go', 'bounded_queue_test.go', STRESS_FILE].sort()
  }
}

test('S07 退出码为该步独占', () => {
  assert.equal(EXIT_STRESS, 79)
})

test('S07 用例名固定且不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestCoreStressInjection')
  assert.deepEqual(CASES, ['TestCoreStressInjection'])
  assert.equal(RUN_PATTERN === 'TestBoundedQueue', false)
})

test('S07 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S07 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S07 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 2 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S07 关闭态：压力测试不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultTests: ['queue_test.go', STRESS_FILE].sort() })
  assert.ok(leaked.some((p) => p.includes(`default tests leaked ${STRESS_FILE}`)))
  const missing = validateIsolation({ ...isolation(), taggedTests: ['queue_test.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged tests missing ${STRESS_FILE}`)))
  const missingQueue = validateIsolation({ ...isolation(), taggedFiles: ['queue.go'] })
  assert.ok(missingQueue.some((p) => p.includes('bounded_queue.go')))
})

test('S07 源码守卫缺一即报', () => {
  assert.deepEqual(validateSource(process.cwd()), [])
  assert.equal(SOURCE_GUARDS.length, 8)
})

test('S07 OOM 路径必须记录插件调用与用户准入', () => {
  assert.deepEqual(validateUserPath(process.cwd()), [])
})

test('S07 用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  assert.equal(validateCaseNames('package task_runtime').length, CASES.length)
})

test('S07 行解析只认 m07-s07 空格前缀', () => {
  const log = [
    '--- PASS: TestCoreStressInjection',
    '    stress_injection_test.go:1: m07-s07 storm a',
    '    stress_injection_test.go:2: m07-s07-detail noise',
    '    stress_injection_test.go:3: m07-s07 oom b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['storm a', 'oom b'])
})

test('S07 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n=== RUN x\n--- FAIL: C\n'), { passed: 1, failed: 1 })
})

test('S07 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S07 双标签与包路径固定', () => {
  assert.equal(BUILD_TAGS, 'rsi_backpressure rsi_stress')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/task_runtime')
  assert.ok(GO_TEST_FLAGS.includes('-race'))
  assert.ok(GO_TEST_FLAGS.includes('rsi_backpressure rsi_stress'))
})

test('S07 渲染结果包含隔离计数与 9 行证据', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=3 default_tests=1 tagged_tests=3'))
  assert.ok(text.includes('invocation=stress package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=1 failed=0 exit=0 race=true'))
  assert.deepEqual(parseRows(text), EVIDENCE_ROWS)
})

test('S07 期望通过数固定为 1', () => {
  assert.equal(EXPECTED_PASSED, 1)
  assert.equal(EXPECTED_PASSED, CASES.length)
})
