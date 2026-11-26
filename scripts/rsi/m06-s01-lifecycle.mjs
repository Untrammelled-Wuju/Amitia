import { spawnSync } from 'node:child_process'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/enablement'
export const RUN_PATTERN = 'TestLifecycle'
export const BUILD_TAG = 'rsi_lifecycle'
export const EXIT_LIFECYCLE = 65
export const EXPECTED_PASSED = 10
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const LIFECYCLE_SOURCE = 'lifecycle.go'
export const LIFECYCLE_TEST = 'lifecycle_state_test.go'
export const PACKAGE_DIR = 'backend/internal/extension/kernel/enablement'

export const LIFECYCLE_CASES = [
  'TestLifecycleHappyPath',
  'TestLifecycleInstalledCannotJumpToRunning',
  'TestLifecycleNoBackwardWhileRunning',
  'TestLifecycleIdempotentReplay',
  'TestLifecycleStaleEpochRejected',
  'TestLifecycleInvalidInputAndCancel',
  'TestLifecycleMinimumHops',
  'TestLifecycleDeriveFromExistingEnums',
  'TestLifecycleProjectionsStayConsistent',
  'TestLifecycleConcurrentReplay'
]

export const SOURCE_GUARDS = [
  'LifecycleAbsent    LifecycleState = "absent"',
  'LifecycleInstalled LifecycleState = "installed"',
  'LifecycleEnabled   LifecycleState = "enabled"',
  'LifecycleActivated LifecycleState = "activated"',
  'LifecycleWarm      LifecycleState = "warm"',
  'LifecycleRunning   LifecycleState = "running"',
  'LifecycleSuspended LifecycleState = "suspended"',
  '{EventEnable, LifecycleEnabled},',
  '{EventActivate, LifecycleActivated},',
  '{EventWarm, LifecycleWarm},',
  '{EventStart, LifecycleRunning},',
  'if epoch <= m.epoch {',
  'if previous, ok := m.applied[epoch]; ok {',
  'ErrLifecycleTransition = errors.New("plugin lifecycle: illegal transition")',
  'func DeriveLifecycle(installation InstallationState, mode EnablementState, desired DesiredRuntimeState, actual ActualRuntimeState) (LifecycleState, error) {',
  'if desired == DesiredRuntimePaused {'
]

export const ASSERT_GUARDS = [
  'if !errors.Is(err, ErrLifecycleTransition) {',
  'if !strings.Contains(err.Error(), "installed + start") {',
  'if _, err := machine.Apply(ctx, event, 1); !errors.Is(err, ErrLifecycleTransition) {',
  'if replay.Changed || replay.From != first.From || replay.To != first.To {',
  'if _, err := machine.Apply(ctx, EventDisable, 2); !errors.Is(err, ErrLifecycleStale) {',
  'if !errors.Is(err, ErrLifecycleStale) {',
  'if _, err := machine.Apply(ctx, EventEnable, 1); !errors.Is(err, context.Canceled) {',
  'if _, ok := MinHops(LifecycleState("bogus"), LifecycleRunning); ok {',
  'if !errors.Is(err, item.wantErr) {',
  'if changed.Load() != 1 {'
]

export const FORBIDDEN_TOKENS = ['os.Setenv', 'time.Sleep(']
export const FORBIDDEN_IMPORTS = ['internal/prompt', 'internal/chat', 'openai']

export const EVIDENCE_ROWS = [
  'happy steps=12 final=absent epoch=12 history=12',
  'no_skip installed_start=ErrLifecycleTransition state=installed epoch=0',
  'no_backward running_events=4 rejected=true state=running',
  'idempotent replay_changed=false history=2 conflict=ErrLifecycleStale',
  'stale epoch=3 current=6 rejected=true state=running',
  'invalid bogus_state=ErrLifecycleInvalid bogus_event=ErrLifecycleInvalid cancel=context.Canceled',
  'hops installed_running=4 absent_running=5 running_absent=2',
  'derive cases=8 stable=5 inconsistent=2 transitional=1',
  'projections states=7 suspended_derives=suspended legal_edges=16',
  'concurrent workers=8 changed=1 history=1 state=enabled'
]

export function parseLifecycleRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m06-s01 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm06-s01 '.length).trim())
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
  return missingGuards(text, SOURCE_GUARDS, 'lifecycle machine')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'lifecycle test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return LIFECYCLE_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `lifecycle test is missing ${name}`)
}

export function validateNoForbidden(text) {
  const body = String(text ?? '')
  return [...FORBIDDEN_TOKENS, ...FORBIDDEN_IMPORTS].filter((token) => body.includes(token)).map((token) => `lifecycle surface must not contain ${token}`)
}

export function scanLifecycleConsumers(root, target = 'NewMachine(') {
  const ownPrefix = resolve(root, PACKAGE_DIR) + '/'
  const hits = []
  const walk = (directory) => {
    for (const entry of readdirSync(directory)) {
      const path = join(directory, entry)
      const stats = statSync(path)
      if (stats.isDirectory()) {
        if (entry === 'node_modules' || entry === '.git' || entry === 'artifacts') continue
        walk(path)
        continue
      }
      if (!entry.endsWith('.go') || entry.endsWith('_test.go')) continue
      if (path.startsWith(ownPrefix)) continue
      if (readFileSync(path, 'utf8').includes(target)) hits.push(path.slice(resolve(root).length + 1))
    }
  }
  walk(resolve(root, 'backend'))
  return hits.sort()
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(LIFECYCLE_TEST)) problems.push(`default build leaked ${LIFECYCLE_TEST}`)
  if (!taggedTests.includes(LIFECYCLE_TEST)) problems.push(`tagged tests missing ${LIFECYCLE_TEST}`)
  for (const file of defaultTests) {
    if (!taggedTests.includes(file)) problems.push(`tagged build dropped default test ${file}`)
  }
  for (const file of taggedTests) {
    if (!defaultTests.includes(file) && !file.endsWith('_test.go')) problems.push(`tagged build leaked non-test file ${file}`)
  }
  for (const consumer of evidence?.consumers ?? []) problems.push(`lifecycle machine is wired before the integration step: ${consumer}`)
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
  const guards = SOURCE_GUARDS.length + ASSERT_GUARDS.length + LIFECYCLE_CASES.length
  const lines = [
    `invocation=lifecycle package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${LIFECYCLE_TEST}] consumers=${(evidence?.consumers ?? []).length}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m06-s01 ${row}`)
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

export function runLifecycleEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement')
  const source = readFileSync(resolve(base, LIFECYCLE_SOURCE), 'utf8')
  const testSource = readFileSync(resolve(base, LIFECYCLE_TEST), 'utf8')
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
    testSource,
    rows: parseLifecycleRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultTests: defaultList.tests,
    taggedTests: taggedList.tests,
    consumers: scanLifecycleConsumers(root),
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runLifecycleEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(evidence.source),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoForbidden(evidence.source + evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_LIFECYCLE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
