import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/event'
export const RUN_PATTERN = 'TestScoped'
export const BUILD_TAG = 'rsi_eventing'
export const EXIT_EVENTING = 74
export const EXPECTED_PASSED = 13
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const REGISTRY_FILE = 'scope_registry.go'
export const REGISTRY_TEST_FILE = 'scope_registry_test.go'

export const EVIDENCE_ROWS = [
  'subscribe created=true generation=1',
  'duplicate created=false generation=1 subscriptions=1',
  'conflict rejected=true filter="kind=chat"',
  'reject invalid=4 subscriptions=0',
  'unsubscribe first=true second=false subscriptions=0',
  'deliver events=10 calls=10 per_event=1',
  'scope exact_a=1 exact_b=1 wildcard=2 cross=0',
  'cold wakes_on_subscribe=0 wakes_on_dispatch=0 calls=5',
  'unsubscribed wakes=0 calls=0',
  'wake failed=1 delivered=1 wakes=2',
  'handler errors_joined=true delivered=1 failed=1',
  'concurrent goroutines=32 created=1 duplicates=31',
  'cancel subscribe=context.Canceled dispatch=context.Canceled calls=0',
  'resubscribe generation=2 deliveries=2'
]

export const CASES = [
  'TestScopedSubscribeIsIdempotent',
  'TestScopedSubscribeConflict',
  'TestScopedSubscribeInvalid',
  'TestScopedUnsubscribeIsIdempotent',
  'TestScopedDeliverOncePerKey',
  'TestScopedScopeIsolation',
  'TestScopedColdSubscribeDoesNotWake',
  'TestScopedUnsubscribeStopsDispatch',
  'TestScopedWakeFailureSkipsHandler',
  'TestScopedHandlerErrorIsolated',
  'TestScopedConcurrentSubscribe',
  'TestScopedCancel',
  'TestScopedResubscribeGeneration'
]

export const SOURCE_GUARDS = [
  'if existing.filter.Canonical() != filter {',
  'existing.duplicates++',
  'generation := r.epochs[key] + 1',
  'if _, ok := r.entries[normalized]; !ok {',
  'if entry.key.Scope != "*" && entry.key.Scope != strings.TrimSpace(event.Scope) {',
  'if !entry.filter.Matches(event.Fields) {',
  'if entry.wake && activator != nil {',
  'if err := activator.EnsureActive(ctx, entry.key.Owner); err != nil {'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s02 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s02 '.length).trim())
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
  if (defaultFiles.includes(REGISTRY_FILE)) problems.push(`default build leaked ${REGISTRY_FILE}`)
  if (!taggedFiles.includes(REGISTRY_FILE)) problems.push(`tagged build missing ${REGISTRY_FILE}`)
  if (defaultTests.includes(REGISTRY_TEST_FILE)) problems.push(`default tests leaked ${REGISTRY_TEST_FILE}`)
  if (!taggedTests.includes(REGISTRY_TEST_FILE)) problems.push(`tagged tests missing ${REGISTRY_TEST_FILE}`)
  if (taggedFiles.length !== defaultFiles.length + 1) problems.push(`tagged files=${taggedFiles.length} want default+1=${defaultFiles.length + 1}`)
  return problems
}

export function validateNoWakeOnSubscribe(root) {
  const problems = []
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'event', REGISTRY_FILE)
  const body = readFileSync(path, 'utf8')
  const subscribe = body.indexOf('func (r *ScopedRegistry) Subscribe(')
  const dispatch = body.indexOf('func (r *ScopedRegistry) Dispatch(')
  if (subscribe < 0) problems.push('scope registry has no Subscribe')
  if (dispatch < 0) problems.push('scope registry has no Dispatch')
  if (subscribe >= 0 && dispatch > subscribe) {
    const subscribeBody = body.slice(subscribe, dispatch)
    if (subscribeBody.includes('EnsureActive')) problems.push('Subscribe must not activate cold owners')
  }
  const ensureCalls = body.split('EnsureActive(').length - 1
  if (ensureCalls !== 2) problems.push(`EnsureActive must appear exactly twice (interface + dispatch), got ${ensureCalls}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'event', REGISTRY_FILE)
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `scope registry is missing ${guard}`)
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `scoped event test is missing ${name}`)
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
    `invocation=eventing package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s02 ${row}`)
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

export function runEventingEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'event', REGISTRY_TEST_FILE), 'utf8')
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
  const evidence = runEventingEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateNoWakeOnSubscribe(root),
    ...validateSource(root),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_EVENTING)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
