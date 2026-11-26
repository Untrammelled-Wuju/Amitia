import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestSchemaCache'
export const BUILD_TAG = 'rsi_schema_cache'
export const EXIT_SCHEMA_CACHE = 62
export const EXPECTED_PASSED = 9
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const CACHE_SOURCE = 'schema_cache_test.go'
export const DETAIL_SOURCE = 'detail_budget.go'
export const STORE_SOURCE = 'schema_store.go'
export const REVISION_SOURCE = 'descriptor_revision.go'

export const SCHEMA_CASES = [
  'TestSchemaCacheContentAddressIsolation',
  'TestSchemaCacheRevisionBumpReplacesRef',
  'TestSchemaCacheActorIndexIsolation',
  'TestSchemaCachePermissionSwitchNoRead',
  'TestSchemaCacheExpiryRequiresExplicitFresh',
  'TestSchemaCacheRevisionGapStopsApply',
  'TestSchemaCacheStaleRepairRejected',
  'TestSchemaCacheOnlySelectedToolsRead',
  'TestSchemaCacheCancelAndInvalid'
]

export const DETAIL_GUARDS = [
  'for _, selected := range report.Selected[offset:end] {',
  'schema, err := m.detailSchema(selected.Tool)',
  'page.SchemaReads = m.schemaStore.Reads() - readsBefore'
]

export const STORE_GUARDS = [
  'ref := NewSchemaRef(body)',
  'if NewSchemaRef(body) != ref {',
  'if requireFresh {',
  'return nil, fmt.Errorf("%w: ref=%s", ErrSchemaExpired, ref)'
]

export const REVISION_GUARDS = [
  'if event.Revision > i.revision+1 {',
  'return report, fmt.Errorf("%w: snapshot=%d current=%d", ErrDescriptorRevisionStale, revision, current)'
]

export const ASSERT_GUARDS = [
  'if refFirst != capability.NewSchemaRef(first) {',
  'if _, err := store.PutAt(refFirst, second); !errors.Is(err, capability.ErrSchemaRefMismatch) {',
  'if entryV1.SchemaRef == entryV2.SchemaRef {',
  'if _, err := indexB.Hydrate(context.Background(), store, []string{allowed.ID}, capability.HydrateOptions{RequireFresh: true}); !errors.Is(err, capability.ErrDescriptorUnknown) {',
  'if store.Reads() != readsBefore {',
  'if _, err := store.Load(ref, true); !errors.Is(err, capability.ErrSchemaExpired) {',
  'if report.Gap == nil || report.Gap.Expected != 2 || report.Gap.Received != 3 {',
  'if page.SchemaReads != int64(len(page.Views)) || page.SchemaReads > 2 {',
  'if injected.SchemaReads != int64(len(injected.Views)) || len(injected.Views) > 2 {',
  'if _, err := store.Load(capability.SchemaRef("sha256:not-a-hash"), false); !errors.Is(err, capability.ErrSchemaRefInvalid) {'
]

export const FORBIDDEN_TOKENS = ['range report.Selected {', 'store.Get(', 'registry.Register(ctx']

export const EVIDENCE_ROWS = [
  'content distinct_refs=true overwrite=ErrSchemaRefMismatch stable=true',
  'revision v1!=v2=true hydrate=updated index_refs_stale=false',
  'actor cross_index=ErrDescriptorUnknown cross_reads=0 owner_reads=1',
  'permission blocked_views=1 blocked_reads=1 granted_views=2 granted_reads=2',
  'ttl stale_load=ok fresh_load=ErrSchemaExpired content_immutable=true',
  'gap expected=2 got=3 revision_unchanged=true',
  'repair stale=ErrDescriptorRevisionStale index_unchanged=true',
  'scope selected_only=true views=2 reads=2 injected_extra=0',
  'invalid cancel=context.Canceled nil_store=ErrDescriptorInvalid bad_ref=ErrSchemaRefInvalid'
]

export function parseSchemaRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s10 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s10 '.length).trim())
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

export function validateDetailSource(text) {
  return missingGuards(text, DETAIL_GUARDS, 'detail path')
}

export function validateStoreSource(text) {
  return missingGuards(text, STORE_GUARDS, 'schema store')
}

export function validateRevisionSource(text) {
  return missingGuards(text, REVISION_GUARDS, 'descriptor revision')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'schema cache test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return SCHEMA_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `schema cache test is missing ${name}`)
}

export function validateNoFullScan(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `schema cache surface must not contain ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(CACHE_SOURCE)) problems.push(`default build leaked ${CACHE_SOURCE}`)
  if (!taggedTests.includes(CACHE_SOURCE)) problems.push(`tagged tests missing ${CACHE_SOURCE}`)
  const expected = [...defaultTests, CACHE_SOURCE].sort().join(',')
  if (taggedTests.join(',') !== expected) problems.push(`tagged tests=${taggedTests.join(',')} want ${expected}`)
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
  const guards = DETAIL_GUARDS.length + STORE_GUARDS.length + REVISION_GUARDS.length + ASSERT_GUARDS.length + SCHEMA_CASES.length
  const lines = [
    `invocation=schema-cache package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${CACHE_SOURCE}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m05-s10 ${row}`)
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

export function runSchemaEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability')
  const detailSource = readFileSync(resolve(base, DETAIL_SOURCE), 'utf8')
  const storeSource = readFileSync(resolve(base, STORE_SOURCE), 'utf8')
  const revisionSource = readFileSync(resolve(base, REVISION_SOURCE), 'utf8')
  const testSource = readFileSync(resolve(base, CACHE_SOURCE), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    detailSource,
    storeSource,
    revisionSource,
    testSource,
    rows: parseSchemaRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultTests: defaultList.tests,
    taggedTests: taggedList.tests,
    listExitCode: defaultList.exitCode + taggedList.exitCode,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runSchemaEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateDetailSource(evidence.detailSource),
    ...validateStoreSource(evidence.storeSource),
    ...validateRevisionSource(evidence.revisionSource),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoFullScan(evidence.detailSource + evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_SCHEMA_CACHE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
