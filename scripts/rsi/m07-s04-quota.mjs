import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestQuota'
export const BUILD_TAG = 'rsi_quota'
export const EXIT_QUOTA = 76
export const EXPECTED_PASSED = 15
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const QUOTA_FILES = ['quota_schema.go', 'quota_schema_test.go']

export const EVIDENCE_ROWS = [
  'reject nan=true inf=true',
  'reject negative=5 task_code=400',
  'default dimensions=5',
  'clamp dimensions=5',
  'floor cpu=1',
  'keep unchanged=true',
  'ceiling invalid=true',
  'schema=1 roundtrip=true',
  'support unenforced=2 enforced=processes',
  'ledger reject nonfinite=true used=0',
  'ledger admitted=2 used_mb=200',
  'budget rejected=true dimension=memory',
  'atomic=true dimension=cpu',
  'release zero=true',
  'concurrent attempts=32 admitted=8 used_mb=64'
]

export const CASES = [
  'TestQuotaRejectsNonFinite',
  'TestQuotaRejectsNegative',
  'TestQuotaNormalizesUnset',
  'TestQuotaClampsOverCeiling',
  'TestQuotaFloorsSubPercentCPU',
  'TestQuotaKeepsValuesWithinBounds',
  'TestQuotaRejectsInvalidCeiling',
  'TestQuotaSchemaRoundTrip',
  'TestQuotaSupportZeroesUnenforcedDimensions',
  'TestQuotaLedgerRejectsNonFinite',
  'TestQuotaLedgerAdmitsWithinBudget',
  'TestQuotaLedgerRejectsOverBudget',
  'TestQuotaLedgerIsAllOrNothing',
  'TestQuotaLedgerReleaseIsIdempotent',
  'TestQuotaLedgerConcurrentAdmission'
]

export const SOURCE_GUARDS = [
  'if math.IsNaN(item.value) || math.IsInf(item.value, 0) {',
  'if item.value < 0 {',
  'case value == 0:',
  'case value > ceiling:',
  'return MinQuotaCPUPercent, append(notes, "cpuPercent=floor")',
  'if check.used+check.requested > check.budget+1e-9 {',
  'l.used.MemoryBytes = releaseInt(l.used.MemoryBytes, quota.MemoryBytes)',
  'return &QuotaRejection{Violations: violations}'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s04 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s04 '.length).trim())
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
  for (const file of QUOTA_FILES) {
    const inTests = file.endsWith('_test.go')
    const defaultList = inTests ? defaultTests : defaultFiles
    const taggedList = inTests ? taggedTests : taggedFiles
    if (defaultList.includes(file)) problems.push(`${inTests ? 'default tests leaked' : 'default build leaked'} ${file}`)
    if (!taggedList.includes(file)) problems.push(`tagged build missing ${file}`)
  }
  if (taggedFiles.length !== defaultFiles.length + 1) problems.push(`tagged files=${taggedFiles.length} want default+1=${defaultFiles.length + 1}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', 'quota_schema.go')
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `quota schema is missing ${guard}`)
}

export function validateCeilingSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', 'quota_schema.go')
  const body = readFileSync(path, 'utf8')
  const problems = []
  if (!body.includes('Processes:   int64(taskProcessPIDBudget(runtime.GOOS))')) problems.push('default ceiling must reuse taskProcessPIDBudget')
  if (body.split('ceiling.Validate()').length - 1 !== 1) problems.push('normalization must validate the ceiling exactly once')
  return problems
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `quota test is missing ${name}`)
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
    `invocation=quota package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s04 ${row}`)
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

export function runQuotaEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', 'quota_schema_test.go'), 'utf8')
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
  const evidence = runQuotaEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validateCeilingSource(root),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_QUOTA)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
