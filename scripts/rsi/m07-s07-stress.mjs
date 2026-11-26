import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestCoreStressInjection'
export const BUILD_TAGS = 'rsi_backpressure rsi_stress'
export const EXIT_STRESS = 79
export const EXPECTED_PASSED = 1
export const GO_TEST_FLAGS = ['-tags', BUILD_TAGS, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const STRESS_FILE = 'stress_injection_test.go'

export const EVIDENCE_ROWS = [
  'storm events=10000 accepted=1024 rejected=8976 depth=1024',
  'memory within_budget=true limit_mb=64',
  'latency within_budget=true budget_ms=50',
  'goroutines within_budget=true delta=0',
  'oom breaker_open=true plugin_calls=3 user_admitted=true',
  'spin canceled=true breaker_open=true',
  'blocked user_admitted=true',
  'invalid rejected=true cancel=true',
  'restart depth=0'
]

export const CASES = ['TestCoreStressInjection']

export const SOURCE_GUARDS = [
  'if accepted != 1024 || rejected != 8976 || queue.Len() != 1024 {',
  'if heapDelta > 64<<20 {',
  'if maxLatency > 50*time.Millisecond {',
  'if goroutinesDelta > 8 {',
  'if !breaker.Open() || breaker.Allow() {',
  'if !errors.Is(spinErr, context.DeadlineExceeded) {',
  'if err := blocked.Enqueue(context.Background(), QueueItem{ID: "user", Priority: PriorityUserChat, Bytes: 8}); err != nil {',
  'if restarted.Len() != 0 || restarted.Stats().Accepted != 0 {'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s07 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s07 '.length).trim())
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
  if (defaultTests.includes(STRESS_FILE)) problems.push(`default tests leaked ${STRESS_FILE}`)
  if (!taggedTests.includes(STRESS_FILE)) problems.push(`tagged tests missing ${STRESS_FILE}`)
  if (!taggedFiles.includes('bounded_queue.go')) problems.push('tagged build missing bounded_queue.go')
  if (taggedFiles.length !== defaultFiles.length + 1) problems.push(`tagged files=${taggedFiles.length} want default+1=${defaultFiles.length + 1}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', STRESS_FILE)
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `stress test is missing ${guard}`)
}

export function validateUserPath(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', STRESS_FILE)
  const body = readFileSync(path, 'utf8')
  const problems = []
  if (!body.includes('plugin_calls=%d user_admitted=true')) problems.push('oom evidence must record plugin calls and user admission')
  if (body.split('runOOMPlugin(256 << 10)').length - 1 !== 1) problems.push('oom injection must run exactly once')
  return problems
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `stress test is missing ${name}`)
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
    `invocation=stress package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s07 ${row}`)
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

export function runStressEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAGS])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', STRESS_FILE), 'utf8')
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
  const evidence = runStressEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validateUserPath(root),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_STRESS)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
