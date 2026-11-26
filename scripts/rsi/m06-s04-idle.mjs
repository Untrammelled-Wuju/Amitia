import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/enablement'
export const RUN_PATTERN = 'TestPluginIdle'
export const BUILD_TAG = 'rsi_lifecycle'
export const EXIT_IDLE = 68
export const EXPECTED_PASSED = 10
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const IDLE_SOURCE = 'idle.go'
export const IDLE_TEST = 'idle_test.go'

export const IDLE_CASES = [
  'TestPluginIdleReleasesAfterTTL',
  'TestPluginIdleKeepsRecentRuntime',
  'TestPluginIdleTouchExtendsDeadline',
  'TestPluginIdleReleasesWarmRuntime',
  'TestPluginIdleSkipsRunningRuntime',
  'TestPluginIdleReactivateAfterRelease',
  'TestPluginIdleDrainFailureKeepsRuntime',
  'TestPluginIdleRejectsInvalidPolicy',
  'TestPluginIdleCancelLeavesRuntime',
  'TestPluginIdleSkipsUntrackedRuntime'
]

export const SOURCE_GUARDS = [
  'type SupervisorDrainer interface {',
  'if now.Sub(lastUsed) < r.policy.TTL {',
  'if err := r.drainer.Drain(ctx, instanceID, r.policy.DrainTimeout); err != nil {',
  'handle, err := r.installer.releaseIdle(candidate.id, candidate.state)',
  'if closer, ok := handle.(io.Closer); ok {',
  'if current == LifecycleWarm {',
  'if _, err := machine.Apply(context.Background(), EventDeactivate, machine.Epoch()+1); err != nil {',
  'delete(i.handles, pluginID)',
  'func (i *Installer) WithActivity(activity func(string)) *Installer {',
  'if i.activity != nil {'
]

export const ASSERT_GUARDS = [
  'if len(report.Released) != 1 || report.Released[0] != "plugin-0000" {',
  'if fixture.runtime.StopCount() != 1 {',
  'if len(report.Released) != 0 || len(report.Skipped) != 1 {',
  'if fixture.installer.Snapshot().Processes != 1 {',
  'if state, _ := fixture.installer.State("plugin-0000"); state != LifecycleWarm {',
  'if state, _ := fixture.installer.State("plugin-0000"); state != LifecycleEnabled {',
  'if fixture.spawner.Calls() != 2 {',
  'if _, err := reaper.Sweep(ctx); err == nil {',
  'IdlePolicy{}, nil, nil); !errors.Is(err, ErrIdleInvalid) {',
  'if !errors.Is(err, context.Canceled) {',
  'if len(report.Released) != 0 || fixture.installer.Snapshot().Processes != 1 {'
]

export const FORBIDDEN_TOKENS = ['go func(', 'time.AfterFunc(', 'time.Tick(', 'time.NewTicker(']

export const EVIDENCE_ROWS = [
  'release after_ttl=true state=enabled processes=0 supervisor_stops=1',
  'keep before_ttl=true processes=1 supervisor_stops=0',
  'touch extends=true total=90s ttl=60s released=0',
  'warm_release warm_to_enabled=true stops=1',
  'running skipped=true processes=1',
  'reactivate spawns=2 state=activated',
  'drain_failure state=activated processes=1 released=0',
  'invalid ttl=0 rejected=ErrIdleInvalid',
  'cancel released=0 err=context.Canceled',
  'untracked skipped=true released=0'
]

export function parseIdleRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m06-s04 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm06-s04 '.length).trim())
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
  return missingGuards(text, SOURCE_GUARDS, 'idle reaper')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'idle test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return IDLE_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `idle test is missing ${name}`)
}

export function validateNoTimers(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `idle reaper must not contain ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(IDLE_TEST)) problems.push(`default build leaked ${IDLE_TEST}`)
  if (!taggedTests.includes(IDLE_TEST)) problems.push(`tagged tests missing ${IDLE_TEST}`)
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
  const guards = SOURCE_GUARDS.length + ASSERT_GUARDS.length + IDLE_CASES.length
  const lines = [
    `invocation=idle package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${IDLE_TEST}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m06-s04 ${row}`)
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

export function runIdleEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement')
  const source = readFileSync(resolve(base, IDLE_SOURCE), 'utf8')
  const installSource = readFileSync(resolve(base, 'install.go'), 'utf8')
  const executeSource = readFileSync(resolve(base, 'execute.go'), 'utf8')
  const testSource = readFileSync(resolve(base, IDLE_TEST), 'utf8')
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
    installSource,
    executeSource,
    testSource,
    rows: parseIdleRows(log),
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
  const evidence = runIdleEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(evidence.source + evidence.installSource + evidence.executeSource),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoTimers(evidence.source)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_IDLE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
