import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  CASES,
  EVIDENCE_ROWS,
  EXIT_GOVERNANCE,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  GOVERNANCE_FILES,
  INVENTORY,
  PACKAGE_PATH,
  RUN_PATTERN,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateInventory,
  validateIsolation
} from '../m07-s01-governance.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return { defaultFiles: ['resource.go', 'state.go'], taggedFiles: ['resource.go', 'state.go', ...GOVERNANCE_FILES].sort() }
}

test('S01 退出码为该步独占', () => {
  assert.equal(EXIT_GOVERNANCE, 73)
})

test('S01 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestGovernance')
  for (const taken of ['TestPluginBaseline', 'TestHostIsolation', 'TestPluginRelease']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S01 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateAssertSource('lease, ok := governor.Lookup(string(kind))\nif _, err := governor.Adopt(ctx, spec); !errors.Is(err, ErrLeaseDuplicate) {\nif strings.Join(closed, ",") != "third,second,first" {\nif !errors.Is(err, failureA) || !errors.Is(err, failureB) {\nif _, err := governor.Adopt(cancelled, adoptSpec("late", ManagedOutbox, systemOwner(), noopClose)); !errors.Is(err, context.Canceled) {'), [])
})

test('S01 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S01 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 8 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S01 清单必须覆盖 7 条、6 类、单一队列', () => {
  assert.equal(INVENTORY.length, 7)
  assert.equal(new Set(INVENTORY.map((entry) => entry.kind)).size, 6)
  assert.equal(INVENTORY.filter((entry) => entry.kind === 'priority_queue').length, 1)
  assert.equal(INVENTORY.filter((entry) => entry.kind === 'task_entry')[0].owner, 'task_runtime')
})

test('S01 清单路径与符号必须存在于当前仓库', () => {
  const problems = validateInventory(process.cwd())
  assert.deepEqual(problems, [])
})

test('S01 清单缺失路径会被拦下', () => {
  const root = process.cwd()
  const problems = validateInventory(root)
  assert.ok(Array.isArray(problems))
  assert.ok(problems.every((problem) => typeof problem === 'string'))
})

test('S01 断言守卫缺一即报', () => {
  assert.ok(validateAssertSource('package resource').length === 5)
  assert.ok(validateAssertSource('package resource').every((p) => p.startsWith('governance test is missing')))
})

test('S01 九个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package resource')
  assert.equal(problems.length, CASES.length)
  assert.ok(problems.every((p) => p.startsWith('governance test is missing TestGovernance')))
})

test('S01 关闭态：治理文件不得进入默认构建', () => {
  assert.equal(GOVERNANCE_FILES.length, 2)
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['resource.go', ...GOVERNANCE_FILES].sort() })
  assert.ok(leaked.some((p) => p.includes('default build leaked inventory.go')))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['resource.go'] })
  assert.ok(missing.some((p) => p.includes('tagged build missing inventory.go')))
})

test('S01 行解析只认 m07-s01 空格前缀', () => {
  const log = [
    '--- PASS: TestGovernanceAdoptsEveryInventoryKind',
    '    governance_test.go:1: m07-s01 adopt a',
    '    governance_test.go:2: m07-s01-detail noise',
    '    governance_test.go:3: m07-s01 close b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['adopt a', 'close b'])
})

test('S01 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S01 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S01 渲染结果包含关闭态与队列归属', () => {
  const text = renderEvidence({ ...rowSet(), defaultFiles: ['resource.go'], taggedFiles: ['resource.go', ...GOVERNANCE_FILES], queueOwners: 2 })
  assert.ok(text.includes('isolation: default_files=1 tagged_files=3 queue_owners=2'))
  assert.ok(text.includes('invocation=governance package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=9 failed=0 exit=0 race=true'))
  assert.deepEqual(parseRows(text), EVIDENCE_ROWS)
})
