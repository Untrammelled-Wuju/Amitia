import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BREAKER_CASES,
  BREAKER_GUARDS,
  EXIT_SCOPED_BREAKER,
  EXPECTED_PASSED,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  SCOPED_BREAKER_GUARDS,
  parseBreakerRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  rowPattern,
  validateBreakerSource,
  validateEvidence,
  validateScopedSource
} from '../m04-scoped-breaker.mjs'

const SOURCE = [...BREAKER_GUARDS, ...SCOPED_BREAKER_GUARDS].join('\n')

const LOG = `=== RUN   TestScopedBreakerRejectsInvalidKeys
m04-s06 breaker invalid-keys invalid=7 accepted=1 trimmed=true registered=0
=== RUN   TestScopedBreakerTripStaysInScope
m04-s06 breaker scope-isolation tripped=open otherScope=closed otherService=closed otherProvider=closed
=== RUN   TestScopedBreakerThresholdBoundary
m04-s06 breaker threshold threshold=3 closedAt=2 openAt=3 openAfterExtra=true failures=4
=== RUN   TestScopedBreakerHalfOpenProbeCap
m04-s06 breaker probe-cap probes=2 denied=1 state=half_open
=== RUN   TestScopedBreakerHalfOpenSuccessCloses
m04-s06 breaker probe-success successes=2 state=closed failures=0 probes=0
=== RUN   TestScopedBreakerHalfOpenFailureReopens
m04-s06 breaker probe-failure reopened=true probes=0 denied=1 state=open
=== RUN   TestScopedBreakerAbortReleasesProbe
m04-s06 breaker probe-abort granted=3 aborted=3 probes=0 state=half_open
=== RUN   TestScopedBreakerConcurrentProbesSingle
m04-s06 breaker concurrent-probe workers=16 granted=1 denied=15 other=0
=== RUN   TestScopedBreakerConcurrentKeysIsolated
m04-s06 breaker concurrent-keys keys=8 open=8 failures=4 untouched=closed
=== RUN   TestScopedBreakerOpenInScopeLists
m04-s06 breaker open-list scope-a=2 scope-b=1 missing=0 order=ascending rejected=true
=== RUN   TestScopedBreakerResetIsLocal
m04-s06 breaker reset-local reset=closed peer=open peerFailures=3
=== RUN   TestScopedBreakerSnapshotOrderStable
m04-s06 breaker snapshot entries=4 sorted=true stable=true open=1
=== RUN   TestScopedBreakerConfigDefaults
m04-s06 breaker config-defaults threshold=5 openAt=5 failures=5
=== RUN   TestScopedBreakerClosedStateDoesNotConsumeProbes
m04-s06 breaker closed-no-probe closedAcquires=5 probes=0 halfOpenStart=0
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/trusted_service	1.662s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['circuit_breaker.go', 'scoped_breaker.go'],
    testFiles: ['circuit_breaker_test.go', 'scoped_breaker_test.go'],
    rows: parseBreakerRows(LOG),
    source: SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the scoped breaker surface', () => {
  assert.equal(EXIT_SCOPED_BREAKER, 46)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/trusted_service')
  assert.equal(RUN_PATTERN, 'TestScopedBreaker')
  assert.equal(EXPECTED_PASSED, 14)
  assert.deepEqual(REQUIRED_SOURCES, ['scoped_breaker.go', 'circuit_breaker.go'])
  assert.deepEqual(REQUIRED_TESTS, ['scoped_breaker_test.go'])
  assert.equal(BREAKER_CASES.length, 14)
  assert.equal(BREAKER_GUARDS.length, 11)
  assert.equal(SCOPED_BREAKER_GUARDS.length, 1)
})

test('every case has a distinct evidence pattern', () => {
  const sources = BREAKER_CASES.map((name) => rowPattern(name).source)
  assert.equal(new Set(sources).size, BREAKER_CASES.length)
  for (const pattern of BREAKER_CASES.map(rowPattern)) assert.ok(pattern instanceof RegExp)
})

test('unknown case names fall back to the closed-no-probe pattern', () => {
  assert.equal(rowPattern('whatever').source, rowPattern('closed-no-probe').source)
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

test('all fourteen evidence rows are parsed', () => {
  const rows = parseBreakerRows(LOG)
  assert.deepEqual(rows['invalid-keys'], [7, 1, 'true', 0])
  assert.deepEqual(rows['scope-isolation'], ['open', 'closed', 'closed', 'closed'])
  assert.deepEqual(rows['probe-cap'], [2, 1, 'half_open'])
  assert.deepEqual(rows['concurrent-probe'], [16, 1, 15, 0])
  assert.deepEqual(rows['open-list'], [2, 1, 0, 'ascending', 'true'])
  assert.deepEqual(rows.snapshot, [4, 'true', 'true', 1])
  assert.equal(Object.keys(rows).length, BREAKER_CASES.length)
})

test('a clean evidence bundle has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('missing evidence rows are reported by name', () => {
  const problems = validateEvidence(evidence({ rows: parseBreakerRows(''), log: '' }))
  for (const name of BREAKER_CASES) assert.ok(problems.includes(`evidence row ${name} is missing`), name)
})

test('a duplicated evidence row is rejected', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}\nm04-s06 breaker probe-cap probes=2 denied=1 state=half_open\n` }))
  assert.ok(problems.includes('evidence row probe-cap must print exactly once'))
})

test('invalid keys that registered a breaker are rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['invalid-keys'] = [6, 1, 'false', 1]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row invalid-keys column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row invalid-keys column 2')))
  assert.ok(problems.some((problem) => problem.includes('evidence row invalid-keys column 3')))
})

test('a trip that crossed scopes is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['scope-isolation'] = ['open', 'open', 'closed', 'closed']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row scope-isolation column 1')))
})

test('a trip that crossed services is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['scope-isolation'] = ['open', 'closed', 'open', 'closed']
  assert.ok(validateEvidence(evidence({ rows })).some((problem) => problem.includes('evidence row scope-isolation column 2')))
})

test('a trip that crossed providers is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['scope-isolation'] = ['open', 'closed', 'closed', 'open']
  assert.ok(validateEvidence(evidence({ rows })).some((problem) => problem.includes('evidence row scope-isolation column 3')))
})

test('an early trip or a reset that never happened is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows.threshold = [3, 3, 3, 'false', 2]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row threshold column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row threshold column 3')))
  assert.ok(problems.some((problem) => problem.includes('evidence row threshold column 4')))
})

test('an unlimited half-open window is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['probe-cap'] = [3, 0, 'half_open']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row probe-cap column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row probe-cap column 1')))
})

test('a half-open success that left the circuit open is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['probe-success'] = [1, 'half_open', 2, 1]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row probe-success column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row probe-success column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row probe-success column 2')))
})

test('a half-open failure that stayed half-open is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['probe-failure'] = ['false', 1, 0, 'half_open']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row probe-failure column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row probe-failure column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row probe-failure column 3')))
})

test('an aborted probe that kept its slot is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['probe-abort'] = [3, 3, 3, 'half_open']
  assert.ok(validateEvidence(evidence({ rows })).some((problem) => problem.includes('evidence row probe-abort column 2')))
})

test('concurrent probes that both passed are rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['concurrent-probe'] = [16, 2, 14, 0]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row concurrent-probe column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row concurrent-probe column 2')))
})

test('concurrent keys that bled into another key are rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['concurrent-keys'] = [8, 8, 5, 'open']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row concurrent-keys column 2')))
  assert.ok(problems.some((problem) => problem.includes('evidence row concurrent-keys column 3')))
})

test('an open list that leaked scopes is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['open-list'] = [3, 2, 1, 'descending', 'false']
  const problems = validateEvidence(evidence({ rows }))
  ;['0', '1', '2', '3', '4'].forEach((column) => {
    assert.ok(problems.some((problem) => problem.includes(`evidence row open-list column ${column}`)), column)
  })
})

test('a reset that cleared a peer is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['reset-local'] = ['closed', 'closed', 0]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row reset-local column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row reset-local column 2')))
})

test('an unstable snapshot is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows.snapshot = [4, 'false', 'false', 0]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row snapshot column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row snapshot column 2')))
  assert.ok(problems.some((problem) => problem.includes('evidence row snapshot column 3')))
})

test('defaults that do not kick in are rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['config-defaults'] = [0, 4, 4]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row config-defaults column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row config-defaults column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row config-defaults column 2')))
})

test('a closed circuit that consumed probes is rejected', () => {
  const rows = parseBreakerRows(LOG)
  rows['closed-no-probe'] = [5, 5, 5]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row closed-no-probe column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row closed-no-probe column 2')))
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
  const problems = validateEvidence(evidence({ goFiles: ['circuit_breaker.go'] }))
  assert.ok(problems.includes('default build is missing scoped_breaker.go'))
})

test('a leaked test file in the default build is rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['circuit_breaker.go', 'scoped_breaker_test.go'] }))
  assert.ok(problems.includes('default build leaked test files'))
})

test('a missing test file is rejected', () => {
  const problems = validateEvidence(evidence({ testFiles: ['circuit_breaker_test.go'] }))
  assert.ok(problems.includes('test build is missing scoped_breaker_test.go'))
})

test('breaker guards are validated one by one', () => {
  assert.deepEqual(validateBreakerSource(SOURCE), [])
  for (const guard of BREAKER_GUARDS) {
    const problems = validateBreakerSource(SOURCE.split(guard).join(''))
    assert.ok(problems.includes(`circuit breaker source is missing ${guard}`), guard)
  }
})

test('scoped breaker guards are validated one by one', () => {
  assert.deepEqual(validateScopedSource(SOURCE), [])
  for (const guard of SCOPED_BREAKER_GUARDS) {
    const problems = validateScopedSource(SOURCE.split(guard).join(''))
    assert.ok(problems.includes(`scoped breaker source is missing ${guard}`), guard)
  }
})

test('an empty source reports every guard', () => {
  assert.equal(validateBreakerSource('').length, BREAKER_GUARDS.length)
  assert.equal(validateScopedSource('').length, SCOPED_BREAKER_GUARDS.length)
})

test('rendered evidence carries every case line plus the summary', () => {
  const lines = renderEvidence(evidence()).trimEnd().split('\n')
  assert.equal(lines.length, 18)
  assert.ok(lines[0].startsWith('invocation=scoped-breaker package='))
  assert.ok(lines[1].startsWith('files=2 tests=2 sources=2/2 testExcluded=true'))
  assert.equal(lines[2], 'breaker guards frozen=true scoped guards frozen=true')
  for (const name of BREAKER_CASES) assert.ok(lines.some((line) => line.includes(`breaker ${name} `)), name)
  assert.ok(lines.at(-1).startsWith('passed=14 failed=0 exit=0 race=true'))
})

test('rendered evidence marks missing rows with fallbacks', () => {
  const lines = renderEvidence(evidence({ rows: {}, source: '', passed: 0, failed: 0, exitCode: 1, race: false })).trimEnd().split('\n')
  assert.ok(lines.includes('breaker guards frozen=false scoped guards frozen=false'))
  assert.ok(lines.some((line) => line.includes('probe-cap probes=0 denied=0 state=')))
  assert.ok(lines.some((line) => line.includes('concurrent-probe workers=0 granted=0 denied=0 other=0')))
  assert.ok(lines.some((line) => line.endsWith('passed=0 failed=0 exit=1 race=false')))
})
