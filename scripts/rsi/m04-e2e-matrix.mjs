import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/devicemesh/executionjournal'
export const RUN_PATTERN = 'TestE2E'
export const EXIT_E2E_MATRIX = 48
export const EXPECTED_PASSED = 6
export const EXPECTED_SKIPPED = 1
export const REQUIRED_SOURCES = ['ledger.go', 'effect_token.go', 'reconciliation.go', 'run_guard.go']
export const REQUIRED_TESTS = ['e2e_sink_test.go']
export const E2E_CASES = ['no-duplicate', 'concurrent', 'irreversible', 'default-core', 'restart', 'cancelled']
export const SINK_GUARDS = [
  'AMITIA_RSI_SINK_MODE',
  'net.Listen("unix", socket)',
  'os.O_APPEND|os.O_CREATE|os.O_WRONLY',
  'file.Sync()',
  'exec.Command(os.Args[0], "-test.run=TestE2ESinkHelperProcess")',
  'errSinkAckLost'
]
export const ASSERT_GUARDS = [
  'if writes != operations || unique != operations || duplicates != 0 {',
  'if executed != 1 || writes != 1 || unique != 1 {',
  'if executed || writes != 0 || unique != 0 {'
]
export const INTEGRATION_PATH = 'artifacts/rsi/m04-s08/INTEGRATION.json'
export const INTEGRATION_FILES = [
  'backend/internal/devicemesh/executionjournal/ledger.go',
  'backend/internal/devicemesh/executionjournal/ledger_test.go',
  'backend/internal/devicemesh/executionjournal/effect_token.go',
  'backend/internal/devicemesh/executionjournal/effect_token_test.go',
  'backend/internal/devicemesh/executionjournal/reconciliation.go',
  'backend/internal/devicemesh/executionjournal/reconciliation_test.go',
  'backend/internal/devicemesh/executionjournal/run_guard.go',
  'backend/internal/devicemesh/executionjournal/fault_matrix_test.go',
  'backend/internal/devicemesh/executionjournal/e2e_sink_test.go',
  'scripts/rsi/m04-ledger.mjs',
  'scripts/rsi/m04-effect-token.mjs',
  'scripts/rsi/m04-reconciliation.mjs',
  'scripts/rsi/m04-fault-matrix.mjs',
  'scripts/rsi/m04-e2e-matrix.mjs'
]
export const INTEGRATION_GATES = [
  { id: 'LEDGER', script: 'scripts/rsi/m04-ledger.mjs', exit: 42 },
  { id: 'EFFECT_TOKEN', script: 'scripts/rsi/m04-effect-token.mjs', exit: 44 },
  { id: 'RECONCILIATION', script: 'scripts/rsi/m04-reconciliation.mjs', exit: 45 },
  { id: 'FAULT_MATRIX', script: 'scripts/rsi/m04-fault-matrix.mjs', exit: 47 },
  { id: 'E2E_MATRIX', script: 'scripts/rsi/m04-e2e-matrix.mjs', exit: 48 }
]

const NO_DUP_PATTERN = /m04-s08 e2e no-duplicate ops=(\d+) executed=(\d+) blocked=(\d+) sink_writes=(\d+) unique=(\d+) duplicates=(\d+)/
const CONCURRENT_PATTERN = /m04-s08 e2e concurrent workers=(\d+) executed=(\d+) sink_writes=(\d+) unique=(\d+)/
const IRREVERSIBLE_PATTERN = /m04-s08 e2e irreversible state=(\w+) sink_writes=(\d+) unique=(\d+) blocked=(true|false)/
const CORE_PATTERN = /m04-s08 e2e default-core provider=(\S+) catalog=(\d+) permissions=(\S+) available=(true|false)/
const RESTART_PATTERN = /m04-s08 e2e restart ops=(\d+) blocked=(\d+) sink_writes=(\d+) unique=(\d+)/
const CANCELLED_PATTERN = /m04-s08 e2e cancelled ran=(true|false) rows=(\d+) sink_writes=(\d+)/

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length,
    skipped: (body.match(/--- SKIP: /g) ?? []).length
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
  return match.slice(1).map((value) => {
    if (/^\d+$/.test(value)) return Number(value)
    return value
  })
}

export function parseE2ERows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    'no-duplicate': parseNamed(body, NO_DUP_PATTERN),
    concurrent: parseNamed(body, CONCURRENT_PATTERN),
    irreversible: parseNamed(body, IRREVERSIBLE_PATTERN),
    'default-core': parseNamed(body, CORE_PATTERN),
    restart: parseNamed(body, RESTART_PATTERN),
    cancelled: parseNamed(body, CANCELLED_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'no-duplicate':
      return NO_DUP_PATTERN
    case 'concurrent':
      return CONCURRENT_PATTERN
    case 'irreversible':
      return IRREVERSIBLE_PATTERN
    case 'default-core':
      return CORE_PATTERN
    case 'restart':
      return RESTART_PATTERN
    default:
      return CANCELLED_PATTERN
  }
}

export function countCaseLines(log, name) {
  const body = typeof log === 'string' ? log : ''
  const pattern = new RegExp(`m04-s08 e2e ${name.replace(/[-/\\^$*+?.()|[\]{}]/g, '\\$&')} `, 'g')
  return (body.match(pattern) ?? []).length
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateSinkSource(text) {
  return missingGuards(text, SINK_GUARDS, 'sink e2e source')
}

export function validateAssertions(text) {
  return missingGuards(text, ASSERT_GUARDS, 'sink e2e source')
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
  for (const name of E2E_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countCaseLines(log, name) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  expectRow(problems, rows, 'no-duplicate', [100, 100, 100, 100, 100, 0])
  expectRow(problems, rows, 'concurrent', [32, 1, 1, 1])
  expectRow(problems, rows, 'irreversible', ['needs_reconciliation', 1, 1, 'true'])
  expectRow(problems, rows, 'default-core', ['builtin.core/core/memory.search', 1, 'memory.read/low', 'true'])
  expectRow(problems, rows, 'restart', [20, 20, 20, 20])
  expectRow(problems, rows, 'cancelled', ['false', 0, 0])

  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.skipped !== EXPECTED_SKIPPED) problems.push(`skipped=${evidence?.skipped} want ${EXPECTED_SKIPPED}`)
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
    `invocation=e2e-matrix package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${E2E_CASES.length} guards=${SINK_GUARDS.length + ASSERT_GUARDS.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `sink guards frozen=${SINK_GUARDS.every((guard) => String(evidence?.sinkSource ?? '').includes(guard))} assertions frozen=${ASSERT_GUARDS.every((guard) => String(evidence?.sinkSource ?? '').includes(guard))}`,
    `m04-s08 e2e no-duplicate ops=${value('no-duplicate', 0)} executed=${value('no-duplicate', 1)} blocked=${value('no-duplicate', 2)} sink_writes=${value('no-duplicate', 3)} unique=${value('no-duplicate', 4)} duplicates=${value('no-duplicate', 5)}`,
    `m04-s08 e2e concurrent workers=${value('concurrent', 0)} executed=${value('concurrent', 1)} sink_writes=${value('concurrent', 2)} unique=${value('concurrent', 3)}`,
    `m04-s08 e2e irreversible state=${value('irreversible', 0, '')} sink_writes=${value('irreversible', 1)} unique=${value('irreversible', 2)} blocked=${value('irreversible', 3, 'false')}`,
    `m04-s08 e2e default-core provider=${value('default-core', 0, '')} catalog=${value('default-core', 1)} permissions=${value('default-core', 2, '')} available=${value('default-core', 3, 'false')}`,
    `m04-s08 e2e restart ops=${value('restart', 0)} blocked=${value('restart', 1)} sink_writes=${value('restart', 2)} unique=${value('restart', 3)}`,
    `m04-s08 e2e cancelled ran=${value('cancelled', 0, 'false')} rows=${value('cancelled', 1)} sink_writes=${value('cancelled', 2)}`,
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} skipped=${evidence?.skipped ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`
  ].join('\n') + '\n'
}

export function digestFiles(root, files = INTEGRATION_FILES) {
  return files.map((path) => {
    const absolute = resolve(root, path)
    if (!existsSync(absolute)) return { path, bytes: 0, sha256: '', missing: true }
    const raw = readFileSync(absolute)
    return {
      path,
      bytes: raw.length,
      sha256: createHash('sha256').update(raw).digest('hex')
    }
  })
}

export function buildIntegration(evidence, root) {
  return {
    schemaVersion: 1,
    step: 'M04-S08',
    package: PACKAGE_PATH,
    runPattern: RUN_PATTERN,
    files: digestFiles(root),
    gates: INTEGRATION_GATES,
    cases: E2E_CASES,
    evidence: {
      passed: evidence.passed,
      failed: evidence.failed,
      skipped: evidence.skipped,
      exitCode: evidence.exitCode,
      race: evidence.race,
      rows: evidence.rows
    },
    verdict: 'READY_FOR_APPROVAL'
  }
}

export function validateIntegration(manifest, root) {
  const problems = []
  if (manifest?.schemaVersion !== 1) problems.push(`schemaVersion=${manifest?.schemaVersion} want 1`)
  if (manifest?.step !== 'M04-S08') problems.push(`step=${manifest?.step} want M04-S08`)
  if (manifest?.verdict !== 'READY_FOR_APPROVAL') problems.push(`verdict=${manifest?.verdict} want READY_FOR_APPROVAL`)
  const recorded = Array.isArray(manifest?.files) ? manifest.files : []
  if (recorded.length !== INTEGRATION_FILES.length) problems.push(`files=${recorded.length} want ${INTEGRATION_FILES.length}`)
  const current = digestFiles(root)
  const byPath = new Map(current.map((entry) => [entry.path, entry]))
  for (const entry of recorded) {
    const live = byPath.get(entry.path)
    if (!live) {
      problems.push(`integration records unknown file ${entry.path}`)
      continue
    }
    if (live.missing) problems.push(`integrated file is missing ${entry.path}`)
    if (entry.sha256 !== live.sha256) problems.push(`integration sha256 drift on ${entry.path}`)
    if (entry.bytes !== live.bytes) problems.push(`integration size drift on ${entry.path}`)
  }
  const gateIds = (Array.isArray(manifest?.gates) ? manifest.gates : []).map((gate) => gate.id)
  for (const gate of INTEGRATION_GATES) {
    if (!gateIds.includes(gate.id)) problems.push(`integration is missing gate ${gate.id}`)
  }
  const cases = Array.isArray(manifest?.cases) ? manifest.cases : []
  for (const name of E2E_CASES) {
    if (!cases.includes(name)) problems.push(`integration is missing case ${name}`)
  }
  return problems
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

export function runE2EEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'devicemesh', 'executionjournal')
  const sinkSource = readFileSync(resolve(base, REQUIRED_TESTS[0]), 'utf8')
  const result = spawnSync('go', ['test', '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    goFiles: listed.files,
    testFiles: listed.tests,
    rows: parseE2ERows(log),
    sinkSource,
    passed: counts.passed,
    failed: counts.failed,
    skipped: counts.skipped,
    exitCode: result.status === null ? 1 : result.status,
    race: true,
    log
  }
}

export function runInventoryGate(root) {
  const result = spawnSync(process.execPath, ['scripts/rsi/m03-core-inventory.mjs'], {
    cwd: root,
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  return {
    exitCode: result.status === null ? 1 : result.status,
    tail: `${result.stdout ?? ''}${result.stderr ?? ''}`.trim().split('\n').slice(-3).join(' | ')
  }
}

function reportProblems(problems) {
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  return problems.length > 0
}

function main() {
  try {
    if (process.argv.includes('--list')) {
      process.stdout.write(`invocation=e2e-matrix package=${PACKAGE_PATH} cases=${E2E_CASES.join(',')}\n`)
      process.stdout.write(`integration=${INTEGRATION_PATH} files=${INTEGRATION_FILES.length} gates=${INTEGRATION_GATES.length}\n`)
      return
    }
    const target = resolve(repoRoot, INTEGRATION_PATH)
    if (process.argv.includes('--verify')) {
      const manifest = JSON.parse(readFileSync(target, 'utf8'))
      const problems = validateIntegration(manifest, repoRoot)
      process.stdout.write(`verify=${problems.length === 0} integration=${target} files=${manifest.files?.length ?? 0}\n`)
      if (reportProblems(problems)) process.exit(EXIT_E2E_MATRIX)
      return
    }
    const inventory = runInventoryGate(repoRoot)
    const evidence = runE2EEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    process.stdout.write(`production-surface routes=${inventory.exitCode === 0 ? 'unchanged' : 'drift'} exit=${inventory.exitCode} ${inventory.tail}\n`)
    const problems = [
      ...validateSinkSource(evidence.sinkSource),
      ...validateAssertions(evidence.sinkSource),
      ...validateEvidence(evidence)
    ]
    if (inventory.exitCode !== 0) problems.push(`route inventory gate exit=${inventory.exitCode} want 0`)
    if (reportProblems(problems)) process.exit(EXIT_E2E_MATRIX)
    if (evidence.exitCode !== 0) process.exit(EXIT_E2E_MATRIX)
    const manifest = buildIntegration(evidence, repoRoot)
    const integrationProblems = validateIntegration(manifest, repoRoot)
    if (reportProblems(integrationProblems)) process.exit(EXIT_E2E_MATRIX)
    mkdirSync(dirname(target), { recursive: true })
    writeFileSync(target, `${JSON.stringify(manifest, null, 2)}\n`)
    process.stdout.write(`integration=${target} files=${manifest.files.length} gates=${manifest.gates.length} verdict=${manifest.verdict}\n`)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_E2E_MATRIX)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
