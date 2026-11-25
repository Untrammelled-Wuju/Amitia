import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  CAS_CASES,
  EXPECTED_PASSED,
  EXPECTED_WORKERS,
  EXIT_BINDING_CAS,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  parseCommitRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateEvidence
} from '../m03-binding-cas.mjs'

const LOG = `=== RUN   TestBindingCommitCASAdvancesRevision
binding commit cas epoch=boot-1 from=1 to=2 installed=true digestChanged=true
=== RUN   TestBindingCommitRejectsStaleRevision
binding commit stale expected=1 actual=2 conflicts=1 chain=ok unchanged=true
=== RUN   TestBindingCommitRejectsInvalidWithoutMutation
binding commit invalid field=providerId rejected=1 revisionBefore=2 revisionAfter=2 retry=3
=== RUN   TestBindingCommitFromEmptyStore
binding commit empty from=0 to=1 installedBefore=false
=== RUN   TestBindingCommitPinsInflightProvider
binding commit pinned provider=global.host revision=1 currentProvider=next.host currentRevision=2 retained=1 releasedRetained=0 stable=true
=== RUN   TestBindingCommitCancellationSemantics
binding commit cancellation resolvedAfterRelease=ErrSnapshotRelease doubleRelease=idempotent acquireAfterRelease=2
=== RUN   TestBindingCommitConcurrentSingleWinner
binding commit concurrent workers=32 successes=1 conflicts=31 revision=2 winners=1
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/plugincontract	1.617s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['binding_commit.go', 'binding_snapshot.go', 'doc.go'],
    testFiles: ['binding_commit_test.go', 'binding_snapshot_test.go'],
    rows: parseCommitRows(LOG),
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the binding cas surface', () => {
  assert.equal(EXIT_BINDING_CAS, 36)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/plugincontract')
  assert.equal(RUN_PATTERN, 'TestBindingCommit')
  assert.equal(EXPECTED_PASSED, 7)
  assert.equal(EXPECTED_WORKERS, 32)
  assert.deepEqual(REQUIRED_SOURCES, ['binding_commit.go', 'binding_snapshot.go'])
  assert.deepEqual(REQUIRED_TESTS, ['binding_commit_test.go'])
  assert.deepEqual(CAS_CASES, ['cas', 'stale', 'invalid', 'empty', 'pinned', 'cancellation', 'concurrent'])
})

test('go list output is parsed into sorted file lists', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('a.go'), [])
})

test('go test output counts pass and fail lines', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 0, failed: 0 })
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('commit rows expose every field as typed values', () => {
  const rows = parseCommitRows(LOG)
  assert.deepEqual(rows.cas, ['boot-1', 1, 2, 'true', 'true'])
  assert.deepEqual(rows.stale, [1, 2, 1, 'ok', 'true'])
  assert.deepEqual(rows.invalid, ['providerId', 1, 2, 2, 3])
  assert.deepEqual(rows.empty, [0, 1, 'false'])
  assert.deepEqual(rows.pinned, ['global.host', 1, 'next.host', 2, 1, 0, 'true'])
  assert.deepEqual(rows.cancellation, ['ErrSnapshotRelease', 'idempotent', 2])
  assert.deepEqual(rows.concurrent, [32, 1, 31, 2, 1])
})

test('missing rows parse to null', () => {
  const rows = parseCommitRows('nothing here')
  for (const name of CAS_CASES) assert.equal(rows[name], null)
})

test('clean evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must carry both binding sources', () => {
  const problems = validateEvidence(evidence({ goFiles: ['binding_snapshot.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing binding_commit.go')))
})

test('default build must not leak test files', () => {
  const problems = validateEvidence(evidence({ goFiles: ['binding_commit.go', 'binding_snapshot.go', 'binding_commit_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('leaked test files')))
})

test('test build must carry the commit test file', () => {
  const problems = validateEvidence(evidence({ testFiles: ['binding_snapshot_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing binding_commit_test.go')))
})

test('each evidence row must print exactly once', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}binding commit cas epoch=boot-1 from=1 to=2 installed=true digestChanged=true\n` }))
  assert.ok(problems.some((problem) => problem.includes('evidence row cas must print exactly once')))
})

test('cas row must advance one revision and change the digest', () => {
  const rows = parseCommitRows(LOG)
  const drifted = evidence({ rows: { ...rows, cas: ['boot-1', 1, 3, 'true', 'true'] } })
  assert.ok(validateEvidence(drifted).some((problem) => problem.includes('cas from=1 to=3')))
  const flat = evidence({ rows: { ...rows, cas: ['boot-1', 1, 2, 'true', 'false'] } })
  assert.ok(validateEvidence(flat).some((problem) => problem.includes('digestChanged=false')))
})

test('stale commit must be rejected and leave the store unchanged', () => {
  const rows = parseCommitRows(LOG)
  const drifted = evidence({ rows: { ...rows, stale: [1, 2, 0, 'ok', 'false'] } })
  const problems = validateEvidence(drifted)
  assert.ok(problems.some((problem) => problem.includes('stale conflicts=0')))
  assert.ok(problems.some((problem) => problem.includes('stale unchanged=false')))
})

test('invalid commit must not move the revision and must allow a retry', () => {
  const rows = parseCommitRows(LOG)
  const moved = evidence({ rows: { ...rows, invalid: ['providerId', 1, 2, 3, 4] } })
  assert.ok(validateEvidence(moved).some((problem) => problem.includes('must not move')))
  const noRetry = evidence({ rows: { ...rows, invalid: ['providerId', 1, 2, 2, 2] } })
  assert.ok(validateEvidence(noRetry).some((problem) => problem.includes('invalid retry=2')))
})

test('empty store must start at revision zero', () => {
  const rows = parseCommitRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, empty: [1, 1, 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('empty from=1 to=1')))
  assert.ok(problems.some((problem) => problem.includes('installedBefore=true')))
})

test('pinned request must keep the old provider while the store moves on', () => {
  const rows = parseCommitRows(LOG)
  const drifted = evidence({ rows: { ...rows, pinned: ['next.host', 1, 'next.host', 2, 1, 0, 'false'] } })
  const problems = validateEvidence(drifted)
  assert.ok(problems.some((problem) => problem.includes('pinned provider=next.host')))
  assert.ok(problems.some((problem) => problem.includes('stable=false')))
})

test('released handles stay released and the store keeps serving', () => {
  const rows = parseCommitRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, cancellation: ['released', 'panic', 0] } }))
  assert.ok(problems.some((problem) => problem.includes('resolvedAfterRelease=released')))
  assert.ok(problems.some((problem) => problem.includes('doubleRelease=panic')))
  assert.ok(problems.some((problem) => problem.includes('acquireAfterRelease=0')))
})

test('concurrent commits must have exactly one winner', () => {
  const rows = parseCommitRows(LOG)
  const drifted = evidence({ rows: { ...rows, concurrent: [32, 2, 30, 2, 2] } })
  const problems = validateEvidence(drifted)
  assert.ok(problems.some((problem) => problem.includes('concurrent successes=2')))
  assert.ok(problems.some((problem) => problem.includes('concurrent winners=2')))
})

test('run metadata must prove the race detector and a clean exit', () => {
  const problems = validateEvidence(evidence({ passed: 6, failed: 1, exitCode: 1, race: false }))
  assert.ok(problems.some((problem) => problem.includes('passed=6')))
  assert.ok(problems.some((problem) => problem.includes('failed=1')))
  assert.ok(problems.some((problem) => problem.includes('exit=1')))
  assert.ok(problems.some((problem) => problem.includes('must run with -race')))
})

test('rendered evidence keeps the seven frozen rows in order', () => {
  const output = renderEvidence(evidence())
  const lines = output.trimEnd().split('\n')
  assert.equal(lines.length, 10)
  assert.match(lines[0], /binding=cas package=\.\/internal\/extension\/kernel\/plugincontract/)
  assert.match(lines[1], /^files=3 tests=2 sources=2\/2 testExcluded=true$/)
  assert.match(lines[2], /^binding commit cas /)
  assert.match(lines[8], /^binding commit concurrent /)
  assert.match(lines[9], /^passed=7 failed=0 exit=0 race=true$/)
})

test('rendered evidence survives a total parse failure without inventing values', () => {
  const output = renderEvidence(evidence({ rows: parseCommitRows(''), passed: 0, exitCode: 1, race: false, log: '' }))
  assert.match(output, /binding commit cas epoch= from=0 to=0 installed=false digestChanged=false/)
  assert.match(output, /binding commit concurrent workers=0 successes=0 conflicts=0 revision=0 winners=0/)
})
