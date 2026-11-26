import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestCapabilityDescriptor'
export const EXIT_DESCRIPTOR_INDEX = 53
export const EXPECTED_PASSED = 8
export const REQUIRED_SOURCES = ['schema_store.go', 'descriptor_index.go']
export const REQUIRED_TESTS = ['descriptor_index_test.go']

export const STORE_GUARDS = [
  'const SchemaRefPrefix = "sha256:"',
  'func NewSchemaRef(body []byte) SchemaRef {',
  'sum := sha256.Sum256(body)',
  'func (r SchemaRef) Valid() bool {',
  'ErrSchemaRefMismatch  = errors.New(',
  'ErrSchemaRefImmutable = errors.New(',
  'func (s *SchemaStore) PutAt(ref SchemaRef, body []byte) (bool, error) {',
  'if NewSchemaRef(body) != ref {',
  'func (s *SchemaStore) Load(ref SchemaRef, requireFresh bool) ([]byte, error) {',
  'if fresh, _ := s.Fresh(ref, s.now()); !fresh {'
]

export const INDEX_GUARDS = [
  'type DescriptorEntry struct {',
  'type DescriptorIndex struct {',
  'func BuildDescriptorIndex(store *SchemaStore, tools []ToolDefinition) (*DescriptorIndex, error) {',
  'ref, _, err := store.Put(tool.InputSchema)',
  'func (i *DescriptorIndex) Query(filter ToolFilter) []DescriptorEntry {',
  'func (i *DescriptorIndex) Hydrate(ctx context.Context, store *SchemaStore, toolIDs []string, options HydrateOptions) ([]ModelToolView, error) {',
  'if err := ctx.Err(); err != nil {',
  'if _, exists := byID[tool.ID]; exists {'
]

export const ASSERT_GUARDS = [
  'if store.Writes() != 1000 || store.Reads() != 0 {',
  'if store.Reads() != before {',
  'if store.Reads() != int64(len(wanted)) {',
  'if first != second || createdAgain {',
  'if _, err := store.PutAt(ref, descriptorSchema(4)); !errors.Is(err, capability.ErrSchemaRefMismatch) {',
  'if _, err := store.Load(ref, true); !errors.Is(err, capability.ErrSchemaExpired) {',
  'stale, err := store.Load(ref, false)',
  'if _, err := index.Hydrate(canceled, store, wanted, capability.HydrateOptions{}); !errors.Is(err, context.Canceled) {'
]

export const EVIDENCE_ROWS = [
  'tools=1000 entries=1000 schema_reads=0 schema_writes=1000 refs_valid=true',
  'queries=2 entries=1000 schema_reads=0 sorted=true',
  'requested=5 hydrated=5 schema_reads=5 unknown=rejected canceled=passed_through',
  'ref=content_hash repeat=idempotent writes=2 distinct_refs=true',
  'overwrite=mismatch_rejected invalid_ref=rejected historical_read=identical missing=rejected',
  'ttl=1m fresh_then_expired=true require_fresh=rejected historical=readable',
  'empty_body=rejected malformed_json=rejected duplicate_id=rejected nameless=rejected',
  'tools=50 rebuild_writes=50 entries_equal=true'
]

export const QUERY_WRITE_TOKENS = ['store.Load(', 'store.Get(', 'store.Put(', 'store.PutAt(']

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

export function parseDescriptorRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s01 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s01 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateStoreSource(text) {
  return missingGuards(text, STORE_GUARDS, 'schema store source')
}

export function validateIndexSource(text) {
  return missingGuards(text, INDEX_GUARDS, 'descriptor index source')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'descriptor index test')
}

function functionBody(text, signature) {
  const body = String(text ?? '')
  const start = body.indexOf(signature)
  if (start < 0) return ''
  const rest = body.slice(start + signature.length)
  const end = rest.indexOf('\nfunc ')
  return end < 0 ? rest : rest.slice(0, end)
}

export function validateQueryReadOnly(text) {
  const query = functionBody(text, 'func (i *DescriptorIndex) Query(filter ToolFilter) []DescriptorEntry {')
  if (query === '') return ['descriptor index source is missing Query']
  return QUERY_WRITE_TOKENS.filter((token) => query.includes(token)).map((token) => `descriptor query must stay schema-free but contains ${token}`)
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
  const goFiles = evidence?.goFiles ?? []
  const testFiles = evidence?.testFiles ?? []
  const rows = evidence?.rows ?? []
  const guards = STORE_GUARDS.length + INDEX_GUARDS.length + ASSERT_GUARDS.length
  const lines = [
    `invocation=descriptor-index package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m05-s01 ${row}`)
  lines.push(`passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`)
  return lines.join('\n') + '\n'
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

export function runDescriptorEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability')
  const storeSource = readFileSync(resolve(base, 'schema_store.go'), 'utf8')
  const indexSource = readFileSync(resolve(base, 'descriptor_index.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'descriptor_index_test.go'), 'utf8')
  const result = spawnSync('go', ['test', '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
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
    rows: parseDescriptorRows(log),
    storeSource,
    indexSource,
    testSource,
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
      process.stdout.write(`invocation=descriptor-index package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runDescriptorEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateStoreSource(evidence.storeSource),
      ...validateIndexSource(evidence.indexSource),
      ...validateQueryReadOnly(evidence.indexSource),
      ...validateAssertSource(evidence.testSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_DESCRIPTOR_INDEX)
    if (evidence.exitCode !== 0) process.exit(EXIT_DESCRIPTOR_INDEX)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_DESCRIPTOR_INDEX)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
