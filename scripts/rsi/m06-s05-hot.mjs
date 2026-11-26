import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/enablement'
export const RUN_PATTERN = 'TestPluginHot'
export const BUILD_TAG = 'rsi_lifecycle'
export const EXIT_HOT = 69
export const EXPECTED_PASSED = 12
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const HOT_SOURCE = 'hot.go'
export const IDLE_SOURCE = 'idle.go'
export const HOT_TEST = 'hot_test.go'

export const HOT_CASES = [
  'TestPluginHotAuthorizedCategoryResides',
  'TestPluginHotRequiresUserGrant',
  'TestPluginHotRejectsPluginDeclaredProfile',
  'TestPluginHotRejectsProfileBelowFloor',
  'TestPluginHotRejectsUnknownCategory',
  'TestPluginHotInvalidPolicyAndRequest',
  'TestPluginHotRequiresActivatedRuntime',
  'TestPluginHotIdempotentAndCategoryConflict',
  'TestPluginHotCapacityLimitAndRelease',
  'TestPluginHotReleaseAllowsIdleReclaim',
  'TestPluginHotCancelAndConcurrency',
  'TestPluginHotDenialKeepsErrorChain'
]

export const SOURCE_GUARDS = [
  'if !h.policy.AllowedCategories[request.Category] {',
  'if !request.Granted {',
  'if request.Profile.Source != ProfileSourceHost {',
  'if request.Profile.MaxMemoryBytes < h.policy.MinMemoryBytes {',
  'if request.Profile.MaxConcurrent < h.policy.MinConcurrent {',
  'if h.policy.MaxHot > 0 && len(h.hot) >= h.policy.MaxHot {',
  'if existing, ok := h.hot[request.PluginID]; ok {',
  'func (h *HotResidency) Denial(pluginID string) (string, bool) {',
  'func (r *IdleReaper) WithHotCheck(check func(string) bool) *IdleReaper {',
  'if r.hotCheck != nil && r.hotCheck(candidate.id) {'
]

export const ASSERT_GUARDS = [
  'if !fixture.residency.IsHot("plugin-0000") || fixture.installer.Snapshot().Processes != 1 {',
  'if !ok || reason != "authorization_missing" {',
  'if reason != "profile_source_not_host" {',
  'if reason, _ := fixture.residency.Denial("plugin-0000"); reason != "profile_memory_below_floor" {',
  'if reason, _ := fixture.residency.Denial("plugin-0000"); reason != "category_not_allowed" {',
  'if _, err := NewHotResidency(installer, HotPolicy{}); !errors.Is(err, ErrHotInvalid) {',
  'if err := fixture.residency.Acquire(ctx, hotRequest("plugin-0000")); !errors.Is(err, ErrHotInvalid) {',
  'if err := fixture.residency.Acquire(ctx, other); !errors.Is(err, ErrHotInvalid) {',
  'if reason, _ := fixture.residency.Denial("plugin-0001"); reason != "capacity_exhausted" {',
  'if len(report.Released) != 1 || report.Released[0] != "plugin-0000" {',
  'if wins.Load() != 1 || fixture.residency.Len() != 1 {',
  'if !strings.Contains(err.Error(), "category_not_allowed") {'
]

export const FORBIDDEN_TOKENS = ['go func(', 'recover()', 'os.Setenv']

export const EVIDENCE_ROWS = [
  'resident authorized=true released=0 processes=1',
  'denied reason=authorization_missing hot=false',
  'denied reason=profile_source_not_host',
  'denied profile floors=2',
  'denied reason=category_not_allowed',
  'invalid policy=ErrHotInvalid request=ErrHotInvalid',
  'requires_runtime=true',
  'idempotent=true conflict=ErrHotInvalid',
  'capacity max=1 denied=capacity_exhausted release_frees=true',
  'release reclaims=true released=1',
  'cancel=context.Canceled concurrent_wins=1',
  'reason_visible=true chain=true'
]

export function parseHotRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m06-s05 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm06-s05 '.length).trim())
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

export function validateSource(text) {
  return missingGuards(text, SOURCE_GUARDS, 'hot residency')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'hot test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return HOT_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `hot test is missing ${name}`)
}

export function validateNoBypass(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `hot path must not contain ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(HOT_TEST)) problems.push(`default build leaked ${HOT_TEST}`)
  if (!taggedTests.includes(HOT_TEST)) problems.push(`tagged tests missing ${HOT_TEST}`)
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
  const guards = SOURCE_GUARDS.length + ASSERT_GUARDS.length + HOT_CASES.length
  const lines = [
    `invocation=hot package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${HOT_TEST}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m06-s05 ${row}`)
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

export function runHotEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement')
  const source = readFileSync(resolve(base, HOT_SOURCE), 'utf8')
  const idleSource = readFileSync(resolve(base, IDLE_SOURCE), 'utf8')
  const testSource = readFileSync(resolve(base, HOT_TEST), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    source: source + idleSource,
    testSource,
    rows: parseHotRows(log),
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
  const evidence = runHotEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(evidence.source),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoBypass(evidence.source)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_HOT)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
