import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EFFECT_STATES,
  EXPECTED_PASSED,
  EXIT_LEDGER,
  LEDGER_CASES,
  LEDGER_CONSTRAINTS,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  parseGoListFiles,
  parseGoTestOutput,
  parseLedgerRows,
  renderEvidence,
  rowPattern,
  validateEvidence,
  validateLedgerSource
} from '../m04-ledger.mjs'

const SOURCE = `const LedgerSchema = \`CREATE TABLE IF NOT EXISTS kernel_invocation_ledger (
	scope_key TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	effect_state TEXT NOT NULL CHECK(effect_state IN ('pending','committed','rejected','unknown')),
	PRIMARY KEY(scope_key,idempotency_key))\`
const EffectPending EffectState = "pending"
const EffectCommitted EffectState = "committed"
const EffectRejected EffectState = "rejected"
const EffectUnknown EffectState = "unknown"
INSERT INTO kernel_invocation_ledger(...) VALUES(...) ON CONFLICT(scope_key,idempotency_key) DO NOTHING
`

const LOG = `=== RUN   TestLedgerRejectsInvalidKeys
m04-s02 ledger invalid-keys rejected=7 executed=0
=== RUN   TestLedgerFirstCommitRunsOnceAndReplayReusesResult
m04-s02 ledger replay-runs-once executions=1 state=committed result={"value":"once"}
=== RUN   TestLedgerDefiniteFailureReleasesReservation
m04-s02 ledger retry-after-failure attempts=2 state=committed
=== RUN   TestLedgerConcurrentSameKeyRunsOnce
m04-s02 ledger concurrent workers=16 executions=1 leaders=1
=== RUN   TestLedgerSameKeyDifferentScopeIsIndependent
m04-s02 ledger cross-scope executions=2 rows=2
=== RUN   TestLedgerPayloadChangeRejected
m04-s02 ledger payload-conflict executions=1 conflict=执行账本：同一幂等键的请求内容已变化，拒绝重复执行
=== RUN   TestLedgerBindingRevisionFrozen
m04-s02 ledger binding-frozen revision=rev-7 executions=1
=== RUN   TestLedgerUnknownOutcomeBlocksRetry
m04-s02 ledger unknown-blocks-retry executions=1 state=unknown retrySafe=false
=== RUN   TestLedgerCommittedSurvivesRestart
m04-s02 ledger restart-replay executions=1 state=committed
PASS
ok  	github.com/u-ai/backend/internal/devicemesh/executionjournal	2.219s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['ledger.go', 'store.go'],
    testFiles: ['ledger_test.go', 'store_test.go'],
    rows: parseLedgerRows(LOG),
    source: SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

function withLog(transform) {
  return evidence({ log: transform(LOG), rows: parseLedgerRows(transform(LOG)) })
}

function withRows(patch, log = LOG) {
  return evidence({ log, rows: { ...parseLedgerRows(log), ...patch } })
}

test('constants pin the ledger gate surface', () => {
  assert.equal(EXIT_LEDGER, 42)
  assert.equal(PACKAGE_PATH, './internal/devicemesh/executionjournal')
  assert.equal(RUN_PATTERN, 'TestLedger')
  assert.equal(EXPECTED_PASSED, 9)
  assert.deepEqual(REQUIRED_SOURCES, ['ledger.go'])
  assert.deepEqual(REQUIRED_TESTS, ['ledger_test.go'])
  assert.deepEqual(EFFECT_STATES, ['pending', 'committed', 'rejected', 'unknown'])
  assert.deepEqual(LEDGER_CASES, [
    'invalid-keys',
    'replay-runs-once',
    'retry-after-failure',
    'concurrent',
    'cross-scope',
    'payload-conflict',
    'binding-frozen',
    'unknown-blocks-retry',
    'restart-replay'
  ])
  assert.equal(LEDGER_CONSTRAINTS.length, 3)
})

test('go list output is parsed into sorted file lists', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('a.go'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('[ledger_test.go ledger.go]'), ['ledger.go', 'ledger_test.go'])
})

test('go test output counts passes and failures', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 0, failed: 0 })
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n--- FAIL: C\n'), { passed: 2, failed: 1 })
  assert.deepEqual(parseGoTestOutput(''), { passed: 0, failed: 0 })
})

test('every ledger evidence row is parsed with its values', () => {
  const rows = parseLedgerRows(LOG)
  assert.deepEqual(rows['invalid-keys'], [7, 0])
  assert.deepEqual(rows['replay-runs-once'], [1, 'committed', '{"value":"once"}'])
  assert.deepEqual(rows['retry-after-failure'], [2, 'committed'])
  assert.deepEqual(rows.concurrent, [16, 1, 1])
  assert.deepEqual(rows['cross-scope'], [2, 2])
  assert.deepEqual(rows['payload-conflict'], [1, '执行账本：同一幂等键的请求内容已变化，拒绝重复执行'])
  assert.deepEqual(rows['binding-frozen'], ['rev-7', 1])
  assert.deepEqual(rows['unknown-blocks-retry'], [1, 'unknown', 'false'])
  assert.deepEqual(rows['restart-replay'], [1, 'committed'])
})

test('each ledger case has a distinct row pattern', () => {
  const sources = LEDGER_CASES.map((name) => rowPattern(name).source)
  assert.equal(new Set(sources).size, LEDGER_CASES.length)
  assert.equal(rowPattern('restart-replay'), rowPattern('unknown-case'))
  assert.match('m04-s02 ledger concurrent workers=16 executions=1 leaders=1', rowPattern('concurrent'))
  assert.doesNotMatch('m04-s02 ledger concurrent workers=16 executions=1 leaders=1', rowPattern('cross-scope'))
})

test('a complete ledger run validates clean', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
  assert.deepEqual(validateLedgerSource(SOURCE), [])
})

test('the ledger package must keep its production source file', () => {
  const problems = validateEvidence(evidence({ goFiles: ['store.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing ledger.go')))
})

test('a default build that leaks test files is rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['ledger.go', 'ledger_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('leaked test files')))
})

test('the ledger test file must exist', () => {
  const problems = validateEvidence(evidence({ testFiles: ['store_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing ledger_test.go')))
})

test('a duplicated evidence row is rejected', () => {
  const problems = validateEvidence(withLog((log) => `${log}m04-s02 ledger restart-replay executions=1 state=committed\n`))
  assert.deepEqual(problems, ['evidence row restart-replay must print exactly once'])
})

test('a missing evidence row is rejected', () => {
  const problems = validateEvidence(withLog((log) => log.replace(/m04-s02 ledger binding-frozen[^\n]*\n/, '')))
  assert.ok(problems.some((problem) => problem === 'evidence row binding-frozen is missing'))
  assert.ok(problems.some((problem) => problem === 'evidence row binding-frozen must print exactly once'))
})

test('invalid key coverage must stay at seven rejected and zero executed', () => {
  assert.ok(validateEvidence(withRows({ 'invalid-keys': [6, 0] })).some((problem) => problem.includes('rejected=6 want 7')))
  assert.ok(validateEvidence(withRows({ 'invalid-keys': [7, 1] })).some((problem) => problem.includes('executed=1 want 0')))
})

test('a replayed key must not execute twice', () => {
  assert.ok(validateEvidence(withRows({ 'replay-runs-once': [2, 'committed', '{"value":"once"}'] })).some((problem) => problem.includes('executions=2 want 1')))
})

test('a replay must return the recorded result', () => {
  const problems = validateEvidence(withRows({ 'replay-runs-once': [1, 'committed', '{"value":"twice"}'] }))
  assert.ok(problems.some((problem) => problem.includes('is not the recorded result')))
})

test('a definite failure must release the reservation for a retry', () => {
  assert.ok(validateEvidence(withRows({ 'retry-after-failure': [1, 'committed'] })).some((problem) => problem.includes('attempts=1 want 2')))
  assert.ok(validateEvidence(withRows({ 'retry-after-failure': [2, 'unknown'] })).some((problem) => problem.includes('retry state=unknown want committed')))
})

test('concurrent commits must have exactly one executing caller', () => {
  assert.ok(validateEvidence(withRows({ concurrent: [8, 1, 1] })).some((problem) => problem.includes('workers=8 want 16')))
  assert.ok(validateEvidence(withRows({ concurrent: [16, 2, 2] })).some((problem) => problem.includes('executions=2 want 1')))
  assert.ok(validateEvidence(withRows({ concurrent: [16, 1, 2] })).some((problem) => problem.includes('leaders=2 want 1')))
})

test('the same key in two scopes must stay independent', () => {
  assert.ok(validateEvidence(withRows({ 'cross-scope': [2, 1] })).some((problem) => problem.includes('rows=1 want 2')))
  assert.ok(validateEvidence(withRows({ 'cross-scope': [1, 2] })).some((problem) => problem.includes('executions=1 want 2')))
})

test('a changed payload must be rejected without re-executing', () => {
  assert.ok(validateEvidence(withRows({ 'payload-conflict': [1, 'executionjournal: nope'] })).some((problem) => problem.includes('does not name the conflict')))
  assert.ok(validateEvidence(withRows({ 'payload-conflict': [2, '执行账本：请求内容已变化'] })).some((problem) => problem.includes('executions=2 want 1')))
})

test('the binding revision must stay frozen at the first write', () => {
  assert.ok(validateEvidence(withRows({ 'binding-frozen': ['rev-8', 1] })).some((problem) => problem.includes('revision=rev-8 want rev-7')))
  assert.ok(validateEvidence(withRows({ 'binding-frozen': ['rev-7', 2] })).some((problem) => problem.includes('executions=2 want 1')))
})

test('an unknown outcome must block retry and stay unknown', () => {
  assert.ok(validateEvidence(withRows({ 'unknown-blocks-retry': [2, 'unknown', 'false'] })).some((problem) => problem.includes('executions=2 want 1')))
  assert.ok(validateEvidence(withRows({ 'unknown-blocks-retry': [1, 'committed', 'false'] })).some((problem) => problem.includes('state=committed want unknown')))
  assert.ok(validateEvidence(withRows({ 'unknown-blocks-retry': [1, 'unknown', 'true'] })).some((problem) => problem.includes('must not be retry safe')))
})

test('a committed record must survive a restart', () => {
  assert.ok(validateEvidence(withRows({ 'restart-replay': [1, 'pending'] })).some((problem) => problem.includes('state=pending want committed')))
  assert.ok(validateEvidence(withRows({ 'restart-replay': [2, 'committed'] })).some((problem) => problem.includes('executions=2 want 1')))
})

test('the run itself must be green, complete and race checked', () => {
  assert.ok(validateEvidence(evidence({ passed: 8 })).some((problem) => problem.includes('passed=8 want 9')))
  assert.ok(validateEvidence(evidence({ failed: 1 })).some((problem) => problem.includes('failed=1 want 0')))
  assert.ok(validateEvidence(evidence({ exitCode: 1 })).some((problem) => problem.includes('exit=1 want 0')))
  assert.ok(validateEvidence(evidence({ race: false })).some((problem) => problem.includes('must run with -race')))
})

test('the ledger schema must stay unique on scope plus key', () => {
  const withoutPrimary = SOURCE.replace('PRIMARY KEY(scope_key,idempotency_key)', '')
  assert.ok(validateLedgerSource(withoutPrimary).some((problem) => problem.includes('PRIMARY KEY(scope_key,idempotency_key)')))
  const withoutConflict = SOURCE.replace('ON CONFLICT(scope_key,idempotency_key) DO NOTHING', '')
  assert.ok(validateLedgerSource(withoutConflict).some((problem) => problem.includes('ON CONFLICT(scope_key,idempotency_key) DO NOTHING')))
  const withoutCheck = SOURCE.replace("CHECK(effect_state IN ('pending','committed','rejected','unknown'))", '')
  assert.ok(validateLedgerSource(withoutCheck).some((problem) => problem.includes('CHECK(effect_state')))
})

test('every effect state must keep an exported constant', () => {
  const trimmed = SOURCE.replace(/const EffectUnknown[^\n]*\n/, '')
  assert.deepEqual(validateLedgerSource(trimmed), ['effect state unknown has no exported constant'])
})

test('the rendered evidence carries every case and the final counts', () => {
  const lines = renderEvidence(evidence()).trimEnd().split('\n')
  assert.equal(lines.length, 13)
  assert.match(lines[0], /^invocation=ledger package=\.\/internal\/devicemesh\/executionjournal run=TestLedger states=pending,committed,rejected,unknown cases=9$/)
  assert.match(lines[2], /ledger schema constraints=3 frozen=true$/)
  assert.equal(lines[11], 'm04-s02 ledger restart-replay executions=1 state=committed')
  assert.equal(lines[12], 'passed=9 failed=0 exit=0 race=true')
})

test('the rendered evidence degrades to defaults when rows are absent', () => {
  const lines = renderEvidence({ rows: {}, goFiles: [], testFiles: [] }).trimEnd().split('\n')
  assert.equal(lines[1], 'files=0 tests=0 sources=0/1 testExcluded=false')
  assert.equal(lines[2], 'ledger schema constraints=3 frozen=false')
  assert.equal(lines[3], 'm04-s02 ledger invalid-keys rejected=0 executed=0')
  assert.equal(lines[11], 'm04-s02 ledger restart-replay executions=0 state=')
  assert.equal(lines[12], 'passed=0 failed=0 exit=1 race=false')
})
