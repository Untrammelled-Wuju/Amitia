import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EFFECT_CASES,
  EFFECT_GUARDS,
  EXIT_EFFECT_TOKEN,
  EXPECTED_PASSED,
  LEDGER_GUARDS,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  parseEffectRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  rowPattern,
  validateEffectSource,
  validateEvidence,
  validateLedgerSource
} from '../m04-effect-token.mjs'

const SOURCE = [...EFFECT_GUARDS, ...LEDGER_GUARDS].join('\n')

const LOG = `=== RUN   TestEffectTokenBindsRequest
m04-s04 effect token-binds attempts=2 verified=1 rejected=4 problems=0
=== RUN   TestEffectTokenRequiresMutatingRequest
m04-s04 effect token-mutating write=1 external=1 readRejected=true noneRejected=true
=== RUN   TestEffectTimeoutAfterWriteIsNotReplayed
m04-s04 effect timeout-after-write executions=1 replays=false state=committed result={"value":"landed"}
=== RUN   TestEffectSameKeyReturnsRecordedAcrossProviders
m04-s04 effect same-key executions=1 providers=2 result={"value":"once"}
=== RUN   TestEffectPayloadConflictRejectsDifferentContent
m04-s04 effect payload-conflict executions=1 conflict=true
=== RUN   TestEffectAuthorityPayloadMismatchIsConflict
m04-s04 effect authority-payload-mismatch executions=1 conflict=true
=== RUN   TestEffectAuthorityTokenMustBeIssued
m04-s04 effect authority-token-forged executions=1 rejected=true
=== RUN   TestEffectAuthorityUnavailableBlocksRetry
m04-s04 effect authority-unavailable executions=1 blocked=true cause=true
=== RUN   TestEffectAuthorityAbsentAllowsRetry
m04-s04 effect authority-absent executions=2 retrySafe=true state=committed
=== RUN   TestEffectRetrySafeOnlyWhenAbsentOrRejected
m04-s04 effect retry-safe resolutions=4 allowed=2
=== RUN   TestEffectCancelledBeforeRunIssuesNothing
m04-s04 effect cancelled executions=0 recorded=false err=true
=== RUN   TestEffectRestartKeepsConfirmedResult
m04-s04 effect restart executions=1 state=committed result={"value":"persisted"}
=== RUN   TestEffectConcurrentSingleExecution
m04-s04 effect concurrent workers=8 executions=1 leaders=1
=== RUN   TestEffectReconcileRejectsNilAuthority
m04-s04 effect nil-authority executions=0 rejected=true
PASS
ok  	github.com/u-ai/backend/internal/devicemesh/executionjournal	2.214s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['effect_token.go', 'ledger.go'],
    testFiles: ['effect_token_test.go', 'ledger_test.go'],
    rows: parseEffectRows(LOG),
    source: SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the effect token surface', () => {
  assert.equal(EXIT_EFFECT_TOKEN, 44)
  assert.equal(PACKAGE_PATH, './internal/devicemesh/executionjournal')
  assert.equal(RUN_PATTERN, 'TestEffect')
  assert.equal(EXPECTED_PASSED, 14)
  assert.deepEqual(REQUIRED_SOURCES, ['effect_token.go', 'ledger.go'])
  assert.deepEqual(REQUIRED_TESTS, ['effect_token_test.go'])
  assert.equal(EFFECT_CASES.length, 14)
  assert.equal(EFFECT_GUARDS.length, 11)
  assert.equal(LEDGER_GUARDS.length, 2)
})

test('every case has a distinct evidence pattern', () => {
  const sources = EFFECT_CASES.map((name) => rowPattern(name).source)
  assert.equal(new Set(sources).size, EFFECT_CASES.length)
  for (const pattern of EFFECT_CASES.map(rowPattern)) assert.ok(pattern instanceof RegExp)
})

test('go list output is parsed into sorted file lists', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('a.go'), [])
  assert.deepEqual(parseGoListFiles(''), [])
})

test('go test output counts passes and failures', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 0, failed: 0 })
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n--- FAIL: C\n'), { passed: 2, failed: 1 })
  assert.deepEqual(parseGoTestOutput(''), { passed: 0, failed: 0 })
})

test('all fourteen evidence rows are parsed', () => {
  const rows = parseEffectRows(LOG)
  assert.deepEqual(rows['token-binds'], [2, 1, 4, 0])
  assert.deepEqual(rows['timeout-after-write'], [1, 'false', 'committed', '{"value":"landed"}'])
  assert.deepEqual(rows['same-key'], [1, 2, '{"value":"once"}'])
  assert.deepEqual(rows['authority-unavailable'], [1, 'true', 'true'])
  assert.deepEqual(rows['retry-safe'], [4, 2])
  assert.deepEqual(rows.concurrent, [8, 1, 1])
  assert.equal(Object.keys(rows).length, EFFECT_CASES.length)
})

test('a clean evidence bundle has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('missing evidence rows are reported by name', () => {
  const problems = validateEvidence(evidence({ rows: parseEffectRows(''), log: '' }))
  for (const name of EFFECT_CASES) assert.ok(problems.includes(`evidence row ${name} is missing`), name)
})

test('a duplicated evidence row is rejected', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}\nm04-s04 effect cancelled executions=0 recorded=false err=true\n` }))
  assert.ok(problems.includes('evidence row cancelled must print exactly once'))
})

test('a timeout that replayed the write is rejected', () => {
  const rows = parseEffectRows(LOG)
  rows['timeout-after-write'] = [2, 'true', 'committed', '{"value":"landed"}']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row timeout-after-write column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row timeout-after-write column 1')))
})

test('a same-key retry that executed twice is rejected', () => {
  const rows = parseEffectRows(LOG)
  rows['same-key'] = [2, 1, '{"value":"once"}']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row same-key column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row same-key column 1')))
})

test('an unavailable authority that did not block is rejected', () => {
  const rows = parseEffectRows(LOG)
  rows['authority-unavailable'] = [2, 'false', 'false']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row authority-unavailable column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row authority-unavailable column 1')))
})

test('an absent authority that was not retry safe is rejected', () => {
  const rows = parseEffectRows(LOG)
  rows['authority-absent'] = [2, 'false', 'committed']
  assert.ok(validateEvidence(evidence({ rows })).some((problem) => problem.includes('evidence row authority-absent column 1')))
})

test('a retry that stayed safe for an undetermined outcome is rejected', () => {
  const rows = parseEffectRows(LOG)
  rows['retry-safe'] = [4, 3]
  assert.ok(validateEvidence(evidence({ rows })).some((problem) => problem.includes('evidence row retry-safe column 1')))
})

test('a cancelled request that recorded state is rejected', () => {
  const rows = parseEffectRows(LOG)
  rows.cancelled = [1, 'true', 'false']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row cancelled column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row cancelled column 1')))
})

test('concurrent writers that both executed are rejected', () => {
  const rows = parseEffectRows(LOG)
  rows.concurrent = [8, 2, 2]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row concurrent column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row concurrent column 2')))
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
  assert.ok(validateEvidence(evidence({ passed: 13 })).includes('passed=13 want 14'))
})

test('a missing source file is rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['ledger.go'] }))
  assert.ok(problems.includes('default build is missing effect_token.go'))
})

test('a leaked test file in the default build is rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['effect_token.go', 'effect_token_test.go'] }))
  assert.ok(problems.includes('default build leaked test files'))
})

test('a missing test file is rejected', () => {
  const problems = validateEvidence(evidence({ testFiles: ['ledger_test.go'] }))
  assert.ok(problems.includes('test build is missing effect_token_test.go'))
})

test('effect token guards are validated one by one', () => {
  assert.deepEqual(validateEffectSource(SOURCE), [])
  for (const guard of EFFECT_GUARDS) {
    const problems = validateEffectSource(SOURCE.split(guard).join(''))
    assert.ok(problems.includes(`effect token source is missing ${guard}`), guard)
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
  assert.equal(validateEffectSource('').length, EFFECT_GUARDS.length)
  assert.equal(validateLedgerSource('').length, LEDGER_GUARDS.length)
})

test('rendered evidence carries every case line plus the summary', () => {
  const lines = renderEvidence(evidence()).trimEnd().split('\n')
  assert.equal(lines.length, 18)
  assert.ok(lines[0].startsWith('invocation=effect-token package='))
  assert.ok(lines[1].startsWith('files=2 tests=2 sources=2/2 testExcluded=true'))
  assert.equal(lines[2], 'effect token guards frozen=true ledger guards frozen=true')
  for (const name of EFFECT_CASES) {
    assert.ok(lines.some((line) => line.includes(`effect ${name} `) || line.includes(`effect ${name}=`)), name)
  }
  assert.ok(lines.at(-1).startsWith('passed=14 failed=0 exit=0 race=true'))
})

test('rendered evidence marks missing rows with fallbacks', () => {
  const lines = renderEvidence(evidence({ rows: {}, source: '', passed: 0, failed: 0, exitCode: 1, race: false })).trimEnd().split('\n')
  assert.ok(lines.includes('effect token guards frozen=false ledger guards frozen=false'))
  assert.ok(lines.some((line) => line.includes('timeout-after-write executions=0 replays=true state= result=')))
  assert.ok(lines.some((line) => line.includes('concurrent workers=0 executions=0 leaders=0')))
  assert.ok(lines.some((line) => line.endsWith('passed=0 failed=0 exit=1 race=false')))
})
