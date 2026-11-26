import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestRanking'
export const EXIT_TASK_RANKING = 56
export const EXPECTED_PASSED = 9
export const REQUIRED_SOURCES = ['ranking.go', 'task_selector.go']
export const REQUIRED_TESTS = ['ranking_test.go']
export const REQUIRED_FIXTURES = ['testdata/ranking_fixture.json']

export const STRATEGY_GUARDS = [
  'type RankingStrategy interface {',
  'Version() string',
  'Weights() RankingWeights',
  'Rank(ctx context.Context, profile TaskProfile, query string, candidates []RankCandidate) ([]RankedTool, error)',
  'type LexicalStrategy struct{}',
  'func NewHybridStrategy(weights RankingWeights, usage UsageProvider) (*HybridStrategy, error) {',
  'func (w RankingWeights) Valid() bool {',
  'func (s UsageStat) Reliability() float64 {',
  'func (s UsageStat) Volume() float64 {',
  'ErrRankingWeightsInvalid      = errors.New('
]

export const POLICY_GUARDS = [
  'type StrategySnapshot struct {',
  'Digest   string         `json:"digest"`',
  'func (s StrategySnapshot) computeDigest() (string, error) {',
  'sum := sha256.Sum256(body)',
  'func (p *RankingPolicy) Activate(ctx context.Context, version string) error {',
  'func (p *RankingPolicy) Restore(ctx context.Context, snapshot StrategySnapshot) error {',
  'func (p *RankingPolicy) Rollback(ctx context.Context) (StrategySnapshot, error) {',
  'if p.previous == "" {',
  'p.active, p.previous = p.previous, p.active',
  'if digest != snapshot.Digest {',
  'return fmt.Errorf("%w: want=%s got=%s", ErrRankingSnapshotDrift, digest, snapshot.Digest)',
  'ErrRankingVersionUnknown      = errors.New('
]

export const INTEGRATION_GUARDS = [
  'ranking      *RankingPolicy',
  'func (s *TaskSelector) WithRanking(policy *RankingPolicy) *TaskSelector {',
  'candidates := make([]RankCandidate, 0, 8)',
  'if s.ranking != nil {',
  'ranked, snapshot, err := s.ranking.Rank(ctx, report.Profile, query, candidates)',
  'report.Strategy = snapshot',
  'sort.SliceStable(report.Selected, func(i, j int) bool {'
]

export const ASSERT_GUARDS = [
  'if report.Selected[0].Tool.ID != query.Top1 {',
  'if lexicalReport.Selected[0].Tool.ID != "tool-report-reader" {',
  'if top := rankingTop1(t, selector, "把周报整理成报告").Selected[0].Tool.ID; top != "tool-report-writer" {',
  'if !strings.HasPrefix(before.Digest, "sha256:") || len(before.Digest) != len("sha256:")+64 {',
  'if err := fresh.Restore(ctx, tampered); !errors.Is(err, capability.ErrRankingSnapshotDrift) {',
  'if _, err := policy.Rollback(ctx); !errors.Is(err, capability.ErrRankingRollbackUnavailable) {',
  'if policy.Rankings() != 2 {',
  'if !errors.Is(err, sentinel) {',
  'if len(fixture.Tools) != 4 {'
]

export const FILTER_BYPASS_TOKENS = ['PermissionGranted', 'ReasonPermissionMissing', 'ComputedState()', 's.grants', 'HealthReady']

export const EVIDENCE_ROWS = [
  'explicit_writer_top1=tool-report-writer explicit_reader_top1=tool-report-reader',
  'confusable_lexical_top1=tool-report-reader hybrid_top1=tool-report-writer usage_prior=4.38',
  'snapshot_version=lexical-v1 digest_sha256=true rollback_version=lexical-v1 rollback_restored=true',
  'restart_restored=true snapshot_version=hybrid-v2 corrupted_snapshot=drift',
  'missing_strategy=rejected invalid_weights=rejected unknown_version=rejected empty_rollback=rejected',
  'repeat_orders_equal=true rankings=2 canceled=passed_through',
  'selector_strategy=hybrid-v2 selector_top1=tool-report-writer denied_filtered=true',
  'usage_error=chained top1_empty=true',
  'fixture_tools=4 fixture_queries=3 modes=explicit,confusable top1_valid=true'
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

export function parseRankingRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s04 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s04 '.length).trim())
  }
  return rows
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateStrategySource(text) {
  return missingGuards(text, STRATEGY_GUARDS, 'ranking strategy source')
}

export function validatePolicySource(text) {
  return missingGuards(text, POLICY_GUARDS, 'ranking policy source')
}

export function validateIntegrationSource(text) {
  return missingGuards(text, INTEGRATION_GUARDS, 'task selector integration')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'ranking test')
}

export function validateNoFilterBypass(text) {
  const body = String(text ?? '')
  return FILTER_BYPASS_TOKENS.filter((token) => body.includes(token)).map((token) => `ranking strategy must not re-implement the hard filter but contains ${token}`)
}

export function validateFixture(raw) {
  const problems = []
  let fixture
  try {
    fixture = JSON.parse(String(raw ?? ''))
  } catch (error) {
    return [`ranking fixture is not valid JSON: ${error.message}`]
  }
  const tools = Array.isArray(fixture.tools) ? fixture.tools : []
  const queries = Array.isArray(fixture.queries) ? fixture.queries : []
  if (fixture.schemaVersion !== 1) problems.push(`ranking fixture schemaVersion=${fixture.schemaVersion} want 1`)
  if (tools.length !== 4) problems.push(`ranking fixture tools=${tools.length} want 4`)
  if (queries.length !== 3) problems.push(`ranking fixture queries=${queries.length} want 3`)
  const ids = new Set(tools.map((tool) => tool?.id))
  const modes = new Set(queries.map((query) => query?.mode))
  for (const mode of ['explicit', 'confusable']) {
    if (!modes.has(mode)) problems.push(`ranking fixture is missing a ${mode} query`)
  }
  for (const query of queries) {
    if (!ids.has(query?.top1)) problems.push(`ranking fixture query expects unknown tool ${query?.top1}`)
  }
  const weights = fixture.weights ?? {}
  const values = ['lexical', 'domain', 'usage', 'explicit'].map((key) => weights[key])
  if (values.some((value) => typeof value !== 'number' || !Number.isFinite(value) || value < 0)) {
    problems.push('ranking fixture weights must be finite and non-negative')
  } else if (values.reduce((sum, value) => sum + value, 0) <= 0) {
    problems.push('ranking fixture weights must not all be zero')
  }
  return problems
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
  const guards = STRATEGY_GUARDS.length + POLICY_GUARDS.length + INTEGRATION_GUARDS.length + ASSERT_GUARDS.length
  const lines = [
    `invocation=task-ranking package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `files=${goFiles.length} tests=${testFiles.length} fixtures=${REQUIRED_FIXTURES.length} sources=${REQUIRED_SOURCES.filter((file) => goFiles.includes(file)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`
  ]
  for (const row of rows) lines.push(`m05-s04 ${row}`)
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

export function runRankingEvidence(root) {
  const listed = goList(root)
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability')
  const rankingSource = readFileSync(resolve(base, 'ranking.go'), 'utf8')
  const selectorSource = readFileSync(resolve(base, 'task_selector.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'ranking_test.go'), 'utf8')
  const fixtureSource = readFileSync(resolve(base, 'testdata', 'ranking_fixture.json'), 'utf8')
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
    rows: parseRankingRows(log),
    rankingSource,
    selectorSource,
    testSource,
    fixtureSource,
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
      process.stdout.write(`invocation=task-ranking package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runRankingEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateStrategySource(evidence.rankingSource),
      ...validatePolicySource(evidence.rankingSource),
      ...validateNoFilterBypass(evidence.rankingSource),
      ...validateIntegrationSource(evidence.selectorSource),
      ...validateAssertSource(evidence.testSource),
      ...validateFixture(evidence.fixtureSource),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_TASK_RANKING)
    if (evidence.exitCode !== 0) process.exit(EXIT_TASK_RANKING)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_TASK_RANKING)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
