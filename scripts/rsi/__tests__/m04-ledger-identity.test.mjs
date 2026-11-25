import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  ASSERT_GUARDS,
  COMMIT_GUARDS,
  EVIDENCE_ROWS,
  EXIT_LEDGER_IDENTITY,
  EXPECTED_PASSED,
  IDENTITY_GUARDS,
  INSPECT_WRITE_TOKENS,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  SCHEMA_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseIdentityRows,
  renderEvidence,
  validateAssertSource,
  validateCommitGuard,
  validateEvidence,
  validateIdentitySource,
  validateInspectReadOnly,
  validateSchemaSource
} from '../m04-ledger-identity.mjs'

const LOG = [
  '=== RUN   TestLedgerIdentityRejectsForeignInvocation',
  `    ledger_identity_test.go:97: m04-s10 ${EVIDENCE_ROWS[0]}`,
  '--- PASS: TestLedgerIdentityRejectsForeignInvocation (0.02s)',
  ...EVIDENCE_ROWS.slice(1, 5).map((row, index) => `    ledger_identity_test.go:${120 + index}: m04-s10 ${row}`),
  '--- PASS: TestLedgerIdentityKeyParticipates (0.02s)',
  '=== RUN   TestLedgerIdentitySurvivesRestart',
  `    ledger_identity_test.go:258: m04-s10 ${EVIDENCE_ROWS[5]}`,
  '--- PASS: TestLedgerIdentitySurvivesRestart (0.03s)',
  ...EVIDENCE_ROWS.slice(6).map((row, index) => `    ledger_identity_test.go:${300 + index}: m04-s10 ${row}`),
  '--- PASS: TestEffectTokenImmutabilitySurvivesRestart (0.02s)',
  'PASS',
  'ok  \tgithub.com/u-ai/backend/internal/devicemesh/executionjournal\t2.035s'
].join('\n')

const SOURCE = IDENTITY_GUARDS.join('\n')
const LEDGER_SOURCE = COMMIT_GUARDS.join('\n')
const SCHEMA_SOURCE = SCHEMA_GUARDS.join('\n')
const TEST_SOURCE = ASSERT_GUARDS.join('\n')

function evidence(overrides = {}) {
  return {
    goFiles: [
      'effect_token.go',
      'inspection.go',
      'ledger.go',
      'ledger_identity.go',
      'readiness.go',
      'reconciling_state.go',
      'reconciliation.go',
      'run_guard.go',
      'store.go',
      'task_inspection.go'
    ],
    testFiles: ['ledger_identity_test.go'],
    rows: [...EVIDENCE_ROWS],
    identitySource: SOURCE,
    ledgerSource: LEDGER_SOURCE,
    schemaSource: SCHEMA_SOURCE,
    testSource: TEST_SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('S10 门禁常量已冻结', () => {
  assert.equal(EXIT_LEDGER_IDENTITY, 50)
  assert.equal(PACKAGE_PATH, './internal/devicemesh/executionjournal')
  assert.equal(EXPECTED_PASSED, 10)
  assert.deepEqual(REQUIRED_SOURCES, ['ledger.go', 'ledger_identity.go', 'effect_token.go'])
  assert.deepEqual(REQUIRED_TESTS, ['ledger_identity_test.go'])
  assert.equal(EVIDENCE_ROWS.length, 10)
})

test('S10 的测试运行模式只命中本步骤的十个用例', () => {
  const pattern = new RegExp(RUN_PATTERN)
  const included = [
    'TestJournalIdentityRejectsForeignInvocation',
    'TestJournalIdentityReplayOfSameInvocation',
    'TestJournalIdentityScopeParticipates',
    'TestJournalIdentityKeyParticipates',
    'TestJournalIdentityInspectBindingIsReadOnly',
    'TestJournalIdentitySurvivesRestart',
    'TestJournalTokenIdentityIsImmutable',
    'TestJournalTokenRemovalIsBlocked',
    'TestJournalTokenStateTransitionAllowed',
    'TestJournalTokenImmutabilitySurvivesRestart'
  ]
  const excluded = [
    'TestEffectTokenBindsRequest',
    'TestEffectTokenRequiresMutatingRequest',
    'TestReconcilingStateAbsent',
    'TestLedgerRejectsInvalidKeys',
    'TestLedgerReplaysWithoutReexecution'
  ]
  for (const name of included) assert.ok(pattern.test(name), `missing ${name}`)
  for (const name of excluded) assert.ok(!pattern.test(name), `unexpected ${name}`)
})

test('S10 的用例名不与 S02/S04 的前缀门禁撞车', () => {
  const s02 = /TestLedger/
  const s04 = /TestEffect/
  const s10 = new RegExp(RUN_PATTERN)
  const names = [
    'TestJournalIdentityRejectsForeignInvocation',
    'TestJournalIdentityReplayOfSameInvocation',
    'TestJournalIdentityScopeParticipates',
    'TestJournalIdentityKeyParticipates',
    'TestJournalIdentityInspectBindingIsReadOnly',
    'TestJournalIdentitySurvivesRestart',
    'TestJournalTokenIdentityIsImmutable',
    'TestJournalTokenRemovalIsBlocked',
    'TestJournalTokenStateTransitionAllowed',
    'TestJournalTokenImmutabilitySurvivesRestart'
  ]
  for (const name of names) {
    assert.ok(s10.test(name), `S10 未命中 ${name}`)
    assert.ok(!s02.test(name), `S02 前缀意外命中 ${name}`)
    assert.ok(!s04.test(name), `S04 前缀意外命中 ${name}`)
  }
})

test('S10 的身份判定面被冻结且只读', () => {
  assert.deepEqual(validateIdentitySource(SOURCE), [])
  assert.deepEqual(validateInspectReadOnly(SOURCE), [])
  const missing = validateIdentitySource(IDENTITY_GUARDS.filter((guard) => guard !== IDENTITY_GUARDS[4]).join('\n'))
  assert.equal(missing.length, 1)
  assert.match(missing[0], /ledger identity source is missing entry, found, err := l\.Lookup/)
})

test('S10 拒绝把只读判定写成写入', () => {
  for (const token of INSPECT_WRITE_TOKENS) {
    const problems = validateInspectReadOnly(`${SOURCE}\n${token} kernel_invocation_ledger`)
    assert.equal(problems.length, 1, `token ${token} 未被拦住`)
    assert.match(problems[0], /binding inspection must stay read-only/)
  }
})

test('S10 的提交身份校验被冻结', () => {
  assert.deepEqual(validateCommitGuard(LEDGER_SOURCE), [])
  const dropped = COMMIT_GUARDS.filter((guard) => guard !== COMMIT_GUARDS[0]).join('\n')
  const problems = validateCommitGuard(dropped)
  assert.equal(problems.length, 1)
  assert.match(problems[0], /ledger commit source is missing if entry\.InvocationID/)
})

test('S10 的不可篡改约束必须落在数据库层', () => {
  assert.deepEqual(validateSchemaSource(SCHEMA_SOURCE), [])
  for (const guard of ['BEFORE UPDATE OF scope_key,idempotency_key,attempt,salt,token ON kernel_invocation_effect', 'BEFORE DELETE ON kernel_invocation_effect']) {
    const problems = validateSchemaSource(SCHEMA_GUARDS.filter((entry) => entry !== guard).join('\n'))
    assert.equal(problems.length, 1, `guard ${guard} 未生效`)
  }
})

test('S10 的断言面被冻结', () => {
  assert.deepEqual(validateAssertSource(TEST_SOURCE), [])
  const problems = validateAssertSource(ASSERT_GUARDS.filter((guard) => guard !== ASSERT_GUARDS[2]).join('\n'))
  assert.equal(problems.length, 1)
  assert.match(problems[0], /ledger identity test is missing if !executionjournal\.IsEffectTokenImmutableError\(saltErr\)/)
})

test('S10 的证据行逐条解析且顺序稳定', () => {
  const rows = parseIdentityRows(LOG)
  assert.deepEqual(rows, EVIDENCE_ROWS)
  assert.deepEqual(parseIdentityRows('no evidence here'), [])
  assert.deepEqual(parseIdentityRows(''), [])
})

test('S10 的 go test 计数按真实标记统计', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 4, failed: 0 })
  const failing = `${LOG}\n--- FAIL: TestEffectTokenRemovalIsBlocked (0.02s)`
  assert.deepEqual(parseGoTestOutput(failing), { passed: 4, failed: 1 })
})

test('S10 的 go list 解析对非数组输出保持空集', () => {
  assert.deepEqual(parseGoListFiles('[ledger.go ledger_identity.go]'), ['ledger.go', 'ledger_identity.go'])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('nope'), [])
})

test('S10 的完整证据可以通过校验', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('S10 的证据校验拒绝缺失源文件', () => {
  const problems = validateEvidence(evidence({ goFiles: ['ledger.go', 'effect_token.go'] }))
  assert.equal(problems.length, 1)
  assert.match(problems[0], /default build is missing ledger_identity\.go/)
})

test('S10 的证据校验拒绝默认构建混入测试文件', () => {
  const problems = validateEvidence(evidence({ goFiles: ['ledger.go', 'ledger_identity.go', 'effect_token.go', 'ledger_identity_test.go'] }))
  assert.equal(problems.length, 1)
  assert.match(problems[0], /default build leaked test files/)
})

test('S10 的证据校验拒绝缺失测试文件', () => {
  const problems = validateEvidence(evidence({ testFiles: ['ledger_test.go'] }))
  assert.equal(problems.length, 1)
  assert.match(problems[0], /test build is missing ledger_identity_test\.go/)
})

test('S10 的证据校验拒绝重复或缺失的证据行', () => {
  const rows = [...EVIDENCE_ROWS]
  rows[3] = EVIDENCE_ROWS[2]
  const problems = validateEvidence(evidence({ rows }))
  assert.equal(problems.length, 2)
  assert.ok(problems.some((problem) => problem.includes('must print exactly once (2)')))
  assert.ok(problems.some((problem) => problem.includes('must print exactly once (0)')))
})

test('S10 的证据校验拒绝行数与用例数不符', () => {
  const problems = validateEvidence(evidence({ rows: EVIDENCE_ROWS.slice(0, 9) }))
  assert.ok(problems.length >= 2)
  assert.ok(problems.some((problem) => problem.includes('evidence rows=9 want 10')))
})

test('S10 的证据校验拒绝未加 race 的跑法', () => {
  const problems = validateEvidence(evidence({ race: false }))
  assert.deepEqual(problems, ['go test must run with -race'])
})

test('S10 的证据校验拒绝非零退出与失败用例', () => {
  const exit = validateEvidence(evidence({ exitCode: 1 }))
  assert.deepEqual(exit, ['go test exit=1 want 0'])
  const failed = validateEvidence(evidence({ failed: 1 }))
  assert.deepEqual(failed, ['failed=1 want 0'])
  const short = validateEvidence(evidence({ passed: 9 }))
  assert.deepEqual(short, ['passed=9 want 10'])
})

test('S10 的证据渲染逐行可核', () => {
  const rendered = renderEvidence(evidence())
  assert.match(rendered, /invocation=ledger-identity package=\.\/internal\/devicemesh\/executionjournal/)
  assert.match(rendered, /rows=10 guards=22/)
  for (const row of EVIDENCE_ROWS) assert.ok(rendered.includes(`m04-s10 ${row}`), `missing ${row}`)
  assert.match(rendered, /passed=10 failed=0 exit=0 race=true/)
})

test('S10 的证据渲染在缺字段时不虚构数字', () => {
  const rendered = renderEvidence({})
  assert.match(rendered, /passed=0 failed=0 exit=1 race=false/)
  assert.match(rendered, /files=0 tests=0 sources=0\/3/)
})
