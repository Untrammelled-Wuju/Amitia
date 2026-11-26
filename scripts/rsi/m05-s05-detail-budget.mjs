import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestDetail'
export const EXIT_DETAIL_BUDGET = 57
export const EXPECTED_PASSED = 10
export const REQUIRED_SOURCES = ['detail_budget.go', 'exposure.go']
export const REQUIRED_TESTS = ['detail_budget_test.go']

export const TRIM_GUARDS = [
  'ErrDetailBudgetExhausted = errors.New(',
  'ErrDetailCursorInvalid   = errors.New(',
  'func TrimSchemaToBudget(schema json.RawMessage, budget int) (json.RawMessage, TrimReport, error) {',
  'if report.OriginalTokens <= budget {',
  'for _, name := range optional {',
  'if !requiredSet[name] {',
  'delete(properties, name)',
  'return nil, report, fmt.Errorf("%w: required=%v need=%d budget=%d", ErrDetailBudgetExhausted, required, report.OriginalTokens, budget)'
]

export const CURSOR_GUARDS = [
  'type detailCursor struct {',
  'func detailDigest(query string, snapshot StrategySnapshot) string {',
  'func encodeDetailCursor(offset int, digest string) (string, error) {',
  'if decoded.Version != detailCursorVersion {',
  'if decoded.Digest != digest {',
  'if decoded.Offset < 0 {'
]

export const PAGE_GUARDS = [
  'func (m *ToolExposureManager) WithSchemaSource(index *DescriptorIndex, store *SchemaStore) *ToolExposureManager {',
  'func (m *ToolExposureManager) GetModelToolDetails(ctx context.Context, sessionCtx ToolInvocationContext, request DetailRequest) (DetailPage, error) {',
  'if limit > MaxDetailTopK {',
  'if budget == 0 {',
  'page.SchemaReads = m.schemaStore.Reads() - readsBefore',
  'if len(page.Views) == 0 {',
  'page.NextCursor = cursor'
]

export const ASSERT_GUARDS = [
  'if len(required) != 1 || required[0] != "req_00" {',
  'if report.FinalTokens > 120 || (bytes+3)/4 > 120 {',
  'if tokens[100] != tokens[1000] || tokens[1000] != tokens[2000] {',
  'if len(seen) != 5 {',
  'if store.Reads() != before {',
  'if len(restartedPage.Views) != 1 || restartedPage.Views[0].Name != "restart-01" {',
  'if len(clamped.Views) != capability.MaxDetailTopK {',
  'if store.Reads() != int64(2*len(first.Views)) {'
]

export const ENUMERATION_TOKENS = ['registry.List(', 'Descriptors(', 'Query(ToolFilter', 'GetModelTools(']

export const EVIDENCE_ROWS = [
  'trim required_kept=1 dropped=10 final_tokens=70 budget=120',
  'required_overflow=refused budget=50 need=351',
  'tokens_n100=5960 tokens_n1000=5960 tokens_n2000=5960 reads=8 equal=true',
  'pages=3 first=2 second=2 last=1 distinct=5',
  'cursor_other_query=rejected cursor_version=rejected cursor_garbage=rejected cursor_past_end=rejected',
  'exhausted=closed views=0 budget=10',
  'trimmed_page views=1 used_within_budget=true dropped>=1 has_more=true pages=3',
  'repeat_equal=true reads_per_call=2',
  'cancel=passed_through store_untouched=true restart_page_equal=true',
  'no_selector=rejected negative_limit=rejected negative_budget=rejected clamp=12'
]

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

export function parseDetailRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s05 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s05 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateTrimSource(text) {
  return missingGuards(text, TRIM_GUARDS, 'detail trim source')
}

export function validateCursorSource(text) {
  return missingGuards(text, CURSOR_GUARDS, 'detail cursor source')
}

export function validatePageSource(text) {
  return missingGuards(text, PAGE_GUARDS, 'detail page source')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'detail budget test')
}

export function validateNoRegistryEnumeration(text) {
  const body = String(text ?? '')
  return ENUMERATION_TOKENS.filter((token) => body.includes(token)).map((token) => `detail page must not enumerate the registry but contains ${token}`)
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
  const guards = TRIM_GUARDS.length + CURSOR_GUARDS.length + PAGE_GUARDS.length + ASSERT_GUARDS.length
  const lines = [
    `invocation=detail-budget package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m05-s05 ${row}`)
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

export function runDetailEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability')
  const detailSource = readFileSync(resolve(base, 'detail_budget.go'), 'utf8')
  const exposureSource = readFileSync(resolve(base, 'exposure.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'detail_budget_test.go'), 'utf8')
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
    rows: parseDetailRows(log),
    detailSource,
    exposureSource,
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
      process.stdout.write(`invocation=detail-budget package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runDetailEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateTrimSource(evidence.detailSource),
      ...validateCursorSource(evidence.detailSource),
      ...validatePageSource(evidence.detailSource),
      ...validateNoRegistryEnumeration(evidence.detailSource),
      ...validateAssertSource(evidence.testSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_DETAIL_BUDGET)
    if (evidence.exitCode !== 0) process.exit(EXIT_DETAIL_BUDGET)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_DETAIL_BUDGET)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
