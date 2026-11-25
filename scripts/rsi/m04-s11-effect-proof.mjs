import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestJournalProof'
export const EXIT_EFFECT_PROOF = 51
export const EXPECTED_PASSED = 11
export const REQUIRED_SOURCES = ['ledger.go', 'proof.go']
export const REQUIRED_TESTS = ['proof_test.go']

export const PROOF_GUARDS = [
  'type EffectProofKind string',
  'ProofKindMessage       EffectProofKind = "message"',
  'func ProofKindForCapability(capabilityID string) (EffectProofKind, bool) {',
  'func (p EffectProof) missingFields() []string {',
  'func (l *InvocationLedger) RecordEffectProof(ctx context.Context, request LedgerRequest, proof EffectProof) (EffectProofRecord, bool, error) {',
  'func (l *InvocationLedger) InspectProofs(ctx context.Context, scope LedgerScope, idempotencyKey string) ([]EffectProofRecord, error) {'
]

export const KIND_GUARDS = [
  'func (p EffectProof) missingFields() []string {',
  'case ProofKindMessage:',
  'case ProofKindFileWrite:',
  'case ProofKindDeviceControl:',
  'missing = append(missing, "bytes")'
]

export const CLAIM_GUARDS = [
  'func (l *InvocationLedger) ExactlyOnceClaim(ctx context.Context, scope LedgerScope, idempotencyKey string) (ExactlyOnceClaim, error) {',
  'if !defined {',
  'claim.Reason = ClaimReasonSurfaceUnknown',
  'if entry.State != EffectCommitted {',
  'claim.Reason = ClaimReasonNotSettled',
  'claim.Reason = ClaimReasonMissingProof',
  'if record.PayloadDigest != entry.PayloadHash {',
  'claim.Claimable = true'
]

export const SCHEMA_GUARDS = [
  'const EffectProofSchema = `CREATE TABLE IF NOT EXISTS kernel_invocation_effect_proof (',
  'PRIMARY KEY(scope_key,idempotency_key,proof_kind))',
  'const EffectProofImmutableSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_effect_proof_immutable',
  'BEFORE UPDATE ON kernel_invocation_effect_proof',
  'const EffectProofRemovalSchema = `CREATE TRIGGER IF NOT EXISTS kernel_invocation_effect_proof_removal',
  'BEFORE DELETE ON kernel_invocation_effect_proof',
  'const effectProofImmutableMessage = "effect proof row is immutable"'
]

export const INSTALL_GUARDS = [
  'db.Exec(EffectProofSchema)',
  'db.Exec(EffectProofImmutableSchema)',
  'db.Exec(EffectProofRemovalSchema)'
]

export const ASSERT_GUARDS = [
  'if claim.Claimable || claim.Reason != executionjournal.ClaimReasonMissingProof || claim.ProofFound || claim.Kind != executionjournal.ProofKindMessage {',
  'if !errors.Is(err, executionjournal.ErrEffectProofIncomplete) || !strings.Contains(err.Error(), testCase.missing) {',
  'if !errors.Is(err, executionjournal.ErrEffectProofKindMismatch) {',
  'if !errors.Is(err, executionjournal.ErrEffectProofConflict) {',
  'if claim.Claimable || claim.Reason != executionjournal.ClaimReasonSurfaceUnknown {',
  '!executionjournal.IsEffectProofImmutableError(err) {',
  'if !claim.Claimable || claim.Kind != executionjournal.ProofKindDeviceControl {'
]

export const INSPECT_WRITE_TOKENS = [
  'INSERT INTO',
  'UPDATE kernel_',
  'DELETE FROM'
]

export const EVIDENCE_ROWS = [
  'claimable=false reason=missing_effect_proof state=committed proof=false kind=message',
  'claimable=true reason=claimable state=committed proof=true kind=message',
  'kind=message inserted=true target=conversation-42 remote=mid-9001 digest=bound',
  'repeat=idempotent inserted=false rows=1',
  'kind=file_write inserted=true bytes=2048 remote=empty digest=bound',
  'kind=file_write claimable=true reason=claimable',
  'kind=device_control inserted=true target=device-android-8864cd52 remote=receipt-7f3',
  'conflict=rejected rows=1',
  'incomplete_message=rejected missing=remoteId,digest',
  'incomplete_file_write=rejected missing=bytes',
  'kind_mismatch=rejected capability=host.message.send proof=file_write',
  'foreign_capability=rejected',
  'unknown_kind=rejected',
  'surface_undefined=rejected claimable=false reason=unknown_effect_surface',
  'payload_mismatch=rejected',
  'claimable=false reason=proof_payload_mismatch proof=true',
  'orphan=rejected rows=0',
  'state=unknown claimable=false reason=not_settled proof=true',
  'state=committed claimable=true reason=claimable',
  'update=blocked delete=blocked rows=1 remote=mid-9001',
  'restart=preserved rows=1 claimable=true kind=device_control',
  'restart_repeat=idempotent delete=blocked'
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

export function parseProofRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m04-s11 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm04-s11 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateProofSource(text) {
  return missingGuards(text, PROOF_GUARDS, 'effect proof source')
}

export function validateKindSource(text) {
  return missingGuards(text, KIND_GUARDS, 'effect proof kind source')
}

export function validateClaimSource(text) {
  return missingGuards(text, CLAIM_GUARDS, 'exactly-once claim source')
}

export function validateSchemaSource(text) {
  return missingGuards(text, SCHEMA_GUARDS, 'effect proof schema source')
}

export function validateInstallSource(text) {
  return missingGuards(text, INSTALL_GUARDS, 'ledger bootstrap source')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'effect proof test')
}

export function inspectBody(text) {
  const body = String(text ?? '')
  const start = body.indexOf('func (l *InvocationLedger) InspectProofs')
  if (start < 0) return ''
  const end = body.indexOf('\nfunc ', start + 1)
  return end < 0 ? body.slice(start) : body.slice(start, end)
}

export function validateInspectReadOnly(text) {
  const body = inspectBody(text)
  const problems = INSPECT_WRITE_TOKENS.filter((token) => body.includes(token)).map((token) => `proof inspection must stay read-only but contains ${token}`)
  if (body === '') problems.push('proof inspection function not found')
  return problems
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
    `invocation=effect-proof package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${PROOF_GUARDS.length + KIND_GUARDS.length + CLAIM_GUARDS.length + SCHEMA_GUARDS.length + INSTALL_GUARDS.length + ASSERT_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m04-s11 ${row}`)
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

export function runEffectProofEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal')
  const proofSource = readFileSync(resolve(base, 'proof.go'), 'utf8')
  const ledgerSource = readFileSync(resolve(base, 'ledger.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'proof_test.go'), 'utf8')
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
    rows: parseProofRows(log),
    proofSource,
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
      process.stdout.write(`invocation=effect-proof package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runEffectProofEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateProofSource(evidence.proofSource),
      ...validateKindSource(evidence.proofSource),
      ...validateClaimSource(evidence.proofSource),
      ...validateSchemaSource(evidence.proofSource),
      ...validateInspectReadOnly(evidence.proofSource),
      ...validateInstallSource(evidence.ledgerSource),
      ...validateAssertSource(evidence.testSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_EFFECT_PROOF)
    if (evidence.exitCode !== 0) process.exit(EXIT_EFFECT_PROOF)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_EFFECT_PROOF)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
