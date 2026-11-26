import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/enablement'
export const RUN_PATTERN = 'TestPluginBaseline'
export const BUILD_TAG = 'rsi_lifecycle'
export const EXIT_BASELINE = 71
export const EXPECTED_PASSED = 9
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const BASELINE_TEST = 'baseline_test.go'

export const BASELINE_CASES = [
  'TestPluginBaselineZeroInstalled',
  'TestPluginBaselineThousand',
  'TestPluginBaselineTwoThousand',
  'TestPluginBaselineRealPluginCounts',
  'TestPluginBaselineEnableStaysCold',
  'TestPluginBaselineActivationIsExplicit',
  'TestPluginBaselineIdempotentInstall',
  'TestPluginBaselineCancelKeepsZero',
  'TestPluginBaselineMachineInfo'
]

export const ASSERT_GUARDS = [
  'if snapshot.Plugins != 0 || snapshot.Processes != 0 || snapshot.Workers != 0 || snapshot.SpawnCalls != 0 {',
  'if sample.Processes != 0 || sample.Workers != 0 || sample.SpawnCalls != 0 {',
  'if sample.Goroutines > 2 {',
  'if heapPerPlugin == 0 || heapPerPlugin > 64*1024 {',
  'if ratio > 2.5 {',
  'if timeRatio > 4 {',
  'if second.Processes != 0 || second.SpawnCalls != 0 {',
  'if snapshot.Processes != 0 || snapshot.Workers != 0 || spawner.Starts() != 0 {',
  'if snapshot.Processes != 5 || spawner.Starts() != 5 {',
  'if report.Installed != 0 || report.Skipped != 1000 {'
]

export const EVIDENCE_ROWS = [
  'zero plugins=0 processes=0 workers=0 spawn_calls=0',
  'thousand installed=1000 processes=0 workers=0 spawn_calls=0',
  'two_thousand installed=2000 processes=0 workers=0 spawn_calls=0',
  'scaling linear=true per_plugin_ratio<=2.5 time_ratio<=4',
  'real_plugins counts=1,3,5,10 processes=0',
  'enable plugins=1000 processes=0',
  'activate instances=5 spawn_calls=5',
  'idempotent skipped=1000 handles=0',
  'cancel entries=0 err=context.Canceled',
  'machine reported=true'
]

export function parseBaselineRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m06-s07 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm06-s07 '.length).trim())
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

export function validateAssertSource(text) {
  const body = String(text ?? '')
  return ASSERT_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `baseline test is missing ${guard}`)
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return BASELINE_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `baseline test is missing ${name}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(BASELINE_TEST)) problems.push(`default build leaked ${BASELINE_TEST}`)
  if (!taggedTests.includes(BASELINE_TEST)) problems.push(`tagged tests missing ${BASELINE_TEST}`)
  for (const file of defaultTests) {
    if (!taggedTests.includes(file)) problems.push(`tagged build dropped default test ${file}`)
  }
  for (const file of taggedTests) {
    if (!defaultTests.includes(file) && !file.endsWith('_test.go')) problems.push(`tagged build leaked non-test file ${file}`)
  }
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
  const guards = ASSERT_GUARDS.length + BASELINE_CASES.length
  const lines = [
    `invocation=baseline package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${BASELINE_TEST}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m06-s07 ${row}`)
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

export function runBaselineEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement', BASELINE_TEST), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    testSource,
    rows: parseBaselineRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultTests: defaultList.tests,
    taggedTests: taggedList.tests,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runBaselineEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_BASELINE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
