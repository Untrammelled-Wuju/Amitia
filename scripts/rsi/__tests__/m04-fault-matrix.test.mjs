import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EXIT_FAULT_MATRIX,
  EXPECTED_PASSED,
  GUARD_GUARDS,
  MATRIX_CASES,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  TABLE_GUARDS,
  WIRING_GUARDS,
  countCaseLines,
  parseFaultLines,
  parseGoListFiles,
  parseGoTestOutput,
  parseTruthLine,
  renderEvidence,
  validateEvidence,
  validateGuardSource,
  validateTruthTable,
  validateWiringSource
} from '../m04-fault-matrix.mjs'

const LOG = `=== RUN   TestFaultMatrixReadPanic
m04-s07 matrix read-panic state=read fallback=core executions=2 chain=panic
--- PASS: TestFaultMatrixReadPanic (0.00s)
=== RUN   TestFaultMatrixReadPermissionDenied
m04-s07 matrix read-permission state=read fallback=none executions=1 chain=permission
--- PASS: TestFaultMatrixReadPermissionDenied (0.00s)
=== RUN   TestFaultMatrixReadTimeout
m04-s07 matrix read-timeout state=read fallback=core executions=2 chain=deadline
--- PASS: TestFaultMatrixReadTimeout (0.00s)
=== RUN   TestFaultMatrixWritePanic
m04-s07 matrix write-panic state=unknown fallback=none executions=1 chain=panic
--- PASS: TestFaultMatrixWritePanic (0.01s)
=== RUN   TestFaultMatrixWriteTimeout
m04-s07 matrix write-timeout state=unknown fallback=none executions=1 chain=deadline
--- PASS: TestFaultMatrixWriteTimeout (0.01s)
=== RUN   TestFaultMatrixLostAck
m04-s07 matrix lost-ack state=needs_reconciliation fallback=none executions=1 chain=deadline
--- PASS: TestFaultMatrixLostAck (0.01s)
=== RUN   TestFaultMatrixCrashAfterCommit
m04-s07 matrix crash-after-commit state=committed fallback=none executions=1 chain=none
--- PASS: TestFaultMatrixCrashAfterCommit (0.01s)
=== RUN   TestFaultMatrixDBBusy
m04-s07 matrix db-busy state=absent fallback=none executions=0 chain=busy
--- PASS: TestFaultMatrixDBBusy (0.01s)
=== RUN   TestFaultMatrixTruthTable
m04-s07 matrix-truth cases=8 complete=true
--- PASS: TestFaultMatrixTruthTable (0.00s)
PASS
ok  	github.com/u-ai/backend/internal/devicemesh/executionjournal	1.842s
`

const GUARD_SOURCE = GUARD_GUARDS.join('\n')
const WIRING_SOURCE = WIRING_GUARDS.join('\n')
const MATRIX_SOURCE = TABLE_GUARDS.join('\n')

function evidence(overrides = {}) {
  return {
    goFiles: ['effect_token.go', 'ledger.go', 'reconciliation.go', 'run_guard.go'],
    testFiles: ['fault_matrix_test.go', 'ledger_test.go'],
    rows: parseFaultLines(LOG),
    truth: parseTruthLine(LOG),
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

function fromLog(log) {
  return evidence({ log, rows: parseFaultLines(log), truth: parseTruthLine(log) })
}

test('constants pin the fault matrix surface', () => {
  assert.equal(EXIT_FAULT_MATRIX, 47)
  assert.equal(PACKAGE_PATH, './internal/devicemesh/executionjournal')
  assert.equal(RUN_PATTERN, 'TestFaultMatrix')
  assert.equal(EXPECTED_PASSED, 9)
  assert.deepEqual(REQUIRED_SOURCES, ['run_guard.go', 'ledger.go', 'reconciliation.go'])
  assert.deepEqual(REQUIRED_TESTS, ['fault_matrix_test.go'])
  assert.equal(MATRIX_CASES.length, 8)
  assert.equal(GUARD_GUARDS.length, 7)
  assert.equal(WIRING_GUARDS.length, 2)
  assert.equal(TABLE_GUARDS.length, 3)
})

test('matrix case names are the eight injection rows', () => {
  assert.deepEqual(MATRIX_CASES, [
    'read-panic',
    'read-permission',
    'read-timeout',
    'write-panic',
    'write-timeout',
    'lost-ack',
    'crash-after-commit',
    'db-busy'
  ])
})

test('parseFaultLines reads every row exactly once', () => {
  const rows = parseFaultLines(LOG)
  assert.equal(Object.keys(rows).length, 8)
  for (const name of MATRIX_CASES) {
    assert.equal(countCaseLines(LOG, name), 1)
    assert.ok(Array.isArray(rows[name]))
  }
  assert.deepEqual(rows['write-panic'], ['unknown', 'none', 1, 'panic'])
  assert.deepEqual(rows['db-busy'], ['absent', 'none', 0, 'busy'])
  assert.deepEqual(rows['lost-ack'], ['needs_reconciliation', 'none', 1, 'deadline'])
})

test('parseTruthLine reads the summary row', () => {
  assert.deepEqual(parseTruthLine(LOG), [8, 'true'])
  assert.equal(parseTruthLine('no summary here'), null)
})

test('baseline evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
  assert.deepEqual(validateGuardSource(GUARD_SOURCE), [])
  assert.deepEqual(validateWiringSource(WIRING_SOURCE), [])
  assert.deepEqual(validateTruthTable(MATRIX_SOURCE), [])
})

test('a duplicated row is rejected', () => {
  const log = `${LOG}m04-s07 matrix write-panic state=unknown fallback=none executions=1 chain=panic\n`
  const problems = validateEvidence(fromLog(log))
  assert.ok(problems.includes('evidence row write-panic must print exactly once'), problems.join('; '))
})

test('a missing row is rejected', () => {
  const log = LOG.replace('m04-s07 matrix db-busy state=absent fallback=none executions=0 chain=busy\n', '')
  const problems = validateEvidence(fromLog(log))
  assert.ok(problems.includes('evidence row db-busy is missing'), problems.join('; '))
})

for (const [name, column, wrong] of [
  ['write-panic', 0, 'committed'],
  ['write-panic', 1, 'core'],
  ['write-panic', 2, '2'],
  ['write-panic', 3, 'deadline'],
  ['read-panic', 1, 'none'],
  ['read-permission', 2, '2'],
  ['crash-after-commit', 0, 'pending'],
  ['db-busy', 2, '1']
]) {
  test(`column mutation ${name}#${column} is rejected`, () => {
    const row = parseFaultLines(LOG)[name]
    const cells = [`state=${row[0]}`, `fallback=${row[1]}`, `executions=${row[2]}`, `chain=${row[3]}`]
    const originalLine = `m04-s07 matrix ${name} ${cells.join(' ')}`
    assert.ok(LOG.includes(originalLine), originalLine)
    const mutatedCells = cells.slice()
    mutatedCells[column] = `${mutatedCells[column].split('=')[0]}=${wrong}`
    const mutatedLog = LOG.replace(originalLine, `m04-s07 matrix ${name} ${mutatedCells.join(' ')}`)
    const problems = validateEvidence(fromLog(mutatedLog))
    assert.ok(
      problems.some((problem) => problem.includes(`evidence row ${name} column ${column}`)),
      problems.join('; ')
    )
  })
}

for (const guard of GUARD_GUARDS) {
  test(`guard is enforced: ${guard}`, () => {
    const problems = validateGuardSource(GUARD_SOURCE.replace(guard, ''))
    assert.equal(problems.length, 1)
    assert.ok(problems[0].includes('run guard source is missing'))
  })
}

for (const guard of WIRING_GUARDS) {
  test(`wiring is enforced: ${guard}`, () => {
    const problems = validateWiringSource(WIRING_SOURCE.replace(guard, ''))
    assert.equal(problems.length, 1)
    assert.ok(problems[0].includes('source is missing'))
  })
}

for (const guard of TABLE_GUARDS) {
  test(`truth table row is frozen: ${guard}`, () => {
    const problems = validateTruthTable(MATRIX_SOURCE.replace(guard, ''))
    assert.equal(problems.length, 1)
    assert.ok(problems[0].includes('fault matrix source is missing'))
  })
}

test('production build must not leak test files', () => {
  const problems = validateEvidence(evidence({ goFiles: ['run_guard.go', 'ledger.go', 'reconciliation.go', 'fault_matrix_test.go'] }))
  assert.ok(problems.includes('default build leaked test files'), problems.join('; '))
})

test('missing sources and tests are rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['ledger.go', 'reconciliation.go'], testFiles: [] }))
  assert.ok(problems.includes('default build is missing run_guard.go'), problems.join('; '))
  assert.ok(problems.includes('test build is missing fault_matrix_test.go'), problems.join('; '))
})

test('truth summary must exist and match the case count', () => {
  assert.ok(validateEvidence(evidence({ truth: null })).includes('truth table summary is missing'))
  assert.ok(validateEvidence(evidence({ truth: [7, 'true'] })).includes('truth table cases=7 want 8'))
  assert.ok(validateEvidence(evidence({ truth: [8, 'false'] })).includes('truth table complete=false want true'))
})

test('run counts and flags are enforced', () => {
  assert.ok(validateEvidence(evidence({ passed: 8 })).includes('passed=8 want 9'))
  assert.ok(validateEvidence(evidence({ failed: 1 })).includes('failed=1 want 0'))
  assert.ok(validateEvidence(evidence({ exitCode: 1 })).includes('go test exit=1 want 0'))
  assert.ok(validateEvidence(evidence({ race: false })).includes('go test must run with -race'))
})

test('renderEvidence replays the frozen matrix', () => {
  const lines = renderEvidence(evidence()).trimEnd().split('\n')
  assert.equal(lines.length, 12)
  assert.equal(lines[0], 'invocation=fault-matrix package=./internal/devicemesh/executionjournal run=TestFaultMatrix cases=8 guards=12')
  assert.equal(lines[1], 'files=4 tests=2 sources=3/3 testExcluded=true')
  assert.equal(lines[2], 'truth rows=8 complete=true')
  assert.equal(lines[3], 'm04-s07 matrix read-panic state=read fallback=core executions=2 chain=panic')
  assert.equal(lines[9], 'm04-s07 matrix crash-after-commit state=committed fallback=none executions=1 chain=none')
  assert.equal(lines[11], 'passed=9 failed=0 exit=0 race=true')
})

test('parseGoTestOutput counts pass and fail lines', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 9, failed: 0 })
  assert.deepEqual(parseGoTestOutput('--- FAIL: TestX (0.00s)\n--- PASS: TestY (0.00s)\n'), { passed: 1, failed: 1 })
})

test('parseGoListFiles handles empty and populated lists', () => {
  assert.deepEqual(parseGoListFiles('[a.go b.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('not a list'), [])
})
