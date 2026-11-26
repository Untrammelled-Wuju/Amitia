import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/enablement'
export const RUN_PATTERN = 'TestPluginExecute'
export const BUILD_TAG = 'rsi_lifecycle'
export const EXIT_EXECUTE = 67
export const EXPECTED_PASSED = 10
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const EXECUTE_SOURCE = 'execute.go'
export const EXECUTE_TEST = 'execute_test.go'
export const PACKAGE_DIR = 'backend/internal/extension/kernel/enablement'

export const EXECUTE_CASES = [
  'TestPluginExecuteColdStartSpawnsOnce',
  'TestPluginExecuteInstalledDenied',
  'TestPluginExecuteActivationTimeoutRollsBack',
  'TestPluginExecuteSpawnErrorRollsBack',
  'TestPluginExecuteInvokeErrorRollsBackColdStart',
  'TestPluginExecuteDuplicateRequestIdempotent',
  'TestPluginExecuteDuplicateInFlight',
  'TestPluginExecuteCanceledBeforeStart',
  'TestPluginExecuteInvalidRequest',
  'TestPluginExecuteNotInvocableRollsBack'
]

export const SOURCE_GUARDS = [
  'ErrActivationTimeout   = errors.New("plugin execute: activation timed out")',
  'func (i *Installer) Execute(ctx context.Context, pluginID string, request ExecuteRequest) (ExecuteResult, error) {',
  'if cached, ok := results[request.ID]; ok {',
  'if inflight[request.ID] {',
  'invoker, ok := handle.(RuntimeInvoker)',
  'i.rollbackColdStart(pluginID, handle, cold)',
  'if _, err := machine.Apply(ctx, EventActivate, machine.Epoch()+1); err != nil {',
  '_, _ = machine.Apply(context.Background(), EventDeactivate, machine.Epoch()+1)',
  'if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {',
  'i.results[request.ID] = result'
]

export const ASSERT_GUARDS = [
  'if spawner.Calls() != 1 || spawner.Last().Invocations() != 2 {',
  'if !errors.Is(err, ErrExecuteDenied) {',
  'if !errors.Is(err, ErrActivationTimeout) {',
  'if !errors.Is(err, context.DeadlineExceeded) {',
  'if !errors.Is(err, ErrExecuteFailed) || !errors.Is(err, failure) {',
  'if !runtime.Closed() {',
  'if !second.Reused || string(second.Output) != string(first.Output) {',
  'if !errors.Is(err, ErrRequestInFlight) {',
  'if !errors.Is(err, context.Canceled) {',
  'if !errors.Is(err, ErrRuntimeNotInvocable) {'
]

export const FORBIDDEN_TOKENS = ['go func(', 'time.Sleep(', 'recover()', 'panic(']

export const EVIDENCE_ROWS = [
  'cold spawns=1 invokes=2 state=activated',
  'installed denied=ErrExecuteDenied spawns=0',
  'timeout rollback=true state=enabled processes=0 cause=DeadlineExceeded',
  'spawn_error rollback=true chained=true state=enabled',
  'invoke_error rollback=true closed=true state=enabled',
  'duplicate reused=true invokes=1',
  'in_flight rejected=ErrRequestInFlight invokes=1',
  'cancel spawns=0 err=context.Canceled',
  'invalid empty_id=ErrExecuteInvalid unknown=ErrInstallerState',
  'not_invocable rejected=ErrRuntimeNotInvocable state=enabled'
]

export function parseExecuteRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m06-s03 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm06-s03 '.length).trim())
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
  return missingGuards(text, SOURCE_GUARDS, 'execute path')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'execute test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return EXECUTE_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `execute test is missing ${name}`)
}

export function validateNoBackgroundWork(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `execute path must not contain ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(EXECUTE_TEST)) problems.push(`default build leaked ${EXECUTE_TEST}`)
  if (!taggedTests.includes(EXECUTE_TEST)) problems.push(`tagged tests missing ${EXECUTE_TEST}`)
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
  const guards = SOURCE_GUARDS.length + ASSERT_GUARDS.length + EXECUTE_CASES.length
  const lines = [
    `invocation=execute package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${EXECUTE_TEST}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m06-s03 ${row}`)
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

export function runExecuteEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement')
  const source = readFileSync(resolve(base, EXECUTE_SOURCE), 'utf8')
  const testSource = readFileSync(resolve(base, EXECUTE_TEST), 'utf8')
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
    rows: parseExecuteRows(log),
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
  const evidence = runExecuteEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(evidence.source),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoBackgroundWork(evidence.source)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_EXECUTE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
