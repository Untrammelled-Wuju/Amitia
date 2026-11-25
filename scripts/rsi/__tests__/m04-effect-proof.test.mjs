import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  ASSERT_GUARDS,
  CLAIM_GUARDS,
  EVIDENCE_ROWS,
  EXIT_EFFECT_PROOF,
  EXPECTED_PASSED,
  INSTALL_GUARDS,
  KIND_GUARDS,
  PACKAGE_PATH,
  PROOF_GUARDS,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  SCHEMA_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseProofRows,
  renderEvidence,
  validateAssertSource,
  validateClaimSource,
  validateEvidence,
  validateInspectReadOnly,
  validateInstallSource,
  validateKindSource,
  validateProofSource,
  validateSchemaSource
} from '../m04-s11-effect-proof.mjs'

const LOG = [
  '=== RUN   TestJournalProofMissingProofBlocksClaim',
  `    proof_test.go:64: m04-s11 ${EVIDENCE_ROWS[0]}`,
  `    proof_test.go:76: m04-s11 ${EVIDENCE_ROWS[1]}`,
  '--- PASS: TestJournalProofMissingProofBlocksClaim (0.03s)',
  `    proof_test.go:92: m04-s11 ${EVIDENCE_ROWS[2]}`,
  `    proof_test.go:101: m04-s11 ${EVIDENCE_ROWS[3]}`,
  `    proof_test.go:123: m04-s11 ${EVIDENCE_ROWS[4]}`,
  `    proof_test.go:132: m04-s11 ${EVIDENCE_ROWS[5]}`,
  `    proof_test.go:154: m04-s11 ${EVIDENCE_ROWS[6]}`,
  `    proof_test.go:166: m04-s11 ${EVIDENCE_ROWS[7]}`,
  `    proof_test.go:210: m04-s11 ${EVIDENCE_ROWS[8]}`,
  `    proof_test.go:223: m04-s11 ${EVIDENCE_ROWS[9]}`,
  `    proof_test.go:242: m04-s11 ${EVIDENCE_ROWS[10]}`,
  `    proof_test.go:254: m04-s11 ${EVIDENCE_ROWS[11]}`,
  `    proof_test.go:266: m04-s11 ${EVIDENCE_ROWS[12]}`,
  `    proof_test.go:287: m04-s11 ${EVIDENCE_ROWS[13]}`,
  `    proof_test.go:306: m04-s11 ${EVIDENCE_ROWS[14]}`,
  `    proof_test.go:320: m04-s11 ${EVIDENCE_ROWS[15]}`,
  `    proof_test.go:338: m04-s11 ${EVIDENCE_ROWS[16]}`,
  `    proof_test.go:360: m04-s11 ${EVIDENCE_ROWS[17]}`,
  `    proof_test.go:371: m04-s11 ${EVIDENCE_ROWS[18]}`,
  `    proof_test.go:395: m04-s11 ${EVIDENCE_ROWS[19]}`,
  `    proof_test.go:434: m04-s11 ${EVIDENCE_ROWS[20]}`,
  `    proof_test.go:449: m04-s11 ${EVIDENCE_ROWS[21]}`,
  ...Array.from({ length: EXPECTED_PASSED - 1 }, (_, index) => `--- PASS: TestJournalProofExtra${index} (0.01s)`),
  'PASS',
  'ok  \tgithub.com/u-ai/backend/internal/devicemesh/executionjournal\t2.048s'
].join('\n')

const SOURCE = [...PROOF_GUARDS, ...KIND_GUARDS, ...CLAIM_GUARDS, ...SCHEMA_GUARDS, ...INSTALL_GUARDS].join('\n')
const TEST_SOURCE = ASSERT_GUARDS.join('\n')

function evidence(overrides = {}) {
  return {
    goFiles: [
      'effect_token.go',
      'inspection.go',
      'ledger.go',
      'proof.go',
      'readiness.go',
      'reconciliation.go',
      'reconciling_state.go',
      'run_guard.go',
      'store.go',
      'task_inspection.go'
    ],
    testFiles: ['proof_test.go'],
    rows: [...EVIDENCE_ROWS],
    proofSource: SOURCE,
    ledgerSource: INSTALL_GUARDS.join('\n'),
    testSource: TEST_SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('S11 门禁常量已冻结', () => {
  assert.equal(EXIT_EFFECT_PROOF, 51)
  assert.equal(PACKAGE_PATH, './internal/devicemesh/executionjournal')
  assert.equal(RUN_PATTERN, 'TestJournalProof')
  assert.equal(EXPECTED_PASSED, 11)
  assert.deepEqual(REQUIRED_SOURCES, ['ledger.go', 'proof.go'])
  assert.deepEqual(REQUIRED_TESTS, ['proof_test.go'])
  assert.equal(EVIDENCE_ROWS.length, 22)
  assert.equal(new Set(EVIDENCE_ROWS).size, EVIDENCE_ROWS.length)
})

test('S11 证据行覆盖三类效果面与全部拒绝原因', () => {
  const joined = EVIDENCE_ROWS.join('\n')
  for (const kind of ['message', 'file_write', 'device_control']) {
    assert.match(joined, new RegExp(`kind=${kind}`))
  }
  for (const reason of ['missing_effect_proof', 'unknown_effect_surface', 'proof_payload_mismatch', 'not_settled', 'claimable']) {
    assert.match(joined, new RegExp(`reason=${reason}`))
  }
  assert.match(joined, /orphan=rejected/)
  assert.match(joined, /update=blocked delete=blocked/)
  assert.match(joined, /restart=preserved/)
})

test('S11 证据行逐条只出现一次', () => {
  assert.deepEqual(parseProofRows(LOG), [...EVIDENCE_ROWS])
  assert.equal(parseProofRows(LOG).filter((row) => row.startsWith('m04-s11 ')).length, 0)
})

test('S11 解析器只认 m04-s11 前缀', () => {
  const mixed = ['m04-s10 claimable=true', 'm04-s11 a=1', 'prefix m04-s11 b=2', 'm04-s11x c=3'].join('\n')
  assert.deepEqual(parseProofRows(mixed), ['a=1', 'b=2'])
  assert.deepEqual(parseGoTestOutput(LOG), { passed: EXPECTED_PASSED, failed: 0 })
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
})

test('S11 三类效果面的必需字段互不相同', () => {
  const message = KIND_GUARDS.find((guard) => guard.includes('case ProofKindMessage'))
  const file = KIND_GUARDS.find((guard) => guard.includes('case ProofKindFileWrite'))
  const device = KIND_GUARDS.find((guard) => guard.includes('case ProofKindDeviceControl'))
  assert.ok(message && file && device)
  assert.ok(KIND_GUARDS.includes('missing = append(missing, "bytes")'))
  assert.ok(KIND_GUARDS.includes('func (p EffectProof) missingFields() []string {'))
})

test('S11 申报资格必须先过定义面再谈证明', () => {
  const order = [
    'if !defined {',
    'claim.Reason = ClaimReasonSurfaceUnknown',
    'if entry.State != EffectCommitted {',
    'claim.Reason = ClaimReasonNotSettled',
    'claim.Reason = ClaimReasonMissingProof',
    'if record.PayloadDigest != entry.PayloadHash {',
    'claim.Claimable = true'
  ]
  let cursor = -1
  for (const guard of order) {
    const index = CLAIM_GUARDS.indexOf(guard)
    assert.ok(index > cursor, `guard out of order: ${guard}`)
    cursor = index
  }
})

test('S11 真实源码通过全部守卫', () => {
  const real = evidence()
  assert.deepEqual(validateProofSource(real.proofSource), [])
  assert.deepEqual(validateKindSource(real.proofSource), [])
  assert.deepEqual(validateClaimSource(real.proofSource), [])
  assert.deepEqual(validateSchemaSource(real.proofSource), [])
  assert.deepEqual(validateInstallSource(real.ledgerSource), [])
  assert.deepEqual(validateAssertSource(real.testSource), [])
})

test('S11 缺任一守卫都会报出具体条目', () => {
  for (const guard of PROOF_GUARDS) {
    const source = PROOF_GUARDS.filter((entry) => entry !== guard).join('\n')
    assert.deepEqual(validateProofSource(source), [`effect proof source is missing ${guard}`])
  }
  for (const guard of ASSERT_GUARDS) {
    const source = ASSERT_GUARDS.filter((entry) => entry !== guard).join('\n')
    assert.deepEqual(validateAssertSource(source), [`effect proof test is missing ${guard}`])
  }
})

test('S11 只读查询面不得写库', () => {
  const readOnly = `func (l *InvocationLedger) InspectProofs(ctx context.Context, scope LedgerScope, idempotencyKey string) ([]EffectProofRecord, error) {\n\trows, err := l.db.QueryContext(ctx, \`SELECT 1\`)\n\treturn nil, err\n}\n\nfunc next() {}\n`
  assert.deepEqual(validateInspectReadOnly(readOnly), [])
  assert.deepEqual(validateInspectReadOnly(readOnly.replace('QueryContext', 'ExecContext(`DELETE FROM kernel_invocation_effect_proof`)')), [
    'proof inspection must stay read-only but contains DELETE FROM'
  ])
  assert.deepEqual(validateInspectReadOnly('package x'), ['proof inspection function not found'])
})

test('S11 证据校验拦截缺文件、缺行与未开 race', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
  assert.match(validateEvidence(evidence({ goFiles: ['ledger.go', 'proof_test.go'] })).join('\n'), /default build is missing proof.go/)
  assert.match(validateEvidence(evidence({ goFiles: ['ledger.go', 'proof.go', 'proof_test.go'] })).join('\n'), /default build leaked test files/)
  assert.match(validateEvidence(evidence({ testFiles: [] })).join('\n'), /test build is missing proof_test.go/)
  assert.match(validateEvidence(evidence({ rows: EVIDENCE_ROWS.slice(1) })).join('\n'), /evidence rows=21 want 22/)
  assert.match(validateEvidence(evidence({ rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] })).join('\n'), /must print exactly once \(2\)/)
  assert.match(validateEvidence(evidence({ passed: 10 })).join('\n'), /passed=10 want 11/)
  assert.match(validateEvidence(evidence({ failed: 1 })).join('\n'), /failed=1 want 0/)
  assert.match(validateEvidence(evidence({ exitCode: 51 })).join('\n'), /go test exit=51 want 0/)
  assert.match(validateEvidence(evidence({ race: false })).join('\n'), /must run with -race/)
})

test('S11 渲染结果可直接作为门禁日志', () => {
  const rendered = renderEvidence(evidence())
  assert.match(rendered, /^invocation=effect-proof package=\.\/internal\/devicemesh\/executionjournal run=TestJournalProof rows=22 guards=36$/m)
  assert.match(rendered, /sources=2\/2 testExcluded=true/)
  assert.match(rendered, /passed=11 failed=0 exit=0 race=true/)
  assert.equal(rendered.trimEnd().split('\n').length, 2 + EVIDENCE_ROWS.length + 1)
})

test('S11 的用例名不与 S02/S04/S10 的前缀门禁撞车', () => {
  const names = [
    'TestJournalProofMissingProofBlocksClaim',
    'TestJournalProofMessageSend',
    'TestJournalProofFileWrite',
    'TestJournalProofDeviceControl',
    'TestJournalProofIncompleteRejected',
    'TestJournalProofSurfaceGuards',
    'TestJournalProofPayloadMismatch',
    'TestJournalProofOrphanRejected',
    'TestJournalProofClaimRequiresSettledLedger',
    'TestJournalProofImmutableRows',
    'TestJournalProofSurvivesRestart'
  ]
  assert.equal(names.length, EXPECTED_PASSED)
  for (const name of names) {
    assert.ok(!/^TestLedger/.test(name), `collides with S02: ${name}`)
    assert.ok(!/^TestEffect/.test(name), `collides with S04: ${name}`)
    assert.ok(!/^TestJournalIdentity|^TestJournalToken/.test(name), `collides with S10: ${name}`)
    assert.match(name, /^TestJournalProof/)
  }
})
