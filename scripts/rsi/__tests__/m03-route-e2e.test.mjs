import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BUILD_TAGS,
  CORE_PROVIDER_ID,
  EXIT_ROUTE_E2E,
  EXPECTED_CASES,
  EXPECTED_PASSED,
  PACKAGE_PATH,
  PLUGIN_PROVIDER_ID,
  parseCaseSummaries,
  parseGoListFiles,
  parseGoTestOutput,
  parseSummary,
  renderEvidence,
  validateEvidence
} from '../m03-route-e2e.mjs'

const LOG = `=== RUN   TestRouteE2EIsolatedRoute
=== RUN   TestRouteE2EIsolatedRoute/plugin-receipt
    route_e2e_test.go:430: route e2e case=plugin-receipt provider=com.amitia/emote/ai hops=core>plugin>core fallback=false callbacks=1 pid=101
--- PASS: TestRouteE2EIsolatedRoute/plugin-receipt (0.02s)
=== RUN   TestRouteE2EIsolatedRoute/crash-fallback
    route_e2e_test.go:490: route e2e case=crash-fallback provider=builtin.core/core/memory.search hops=core fallback=true attempts=2 commits=0 error="plugincontract: external adapter transport closed: EOF"
--- PASS: TestRouteE2EIsolatedRoute/crash-fallback (0.02s)
=== RUN   TestRouteE2EIsolatedRoute/denied-write
    route_e2e_test.go:518: route e2e case=denied-write provider=builtin.core/core/memory.search hops=core fallback=true deniedWrites=1 commits=0 error="plugincontract: adapter rpc 403: routee2e: authoritative writes are not permitted on this route"
--- PASS: TestRouteE2EIsolatedRoute/denied-write (0.02s)
=== RUN   TestRouteE2EIsolatedRoute/cross-scope
    route_e2e_test.go:543: route e2e case=cross-scope provider=builtin.core/core/memory.search hops=core fallback=true deniedScopes=1 commits=0
--- PASS: TestRouteE2EIsolatedRoute/cross-scope (0.02s)
    route_e2e_test.go:553: route e2e summary cases=4 pluginReceipts=1 fallbacks=3 denials=2 commits=0 digest=unchanged pid=100
--- PASS: TestRouteE2EIsolatedRoute (0.34s)
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/routee2e	0.945s
`

function evidence(overrides = {}) {
  return {
    defaultFiles: ['doc.go'],
    taggedFiles: ['doc.go', 'route_e2e.go'],
    taggedTests: ['helper_process_test.go', 'route_e2e_test.go'],
    cases: parseCaseSummaries(LOG),
    summary: parseSummary(LOG),
    passed: 5,
    failed: 0,
    exitCode: 0,
    ...overrides
  }
}

test('constants pin the route e2e surface', () => {
  assert.equal(EXIT_ROUTE_E2E, 34)
  assert.deepEqual(BUILD_TAGS, ['rsi_coreadapter', 'rsi_routee2e'])
  assert.equal(EXPECTED_CASES, 4)
  assert.equal(EXPECTED_PASSED, 5)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/routee2e')
  assert.equal(PLUGIN_PROVIDER_ID, 'com.amitia/emote/ai')
  assert.equal(CORE_PROVIDER_ID, 'builtin.core/core/memory.search')
})

test('go test log counts pass and fail lines', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 5, failed: 0 })
  assert.equal(parseGoTestOutput(`${LOG}--- FAIL: TestRouteE2EIsolatedRoute (0.10s)\n`).failed, 1)
})

test('case summaries expose provider, hops, fallback and counters', () => {
  const cases = parseCaseSummaries(LOG)
  assert.equal(cases.length, 4)
  assert.deepEqual(cases.map((entry) => entry.name), ['plugin-receipt', 'crash-fallback', 'denied-write', 'cross-scope'])
  assert.equal(cases[0].provider, PLUGIN_PROVIDER_ID)
  assert.equal(cases[0].hops, 'core>plugin>core')
  assert.equal(cases[0].fallback, false)
  assert.equal(cases[0].callbacks, 1)
  assert.equal(cases[0].pid, 101)
  assert.equal(cases[1].attempts, 2)
  assert.match(cases[1].error, /transport closed/)
  assert.equal(cases[2].deniedWrites, 1)
  assert.match(cases[2].error, /authoritative writes are not permitted/)
  assert.equal(cases[3].deniedScopes, 1)
  assert.deepEqual(parseCaseSummaries('no cases'), [])
})

test('summary line is parsed', () => {
  assert.deepEqual(parseSummary(LOG), {
    cases: 4,
    pluginReceipts: 1,
    fallbacks: 3,
    denials: 2,
    commits: 0,
    digest: 'unchanged',
    pid: 100
  })
  assert.equal(parseSummary('nothing'), null)
})

test('go list output is parsed into sorted file names', () => {
  assert.deepEqual(parseGoListFiles('[doc.go]'), ['doc.go'])
  assert.deepEqual(parseGoListFiles('[route_e2e.go doc.go]'), ['doc.go', 'route_e2e.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('doc.go'), [])
})

test('complete evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must expose doc.go only', () => {
  const problems = validateEvidence(evidence({ defaultFiles: ['doc.go', 'route_e2e.go'] }))
  assert.ok(problems.some((problem) => problem.includes('default build files')), problems.join('; '))
})

test('tagged build must include the route harness and keep doc.go', () => {
  assert.ok(validateEvidence(evidence({ taggedFiles: ['doc.go'] })).some((problem) => problem.includes('route_e2e.go')))
  assert.ok(validateEvidence(evidence({ taggedFiles: ['route_e2e.go'] })).some((problem) => problem.includes('dropped doc.go')))
})

test('tagged tests must include both the harness and the helper process', () => {
  const missingHarness = validateEvidence(evidence({ taggedTests: ['helper_process_test.go'] }))
  assert.ok(missingHarness.some((problem) => problem.includes('route_e2e_test.go')))
  const missingHelper = validateEvidence(evidence({ taggedTests: ['route_e2e_test.go'] }))
  assert.ok(missingHelper.some((problem) => problem.includes('helper_process_test.go')))
})

test('case order is pinned', () => {
  const cases = parseCaseSummaries(LOG)
  const problems = validateEvidence(evidence({ cases: [cases[1], cases[0], cases[2], cases[3]] }))
  assert.ok(problems.some((problem) => problem.includes('cases=[')), problems.join('; '))
})

test('plugin receipt must name the plugin provider with three hops and one callback', () => {
  const cases = parseCaseSummaries(LOG)
  const wrongProvider = cases.map((entry) => ({ ...entry }))
  wrongProvider[0].provider = 'provider.other'
  assert.ok(validateEvidence(evidence({ cases: wrongProvider })).some((problem) => problem.includes('plugin-receipt provider=')))
  const wrongHops = cases.map((entry) => ({ ...entry }))
  wrongHops[0].hops = 'core>plugin'
  assert.ok(validateEvidence(evidence({ cases: wrongHops })).some((problem) => problem.includes('plugin-receipt hops=')))
  const fallbackTrue = cases.map((entry) => ({ ...entry }))
  fallbackTrue[0].fallback = true
  assert.ok(validateEvidence(evidence({ cases: fallbackTrue })).some((problem) => problem.includes('must not report fallback')))
  const noCallbacks = cases.map((entry) => ({ ...entry }))
  noCallbacks[0].callbacks = 0
  assert.ok(validateEvidence(evidence({ cases: noCallbacks })).some((problem) => problem.includes('callbacks=0')))
})

test('fallback cases must land on core with a single hop and zero commits', () => {
  const cases = parseCaseSummaries(LOG)
  const wrongProvider = cases.map((entry) => ({ ...entry }))
  wrongProvider[1].provider = PLUGIN_PROVIDER_ID
  assert.ok(validateEvidence(evidence({ cases: wrongProvider })).some((problem) => problem.includes('crash-fallback provider=')))
  const noFallback = cases.map((entry) => ({ ...entry }))
  noFallback[2].fallback = false
  assert.ok(validateEvidence(evidence({ cases: noFallback })).some((problem) => problem.includes('denied-write must report fallback')))
  const committed = cases.map((entry) => ({ ...entry }))
  committed[3].commits = 1
  assert.ok(validateEvidence(evidence({ cases: committed })).some((problem) => problem.includes('cross-scope commits=1')))
})

test('original error chains must survive the fallback', () => {
  const cases = parseCaseSummaries(LOG)
  const crashChain = cases.map((entry) => ({ ...entry }))
  crashChain[1].error = 'plugin crashed'
  assert.ok(validateEvidence(evidence({ cases: crashChain })).some((problem) => problem.includes('want original transport error')))
  const deniedChain = cases.map((entry) => ({ ...entry }))
  deniedChain[2].error = 'denied'
  assert.ok(validateEvidence(evidence({ cases: deniedChain })).some((problem) => problem.includes('want the core refusal')))
  const wrongAttempts = cases.map((entry) => ({ ...entry }))
  wrongAttempts[1].attempts = 1
  assert.ok(validateEvidence(evidence({ cases: wrongAttempts })).some((problem) => problem.includes('attempts=1')))
})

test('denied writes and cross-scope reads are counted exactly once', () => {
  const cases = parseCaseSummaries(LOG)
  const writes = cases.map((entry) => ({ ...entry }))
  writes[2].deniedWrites = 0
  assert.ok(validateEvidence(evidence({ cases: writes })).some((problem) => problem.includes('deniedWrites=0')))
  const scopes = cases.map((entry) => ({ ...entry }))
  scopes[3].deniedScopes = 2
  assert.ok(validateEvidence(evidence({ cases: scopes })).some((problem) => problem.includes('deniedScopes=2')))
})

test('summary counters and digest are pinned', () => {
  const summary = parseSummary(LOG)
  for (const [key, value] of [
    ['cases', 3],
    ['pluginReceipts', 0],
    ['fallbacks', 2],
    ['denials', 1],
    ['commits', 1],
    ['digest', 'changed']
  ]) {
    const problems = validateEvidence(evidence({ summary: { ...summary, [key]: value } }))
    assert.ok(problems.some((problem) => problem.includes(key)), `${key}: ${problems.join('; ')}`)
  }
})

test('plugin must not share the core process', () => {
  const summary = parseSummary(LOG)
  const problems = validateEvidence(evidence({ summary: { ...summary, pid: 101 } }))
  assert.ok(problems.some((problem) => problem.includes('not crossing a process boundary')), problems.join('; '))
})

test('short or failing runs are rejected', () => {
  assert.ok(validateEvidence(evidence({ passed: 4 })).some((problem) => problem.includes('passed=4')))
  assert.ok(validateEvidence(evidence({ failed: 1 })).some((problem) => problem.includes('failed=1')))
  assert.ok(validateEvidence(evidence({ exitCode: 1 })).some((problem) => problem.includes('exit=1')))
})

test('rendered evidence reports isolation, cases and process boundary', () => {
  const text = renderEvidence(evidence())
  assert.ok(text.includes(`route=e2e tags=${BUILD_TAGS.join(',')} package=${PACKAGE_PATH}`))
  assert.ok(text.includes('isolation: default=[doc.go] tagged=[doc.go route_e2e.go] tests=[helper_process_test.go route_e2e_test.go]'))
  assert.ok(text.includes('case=plugin-receipt provider=com.amitia/emote/ai hops=core>plugin>core fallback=false commits=0'))
  assert.ok(text.includes('summary cases=4 pluginReceipts=1 fallbacks=3 denials=2 commits=0 digest=unchanged'))
  assert.ok(text.includes('processes corePid=100 pluginPid=101'))
  assert.ok(text.includes('passed=5 failed=0 exit=0'))
})
