import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestQuality'
export const BUILD_TAG = 'rsi_quality rsi_cutover'
export const EXIT_QUALITY = 64
export const EXPECTED_PASSED = 9
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const QUALITY_SOURCE = 'quality_bench_test.go'
export const QUALITY_DIR = 'backend/internal/extension/kernel/capability'

export const QUALITY_CASES = [
  'TestQualityFixedTaskSetRecall',
  'TestQualityPayloadReduction',
  'TestQualityPermissionFalsePositive',
  'TestQualityLatency',
  'TestQualityGateAllowsReplacement',
  'TestQualityGateBlocksDegraded',
  'TestQualityInjectionNoDrift',
  'TestQualityOffStateIdentical',
  'TestQualityInvalidAndCancel'
]

export const BENCH_GUARDS = [
  'qualityCapabilities = 12',
  'qualityNoise        = 288',
  'qualityExactTasks   = 12',
  'qualityLexicalTasks = 24',
  'qualityAdversarial  = 12',
  'reduction := float64(measurement.Baseline) / perTask',
  'retrievalcutover.DefaultThresholds()',
  'retrievalcutover.Metrics{'
]

export const ASSERT_GUARDS = [
  'if top1 < 0.95 || top5 < 0.98 {',
  'if reduction < 10 {',
  'if measurement.Leaks != 0 {',
  'if measurement.P99 > 50*time.Millisecond {',
  'if !verdict.Passed {',
  'if unit.LegacyDisabled() {',
  'if measurement.Drift != 0 {',
  'if strings.Join(qualityIDs(off), ",") != strings.Join(qualityIDs(legacy), ",") {',
  'capability.DetailRequest{Query: "打开页面00", Limit: -1}); !errors.Is(err, capability.ErrDetailRequestInvalid) {'
]

export const FORBIDDEN_IMPORTS = ['internal/prompt', 'internal/chat', 'openai', 'anthropic']
export const FORBIDDEN_TOKENS = ['registry.Register(ctx', 'os.Setenv']

export const EVIDENCE_ROWS = [
  'recall tasks=36 top1=36 top5=36 rate1=1.000 rate5=1.000',
  'payload baseline_bytes=892896 candidate_bytes=393440 reduction=81.7x views_baseline=300',
  'permission baseline_leak=true candidate_leaks=0 tasks=36',
  'latency within_budget=true budget_ms=50',
  'gate passed=true failures=0 replacement_allowed=true',
  'degraded blocked=true reason=top1_recall legacy_disabled=false',
  'injection adversarial=12 drift=0',
  'offstate identical=true legacy_views=300',
  'invalid cancel=context.Canceled negative_limit=ErrDetailRequestInvalid'
]

export function parseQualityRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s12 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s12 '.length).trim())
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

export function validateBenchSource(text) {
  return missingGuards(text, BENCH_GUARDS, 'quality benchmark')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'quality test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return QUALITY_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `quality test is missing ${name}`)
}

export function validateNoModelAnalysis(text) {
  const body = String(text ?? '')
  return FORBIDDEN_IMPORTS.filter((token) => body.includes(token)).map((token) => `quality benchmark must not reference ${token}`)
}

export function validateNoProductionWiring(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `quality benchmark must not contain ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(QUALITY_SOURCE)) problems.push(`default build leaked ${QUALITY_SOURCE}`)
  if (!taggedTests.includes(QUALITY_SOURCE)) problems.push(`tagged tests missing ${QUALITY_SOURCE}`)
  const expected = [...defaultTests, QUALITY_SOURCE].sort().join(',')
  if (taggedTests.join(',') !== expected) problems.push(`tagged tests=${taggedTests.join(',')} want ${expected}`)
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
  const guards = BENCH_GUARDS.length + ASSERT_GUARDS.length + QUALITY_CASES.length
  const lines = [
    `invocation=quality package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards} tags="${BUILD_TAG}"`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${QUALITY_SOURCE}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m05-s12 ${row}`)
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

export function runQualityEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const source = readFileSync(resolve(root, QUALITY_DIR, QUALITY_SOURCE), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    source,
    rows: parseQualityRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultTests: defaultList.tests,
    taggedTests: taggedList.tests,
    listExitCode: defaultList.exitCode + taggedList.exitCode,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runQualityEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateBenchSource(evidence.source),
    ...validateAssertSource(evidence.source),
    ...validateCaseNames(evidence.source),
    ...validateNoModelAnalysis(evidence.source),
    ...validateNoProductionWiring(evidence.source)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_QUALITY)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
