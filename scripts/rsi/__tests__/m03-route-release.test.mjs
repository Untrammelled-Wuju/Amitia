import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BUILD_TAG,
  EXIT_ROUTE_RELEASE,
  EXPECTED_CONCURRENCY,
  EXPECTED_CONTRACT,
  EXPECTED_PASSED,
  EXPECTED_PERMISSION,
  EXPECTED_SELECTION,
  MAX_ALLOCS_PER_OP,
  PACKAGE_PATH,
  ROUTING_EVIDENCE,
  ROUTING_PACKAGES,
  parseConcurrency,
  parseContract,
  parseGoListFiles,
  parseGoTestOutput,
  parsePermission,
  parseSelection,
  parseSnapshot,
  renderEvidence,
  validateEvidence
} from '../m03-route-release.mjs'

const LOG = `=== RUN   TestContractFreeze
route conformance contract bindingFields=3 keyFields=4 requestFields=3 scopes=3 methods=7 golden=ok
--- PASS: TestContractFreeze (0.00s)
=== RUN   TestPermissionMatrix
route conformance permission rows=10 allowed=6 denied=4 sentinels=ok
--- PASS: TestPermissionMatrix (0.00s)
=== RUN   TestSelectionOrderingAndBudget
route conformance selection providers=64 candidates=64 selections=5000 elapsedMs=888 allocsPerOp=86 ordered=ok deterministic=ok
--- PASS: TestSelectionOrderingAndBudget (0.07s)
=== RUN   TestConcurrentSelection
route conformance concurrency goroutines=128 iterations=200 mismatches=0
--- PASS: TestConcurrentSelection (0.30s)
=== RUN   TestSnapshotIsolation
route conformance isolation bindings=1 digest=unchanged mutate=blocked
--- PASS: TestSnapshotIsolation (0.00s)
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/routeconformance	1.820s
`

function packages(overrides = {}) {
  return ROUTING_PACKAGES.map((record) => ({
    name: record.name,
    entry: record.entry,
    defaultFiles: ['doc.go'],
    defaultTests: [],
    taggedFiles: record.name === 'routeconformance' ? ['contract.go', 'doc.go', 'permission.go', 'selection.go'] : ['doc.go', record.entry],
    taggedTests: record.name === 'routeconformance' ? ['route_conformance_test.go'] : [],
    ...(overrides[record.name] ?? {})
  }))
}

function evidence(overrides = {}) {
  return {
    packages: packages(),
    contract: parseContract(LOG),
    permission: parsePermission(LOG),
    selection: parseSelection(LOG),
    concurrency: parseConcurrency(LOG),
    snapshot: parseSnapshot(LOG),
    evidenceFiles: ROUTING_EVIDENCE.map((name) => ({ name, present: true, size: 512 })),
    race: true,
    passed: 5,
    failed: 0,
    exitCode: 0,
    ...overrides
  }
}

test('constants pin the release surface', () => {
  assert.equal(EXIT_ROUTE_RELEASE, 35)
  assert.equal(BUILD_TAG, 'rsi_routeconformance')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/routeconformance')
  assert.equal(ROUTING_PACKAGES.length, 5)
  assert.equal(EXPECTED_PASSED, 5)
  assert.deepEqual(EXPECTED_CONTRACT, { bindingFields: 3, keyFields: 4, requestFields: 3, scopes: 3, methods: 7 })
  assert.deepEqual(EXPECTED_PERMISSION, { rows: 10, allowed: 6, denied: 4 })
  assert.deepEqual(EXPECTED_SELECTION, { providers: 64, candidates: 64, selections: 5000 })
  assert.deepEqual(EXPECTED_CONCURRENCY, { goroutines: 128, iterations: 200 })
  assert.equal(MAX_ALLOCS_PER_OP, 200)
  assert.deepEqual(ROUTING_EVIDENCE, [
    'm03-s06-route-cache.log',
    'm03-s06-kernel-suite.log',
    'm03-s07-route-e2e.log',
    'm03-s07-kernel-suite.log'
  ])
})

test('go test log counts pass and fail lines', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 5, failed: 0 })
  assert.equal(parseGoTestOutput(`${LOG}--- FAIL: TestContractFreeze (0.10s)\n`).failed, 1)
})

test('contract line exposes frozen field counts', () => {
  assert.deepEqual(parseContract(LOG), {
    bindingFields: 3,
    keyFields: 4,
    requestFields: 3,
    scopes: 3,
    methods: 7,
    golden: 'ok'
  })
  assert.equal(parseContract('no evidence here'), null)
})

test('permission line exposes rows and sentinels', () => {
  assert.deepEqual(parsePermission(LOG), { rows: 10, allowed: 6, denied: 4, sentinels: 'ok' })
  assert.equal(parsePermission('permission rows=8'), null)
})

test('selection line exposes budget counters', () => {
  const selection = parseSelection(LOG)
  assert.equal(selection.providers, 64)
  assert.equal(selection.candidates, 64)
  assert.equal(selection.selections, 5000)
  assert.equal(selection.elapsedMs, 888)
  assert.equal(selection.allocsPerOp, 86)
  assert.equal(selection.ordered, 'ok')
  assert.equal(selection.deterministic, 'ok')
})

test('concurrency and snapshot lines are parsed', () => {
  assert.deepEqual(parseConcurrency(LOG), { goroutines: 128, iterations: 200, mismatches: 0 })
  assert.deepEqual(parseSnapshot(LOG), { bindings: 1, digest: 'unchanged', mutate: 'blocked' })
})

test('go list parser handles empty and populated sets', () => {
  assert.deepEqual(parseGoListFiles('[doc.go]'), ['doc.go'])
  assert.deepEqual(parseGoListFiles('[doc.go route_e2e.go]'), ['doc.go', 'route_e2e.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('happy path has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('untagged routing packages must build doc.go only', () => {
  const problems = validateEvidence(evidence({ packages: packages({ routecache: { defaultFiles: ['doc.go', 'route_cache.go'] } }) }))
  assert.ok(problems.some((problem) => problem.includes('routecache default build files=doc.go,route_cache.go')))
})

test('untagged routing packages must not collect tests', () => {
  const problems = validateEvidence(evidence({ packages: packages({ routee2e: { defaultTests: ['route_e2e_test.go'] } }) }))
  assert.ok(problems.some((problem) => problem.includes('routee2e default test files=route_e2e_test.go')))
})

test('tagged builds must carry their entry file', () => {
  const problems = validateEvidence(evidence({ packages: packages({ routeconformance: { taggedFiles: ['doc.go'] } }) }))
  assert.ok(problems.some((problem) => problem.includes('routeconformance tagged build is missing contract.go')))
})

test('conformance tagged tests must include the suite', () => {
  const problems = validateEvidence(evidence({ packages: packages({ routeconformance: { taggedTests: [] } }) }))
  assert.ok(problems.some((problem) => problem.includes('routeconformance tagged tests are missing route_conformance_test.go')))
})

test('contract drift is reported per field', () => {
  const drifted = { ...parseContract(LOG), methods: 6 }
  const problems = validateEvidence(evidence({ contract: drifted }))
  assert.ok(problems.some((problem) => problem.includes('contract methods=6 want 7')))
})

test('contract golden json must stay byte-identical', () => {
  const drifted = { ...parseContract(LOG), golden: 'drifted' }
  const problems = validateEvidence(evidence({ contract: drifted }))
  assert.ok(problems.some((problem) => problem.includes('contract golden=drifted want ok')))
})

test('missing contract or permission lines are reported', () => {
  const problems = validateEvidence(evidence({ contract: null, permission: null }))
  assert.ok(problems.some((problem) => problem.includes('contract line is missing')))
  assert.ok(problems.some((problem) => problem.includes('permission line is missing')))
})

test('permission matrix must keep its allow and deny counts', () => {
  const drifted = { ...parsePermission(LOG), denied: 3 }
  const problems = validateEvidence(evidence({ permission: drifted }))
  assert.ok(problems.some((problem) => problem.includes('permission denied=3 want 4')))
})

test('selection budget rejects allocation drift', () => {
  const drifted = { ...parseSelection(LOG), allocsPerOp: MAX_ALLOCS_PER_OP + 1 }
  const problems = validateEvidence(evidence({ selection: drifted }))
  assert.ok(problems.some((problem) => problem.includes('selection allocsPerOp=201')))
})

test('selection budget rejects slow runs', () => {
  const drifted = { ...parseSelection(LOG), elapsedMs: 5001 }
  const problems = validateEvidence(evidence({ selection: drifted }))
  assert.ok(problems.some((problem) => problem.includes('selection elapsedMs=5001')))
})

test('concurrency mismatches are fatal', () => {
  const drifted = { ...parseConcurrency(LOG), mismatches: 2 }
  const problems = validateEvidence(evidence({ concurrency: drifted }))
  assert.ok(problems.some((problem) => problem.includes('concurrency mismatches=2')))
})

test('snapshot mutation through Bindings must be blocked', () => {
  const drifted = { ...parseSnapshot(LOG), mutate: 'leaked' }
  const problems = validateEvidence(evidence({ snapshot: drifted }))
  assert.ok(problems.some((problem) => problem.includes('snapshot mutate=leaked')))
})

test('routing evidence files must be present and non-empty', () => {
  const missing = ROUTING_EVIDENCE.map((name) => ({ name, present: name !== 'm03-s07-route-e2e.log', size: 512 }))
  const problems = validateEvidence(evidence({ evidenceFiles: missing }))
  assert.ok(problems.some((problem) => problem.includes('routing evidence m03-s07-route-e2e.log is missing or empty')))
})

test('race detector and counts must be reported', () => {
  const problems = validateEvidence(evidence({ race: false, passed: 4, failed: 1, exitCode: 35 }))
  assert.ok(problems.some((problem) => problem.includes('race detector was not enabled')))
  assert.ok(problems.some((problem) => problem.includes('passed=4 want 5')))
  assert.ok(problems.some((problem) => problem.includes('failed=1 want 0')))
  assert.ok(problems.some((problem) => problem.includes('go test exit=35 want 0')))
})

test('render evidence keeps the release summary readable', () => {
  const text = renderEvidence(evidence())
  assert.match(text, /route=release tag=rsi_routeconformance/)
  assert.match(text, /defaultDocOnly=true/)
  assert.match(text, /contract bindingFields=3 keyFields=4 requestFields=3 scopes=3 methods=7 golden=ok/)
  assert.match(text, /permission rows=10 allowed=6 denied=4 sentinels=ok/)
  assert.match(text, /selection providers=64 candidates=64 selections=5000 elapsedMs=888 allocsPerOp=86 ordered=ok deterministic=ok/)
  assert.match(text, /concurrency goroutines=128 iterations=200 mismatches=0/)
  assert.match(text, /snapshot-isolation bindings=1 digest=unchanged mutate=blocked/)
  assert.match(text, /missing=0/)
  assert.match(text, /race enabled=true/)
  assert.match(text, /passed=5 failed=0 exit=0/)
})
