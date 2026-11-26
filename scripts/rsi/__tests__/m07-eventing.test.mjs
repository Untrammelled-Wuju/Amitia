import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAG,
  CASES,
  EVIDENCE_ROWS,
  EXIT_EVENTING,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  REGISTRY_FILE,
  REGISTRY_TEST_FILE,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoWakeOnSubscribe,
  validateSource
} from '../m07-s02-eventing.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultFiles: ['event.go', 'service.go'],
    taggedFiles: ['event.go', 'service.go', REGISTRY_FILE].sort(),
    defaultTests: ['event_test.go'],
    taggedTests: ['event_test.go', REGISTRY_TEST_FILE].sort()
  }
}

test('S02 退出码为该步独占', () => {
  assert.equal(EXIT_EVENTING, 74)
})

test('S02 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestScoped')
  for (const taken of ['TestGovernance', 'TestHostIsolation', 'TestPluginBaseline', 'TestE2E']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S02 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S02 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 13) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S02 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 12 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S02 关闭态：注册表与测试不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['event.go', REGISTRY_FILE].sort() })
  assert.ok(leaked.some((p) => p.includes(`default build leaked ${REGISTRY_FILE}`)))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['event.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged build missing ${REGISTRY_FILE}`)))
  const leakedTest = validateIsolation({ ...isolation(), defaultTests: ['event_test.go', REGISTRY_TEST_FILE].sort() })
  assert.ok(leakedTest.some((p) => p.includes(`default tests leaked ${REGISTRY_TEST_FILE}`)))
})

test('S02 标签构建必须恰好新增一个生产文件', () => {
  const extra = validateIsolation({ ...isolation(), taggedFiles: ['event.go', 'service.go', REGISTRY_FILE, 'extra.go'].sort() })
  assert.ok(extra.some((p) => p.includes('tagged files=')))
})

test('S02 订阅路径不得唤醒冷态宿主', () => {
  assert.deepEqual(validateNoWakeOnSubscribe(process.cwd()), [])
})

test('S02 源码守卫缺一即报', () => {
  const problems = validateSource(process.cwd())
  assert.deepEqual(problems, [])
  assert.equal(SOURCE_GUARDS.length, 8)
  assert.ok(SOURCE_GUARDS.every((guard) => typeof guard === 'string'))
})

test('S02 十三个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package event')
  assert.equal(problems.length, CASES.length)
  assert.ok(problems.every((p) => p.startsWith('scoped event test is missing TestScoped')))
})

test('S02 行解析只认 m07-s02 空格前缀', () => {
  const log = [
    '--- PASS: TestScopedSubscribeIsIdempotent',
    '    scope_registry_test.go:1: m07-s02 subscribe a',
    '    scope_registry_test.go:2: m07-s02-detail noise',
    '    scope_registry_test.go:3: m07-s02 duplicate b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['subscribe a', 'duplicate b'])
})

test('S02 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S02 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S02 标签与包路径固定', () => {
  assert.equal(BUILD_TAG, 'rsi_eventing')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/event')
  assert.ok(GO_TEST_FLAGS.includes('-race'))
  assert.ok(GO_TEST_FLAGS.includes('rsi_eventing'))
})

test('S02 渲染结果包含隔离计数与 14 行证据', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=3 default_tests=1 tagged_tests=2'))
  assert.ok(text.includes('invocation=eventing package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=13 failed=0 exit=0 race=true'))
  assert.deepEqual(parseRows(text), EVIDENCE_ROWS)
})
