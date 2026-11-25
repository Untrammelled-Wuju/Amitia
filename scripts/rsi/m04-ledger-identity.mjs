import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestJournalIdentity|TestJournalToken'
export const EXIT_LEDGER_IDENTITY = 50
export const EXPECTED_PASSED = 10
export const REQUIRED_SOURCES = ['ledger.go', 'ledger_identity.go', 'effect_token.go']
export const REQUIRED_TESTS = ['ledger_identity_test.go']

export const IDENTITY_GUARDS = [
  'var ErrLedgerInvocationMismatch = errors.New(',
  'type LedgerBinding struct {',
  'func (e LedgerEntry) Binding() LedgerBinding {',
  'func (l *InvocationLedger) InspectBinding(ctx context.Context, scope LedgerScope, invocationID, idempotencyKey string) (LedgerEntry, BindingVerdict, error) {',
  'entry, found, err := l.Lookup(ctx, scope, idempotencyKey)',
  'return entry, BindingForeign, nil'
]

export const INSPECT_WRITE_TOKENS = [
  'INSERT INTO',
  'UPDATE kernel_',
  'DELETE FROM'
]

export const COMMIT_GUARDS = [
  'if entry.InvocationID != strings.TrimSpace(contract.Trace.InvocationID) {',
  'requestedInvocation=%q", ErrLedgerInvocationMismatch, entry.Binding()',
  'db.Exec(EffectTokenImmutableIdentitySchema)',
  'db.Exec(EffectTokenImmutableRemovalSchema)'
]

export const SCHEMA_GUARDS = [
  'const EffectTokenImmutableIdentitySchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_effect_immutable_identity',
  'BEFORE UPDATE OF scope_key,idempotency_key,attempt,salt,token ON kernel_invocation_effect',
  'const EffectTokenImmutableRemovalSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_effect_immutable_removal',
  'BEFORE DELETE ON kernel_invocation_effect',
  'const effectTokenImmutableMessage = "effect token row is immutable"',
  'func IsEffectTokenImmutableError(err error) bool {'
]

export const ASSERT_GUARDS = [
  'if !errors.Is(mismatchErr, executionjournal.ErrLedgerInvocationMismatch) {',
  'if intruderRan || intruderRuns != 0 {',
  'if !executionjournal.IsEffectTokenImmutableError(saltErr) || !executionjournal.IsEffectTokenImmutableError(tokenErr) {',
  'if !executionjournal.IsEffectTokenImmutableError(deleteErr) {',
  'if err := restarted.VerifyEffectToken(t.Context(), request, token); err != nil {',
  'if !errors.Is(verifyErr, executionjournal.ErrEffectTokenInvalid) {'
]

export const EVIDENCE_ROWS = [
  'key=key-shared owner=inv-owner intruder=inv-intruder state=committed executions=1 rejected=true verdict=foreign',
  'replay=owned executions=1 result=match state=committed verdict=owned',
  'scope_parts=true rows=2 executions=2',
  'keys=2 rows=2 executions=2',
  'absent=absent owned=owned foreign=foreign rows=1 writes=0',
  'restarted=true state=committed result=match executions_after_restart=0 verdict=owned foreign=rejected',
  'update_salt=blocked update_token=blocked salt_unchanged=true verify=valid',
  'delete=blocked rows=1 verify=valid',
  'state_transition=allowed verify_after_redeem=rejected attempt=1',
  'restarted=true tamper=blocked verify=valid rows=1'
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

export function parseIdentityRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m04-s10 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm04-s10 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateIdentitySource(text) {
  return missingGuards(text, IDENTITY_GUARDS, 'ledger identity source')
}

export function validateInspectReadOnly(text) {
  const body = String(text ?? '')
  return INSPECT_WRITE_TOKENS.filter((token) => body.includes(token)).map((token) => `binding inspection must stay read-only but contains ${token}`)
}

export function validateCommitGuard(text) {
  return missingGuards(text, COMMIT_GUARDS, 'ledger commit source')
}

export function validateSchemaSource(text) {
  return missingGuards(text, SCHEMA_GUARDS, 'effect token schema source')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'ledger identity test')
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
    `invocation=ledger-identity package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${IDENTITY_GUARDS.length + COMMIT_GUARDS.length + SCHEMA_GUARDS.length + ASSERT_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m04-s10 ${row}`)
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

export function runLedgerIdentityEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal')
  const identitySource = readFileSync(resolve(base, 'ledger_identity.go'), 'utf8')
  const ledgerSource = readFileSync(resolve(base, 'ledger.go'), 'utf8')
  const schemaSource = readFileSync(resolve(base, 'effect_token.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'ledger_identity_test.go'), 'utf8')
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
    rows: parseIdentityRows(log),
    identitySource,
    ledgerSource,
    schemaSource,
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
      process.stdout.write(`invocation=ledger-identity package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runLedgerIdentityEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateIdentitySource(evidence.identitySource),
      ...validateInspectReadOnly(evidence.identitySource),
      ...validateCommitGuard(evidence.ledgerSource),
      ...validateSchemaSource(evidence.schemaSource),
      ...validateAssertSource(evidence.testSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_LEDGER_IDENTITY)
    if (evidence.exitCode !== 0) process.exit(EXIT_LEDGER_IDENTITY)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_LEDGER_IDENTITY)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
