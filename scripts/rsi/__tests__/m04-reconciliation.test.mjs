import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EXIT_RECONCILIATION,
  EXPECTED_PASSED,
  LEDGER_GUARDS,
  PACKAGE_PATH,
  RECONCILE_CASES,
  RECONCILE_GUARDS,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  parseGoListFiles,
  parseGoTestOutput,
  parseReconcileRows,
  renderEvidence,
  rowPattern,
  validateEvidence,
  validateLedgerSource,
  validateReconcileSource
} from '../m04-reconciliation.mjs'

const SOURCE = [...RECONCILE_GUARDS, ...LEDGER_GUARDS].join('\n')

const LOG = `=== RUN   TestReconcileLostAckStopsResend
m04-s05 reconcile ack-lost dispatches=1 state=needs_reconciliation retryBlocked=true
=== RUN   TestReconcileSurvivesRestart
m04-s05 reconcile restart visible=true state=needs_reconciliation attempts=1 error=true
=== RUN   TestReconcileRecoversInterruptedAttempts
m04-s05 reconcile crash-recovery recovered=1 state=needs_reconciliation
=== RUN   TestReconcileDeliveredVerdictStopsResend
m04-s05 reconcile resolved-delivered resolved=true retryBlocked=true dispatches=1
=== RUN   TestReconcileNotSentVerdictAllowsRetry
m04-s05 reconcile resolved-not-sent retryAllowed=true attempts=2 state=delivered
=== RUN   TestReconcileRejectsSecondVerdict
m04-s05 reconcile double-resolve rejected=true state=delivered
=== RUN   TestReconcileGuardsVerdictAndMissingCase
m04-s05 reconcile resolve-guard inflightBlocked=true missingReported=true
=== RUN   TestReconcileDefiniteFailureKeepsRetryOpen
m04-s05 reconcile definite-failure state=not_delivered retryAllowed=true dispatches=2
=== RUN   TestReconcileListsOpenCases
m04-s05 reconcile open-list open=2 scoped=1
=== RUN   TestReconcileConcurrentDispatchSendsOnce
m04-s05 reconcile concurrent workers=8 dispatches=1 leaders=1 blocked=7
=== RUN   TestReconcileDoesNotTouchKeyedLedger
m04-s05 reconcile isolation ledger_rows=0 effect_rows=0 reconciliation_rows=1
=== RUN   TestReconcileKeepsErrorTextVerbatim
m04-s05 reconcile error-verbatim bytes=137 preserved=true
=== RUN   TestReconcileRejectsInvalidRequests
m04-s05 reconcile invalid-requests rejected=9 dispatched=0 verdicts=3
PASS
ok  	github.com/u-ai/backend/internal/devicemesh/executionjournal	2.015s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['reconciliation.go', 'ledger.go'],
    testFiles: ['reconciliation_test.go'],
    rows: parseReconcileRows(LOG),
    source: SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the reconciliation surface', () => {
  assert.equal(EXIT_RECONCILIATION, 45)
  assert.equal(PACKAGE_PATH, './internal/devicemesh/executionjournal')
  assert.equal(RUN_PATTERN, 'TestReconcile')
  assert.equal(EXPECTED_PASSED, 13)
  assert.deepEqual(REQUIRED_SOURCES, ['reconciliation.go', 'ledger.go'])
  assert.deepEqual(REQUIRED_TESTS, ['reconciliation_test.go'])
  assert.equal(RECONCILE_CASES.length, 13)
  assert.equal(RECONCILE_GUARDS.length, 11)
  assert.equal(LEDGER_GUARDS.length, 1)
})

test('every case has a distinct evidence pattern', () => {
  const sources = RECONCILE_CASES.map((name) => rowPattern(name).source)
  assert.equal(new Set(sources).size, RECONCILE_CASES.length)
  for (const pattern of RECONCILE_CASES.map(rowPattern)) assert.ok(pattern instanceof RegExp)
})

test('unknown case names fall back to the invalid-request pattern', () => {
  assert.equal(rowPattern('whatever').source, rowPattern('invalid-requests').source)
})

test('go list output is parsed into sorted file lists', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('a.go'), [])
  assert.deepEqual(parseGoListFiles(''), [])
})

test('go test output counts passes and failures', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n--- FAIL: C\n'), { passed: 2, failed: 1 })
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 0, failed: 0 })
  assert.deepEqual(parseGoTestOutput(''), { passed: 0, failed: 0 })
})

test('all thirteen evidence rows are parsed', () => {
  const rows = parseReconcileRows(LOG)
  assert.deepEqual(rows['ack-lost'], [1, 'needs_reconciliation', 'true'])
  assert.deepEqual(rows.restart, ['true', 'needs_reconciliation', 1, 'true'])
  assert.deepEqual(rows['resolve-guard'], ['true', 'true'])
  assert.deepEqual(rows['definite-failure'], ['not_delivered', 'true', 2])
  assert.deepEqual(rows.concurrent, [8, 1, 1, 7])
  assert.deepEqual(rows.isolation, [0, 0, 1])
  assert.deepEqual(rows['invalid-requests'], [9, 0, 3])
  assert.equal(Object.keys(rows).length, RECONCILE_CASES.length)
})

test('a clean evidence bundle has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('missing evidence rows are reported by name', () => {
  const problems = validateEvidence(evidence({ rows: parseReconcileRows(''), log: '' }))
  for (const name of RECONCILE_CASES) assert.ok(problems.includes(`evidence row ${name} is missing`), name)
})

test('a duplicated evidence row is rejected', () => {
  const problems = validateEvidence(
    evidence({ log: `${LOG}\nm04-s05 reconcile ack-lost dispatches=1 state=needs_reconciliation retryBlocked=true\n` })
  )
  assert.ok(problems.includes('evidence row ack-lost must print exactly once'))
})

test('a lost ack that was resent is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['ack-lost'] = [2, 'attempting', 'false']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row ack-lost column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row ack-lost column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row ack-lost column 2')))
})

test('a restart that lost the state is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows.restart = ['false', 'attempting', 1, 'false']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row restart column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row restart column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row restart column 3')))
})

test('an interrupted attempt left in attempting is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['crash-recovery'] = [0, 'attempting']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row crash-recovery column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row crash-recovery column 1')))
})

test('a delivered verdict that allowed resend is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['resolved-delivered'] = ['false', 'false', 2]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row resolved-delivered column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row resolved-delivered column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row resolved-delivered column 2')))
})

test('a not-sent verdict that blocked retry is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['resolved-not-sent'] = ['false', 1, 'needs_reconciliation']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row resolved-not-sent column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row resolved-not-sent column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row resolved-not-sent column 2')))
})

test('a second verdict that was accepted is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['double-resolve'] = ['false', 'needs_reconciliation']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row double-resolve column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row double-resolve column 1')))
})

test('a verdict allowed while inflight is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['resolve-guard'] = ['false', 'false']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row resolve-guard column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row resolve-guard column 1')))
})

test('a definite failure kept out of retry is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['definite-failure'] = ['delivered', 'false', 1]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row definite-failure column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row definite-failure column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row definite-failure column 2')))
})

test('an open list that leaked other scopes is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['open-list'] = [3, 3]
  assert.ok(validateEvidence(evidence({ rows })).some((problem) => problem.includes('evidence row open-list column 1')))
})

test('isolation that touched the keyed ledger is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows.isolation = [1, 1, 1]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row isolation column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row isolation column 1')))
})

test('an error text that was replaced is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['error-verbatim'] = [42, 'false']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row error-verbatim column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row error-verbatim column 1')))
})

test('an invalid request that dispatched is rejected', () => {
  const rows = parseReconcileRows(LOG)
  rows['invalid-requests'] = [8, 1, 2]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row invalid-requests column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row invalid-requests column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row invalid-requests column 2')))
})

test('a run without race detection is rejected', () => {
  assert.ok(validateEvidence(evidence({ race: false })).includes('go test must run with -race'))
})

test('a failing run is rejected', () => {
  const problems = validateEvidence(evidence({ failed: 1, exitCode: 1 }))
  assert.ok(problems.includes('failed=1 want 0'))
  assert.ok(problems.includes('go test exit=1 want 0'))
})

test('a short run is rejected', () => {
  assert.ok(validateEvidence(evidence({ passed: 12 })).includes('passed=12 want 13'))
})

test('a missing source file is rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['ledger.go'] }))
  assert.ok(problems.includes('default build is missing reconciliation.go'))
})

test('a leaked test file in the default build is rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['reconciliation.go', 'reconciliation_test.go'] }))
  assert.ok(problems.includes('default build leaked test files'))
})

test('a missing test file is rejected', () => {
  const problems = validateEvidence(evidence({ testFiles: [] }))
  assert.ok(problems.includes('test build is missing reconciliation_test.go'))
})

test('reconciliation guards are validated one by one', () => {
  assert.deepEqual(validateReconcileSource(SOURCE), [])
  for (const guard of RECONCILE_GUARDS) {
    const problems = validateReconcileSource(SOURCE.split(guard).join(''))
    assert.ok(problems.includes(`reconciliation source is missing ${guard}`), guard)
  }
})

test('ledger guards are validated one by one', () => {
  assert.deepEqual(validateLedgerSource(SOURCE), [])
  for (const guard of LEDGER_GUARDS) {
    const problems = validateLedgerSource(SOURCE.split(guard).join(''))
    assert.ok(problems.includes(`ledger source is missing ${guard}`), guard)
  }
})

test('an empty source reports every guard', () => {
  assert.equal(validateReconcileSource('').length, RECONCILE_GUARDS.length)
  assert.equal(validateLedgerSource('').length, LEDGER_GUARDS.length)
})

test('rendered evidence carries every case line plus the summary', () => {
  const lines = renderEvidence(evidence()).trimEnd().split('\n')
  assert.equal(lines.length, 17)
  assert.ok(lines[0].startsWith('invocation=reconciliation package='))
  assert.ok(lines[1].startsWith('files=2 tests=1 sources=2/2 testExcluded=true'))
  assert.equal(lines[2], 'reconciliation guards frozen=true ledger guards frozen=true')
  for (const name of RECONCILE_CASES) assert.ok(lines.some((line) => line.includes(`reconcile ${name} `)), name)
  assert.ok(lines.at(-1).startsWith('passed=13 failed=0 exit=0 race=true'))
})

test('rendered evidence marks missing rows with fallbacks', () => {
  const lines = renderEvidence(evidence({ rows: {}, source: '', passed: 0, failed: 0, exitCode: 1, race: false })).trimEnd().split('\n')
  assert.ok(lines.includes('reconciliation guards frozen=false ledger guards frozen=false'))
  assert.ok(lines.some((line) => line.includes('ack-lost dispatches=0 state= retryBlocked=false')))
  assert.ok(lines.some((line) => line.includes('concurrent workers=0 dispatches=0 leaders=0 blocked=0')))
  assert.ok(lines.some((line) => line.endsWith('passed=0 failed=0 exit=1 race=false')))
})
