import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EXPECTED_PASSED,
  EXIT_INVOCATION_CONTRACT,
  HOST_HOP,
  INVOCATION_CASES,
  PACKAGE_PATH,
  PROVIDER_HOP,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  SIDE_EFFECT_CLASSES,
  parseGoListFiles,
  parseGoTestOutput,
  parseInvocationRows,
  renderEvidence,
  validateEvidence
} from '../m04-invocation-contract.mjs'

const LOG = `=== RUN   TestInvocationTraceRequiresIdentity
invocation trace guards fields=root,trace,invocation error=ErrInvocationContractInvalid
=== RUN   TestInvocationTraceChildKeepsRoot
invocation trace child root=root-1 trace=trace-1 parent=inv-1 child=inv-2
=== RUN   TestInvocationContractRequiresIdempotencyForMutating
invocation contract mutating=write,external requiresKey=true readMutating=false
=== RUN   TestInvocationOutcomeSuccessAndPreDispatch
invocation outcome success=success preDispatch=write+timeout result=failed unknown=false
=== RUN   TestInvocationOutcomePostDispatchWriteIsUnknown
invocation outcome write postDispatch=timeout outcome=unknown retrySafe=false cause=true
=== RUN   TestInvocationOutcomePostDispatchReadIsFailed
invocation outcome read postDispatch=timeout outcome=failed ambiguous=true
=== RUN   TestInvocationContractJournalEntryCarriesTrace
invocation journal root=root-1 trace=trace-1 invocation=inv-1 entries=2
=== RUN   TestInvocationChainSpansHostAndProvider
invocation chain root=root-1 trace=trace-1 invocation=inv-1 parent=inv-0 child=inv-2 hops=host_api>capability entries=3 sideEffect=write outcome=unknown
PASS
ok  	github.com/u-ai/backend/internal/execution	1.758s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['invocation.go', 'journal.go', 'context.go'],
    testFiles: ['invocation_chain_test.go', 'invocation_test.go', 'journal_test.go'],
    rows: parseInvocationRows(LOG),
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the invocation contract surface', () => {
  assert.equal(EXIT_INVOCATION_CONTRACT, 41)
  assert.equal(PACKAGE_PATH, './internal/execution')
  assert.equal(RUN_PATTERN, 'TestInvocation')
  assert.equal(EXPECTED_PASSED, 8)
  assert.equal(HOST_HOP, 'host_api')
  assert.equal(PROVIDER_HOP, 'capability')
  assert.deepEqual(REQUIRED_SOURCES, ['invocation.go'])
  assert.deepEqual(REQUIRED_TESTS, ['invocation_test.go', 'invocation_chain_test.go'])
  assert.deepEqual(SIDE_EFFECT_CLASSES, ['none', 'read', 'write', 'external'])
  assert.deepEqual(INVOCATION_CASES, [
    'trace-guard',
    'trace-child',
    'contract-key',
    'outcome-success',
    'outcome-write-unknown',
    'outcome-read-failed',
    'journal',
    'chain'
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

test('invocation rows expose every field as typed values', () => {
  const rows = parseInvocationRows(LOG)
  assert.deepEqual(rows['trace-guard'], ['root,trace,invocation', 'ErrInvocationContractInvalid'])
  assert.deepEqual(rows['trace-child'], ['root-1', 'trace-1', 'inv-1', 'inv-2'])
  assert.deepEqual(rows['contract-key'], ['write,external', 'true', 'false'])
  assert.deepEqual(rows['outcome-success'], ['success', 'write+timeout', 'failed', 'false'])
  assert.deepEqual(rows['outcome-write-unknown'], ['timeout', 'unknown', 'false', 'true'])
  assert.deepEqual(rows['outcome-read-failed'], ['timeout', 'failed', 'true'])
  assert.deepEqual(rows.journal, ['root-1', 'trace-1', 'inv-1', 2])
  assert.deepEqual(rows.chain, ['root-1', 'trace-1', 'inv-1', 'inv-0', 'inv-2', 'host_api>capability', 3, 'write', 'unknown'])
})

test('missing rows parse to null', () => {
  const rows = parseInvocationRows('nothing here')
  for (const name of INVOCATION_CASES) assert.equal(rows[name], null)
})

test('clean evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must carry invocation.go', () => {
  const problems = validateEvidence(evidence({ goFiles: ['journal.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing invocation.go')))
})

test('default build must not leak test files', () => {
  const problems = validateEvidence(evidence({ goFiles: ['invocation.go', 'invocation_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('leaked test files')))
})

test('test build must carry both the unit and the chain test', () => {
  const problems = validateEvidence(evidence({ testFiles: ['invocation_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing invocation_chain_test.go')))
  const unit = validateEvidence(evidence({ testFiles: ['invocation_chain_test.go'] }))
  assert.ok(unit.some((problem) => problem.includes('missing invocation_test.go')))
})

test('each evidence row must print exactly once', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}invocation journal root=root-1 trace=trace-1 invocation=inv-1 entries=2\n` }))
  assert.ok(problems.some((problem) => problem.includes('evidence row journal must print exactly once')))
})

test('a trace missing any identity must be rejected', () => {
  const rows = parseInvocationRows(LOG)
  const drifted = validateEvidence(evidence({ rows: { ...rows, 'trace-guard': ['root,trace', 'ErrInvocationContractInvalid'] } }))
  assert.ok(drifted.some((problem) => problem.includes('trace guard fields=root,trace')))
  const loose = validateEvidence(evidence({ rows: { ...rows, 'trace-guard': ['root,trace,invocation', 'nil'] } }))
  assert.ok(loose.some((problem) => problem.includes('trace guard error=nil')))
})

test('a child invocation must keep the root and trace', () => {
  const rows = parseInvocationRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'trace-child': ['root-2', 'trace-1', 'inv-1', 'inv-2'] } }))
  assert.ok(problems.some((problem) => problem.includes('child lost root or trace root=root-2')))
  const forked = validateEvidence(evidence({ rows: { ...rows, 'trace-child': ['root-1', 'trace-2', 'inv-1', 'inv-2'] } }))
  assert.ok(forked.some((problem) => problem.includes('trace=trace-2')))
})

test('a child invocation must point at its parent', () => {
  const rows = parseInvocationRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'trace-child': ['root-1', 'trace-1', 'inv-9', 'inv-2'] } }))
  assert.ok(problems.some((problem) => problem.includes('child parent=inv-9')))
  const reused = validateEvidence(evidence({ rows: { ...rows, 'trace-child': ['root-1', 'trace-1', 'inv-1', 'inv-1'] } }))
  assert.ok(reused.some((problem) => problem.includes('child id=inv-1')))
})

test('mutating invocations must carry an idempotency key', () => {
  const rows = parseInvocationRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'contract-key': ['write,external', 'false', 'false'] } }))
  assert.ok(problems.some((problem) => problem.includes('must be rejected')))
  const wrongClasses = validateEvidence(evidence({ rows: { ...rows, 'contract-key': ['write', 'true', 'false'] } }))
  assert.ok(wrongClasses.some((problem) => problem.includes('mutating classes=write')))
})

test('a read invocation must never require a key', () => {
  const rows = parseInvocationRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'contract-key': ['write,external', 'true', 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('a read invocation must not be mutating')))
})

test('a pre-dispatch timeout must stay failed', () => {
  const rows = parseInvocationRows(LOG)
  const unknown = validateEvidence(evidence({ rows: { ...rows, 'outcome-success': ['success', 'write+timeout', 'failed', 'true'] } }))
  assert.ok(unknown.some((problem) => problem.includes('must never be unknown')))
  const denied = validateEvidence(evidence({ rows: { ...rows, 'outcome-success': ['success', 'write+timeout', 'unknown', 'false'] } }))
  assert.ok(denied.some((problem) => problem.includes('pre-dispatch timeout outcome=unknown')))
})

test('a dispatched mutating timeout must be unknown and not retry safe', () => {
  const rows = parseInvocationRows(LOG)
  const safe = validateEvidence(evidence({ rows: { ...rows, 'outcome-write-unknown': ['timeout', 'unknown', 'true', 'true'] } }))
  assert.ok(safe.some((problem) => problem.includes('must not be retry safe')))
  const failed = validateEvidence(evidence({ rows: { ...rows, 'outcome-write-unknown': ['timeout', 'failed', 'false', 'true'] } }))
  assert.ok(failed.some((problem) => problem.includes('outcome=failed want unknown')))
  const lost = validateEvidence(evidence({ rows: { ...rows, 'outcome-write-unknown': ['timeout', 'unknown', 'false', 'false'] } }))
  assert.ok(lost.some((problem) => problem.includes('must preserve the original cause')))
})

test('a dispatched read timeout must stay failed but ambiguous', () => {
  const rows = parseInvocationRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'outcome-read-failed': ['timeout', 'unknown', 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('outcome=unknown want failed')))
  const plain = validateEvidence(evidence({ rows: { ...rows, 'outcome-read-failed': ['none', 'failed', 'false'] } }))
  assert.ok(plain.some((problem) => problem.includes('must be recognised as an ambiguous failure')))
})

test('journal entries must carry the frozen trace', () => {
  const rows = parseInvocationRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, journal: ['root-1', 'trace-1', 'inv-9', 2] } }))
  assert.ok(problems.some((problem) => problem.includes('journal trace drifted')))
  const missing = validateEvidence(evidence({ rows: { ...rows, journal: ['root-1', 'trace-1', 'inv-1', 1] } }))
  assert.ok(missing.some((problem) => problem.includes('journal entries=1 want 2')))
})

test('the chain must cross host_api and capability with one invocation id', () => {
  const rows = parseInvocationRows(LOG)
  const hops = validateEvidence(evidence({ rows: { ...rows, chain: ['root-1', 'trace-1', 'inv-1', 'inv-0', 'inv-2', 'capability>host_api', 3, 'write', 'unknown'] } }))
  assert.ok(hops.some((problem) => problem.includes('chain hops=capability>host_api')))
  const entries = validateEvidence(evidence({ rows: { ...rows, chain: ['root-1', 'trace-1', 'inv-1', 'inv-0', 'inv-2', 'host_api>capability', 2, 'write', 'unknown'] } }))
  assert.ok(entries.some((problem) => problem.includes('chain entries=2 want 3')))
})

test('the chain must report the write class and the unknown outcome', () => {
  const rows = parseInvocationRows(LOG)
  const klass = validateEvidence(evidence({ rows: { ...rows, chain: ['root-1', 'trace-1', 'inv-1', 'inv-0', 'inv-2', 'host_api>capability', 3, 'hybrid', 'unknown'] } }))
  assert.ok(klass.some((problem) => problem.includes('chain sideEffect=hybrid is not a frozen class')))
  const readOnly = validateEvidence(evidence({ rows: { ...rows, chain: ['root-1', 'trace-1', 'inv-1', 'inv-0', 'inv-2', 'host_api>capability', 3, 'read', 'unknown'] } }))
  assert.ok(readOnly.some((problem) => problem.includes('chain sideEffect=read want write')))
  const silent = validateEvidence(evidence({ rows: { ...rows, chain: ['root-1', 'trace-1', 'inv-1', 'inv-0', 'inv-2', 'host_api>capability', 3, 'write', 'failed'] } }))
  assert.ok(silent.some((problem) => problem.includes('chain outcome=failed want unknown')))
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
  assert.match(lines[0], /invocation=contract package=\.\/internal\/execution run=TestInvocation classes=none,read,write,external cases=8/)
  assert.match(lines[1], /^files=3 tests=3 sources=1\/1 testExcluded=true$/)
  assert.match(lines[2], /^invocation trace guards /)
  assert.match(lines[9], /^invocation chain root=root-1 trace=trace-1 invocation=inv-1 parent=inv-0 child=inv-2 hops=host_api>capability entries=3 sideEffect=write outcome=unknown$/)
  assert.match(lines[10], /^passed=8 failed=0 exit=0 race=true$/)
})

test('rendered evidence survives a total parse failure without inventing values', () => {
  const output = renderEvidence(evidence({ rows: parseInvocationRows(''), passed: 0, exitCode: 1, race: false, log: '' }))
  assert.match(output, /invocation trace guards fields= error=/)
  assert.match(output, /invocation outcome write postDispatch= outcome= retrySafe=true cause=false/)
  assert.match(output, /invocation chain root= trace= invocation= parent= child= hops= entries=0 sideEffect= outcome=/)
})
