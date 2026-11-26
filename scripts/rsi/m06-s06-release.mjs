import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/enablement'
export const RUN_PATTERN = 'TestPluginRelease'
export const BUILD_TAG = 'rsi_lifecycle'
export const EXIT_RELEASE = 70
export const EXPECTED_PASSED = 10
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const RELEASE_SOURCE = 'release.go'
export const RELEASE_TEST = 'leak_test.go'

export const RELEASE_CASES = [
  'TestPluginReleaseDeactivateClosesOnce',
  'TestPluginReleaseDeactivateWarm',
  'TestPluginReleaseBusyRefused',
  'TestPluginReleaseDoubleDeactivateRejected',
  'TestPluginReleaseSuspendResume',
  'TestPluginReleaseUninstallCloses',
  'TestPluginReleaseThousandCyclesStable',
  'TestPluginReleaseFailedActivationNoHandle',
  'TestPluginReleaseCancelAndInvalid',
  'TestPluginReleaseConcurrentExecuteDeactivate'
]

export const SOURCE_GUARDS = [
  'func (i *Installer) Deactivate(ctx context.Context, pluginID string, epoch uint64) error {',
  'func (i *Installer) Suspend(ctx context.Context, pluginID string, epoch uint64) error {',
  'func (i *Installer) Resume(ctx context.Context, pluginID string, epoch uint64) error {',
  'if active := i.activeCalls[pluginID]; active > 0 {',
  'i.results = map[string]ExecuteResult{}',
  'delete(i.handles, pluginID)',
  'i.beginCall(pluginID)',
  'i.endCall(pluginID)',
  'if len(m.history) > 128 {',
  'if appliedEpoch+128 < m.epoch {'
]

export const ASSERT_GUARDS = [
  'if spawner.Starts() != 1 || spawner.Stops() != 1 {',
  'if err := installer.Deactivate(ctx, "plugin-0000", 5); !errors.Is(err, ErrPluginBusy) {',
  'if err := installer.Deactivate(ctx, "plugin-0000", 4); !errors.Is(err, ErrInstallerState) {',
  'if spawner.Starts() != 2 || installer.Snapshot().Processes != 1 {',
  'if installer.Snapshot().Plugins != 0 || spawner.Stops() != 1 {',
  'if spawner.Starts() != 1000 || spawner.Stops() != 1000 {',
  'if after > baseline+2 {',
  'if len(machine.History()) > 128 {',
  'if len(installer.handles) != 0 || spawner.Stops() != 0 {',
  'if spawner.Stops() != 1 || installer.Snapshot().Processes != 0 {'
]

export const FORBIDDEN_TOKENS = ['go func(', 'recover()', 'time.Sleep(']

export const EVIDENCE_ROWS = [
  'deactivate starts=1 stops=1 processes=0',
  'deactivate_warm stops=1 state=enabled',
  'busy refused=ErrPluginBusy stops=0',
  'double_deactivate rejected=true stops=1',
  'suspend_resume stops=1 starts=2',
  'uninstall entries=0 stops=1',
  'thousand_cycles starts=1000 stops=1000 handles=0 results=0 history<=128 goroutines_delta=0',
  'failed_activation no_handle=true retry_balanced=true',
  'cancel stops=0 unknown=ErrInstallerState',
  'concurrent starts=1 stops=1'
]

export function parseReleaseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m06-s06 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm06-s06 '.length).trim())
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
  return missingGuards(text, SOURCE_GUARDS, 'release path')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'release test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return RELEASE_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `release test is missing ${name}`)
}

export function validateNoLeakyPrimitives(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `release path must not contain ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(RELEASE_TEST)) problems.push(`default build leaked ${RELEASE_TEST}`)
  if (!taggedTests.includes(RELEASE_TEST)) problems.push(`tagged tests missing ${RELEASE_TEST}`)
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
  const guards = SOURCE_GUARDS.length + ASSERT_GUARDS.length + RELEASE_CASES.length
  const lines = [
    `invocation=release package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${RELEASE_TEST}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m06-s06 ${row}`)
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

export function runReleaseEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement')
  const source = readFileSync(resolve(base, RELEASE_SOURCE), 'utf8') + readFileSync(resolve(base, 'lifecycle.go'), 'utf8') + readFileSync(resolve(base, 'execute.go'), 'utf8')
  const testSource = readFileSync(resolve(base, RELEASE_TEST), 'utf8')
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
    rows: parseReleaseRows(log),
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
  const evidence = runReleaseEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(evidence.source),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoLeakyPrimitives(readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement', RELEASE_SOURCE), 'utf8'))
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_RELEASE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
