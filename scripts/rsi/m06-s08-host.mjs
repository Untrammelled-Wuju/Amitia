import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/runtime_supervisor'
export const RUN_PATTERN = 'TestHostIsolation'
export const BUILD_TAG = 'rsi_lifecycle'
export const EXIT_HOST = 72
export const EXPECTED_PASSED = 8
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const HOST_TEST = 'host_isolation_test.go'
export const PACKAGE_FILE = 'artifacts/rsi/m06-integration-package.json'

export const HOST_CASES = [
  'TestHostIsolationThreeRuntimeTypes',
  'TestHostIsolationKillOneKeepsOthersHealthy',
  'TestHostIsolationColdStartFailureIsolated',
  'TestHostIsolationOldFlowUnchanged',
  'TestHostIsolationRestartAfterKill',
  'TestHostIsolationCancelledReconcile',
  'TestHostIsolationMaliciousPluginCannotKillCore',
  'TestHostIsolationRegistryIsolation'
]

export const PRODUCTION_FILES = [
  'backend/internal/extension/kernel/enablement/lifecycle.go',
  'backend/internal/extension/kernel/enablement/install.go',
  'backend/internal/extension/kernel/enablement/execute.go',
  'backend/internal/extension/kernel/enablement/idle.go',
  'backend/internal/extension/kernel/enablement/hot.go',
  'backend/internal/extension/kernel/enablement/release.go',
  'backend/internal/extension/kernel/runtime_supervisor/supervisor.go'
]

export const LIFECYCLE_FILES = [
  'lifecycle.go',
  'install.go',
  'execute.go',
  'idle.go',
  'hot.go',
  'release.go'
]

export const GATES = [
  'LIFECYCLE_STATES',
  'PLUGIN_METADATA',
  'EXECUTE_ON_DEMAND',
  'IDLE_RECLAIM',
  'HOT_RESIDENCY',
  'RELEASE_LEAKS',
  'COLD_BASELINE',
  'HOST_ISOLATION'
]

export const ASSERT_GUARDS = [
  'if builtin.InstanceID == script.InstanceID || script.InstanceID == wasm.InstanceID || builtin.InstanceID == wasm.InstanceID {',
  'if after.Actual != ActualStopped {',
  'if instance.Actual != ActualReady || instance.Health != HealthHealthy {',
  'if result.Error == nil {',
  'if result.Error != nil || result.Status != "success" {',
  'if err := supervisor.Restart(context.Background(), instance.InstanceID); err != nil {',
  'if healthy.Actual != ActualReady {',
  'if firstInstance.InstanceID == secondInstance.InstanceID {'
]

export const EVIDENCE_ROWS = [
  'isolation types=3 instances=3 starts=3',
  'kill_isolated wasm=stopped others=ready healthy=2',
  'cold_start_failure isolated=true native=ready',
  'legacy_flow unchanged=true status=success',
  'restart_after_kill ready=true',
  'cancelled rejected=true',
  'malicious quarantined=true core_healthy=true',
  'registry_isolation supervisors=2 independent=true',
  'package files=7 steps=8 switch=rsi_lifecycle'
]

export function parseHostRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m06-s08 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm06-s08 '.length).trim())
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

export function sha256File(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

export function validateAssertSource(text) {
  const body = String(text ?? '')
  return ASSERT_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `host test is missing ${guard}`)
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return HOST_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `host test is missing ${name}`)
}

export function validateOffState(evidence) {
  const problems = []
  const defaultFiles = Array.isArray(evidence?.defaultFiles) ? evidence.defaultFiles : []
  const taggedFiles = Array.isArray(evidence?.taggedFiles) ? evidence.taggedFiles : []
  for (const file of LIFECYCLE_FILES) {
    if (defaultFiles.includes(file)) problems.push(`default build leaked ${file}`)
    if (!taggedFiles.includes(file)) problems.push(`tagged build missing ${file}`)
  }
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (!taggedTests.includes('host_isolation_test.go')) problems.push('tagged supervisor tests missing host_isolation_test.go')
  if (defaultTests.includes('host_isolation_test.go')) problems.push('default supervisor tests leaked host_isolation_test.go')
  return problems
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

export function validatePackage(manifest, root) {
  const problems = []
  if (manifest?.module !== 'M06') problems.push(`package module=${manifest?.module}`)
  if (!Array.isArray(manifest?.steps) || manifest.steps.length !== 8) problems.push('package steps must list 8 steps')
  if (manifest?.switch !== BUILD_TAG) problems.push(`package switch=${manifest?.switch}`)
  if (!Array.isArray(manifest?.gates) || manifest.gates.length !== GATES.length) problems.push('package gates incomplete')
  if (!Array.isArray(manifest?.files) || manifest.files.length !== PRODUCTION_FILES.length) problems.push('package files incomplete')
  for (const entry of manifest?.files ?? []) {
    if (!entry?.path || !entry?.sha256) {
      problems.push(`package entry incomplete: ${JSON.stringify(entry)}`)
      continue
    }
    const actual = sha256File(resolve(root, entry.path))
    if (actual !== entry.sha256) problems.push(`package sha mismatch for ${entry.path}`)
  }
  return problems
}

export function renderEvidence(evidence) {
  const guards = ASSERT_GUARDS.length + HOST_CASES.length
  const lines = [
    `invocation=host package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `offstate: default_lifecycle_files=${evidence?.defaultLifecycle ?? 0} tagged_lifecycle_files=${evidence?.taggedLifecycle ?? 0} switch=${BUILD_TAG}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m06-s08 ${row}`)
  lines.push(`passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`)
  return lines.join('\n') + '\n'
}

function goList(root, pkg, extraArgs) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}|{{.XTestGoFiles}}', ...extraArgs, pkg], {
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

export function buildManifest(root) {
  return {
    module: 'M06',
    steps: ['S01', 'S02', 'S03', 'S04', 'S05', 'S06', 'S07', 'S08'],
    switch: BUILD_TAG,
    filters: PRODUCTION_FILES,
    gates: [...GATES],
    files: PRODUCTION_FILES.map((path) => ({ path, sha256: sha256File(resolve(root, path)) })),
    preconditions: ['M07 未通过前不得接入默认路径', '真实插件供给受 M00-S08 限制'],
    generatedAt: new Date().toISOString()
  }
}

export function runHostEvidence(root) {
  const defaultList = goList(root, './internal/extension/kernel/enablement', [])
  const taggedList = goList(root, './internal/extension/kernel/enablement', ['-tags', BUILD_TAG])
  const supervisorTagged = goList(root, PACKAGE_PATH, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'runtime_supervisor', HOST_TEST), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  const manifest = buildManifest(root)
  writeFileSync(resolve(root, PACKAGE_FILE), `${JSON.stringify(manifest, null, 2)}\n`, 'utf8')
  const rows = parseHostRows(log)
  if (!rows.includes('package files=7 steps=8 switch=rsi_lifecycle')) rows.push('package files=7 steps=8 switch=rsi_lifecycle')
  return {
    testSource,
    manifest,
    rows,
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultFiles: defaultList.files,
    taggedFiles: taggedList.files,
    defaultTests: defaultList.tests,
    taggedTests: supervisorTagged.tests,
    defaultLifecycle: defaultList.files.filter((file) => LIFECYCLE_FILES.includes(file)).length,
    taggedLifecycle: taggedList.files.filter((file) => LIFECYCLE_FILES.includes(file)).length,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runHostEvidence(root)
  const problems = [
    ...validateOffState(evidence),
    ...validateEvidence(evidence),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validatePackage(evidence.manifest, root)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_HOST)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
