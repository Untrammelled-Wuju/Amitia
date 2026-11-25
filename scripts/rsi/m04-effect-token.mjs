import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestEffect'
export const EXIT_EFFECT_TOKEN = 44
export const EXPECTED_PASSED = 14
export const REQUIRED_SOURCES = ['effect_token.go', 'ledger.go']
export const REQUIRED_TESTS = ['effect_token_test.go']
export const EFFECT_CASES = [
  'token-binds',
  'token-mutating',
  'timeout-after-write',
  'same-key',
  'payload-conflict',
  'authority-payload-mismatch',
  'authority-token-forged',
  'authority-unavailable',
  'authority-absent',
  'retry-safe',
  'cancelled',
  'restart',
  'concurrent',
  'nil-authority'
]
export const EFFECT_GUARDS = [
  'ErrEffectTokenInvalid',
  'ErrEffectRecordConflict',
  'subtle.ConstantTimeCompare',
  'EffectTokenSchema',
  'ON CONFLICT(scope_key,idempotency_key,attempt) DO NOTHING',
  "CHECK(state IN ('issued','redeemed','abandoned'))",
  'EffectResolvedAbsent',
  'EffectResolvedUndetermined',
  '未提供权威效果来源',
  'RetrySafe()',
  'verifyEffectEvidence'
]
export const LEDGER_GUARDS = [
  'bookkeeping := context.WithoutCancel(ctx)',
  'WHERE scope_key=? AND idempotency_key=? AND effect_state=\'pending\''
]

const BINDS_PATTERN = /m04-s04 effect token-binds attempts=(\d+) verified=(\d+) rejected=(\d+) problems=(\d+)/
const MUTATING_PATTERN = /m04-s04 effect token-mutating write=(\d+) external=(\d+) readRejected=(true|false) noneRejected=(true|false)/
const TIMEOUT_PATTERN = /m04-s04 effect timeout-after-write executions=(\d+) replays=(true|false) state=(\w+) result=(\S+)/
const SAME_KEY_PATTERN = /m04-s04 effect same-key executions=(\d+) providers=(\d+) result=(\S+)/
const PAYLOAD_PATTERN = /m04-s04 effect payload-conflict executions=(\d+) conflict=(true|false)/
const MISMATCH_PATTERN = /m04-s04 effect authority-payload-mismatch executions=(\d+) conflict=(true|false)/
const FORGED_PATTERN = /m04-s04 effect authority-token-forged executions=(\d+) rejected=(true|false)/
const UNAVAILABLE_PATTERN = /m04-s04 effect authority-unavailable executions=(\d+) blocked=(true|false) cause=(true|false)/
const ABSENT_PATTERN = /m04-s04 effect authority-absent executions=(\d+) retrySafe=(true|false) state=(\w+)/
const RETRY_SAFE_PATTERN = /m04-s04 effect retry-safe resolutions=(\d+) allowed=(\d+)/
const CANCELLED_PATTERN = /m04-s04 effect cancelled executions=(\d+) recorded=(true|false) err=(true|false)/
const RESTART_PATTERN = /m04-s04 effect restart executions=(\d+) state=(\w+) result=(\S+)/
const CONCURRENT_PATTERN = /m04-s04 effect concurrent workers=(\d+) executions=(\d+) leaders=(\d+)/
const NIL_PATTERN = /m04-s04 effect nil-authority executions=(\d+) rejected=(true|false)/

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

export function parseEffectRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    'token-binds': parseNamed(body, BINDS_PATTERN),
    'token-mutating': parseNamed(body, MUTATING_PATTERN),
    'timeout-after-write': parseNamed(body, TIMEOUT_PATTERN),
    'same-key': parseNamed(body, SAME_KEY_PATTERN),
    'payload-conflict': parseNamed(body, PAYLOAD_PATTERN),
    'authority-payload-mismatch': parseNamed(body, MISMATCH_PATTERN),
    'authority-token-forged': parseNamed(body, FORGED_PATTERN),
    'authority-unavailable': parseNamed(body, UNAVAILABLE_PATTERN),
    'authority-absent': parseNamed(body, ABSENT_PATTERN),
    'retry-safe': parseNamed(body, RETRY_SAFE_PATTERN),
    cancelled: parseNamed(body, CANCELLED_PATTERN),
    restart: parseNamed(body, RESTART_PATTERN),
    concurrent: parseNamed(body, CONCURRENT_PATTERN),
    'nil-authority': parseNamed(body, NIL_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'token-binds':
      return BINDS_PATTERN
    case 'token-mutating':
      return MUTATING_PATTERN
    case 'timeout-after-write':
      return TIMEOUT_PATTERN
    case 'same-key':
      return SAME_KEY_PATTERN
    case 'payload-conflict':
      return PAYLOAD_PATTERN
    case 'authority-payload-mismatch':
      return MISMATCH_PATTERN
    case 'authority-token-forged':
      return FORGED_PATTERN
    case 'authority-unavailable':
      return UNAVAILABLE_PATTERN
    case 'authority-absent':
      return ABSENT_PATTERN
    case 'retry-safe':
      return RETRY_SAFE_PATTERN
    case 'cancelled':
      return CANCELLED_PATTERN
    case 'restart':
      return RESTART_PATTERN
    case 'concurrent':
      return CONCURRENT_PATTERN
    default:
      return NIL_PATTERN
  }
}

function countMatches(text, pattern) {
  return (text.match(new RegExp(pattern.source, 'g')) ?? []).length
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateEffectSource(text) {
  return missingGuards(text, EFFECT_GUARDS, 'effect token source')
}

export function validateLedgerSource(text) {
  return missingGuards(text, LEDGER_GUARDS, 'ledger source')
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
  for (const name of EFFECT_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  expectRow(problems, rows, 'token-binds', [2, 1, 4, 0])
  expectRow(problems, rows, 'token-mutating', [1, 1, 'true', 'true'])
  expectRow(problems, rows, 'timeout-after-write', [1, 'false', 'committed', '{"value":"landed"}'])
  expectRow(problems, rows, 'same-key', [1, 2, '{"value":"once"}'])
  expectRow(problems, rows, 'payload-conflict', [1, 'true'])
  expectRow(problems, rows, 'authority-payload-mismatch', [1, 'true'])
  expectRow(problems, rows, 'authority-token-forged', [1, 'true'])
  expectRow(problems, rows, 'authority-unavailable', [1, 'true', 'true'])
  expectRow(problems, rows, 'authority-absent', [2, 'true', 'committed'])
  expectRow(problems, rows, 'retry-safe', [4, 2])
  expectRow(problems, rows, 'cancelled', [0, 'false', 'true'])
  expectRow(problems, rows, 'restart', [1, 'committed', '{"value":"persisted"}'])
  expectRow(problems, rows, 'concurrent', [8, 1, 1])
  expectRow(problems, rows, 'nil-authority', [0, 'true'])

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
    `invocation=effect-token package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${EFFECT_CASES.length} guards=${EFFECT_GUARDS.length + LEDGER_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `effect token guards frozen=${EFFECT_GUARDS.every((guard) => source.includes(guard))} ledger guards frozen=${LEDGER_GUARDS.every((guard) => source.includes(guard))}`,
    `m04-s04 effect token-binds attempts=${value('token-binds', 0)} verified=${value('token-binds', 1)} rejected=${value('token-binds', 2)} problems=${value('token-binds', 3)}`,
    `m04-s04 effect token-mutating write=${value('token-mutating', 0)} external=${value('token-mutating', 1)} readRejected=${value('token-mutating', 2, 'false')} noneRejected=${value('token-mutating', 3, 'false')}`,
    `m04-s04 effect timeout-after-write executions=${value('timeout-after-write', 0)} replays=${value('timeout-after-write', 1, 'true')} state=${value('timeout-after-write', 2, '')} result=${value('timeout-after-write', 3, '')}`,
    `m04-s04 effect same-key executions=${value('same-key', 0)} providers=${value('same-key', 1)} result=${value('same-key', 2, '')}`,
    `m04-s04 effect payload-conflict executions=${value('payload-conflict', 0)} conflict=${value('payload-conflict', 1, 'false')}`,
    `m04-s04 effect authority-payload-mismatch executions=${value('authority-payload-mismatch', 0)} conflict=${value('authority-payload-mismatch', 1, 'false')}`,
    `m04-s04 effect authority-token-forged executions=${value('authority-token-forged', 0)} rejected=${value('authority-token-forged', 1, 'false')}`,
    `m04-s04 effect authority-unavailable executions=${value('authority-unavailable', 0)} blocked=${value('authority-unavailable', 1, 'false')} cause=${value('authority-unavailable', 2, 'false')}`,
    `m04-s04 effect authority-absent executions=${value('authority-absent', 0)} retrySafe=${value('authority-absent', 1, 'false')} state=${value('authority-absent', 2, '')}`,
    `m04-s04 effect retry-safe resolutions=${value('retry-safe', 0)} allowed=${value('retry-safe', 1)}`,
    `m04-s04 effect cancelled executions=${value('cancelled', 0)} recorded=${value('cancelled', 1, 'true')} err=${value('cancelled', 2, 'false')}`,
    `m04-s04 effect restart executions=${value('restart', 0)} state=${value('restart', 1, '')} result=${value('restart', 2, '')}`,
    `m04-s04 effect concurrent workers=${value('concurrent', 0)} executions=${value('concurrent', 1)} leaders=${value('concurrent', 2)}`,
    `m04-s04 effect nil-authority executions=${value('nil-authority', 0)} rejected=${value('nil-authority', 1, 'false')}`,
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

export function runEffectToken(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal')
  const source = [readFileSync(resolve(base, 'effect_token.go'), 'utf8'), readFileSync(resolve(base, 'ledger.go'), 'utf8')].join('\n')
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
    rows: parseEffectRows(log),
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
      process.stdout.write(`invocation=effect-token package=${PACKAGE_PATH} cases=${EFFECT_CASES.join(',')}\n`)
      return
    }
    const evidence = runEffectToken(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [...validateEffectSource(evidence.source), ...validateLedgerSource(evidence.source), ...validateEvidence(evidence)]
    if (reportProblems(problems)) process.exit(EXIT_EFFECT_TOKEN)
    if (evidence.exitCode !== 0) process.exit(EXIT_EFFECT_TOKEN)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_EFFECT_TOKEN)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
