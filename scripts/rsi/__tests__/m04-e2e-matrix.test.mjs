import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { test } from 'node:test'
import {
  ASSERT_GUARDS,
  E2E_CASES,
  EXIT_E2E_MATRIX,
  EXPECTED_PASSED,
  EXPECTED_SKIPPED,
  INTEGRATION_FILES,
  INTEGRATION_GATES,
  PACKAGE_PATH,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  SINK_GUARDS,
  buildIntegration,
  countCaseLines,
  digestFiles,
  parseE2ERows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateAssertions,
  validateEvidence,
  validateIntegration,
  validateSinkSource
} from '../m04-e2e-matrix.mjs'

const LOG = `=== RUN   TestE2ESinkHelperProcess
--- SKIP: TestE2ESinkHelperProcess (0.00s)
=== RUN   TestE2ESinkNoDuplicateWrites
m04-s08 e2e no-duplicate ops=100 executed=100 blocked=100 sink_writes=100 unique=100 duplicates=0
--- PASS: TestE2ESinkNoDuplicateWrites (1.00s)
=== RUN   TestE2ESinkConcurrentSameKey
m04-s08 e2e concurrent workers=32 executed=1 sink_writes=1 unique=1
--- PASS: TestE2ESinkConcurrentSameKey (0.14s)
=== RUN   TestE2EIrreversibleNoResend
m04-s08 e2e irreversible state=needs_reconciliation sink_writes=1 unique=1 blocked=true
--- PASS: TestE2EIrreversibleNoResend (0.07s)
=== RUN   TestE2EDefaultCoreAvailable
m04-s08 e2e default-core provider=builtin.core/core/memory.search catalog=1 permissions=memory.read/low available=true
--- PASS: TestE2EDefaultCoreAvailable (0.00s)
=== RUN   TestE2ERestartKeepsUnknownBlocked
m04-s08 e2e restart ops=20 blocked=20 sink_writes=20 unique=20
--- PASS: TestE2ERestartKeepsUnknownBlocked (0.25s)
=== RUN   TestE2ECancelledWriteNoEffect
m04-s08 e2e cancelled ran=false rows=0 sink_writes=0
--- PASS: TestE2ECancelledWriteNoEffect (0.06s)
PASS
ok  	github.com/u-ai/backend/internal/devicemesh/executionjournal	3.288s
`

const SINK_SOURCE = SINK_GUARDS.join('\n') + '\n' + ASSERT_GUARDS.join('\n')

const ROWS = {
  'no-duplicate': [100, 100, 100, 100, 100, 0],
  concurrent: [32, 1, 1, 1],
  irreversible: ['needs_reconciliation', 1, 1, 'true'],
  'default-core': ['builtin.core/core/memory.search', 1, 'memory.read/low', 'true'],
  restart: [20, 20, 20, 20],
  cancelled: ['false', 0, 0]
}

function evidence(overrides = {}) {
  return {
    goFiles: [...REQUIRED_SOURCES],
    testFiles: [...REQUIRED_TESTS],
    rows: parseE2ERows(LOG),
    sinkSource: SINK_SOURCE,
    passed: EXPECTED_PASSED,
    failed: 0,
    skipped: EXPECTED_SKIPPED,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

function rowLine(name, values) {
  switch (name) {
    case 'no-duplicate':
      return `m04-s08 e2e no-duplicate ops=${values[0]} executed=${values[1]} blocked=${values[2]} sink_writes=${values[3]} unique=${values[4]} duplicates=${values[5]}`
    case 'concurrent':
      return `m04-s08 e2e concurrent workers=${values[0]} executed=${values[1]} sink_writes=${values[2]} unique=${values[3]}`
    case 'irreversible':
      return `m04-s08 e2e irreversible state=${values[0]} sink_writes=${values[1]} unique=${values[2]} blocked=${values[3]}`
    case 'default-core':
      return `m04-s08 e2e default-core provider=${values[0]} catalog=${values[1]} permissions=${values[2]} available=${values[3]}`
    case 'restart':
      return `m04-s08 e2e restart ops=${values[0]} blocked=${values[1]} sink_writes=${values[2]} unique=${values[3]}`
    default:
      return `m04-s08 e2e cancelled ran=${values[0]} rows=${values[1]} sink_writes=${values[2]}`
  }
}

function fromLog(log) {
  return evidence({ log, rows: parseE2ERows(log) })
}

test('constants pin the e2e matrix surface', () => {
  assert.equal(PACKAGE_PATH, './internal/devicemesh/executionjournal')
  assert.equal(RUN_PATTERN, 'TestE2E')
  assert.equal(EXIT_E2E_MATRIX, 48)
  assert.equal(EXPECTED_PASSED, 6)
  assert.equal(EXPECTED_SKIPPED, 1)
  assert.deepEqual(REQUIRED_SOURCES, ['ledger.go', 'effect_token.go', 'reconciliation.go', 'run_guard.go'])
  assert.deepEqual(REQUIRED_TESTS, ['e2e_sink_test.go'])
  assert.equal(E2E_CASES.length, 6)
  assert.equal(SINK_GUARDS.length, 6)
  assert.equal(ASSERT_GUARDS.length, 3)
})

test('e2e case names are the six acceptance rows', () => {
  assert.deepEqual(E2E_CASES, ['no-duplicate', 'concurrent', 'irreversible', 'default-core', 'restart', 'cancelled'])
})

test('parseE2ERows reads every row exactly once', () => {
  const rows = parseE2ERows(LOG)
  assert.equal(Object.keys(rows).length, 6)
  for (const name of E2E_CASES) {
    assert.equal(countCaseLines(LOG, name), 1)
    assert.deepEqual(rows[name], ROWS[name])
  }
})

test('baseline evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
  assert.deepEqual(validateSinkSource(SINK_SOURCE), [])
  assert.deepEqual(validateAssertions(SINK_SOURCE), [])
})

test('a duplicated row is rejected', () => {
  const log = `${LOG}${rowLine('no-duplicate', ROWS['no-duplicate'])}\n`
  const problems = validateEvidence(fromLog(log))
  assert.ok(problems.includes('evidence row no-duplicate must print exactly once'), problems.join('; '))
})

test('a missing row is rejected', () => {
  const log = LOG.replace(`${rowLine('restart', ROWS.restart)}\n`, '')
  const problems = validateEvidence(fromLog(log))
  assert.ok(problems.includes('evidence row restart is missing'), problems.join('; '))
})

for (const [name, column, wrong] of [
  ['no-duplicate', 5, '1'],
  ['no-duplicate', 1, '99'],
  ['no-duplicate', 4, '98'],
  ['concurrent', 1, '2'],
  ['concurrent', 2, '2'],
  ['irreversible', 0, 'delivered'],
  ['irreversible', 3, 'false'],
  ['default-core', 0, 'builtin.core/core/memory.other'],
  ['default-core', 2, 'memory.write/high'],
  ['restart', 1, '19'],
  ['cancelled', 0, 'true'],
  ['cancelled', 2, '1']
]) {
  test(`column mutation ${name}#${column} is rejected`, () => {
    const values = ROWS[name].slice()
    const keys = {
      'no-duplicate': ['ops', 'executed', 'blocked', 'sink_writes', 'unique', 'duplicates'],
      concurrent: ['workers', 'executed', 'sink_writes', 'unique'],
      irreversible: ['state', 'sink_writes', 'unique', 'blocked'],
      'default-core': ['provider', 'catalog', 'permissions', 'available'],
      restart: ['ops', 'blocked', 'sink_writes', 'unique'],
      cancelled: ['ran', 'rows', 'sink_writes']
    }[name]
    const original = rowLine(name, values)
    assert.ok(LOG.includes(original), original)
    values[column] = wrong
    const mutated = rowLine(name, values)
    assert.notEqual(mutated, original)
    assert.ok(mutated.includes(`${keys[column]}=${wrong}`), mutated)
    const problems = validateEvidence(fromLog(LOG.replace(original, mutated)))
    assert.ok(
      problems.some((problem) => problem.includes(`evidence row ${name} column ${column}`)),
      problems.join('; ')
    )
  })
}

for (const guard of SINK_GUARDS) {
  test(`sink guard is enforced: ${guard}`, () => {
    const problems = validateSinkSource(SINK_SOURCE.replace(guard, ''))
    assert.equal(problems.length, 1)
    assert.ok(problems[0].includes('sink e2e source is missing'))
  })
}

for (const guard of ASSERT_GUARDS) {
  test(`no-duplicate assertion is enforced: ${guard}`, () => {
    const problems = validateAssertions(SINK_SOURCE.replace(guard, ''))
    assert.equal(problems.length, 1)
    assert.ok(problems[0].includes('sink e2e source is missing'))
  })
}

test('production build must not leak test files', () => {
  const problems = validateEvidence(evidence({ goFiles: [...REQUIRED_SOURCES, 'e2e_sink_test.go'] }))
  assert.ok(problems.includes('default build leaked test files'), problems.join('; '))
})

test('missing sources and tests are rejected', () => {
  const problems = validateEvidence(evidence({ goFiles: ['ledger.go'], testFiles: [] }))
  assert.ok(problems.includes('default build is missing run_guard.go'), problems.join('; '))
  assert.ok(problems.includes('test build is missing e2e_sink_test.go'), problems.join('; '))
})

test('run counts and flags are enforced', () => {
  const problems = validateEvidence(evidence({ passed: 5, failed: 1, skipped: 0, exitCode: 1, race: false }))
  assert.ok(problems.includes('passed=5 want 6'), problems.join('; '))
  assert.ok(problems.includes('failed=1 want 0'), problems.join('; '))
  assert.ok(problems.includes('skipped=0 want 1'), problems.join('; '))
  assert.ok(problems.includes('go test exit=1 want 0'), problems.join('; '))
  assert.ok(problems.includes('go test must run with -race'), problems.join('; '))
})

test('renderEvidence replays the frozen matrix', () => {
  const lines = renderEvidence(evidence()).trimEnd().split('\n')
  assert.equal(lines.length, 10)
  assert.ok(lines[0].includes('cases=6'))
  const replay = parseE2ERows(lines.slice(3, 9).join('\n'))
  assert.deepEqual(replay, ROWS)
  assert.ok(lines[9].includes('passed=6 failed=0 skipped=1 exit=0 race=true'))
})

test('parseGoTestOutput counts pass, fail and skip lines', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 6, failed: 0, skipped: 1 })
  assert.deepEqual(parseGoTestOutput(''), { passed: 0, failed: 0, skipped: 0 })
})

test('parseGoListFiles handles empty and populated lists', () => {
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('not a list'), [])
})

function sandbox() {
  const root = mkdtempSync(join(tmpdir(), 'm04-s08-'))
  for (const path of INTEGRATION_FILES) {
    const target = resolve(root, path)
    mkdirSync(dirname(target), { recursive: true })
    writeFileSync(target, `content:${path}\n`)
  }
  return root
}

test('integration package verifies against a clean tree', () => {
  const root = sandbox()
  try {
    const manifest = buildIntegration(evidence(), root)
    assert.equal(manifest.files.length, INTEGRATION_FILES.length)
    assert.equal(manifest.gates.length, INTEGRATION_GATES.length)
    assert.equal(manifest.verdict, 'READY_FOR_APPROVAL')
    assert.deepEqual(validateIntegration(manifest, root), [])
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('integration detects recorded file drift', () => {
  const root = sandbox()
  try {
    const manifest = buildIntegration(evidence(), root)
    writeFileSync(resolve(root, INTEGRATION_FILES[0]), 'tampered\n')
    const problems = validateIntegration(manifest, root)
    assert.ok(problems.includes(`integration sha256 drift on ${INTEGRATION_FILES[0]}`), problems.join('; '))
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('integration detects missing files, gates and cases', () => {
  const root = sandbox()
  try {
    const manifest = buildIntegration(evidence(), root)
    rmSync(resolve(root, INTEGRATION_FILES[1]))
    const stripped = {
      ...manifest,
      gates: manifest.gates.filter((gate) => gate.id !== 'FAULT_MATRIX'),
      cases: manifest.cases.filter((name) => name !== 'restart')
    }
    const problems = validateIntegration(stripped, root)
    assert.ok(problems.includes(`integrated file is missing ${INTEGRATION_FILES[1]}`), problems.join('; '))
    assert.ok(problems.includes('integration is missing gate FAULT_MATRIX'), problems.join('; '))
    assert.ok(problems.includes('integration is missing case restart'), problems.join('; '))
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('integration rejects a wrong step or verdict', () => {
  const root = sandbox()
  try {
    const manifest = buildIntegration(evidence(), root)
    const problems = validateIntegration({ ...manifest, step: 'M04-S07', verdict: 'PENDING' }, root)
    assert.ok(problems.includes('step=M04-S07 want M04-S08'), problems.join('; '))
    assert.ok(problems.includes('verdict=PENDING want READY_FOR_APPROVAL'), problems.join('; '))
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('digestFiles marks absent files as missing', () => {
  const root = mkdtempSync(join(tmpdir(), 'm04-s08-empty-'))
  try {
    const [entry] = digestFiles(root, ['scripts/rsi/m04-e2e-matrix.mjs'])
    assert.equal(entry.missing, true)
    assert.equal(entry.sha256, '')
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
