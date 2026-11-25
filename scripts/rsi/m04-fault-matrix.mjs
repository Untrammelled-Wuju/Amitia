import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestFaultMatrix'
export const EXIT_FAULT_MATRIX = 47
export const EXPECTED_PASSED = 9
export const REQUIRED_SOURCES = ['run_guard.go', 'ledger.go', 'reconciliation.go']
export const REQUIRED_TESTS = ['fault_matrix_test.go']
export const MATRIX_CASES = [
  'read-panic',
  'read-permission',
  'read-timeout',
  'write-panic',
  'write-timeout',
  'lost-ack',
  'crash-after-commit',
  'db-busy'
]
export const GUARD_GUARDS = [
  'type RunPanicError struct',
  'func (e *RunPanicError) Unwrap() error { return execution.ErrInvocationTransportLost }',
  'func guardRun(run func() (json.RawMessage, error)) (result json.RawMessage, err error)',
  'func guardDispatch(dispatch func() error) (err error)',
  'recovered := recover()',
  'err = &RunPanicError{Value: fmt.Sprint(recovered), Stack: string(debug.Stack())}',
  'runtime/debug'
]
export const WIRING_GUARDS = ['result, runErr := guardRun(run)', 'dispatchErr := guardDispatch(dispatch)']
export const TABLE_GUARDS = [
  '{Name: "write-panic", State: "unknown", Fallback: "none", Executions: 1, Chain: "panic"}',
  '{Name: "lost-ack", State: "needs_reconciliation", Fallback: "none", Executions: 1, Chain: "deadline"}',
  '{Name: "db-busy", State: "absent", Fallback: "none", Executions: 0, Chain: "busy"}'
]

const MATRIX_PATTERN = /m04-s07 matrix (\S+) state=(\w+) fallback=(\w+) executions=(\d+) chain=(\w+)/
const TRUTH_PATTERN = /m04-s07 matrix-truth cases=(\d+) complete=(true|false)/

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function parseGoListFiles(output) {
  const body = String(output ?? '').trim()
  if (!body.startsWith('[') || !body.endsWith(']')) return []
  const inner = body.slice(1, -1).trim()
  if (inner === '') return []
  return inner.split(/\s+/).sort()
}

export function parseFaultLines(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = {}
  for (const line of body.split('\n')) {
    const match = line.match(MATRIX_PATTERN)
    if (!match) continue
    const [, name, state, fallback, executions, chain] = match
    if (!rows[name]) rows[name] = [state, fallback, Number(executions), chain]
  }
  return rows
}

export function parseTruthLine(log) {
  const body = typeof log === 'string' ? log : ''
  const match = body.match(TRUTH_PATTERN)
  if (!match) return null
  return [Number(match[1]), match[2]]
}

export function countCaseLines(log, name) {
  const body = typeof log === 'string' ? log : ''
  const pattern = new RegExp(`m04-s07 matrix ${name.replace(/[-/\\^$*+?.()|[\]{}]/g, '\\$&')} `, 'g')
  return (body.match(pattern) ?? []).length
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateGuardSource(text) {
  return missingGuards(text, GUARD_GUARDS, 'run guard source')
}

export function validateWiringSource(text) {
  return [
    ...missingGuards(text, WIRING_GUARDS.slice(0, 1), 'ledger source'),
    ...missingGuards(text, WIRING_GUARDS.slice(1), 'reconciliation source')
  ]
}

export function validateTruthTable(text) {
  return missingGuards(text, TABLE_GUARDS, 'fault matrix source')
}

function expectRow(problems, rows, name, expected) {
  const row = rows[name]
  if (!Array.isArray(row)) {
    problems.push(`evidence row ${name} is missing`)
    return
  }
  expected.forEach((value, index) => {
    if (row[index] !== value) problems.push(`evidence row ${name} column ${index} is ${row[index]} want ${value}`)
  })
}

export function validateEvidence(evidence) {
  const problems = []
  const goFiles = Array.isArray(evidence?.goFiles) ? evidence.goFiles : []
  const testFiles = Array.isArray(evidence?.testFiles) ? evidence.testFiles : []
  for (const source of REQUIRED_SOURCES) {
    if (!goFiles.includes(source)) problems.push(`default build is missing ${source}`)
  }
  if (goFiles.some((file) => file.endsWith('_test.go'))) problems.push('default build leaked test files')
  for (const test of REQUIRED_TESTS) {
    if (!testFiles.includes(test)) problems.push(`test build is missing ${test}`)
  }

  const rows = evidence?.rows ?? {}
  const log = typeof evidence?.log === 'string' ? evidence.log : ''
  for (const name of MATRIX_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countCaseLines(log, name) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  expectRow(problems, rows, 'read-panic', ['read', 'core', 2, 'panic'])
  expectRow(problems, rows, 'read-permission', ['read', 'none', 1, 'permission'])
  expectRow(problems, rows, 'read-timeout', ['read', 'core', 2, 'deadline'])
  expectRow(problems, rows, 'write-panic', ['unknown', 'none', 1, 'panic'])
  expectRow(problems, rows, 'write-timeout', ['unknown', 'none', 1, 'deadline'])
  expectRow(problems, rows, 'lost-ack', ['needs_reconciliation', 'none', 1, 'deadline'])
  expectRow(problems, rows, 'crash-after-commit', ['committed', 'none', 1, 'none'])
  expectRow(problems, rows, 'db-busy', ['absent', 'none', 0, 'busy'])

  const truth = evidence?.truth
  if (!Array.isArray(truth)) problems.push('truth table summary is missing')
  else {
    if (truth[0] !== MATRIX_CASES.length) problems.push(`truth table cases=${truth[0]} want ${MATRIX_CASES.length}`)
    if (truth[1] !== 'true') problems.push(`truth table complete=${truth[1]} want true`)
  }

  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  if (evidence?.race !== true) problems.push('go test must run with -race')
  return problems
}

export function renderEvidence(evidence) {
  const rows = evidence?.rows ?? {}
  const value = (name, index, fallback = 0) => {
    const row = rows[name]
    return Array.isArray(row) ? row[index] : fallback
  }
  const goFiles = evidence?.goFiles ?? []
  const testFiles = evidence?.testFiles ?? []
  const truth = Array.isArray(evidence?.truth) ? evidence.truth : [0, 'false']
  const lines = [
    `invocation=fault-matrix package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${MATRIX_CASES.length} guards=${GUARD_GUARDS.length + WIRING_GUARDS.length + TABLE_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `truth rows=${truth[0]} complete=${truth[1]}`
  ]
  for (const name of MATRIX_CASES) {
    lines.push(`m04-s07 matrix ${name} state=${value(name, 0, '')} fallback=${value(name, 1, '')} executions=${value(name, 2)} chain=${value(name, 3, '')}`)
  }
  lines.push(`passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`)
  return lines.join('\n') + '\n'
}

function goList(root) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}|{{.XTestGoFiles}}', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 120000
  })
  const line = `${result.stdout ?? ''}`.trim()
  const [files = '', tests = '', external = ''] = line.split('|')
  return {
    files: parseGoListFiles(files),
    tests: [...parseGoListFiles(tests), ...parseGoListFiles(external)].sort()
  }
}

export function runFaultMatrixEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal')
  const guardSource = readFileSync(resolve(base, 'run_guard.go'), 'utf8')
  const ledgerSource = readFileSync(resolve(base, 'ledger.go'), 'utf8')
  const reconciliationSource = readFileSync(resolve(base, 'reconciliation.go'), 'utf8')
  const matrixSource = readFileSync(resolve(base, 'fault_matrix_test.go'), 'utf8')
  const result = spawnSync('go', ['test', '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 600000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    goFiles: listed.files,
    testFiles: listed.tests,
    rows: parseFaultLines(log),
    truth: parseTruthLine(log),
    guardSource,
    ledgerSource,
    reconciliationSource,
    matrixSource,
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: true,
    log
  }
}

function reportProblems(problems) {
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  return problems.length > 0
}

function main() {
  try {
    if (process.argv.includes('--list')) {
      process.stdout.write(`invocation=fault-matrix package=${PACKAGE_PATH} cases=${MATRIX_CASES.join(',')}\n`)
      return
    }
    const evidence = runFaultMatrixEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateGuardSource(evidence.guardSource),
      ...validateWiringSource(`${evidence.ledgerSource}\n${evidence.reconciliationSource}`),
      ...validateTruthTable(evidence.matrixSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_FAULT_MATRIX)
    if (evidence.exitCode !== 0) process.exit(EXIT_FAULT_MATRIX)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_FAULT_MATRIX)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
