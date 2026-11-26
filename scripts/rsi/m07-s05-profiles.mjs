import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestQuotaProfile'
export const BUILD_TAGS = 'rsi_quota rsi_profiles'
export const EXIT_PROFILES = 77
export const EXPECTED_PASSED = 10
export const GO_TEST_FLAGS = ['-tags', BUILD_TAGS, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const PROFILE_FILES = ['quota_profiles.go', 'quota_profiles_test.go']

export const EVIDENCE_ROWS = [
  'mobile memory_mb=128 concurrency=1',
  'desktop memory_mb=512 concurrency=4',
  'server memory_mb=2048 concurrency=16',
  'resolve mobile=mobile-lite',
  'deny mobile_server=true',
  'unauthorized=true granted server=true',
  'unknown device=true profile=true',
  'configure unauthorized=true unchanged=true',
  'configured quota_mb=256 concurrency=2',
  'oversize rejected=true',
  'validate quota_exceeds=true',
  'mobile_ceiling concurrency<=2'
]

export const CASES = [
  'TestQuotaProfileDefaultsPerDevice',
  'TestQuotaProfileResolveDefaults',
  'TestQuotaProfileMobileDeniesServer',
  'TestQuotaProfileRequiresGrantForNonDefault',
  'TestQuotaProfileRejectsUnknownDeviceAndProfile',
  'TestQuotaProfileConfigureRequiresGrant',
  'TestQuotaProfileConfigureClampsQuota',
  'TestQuotaProfileConfigureRejectsAboveAbsolute',
  'TestQuotaProfileValidateRejectsQuotaAboveCeiling',
  'TestQuotaProfileMobileCeilingBounds'
]

export const SOURCE_GUARDS = [
  'if device == DeviceMobile && target == ProfileServer {',
  'if target != device.defaultProfile() {',
  'if err := grant.authorize(); err != nil {',
  'if configured.MemoryBytes > absolute.MemoryBytes',
  'normalized, _, err := NormalizeQuota(spec.Quota, configured)',
  'if s.Quota.MemoryBytes > s.Ceiling.MemoryBytes',
  'ceiling := DefaultQuotaCeiling()',
  'Concurrency: 1,'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s05 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s05 '.length).trim())
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
  if (defaultFiles.includes(PROFILE_FILES[0])) problems.push(`default build leaked ${PROFILE_FILES[0]}`)
  if (!taggedFiles.includes(PROFILE_FILES[0])) problems.push(`tagged build missing ${PROFILE_FILES[0]}`)
  if (defaultTests.includes(PROFILE_FILES[1])) problems.push(`default tests leaked ${PROFILE_FILES[1]}`)
  if (!taggedTests.includes(PROFILE_FILES[1])) problems.push(`tagged tests missing ${PROFILE_FILES[1]}`)
  if (taggedFiles.length !== defaultFiles.length + 2) problems.push(`tagged files=${taggedFiles.length} want default+2=${defaultFiles.length + 2}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', 'quota_profiles.go')
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `quota profiles is missing ${guard}`)
}

export function validateAbsoluteBound(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', 'quota_profiles.go')
  const body = readFileSync(path, 'utf8')
  const problems = []
  if (!body.includes('MemoryBytes: 8 << 30,')) problems.push('absolute maximum memory must be 8 GiB')
  if (!body.includes('Concurrency: 64,')) problems.push('absolute maximum concurrency must be 64')
  if (body.split('AbsoluteQuotaMax()').length - 1 !== 3) problems.push('absolute maximum must be consulted exactly three times')
  return problems
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `quota profile test is missing ${name}`)
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
    `invocation=profiles package=${PACKAGE_PATH} run=${RUN_PATTERN} tags="${BUILD_TAGS}" rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s05 ${row}`)
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

export function runProfileEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAGS])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', 'quota_profiles_test.go'), 'utf8')
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
  const evidence = runProfileEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validateAbsoluteBound(root),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_PROFILES)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
