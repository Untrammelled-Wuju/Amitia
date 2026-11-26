import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestPlatformQuota'
export const BUILD_TAG = 'rsi_platformquota'
export const EXIT_PLATFORMQUOTA = 84
export const EXPECTED_PASSED = 12
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const QUOTA_FILE = 'platform_quota.go'
export const QUOTA_TEST_FILE = 'platform_quota_test.go'

export const EVIDENCE_ROWS = [
  'android level=full',
  'android level=limited unsupported=cpuPercent',
  'android level=remote delegated=true',
  'ios level=remote unsupported=3',
  'ios conditions=network-required,owner-consent',
  'desktop level=full',
  'platform rejected=true',
  'level rejected=true',
  'zero_request level=full',
  'background unsupported=true',
  'remote_requested delegated=true',
  'request rejected=true'
]

export const CASES = [
  'TestPlatformQuotaAndroidFull',
  'TestPlatformQuotaAndroidLimitedCPU',
  'TestPlatformQuotaAndroidRemote',
  'TestPlatformQuotaIOSAlwaysRemote',
  'TestPlatformQuotaIOSRemoteRequest',
  'TestPlatformQuotaDesktopFull',
  'TestPlatformQuotaInvalidPlatform',
  'TestPlatformQuotaInvalidLevel',
  'TestPlatformQuotaZeroRequestFull',
  'TestPlatformQuotaBackgroundOnlyMissing',
  'TestPlatformQuotaRemoteRequestOnDesktop',
  'TestPlatformQuotaInvalidRequest'
]

export const SOURCE_GUARDS = [
  'if platform == PlatformIOS {',
  'if !platform.valid() {',
  'if !requested.valid() {',
  'if request.MemoryBytes < 0 || request.CPUPercent > 100 || request.ActiveProcesses > MaxPlatformQuotaProcesses {',
  'case len(unsupported) == 0:',
  'decision.Level = QuotaLevelLimited',
  'decision.Level = QuotaLevelRemote',
  'sort.Strings(unsupported)'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s12 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s12 '.length).trim())
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
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `platform quota is missing ${guard}`)
}

export function validateMobileGate(evidence) {
  const problems = []
  if (evidence?.mobileDevice === true) problems.push('mobile e2e path not implemented; refusing to claim device evidence')
  return problems
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `platform quota test is missing ${name}`)
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
    `invocation=platformquota package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=14 guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s12 ${row}`)
  lines.push('m07-s12 platforms declared=3 android,ios,desktop')
  lines.push('m07-s12 mobile_e2e status=BLOCKED reason=no_device_in_environment')
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

export function runPlatformQuotaEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', QUOTA_TEST_FILE), 'utf8')
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
    mobileDevice: false,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runPlatformQuotaEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validateMobileGate(evidence),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_PLATFORMQUOTA)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
