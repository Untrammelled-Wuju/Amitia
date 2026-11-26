import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/retrievalcutover'
export const RUN_PATTERN = 'TestCutover'
export const BUILD_TAG = 'rsi_cutover'
export const EXIT_CUTOVER = 60
export const EXPECTED_PASSED = 8
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const DEFAULT_SOURCES = ['doc.go']
export const TAGGED_SOURCES = ['doc.go', 'gate.go']
export const TAGGED_TESTS = ['gate_test.go']
export const BASELINE_PATH = 'backend/internal/extension/kernel/retrievalcutover/testdata/cutover_baseline.json'
export const BASELINE_FIELDS = {
  tasks: 48,
  top1_hits: 48,
  top5_hits: 48,
  false_injection: 0,
  permission_leaks: 0,
  injection_drift: 0,
  schema_tokens: 1096,
  token_drift: 0,
  schema_reads: 8
}

export const CUTOVER_CASES = [
  'TestCutoverMeasuredMetricsPass',
  'TestCutoverLegacyStaysOnBelowThreshold',
  'TestCutoverPermissionLeakBlocks',
  'TestCutoverInjectionDriftBlocks',
  'TestCutoverBaselineComparison',
  'TestCutoverRollbackRestoresLegacy',
  'TestCutoverRejectsInvalidInput',
  'TestCutoverInjectionKeepsSelection'
]

export const CUTOVER_GUARDS = [
  'MinTop1Recall:      0.95,',
  'MinTop5Recall:      0.98,',
  'MaxFalseInjection:  0,',
  'MaxPermissionLeaks: 0,',
  'MaxInjectionDrift:  0,',
  'MaxP99Latency:      500 * time.Millisecond,',
  'MaxSchemaTokens:    2048,',
  'MaxTokenDrift:      0,',
  'MaxSchemaReads:     12,',
  'if m.PermissionLeaks > t.MaxPermissionLeaks {',
  'if m.InjectionDrift > t.MaxInjectionDrift {',
  'if m.P99Latency > t.MaxP99Latency {',
  'if m.TokenDrift > t.MaxTokenDrift {',
  'if !u.verdict.Passed {',
  'return fmt.Errorf("%w: %d failing metrics: %v", ErrThresholdsNotMet, len(u.verdict.Failures), u.verdict.Failures)',
  'u.legacyDisabled = true'
]

export const ASSERT_GUARDS = [
  'if err := unit.DisableLegacy(); !errors.Is(err, retrievalcutover.ErrThresholdsNotMet) {',
  'if !strings.Contains(err.Error(), "top1_recall") {',
  'if unit.LegacyDisabled() {',
  'if metrics.FalseInjection != 0 || metrics.InjectionDrift != 0 {',
  'if failures := baseline.Compare(metrics); len(failures) != 0 {',
  'if failures[0].Metric != "baseline.top1_hits" {',
  'if identical != 8 {',
  'if err := unit.Rollback(); err != nil {'
]

export const FORBIDDEN_IMPORTS = ['internal/prompt', 'internal/chat', 'internal/character']
export const FORBIDDEN_TOKENS = ['registry.Register(ctx', 'GetModelTools(ctx']

export const EVIDENCE_ROWS = [
  'metrics tasks=48 top1=48 top5=48 false_injection=0',
  'permission leaks=0 denied_top1_rejected=true',
  'injection drift=0 suffix_ignored=true',
  'cost schema_tokens=1096 schema_reads=8 token_drift=0 flat=true',
  'verdict passed=true failures=0 legacy_disabled=true',
  'blocked reason=thresholds_not_met legacy_disabled=false',
  'permission_block leaks=1 blocked=true',
  'injection_block drift=2 blocked=true',
  'baseline equal=true fields=9 drift=0',
  'rollback restored=true second_rollback_rejected=true',
  'invalid rejected=6 cancel=context.Canceled',
  'injection identical=8 of=8'
]

export function parseCutoverRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s08 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s08 '.length).trim())
  }
  return rows
}

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

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateGateSource(text) {
  return missingGuards(text, CUTOVER_GUARDS, 'cutover gate')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'cutover test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CUTOVER_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `cutover test is missing ${name}`)
}

export function validateNoPromptImports(text) {
  const body = String(text ?? '')
  return FORBIDDEN_IMPORTS.filter((token) => body.includes(token)).map((token) => `cutover gate must not import ${token}`)
}

export function validateNoLegacyEnumerations(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `cutover surface must not call ${token}`)
}

export function validateBaseline(baseline) {
  const problems = []
  for (const [field, expected] of Object.entries(BASELINE_FIELDS)) {
    if (baseline?.[field] !== expected) problems.push(`baseline.${field}=${baseline?.[field]} want ${expected}`)
  }
  return problems
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultFiles = Array.isArray(evidence?.defaultFiles) ? evidence.defaultFiles : []
  const taggedFiles = Array.isArray(evidence?.taggedFiles) ? evidence.taggedFiles : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultFiles.join(',') !== DEFAULT_SOURCES.join(',')) {
    problems.push(`default build files=${defaultFiles.join(',')} want ${DEFAULT_SOURCES.join(',')}`)
  }
  if (taggedFiles.join(',') !== TAGGED_SOURCES.join(',')) {
    problems.push(`tagged build files=${taggedFiles.join(',')} want ${TAGGED_SOURCES.join(',')}`)
  }
  if (taggedTests.join(',') !== TAGGED_TESTS.join(',')) {
    problems.push(`tagged tests=${taggedTests.join(',')} want ${TAGGED_TESTS.join(',')}`)
  }
  if (defaultFiles.some((file) => file.endsWith('_test.go'))) problems.push('default build leaked test files')
  return problems
}

export function validateEvidence(evidence) {
  const problems = []
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
  const guards = CUTOVER_GUARDS.length + ASSERT_GUARDS.length + CUTOVER_CASES.length
  const lines = [
    `invocation=cutover package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default=[${(evidence?.defaultFiles ?? []).join(' ')}] tagged=[${(evidence?.taggedFiles ?? []).join(' ')}] tests=[${(evidence?.taggedTests ?? []).join(' ')}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m05-s08 ${row}`)
  lines.push(`passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`)
  return lines.join('\n') + '\n'
}

function goList(root, extraArgs) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}|{{.XTestGoFiles}}', ...extraArgs, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 120000
  })
  const line = `${result.stdout ?? ''}`.trim()
  const [files = '', tests = '', external = ''] = line.split('|')
  return {
    files: parseGoListFiles(files),
    tests: [...parseGoListFiles(tests), ...parseGoListFiles(external)].sort(),
    exitCode: result.status === null ? 1 : result.status
  }
}

export function runCutoverEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'retrievalcutover')
  const gateSource = readFileSync(resolve(base, 'gate.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'gate_test.go'), 'utf8')
  const baseline = JSON.parse(readFileSync(resolve(root, BASELINE_PATH), 'utf8'))
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    gateSource,
    testSource,
    baseline,
    rows: parseCutoverRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultFiles: defaultList.files,
    defaultTests: defaultList.tests,
    taggedFiles: taggedList.files,
    taggedTests: taggedList.tests,
    listExitCode: defaultList.exitCode + taggedList.exitCode,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runCutoverEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateBaseline(evidence.baseline),
    ...validateGateSource(evidence.gateSource),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoPromptImports(evidence.gateSource + evidence.testSource),
    ...validateNoLegacyEnumerations(evidence.gateSource + evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_CUTOVER)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
