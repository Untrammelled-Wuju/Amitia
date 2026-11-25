import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestLedger'
export const EXIT_LEDGER = 42
export const EXPECTED_PASSED = 9
export const REQUIRED_SOURCES = ['ledger.go']
export const REQUIRED_TESTS = ['ledger_test.go']
export const LEDGER_CASES = [
  'invalid-keys',
  'replay-runs-once',
  'retry-after-failure',
  'concurrent',
  'cross-scope',
  'payload-conflict',
  'binding-frozen',
  'unknown-blocks-retry',
  'restart-replay'
]
export const EFFECT_STATES = ['pending', 'committed', 'rejected', 'unknown']
export const LEDGER_CONSTRAINTS = [
  'PRIMARY KEY(scope_key,idempotency_key)',
  'ON CONFLICT(scope_key,idempotency_key) DO NOTHING',
  "CHECK(effect_state IN ('pending','committed','rejected','unknown'))"
]

const INVALID_KEYS_PATTERN = /m04-s02 ledger invalid-keys rejected=(\d+) executed=(\d+)/
const REPLAY_PATTERN = /m04-s02 ledger replay-runs-once executions=(\d+) state=(\w+) result=(\S+)/
const RETRY_PATTERN = /m04-s02 ledger retry-after-failure attempts=(\d+) state=(\w+)/
const CONCURRENT_PATTERN = /m04-s02 ledger concurrent workers=(\d+) executions=(\d+) leaders=(\d+)/
const CROSS_SCOPE_PATTERN = /m04-s02 ledger cross-scope executions=(\d+) rows=(\d+)/
const PAYLOAD_PATTERN = /m04-s02 ledger payload-conflict executions=(\d+) conflict=(.+)/
const BINDING_PATTERN = /m04-s02 ledger binding-frozen revision=(\S+) executions=(\d+)/
const UNKNOWN_PATTERN = /m04-s02 ledger unknown-blocks-retry executions=(\d+) state=(\w+) retrySafe=(true|false)/
const RESTART_PATTERN = /m04-s02 ledger restart-replay executions=(\d+) state=(\w+)/

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

export function parseLedgerRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    'invalid-keys': parseNamed(body, INVALID_KEYS_PATTERN),
    'replay-runs-once': parseNamed(body, REPLAY_PATTERN),
    'retry-after-failure': parseNamed(body, RETRY_PATTERN),
    concurrent: parseNamed(body, CONCURRENT_PATTERN),
    'cross-scope': parseNamed(body, CROSS_SCOPE_PATTERN),
    'payload-conflict': parseNamed(body, PAYLOAD_PATTERN),
    'binding-frozen': parseNamed(body, BINDING_PATTERN),
    'unknown-blocks-retry': parseNamed(body, UNKNOWN_PATTERN),
    'restart-replay': parseNamed(body, RESTART_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'invalid-keys':
      return INVALID_KEYS_PATTERN
    case 'replay-runs-once':
      return REPLAY_PATTERN
    case 'retry-after-failure':
      return RETRY_PATTERN
    case 'concurrent':
      return CONCURRENT_PATTERN
    case 'cross-scope':
      return CROSS_SCOPE_PATTERN
    case 'payload-conflict':
      return PAYLOAD_PATTERN
    case 'binding-frozen':
      return BINDING_PATTERN
    case 'unknown-blocks-retry':
      return UNKNOWN_PATTERN
    default:
      return RESTART_PATTERN
  }
}

function countMatches(text, pattern) {
  return (text.match(new RegExp(pattern.source, 'g')) ?? []).length
}

export function validateLedgerSource(text) {
  const body = String(text ?? '')
  const problems = []
  for (const constraint of LEDGER_CONSTRAINTS) {
    if (!body.includes(constraint)) problems.push(`ledger schema is missing ${constraint}`)
  }
  for (const state of EFFECT_STATES) {
    if (!body.includes(`Effect${state.charAt(0).toUpperCase()}${state.slice(1)}`)) problems.push(`effect state ${state} has no exported constant`)
  }
  return problems
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
  for (const name of LEDGER_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  const invalid = rows['invalid-keys']
  if (Array.isArray(invalid)) {
    if (invalid[0] !== 7) problems.push(`invalid keys rejected=${invalid[0]} want 7`)
    if (invalid[1] !== 0) problems.push(`invalid keys executed=${invalid[1]} want 0`)
  }

  const replay = rows['replay-runs-once']
  if (Array.isArray(replay)) {
    if (replay[0] !== 1) problems.push(`replay executions=${replay[0]} want 1`)
    if (replay[1] !== 'committed') problems.push(`replay state=${replay[1]} want committed`)
    if (replay[2] !== '{"value":"once"}') problems.push(`replay result=${replay[2]} is not the recorded result`)
  }

  const retry = rows['retry-after-failure']
  if (Array.isArray(retry)) {
    if (retry[0] !== 2) problems.push(`definite failure attempts=${retry[0]} want 2`)
    if (retry[1] !== 'committed') problems.push(`retry state=${retry[1]} want committed`)
  }

  const concurrent = rows.concurrent
  if (Array.isArray(concurrent)) {
    if (concurrent[0] !== 16) problems.push(`concurrent workers=${concurrent[0]} want 16`)
    if (concurrent[1] !== 1) problems.push(`concurrent executions=${concurrent[1]} want 1`)
    if (concurrent[2] !== 1) problems.push(`concurrent leaders=${concurrent[2]} want 1`)
  }

  const crossScope = rows['cross-scope']
  if (Array.isArray(crossScope)) {
    if (crossScope[0] !== 2) problems.push(`cross-scope executions=${crossScope[0]} want 2`)
    if (crossScope[1] !== 2) problems.push(`cross-scope rows=${crossScope[1]} want 2`)
  }

  const payload = rows['payload-conflict']
  if (Array.isArray(payload)) {
    if (payload[0] !== 1) problems.push(`payload conflict executions=${payload[0]} want 1`)
    if (!payload[1].includes('ErrLedgerPayloadConflict') && !payload[1].includes('请求内容已变化')) {
      problems.push(`payload conflict error=${payload[1]} does not name the conflict`)
    }
  }

  const binding = rows['binding-frozen']
  if (Array.isArray(binding)) {
    if (binding[0] !== 'rev-7') problems.push(`frozen revision=${binding[0]} want rev-7`)
    if (binding[1] !== 1) problems.push(`binding frozen executions=${binding[1]} want 1`)
  }

  const unknown = rows['unknown-blocks-retry']
  if (Array.isArray(unknown)) {
    if (unknown[0] !== 1) problems.push(`unknown outcome executions=${unknown[0]} want 1`)
    if (unknown[1] !== 'unknown') problems.push(`unknown outcome state=${unknown[1]} want unknown`)
    if (unknown[2] !== 'false') problems.push('an unknown outcome must not be retry safe')
  }

  const restart = rows['restart-replay']
  if (Array.isArray(restart)) {
    if (restart[0] !== 1) problems.push(`restart replay executions=${restart[0]} want 1`)
    if (restart[1] !== 'committed') problems.push(`restart replay state=${restart[1]} want committed`)
  }

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
  return [
    `invocation=ledger package=${PACKAGE_PATH} run=${RUN_PATTERN} states=${EFFECT_STATES.join(',')} cases=${LEDGER_CASES.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((source) => goFiles.includes(source)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `ledger schema constraints=${LEDGER_CONSTRAINTS.length} frozen=${LEDGER_CONSTRAINTS.every((item) => evidence?.source?.includes(item))}`,
    `m04-s02 ledger invalid-keys rejected=${value('invalid-keys', 0)} executed=${value('invalid-keys', 1)}`,
    `m04-s02 ledger replay-runs-once executions=${value('replay-runs-once', 0)} state=${value('replay-runs-once', 1, '')} result=${value('replay-runs-once', 2, '')}`,
    `m04-s02 ledger retry-after-failure attempts=${value('retry-after-failure', 0)} state=${value('retry-after-failure', 1, '')}`,
    `m04-s02 ledger concurrent workers=${value('concurrent', 0)} executions=${value('concurrent', 1)} leaders=${value('concurrent', 2)}`,
    `m04-s02 ledger cross-scope executions=${value('cross-scope', 0)} rows=${value('cross-scope', 1)}`,
    `m04-s02 ledger payload-conflict executions=${value('payload-conflict', 0)} conflict=${value('payload-conflict', 1, '')}`,
    `m04-s02 ledger binding-frozen revision=${value('binding-frozen', 0, '')} executions=${value('binding-frozen', 1)}`,
    `m04-s02 ledger unknown-blocks-retry executions=${value('unknown-blocks-retry', 0)} state=${value('unknown-blocks-retry', 1, '')} retrySafe=${value('unknown-blocks-retry', 2, 'true')}`,
    `m04-s02 ledger restart-replay executions=${value('restart-replay', 0)} state=${value('restart-replay', 1, '')}`,
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

export function runLedger(root) {
  const listed = goList(root)
  const source = readFileSync(resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal', 'ledger.go'), 'utf8')
  const result = spawnSync('go', ['test', '-race', '-count=1', '-timeout', '180s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
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
    rows: parseLedgerRows(log),
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
      process.stdout.write(`invocation=ledger package=${PACKAGE_PATH} cases=${LEDGER_CASES.join(',')}\n`)
      return
    }
    const evidence = runLedger(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [...validateLedgerSource(evidence.source), ...validateEvidence(evidence)]
    if (reportProblems(problems)) process.exit(EXIT_LEDGER)
    if (evidence.exitCode !== 0) process.exit(EXIT_LEDGER)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_LEDGER)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
