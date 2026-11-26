import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAG,
  CASES,
  EVIDENCE_ROWS,
  EXIT_EXPERIMENT,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  GOVERNOR_FILE,
  GOVERNOR_TEST_FILE,
  LADDER_PACKAGE_PATH,
  LADDER_TAGS,
  LADDER_TEST_FILE,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOURCE_GUARDS,
  countPluginPackages,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateLadderIsolation,
  validateLadderSource,
  validatePendingFilter,
  validateSource
} from '../m07-s06-experiment.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true, pluginPackages: 9 }
}

function isolation() {
  return {
    defaultFiles: ['queue.go', 'service.go'],
    taggedFiles: ['queue.go', 'service.go', GOVERNOR_FILE].sort(),
    defaultTests: ['queue_test.go'],
    taggedTests: ['queue_test.go', GOVERNOR_TEST_FILE].sort(),
    ladderDefaultTests: ['enablement_test.go'],
    ladderTaggedTests: ['baseline_test.go', 'enablement_test.go', LADDER_TEST_FILE].sort()
  }
}

test('S06 退出码为该步独占', () => {
  assert.equal(EXIT_EXPERIMENT, 78)
})

test('S06 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestExperimentGovernor')
  for (const taken of ['TestQuotaProfile', 'TestQuotaRejectsNonFinite', 'TestBoundedQueue']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
})

test('S06 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateLadderIsolation(isolation()), [])
})

test('S06 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 14) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S06 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 13 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S06 关闭态：治理器不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['queue.go', GOVERNOR_FILE].sort() })
  assert.ok(leaked.some((p) => p.includes(`default build leaked ${GOVERNOR_FILE}`)))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['queue.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged build missing ${GOVERNOR_FILE}`)))
  const leakedTest = validateIsolation({ ...isolation(), defaultTests: ['queue_test.go', GOVERNOR_TEST_FILE].sort() })
  assert.ok(leakedTest.some((p) => p.includes(`default tests leaked ${GOVERNOR_TEST_FILE}`)))
})

test('S06 真实阶梯测试不得进入默认构建', () => {
  const leaked = validateLadderIsolation({ ...isolation(), ladderDefaultTests: ['enablement_test.go', LADDER_TEST_FILE].sort() })
  assert.ok(leaked.some((p) => p.includes(`default tests leaked ${LADDER_TEST_FILE}`)))
  const missing = validateLadderIsolation({ ...isolation(), ladderTaggedTests: ['enablement_test.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged tests missing ${LADDER_TEST_FILE}`)))
})

test('S06 源码守卫缺一即报', () => {
  assert.deepEqual(validateSource(process.cwd()), [])
  assert.equal(SOURCE_GUARDS.length, 8)
})

test('S06 恢复必须过滤已完成副作用', () => {
  assert.deepEqual(validatePendingFilter(process.cwd()), [])
})

test('S06 真实阶梯必须解析并安装真实清单', () => {
  assert.deepEqual(validateLadderSource(process.cwd()), [])
})

test('S06 十四个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package task_runtime')
  assert.equal(problems.length, CASES.length)
})

test('S06 行解析只认 m07-s06 空格前缀', () => {
  const log = [
    '--- PASS: TestExperimentGovernorIdleReclaimsAfterTTL',
    '    experiment_governor_test.go:1: m07-s06 idle a',
    '    experiment_governor_test.go:2: m07-s06-detail noise',
    '    experiment_governor_test.go:3: m07-s06 resume b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['idle a', 'resume b'])
})

test('S06 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S06 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S06 真实插件供给必须被如实计数', () => {
  const unique = countPluginPackages(process.cwd())
  assert.ok(Number.isInteger(unique))
  assert.ok(unique >= 1)
  assert.ok(unique < 10)
})

test('S06 标签、包路径与阶梯包固定', () => {
  assert.equal(BUILD_TAG, 'rsi_experiment')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/task_runtime')
  assert.equal(LADDER_PACKAGE_PATH, './internal/extension/kernel/enablement')
  assert.equal(LADDER_TAGS, 'rsi_lifecycle rsi_experiment')
  assert.ok(GO_TEST_FLAGS.includes('-race'))
})

test('S06 渲染结果包含供给缺口、阶梯隔离与 15 行证据', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=3 default_tests=1 tagged_tests=2'))
  assert.ok(text.includes('ladder_isolation: default_tests=1 tagged_tests=3'))
  assert.ok(text.includes('plugin_supply unique=9 required=10 gap=true'))
  assert.ok(text.includes('invocation=experiment package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=14 failed=0 exit=0 race=true'))
  assert.deepEqual(parseRows(text), EVIDENCE_ROWS)
})
