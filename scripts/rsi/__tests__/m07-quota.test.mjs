import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAG,
  CASES,
  EVIDENCE_ROWS,
  EXIT_QUOTA,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  QUOTA_FILES,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateCaseNames,
  validateCeilingSource,
  validateEvidence,
  validateIsolation,
  validateSource
} from '../m07-s04-quota.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultFiles: ['queue.go', 'service.go'],
    taggedFiles: ['queue.go', 'service.go', QUOTA_FILES[0]].sort(),
    defaultTests: ['queue_test.go'],
    taggedTests: ['queue_test.go', QUOTA_FILES[1]].sort()
  }
}

test('S04 退出码为该步独占', () => {
  assert.equal(EXIT_QUOTA, 76)
})

test('S04 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestQuota')
  for (const taken of ['TestBoundedQueue', 'TestScoped', 'TestGovernance', 'TestHostIsolation']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S04 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S04 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 14) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S04 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 14 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S04 关闭态：配额文件不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['queue.go', QUOTA_FILES[0]].sort() })
  assert.ok(leaked.some((p) => p.includes(`default build leaked ${QUOTA_FILES[0]}`)))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['queue.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged build missing ${QUOTA_FILES[0]}`)))
  const leakedTest = validateIsolation({ ...isolation(), defaultTests: ['queue_test.go', QUOTA_FILES[1]].sort() })
  assert.ok(leakedTest.some((p) => p.includes(`default tests leaked ${QUOTA_FILES[1]}`)))
})

test('S04 标签构建必须恰好新增一个生产文件', () => {
  const extra = validateIsolation({ ...isolation(), taggedFiles: ['queue.go', 'service.go', QUOTA_FILES[0], 'extra.go'].sort() })
  assert.ok(extra.some((p) => p.includes('tagged files=')))
})

test('S04 源码守卫缺一即报', () => {
  assert.deepEqual(validateSource(process.cwd()), [])
  assert.equal(SOURCE_GUARDS.length, 8)
})

test('S04 默认上限必须复用进程 PID 预算', () => {
  assert.deepEqual(validateCeilingSource(process.cwd()), [])
})

test('S04 十五个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package task_runtime')
  assert.equal(problems.length, CASES.length)
  assert.ok(problems.every((p) => p.startsWith('quota test is missing TestQuota')))
})

test('S04 行解析只认 m07-s04 空格前缀', () => {
  const log = [
    '--- PASS: TestQuotaRejectsNegative',
    '    quota_schema_test.go:1: m07-s04 reject a',
    '    quota_schema_test.go:2: m07-s04-detail noise',
    '    quota_schema_test.go:3: m07-s04 clamp b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['reject a', 'clamp b'])
})

test('S04 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S04 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S04 标签与包路径固定', () => {
  assert.equal(BUILD_TAG, 'rsi_quota')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/task_runtime')
  assert.ok(GO_TEST_FLAGS.includes('-race'))
  assert.ok(GO_TEST_FLAGS.includes('rsi_quota'))
})

test('S04 渲染结果包含隔离计数与 14 行证据', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=3 default_tests=1 tagged_tests=2'))
  assert.ok(text.includes('invocation=quota package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=15 failed=0 exit=0 race=true'))
  assert.deepEqual(parseRows(text), EVIDENCE_ROWS)
})
