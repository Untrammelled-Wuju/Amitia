import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestBoundedQueue'
export const BUILD_TAG = 'rsi_backpressure'
export const EXIT_BACKPRESSURE = 75
export const EXPECTED_PASSED = 14
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const QUEUE_FILE = 'bounded_queue.go'
export const QUEUE_TEST_FILE = 'bounded_queue_test.go'

export const EVIDENCE_ROWS = [
  'existing max=16 depth=100 overflow=true',
  'capacity accepted=256 rejected=9744 depth=256',
  'bytes accepted=8 bytes=4096 limit=4096',
  'oversize rejected=true retryable=false',
  'retryable=true retry_after=25ms http=429 depth=4',
  'preempt user=true evicted=1 depth=4',
  'no_preempt peer=true depth=4',
  'order=chat,foreground,experiment,background',
  'aging promoted=true wait=1',
  'aging user_guard=true',
  'reserve experiments=6 rejected=1 chat=2',
  'pressure medium=true high=true critical=true shed=true',
  'concurrent goroutines=32 accepted=128 depth=128',
  'cancel enqueue=context.Canceled dequeue=context.Canceled depth=0'
]

export const CASES = [
  'TestBoundedQueueExistingPriorityQueueOverflows',
  'TestBoundedQueueCapacityBound',
  'TestBoundedQueueByteBudget',
  'TestBoundedQueueOversizeRejected',
  'TestBoundedQueueRetryableRejection',
  'TestBoundedQueueUserPreemptsExperiment',
  'TestBoundedQueuePeerNotPreempted',
  'TestBoundedQueuePriorityOrder',
  'TestBoundedQueueAgingFairness',
  'TestBoundedQueueAgedDoesNotOutrankChat',
  'TestBoundedQueueReservedUserSlots',
  'TestBoundedQueuePressureAndShed',
  'TestBoundedQueueConcurrentEnqueue',
  'TestBoundedQueueCancel'
]

export const SOURCE_GUARDS = [
  'if item.Priority > PriorityUserForeground {',
  'if q.stats.Depth < q.config.MaxItems && q.bytes+item.Bytes <= q.config.MaxBytes {',
  'if victim := q.evictLowerLocked(item.Priority); victim != "" {',
  'RetryAfter: q.config.RetryAfter,',
  'steps := int(now.Sub(entry.enqueuedAt) / q.config.AgingAfter)',
  'case ratio >= 0.9:',
  'if level == PressureHigh && priority >= PriorityExperiment {',
  'if level == PressureCritical && priority >= PriorityUserForeground {'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s03 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s03 '.length).trim())
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
  if (defaultFiles.includes(QUEUE_FILE)) problems.push(`default build leaked ${QUEUE_FILE}`)
  if (!taggedFiles.includes(QUEUE_FILE)) problems.push(`tagged build missing ${QUEUE_FILE}`)
  if (defaultTests.includes(QUEUE_TEST_FILE)) problems.push(`default tests leaked ${QUEUE_TEST_FILE}`)
  if (!taggedTests.includes(QUEUE_TEST_FILE)) problems.push(`tagged tests missing ${QUEUE_TEST_FILE}`)
  if (taggedFiles.length !== defaultFiles.length + 1) problems.push(`tagged files=${taggedFiles.length} want default+1=${defaultFiles.length + 1}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', QUEUE_FILE)
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `bounded queue is missing ${guard}`)
}

export function validateHardBound(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', QUEUE_FILE)
  const body = readFileSync(path, 'utf8')
  const problems = []
  if (body.includes('isP0P1')) problems.push('bounded queue must not copy the legacy priority bypass')
  const appends = body.split('q.appendLocked(item)').length - 1
  if (appends !== 2) problems.push(`append sites=${appends} want 2 (admit + preempt)`)
  return problems
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `bounded queue test is missing ${name}`)
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
    `invocation=backpressure package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s03 ${row}`)
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

export function runBackpressureEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', QUEUE_TEST_FILE), 'utf8')
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
  const evidence = runBackpressureEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validateHardBound(root),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_BACKPRESSURE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
