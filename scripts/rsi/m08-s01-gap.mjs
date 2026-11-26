import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestGapReport'
export const BUILD_TAG = 'rsi_gap'
export const EXIT_GAP = 87
export const EXPECTED_PASSED = 12
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const GAP_FILE = 'gap_report.go'
export const GAP_TEST_FILE = 'gap_report_test.go'

export const EVIDENCE_ROWS = [
  'existing count=1 verdict=existing',
  'composable count=1 missing=0',
  'missing_dependency=1',
  'no_catalog missing=1 reason=not_installed',
  'duplicate_rule rejected=true',
  'empty_goal rejected=true',
  'nil_selector rejected=true',
  'rule_requires rejected=true',
  'rejected reason=tool_disabled',
  'ordering sorted=true',
  'existing_precedence=true',
  'verdict precedence=existing>composable>missing'
]

export const CASES = [
  'TestGapReportExistingCapability',
  'TestGapReportComposableCapability',
  'TestGapReportMissingDependency',
  'TestGapReportNoCatalog',
  'TestGapReportDuplicateRule',
  'TestGapReportEmptyGoal',
  'TestGapReportNilSelector',
  'TestGapReportRuleWithoutRequires',
  'TestGapReportRejectedTool',
  'TestGapReportOrdering',
  'TestGapReportExistingWinsOverRule',
  'TestGapReportVerdictPrecedence'
]

export const SOURCE_GUARDS = [
  'if goal == "" {',
  'if selector == nil {',
  'if len(rule.Requires) == 0 {',
  'if seenRules[result] {',
  'if selectedIDs[rule.Result] || selectedNames[strings.ToLower(rule.Result)] {',
  'Reasons:         []AvailabilityReason{ReasonDependencyMissing},',
  'if len(report.Existing) == 0 && len(report.Composable) == 0 && len(report.Missing) == 0 {',
  'sort.SliceStable(selected,'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m08-s01 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm08-s01 '.length).trim())
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

export function validateIsolation(evidence) {
  const problems = []
  const defaultFiles = Array.isArray(evidence?.defaultFiles) ? evidence.defaultFiles : []
  const taggedFiles = Array.isArray(evidence?.taggedFiles) ? evidence.taggedFiles : []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultFiles.includes(GAP_FILE)) problems.push(`default build leaked ${GAP_FILE}`)
  if (!taggedFiles.includes(GAP_FILE)) problems.push(`tagged build missing ${GAP_FILE}`)
  if (defaultTests.includes(GAP_TEST_FILE)) problems.push(`default tests leaked ${GAP_TEST_FILE}`)
  if (!taggedTests.includes(GAP_TEST_FILE)) problems.push(`tagged tests missing ${GAP_TEST_FILE}`)
  if (taggedFiles.length !== defaultFiles.length + 1) problems.push(`tagged files=${taggedFiles.length} want default+1=${defaultFiles.length + 1}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability', GAP_FILE)
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `gap report is missing ${guard}`)
}

export function validateReuse(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability', GAP_FILE)
  const body = readFileSync(path, 'utf8')
  const problems = []
  if (!body.includes('selector.Select(ctx, ToolInvocationContext{}, goal, 0)')) problems.push('gap report must reuse TaskSelector.Select')
  if (body.includes('func scoreTool')) problems.push('gap report must not reimplement lexical scoring')
  if (!body.includes('Reasons:         []AvailabilityReason{ReasonDependencyMissing},')) problems.push('gap report must reuse AvailabilityReason')
  return problems
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `gap test is missing ${name}`)
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
  const guards = CASES.length + SOURCE_GUARDS.length + 2
  const lines = [
    `invocation=gap package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m08-s01 ${row}`)
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
    tests: [...parseGoListFiles(tests), ...parseGoListFiles(external)].sort()
  }
}

export function runGapEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability', GAP_TEST_FILE), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 1200000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    testSource,
    rows: parseRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultFiles: defaultList.files,
    taggedFiles: taggedList.files,
    defaultTests: defaultList.tests,
    taggedTests: taggedList.tests,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runGapEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validateReuse(root),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_GAP)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
