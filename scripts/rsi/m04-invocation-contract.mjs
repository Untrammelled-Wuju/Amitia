import { spawnSync } from 'node:child_process'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/execution'
export const RUN_PATTERN = 'TestInvocation'
export const EXIT_INVOCATION_CONTRACT = 41
export const EXPECTED_PASSED = 8
export const REQUIRED_SOURCES = ['invocation.go']
export const REQUIRED_TESTS = ['invocation_test.go', 'invocation_chain_test.go']
export const INVOCATION_CASES = [
  'trace-guard',
  'trace-child',
  'contract-key',
  'outcome-success',
  'outcome-write-unknown',
  'outcome-read-failed',
  'journal',
  'chain'
]
export const SIDE_EFFECT_CLASSES = ['none', 'read', 'write', 'external']
export const HOST_HOP = 'host_api'
export const PROVIDER_HOP = 'capability'

const TRACE_GUARD_PATTERN = /invocation trace guards fields=(\S+) error=(\w+)/
const TRACE_CHILD_PATTERN = /invocation trace child root=(\S+) trace=(\S+) parent=(\S+) child=(\S+)/
const CONTRACT_KEY_PATTERN = /invocation contract mutating=(\S+) requiresKey=(true|false) readMutating=(true|false)/
const OUTCOME_SUCCESS_PATTERN = /invocation outcome success=(\w+) preDispatch=(\S+) result=(\w+) unknown=(true|false)/
const OUTCOME_WRITE_PATTERN = /invocation outcome write postDispatch=(\w+) outcome=(\w+) retrySafe=(true|false) cause=(true|false)/
const OUTCOME_READ_PATTERN = /invocation outcome read postDispatch=(\w+) outcome=(\w+) ambiguous=(true|false)/
const JOURNAL_PATTERN = /invocation journal root=(\S+) trace=(\S+) invocation=(\S+) entries=(\d+)/
const CHAIN_PATTERN = /invocation chain root=(\S+) trace=(\S+) invocation=(\S+) parent=(\S+) child=(\S+) hops=(\S+) entries=(\d+) sideEffect=(\w+) outcome=(\w+)/

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

export function parseInvocationRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    'trace-guard': parseNamed(body, TRACE_GUARD_PATTERN),
    'trace-child': parseNamed(body, TRACE_CHILD_PATTERN),
    'contract-key': parseNamed(body, CONTRACT_KEY_PATTERN),
    'outcome-success': parseNamed(body, OUTCOME_SUCCESS_PATTERN),
    'outcome-write-unknown': parseNamed(body, OUTCOME_WRITE_PATTERN),
    'outcome-read-failed': parseNamed(body, OUTCOME_READ_PATTERN),
    journal: parseNamed(body, JOURNAL_PATTERN),
    chain: parseNamed(body, CHAIN_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'trace-guard':
      return TRACE_GUARD_PATTERN
    case 'trace-child':
      return TRACE_CHILD_PATTERN
    case 'contract-key':
      return CONTRACT_KEY_PATTERN
    case 'outcome-success':
      return OUTCOME_SUCCESS_PATTERN
    case 'outcome-write-unknown':
      return OUTCOME_WRITE_PATTERN
    case 'outcome-read-failed':
      return OUTCOME_READ_PATTERN
    case 'journal':
      return JOURNAL_PATTERN
    default:
      return CHAIN_PATTERN
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
  for (const name of INVOCATION_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  const guard = rows['trace-guard']
  if (Array.isArray(guard)) {
    if (guard[0] !== 'root,trace,invocation') problems.push(`trace guard fields=${guard[0]} want root,trace,invocation`)
    if (guard[1] !== 'ErrInvocationContractInvalid') problems.push(`trace guard error=${guard[1]} want ErrInvocationContractInvalid`)
  }

  const child = rows['trace-child']
  if (Array.isArray(child)) {
    if (child[0] !== 'root-1' || child[1] !== 'trace-1') problems.push(`child lost root or trace root=${child[0]} trace=${child[1]}`)
    if (child[2] !== 'inv-1') problems.push(`child parent=${child[2]} want inv-1`)
    if (child[3] !== 'inv-2') problems.push(`child id=${child[3]} want inv-2`)
  }

  const contract = rows['contract-key']
  if (Array.isArray(contract)) {
    if (contract[0] !== 'write,external') problems.push(`mutating classes=${contract[0]} want write,external`)
    if (contract[1] !== 'true') problems.push('a mutating invocation without an idempotency key must be rejected')
    if (contract[2] !== 'false') problems.push('a read invocation must not be mutating')
  }

  const success = rows['outcome-success']
  if (Array.isArray(success)) {
    if (success[0] !== 'success') problems.push(`success outcome=${success[0]} want success`)
    if (success[2] !== 'failed') problems.push(`pre-dispatch timeout outcome=${success[2]} want failed`)
    if (success[3] !== 'false') problems.push('a pre-dispatch timeout must never be unknown')
  }

  const write = rows['outcome-write-unknown']
  if (Array.isArray(write)) {
    if (write[0] !== 'timeout') problems.push(`write postDispatch=${write[0]} want timeout`)
    if (write[1] !== 'unknown') problems.push(`dispatched mutating timeout outcome=${write[1]} want unknown`)
    if (write[2] !== 'false') problems.push('an unknown mutating outcome must not be retry safe')
    if (write[3] !== 'true') problems.push('an unknown mutating outcome must preserve the original cause')
  }

  const read = rows['outcome-read-failed']
  if (Array.isArray(read)) {
    if (read[1] !== 'failed') problems.push(`dispatched read timeout outcome=${read[1]} want failed`)
    if (read[2] !== 'true') problems.push('a timeout must be recognised as an ambiguous failure')
  }

  const journal = rows.journal
  if (Array.isArray(journal)) {
    if (journal[0] !== 'root-1' || journal[1] !== 'trace-1' || journal[2] !== 'inv-1') {
      problems.push(`journal trace drifted root=${journal[0]} trace=${journal[1]} invocation=${journal[2]}`)
    }
    if (journal[3] !== 2) problems.push(`journal entries=${journal[3]} want 2`)
  }

  const chain = rows.chain
  if (Array.isArray(chain)) {
    if (chain[0] !== 'root-1' || chain[1] !== 'trace-1' || chain[2] !== 'inv-1') {
      problems.push(`chain trace drifted root=${chain[0]} trace=${chain[1]} invocation=${chain[2]}`)
    }
    if (chain[3] !== 'inv-0') problems.push(`chain parent=${chain[3]} want inv-0`)
    if (chain[4] !== 'inv-2') problems.push(`chain child=${chain[4]} want inv-2`)
    if (chain[5] !== `${HOST_HOP}>${PROVIDER_HOP}`) problems.push(`chain hops=${chain[5]} want ${HOST_HOP}>${PROVIDER_HOP}`)
    if (chain[6] !== 3) problems.push(`chain entries=${chain[6]} want 3`)
    if (!SIDE_EFFECT_CLASSES.includes(chain[7])) problems.push(`chain sideEffect=${chain[7]} is not a frozen class`)
    if (chain[7] !== 'write') problems.push(`chain sideEffect=${chain[7]} want write`)
    if (chain[8] !== 'unknown') problems.push(`chain outcome=${chain[8]} want unknown`)
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
    `invocation=contract package=${PACKAGE_PATH} run=${RUN_PATTERN} classes=${SIDE_EFFECT_CLASSES.join(',')} cases=${INVOCATION_CASES.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((source) => goFiles.includes(source)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `invocation trace guards fields=${value('trace-guard', 0, '')} error=${value('trace-guard', 1, '')}`,
    `invocation trace child root=${value('trace-child', 0, '')} trace=${value('trace-child', 1, '')} parent=${value('trace-child', 2, '')} child=${value('trace-child', 3, '')}`,
    `invocation contract mutating=${value('contract-key', 0, '')} requiresKey=${value('contract-key', 1, 'false')} readMutating=${value('contract-key', 2, 'true')}`,
    `invocation outcome success=${value('outcome-success', 0, '')} preDispatch=${value('outcome-success', 1, '')} result=${value('outcome-success', 2, '')} unknown=${value('outcome-success', 3, 'true')}`,
    `invocation outcome write postDispatch=${value('outcome-write-unknown', 0, '')} outcome=${value('outcome-write-unknown', 1, '')} retrySafe=${value('outcome-write-unknown', 2, 'true')} cause=${value('outcome-write-unknown', 3, 'false')}`,
    `invocation outcome read postDispatch=${value('outcome-read-failed', 0, '')} outcome=${value('outcome-read-failed', 1, '')} ambiguous=${value('outcome-read-failed', 2, 'false')}`,
    `invocation journal root=${value('journal', 0, '')} trace=${value('journal', 1, '')} invocation=${value('journal', 2, '')} entries=${value('journal', 3)}`,
    `invocation chain root=${value('chain', 0, '')} trace=${value('chain', 1, '')} invocation=${value('chain', 2, '')} parent=${value('chain', 3, '')} child=${value('chain', 4, '')} hops=${value('chain', 5, '')} entries=${value('chain', 6)} sideEffect=${value('chain', 7, '')} outcome=${value('chain', 8, '')}`,
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

export function runInvocationContract(root) {
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
    rows: parseInvocationRows(log),
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
      process.stdout.write(`invocation=contract package=${PACKAGE_PATH} cases=${INVOCATION_CASES.join(',')}\n`)
      return
    }
    const evidence = runInvocationContract(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_INVOCATION_CONTRACT)
    if (evidence.exitCode !== 0) process.exit(EXIT_INVOCATION_CONTRACT)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_INVOCATION_CONTRACT)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
