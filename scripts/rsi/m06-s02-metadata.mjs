import { spawnSync } from 'node:child_process'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/enablement'
export const RUN_PATTERN = 'TestPluginMetadata'
export const BUILD_TAG = 'rsi_lifecycle'
export const EXIT_METADATA = 66
export const EXPECTED_PASSED = 10
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const INSTALL_SOURCE = 'install.go'
export const INSTALL_TEST = 'install_metadata_test.go'
export const STATE_TEST = 'lifecycle_state_test.go'
export const PACKAGE_DIR = 'backend/internal/extension/kernel/enablement'

export const METADATA_CASES = [
  'TestPluginMetadataThousandStaysCold',
  'TestPluginMetadataInstallDoesNotSpawn',
  'TestPluginMetadataIndexFieldsOnly',
  'TestPluginMetadataDuplicateIdempotent',
  'TestPluginMetadataInvalidRejected',
  'TestPluginMetadataCancelAtomic',
  'TestPluginMetadataEnableNoSpawn',
  'TestPluginMetadataUninstallRemoves',
  'TestPluginMetadataActivationBoundary',
  'TestPluginMetadataConcurrentBatches'
]

export const SOURCE_GUARDS = [
  'type MetadataIndex struct {',
  'func (i *MetadataIndex) set(entry Metadata) {',
  'type RuntimeSpawner interface {',
  'Spawn(ctx context.Context, metadata Metadata) (RuntimeHandle, error)',
  'staged := map[string]Metadata{}',
  'if err := ctx.Err(); err != nil {',
  'i.index.set(entry)',
  'if machine.State() != LifecycleEnabled {',
  'handle, err := i.spawner.Spawn(ctx, metadata)',
  'SpawnCalls: i.spawns,'
]

export const ASSERT_GUARDS = [
  'if snapshot.Plugins != 1000 || snapshot.Processes != 0 || snapshot.Workers != 0 || snapshot.SpawnCalls != 0 {',
  'if spawner.Calls() != 1 || installer.Snapshot().Processes != 1 {',
  'if handles != 0 {',
  'if report.Installed != 0 || report.Skipped != 3 {',
  'if installer.Snapshot().Plugins != 0 {',
  'if _, err := installer.InstallBatch(ctx, metadataManifests(10)); !errors.Is(err, context.Canceled) {',
  'if snapshot.Processes != 0 || snapshot.Workers != 0 || spawner.Calls() != 0 {',
  'if _, ok := installer.index.Get("plugin-0000"); ok {',
  'if err := installer.Activate(ctx, "plugin-0000", 1); !errors.Is(err, ErrInstallerState) {',
  'if snapshot.Plugins != 1000 || snapshot.Processes != 0 || spawner.Calls() != 0 {'
]

export const FORBIDDEN_TOKENS = ['Reconcile(', 'go func(', 'time.Sleep(', 'os.Setenv']

export const EVIDENCE_ROWS = [
  'cold plugins=1000 processes=0 workers=0 spawn_calls=0',
  'install spawn_calls=0 enable_calls=0 activate_calls=1',
  'metadata fields=4 runtime_handles=0 state=installed',
  'duplicate installed=0 skipped=3 entries=3',
  'invalid rejected=3 entries=0',
  'cancel entries=0 err=context.Canceled',
  'enable plugins=5 processes=0 workers=0 spawn_calls=0',
  'uninstall entries=1 state_missing=true spawn_calls=0',
  'activation requires_enabled=true replay_spawns=1 unknown=ErrInstallerState',
  'concurrent batches=2 entries=1000 spawn_calls=0'
]

export function parseMetadataRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m06-s02 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm06-s02 '.length).trim())
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

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateSource(text) {
  return missingGuards(text, SOURCE_GUARDS, 'install metadata path')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'metadata test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return METADATA_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `metadata test is missing ${name}`)
}

export function validateNoRuntimeWork(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `install path must not contain ${token}`)
}

export function scanInstallerConsumers(root, target = 'enablement.NewInstaller(') {
  const ownPrefix = resolve(root, PACKAGE_DIR) + '/'
  const hits = []
  const walk = (directory) => {
    for (const entry of readdirSync(directory)) {
      const path = join(directory, entry)
      const stats = statSync(path)
      if (stats.isDirectory()) {
        if (entry === 'node_modules' || entry === '.git' || entry === 'artifacts') continue
        walk(path)
        continue
      }
      if (!entry.endsWith('.go') || entry.endsWith('_test.go')) continue
      if (path.startsWith(ownPrefix)) continue
      if (readFileSync(path, 'utf8').includes(target)) hits.push(path.slice(resolve(root).length + 1))
    }
  }
  walk(resolve(root, 'backend'))
  return hits.sort()
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  for (const file of [INSTALL_TEST, STATE_TEST]) {
    if (defaultTests.includes(file)) problems.push(`default build leaked ${file}`)
    if (!taggedTests.includes(file)) problems.push(`tagged tests missing ${file}`)
  }
  for (const file of defaultTests) {
    if (!taggedTests.includes(file)) problems.push(`tagged build dropped default test ${file}`)
  }
  for (const file of taggedTests) {
    if (!defaultTests.includes(file) && !file.endsWith('_test.go')) problems.push(`tagged build leaked non-test file ${file}`)
  }
  for (const consumer of evidence?.consumers ?? []) problems.push(`installer is wired before the integration step: ${consumer}`)
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

export function renderEvidence(evidence) {
  const guards = SOURCE_GUARDS.length + ASSERT_GUARDS.length + METADATA_CASES.length
  const lines = [
    `invocation=metadata package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} consumers=${(evidence?.consumers ?? []).length}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m06-s02 ${row}`)
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
    tests: [...parseGoListFiles(tests), ...parseGoListFiles(external)].sort(),
    exitCode: result.status === null ? 1 : result.status
  }
}

export function runMetadataEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement')
  const source = readFileSync(resolve(base, INSTALL_SOURCE), 'utf8')
  const testSource = readFileSync(resolve(base, INSTALL_TEST), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    source,
    testSource,
    rows: parseMetadataRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultTests: defaultList.tests,
    taggedTests: taggedList.tests,
    consumers: scanInstallerConsumers(root),
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runMetadataEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(evidence.source),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoRuntimeWork(evidence.source)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_METADATA)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
