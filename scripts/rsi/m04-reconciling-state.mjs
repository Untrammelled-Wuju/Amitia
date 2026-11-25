import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestReconcilingState'
export const EXIT_RECONCILING_STATE = 49
export const EXPECTED_PASSED = 9
export const REQUIRED_SOURCES = ['reconciling_state.go']
export const REQUIRED_TESTS = ['reconciling_state_test.go']

export const STATE_GUARDS = [
  'type ReconcilingState string',
  'func (s ReconcilingState) AutoRetryAllowed() bool {',
  'func (s ReconcilingState) RequiresManualReview() bool {',
  'func (s ReconcilingState) NextAction() string {',
  'func (l *InvocationLedger) DescribeReconciliation(ctx context.Context, scope LedgerScope, idempotencyKey, operationID string) (ReconcileSnapshot, error) {',
  'entry, found, lookupErr := l.Lookup(ctx, scope, key)',
  'record, found, lookupErr := l.Reconciliation(ctx, scope, operation)',
  'snapshot.AutoRetry = state.AutoRetryAllowed()',
  'snapshot.ManualReview = state.RequiresManualReview()',
  'state = ReconcilingBlocked'
]

export const ASSERT_GUARDS = [
  'if !errors.Is(err, executionjournal.ErrLedgerEffectUnknown) {',
  'if executed != 0 {',
  'if before != after {',
  'strings.Contains(strings.ToLower(err.Error()), "readonly")'
]

export const WRITE_TOKENS = [
  'INSERT INTO',
  'UPDATE kernel_',
  'DELETE FROM',
  'DispatchIrreversible',
  'ReserveIrreversible',
  'ResolveReconciliation'
]

export const EVIDENCE_ROWS = [
  'state=absent ledger=none reconciliation=none auto=false next=none',
  'state=settled ledger=committed reconciliation=delivered auto=false next=none',
  'state=ambiguous ledger=unknown reconciliation=none auto=false next=manual_review resends=0',
  'state=ambiguous ledger=none reconciliation=needs_reconciliation auto=false next=manual_review',
  'state=retryable ledger=none reconciliation=not_delivered auto=true next=retry',
  'state=blocked ledger=committed reconciliation=not_delivered conflict=true auto=false next=manual_review',
  'state=in_flight ledger=pending reconciliation=none auto=false next=manual_review',
  'state=ambiguous readonly=true writes=0 unchanged=true rows=1',
  'invalid=3 cancelled=true scope=rejected'
]

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

export function parseStateRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m04-s09 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm04-s09 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateStateSource(text) {
  return missingGuards(text, STATE_GUARDS, 'reconciling state source')
}

export function validateWriteFree(text) {
  const body = String(text ?? '')
  return WRITE_TOKENS.filter((token) => body.includes(token)).map((token) => `reconciling state source must stay read-only but contains ${token}`)
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'reconciling state test')
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
  const rows = Array.isArray(evidence?.rows) ? evidence.rows : []
  if (rows.length !== EVIDENCE_ROWS.length) problems.push(`evidence rows=${rows.length} want ${EVIDENCE_ROWS.length}`)
  for (const expected of EVIDENCE_ROWS) {
    const count = rows.filter((row) => row === expected).length
    if (count !== 1) problems.push(`evidence row must print exactly once (${count}): ${expected}`)
  }
  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  if (evidence?.race !== true) problems.push('go test must run with -race')
  return problems
}

export function renderEvidence(evidence) {
  const goFiles = evidence?.goFiles ?? []
  const testFiles = evidence?.testFiles ?? []
  const rows = evidence?.rows ?? []
  const lines = [
    `invocation=reconciling-state package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${STATE_GUARDS.length + ASSERT_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m04-s09 ${row}`)
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

export function runReconcilingStateEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal')
  const source = readFileSync(resolve(base, 'reconciling_state.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'reconciling_state_test.go'), 'utf8')
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
    rows: parseStateRows(log),
    source,
    testSource,
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
      process.stdout.write(`invocation=reconciling-state package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runReconcilingStateEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateStateSource(evidence.source),
      ...validateWriteFree(evidence.source),
      ...validateAssertSource(evidence.testSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_RECONCILING_STATE)
    if (evidence.exitCode !== 0) process.exit(EXIT_RECONCILING_STATE)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_RECONCILING_STATE)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
