import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EXIT_READ_FALLBACK,
  EXPECTED_PASSED,
  PACKAGE_PATH,
  READ_FALLBACK_CASES,
  READ_FALLBACK_GUARDS,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  parseGoListFiles,
  parseGoTestOutput,
  parseReadFallbackRows,
  renderEvidence,
  rowPattern,
  validateEvidence,
  validateReadFallbackSource
} from '../m04-read-fallback.mjs'

const SOURCE = READ_FALLBACK_GUARDS.join('\n')

const LOG = `=== RUN   TestReadFallbackRejectsMutatingContract
m04-s03 read-fallback mutating-blocked sideEffects=2 executed=0
=== RUN   TestReadFallbackPluginCrashReturnsCoreResult
m04-s03 read-fallback crash-core candidates=2 executed=2 selected=builtin.core/core/memory fallback=true
=== RUN   TestReadFallbackPermissionDeniedNeverFallsBack
m04-s03 read-fallback permission-denied candidates=2 executed=1 coreExecuted=0
=== RUN   TestReadFallbackUndeclaredPermissionSkipsEveryCandidate
m04-s03 read-fallback permission-undeclared candidates=2 executed=0
=== RUN   TestReadFallbackCrossScopeCandidateIsNotInvoked
m04-s03 read-fallback cross-scope candidates=2 executed=1 pluginExecuted=0
=== RUN   TestReadFallbackExhaustedKeepsErrorChain
m04-s03 read-fallback exhausted candidates=2 executed=2
=== RUN   TestReadFallbackPrimarySuccessDoesNotTouchCore
m04-s03 read-fallback primary-success candidates=2 executed=1 coreExecuted=0
=== RUN   TestReadFallbackInjectedErrorTextDoesNotOverride
m04-s03 read-fallback injection-contained pluginCalls=1 coreCalls=1 fallback=true
=== RUN   TestReadFallbackCancelledContextStopsBeforeFallback
m04-s03 read-fallback cancelled executed=0
=== RUN   TestReadFallbackEmptyOutputFallsBack
m04-s03 read-fallback empty-output candidates=2 executed=2 selected=builtin.core/core/memory
=== RUN   TestReadFallbackUnknownOutcomeIsNotReplayed
m04-s03 read-fallback unknown-outcome candidates=2 executed=1 coreExecuted=0
=== RUN   TestReadFallbackOpenCircuitSkipsPlugin
m04-s03 read-fallback open-circuit state=open pluginExecuted=0 selected=builtin.core/core/memory
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/plugincontract	2.402s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['route_read_fallback.go', 'route_fallback.go'],
    testFiles: ['route_read_fallback_test.go', 'route_fallback_test.go'],
    rows: parseReadFallbackRows(LOG),
    source: SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the read fallback surface', () => {
  assert.equal(EXIT_READ_FALLBACK, 43)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/plugincontract')
  assert.equal(RUN_PATTERN, 'TestReadFallback')
  assert.equal(EXPECTED_PASSED, 12)
  assert.deepEqual(REQUIRED_SOURCES, ['route_read_fallback.go'])
  assert.deepEqual(REQUIRED_TESTS, ['route_read_fallback_test.go'])
  assert.equal(READ_FALLBACK_CASES.length, 12)
  assert.equal(READ_FALLBACK_GUARDS.length, 10)
})

test('every case has a distinct evidence pattern', () => {
  const sources = READ_FALLBACK_CASES.map((name) => rowPattern(name).source)
  assert.equal(new Set(sources).size, READ_FALLBACK_CASES.length)
  for (const pattern of READ_FALLBACK_CASES.map(rowPattern)) assert.ok(pattern instanceof RegExp)
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

test('all twelve evidence rows are parsed', () => {
  const rows = parseReadFallbackRows(LOG)
  assert.deepEqual(rows['mutating-blocked'], [2, 0])
  assert.deepEqual(rows['permission-denied'], [2, 1, 0])
  assert.deepEqual(rows['injection-contained'], [1, 1, 'true'])
  assert.deepEqual(rows['open-circuit'], ['open', 0, 'builtin.core/core/memory'])
  assert.equal(Object.keys(rows).length, READ_FALLBACK_CASES.length)
})

test('a clean evidence bundle has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('missing evidence rows are reported by name', () => {
  const rows = parseReadFallbackRows('')
  const problems = validateEvidence(evidence({ rows, log: '' }))
  for (const name of READ_FALLBACK_CASES) assert.ok(problems.some((problem) => problem === `evidence row ${name} is missing`), name)
})

test('a duplicated evidence row is rejected', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}\nm04-s03 read-fallback cancelled executed=0\n` }))
  assert.ok(problems.includes('evidence row cancelled must print exactly once'))
})

test('a crash row that stayed on the plugin is rejected', () => {
  const rows = parseReadFallbackRows(LOG)
  rows['crash-core'] = [2, 1, 'plugins-code/emote/memory', 'false']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row crash-core column 1')))
  assert.ok(problems.some((problem) => problem.includes('evidence row crash-core column 2')))
  assert.ok(problems.some((problem) => problem.includes('evidence row crash-core column 3')))
})

test('a permission denial that still executed core is rejected', () => {
  const rows = parseReadFallbackRows(LOG)
  rows['permission-denied'] = [2, 2, 1]
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row permission-denied column 2')))
})

test('a cross-scope plugin that ran is rejected', () => {
  const rows = parseReadFallbackRows(LOG)
  rows['cross-scope'] = [2, 2, 1]
  assert.ok(validateEvidence(evidence({ rows })).some((problem) => problem.includes('evidence row cross-scope column 2')))
})

test('mutating contracts that executed a provider are rejected', () => {
  const rows = parseReadFallbackRows(LOG)
  rows['mutating-blocked'] = [2, 1]
  assert.ok(validateEvidence(evidence({ rows })).some((problem) => problem.includes('evidence row mutating-blocked column 1')))
})

test('an open breaker that still executed the plugin is rejected', () => {
  const rows = parseReadFallbackRows(LOG)
  rows['open-circuit'] = ['closed', 1, 'plugins-code/emote/memory']
  const problems = validateEvidence(evidence({ rows }))
  assert.ok(problems.some((problem) => problem.includes('evidence row open-circuit column 0')))
  assert.ok(problems.some((problem) => problem.includes('evidence row open-circuit column 1')))
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
  assert.ok(validateEvidence(evidence({ passed: 11 })).includes('passed=11 want 12'))
})

test('a missing source file is rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['route_fallback.go'] }))
  assert.ok(problems.includes('default build is missing route_read_fallback.go'))
})

test('a leaked test file in the default build is rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['route_read_fallback.go', 'route_read_fallback_test.go'] }))
  assert.ok(problems.includes('default build leaked test files'))
})

test('a missing test file is rejected', () => {
  const problems = validateEvidence(evidence({ testFiles: ['route_fallback_test.go'] }))
  assert.ok(problems.includes('test build is missing route_read_fallback_test.go'))
})

test('source guards are validated one by one', () => {
  assert.deepEqual(validateReadFallbackSource(SOURCE), [])
  for (const guard of READ_FALLBACK_GUARDS) {
    const problems = validateReadFallbackSource(SOURCE.split(guard).join(''))
    assert.ok(problems.includes(`read fallback source is missing ${guard}`), guard)
  }
})

test('an empty source reports every guard', () => {
  assert.equal(validateReadFallbackSource('').length, READ_FALLBACK_GUARDS.length)
})

test('rendered evidence carries every case line plus the summary', () => {
  const lines = renderEvidence(evidence()).trimEnd().split('\n')
  assert.equal(lines.length, 16)
  assert.ok(lines[0].startsWith('invocation=read-fallback package='))
  assert.ok(lines[1].startsWith('files=2 tests=2 sources=1/1 testExcluded=true'))
  assert.equal(lines[2], 'read fallback guards frozen=true')
  for (const name of READ_FALLBACK_CASES) {
    assert.ok(lines.some((line) => line.includes(`read-fallback ${name} `)), name)
  }
  assert.ok(lines.at(-1).startsWith('passed=12 failed=0 exit=0 race=true'))
})

test('rendered evidence marks missing rows with fallbacks', () => {
  const lines = renderEvidence(evidence({ rows: {}, source: '', passed: 0, failed: 0, exitCode: 1, race: false })).trimEnd().split('\n')
  assert.ok(lines.includes('read fallback guards frozen=false'))
  assert.ok(lines.some((line) => line.includes('crash-core candidates=0 executed=0 selected= fallback=false')))
  assert.ok(lines.some((line) => line.includes('open-circuit state= pluginExecuted=0 selected=')))
  assert.ok(lines.some((line) => line.endsWith('passed=0 failed=0 exit=1 race=false')))
})
