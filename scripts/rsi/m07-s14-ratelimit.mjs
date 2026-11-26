import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestEventLimiter'
export const BUILD_TAG = 'rsi_ratelimit'
export const EXIT_RATELIMIT = 86
export const EXPECTED_PASSED = 12
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const LIMITER_FILE = 'event_rate_limit.go'
export const LIMITER_TEST_FILE = 'event_rate_limit_test.go'

export const EVIDENCE_ROWS = [
  'admit pending=2 dead=0',
  'plugin_rate dead=1 reason=plugin_rate',
  'topic_rate dead=1 reason=topic_rate',
  'retry after_ms>0 http=429',
  'queue_full dead=1 reason=queue_full',
  'dead_bounded len=2 dropped=1',
  'trace count=2 ok=true',
  'window_reset ok=true',
  'config invalid=4',
  'names invalid=2',
  'concurrent admitted=5',
  'complete frees=true'
]

export const CASES = [
  'TestEventLimiterAdmitsWithinLimits',
  'TestEventLimiterPluginRate',
  'TestEventLimiterTopicRate',
  'TestEventLimiterRetryable',
  'TestEventLimiterQueueFull',
  'TestEventLimiterDeadLetterBounded',
  'TestEventLimiterTrace',
  'TestEventLimiterWindowReset',
  'TestEventLimiterInvalidConfig',
  'TestEventLimiterInvalidNames',
  'TestEventLimiterConcurrent',
  'TestEventLimiterCompleteFreesCapacity'
]

export const SOURCE_GUARDS = [
  'if l.pending >= l.config.Capacity {',
  'if ok, wait := l.windowLocked(l.pluginWindows, plugin, l.config.PerPluginPerSecond, now); !ok {',
  'if ok, wait := l.windowLocked(l.topicWindows, topic, l.config.PerTopicPerSecond, now); !ok {',
  'l.pluginWindows[plugin].count++',
  'l.topicWindows[topic].count++',
  'l.deadLetters = l.deadLetters[1:]',
  'return WrapTaskError(ErrTaskQueueFull, "event rate limited", ErrRateLimited)',
  'if record.Plugin == plugin && record.Topic == topic && record.Reason == reason {'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s14 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s14 '.length).trim())
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
  if (defaultFiles.includes(LIMITER_FILE)) problems.push(`default build leaked ${LIMITER_FILE}`)
  if (!taggedFiles.includes(LIMITER_FILE)) problems.push(`tagged build missing ${LIMITER_FILE}`)
  if (defaultTests.includes(LIMITER_TEST_FILE)) problems.push(`default tests leaked ${LIMITER_TEST_FILE}`)
  if (!taggedTests.includes(LIMITER_TEST_FILE)) problems.push(`tagged tests missing ${LIMITER_TEST_FILE}`)
  if (taggedFiles.length !== defaultFiles.length + 1) problems.push(`tagged files=${taggedFiles.length} want default+1=${defaultFiles.length + 1}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', LIMITER_FILE)
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `event limiter is missing ${guard}`)
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `event limiter test is missing ${name}`)
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
    `invocation=ratelimit package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s14 ${row}`)
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

export function runRateLimitEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', LIMITER_TEST_FILE), 'utf8')
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
  const evidence = runRateLimitEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_RATELIMIT)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
