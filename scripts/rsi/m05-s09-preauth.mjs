import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestPreauth'
export const BUILD_TAG = 'rsi_preauth'
export const EXIT_PREAUTH = 61
export const EXPECTED_PASSED = 9
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const PREAUTH_SOURCE = 'preauth_test.go'
export const SELECTOR_SOURCE = 'task_selector.go'

export const PREAUTH_CASES = [
  'TestPreauthDeniedNeverReachesRanking',
  'TestPreauthContrastHighestScoreExcluded',
  'TestPreauthGrantErrorFailsClosed',
  'TestPreauthScopeAndPlatformFilteredBeforeRanking',
  'TestPreauthLimitDoesNotFallBackToDenied',
  'TestPreauthInjectedTextCannotBypass',
  'TestPreauthRepeatedRequestsAreStable',
  'TestPreauthCancelBeforeRanking',
  'TestPreauthGrantedOrderUnchanged'
]

export const ORDER_ANCHORS = [
  'if err := ctx.Err(); err != nil {',
  'for _, tool := range s.registry.List(ctx, ToolFilter{}) {',
  'reasons, cause, err := s.hardFilter(ctx, tool, invocation)',
  'if len(reasons) > 0 {',
  'continue',
  'score, matched, domains := scoreTool(tool, lowered)',
  'candidates = append(candidates, RankCandidate{',
  'if s.ranking != nil {',
  'ranked, snapshot, err := s.ranking.Rank(ctx, report.Profile, query, candidates)'
]

export const ASSERT_GUARDS = [
  'if preauthContains(seen[0], denied.ID) {',
  'if !taskGateHasReason(taskGateReasons(report, denied.ID), capability.ReasonPermissionMissing) {',
  'if !errors.Is(err, failure) {',
  'if calls != 0 {',
  'if len(report.Selected) != 0 {',
  'if len(seen) != 1 || len(seen[0]) != 1 || seen[0][0] != allowed.ID {',
  'if len(report.Rejected) != 1 || report.Rejected[0].ToolID != denied.ID {',
  'if strings.Join(seen[index], ",") != strings.Join(seen[0], ",") {',
  'if !errors.Is(err, context.Canceled) {',
  'if strings.Join(preauthIDs(plain), ",") != strings.Join(preauthIDs(withPolicy), ",") {'
]

export const FORBIDDEN_TOKENS = ['registry.Register(ctx', 'GetModelTools(ctx', 'WithRanking(nil)']

export const EVIDENCE_ROWS = [
  'order denied_reached_ranking=false selected=tool-allowed,tool-other',
  'contrast denied_score=28 blocked_score=12 ranking_seen=1',
  'fail_closed ranking_calls=0 selected=0 chained=true',
  'scope platform_filtered=2 candidates=1',
  'limit_no_fallback selected=0 rejected=1 reason=permission_missing',
  'injection bypass=false drift=0 queries=3',
  'repeat stable=true runs=3',
  'cancel ranking_calls=0 err=context.Canceled',
  'equivalence identical=true candidates=3'
]

export function parsePreauthRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s09 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s09 '.length).trim())
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

export function validateOrder(source) {
  const body = String(source ?? '')
  const problems = []
  const positions = ORDER_ANCHORS.map((anchor) => ({ anchor, at: body.indexOf(anchor) }))
  for (const entry of positions) {
    if (entry.at < 0) problems.push(`selector is missing ${entry.anchor}`)
  }
  if (problems.length > 0) return problems
  for (let index = 1; index < positions.length; index++) {
    if (positions[index].at <= positions[index - 1].at) {
      problems.push(`selector order broken: ${positions[index].anchor} must follow ${positions[index - 1].anchor}`)
    }
  }
  const rankingCalls = body.split('s.ranking.Rank(').length - 1
  if (rankingCalls !== 1) problems.push(`selector must call ranking exactly once but calls=${rankingCalls}`)
  return problems
}

export function validateAssertSource(text) {
  const body = String(text ?? '')
  return ASSERT_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `preauth test is missing ${guard}`)
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return PREAUTH_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `preauth test is missing ${name}`)
}

export function validateNoForbiddenCalls(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `preauth surface must not contain ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultTests.includes(PREAUTH_SOURCE)) problems.push(`default build leaked ${PREAUTH_SOURCE}`)
  if (!taggedTests.includes(PREAUTH_SOURCE)) problems.push(`tagged tests missing ${PREAUTH_SOURCE}`)
  const expected = [...defaultTests, PREAUTH_SOURCE].sort().join(',')
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
  const guards = ORDER_ANCHORS.length + ASSERT_GUARDS.length + PREAUTH_CASES.length
  const lines = [
    `invocation=preauth package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0} delta=[${PREAUTH_SOURCE}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m05-s09 ${row}`)
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

export function runPreauthEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability')
  const selectorSource = readFileSync(resolve(base, SELECTOR_SOURCE), 'utf8')
  const testSource = readFileSync(resolve(base, PREAUTH_SOURCE), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    selectorSource,
    testSource,
    rows: parsePreauthRows(log),
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
  const evidence = runPreauthEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateOrder(evidence.selectorSource),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource),
    ...validateNoForbiddenCalls(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_PREAUTH)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
