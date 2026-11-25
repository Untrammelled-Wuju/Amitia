import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestReconcile'
export const EXIT_RECONCILIATION = 45
export const EXPECTED_PASSED = 13
export const REQUIRED_SOURCES = ['reconciliation.go', 'ledger.go']
export const REQUIRED_TESTS = ['reconciliation_test.go']
export const RECONCILE_CASES = [
  'ack-lost',
  'restart',
  'crash-recovery',
  'resolved-delivered',
  'resolved-not-sent',
  'double-resolve',
  'resolve-guard',
  'definite-failure',
  'open-list',
  'concurrent',
  'isolation',
  'error-verbatim',
  'invalid-requests'
]
export const RECONCILE_GUARDS = [
  'ErrReconciliationPending',
  'ErrReconciliationDelivered',
  'ErrReconciliationResolved',
  'ErrReconciliationVerdictInvalid',
  "CHECK(state IN ('attempting','needs_reconciliation','delivered','not_delivered'))",
  'kernel_irreversible_reconciliation',
  'execution.IsAmbiguousInvocationFailure',
  'bookkeeping := context.WithoutCancel(ctx)\n\tscopeKey, _ := request.Scope.Key()',
  'RecoverInterrupted',
  'VerdictNotSent',
  'RetryAllowed()'
]
export const LEDGER_GUARDS = ['db.Exec(ReconciliationSchema)']

const ACK_LOST_PATTERN = /m04-s05 reconcile ack-lost dispatches=(\d+) state=(\w+) retryBlocked=(true|false)/
const RESTART_PATTERN = /m04-s05 reconcile restart visible=(true|false) state=(\w+) attempts=(\d+) error=(true|false)/
const CRASH_PATTERN = /m04-s05 reconcile crash-recovery recovered=(\d+) state=(\w+)/
const DELIVERED_PATTERN = /m04-s05 reconcile resolved-delivered resolved=(true|false) retryBlocked=(true|false) dispatches=(\d+)/
const NOT_SENT_PATTERN = /m04-s05 reconcile resolved-not-sent retryAllowed=(true|false) attempts=(\d+) state=(\w+)/
const DOUBLE_PATTERN = /m04-s05 reconcile double-resolve rejected=(true|false) state=(\w+)/
const GUARD_PATTERN = /m04-s05 reconcile resolve-guard inflightBlocked=(true|false) missingReported=(true|false)/
const DEFINITE_PATTERN = /m04-s05 reconcile definite-failure state=(\w+) retryAllowed=(true|false) dispatches=(\d+)/
const OPEN_LIST_PATTERN = /m04-s05 reconcile open-list open=(\d+) scoped=(\d+)/
const CONCURRENT_PATTERN = /m04-s05 reconcile concurrent workers=(\d+) dispatches=(\d+) leaders=(\d+) blocked=(\d+)/
const ISOLATION_PATTERN = /m04-s05 reconcile isolation ledger_rows=(\d+) effect_rows=(\d+) reconciliation_rows=(\d+)/
const VERBATIM_PATTERN = /m04-s05 reconcile error-verbatim bytes=(\d+) preserved=(true|false)/
const INVALID_PATTERN = /m04-s05 reconcile invalid-requests rejected=(\d+) dispatched=(\d+) verdicts=(\d+)/

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

function parseNamed(text, pattern) {
  const match = text.match(pattern)
  if (!match) return null
  return match.slice(1).map((value) => (/^\d+$/.test(value) ? Number(value) : value))
}

export function parseReconcileRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    'ack-lost': parseNamed(body, ACK_LOST_PATTERN),
    restart: parseNamed(body, RESTART_PATTERN),
    'crash-recovery': parseNamed(body, CRASH_PATTERN),
    'resolved-delivered': parseNamed(body, DELIVERED_PATTERN),
    'resolved-not-sent': parseNamed(body, NOT_SENT_PATTERN),
    'double-resolve': parseNamed(body, DOUBLE_PATTERN),
    'resolve-guard': parseNamed(body, GUARD_PATTERN),
    'definite-failure': parseNamed(body, DEFINITE_PATTERN),
    'open-list': parseNamed(body, OPEN_LIST_PATTERN),
    concurrent: parseNamed(body, CONCURRENT_PATTERN),
    isolation: parseNamed(body, ISOLATION_PATTERN),
    'error-verbatim': parseNamed(body, VERBATIM_PATTERN),
    'invalid-requests': parseNamed(body, INVALID_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'ack-lost':
      return ACK_LOST_PATTERN
    case 'restart':
      return RESTART_PATTERN
    case 'crash-recovery':
      return CRASH_PATTERN
    case 'resolved-delivered':
      return DELIVERED_PATTERN
    case 'resolved-not-sent':
      return NOT_SENT_PATTERN
    case 'double-resolve':
      return DOUBLE_PATTERN
    case 'resolve-guard':
      return GUARD_PATTERN
    case 'definite-failure':
      return DEFINITE_PATTERN
    case 'open-list':
      return OPEN_LIST_PATTERN
    case 'concurrent':
      return CONCURRENT_PATTERN
    case 'isolation':
      return ISOLATION_PATTERN
    case 'error-verbatim':
      return VERBATIM_PATTERN
    default:
      return INVALID_PATTERN
  }
}

function countMatches(text, pattern) {
  return (text.match(new RegExp(pattern.source, 'g')) ?? []).length
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateReconcileSource(text) {
  return missingGuards(text, RECONCILE_GUARDS, 'reconciliation source')
}

export function validateLedgerSource(text) {
  return missingGuards(text, LEDGER_GUARDS, 'ledger source')
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
  for (const name of RECONCILE_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  expectRow(problems, rows, 'ack-lost', [1, 'needs_reconciliation', 'true'])
  expectRow(problems, rows, 'restart', ['true', 'needs_reconciliation', 1, 'true'])
  expectRow(problems, rows, 'crash-recovery', [1, 'needs_reconciliation'])
  expectRow(problems, rows, 'resolved-delivered', ['true', 'true', 1])
  expectRow(problems, rows, 'resolved-not-sent', ['true', 2, 'delivered'])
  expectRow(problems, rows, 'double-resolve', ['true', 'delivered'])
  expectRow(problems, rows, 'resolve-guard', ['true', 'true'])
  expectRow(problems, rows, 'definite-failure', ['not_delivered', 'true', 2])
  expectRow(problems, rows, 'open-list', [2, 1])
  expectRow(problems, rows, 'concurrent', [8, 1, 1, 7])
  expectRow(problems, rows, 'isolation', [0, 0, 1])
  expectRow(problems, rows, 'error-verbatim', [137, 'true'])
  expectRow(problems, rows, 'invalid-requests', [9, 0, 3])

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
  const source = evidence?.source ?? ''
  return [
    `invocation=reconciliation package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${RECONCILE_CASES.length} guards=${RECONCILE_GUARDS.length + LEDGER_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `reconciliation guards frozen=${RECONCILE_GUARDS.every((guard) => source.includes(guard))} ledger guards frozen=${LEDGER_GUARDS.every((guard) => source.includes(guard))}`,
    `m04-s05 reconcile ack-lost dispatches=${value('ack-lost', 0)} state=${value('ack-lost', 1, '')} retryBlocked=${value('ack-lost', 2, 'false')}`,
    `m04-s05 reconcile restart visible=${value('restart', 0, 'false')} state=${value('restart', 1, '')} attempts=${value('restart', 2)} error=${value('restart', 3, 'false')}`,
    `m04-s05 reconcile crash-recovery recovered=${value('crash-recovery', 0)} state=${value('crash-recovery', 1, '')}`,
    `m04-s05 reconcile resolved-delivered resolved=${value('resolved-delivered', 0, 'false')} retryBlocked=${value('resolved-delivered', 1, 'false')} dispatches=${value('resolved-delivered', 2)}`,
    `m04-s05 reconcile resolved-not-sent retryAllowed=${value('resolved-not-sent', 0, 'false')} attempts=${value('resolved-not-sent', 1)} state=${value('resolved-not-sent', 2, '')}`,
    `m04-s05 reconcile double-resolve rejected=${value('double-resolve', 0, 'false')} state=${value('double-resolve', 1, '')}`,
    `m04-s05 reconcile resolve-guard inflightBlocked=${value('resolve-guard', 0, 'false')} missingReported=${value('resolve-guard', 1, 'false')}`,
    `m04-s05 reconcile definite-failure state=${value('definite-failure', 0, '')} retryAllowed=${value('definite-failure', 1, 'false')} dispatches=${value('definite-failure', 2)}`,
    `m04-s05 reconcile open-list open=${value('open-list', 0)} scoped=${value('open-list', 1)}`,
    `m04-s05 reconcile concurrent workers=${value('concurrent', 0)} dispatches=${value('concurrent', 1)} leaders=${value('concurrent', 2)} blocked=${value('concurrent', 3)}`,
    `m04-s05 reconcile isolation ledger_rows=${value('isolation', 0)} effect_rows=${value('isolation', 1)} reconciliation_rows=${value('isolation', 2)}`,
    `m04-s05 reconcile error-verbatim bytes=${value('error-verbatim', 0)} preserved=${value('error-verbatim', 1, 'false')}`,
    `m04-s05 reconcile invalid-requests rejected=${value('invalid-requests', 0)} dispatched=${value('invalid-requests', 1)} verdicts=${value('invalid-requests', 2)}`,
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`
  ].join('\n') + '\n'
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

export function runReconciliation(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal')
  const source = [readFileSync(resolve(base, 'reconciliation.go'), 'utf8'), readFileSync(resolve(base, 'ledger.go'), 'utf8')].join('\n')
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
    rows: parseReconcileRows(log),
    source,
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
      process.stdout.write(`invocation=reconciliation package=${PACKAGE_PATH} cases=${RECONCILE_CASES.join(',')}\n`)
      return
    }
    const evidence = runReconciliation(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [...validateReconcileSource(evidence.source), ...validateLedgerSource(evidence.source), ...validateEvidence(evidence)]
    if (reportProblems(problems)) process.exit(EXIT_RECONCILIATION)
    if (evidence.exitCode !== 0) process.exit(EXIT_RECONCILIATION)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_RECONCILIATION)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
