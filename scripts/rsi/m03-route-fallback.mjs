import { spawnSync } from 'node:child_process'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestRouteFallback'
export const EXIT_ROUTE_FALLBACK = 39
export const EXPECTED_PASSED = 8
export const REQUIRED_SOURCES = ['route_fallback.go']
export const REQUIRED_TESTS = ['route_fallback_test.go']
export const FALLBACK_CASES = [
  'read-switch',
  'read-empty',
  'write-unavailable',
  'write-failed',
  'write-unknown',
  'read-failed',
  'table',
  'invalid'
]
export const TABLE_ROWS = 8

const READ_SWITCH_PATTERN = /route fallback read unavailable action=(\S+) alternatives=(\d+) switchAllowed=(true|false) retryAllowed=(true|false)/
const READ_EMPTY_PATTERN = /route fallback read unavailable empty action=(\S+) alternatives=(\d+) reason=(.+)/
const WRITE_UNAVAILABLE_PATTERN = /route fallback write unavailable action=(\S+) switchAllowed=(true|false) retryAllowed=(true|false) reason=(.+)/
const WRITE_FAILED_PATTERN = /route fallback write failed action=(\S+) switchAllowed=(true|false) reason=(.+)/
const WRITE_UNKNOWN_PATTERN = /route fallback write unknown action=(\S+) alternatives=(\d+) switchAllowed=(true|false) retryAllowed=(true|false) reason=(.+)/
const READ_FAILED_PATTERN = /route fallback read failed action=(\S+) switchAllowed=(true|false)/
const TABLE_PATTERN = /route fallback table rows=(\d+) writeSwitches=(\d+) unknownWritesSwitch=(\d+)/
const INVALID_PATTERN = /route fallback invalid fields=(\S+) error=(\w+)/

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

export function parseFallbackRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    'read-switch': parseNamed(body, READ_SWITCH_PATTERN),
    'read-empty': parseNamed(body, READ_EMPTY_PATTERN),
    'write-unavailable': parseNamed(body, WRITE_UNAVAILABLE_PATTERN),
    'write-failed': parseNamed(body, WRITE_FAILED_PATTERN),
    'write-unknown': parseNamed(body, WRITE_UNKNOWN_PATTERN),
    'read-failed': parseNamed(body, READ_FAILED_PATTERN),
    table: parseNamed(body, TABLE_PATTERN),
    invalid: parseNamed(body, INVALID_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'read-switch':
      return READ_SWITCH_PATTERN
    case 'read-empty':
      return READ_EMPTY_PATTERN
    case 'write-unavailable':
      return WRITE_UNAVAILABLE_PATTERN
    case 'write-failed':
      return WRITE_FAILED_PATTERN
    case 'write-unknown':
      return WRITE_UNKNOWN_PATTERN
    case 'read-failed':
      return READ_FAILED_PATTERN
    case 'table':
      return TABLE_PATTERN
    default:
      return INVALID_PATTERN
  }
}

function countMatches(text, pattern) {
  return (text.match(new RegExp(pattern.source, 'g')) ?? []).length
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
  for (const name of FALLBACK_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  const readSwitch = rows['read-switch']
  if (Array.isArray(readSwitch)) {
    if (readSwitch[0] !== 'switch_provider') problems.push(`read switch action=${readSwitch[0]} want switch_provider`)
    if (readSwitch[1] !== 1) problems.push(`read switch alternatives=${readSwitch[1]} want 1`)
    if (readSwitch[2] !== 'true') problems.push('read switch must allow the provider switch')
    if (readSwitch[3] !== 'false') problems.push('read switch must not report a re-execution')
  }

  const readEmpty = rows['read-empty']
  if (Array.isArray(readEmpty)) {
    if (readEmpty[0] !== 'stop') problems.push(`read empty action=${readEmpty[0]} want stop`)
    if (readEmpty[1] !== 0) problems.push(`read empty alternatives=${readEmpty[1]} want 0`)
  }

  const writeUnavailable = rows['write-unavailable']
  if (Array.isArray(writeUnavailable)) {
    if (writeUnavailable[0] !== 'stop') problems.push(`write unavailable action=${writeUnavailable[0]} want stop`)
    if (writeUnavailable[1] !== 'false') problems.push('write unavailable must not allow a provider switch')
    if (writeUnavailable[2] !== 'false') problems.push('write unavailable must not allow a re-execution')
  }

  const writeFailed = rows['write-failed']
  if (Array.isArray(writeFailed)) {
    if (writeFailed[0] !== 'stop') problems.push(`write failed action=${writeFailed[0]} want stop`)
    if (writeFailed[1] !== 'false') problems.push('write failed must not allow a provider switch')
  }

  const writeUnknown = rows['write-unknown']
  if (Array.isArray(writeUnknown)) {
    if (writeUnknown[0] !== 'stop') problems.push(`write unknown action=${writeUnknown[0]} want stop`)
    if (writeUnknown[1] !== 3) problems.push(`write unknown alternatives=${writeUnknown[1]} want 3`)
    if (writeUnknown[2] !== 'false') problems.push('an unknown write result must never switch providers')
    if (writeUnknown[3] !== 'false') problems.push('an unknown write result must never re-execute')
  }

  const readFailed = rows['read-failed']
  if (Array.isArray(readFailed)) {
    if (readFailed[0] !== 'stop') problems.push(`read failed action=${readFailed[0]} want stop`)
    if (readFailed[1] !== 'false') problems.push('a failed read must be surfaced, not switched')
  }

  const table = rows.table
  if (Array.isArray(table)) {
    if (table[0] !== TABLE_ROWS) problems.push(`table rows=${table[0]} want ${TABLE_ROWS}`)
    if (table[1] !== 0) problems.push(`table writeSwitches=${table[1]} want 0`)
    if (table[2] !== 0) problems.push(`table unknownWritesSwitch=${table[2]} want 0`)
  }

  const invalid = rows.invalid
  if (Array.isArray(invalid)) {
    if (invalid[0] !== 'operation,outcome,alternatives') problems.push(`invalid fields=${invalid[0]} want operation,outcome,alternatives`)
    if (invalid[1] !== 'ErrRouteFallbackInvalid') problems.push(`invalid error=${invalid[1]} want ErrRouteFallbackInvalid`)
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
    `route=fallback package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${FALLBACK_CASES.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((source) => goFiles.includes(source)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `route fallback read unavailable action=${value('read-switch', 0, '')} alternatives=${value('read-switch', 1)} switchAllowed=${value('read-switch', 2, 'false')} retryAllowed=${value('read-switch', 3, 'false')}`,
    `route fallback read unavailable empty action=${value('read-empty', 0, '')} alternatives=${value('read-empty', 1)} reason=${value('read-empty', 2, '')}`,
    `route fallback write unavailable action=${value('write-unavailable', 0, '')} switchAllowed=${value('write-unavailable', 1, 'false')} retryAllowed=${value('write-unavailable', 2, 'false')} reason=${value('write-unavailable', 3, '')}`,
    `route fallback write failed action=${value('write-failed', 0, '')} switchAllowed=${value('write-failed', 1, 'false')} reason=${value('write-failed', 2, '')}`,
    `route fallback write unknown action=${value('write-unknown', 0, '')} alternatives=${value('write-unknown', 1)} switchAllowed=${value('write-unknown', 2, 'false')} retryAllowed=${value('write-unknown', 3, 'false')} reason=${value('write-unknown', 4, '')}`,
    `route fallback read failed action=${value('read-failed', 0, '')} switchAllowed=${value('read-failed', 1, 'false')}`,
    `route fallback table rows=${value('table', 0)} writeSwitches=${value('table', 1)} unknownWritesSwitch=${value('table', 2)}`,
    `route fallback invalid fields=${value('invalid', 0, '')} error=${value('invalid', 1, '')}`,
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`
  ].join('\n') + '\n'
}

function goList(root) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 120000
  })
  const line = `${result.stdout ?? ''}`.trim()
  const [files = '', tests = ''] = line.split('|')
  return { files: parseGoListFiles(files), tests: parseGoListFiles(tests) }
}

export function runRouteFallback(root) {
  const listed = goList(root)
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
    rows: parseFallbackRows(log),
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
      process.stdout.write(`route=fallback package=${PACKAGE_PATH} cases=${FALLBACK_CASES.join(',')}\n`)
      return
    }
    const evidence = runRouteFallback(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_ROUTE_FALLBACK)
    if (evidence.exitCode !== 0) process.exit(EXIT_ROUTE_FALLBACK)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_ROUTE_FALLBACK)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
