import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestCapabilityRevision'
export const EXIT_DESCRIPTOR_REVISION = 54
export const EXPECTED_PASSED = 7
export const REQUIRED_SOURCES = ['descriptor_revision.go', 'descriptor_index.go']
export const REQUIRED_TESTS = ['descriptor_revision_test.go']

export const EVENT_GUARDS = [
  'type DescriptorEvent struct {',
  'func (k DescriptorEventKind) Valid() bool {',
  'ErrDescriptorRevisionGap   = errors.New(',
  'ErrDescriptorRevisionStale = errors.New(',
  'type RevisionGap struct {',
  'func (g *RevisionGap) Unwrap() error {',
  'func (i *DescriptorIndex) Apply(ctx context.Context, store *SchemaStore, events []DescriptorEvent) (ApplyReport, error) {',
  'func (i *DescriptorIndex) Repair(ctx context.Context, store *SchemaStore, snapshot []ToolDefinition, revision uint64) (RepairReport, error) {'
]

export const APPLY_GUARDS = [
  'if event.Revision <= i.revision {',
  'if event.Revision > i.revision+1 {',
  'gap := &RevisionGap{Expected: i.revision + 1, Received: event.Revision}',
  'if !consumed {',
  'i.revision = event.Revision',
  'if revision <= current {',
  'i.rebuilds++'
]

export const ASSERT_GUARDS = [
  'if report.Added != 200 || index.Len() != 200 || index.Revision() != 201 {',
  'if again.Skipped != 200 || again.Applied != 0 {',
  'if index.Len() != 200 || index.Revision() != 201 || index.Mutations() != mutations {',
  'if !errors.Is(err, capability.ErrDescriptorRevisionGap) {',
  'if report.Gap == nil || report.Gap.Expected != 4 || report.Gap.Received != 5 {',
  'if index.Len() != 2 || index.Revision() != 3 {',
  'if _, err := index.Repair(context.Background(), store, tools, 5); !errors.Is(err, capability.ErrDescriptorRevisionStale) {',
  'if len(left) != len(right) || len(left) != 28 {'
]

export const EVIDENCE_ROWS = [
  'registered=200 uninstalled=150 remaining=50 stale_entries=0 revision=351',
  'replay=200 skipped=200 len=200 uninstall_replay=10 len_after=190',
  'gap_expected=4 gap_received=5 index_untouched=true repaired_added=4 kept=2 revision=5 rebuilds=1 stale_rejected=true',
  'updated=1 len=1 new_ref=true old_schema_readable=true',
  'unknown_unregister=skipped applied=0 revision_consumed=2',
  'zero_revision=rejected unknown_kind=rejected missing_id=rejected nameless=rejected canceled=passed_through',
  'registered=40 uninstalled=12 repaired=28 restart_match=true rebuilds=1'
]

export const TIMER_TOKENS = ['time.NewTicker', 'time.AfterFunc', 'time.Tick(', 'time.After(']

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

export function parseRevisionRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s02 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s02 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateEventSource(text) {
  return missingGuards(text, EVENT_GUARDS, 'descriptor revision source')
}

export function validateApplySource(text) {
  return missingGuards(text, APPLY_GUARDS, 'descriptor revision apply')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'descriptor revision test')
}

export function validateNoTimerRebuild(text) {
  const body = String(text ?? '')
  return TIMER_TOKENS.filter((token) => body.includes(token)).map((token) => `revision repair must stay event-driven but contains ${token}`)
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
  const guards = EVENT_GUARDS.length + APPLY_GUARDS.length + ASSERT_GUARDS.length
  const lines = [
    `invocation=descriptor-revision package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m05-s02 ${row}`)
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

export function runRevisionEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability')
  const revisionSource = readFileSync(resolve(base, 'descriptor_revision.go'), 'utf8')
  const indexSource = readFileSync(resolve(base, 'descriptor_index.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'descriptor_revision_test.go'), 'utf8')
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
    rows: parseRevisionRows(log),
    revisionSource,
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
      process.stdout.write(`invocation=descriptor-revision package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runRevisionEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateEventSource(evidence.revisionSource),
      ...validateApplySource(evidence.revisionSource),
      ...validateNoTimerRebuild(evidence.revisionSource),
      ...validateAssertSource(evidence.testSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_DESCRIPTOR_REVISION)
    if (evidence.exitCode !== 0) process.exit(EXIT_DESCRIPTOR_REVISION)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_DESCRIPTOR_REVISION)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
