import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  CORE_PROVIDER_ID,
  EXIT_ROUTE_POLICY,
  EXPECTED_PASSED,
  PACKAGE_PATH,
  PIN_PRIORITY,
  PLUGIN_PROVIDER_ID,
  POLICY_CASES,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  parseGoListFiles,
  parseGoTestOutput,
  parsePolicyRows,
  renderEvidence,
  validateEvidence
} from '../m03-route-policy.mjs'

const LOG = `=== RUN   TestRoutePolicyAutoKeepsBoundProvider
route policy mode=auto selected=plugin.host bound=plugin.host candidates=2 skipped=0 pinned=false
=== RUN   TestRoutePolicyCoreOnlyExcludesPlugins
route policy mode=core_only selected=builtin.core/core/memory.search candidates=1 skipped=1 reason=mode_excluded
=== RUN   TestRoutePolicyPluginOnlyExcludesCore
route policy mode=plugin_only selected=plugin.host candidates=1 skipped=1 reason=mode_excluded
=== RUN   TestRoutePolicyPinnedCoreBeatsEvolutionCandidate
route policy pin pinned=builtin.core/core/memory.search selected=builtin.core/core/memory.search pluginPriority=90 corePriority=10 overridden=true
=== RUN   TestRoutePolicyPinConflictsWithMode
route policy pin conflict pinned=plugin.host mode=core_only error=ErrRoutePinConflict
=== RUN   TestRoutePolicyCompatibilityGates
route policy compat selected=builtin.core/core/memory.search candidates=1 skipped=2 reasons=contract_mismatch,version_mismatch fallback=true
=== RUN   TestRoutePolicyUnsatisfiedMode
route policy unsatisfied mode=core_only candidates=0 skipped=1 error=ErrRouteModeUnsatisfied
=== RUN   TestRoutePolicyInvalidInput
route policy invalid field=mode value=hybrid error=ErrRoutePolicyInvalid
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/plugincontract	1.600s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['route_policy.go', 'provider_selection.go', 'doc.go'],
    testFiles: ['route_policy_test.go', 'provider_selection_test.go'],
    rows: parsePolicyRows(LOG),
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the route policy surface', () => {
  assert.equal(EXIT_ROUTE_POLICY, 37)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/plugincontract')
  assert.equal(RUN_PATTERN, 'TestRoutePolicy')
  assert.equal(EXPECTED_PASSED, 8)
  assert.equal(PIN_PRIORITY, 90)
  assert.equal(CORE_PROVIDER_ID, 'builtin.core/core/memory.search')
  assert.equal(PLUGIN_PROVIDER_ID, 'plugin.host')
  assert.deepEqual(REQUIRED_SOURCES, ['route_policy.go'])
  assert.deepEqual(REQUIRED_TESTS, ['route_policy_test.go'])
  assert.deepEqual(POLICY_CASES, ['auto', 'core_only', 'plugin_only', 'pin', 'pin-conflict', 'compat', 'unsatisfied', 'invalid'])
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

test('policy rows expose every field as typed values', () => {
  const rows = parsePolicyRows(LOG)
  assert.deepEqual(rows.auto, ['plugin.host', 'plugin.host', 2, 0, 'false'])
  assert.deepEqual(rows.core_only, [CORE_PROVIDER_ID, 1, 1, 'mode_excluded'])
  assert.deepEqual(rows.plugin_only, ['plugin.host', 1, 1, 'mode_excluded'])
  assert.deepEqual(rows.pin, [CORE_PROVIDER_ID, CORE_PROVIDER_ID, 90, 10, 'true'])
  assert.deepEqual(rows['pin-conflict'], ['plugin.host', 'core_only', 'ErrRoutePinConflict'])
  assert.deepEqual(rows.compat, [CORE_PROVIDER_ID, 1, 2, 'contract_mismatch', 'version_mismatch', 'true'])
  assert.deepEqual(rows.unsatisfied, ['core_only', 0, 1, 'ErrRouteModeUnsatisfied'])
  assert.deepEqual(rows.invalid, ['mode', 'hybrid', 'ErrRoutePolicyInvalid'])
})

test('missing rows parse to null', () => {
  const rows = parsePolicyRows('nothing here')
  for (const name of POLICY_CASES) assert.equal(rows[name], null)
})

test('clean evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must carry route_policy.go', () => {
  const problems = validateEvidence(evidence({ goFiles: ['provider_selection.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_policy.go')))
})

test('default build must not leak test files', () => {
  const problems = validateEvidence(evidence({ goFiles: ['route_policy.go', 'route_policy_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('leaked test files')))
})

test('test build must carry the policy test file', () => {
  const problems = validateEvidence(evidence({ testFiles: ['provider_selection_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_policy_test.go')))
})

test('each evidence row must print exactly once', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}route policy invalid field=mode value=hybrid error=ErrRoutePolicyInvalid\n` }))
  assert.ok(problems.some((problem) => problem.includes('evidence row invalid must print exactly once')))
})

test('auto mode must keep the bound provider', () => {
  const rows = parsePolicyRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, auto: ['plugin.new', 'plugin.host', 2, 0, 'false'] } }))
  assert.ok(problems.some((problem) => problem.includes('auto mode replaced the bound provider')))
  const pinned = validateEvidence(evidence({ rows: { ...rows, auto: ['plugin.host', 'plugin.host', 2, 0, 'true'] } }))
  assert.ok(pinned.some((problem) => problem.includes('must not report a pin')))
})

test('core_only must not select a plugin', () => {
  const rows = parsePolicyRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, core_only: ['plugin.host', 1, 1, 'mode_excluded'] } }))
  assert.ok(problems.some((problem) => problem.includes('core_only selected=plugin.host')))
})

test('plugin_only must skip the core provider with a reason', () => {
  const rows = parsePolicyRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, plugin_only: ['plugin.host', 1, 0, 'none'] } }))
  assert.ok(problems.some((problem) => problem.includes('plugin_only skipped=0')))
})

test('a pinned core must beat a higher priority evolution candidate', () => {
  const rows = parsePolicyRows(LOG)
  const drifted = evidence({ rows: { ...rows, pin: [CORE_PROVIDER_ID, PLUGIN_PROVIDER_ID, 90, 10, 'true'] } })
  const problems = validateEvidence(drifted)
  assert.ok(problems.some((problem) => problem.includes('pin pinned=builtin.core/core/memory.search selected=plugin.host')))
  const hidden = evidence({ rows: { ...rows, pin: [CORE_PROVIDER_ID, CORE_PROVIDER_ID, 90, 10, 'false'] } })
  assert.ok(validateEvidence(hidden).some((problem) => problem.includes('overridden')))
})

test('a pin inside a conflicting mode must fail with the pin sentinel', () => {
  const rows = parsePolicyRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'pin-conflict': ['plugin.host', 'core_only', 'ErrRouteModeUnsatisfied'] } }))
  assert.ok(problems.some((problem) => problem.includes('pin conflict error=ErrRouteModeUnsatisfied')))
})

test('compatibility gates must name contract and version mismatches', () => {
  const rows = parsePolicyRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, compat: [CORE_PROVIDER_ID, 1, 2, 'version_mismatch', 'contract_mismatch', 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('compat reasons=version_mismatch,contract_mismatch')))
  const noFallback = validateEvidence(evidence({ rows: { ...rows, compat: [CORE_PROVIDER_ID, 1, 2, 'contract_mismatch', 'version_mismatch', 'false'] } }))
  assert.ok(noFallback.some((problem) => problem.includes('fallback=true')))
})

test('an empty core_only candidate set must be reported, not silently filled', () => {
  const rows = parsePolicyRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, unsatisfied: ['core_only', 1, 1, 'ErrRouteModeUnsatisfied'] } }))
  assert.ok(problems.some((problem) => problem.includes('unsatisfied candidates=1')))
})

test('invalid policy input must carry field and value', () => {
  const rows = parsePolicyRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, invalid: ['requiredVersionRange', '>>1', 'ErrRoutePolicyInvalid'] } }))
  assert.ok(problems.some((problem) => problem.includes('invalid field=requiredVersionRange')))
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
  assert.match(lines[0], /route=policy package=\.\/internal\/extension\/kernel\/plugincontract/)
  assert.match(lines[1], /^files=3 tests=2 sources=1\/1 testExcluded=true$/)
  assert.match(lines[2], /^route policy mode=auto /)
  assert.match(lines[9], /^route policy invalid /)
  assert.match(lines[10], /^passed=8 failed=0 exit=0 race=true$/)
})

test('rendered evidence survives a total parse failure without inventing values', () => {
  const output = renderEvidence(evidence({ rows: parsePolicyRows(''), passed: 0, exitCode: 1, race: false, log: '' }))
  assert.match(output, /route policy mode=auto selected= bound= candidates=0 skipped=0 pinned=false/)
  assert.match(output, /route policy compat selected= candidates=0 skipped=0 reasons=, fallback=false/)
})
