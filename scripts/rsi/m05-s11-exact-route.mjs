import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestExactRoute'
export const EXIT_EXACT_ROUTE = 63
export const EXPECTED_PASSED = 9
export const GO_TEST_FLAGS = ['-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const ROUTE_SOURCE = 'exact_route.go'
export const ROUTE_TEST = 'exact_route_test.go'
export const SELECTOR_SOURCE = 'task_selector.go'
export const PACKAGE_DIR = 'backend/internal/extension/kernel/capability'

export const EXACT_CASES = [
  'TestExactRouteOpensPluginCenterWithoutRanking',
  'TestExactRouteNormalizesWhitespaceCaseAndPunctuation',
  'TestExactRouteRequiresWholeMatch',
  'TestExactRoutePermissionDeniedNoFallback',
  'TestExactRouteUnregisteredToolRejected',
  'TestExactRouteSharedAliasDeterministicAndLimited',
  'TestExactRouteCancelStopsBeforeLookup',
  'TestExactRouteInjectionTextFallsBackSafely',
  'TestExactRouteRebuildTracksMetadata'
]

export const SOURCE_GUARDS = [
  'const ExactRouteVersion = "exact-route-v1"',
  'func normalizeExactKey(text string) string {',
  'return strings.Join(strings.Fields(trimmed), " ")',
  'func (i *ExactRouteIndex) Lookup(query string) ([]string, bool) {',
  'ids, ok := i.byKey[key]',
  'func (s *TaskSelector) WithExactRoutes(index *ExactRouteIndex) *TaskSelector {',
  'func (s *TaskSelector) selectExact(ctx context.Context, profile TaskProfile, invocation ToolInvocationContext, ids []string, limit int) (SelectionReport, error) {',
  'tool, ok := s.registry.Get(ctx, id)',
  'if len(ids) == 0 {'
]

export const SELECTOR_GUARDS = [
  'if ids, ok, err := s.exactLookup(ctx, query); err != nil {',
  '} else if ok {',
  'return s.selectExact(ctx, report.Profile, invocation, ids, limit)'
]

export const ASSERT_GUARDS = [
  'if recorder.calls != 0 {',
  'if report.Strategy.Version != capability.ExactRouteVersion {',
  'if recorder.calls != 3 {',
  'if index.Hits() != 0 || index.Misses() != 3 {',
  'if !taskGateHasReason(report.Rejected[0].Reasons, capability.ReasonPermissionMissing) {',
  'if !taskGateHasReason(report.Rejected[0].Reasons, capability.ReasonNotInstalled) {',
  'if ids != "tool-plugin-center-a,tool-plugin-center-b" {',
  'if index.Hits() != 0 || index.Misses() != 0 {',
  'if index.Keys() >= keysBefore {',
  'if report.Strategy.Version == capability.ExactRouteVersion {'
]

export const FORBIDDEN_TOKENS = ['registry.List(', 's.ranking.Rank(', 'scoreTool(']

export const EVIDENCE_ROWS = [
  'hit route=settings/plugins selected=tool-plugin-center ranking_calls=0 hits=1',
  'normalize queries=4 all_exact=true hits=4',
  'fallback queries=3 ranking_calls=3 exact_hits=0',
  'denied selected=0 rejected=1 ranking_calls=0',
  'unregistered selected=0 rejected=1 reason=not_installed',
  'shared order=tool-plugin-center-a,tool-plugin-center-b limit1=first',
  'cancel hits=0 ranking_calls=0 err=context.Canceled',
  'injection exact=false denied_selected=false ranking_calls=1',
  'rebuild keys_before=4 keys_after=2 stale_alias=false route_still_exact=true'
]

export function parseExactRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s11 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s11 '.length).trim())
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

export function validateRouteSource(text) {
  return missingGuards(text, SOURCE_GUARDS, 'exact route index')
}

export function validateSelectorSource(text) {
  const problems = missingGuards(text, SELECTOR_GUARDS, 'task selector')
  const body = String(text ?? '')
  const lookup = body.indexOf('s.exactLookup(ctx, query)')
  const lexical = body.indexOf('lowered := strings.ToLower(query)')
  if (lookup < 0 || lexical < 0) return problems
  if (lookup > lexical) problems.push('exact lookup must run before the lexical scan')
  return problems
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'exact route test')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return EXACT_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `exact route test is missing ${name}`)
}

export function validateNoCatalogScan(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `exact route index must not contain ${token}`)
}

export function scanRouteConsumers(root, target = 'WithExactRoutes(') {
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
      if (!entry.endsWith('.go')) continue
      if (path.startsWith(ownPrefix)) continue
      if (readFileSync(path, 'utf8').includes(target)) hits.push(path.slice(resolve(root).length + 1))
    }
  }
  walk(resolve(root, 'backend'))
  return hits.sort()
}

export function validateIsolation(evidence) {
  const problems = []
  if (!(evidence?.files ?? []).includes(ROUTE_SOURCE)) problems.push(`default build files missing ${ROUTE_SOURCE}`)
  if (!(evidence?.tests ?? []).includes(ROUTE_TEST)) problems.push(`default tests missing ${ROUTE_TEST}`)
  for (const consumer of evidence?.consumers ?? []) problems.push(`exact routes are wired before the integration step: ${consumer}`)
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
  const guards = SOURCE_GUARDS.length + SELECTOR_GUARDS.length + ASSERT_GUARDS.length + EXACT_CASES.length
  const lines = [
    `invocation=exact-route package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=[${(evidence?.files ?? []).join(' ')}] tests=[${(evidence?.tests ?? []).join(' ')}] consumers=${(evidence?.consumers ?? []).length}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m05-s11 ${row}`)
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
    tests: [...parseGoListFiles(tests), ...parseGoListFiles(external)].sort(),
    exitCode: result.status === null ? 1 : result.status
  }
}

export function runExactEvidence(root) {
  const list = goList(root)
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability')
  const routeSource = readFileSync(resolve(base, ROUTE_SOURCE), 'utf8')
  const selectorSource = readFileSync(resolve(base, SELECTOR_SOURCE), 'utf8')
  const testSource = readFileSync(resolve(base, ROUTE_TEST), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    routeSource,
    selectorSource,
    testSource,
    rows: parseExactRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    files: list.files,
    tests: list.tests,
    listExitCode: list.exitCode,
    consumers: scanRouteConsumers(root),
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runExactEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateRouteSource(evidence.routeSource),
    ...validateSelectorSource(evidence.selectorSource),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoCatalogScan(evidence.routeSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_EXACT_ROUTE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
