import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestReadFallback'
export const EXIT_READ_FALLBACK = 43
export const EXPECTED_PASSED = 12
export const REQUIRED_SOURCES = ['route_read_fallback.go']
export const REQUIRED_TESTS = ['route_read_fallback_test.go']
export const READ_FALLBACK_CASES = [
  'mutating-blocked',
  'crash-core',
  'permission-denied',
  'permission-undeclared',
  'cross-scope',
  'exhausted',
  'primary-success',
  'injection-contained',
  'cancelled',
  'empty-output',
  'unknown-outcome',
  'open-circuit'
]
export const READ_FALLBACK_GUARDS = [
  'ErrReadFallbackMutating',
  'ErrReadFallbackPermissionDenied',
  'ErrReadFallbackExhausted',
  'errors.Join(ErrReadFallbackPermissionDenied',
  'execution.ErrInvocationOutcomeUnknown',
  'permission denied must never fall back',
  'an unknown outcome must never be replayed',
  'SideEffect.Mutating()',
  'recover()',
  'debug.Stack()'
]

const MUTATING_PATTERN = /m04-s03 read-fallback mutating-blocked sideEffects=(\d+) executed=(\d+)/
const CRASH_PATTERN = /m04-s03 read-fallback crash-core candidates=(\d+) executed=(\d+) selected=(\S+) fallback=(true|false)/
const DENIED_PATTERN = /m04-s03 read-fallback permission-denied candidates=(\d+) executed=(\d+) coreExecuted=(\d+)/
const UNDECLARED_PATTERN = /m04-s03 read-fallback permission-undeclared candidates=(\d+) executed=(\d+)/
const SCOPE_PATTERN = /m04-s03 read-fallback cross-scope candidates=(\d+) executed=(\d+) pluginExecuted=(\d+)/
const EXHAUSTED_PATTERN = /m04-s03 read-fallback exhausted candidates=(\d+) executed=(\d+)/
const PRIMARY_PATTERN = /m04-s03 read-fallback primary-success candidates=(\d+) executed=(\d+) coreExecuted=(\d+)/
const INJECTION_PATTERN = /m04-s03 read-fallback injection-contained pluginCalls=(\d+) coreCalls=(\d+) fallback=(true|false)/
const CANCELLED_PATTERN = /m04-s03 read-fallback cancelled executed=(\d+)/
const EMPTY_PATTERN = /m04-s03 read-fallback empty-output candidates=(\d+) executed=(\d+) selected=(\S+)/
const UNKNOWN_PATTERN = /m04-s03 read-fallback unknown-outcome candidates=(\d+) executed=(\d+) coreExecuted=(\d+)/
const CIRCUIT_PATTERN = /m04-s03 read-fallback open-circuit state=(\S+) pluginExecuted=(\d+) selected=(\S+)/

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

export function parseReadFallbackRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    'mutating-blocked': parseNamed(body, MUTATING_PATTERN),
    'crash-core': parseNamed(body, CRASH_PATTERN),
    'permission-denied': parseNamed(body, DENIED_PATTERN),
    'permission-undeclared': parseNamed(body, UNDECLARED_PATTERN),
    'cross-scope': parseNamed(body, SCOPE_PATTERN),
    exhausted: parseNamed(body, EXHAUSTED_PATTERN),
    'primary-success': parseNamed(body, PRIMARY_PATTERN),
    'injection-contained': parseNamed(body, INJECTION_PATTERN),
    cancelled: parseNamed(body, CANCELLED_PATTERN),
    'empty-output': parseNamed(body, EMPTY_PATTERN),
    'unknown-outcome': parseNamed(body, UNKNOWN_PATTERN),
    'open-circuit': parseNamed(body, CIRCUIT_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'mutating-blocked':
      return MUTATING_PATTERN
    case 'crash-core':
      return CRASH_PATTERN
    case 'permission-denied':
      return DENIED_PATTERN
    case 'permission-undeclared':
      return UNDECLARED_PATTERN
    case 'cross-scope':
      return SCOPE_PATTERN
    case 'exhausted':
      return EXHAUSTED_PATTERN
    case 'primary-success':
      return PRIMARY_PATTERN
    case 'injection-contained':
      return INJECTION_PATTERN
    case 'cancelled':
      return CANCELLED_PATTERN
    case 'empty-output':
      return EMPTY_PATTERN
    case 'unknown-outcome':
      return UNKNOWN_PATTERN
    default:
      return CIRCUIT_PATTERN
  }
}

function countMatches(text, pattern) {
  return (text.match(new RegExp(pattern.source, 'g')) ?? []).length
}

export function validateReadFallbackSource(text) {
  const body = String(text ?? '')
  return READ_FALLBACK_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `read fallback source is missing ${guard}`)
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
  for (const name of READ_FALLBACK_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  expectRow(problems, rows, 'mutating-blocked', [2, 0])
  expectRow(problems, rows, 'crash-core', [2, 2, 'builtin.core/core/memory', 'true'])
  expectRow(problems, rows, 'permission-denied', [2, 1, 0])
  expectRow(problems, rows, 'permission-undeclared', [2, 0])
  expectRow(problems, rows, 'cross-scope', [2, 1, 0])
  expectRow(problems, rows, 'exhausted', [2, 2])
  expectRow(problems, rows, 'primary-success', [2, 1, 0])
  expectRow(problems, rows, 'injection-contained', [1, 1, 'true'])
  expectRow(problems, rows, 'cancelled', [0])
  expectRow(problems, rows, 'empty-output', [2, 2, 'builtin.core/core/memory'])
  expectRow(problems, rows, 'unknown-outcome', [2, 1, 0])
  expectRow(problems, rows, 'open-circuit', ['open', 0, 'builtin.core/core/memory'])

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
    `invocation=read-fallback package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${READ_FALLBACK_CASES.length} guards=${READ_FALLBACK_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((source) => goFiles.includes(source)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `read fallback guards frozen=${READ_FALLBACK_GUARDS.every((guard) => evidence?.source?.includes(guard))}`,
    `m04-s03 read-fallback mutating-blocked sideEffects=${value('mutating-blocked', 0)} executed=${value('mutating-blocked', 1)}`,
    `m04-s03 read-fallback crash-core candidates=${value('crash-core', 0)} executed=${value('crash-core', 1)} selected=${value('crash-core', 2, '')} fallback=${value('crash-core', 3, 'false')}`,
    `m04-s03 read-fallback permission-denied candidates=${value('permission-denied', 0)} executed=${value('permission-denied', 1)} coreExecuted=${value('permission-denied', 2)}`,
    `m04-s03 read-fallback permission-undeclared candidates=${value('permission-undeclared', 0)} executed=${value('permission-undeclared', 1)}`,
    `m04-s03 read-fallback cross-scope candidates=${value('cross-scope', 0)} executed=${value('cross-scope', 1)} pluginExecuted=${value('cross-scope', 2)}`,
    `m04-s03 read-fallback exhausted candidates=${value('exhausted', 0)} executed=${value('exhausted', 1)}`,
    `m04-s03 read-fallback primary-success candidates=${value('primary-success', 0)} executed=${value('primary-success', 1)} coreExecuted=${value('primary-success', 2)}`,
    `m04-s03 read-fallback injection-contained pluginCalls=${value('injection-contained', 0)} coreCalls=${value('injection-contained', 1)} fallback=${value('injection-contained', 2, 'false')}`,
    `m04-s03 read-fallback cancelled executed=${value('cancelled', 0)}`,
    `m04-s03 read-fallback empty-output candidates=${value('empty-output', 0)} executed=${value('empty-output', 1)} selected=${value('empty-output', 2, '')}`,
    `m04-s03 read-fallback unknown-outcome candidates=${value('unknown-outcome', 0)} executed=${value('unknown-outcome', 1)} coreExecuted=${value('unknown-outcome', 2)}`,
    `m04-s03 read-fallback open-circuit state=${value('open-circuit', 0, '')} pluginExecuted=${value('open-circuit', 1)} selected=${value('open-circuit', 2, '')}`,
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

export function runReadFallback(root) {
  const listed = goList(root)
  const sourcePath = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'plugincontract', 'route_read_fallback.go')
  const source = readFileSync(sourcePath, 'utf8')
  const result = spawnSync('go', ['test', '-race', '-count=1', '-timeout', '240s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
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
    rows: parseReadFallbackRows(log),
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
      process.stdout.write(`invocation=read-fallback package=${PACKAGE_PATH} cases=${READ_FALLBACK_CASES.join(',')}\n`)
      return
    }
    const evidence = runReadFallback(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [...validateReadFallbackSource(evidence.source), ...validateEvidence(evidence)]
    if (reportProblems(problems)) process.exit(EXIT_READ_FALLBACK)
    if (evidence.exitCode !== 0) process.exit(EXIT_READ_FALLBACK)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_READ_FALLBACK)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
