import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestJournalRecovery'
export const EXIT_RECOVERY = 52
export const EXPECTED_PASSED = 8
export const REQUIRED_SOURCES = ['ledger.go', 'recovery.go']
export const REQUIRED_TESTS = ['recovery_test.go']

export const RECOVERY_GUARDS = [
  'var ErrRecoveryRequired = errors.New(',
  'type RecoveryDecision string',
  'func (r RecoveryRecord) Reexecute() bool {',
  'return false',
  'func (r RecoveryReport) ManualOnly() bool {',
  'func (l *InvocationLedger) RecoverInFlight(ctx context.Context, scope LedgerScope, staleAfter time.Duration) (RecoveryReport, error) {',
  'func (l *InvocationLedger) InspectRecovery(ctx context.Context, scope LedgerScope, idempotencyKey string) (RecoveryRecord, bool, error) {'
]

export const SCAN_GUARDS = [
  "effect_state IN ('pending','unknown')",
  'ORDER BY idempotency_key',
  'if staleAfter > 0 {',
  'cutoff = cutoff.Add(-staleAfter)'
]

export const COMMIT_GUARDS = [
  'recorded, err := recoveryRecorded(ctx, tx, scopeKey, key)',
  'return LedgerEntry{}, false, fmt.Errorf("%w: %s", ErrRecoveryRequired, entry.Binding())'
]

export const SCHEMA_GUARDS = [
  'const RecoverySchema = `CREATE TABLE IF NOT EXISTS kernel_invocation_recovery',
  "decision TEXT NOT NULL CHECK(decision IN ('reconcile'))",
  'const RecoveryImmutableSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_recovery_immutable',
  'BEFORE UPDATE ON kernel_invocation_recovery',
  'const RecoveryRemovalSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_recovery_removal',
  'BEFORE DELETE ON kernel_invocation_recovery',
  'func IsRecoveryImmutableError(err error) bool {'
]

export const INSTALL_GUARDS = [
  'db.Exec(RecoverySchema)',
  'db.Exec(RecoveryImmutableSchema)',
  'db.Exec(RecoveryRemovalSchema)'
]

export const ASSERT_GUARDS = [
  'if !errors.Is(err, executionjournal.ErrRecoveryRequired) {',
  'if executions.Load() != 0 {',
  'if record.Reexecute() {',
  'if !report.ManualOnly() {',
  'if len(first.Recovered) != 1 || len(second.Recovered) != 0 || second.Previously != 1 {',
  'if !executionjournal.IsRecoveryImmutableError(updateErr) || !executionjournal.IsRecoveryImmutableError(deleteErr) {',
  'if stored.StateBefore != executionjournal.EffectPending || stored.InvocationID != "inv-1" {'
]

export const EVIDENCE_ROWS = [
  'scanned=1 recovered=1 decision=reconcile reexecute=false manual_only=true state_before=pending',
  'scanned=1 recovered=1 state_before=unknown decision=reconcile',
  'settled=2 scanned=0 recovered=0 committed_state=committed',
  'workers=2 first_recovered=1 second_recovered=0 previously=1 rows=1',
  'commit_after_recovery=blocked executions=0 state=pending replayed=false',
  'restarted=true recovery_persisted=true commit=blocked executions=0 previously=1',
  'update=blocked delete=blocked rows=1',
  'empty_scope=rejected empty_key=rejected'
]

export const INSPECT_WRITE_TOKENS = ['UPDATE kernel_invocation_recovery', 'DELETE FROM kernel_invocation_recovery']

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

export function parseRecoveryRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m04-s12 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm04-s12 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateRecoverySource(text) {
  return missingGuards(text, RECOVERY_GUARDS, 'recovery source')
}

export function validateScanSource(text) {
  return missingGuards(text, SCAN_GUARDS, 'recovery scan')
}

export function validateCommitGuard(text) {
  return missingGuards(text, COMMIT_GUARDS, 'ledger commit source')
}

export function validateSchemaSource(text) {
  return missingGuards(text, SCHEMA_GUARDS, 'recovery schema source')
}

export function validateInstallSource(text) {
  return missingGuards(text, INSTALL_GUARDS, 'ledger install source')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'recovery test')
}

export function validateInspectReadOnly(text) {
  const body = String(text ?? '')
  const start = body.indexOf('func (l *InvocationLedger) InspectRecovery')
  if (start < 0) return ['recovery source is missing InspectRecovery']
  const end = body.indexOf('\nfunc ', start + 1)
  const scope = end < 0 ? body.slice(start) : body.slice(start, end)
  return INSPECT_WRITE_TOKENS.filter((token) => scope.includes(token)).map((token) => `recovery inspection must stay read-only but contains ${token}`)
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
  const guards = RECOVERY_GUARDS.length + SCAN_GUARDS.length + COMMIT_GUARDS.length + SCHEMA_GUARDS.length + INSTALL_GUARDS.length + ASSERT_GUARDS.length
  const lines = [
    `invocation=interrupted-recovery package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m04-s12 ${row}`)
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

export function runRecoveryEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal')
  const recoverySource = readFileSync(resolve(base, 'recovery.go'), 'utf8')
  const ledgerSource = readFileSync(resolve(base, 'ledger.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'recovery_test.go'), 'utf8')
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
    rows: parseRecoveryRows(log),
    recoverySource,
    ledgerSource,
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
      process.stdout.write(`invocation=interrupted-recovery package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runRecoveryEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateRecoverySource(evidence.recoverySource),
      ...validateScanSource(evidence.recoverySource),
      ...validateInspectReadOnly(evidence.recoverySource),
      ...validateSchemaSource(evidence.recoverySource),
      ...validateCommitGuard(evidence.ledgerSource),
      ...validateInstallSource(evidence.ledgerSource),
      ...validateAssertSource(evidence.testSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_RECOVERY)
    if (evidence.exitCode !== 0) process.exit(EXIT_RECOVERY)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_RECOVERY)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
