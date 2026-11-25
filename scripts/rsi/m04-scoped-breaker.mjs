import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/trusted_service'
export const RUN_PATTERN = 'TestScopedBreaker'
export const EXIT_SCOPED_BREAKER = 46
export const EXPECTED_PASSED = 14
export const REQUIRED_SOURCES = ['scoped_breaker.go', 'circuit_breaker.go']
export const REQUIRED_TESTS = ['scoped_breaker_test.go']
export const BREAKER_CASES = [
  'invalid-keys',
  'scope-isolation',
  'threshold',
  'probe-cap',
  'probe-success',
  'probe-failure',
  'probe-abort',
  'concurrent-probe',
  'concurrent-keys',
  'open-list',
  'reset-local',
  'snapshot',
  'config-defaults',
  'closed-no-probe'
]
export const BREAKER_GUARDS = [
  'ErrCircuitHalfOpenExhausted',
  'func (c *CircuitBreaker) TryAcquire() error',
  'c.halfOpenAttempts >= c.config.HalfOpenAttempts',
  'func (c *CircuitBreaker) AbortProbe()',
  'c.halfOpenAttempts++',
  'c.halfOpenAttempts--',
  'breakerKeySeparator',
  'c.now().Sub(c.openedAt) >= c.config.RecoveryAfter',
  'sort.Slice(snapshot',
  'entry.State != CircuitOpen',
  'strings.TrimSpace(scopeKey)'
]
export const SCOPED_BREAKER_GUARDS = ['func (r *ScopedBreakers) OpenInScope(scopeKey string) ([]BreakerKey, error)']

const INVALID_PATTERN = /m04-s06 breaker invalid-keys invalid=(\d+) accepted=(\d+) trimmed=(true|false) registered=(\d+)/
const SCOPE_PATTERN = /m04-s06 breaker scope-isolation tripped=(\w+) otherScope=(\w+) otherService=(\w+) otherProvider=(\w+)/
const THRESHOLD_PATTERN = /m04-s06 breaker threshold threshold=(\d+) closedAt=(\d+) openAt=(\d+) openAfterExtra=(true|false) failures=(\d+)/
const PROBE_CAP_PATTERN = /m04-s06 breaker probe-cap probes=(\d+) denied=(\d+) state=(\w+)/
const PROBE_SUCCESS_PATTERN = /m04-s06 breaker probe-success successes=(\d+) state=(\w+) failures=(\d+) probes=(\d+)/
const PROBE_FAILURE_PATTERN = /m04-s06 breaker probe-failure reopened=(true|false) probes=(\d+) denied=(\d+) state=(\w+)/
const PROBE_ABORT_PATTERN = /m04-s06 breaker probe-abort granted=(\d+) aborted=(\d+) probes=(\d+) state=(\w+)/
const CONCURRENT_PROBE_PATTERN = /m04-s06 breaker concurrent-probe workers=(\d+) granted=(\d+) denied=(\d+) other=(\d+)/
const CONCURRENT_KEYS_PATTERN = /m04-s06 breaker concurrent-keys keys=(\d+) open=(\d+) failures=(\d+) untouched=(\w+)/
const OPEN_LIST_PATTERN = /m04-s06 breaker open-list scope-a=(\d+) scope-b=(\d+) missing=(\d+) order=(\w+) rejected=(true|false)/
const RESET_PATTERN = /m04-s06 breaker reset-local reset=(\w+) peer=(\w+) peerFailures=(\d+)/
const SNAPSHOT_PATTERN = /m04-s06 breaker snapshot entries=(\d+) sorted=(true|false) stable=(true|false) open=(\d+)/
const DEFAULTS_PATTERN = /m04-s06 breaker config-defaults threshold=(\d+) openAt=(\d+) failures=(\d+)/
const CLOSED_PATTERN = /m04-s06 breaker closed-no-probe closedAcquires=(\d+) probes=(\d+) halfOpenStart=(\d+)/

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

function parseNamed(text, pattern) {
  const match = text.match(pattern)
  if (!match) return null
  return match.slice(1).map((value) => (/^\d+$/.test(value) ? Number(value) : value))
}

export function parseBreakerRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = {}
  for (const name of BREAKER_CASES) {
    rows[name] = parseNamed(body, rowPattern(name))
  }
  return rows
}

export function rowPattern(name) {
  switch (name) {
    case 'invalid-keys':
      return INVALID_PATTERN
    case 'scope-isolation':
      return SCOPE_PATTERN
    case 'threshold':
      return THRESHOLD_PATTERN
    case 'probe-cap':
      return PROBE_CAP_PATTERN
    case 'probe-success':
      return PROBE_SUCCESS_PATTERN
    case 'probe-failure':
      return PROBE_FAILURE_PATTERN
    case 'probe-abort':
      return PROBE_ABORT_PATTERN
    case 'concurrent-probe':
      return CONCURRENT_PROBE_PATTERN
    case 'concurrent-keys':
      return CONCURRENT_KEYS_PATTERN
    case 'open-list':
      return OPEN_LIST_PATTERN
    case 'reset-local':
      return RESET_PATTERN
    case 'snapshot':
      return SNAPSHOT_PATTERN
    case 'config-defaults':
      return DEFAULTS_PATTERN
    default:
      return CLOSED_PATTERN
  }
}

function countMatches(text, pattern) {
  return (text.match(new RegExp(pattern.source, 'g')) ?? []).length
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateBreakerSource(text) {
  return missingGuards(text, BREAKER_GUARDS, 'circuit breaker source')
}

export function validateScopedSource(text) {
  return missingGuards(text, SCOPED_BREAKER_GUARDS, 'scoped breaker source')
}

function expectRow(problems, rows, name, expected) {
  const row = rows[name]
  if (!Array.isArray(row)) {
    problems.push(`evidence row ${name} is missing`)
    return
  }
  expected.forEach((value, index) => {
    if (row[index] !== value) problems.push(`evidence row ${name} column ${index} is ${row[index]} want ${value}`)
  })
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

  const rows = evidence?.rows ?? {}
  const log = typeof evidence?.log === 'string' ? evidence.log : ''
  for (const name of BREAKER_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  expectRow(problems, rows, 'invalid-keys', [7, 1, 'true', 0])
  expectRow(problems, rows, 'scope-isolation', ['open', 'closed', 'closed', 'closed'])
  expectRow(problems, rows, 'threshold', [3, 2, 3, 'true', 4])
  expectRow(problems, rows, 'probe-cap', [2, 1, 'half_open'])
  expectRow(problems, rows, 'probe-success', [2, 'closed', 0, 0])
  expectRow(problems, rows, 'probe-failure', ['true', 0, 1, 'open'])
  expectRow(problems, rows, 'probe-abort', [3, 3, 0, 'half_open'])
  expectRow(problems, rows, 'concurrent-probe', [16, 1, 15, 0])
  expectRow(problems, rows, 'concurrent-keys', [8, 8, 4, 'closed'])
  expectRow(problems, rows, 'open-list', [2, 1, 0, 'ascending', 'true'])
  expectRow(problems, rows, 'reset-local', ['closed', 'open', 3])
  expectRow(problems, rows, 'snapshot', [4, 'true', 'true', 1])
  expectRow(problems, rows, 'config-defaults', [5, 5, 5])
  expectRow(problems, rows, 'closed-no-probe', [5, 0, 0])

  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  if (evidence?.race !== true) problems.push('go test must run with -race')
  return problems
}

export function renderEvidence(evidence) {
  const rows = evidence?.rows ?? {}
  const value = (name, index, fallback = 0) => {
    const row = rows[name]
    return Array.isArray(row) ? row[index] : fallback
  }
  const goFiles = evidence?.goFiles ?? []
  const testFiles = evidence?.testFiles ?? []
  const source = evidence?.source ?? ''
  return [
    `invocation=scoped-breaker package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${BREAKER_CASES.length} guards=${BREAKER_GUARDS.length + SCOPED_BREAKER_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `breaker guards frozen=${BREAKER_GUARDS.every((guard) => source.includes(guard))} scoped guards frozen=${SCOPED_BREAKER_GUARDS.every((guard) => source.includes(guard))}`,
    `m04-s06 breaker invalid-keys invalid=${value('invalid-keys', 0)} accepted=${value('invalid-keys', 1)} trimmed=${value('invalid-keys', 2, 'false')} registered=${value('invalid-keys', 3)}`,
    `m04-s06 breaker scope-isolation tripped=${value('scope-isolation', 0, '')} otherScope=${value('scope-isolation', 1, '')} otherService=${value('scope-isolation', 2, '')} otherProvider=${value('scope-isolation', 3, '')}`,
    `m04-s06 breaker threshold threshold=${value('threshold', 0)} closedAt=${value('threshold', 1)} openAt=${value('threshold', 2)} openAfterExtra=${value('threshold', 3, 'false')} failures=${value('threshold', 4)}`,
    `m04-s06 breaker probe-cap probes=${value('probe-cap', 0)} denied=${value('probe-cap', 1)} state=${value('probe-cap', 2, '')}`,
    `m04-s06 breaker probe-success successes=${value('probe-success', 0)} state=${value('probe-success', 1, '')} failures=${value('probe-success', 2)} probes=${value('probe-success', 3)}`,
    `m04-s06 breaker probe-failure reopened=${value('probe-failure', 0, 'false')} probes=${value('probe-failure', 1)} denied=${value('probe-failure', 2)} state=${value('probe-failure', 3, '')}`,
    `m04-s06 breaker probe-abort granted=${value('probe-abort', 0)} aborted=${value('probe-abort', 1)} probes=${value('probe-abort', 2)} state=${value('probe-abort', 3, '')}`,
    `m04-s06 breaker concurrent-probe workers=${value('concurrent-probe', 0)} granted=${value('concurrent-probe', 1)} denied=${value('concurrent-probe', 2)} other=${value('concurrent-probe', 3)}`,
    `m04-s06 breaker concurrent-keys keys=${value('concurrent-keys', 0)} open=${value('concurrent-keys', 1)} failures=${value('concurrent-keys', 2)} untouched=${value('concurrent-keys', 3, '')}`,
    `m04-s06 breaker open-list scope-a=${value('open-list', 0)} scope-b=${value('open-list', 1)} missing=${value('open-list', 2)} order=${value('open-list', 3, '')} rejected=${value('open-list', 4, 'false')}`,
    `m04-s06 breaker reset-local reset=${value('reset-local', 0, '')} peer=${value('reset-local', 1, '')} peerFailures=${value('reset-local', 2)}`,
    `m04-s06 breaker snapshot entries=${value('snapshot', 0)} sorted=${value('snapshot', 1, 'false')} stable=${value('snapshot', 2, 'false')} open=${value('snapshot', 3)}`,
    `m04-s06 breaker config-defaults threshold=${value('config-defaults', 0)} openAt=${value('config-defaults', 1)} failures=${value('config-defaults', 2)}`,
    `m04-s06 breaker closed-no-probe closedAcquires=${value('closed-no-probe', 0)} probes=${value('closed-no-probe', 1)} halfOpenStart=${value('closed-no-probe', 2)}`,
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`
  ].join('\n') + '\n'
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

export function runBreakerEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'trusted_service')
  const source = [readFileSync(resolve(base, 'circuit_breaker.go'), 'utf8'), readFileSync(resolve(base, 'scoped_breaker.go'), 'utf8')].join('\n')
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
    rows: parseBreakerRows(log),
    source,
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
      process.stdout.write(`invocation=scoped-breaker package=${PACKAGE_PATH} cases=${BREAKER_CASES.join(',')}\n`)
      return
    }
    const evidence = runBreakerEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [...validateBreakerSource(evidence.source), ...validateScopedSource(evidence.source), ...validateEvidence(evidence)]
    if (reportProblems(problems)) process.exit(EXIT_SCOPED_BREAKER)
    if (evidence.exitCode !== 0) process.exit(EXIT_SCOPED_BREAKER)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_SCOPED_BREAKER)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
