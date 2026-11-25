import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EXIT_ROUTE_FALLBACK,
  EXPECTED_PASSED,
  FALLBACK_CASES,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  TABLE_ROWS,
  parseFallbackRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateEvidence
} from '../m03-route-fallback.mjs'

const LOG = `=== RUN   TestRouteFallbackReadUnavailableSwitches
route fallback read unavailable action=switch_provider alternatives=1 switchAllowed=true retryAllowed=false
=== RUN   TestRouteFallbackReadUnavailableWithoutAlternate
route fallback read unavailable empty action=stop alternatives=0 reason=read provider unavailable without an alternate candidate
=== RUN   TestRouteFallbackWriteUnavailableStops
route fallback write unavailable action=stop switchAllowed=false retryAllowed=false reason=write provider unavailable must not switch
=== RUN   TestRouteFallbackWriteFailedStops
route fallback write failed action=stop switchAllowed=false reason=write failures are surfaced to the caller
=== RUN   TestRouteFallbackUnknownWriteNeverSwitches
route fallback write unknown action=stop alternatives=3 switchAllowed=false retryAllowed=false reason=unknown write result must never switch or re-execute
=== RUN   TestRouteFallbackReadFailedStops
route fallback read failed action=stop switchAllowed=false
=== RUN   TestRouteFallbackTruthTable
route fallback table rows=8 writeSwitches=0 unknownWritesSwitch=0
=== RUN   TestRouteFallbackInvalidInput
route fallback invalid fields=operation,outcome,alternatives error=ErrRouteFallbackInvalid
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/plugincontract	1.609s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['doc.go', 'route_fallback.go', 'route_policy.go'],
    testFiles: ['route_fallback_test.go', 'route_policy_test.go'],
    rows: parseFallbackRows(LOG),
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the route fallback surface', () => {
  assert.equal(EXIT_ROUTE_FALLBACK, 39)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/plugincontract')
  assert.equal(RUN_PATTERN, 'TestRouteFallback')
  assert.equal(EXPECTED_PASSED, 8)
  assert.equal(TABLE_ROWS, 8)
  assert.deepEqual(REQUIRED_SOURCES, ['route_fallback.go'])
  assert.deepEqual(REQUIRED_TESTS, ['route_fallback_test.go'])
  assert.deepEqual(FALLBACK_CASES, [
    'read-switch',
    'read-empty',
    'write-unavailable',
    'write-failed',
    'write-unknown',
    'read-failed',
    'table',
    'invalid'
  ])
})

test('go list output is parsed into sorted file lists', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('a.go'), [])
})

test('go test output counts pass and fail lines', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 0, failed: 0 })
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- FAIL: B\n'), { passed: 1, failed: 1 })
})

test('fallback rows expose every field as typed values', () => {
  const rows = parseFallbackRows(LOG)
  assert.deepEqual(rows['read-switch'], ['switch_provider', 1, 'true', 'false'])
  assert.deepEqual(rows['read-empty'], ['stop', 0, 'read provider unavailable without an alternate candidate'])
  assert.deepEqual(rows['write-unavailable'], ['stop', 'false', 'false', 'write provider unavailable must not switch'])
  assert.deepEqual(rows['write-failed'], ['stop', 'false', 'write failures are surfaced to the caller'])
  assert.deepEqual(rows['write-unknown'], ['stop', 3, 'false', 'false', 'unknown write result must never switch or re-execute'])
  assert.deepEqual(rows['read-failed'], ['stop', 'false'])
  assert.deepEqual(rows.table, [8, 0, 0])
  assert.deepEqual(rows.invalid, ['operation,outcome,alternatives', 'ErrRouteFallbackInvalid'])
})

test('missing rows parse to null', () => {
  const rows = parseFallbackRows('nothing here')
  for (const name of FALLBACK_CASES) assert.equal(rows[name], null)
})

test('clean evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must carry route_fallback.go', () => {
  const problems = validateEvidence(evidence({ goFiles: ['doc.go', 'route_policy.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_fallback.go')))
})

test('default build must not leak test files', () => {
  const problems = validateEvidence(evidence({ goFiles: ['route_fallback.go', 'route_fallback_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('leaked test files')))
})

test('test build must carry the fallback test file', () => {
  const problems = validateEvidence(evidence({ testFiles: ['route_policy_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_fallback_test.go')))
})

test('each evidence row must print exactly once', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}route fallback table rows=8 writeSwitches=0 unknownWritesSwitch=0\n` }))
  assert.ok(problems.some((problem) => problem.includes('evidence row table must print exactly once')))
})

test('a read-only outage may switch only when an alternate exists', () => {
  const rows = parseFallbackRows(LOG)
  const drifted = validateEvidence(evidence({ rows: { ...rows, 'read-switch': ['stop', 1, 'true', 'false'] } }))
  assert.ok(drifted.some((problem) => problem.includes('read switch action=stop')))
  const empty = validateEvidence(evidence({ rows: { ...rows, 'read-empty': ['switch_provider', 0, 'x'] } }))
  assert.ok(empty.some((problem) => problem.includes('read empty action=switch_provider')))
})

test('a write outage must never switch providers', () => {
  const rows = parseFallbackRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'write-unavailable': ['switch_provider', 'true', 'false', 'x'] } }))
  assert.ok(problems.some((problem) => problem.includes('write unavailable action=switch_provider')))
})

test('a failed write must be surfaced', () => {
  const rows = parseFallbackRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'write-failed': ['switch_provider', 'false', 'x'] } }))
  assert.ok(problems.some((problem) => problem.includes('write failed action=switch_provider')))
})

test('an unknown write result must never switch or re-execute', () => {
  const rows = parseFallbackRows(LOG)
  const switching = validateEvidence(evidence({ rows: { ...rows, 'write-unknown': ['switch_provider', 3, 'true', 'false', 'x'] } }))
  assert.ok(switching.some((problem) => problem.includes('must never switch providers')))
  const retrying = validateEvidence(evidence({ rows: { ...rows, 'write-unknown': ['stop', 3, 'false', 'true', 'x'] } }))
  assert.ok(retrying.some((problem) => problem.includes('must never re-execute')))
})

test('a failed read must be surfaced instead of switched', () => {
  const rows = parseFallbackRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'read-failed': ['switch_provider', 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('read failed action=switch_provider')))
})

test('the truth table must stay at eight rows with zero write switches', () => {
  const rows = parseFallbackRows(LOG)
  const rowsDrift = validateEvidence(evidence({ rows: { ...rows, table: [6, 0, 0] } }))
  assert.ok(rowsDrift.some((problem) => problem.includes('table rows=6')))
  const writeSwitch = validateEvidence(evidence({ rows: { ...rows, table: [8, 1, 0] } }))
  assert.ok(writeSwitch.some((problem) => problem.includes('table writeSwitches=1')))
  const unknownSwitch = validateEvidence(evidence({ rows: { ...rows, table: [8, 0, 1] } }))
  assert.ok(unknownSwitch.some((problem) => problem.includes('table unknownWritesSwitch=1')))
})

test('invalid input must name every rejected field', () => {
  const rows = parseFallbackRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, invalid: ['operation', 'ErrRouteFallbackInvalid'] } }))
  assert.ok(problems.some((problem) => problem.includes('invalid fields=operation')))
})

test('run metadata must prove the race detector and a clean exit', () => {
  const problems = validateEvidence(evidence({ passed: 7, failed: 1, exitCode: 1, race: false }))
  assert.ok(problems.some((problem) => problem.includes('passed=7')))
  assert.ok(problems.some((problem) => problem.includes('failed=1')))
  assert.ok(problems.some((problem) => problem.includes('exit=1')))
  assert.ok(problems.some((problem) => problem.includes('must run with -race')))
})

test('rendered evidence keeps the eight frozen rows in order', () => {
  const output = renderEvidence(evidence())
  const lines = output.trimEnd().split('\n')
  assert.equal(lines.length, 11)
  assert.match(lines[0], /route=fallback package=\.\/internal\/extension\/kernel\/plugincontract/)
  assert.match(lines[1], /^files=3 tests=2 sources=1\/1 testExcluded=true$/)
  assert.match(lines[2], /^route fallback read unavailable /)
  assert.match(lines[9], /^route fallback invalid /)
  assert.match(lines[10], /^passed=8 failed=0 exit=0 race=true$/)
})

test('rendered evidence survives a total parse failure without inventing values', () => {
  const output = renderEvidence(evidence({ rows: parseFallbackRows(''), passed: 0, exitCode: 1, race: false, log: '' }))
  assert.match(output, /route fallback read unavailable action= alternatives=0 switchAllowed=false retryAllowed=false/)
  assert.match(output, /route fallback table rows=0 writeSwitches=0 unknownWritesSwitch=0/)
})
