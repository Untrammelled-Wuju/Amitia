import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BUILD_TAG,
  EXIT_ROUTE_CACHE,
  EXPECTED_CARRIED,
  EXPECTED_COMPUTATIONS,
  EXPECTED_CONCURRENCY,
  EXPECTED_GRAPH_LOADS,
  EXPECTED_INVALIDATED,
  EXPECTED_LOOKUPS,
  EXPECTED_MIXED,
  EXPECTED_PASSED,
  EXPECTED_REALSCALE,
  EXPECTED_TIERS,
  PACKAGE_PATH,
  parseConcurrencySummary,
  parseGoListFiles,
  parseGoTestOutput,
  parseInvalidationSummary,
  parseLookupSummary,
  parseRealScaleSummaries,
  parseTierSummaries,
  renderEvidence,
  validateEvidence
} from '../m03-route-cache.mjs'

const PASS_LOG = `=== RUN   TestRouteCacheParsesGraphOnceForThousandQueries
    route_cache_test.go:88: route cache evidence lookups=1000 graphLoads=1 computations=1 hits=999 elapsed=1.531083ms
--- PASS: TestRouteCacheParsesGraphOnceForThousandQueries (0.01s)
=== RUN   TestRouteCacheKeepsRevisionForInFlightRequests
    route_cache_test.go:141: route cache evidence concurrent=100 revisions=2 mixed=0
--- PASS: TestRouteCacheKeepsRevisionForInFlightRequests (0.03s)
=== RUN   TestRouteCacheInvalidatesOnlyAffectedService
    route_cache_test.go:198: route cache evidence invalidated=1 carried=1 recomputed=3
--- PASS: TestRouteCacheInvalidatesOnlyAffectedService (0.00s)
--- PASS: TestRouteCacheReloadsGraphPerRevision (0.00s)
--- PASS: TestRouteCacheRejectsInvalidInputs (0.00s)
--- PASS: TestRouteCacheReturnsDefensiveCopies (0.00s)
--- PASS: TestRouteCacheConcurrentLookupsAgree (0.02s)
=== RUN   TestRouteCacheRealScaleProviderGraph
    route_cache_test.go:493: route cache realscale providers=56 graphLoads=1 lookups=1000 elapsed=4.21075ms
    route_cache_test.go:493: route cache realscale providers=57 graphLoads=1 lookups=1000 elapsed=4.513791ms
--- PASS: TestRouteCacheRealScaleProviderGraph (0.01s)
=== RUN   TestRouteCachePluginTierScaling
    route_cache_test.go:352: route cache tier plugins=0 providers=0 graphLoads=1 empty=true
    route_cache_test.go:377: route cache tier plugins=1 providers=1 graphLoads=1 firstLookupUs=1 queryUs=78
    route_cache_test.go:377: route cache tier plugins=3 providers=3 graphLoads=1 firstLookupUs=2 queryUs=99
    route_cache_test.go:377: route cache tier plugins=5 providers=5 graphLoads=1 firstLookupUs=3 queryUs=136
    route_cache_test.go:377: route cache tier plugins=10 providers=10 graphLoads=1 firstLookupUs=6 queryUs=186
--- PASS: TestRouteCacheRealScaleProviderGraph (0.01s)
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/routecache	0.545s
`

function tiers() {
  return [
    { plugins: 0, providers: 0, graphLoads: 1 },
    { plugins: 1, providers: 1, graphLoads: 1 },
    { plugins: 3, providers: 3, graphLoads: 1 },
    { plugins: 5, providers: 5, graphLoads: 1 },
    { plugins: 10, providers: 10, graphLoads: 1 }
  ]
}

function realscale() {
  return [
    { providers: 56, graphLoads: 1, lookups: 1000 },
    { providers: 57, graphLoads: 1, lookups: 1000 }
  ]
}

function evidence(overrides = {}) {
  return {
    defaultFiles: ['doc.go'],
    taggedFiles: ['doc.go', 'route_cache.go'],
    taggedTests: ['route_cache_test.go'],
    lookups: { lookups: 1000, graphLoads: 1, computations: 1 },
    concurrency: { concurrent: 100, revisions: 2, mixed: 0 },
    invalidation: { invalidated: 1, carried: 1, recomputed: 3 },
    tiers: tiers(),
    realscale: realscale(),
    passed: 9,
    failed: 0,
    exitCode: 0,
    ...overrides
  }
}

test('constants pin the route cache surface', () => {
  assert.equal(EXIT_ROUTE_CACHE, 33)
  assert.equal(BUILD_TAG, 'rsi_routecache')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/routecache')
  assert.equal(EXPECTED_LOOKUPS, 1000)
  assert.equal(EXPECTED_GRAPH_LOADS, 1)
  assert.equal(EXPECTED_COMPUTATIONS, 1)
  assert.equal(EXPECTED_CONCURRENCY, 100)
  assert.equal(EXPECTED_MIXED, 0)
  assert.equal(EXPECTED_INVALIDATED, 1)
  assert.equal(EXPECTED_CARRIED, 1)
  assert.equal(EXPECTED_PASSED, 9)
  assert.deepEqual(EXPECTED_REALSCALE, [56, 57])
  assert.deepEqual(EXPECTED_TIERS, [0, 1, 3, 5, 10])
})

test('go test log counts passing and failing cases', () => {
  const counts = parseGoTestOutput(PASS_LOG)
  assert.equal(counts.passed, 9)
  assert.equal(counts.failed, 0)
  assert.equal(parseGoTestOutput(`${PASS_LOG}--- FAIL: TestRouteCacheReloadsGraphPerRevision (0.00s)\n`).failed, 1)
})

test('lookup summary reports one graph load for a thousand queries', () => {
  assert.deepEqual(parseLookupSummary(PASS_LOG), { lookups: 1000, graphLoads: 1, computations: 1 })
  assert.equal(parseLookupSummary('no summary'), null)
})

test('concurrency summary reports zero revision bleed', () => {
  assert.deepEqual(parseConcurrencySummary(PASS_LOG), { concurrent: 100, revisions: 2, mixed: 0 })
  assert.equal(parseConcurrencySummary('no summary'), null)
})

test('invalidation summary reports the selective sweep', () => {
  assert.deepEqual(parseInvalidationSummary(PASS_LOG), { invalidated: 1, carried: 1, recomputed: 3 })
  assert.equal(parseInvalidationSummary('no summary'), null)
})

test('tier summaries are parsed in plugin order', () => {
  assert.deepEqual(parseTierSummaries(PASS_LOG), tiers())
  assert.deepEqual(parseTierSummaries('no summary'), [])
})

test('real scale summaries are parsed in provider order', () => {
  assert.deepEqual(parseRealScaleSummaries(PASS_LOG), realscale())
  assert.deepEqual(parseRealScaleSummaries('no summary'), [])
})

test('go list output is parsed into sorted file names', () => {
  assert.deepEqual(parseGoListFiles('[doc.go]'), ['doc.go'])
  assert.deepEqual(parseGoListFiles('[route_cache.go doc.go]'), ['doc.go', 'route_cache.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('doc.go'), [])
})

test('complete evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must expose doc.go only', () => {
  const problems = validateEvidence(evidence({ defaultFiles: ['doc.go', 'route_cache.go'] }))
  assert.ok(problems.some((problem) => problem.includes('default build files')), problems.join('; '))
})

test('tagged build must include the cache implementation', () => {
  const problems = validateEvidence(evidence({ taggedFiles: ['doc.go'] }))
  assert.ok(problems.some((problem) => problem.includes('route_cache.go')), problems.join('; '))
})

test('tagged build must keep doc.go so the package stays non-empty', () => {
  const problems = validateEvidence(evidence({ taggedFiles: ['route_cache.go'] }))
  assert.ok(problems.some((problem) => problem.includes('dropped doc.go')), problems.join('; '))
})

test('tagged tests must include the cache suite', () => {
  const problems = validateEvidence(evidence({ taggedTests: [] }))
  assert.ok(problems.some((problem) => problem.includes('route_cache_test.go')), problems.join('; '))
})

test('lookup summary must be present and exact', () => {
  const missing = validateEvidence(evidence({ lookups: null }))
  assert.ok(missing.some((problem) => problem.includes('lookup summary is missing')), missing.join('; '))
  const leaked = validateEvidence(evidence({ lookups: { lookups: 1000, graphLoads: 2, computations: 2 } }))
  assert.ok(leaked.some((problem) => problem.includes('graphLoads=2')), leaked.join('; '))
  assert.ok(leaked.some((problem) => problem.includes('computations=2')), leaked.join('; '))
  assert.ok(validateEvidence(evidence({ lookups: { lookups: 999, graphLoads: 1, computations: 1 } })).some((problem) => problem.includes('lookups=999')))
})

test('concurrency summary must be present and must not bleed revisions', () => {
  const missing = validateEvidence(evidence({ concurrency: null }))
  assert.ok(missing.some((problem) => problem.includes('concurrency summary is missing')), missing.join('; '))
  const single = validateEvidence(evidence({ concurrency: { concurrent: 100, revisions: 1, mixed: 0 } }))
  assert.ok(single.some((problem) => problem.includes('revisions=1')), single.join('; '))
  const mixed = validateEvidence(evidence({ concurrency: { concurrent: 100, revisions: 2, mixed: 3 } }))
  assert.ok(mixed.some((problem) => problem.includes('mixed=3')), mixed.join('; '))
  const shrunk = validateEvidence(evidence({ concurrency: { concurrent: 10, revisions: 2, mixed: 0 } }))
  assert.ok(shrunk.some((problem) => problem.includes('concurrent=10')), shrunk.join('; '))
})

test('invalidation summary must be present and must stay selective', () => {
  const missing = validateEvidence(evidence({ invalidation: null }))
  assert.ok(missing.some((problem) => problem.includes('invalidation summary is missing')), missing.join('; '))
  const swept = validateEvidence(evidence({ invalidation: { invalidated: 2, carried: 0, recomputed: 3 } }))
  assert.ok(swept.some((problem) => problem.includes('invalidated=2')), swept.join('; '))
  assert.ok(swept.some((problem) => problem.includes('carried=0')), swept.join('; '))
  assert.ok(validateEvidence(evidence({ invalidation: { invalidated: 1, carried: 0, recomputed: 3 } })).some((problem) => problem.includes('carried=0')))
})

test('tier evidence must cover every expected plugin count', () => {
  const missing = validateEvidence(evidence({ tiers: tiers().slice(0, 3) }))
  assert.ok(missing.some((problem) => problem.includes('tiers=[0,1,3]')), missing.join('; '))
  const order = validateEvidence(evidence({ tiers: [tiers()[1], tiers()[0], ...tiers().slice(2)] }))
  assert.ok(order.some((problem) => problem.includes('tiers=[1,0,3,5,10]')), order.join('; '))
})

test('a tier that reloads the graph is rejected', () => {
  const broken = tiers()
  broken[3] = { plugins: 5, providers: 5, graphLoads: 2 }
  const problems = validateEvidence(evidence({ tiers: broken }))
  assert.ok(problems.some((problem) => problem.includes('tier plugins=5 graphLoads=2')), problems.join('; '))
})

test('a tier whose provider count drifts is rejected', () => {
  const broken = tiers()
  broken[4] = { plugins: 10, providers: 9, graphLoads: 1 }
  const problems = validateEvidence(evidence({ tiers: broken }))
  assert.ok(problems.some((problem) => problem.includes('providers=9')), problems.join('; '))
})

test('real scale evidence must cover both real provider counts', () => {
  const missing = validateEvidence(evidence({ realscale: [realscale()[0]] }))
  assert.ok(missing.some((problem) => problem.includes('realscale=[56]')), missing.join('; '))
  const reloaded = validateEvidence(evidence({ realscale: [realscale()[0], { providers: 57, graphLoads: 2, lookups: 1000 }] }))
  assert.ok(reloaded.some((problem) => problem.includes('realscale providers=57 graphLoads=2')), reloaded.join('; '))
  const short = validateEvidence(evidence({ realscale: [realscale()[0], { providers: 57, graphLoads: 1, lookups: 999 }] }))
  assert.ok(short.some((problem) => problem.includes('realscale providers=57 lookups=999')), short.join('; '))
})

test('failed or short test runs are rejected', () => {
  assert.ok(validateEvidence(evidence({ passed: 8 })).some((problem) => problem.includes('passed=8')))
  assert.ok(validateEvidence(evidence({ failed: 1 })).some((problem) => problem.includes('failed=1')))
  assert.ok(validateEvidence(evidence({ exitCode: 1 })).some((problem) => problem.includes('exit=1')))
})

test('rendered evidence reports isolation, counts and tiers', () => {
  const text = renderEvidence(evidence())
  assert.ok(text.includes('isolation: default=[doc.go] tagged=[doc.go route_cache.go] tests=[route_cache_test.go]'))
  assert.ok(text.includes('lookups=1000 graphLoads=1 computations=1'))
  assert.ok(text.includes('concurrent=100 revisions=2 mixed=0'))
  assert.ok(text.includes('invalidated=1 carried=1 recomputed=3'))
  assert.ok(text.includes('tiers=[0,1,3,5,10] tierGraphLoads=[1,1,1,1,1]'))
  assert.ok(text.includes('realscale=[56,57] realscaleGraphLoads=[1,1]'))
  assert.ok(text.includes('passed=9 failed=0 exit=0'))
  assert.ok(text.includes(`tag=${BUILD_TAG}`))
})
