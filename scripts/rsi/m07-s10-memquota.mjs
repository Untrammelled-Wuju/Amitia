import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestMemoryQuota'
export const BUILD_TAG = 'rsi_memquota'
export const EXIT_MEMQUOTA = 82
export const EXPECTED_PASSED = 13
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const QUOTA_FILE = 'memory_quota.go'
export const QUOTA_TEST_FILE = 'memory_quota_test.go'

export const EVIDENCE_ROWS = [
  'plan bytes=536870912 value=536870912',
  'plan unlimited=true value=max',
  'reject invalid=4',
  'apply verified=true path=memory.max',
  'mismatch rejected=true',
  'write error_propagated=true',
  'read error_propagated=true',
  'cancel rejected=true writes=0',
  'osfs verified=true',
  'nil rejected=true',
  'empty_dir rejected=true',
  'oom kills=7',
  'oom missing=0'
]

export const CASES = [
  'TestMemoryQuotaPlanBounded',
  'TestMemoryQuotaPlanUnlimited',
  'TestMemoryQuotaRejectsInvalid',
  'TestMemoryQuotaApplyVerified',
  'TestMemoryQuotaMismatchRejected',
  'TestMemoryQuotaWriteErrorPropagates',
  'TestMemoryQuotaReadErrorPropagates',
  'TestMemoryQuotaCancelBeforeWrite',
  'TestMemoryQuotaOSFilesystem',
  'TestMemoryQuotaNilFilesystemRejected',
  'TestMemoryQuotaEmptyDirectoryRejected',
  'TestMemoryQuotaOOMKills',
  'TestMemoryQuotaOOMMissing'
]

export const SOURCE_GUARDS = [
  'if bytes < MinMemoryQuotaBytes || bytes > MaxMemoryQuotaBytes {',
  'if bytes == 0 {',
  'if strings.TrimSpace(string(got)) != want {',
  'return MemoryQuotaPlan{}, fmt.Errorf("write memory.max: %w", err)',
  'return MemoryQuotaPlan{}, fmt.Errorf("read memory.max: %w", err)',
  'if fsys == nil {',
  'if strings.TrimSpace(directory) == "" {',
  'fields[0] != "oom_kill"'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s10 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s10 '.length).trim())
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
  if (defaultFiles.includes(QUOTA_FILE)) problems.push(`default build leaked ${QUOTA_FILE}`)
  if (!taggedFiles.includes(QUOTA_FILE)) problems.push(`tagged build missing ${QUOTA_FILE}`)
  if (defaultTests.includes(QUOTA_TEST_FILE)) problems.push(`default tests leaked ${QUOTA_TEST_FILE}`)
  if (!taggedTests.includes(QUOTA_TEST_FILE)) problems.push(`tagged tests missing ${QUOTA_TEST_FILE}`)
  if (taggedFiles.length !== defaultFiles.length + 1) problems.push(`tagged files=${taggedFiles.length} want default+1=${defaultFiles.length + 1}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', QUOTA_FILE)
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `memory quota is missing ${guard}`)
}

export function validateLinuxGate(evidence) {
  const problems = []
  if (evidence?.crossBuildExit !== 0) problems.push(`linux cross build exit=${evidence?.crossBuildExit}`)
  if (evidence?.hostOS === 'linux') problems.push('linux e2e path not implemented; refusing to claim enforcement')
  return problems
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `memory quota test is missing ${name}`)
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
    `invocation=memquota package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=15 guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s10 ${row}`)
  lines.push(`m07-s10 linux_cross_build ok=${evidence?.crossBuildExit === 0} target=linux/arm64`)
  lines.push(`m07-s10 linux_e2e status=BLOCKED reason=host_not_linux host=${evidence?.hostOS ?? 'unknown'}`)
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

export function runMemoryQuotaEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', QUOTA_TEST_FILE), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 1200000
  })
  const cross = spawnSync('go', ['build', '-tags', BUILD_TAG, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: { ...cleanEnv(), GOOS: 'linux', GOARCH: 'arm64', CGO_ENABLED: '0' },
    encoding: 'utf8',
    timeout: 600000
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
    crossBuildExit: cross.status === null ? 1 : cross.status,
    hostOS: process.platform === 'darwin' ? 'darwin' : process.platform,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runMemoryQuotaEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validateLinuxGate(evidence),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_MEMQUOTA)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
