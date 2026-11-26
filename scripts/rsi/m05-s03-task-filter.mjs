import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestTaskGate'
export const EXIT_TASK_FILTER = 55
export const EXPECTED_PASSED = 10
export const REQUIRED_SOURCES = ['task_selector.go', 'exposure.go']
export const REQUIRED_TESTS = ['task_selector_test.go']

export const PROFILE_GUARDS = [
  'type TaskProfile struct {',
  'func ClassifyTask(text string) TaskProfile {',
  'type ContractResolver interface {',
  'type GrantResolver interface {',
  'func NewTaskSelector(registry *ToolRegistry, availability AvailabilityEvaluator) *TaskSelector {',
  'func (s *TaskSelector) Select(ctx context.Context, invocation ToolInvocationContext, query string, limit int) (SelectionReport, error) {'
]

export const FILTER_GUARDS = [
  'if !tool.Enabled {',
  'if !scopeAllows(tool.Scope, invocation) {',
  'if !state.PermissionGranted {',
  'if !state.RuntimeReady {',
  'if state.Health != HealthReady {',
  'if !avail.Visible {',
  'if !avail.Executable && len(avail.Reasons) == 0 {',
  'granted, err := s.grants.Granted(ctx, tool, invocation)',
  'return nil, nil, fmt.Errorf("resolve grant for tool %s: %w", tool.ID, err)',
  'if !s.platformSupported(tool) {',
  'contractReasons, contractCause, err := s.contractProblems(ctx, tool)',
  'sort.SliceStable(report.Selected, func(i, j int) bool {',
  'if limit > 0 && len(report.Selected) > limit {'
]

export const EXPOSURE_GUARDS = [
  'func (m *ToolExposureManager) WithTaskSelector(selector *TaskSelector) *ToolExposureManager {',
  'func (m *ToolExposureManager) GetModelToolsForTask(',
  'if m.taskSelector == nil {',
  'report, err := m.taskSelector.Select(ctx, sessionCtx, query, limit)',
  'return nil, fmt.Errorf("select tools for task: %w", err)',
  'estimated := estimateToolTokens(selected.Tool)'
]

export const ASSERT_GUARDS = [
  'if deniedReport.Claimed(blocked.ID) {',
  'if !taskGateHasReason(reasons, capability.ReasonPermissionMissing) {',
  'if blockedScore <= allowedScore {',
  'if report.Claimed(offline.ID) || report.Claimed(degraded.ID) {',
  'if desktop.Claimed(iosOnly.ID) {',
  'if report.Claimed(incompatible.ID) {',
  'if report.Claimed(foreign.ID) {',
  'if !errors.Is(err, errTaskGateGrantStore) {',
  'if fmt.Sprint(base.SelectedIDs()) != fmt.Sprint(injected.SelectedIDs()) {',
  'if injected.Claimed(guarded.ID) {',
  'if fmt.Sprint(modelNames(offViews)) != fmt.Sprint(modelNames(baseline)) {'
]

export const EVIDENCE_ROWS = [
  'blocked_score=28 allowed_score=0 blocked_selected=false reason=permission_missing',
  'after_grant blocked_selected=true rank=1 score=28',
  'offline_selected=false reason=runtime_not_ready degraded_selected=false reason=health_unavailable healthy_selected=true',
  'platform=darwin/arm64 ios_selected=false universal_selected=true platform=ios/arm64 ios_selected=true',
  'contract=amitia.dsh@1 selected=true contract=amitia.dsh@2 selected=false reason=contract_incompatible malformed_chain=true',
  'cross_scope_selected=false reason=scope_denied same_scope_selected=true global_selected=true',
  'grant_error_fail_closed=true chain_intact=true message="resolve grant for tool tool-grant: store offline: grant store unavailable"',
  'injected_same_selection=true injected_same_rejection=true guarded_selected=false reason=permission_missing',
  'classified_cases=4 deterministic=true',
  'feature_off_identical=true feature_on_filtered=true',
  'canceled_context_passthrough=true'
]

export const BYPASS_TOKENS = ['query ==', 'strings.Contains(query, "grant', 'Metadata["permissionGranted"]', 'Metadata["grants"]']

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

export function parseTaskFilterRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s03 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s03 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateProfileSource(text) {
  return missingGuards(text, PROFILE_GUARDS, 'task selector source')
}

export function validateFilterSource(text) {
  return missingGuards(text, FILTER_GUARDS, 'task selector hard filter')
}

export function validateExposureSource(text) {
  return missingGuards(text, EXPOSURE_GUARDS, 'task exposure wiring')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'task filter test')
}

export function validateNoQueryGrant(text) {
  const body = String(text ?? '')
  return BYPASS_TOKENS.filter((token) => body.includes(token)).map((token) => `hard filter must not read grants from the query but contains ${token}`)
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
  const guards = PROFILE_GUARDS.length + FILTER_GUARDS.length + EXPOSURE_GUARDS.length + ASSERT_GUARDS.length
  const lines = [
    `invocation=task-filter package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m05-s03 ${row}`)
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

export function runTaskFilterEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability')
  const selectorSource = readFileSync(resolve(base, 'task_selector.go'), 'utf8')
  const exposureSource = readFileSync(resolve(base, 'exposure.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'task_selector_test.go'), 'utf8')
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
    rows: parseTaskFilterRows(log),
    selectorSource,
    exposureSource,
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
      process.stdout.write(`invocation=task-filter package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runTaskFilterEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateProfileSource(evidence.selectorSource),
      ...validateFilterSource(evidence.selectorSource),
      ...validateExposureSource(evidence.exposureSource),
      ...validateAssertSource(evidence.testSource),
      ...validateNoQueryGrant(evidence.selectorSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_TASK_FILTER)
    if (evidence.exitCode !== 0) process.exit(EXIT_TASK_FILTER)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_TASK_FILTER)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
