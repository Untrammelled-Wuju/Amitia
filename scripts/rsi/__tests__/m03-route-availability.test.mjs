import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  AVAILABILITY_CASES,
  CORE_PROVIDER_ID,
  EXIT_ROUTE_AVAILABILITY,
  EXPECTED_PASSED,
  PACKAGE_PATH,
  PLUGIN_PROVIDER_ID,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  parseAvailabilityRows,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateEvidence
} from '../m03-route-availability.mjs'

const LOG = `=== RUN   TestRouteAvailabilityCoreMissingImplementation
route availability core missing service=memory.search provider=builtin.core/core/memory.search class=NO_BUILTIN fallbackBlocked=true candidates=2
=== RUN   TestRouteAvailabilityCoreImplemented
route availability core implemented service=memory.search provider=builtin.core/core/memory.search class=NONE implementations=1
=== RUN   TestRouteAvailabilityPluginHealthy
route availability plugin healthy service=memory.search provider=plugin.host class=NONE probes=1
=== RUN   TestRouteAvailabilityPluginCrashed
route availability plugin crashed service=memory.search provider=plugin.host class=UNAVAILABLE error=ErrRouteUnavailable
=== RUN   TestRouteAvailabilityRefusesFakeFallback
route availability no fake fallback mode=auto bound=builtin.core/core/memory.search selected=builtin.core/core/memory.search pluginCandidates=1 fallbackBlocked=true
=== RUN   TestRouteAvailabilityPluginOnlyCrashStaysUnavailable
route availability plugin only crashed mode=plugin_only provider=plugin.host class=UNAVAILABLE skippedCore=1
=== RUN   TestRouteAvailabilityIgnoresHealthForCore
route availability core health ignored mode=core_only provider=builtin.core/core/memory.search healthProbe=skipped class=NONE probeCalls=0
=== RUN   TestRouteAvailabilityPassesPolicyErrorsThrough
route availability invalid policy field=mode value=hybrid error=ErrRoutePolicyInvalid class=none
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/plugincontract	1.620s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['doc.go', 'route_availability.go', 'route_policy.go'],
    testFiles: ['route_availability_test.go', 'route_policy_test.go'],
    rows: parseAvailabilityRows(LOG),
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the route availability surface', () => {
  assert.equal(EXIT_ROUTE_AVAILABILITY, 38)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/plugincontract')
  assert.equal(RUN_PATTERN, 'TestRouteAvailability')
  assert.equal(EXPECTED_PASSED, 8)
  assert.equal(CORE_PROVIDER_ID, 'builtin.core/core/memory.search')
  assert.equal(PLUGIN_PROVIDER_ID, 'plugin.host')
  assert.deepEqual(REQUIRED_SOURCES, ['route_availability.go', 'route_policy.go'])
  assert.deepEqual(REQUIRED_TESTS, ['route_availability_test.go'])
  assert.deepEqual(AVAILABILITY_CASES, [
    'core-missing',
    'core-implemented',
    'plugin-healthy',
    'plugin-crashed',
    'no-fake-fallback',
    'plugin-only-crash',
    'core-health-ignored',
    'invalid-policy'
  ])
})

test('go list output is parsed into sorted file lists', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles('a.go'), [])
})

test('go test output counts pass and fail lines', () => {
  assert.deepEqual(parseGoTestOutput(LOG), { passed: 0, failed: 0 })
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- FAIL: B\n'), { passed: 1, failed: 1 })
})

test('availability rows expose every field as typed values', () => {
  const rows = parseAvailabilityRows(LOG)
  assert.deepEqual(rows['core-missing'], ['memory.search', CORE_PROVIDER_ID, 'NO_BUILTIN', 'true', 2])
  assert.deepEqual(rows['core-implemented'], ['memory.search', CORE_PROVIDER_ID, 'NONE', 1])
  assert.deepEqual(rows['plugin-healthy'], ['memory.search', PLUGIN_PROVIDER_ID, 'NONE', 1])
  assert.deepEqual(rows['plugin-crashed'], ['memory.search', PLUGIN_PROVIDER_ID, 'UNAVAILABLE', 'ErrRouteUnavailable'])
  assert.deepEqual(rows['no-fake-fallback'], ['auto', CORE_PROVIDER_ID, CORE_PROVIDER_ID, 1, 'true'])
  assert.deepEqual(rows['plugin-only-crash'], ['plugin_only', PLUGIN_PROVIDER_ID, 'UNAVAILABLE', 1])
  assert.deepEqual(rows['core-health-ignored'], ['core_only', CORE_PROVIDER_ID, 'skipped', 'NONE', 0])
  assert.deepEqual(rows['invalid-policy'], ['mode', 'hybrid', 'ErrRoutePolicyInvalid', 'none'])
})

test('missing rows parse to null', () => {
  const rows = parseAvailabilityRows('nothing here')
  for (const name of AVAILABILITY_CASES) assert.equal(rows[name], null)
})

test('clean evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must carry route_availability.go', () => {
  const problems = validateEvidence(evidence({ goFiles: ['doc.go', 'route_policy.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_availability.go')))
})

test('default build must still carry route_policy.go', () => {
  const problems = validateEvidence(evidence({ goFiles: ['doc.go', 'route_availability.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_policy.go')))
})

test('default build must not leak test files', () => {
  const problems = validateEvidence(evidence({ goFiles: ['route_availability.go', 'route_availability_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('leaked test files')))
})

test('test build must carry the availability test file', () => {
  const problems = validateEvidence(evidence({ testFiles: ['route_policy_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_availability_test.go')))
})

test('each evidence row must print exactly once', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}route availability plugin crashed service=memory.search provider=plugin.host class=UNAVAILABLE error=ErrRouteUnavailable\n` }))
  assert.ok(problems.some((problem) => problem.includes('evidence row plugin-crashed must print exactly once')))
})

test('a missing core implementation must classify as NO_BUILTIN', () => {
  const rows = parseAvailabilityRows(LOG)
  const drifted = validateEvidence(evidence({ rows: { ...rows, 'core-missing': ['memory.search', CORE_PROVIDER_ID, 'UNAVAILABLE', 'true', 2] } }))
  assert.ok(drifted.some((problem) => problem.includes('core missing class=UNAVAILABLE')))
  const unblocked = validateEvidence(evidence({ rows: { ...rows, 'core-missing': ['memory.search', CORE_PROVIDER_ID, 'NO_BUILTIN', 'false', 2] } }))
  assert.ok(unblocked.some((problem) => problem.includes('must block the fallback')))
})

test('core implementation presence must clear the failure class', () => {
  const rows = parseAvailabilityRows(LOG)
  const drifted = validateEvidence(evidence({ rows: { ...rows, 'core-implemented': ['memory.search', CORE_PROVIDER_ID, 'NO_BUILTIN', 1] } }))
  assert.ok(drifted.some((problem) => problem.includes('core implemented class=NO_BUILTIN')))
  const empty = validateEvidence(evidence({ rows: { ...rows, 'core-implemented': ['memory.search', CORE_PROVIDER_ID, 'NONE', 0] } }))
  assert.ok(empty.some((problem) => problem.includes('implementations=0')))
})

test('a missing core implementation must never switch to the plugin', () => {
  const rows = parseAvailabilityRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'no-fake-fallback': ['auto', CORE_PROVIDER_ID, PLUGIN_PROVIDER_ID, 1, 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('selected=plugin.host')))
})

test('only a crashed plugin is reported as UNAVAILABLE', () => {
  const rows = parseAvailabilityRows(LOG)
  const crashed = validateEvidence(evidence({ rows: { ...rows, 'plugin-crashed': ['memory.search', PLUGIN_PROVIDER_ID, 'NO_BUILTIN', 'ErrRouteUnavailable'] } }))
  assert.ok(crashed.some((problem) => problem.includes('plugin crashed class=NO_BUILTIN')))
  const sentinel = validateEvidence(evidence({ rows: { ...rows, 'plugin-crashed': ['memory.search', PLUGIN_PROVIDER_ID, 'UNAVAILABLE', 'ErrRouteNoBuiltin'] } }))
  assert.ok(sentinel.some((problem) => problem.includes('plugin crashed error=ErrRouteNoBuiltin')))
})

test('a healthy plugin must be probed exactly once', () => {
  const rows = parseAvailabilityRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'plugin-healthy': ['memory.search', PLUGIN_PROVIDER_ID, 'NONE', 0] } }))
  assert.ok(problems.some((problem) => problem.includes('plugin healthy probes=0')))
})

test('plugin_only crash keeps its own classification', () => {
  const rows = parseAvailabilityRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'plugin-only-crash': ['plugin_only', PLUGIN_PROVIDER_ID, 'NO_BUILTIN', 1] } }))
  assert.ok(problems.some((problem) => problem.includes('plugin only crash class=NO_BUILTIN')))
})

test('a core provider must ignore the plugin health probe', () => {
  const rows = parseAvailabilityRows(LOG)
  const probed = validateEvidence(evidence({ rows: { ...rows, 'core-health-ignored': ['core_only', CORE_PROVIDER_ID, 'skipped', 'NONE', 1] } }))
  assert.ok(probed.some((problem) => problem.includes('probeCalls=1')))
  const down = validateEvidence(evidence({ rows: { ...rows, 'core-health-ignored': ['core_only', CORE_PROVIDER_ID, 'down', 'UNAVAILABLE', 0] } }))
  assert.ok(down.some((problem) => problem.includes('core health ignored probe=down')))
})

test('policy errors are passed through without a failure class', () => {
  const rows = parseAvailabilityRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'invalid-policy': ['mode', 'hybrid', 'ErrRoutePolicyInvalid', 'NO_BUILTIN'] } }))
  assert.ok(problems.some((problem) => problem.includes('invalid policy class=NO_BUILTIN')))
})

test('run metadata must prove the race detector and a clean exit', () => {
  const problems = validateEvidence(evidence({ passed: 7, failed: 1, exitCode: 1, race: false }))
  assert.ok(problems.some((problem) => problem.includes('passed=7')))
  assert.ok(problems.some((problem) => problem.includes('failed=1')))
  assert.ok(problems.some((problem) => problem.includes('exit=1')))
  assert.ok(problems.some((problem) => problem.includes('must run with -race')))
})

test('rendered evidence keeps the eight frozen rows in order', () => {
  const output = renderEvidence(evidence())
  const lines = output.trimEnd().split('\n')
  assert.equal(lines.length, 11)
  assert.match(lines[0], /route=availability package=\.\/internal\/extension\/kernel\/plugincontract/)
  assert.match(lines[1], /^files=3 tests=2 sources=2\/2 testExcluded=true$/)
  assert.match(lines[2], /^route availability core missing /)
  assert.match(lines[9], /^route availability invalid policy /)
  assert.match(lines[10], /^passed=8 failed=0 exit=0 race=true$/)
})

test('rendered evidence survives a total parse failure without inventing values', () => {
  const output = renderEvidence(evidence({ rows: parseAvailabilityRows(''), passed: 0, exitCode: 1, race: false, log: '' }))
  assert.match(output, /route availability core missing service= provider= class= fallbackBlocked=false candidates=0/)
  assert.match(output, /route availability no fake fallback mode= bound= selected= pluginCandidates=0 fallbackBlocked=false/)
})
