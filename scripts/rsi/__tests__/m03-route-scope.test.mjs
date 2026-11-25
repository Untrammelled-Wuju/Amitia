import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EXPECTED_PASSED,
  EXIT_ROUTE_SCOPE,
  LOCAL_PROVIDER_ID,
  PACKAGE_PATH,
  REMOTE_PROVIDER_ID,
  REQUIRED_SOURCES,
  REQUIRED_TESTS,
  RUN_PATTERN,
  SCOPE_CASES,
  SCOPE_DIMENSIONS,
  parseGoListFiles,
  parseGoTestOutput,
  parseScopeRows,
  renderEvidence,
  validateEvidence
} from '../m03-route-scope.mjs'

const LOG = `=== RUN   TestScopeDeviceBinding
route scope device provider=local.device device=device-a matched=true
=== RUN   TestScopeSpaceBinding
route scope space provider=space.service space=space-1 matched=true
=== RUN   TestScopeRoleAndSurfaceBinding
route scope role surface provider=role.butler role=butler surface=desktop matched=true
=== RUN   TestScopeRemoteOfflineNeverRebindsLocal
route scope remote offline requirement=device:device-7 selected= error=ErrScopeProviderUnavailable candidates=1 localRejected=true
=== RUN   TestScopeRemoteOnlineSelectsRemote
route scope remote online requirement=device:device-7 selected=remote.7 matched=true
=== RUN   TestScopeCoreCannotServeDevice
route scope core cannot serve device provider=builtin.core/core/memory.search device=device-7 matched=false reason=core_cannot_serve_device
=== RUN   TestScopeMissingClaimDoesNotMatch
route scope missing claim provider=device.only dimension=role matched=false reason=missing_claim error=ErrScopeNoMatch
=== RUN   TestScopeContractInvalidDimensions
route scope invalid field=claims.dimension value=platform error=ErrScopeContractInvalid coreDeviceConflict=true
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/plugincontract	1.600s
`

function evidence(overrides = {}) {
  return {
    goFiles: ['route_scope.go', 'route_policy.go', 'doc.go'],
    testFiles: ['route_scope_test.go', 'route_policy_test.go'],
    rows: parseScopeRows(LOG),
    passed: EXPECTED_PASSED,
    failed: 0,
    exitCode: 0,
    race: true,
    log: LOG,
    ...overrides
  }
}

test('constants pin the scope contract surface', () => {
  assert.equal(EXIT_ROUTE_SCOPE, 40)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/plugincontract')
  assert.equal(RUN_PATTERN, 'TestScope')
  assert.equal(EXPECTED_PASSED, 8)
  assert.equal(REMOTE_PROVIDER_ID, 'remote.7')
  assert.equal(LOCAL_PROVIDER_ID, 'local.device')
  assert.deepEqual(REQUIRED_SOURCES, ['route_scope.go'])
  assert.deepEqual(REQUIRED_TESTS, ['route_scope_test.go'])
  assert.deepEqual(SCOPE_DIMENSIONS, ['device', 'core', 'space', 'role', 'surface'])
  assert.deepEqual(SCOPE_CASES, ['device', 'space', 'role-surface', 'remote-offline', 'remote-online', 'core-device', 'missing-claim', 'invalid'])
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

test('scope rows expose every field as typed values', () => {
  const rows = parseScopeRows(LOG)
  assert.deepEqual(rows.device, ['local.device', 'device-a', 'true'])
  assert.deepEqual(rows.space, ['space.service', 'space-1', 'true'])
  assert.deepEqual(rows['role-surface'], ['role.butler', 'butler', 'desktop', 'true'])
  assert.deepEqual(rows['remote-offline'], ['device-7', '', 'ErrScopeProviderUnavailable', 1, 'true'])
  assert.deepEqual(rows['remote-online'], ['device-7', REMOTE_PROVIDER_ID, 'true'])
  assert.deepEqual(rows['core-device'], ['builtin.core/core/memory.search', 'device-7', 'false', 'core_cannot_serve_device'])
  assert.deepEqual(rows['missing-claim'], ['device.only', 'role', 'false', 'missing_claim', 'ErrScopeNoMatch'])
  assert.deepEqual(rows.invalid, ['claims.dimension', 'platform', 'ErrScopeContractInvalid', 'true'])
})

test('missing rows parse to null', () => {
  const rows = parseScopeRows('nothing here')
  for (const name of SCOPE_CASES) assert.equal(rows[name], null)
})

test('clean evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must carry route_scope.go', () => {
  const problems = validateEvidence(evidence({ goFiles: ['route_policy.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_scope.go')))
})

test('default build must not leak test files', () => {
  const problems = validateEvidence(evidence({ goFiles: ['route_scope.go', 'route_scope_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('leaked test files')))
})

test('test build must carry the scope test file', () => {
  const problems = validateEvidence(evidence({ testFiles: ['route_policy_test.go'] }))
  assert.ok(problems.some((problem) => problem.includes('missing route_scope_test.go')))
})

test('each evidence row must print exactly once', () => {
  const problems = validateEvidence(evidence({ log: `${LOG}route scope invalid field=claims.dimension value=platform error=ErrScopeContractInvalid coreDeviceConflict=true\n` }))
  assert.ok(problems.some((problem) => problem.includes('evidence row invalid must print exactly once')))
})

test('a device claim must carry its own device id', () => {
  const rows = parseScopeRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, device: ['local.device', 'device-b', 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('device claim=device-b')))
  const unmatched = validateEvidence(evidence({ rows: { ...rows, device: ['local.device', 'device-a', 'false'] } }))
  assert.ok(unmatched.some((problem) => problem.includes('a device claim must match')))
})

test('a space claim must be refused outside its own space', () => {
  const rows = parseScopeRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, space: ['space.service', 'space-1', 'false'] } }))
  assert.ok(problems.some((problem) => problem.includes('a space claim must match')))
})

test('role and surface must both be declared to match', () => {
  const rows = parseScopeRows(LOG)
  const drifted = validateEvidence(evidence({ rows: { ...rows, 'role-surface': ['role.butler', 'butler', 'mobile', 'true'] } }))
  assert.ok(drifted.some((problem) => problem.includes('surface claim=mobile')))
  const hidden = validateEvidence(evidence({ rows: { ...rows, 'role-surface': ['role.butler', 'butler', 'desktop', 'false'] } }))
  assert.ok(hidden.some((problem) => problem.includes('role and surface claims must both match')))
})

test('an offline remote must never select any provider', () => {
  const rows = parseScopeRows(LOG)
  const rebound = validateEvidence(evidence({ rows: { ...rows, 'remote-offline': ['device-7', LOCAL_PROVIDER_ID, 'ErrScopeProviderUnavailable', 1, 'true'] } }))
  assert.ok(rebound.some((problem) => problem.includes(`offline selected=${LOCAL_PROVIDER_ID}`)))
  const quiet = validateEvidence(evidence({ rows: { ...rows, 'remote-offline': ['device-7', '', 'ErrScopeProviderUnavailable', 1, 'false'] } }))
  assert.ok(quiet.some((problem) => problem.includes('never rebind the same-named local capability')))
})

test('an offline remote must fail with the unavailable sentinel', () => {
  const rows = parseScopeRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'remote-offline': ['device-7', '', 'ErrScopeNoMatch', 1, 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('offline error=ErrScopeNoMatch')))
  const twoCandidates = validateEvidence(evidence({ rows: { ...rows, 'remote-offline': ['device-7', '', 'ErrScopeProviderUnavailable', 2, 'true'] } }))
  assert.ok(twoCandidates.some((problem) => problem.includes('offline candidates=2')))
})

test('only the remote may be selected while it is online', () => {
  const rows = parseScopeRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'remote-online': ['device-7', LOCAL_PROVIDER_ID, 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes(`online selected=${LOCAL_PROVIDER_ID}`)))
  const unmatched = validateEvidence(evidence({ rows: { ...rows, 'remote-online': ['device-7', REMOTE_PROVIDER_ID, 'false'] } }))
  assert.ok(unmatched.some((problem) => problem.includes('must be selected by scope')))
})

test('a core provider must never answer a device requirement', () => {
  const rows = parseScopeRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'core-device': ['builtin.core/core/memory.search', 'device-7', 'true', 'core_cannot_serve_device'] } }))
  assert.ok(problems.some((problem) => problem.includes('must not claim to serve a remote device')))
  const wrongReason = validateEvidence(evidence({ rows: { ...rows, 'core-device': ['builtin.core/core/memory.search', 'device-7', 'false', 'missing_claim'] } }))
  assert.ok(wrongReason.some((problem) => problem.includes('core device reason=missing_claim')))
})

test('an undeclared requirement dimension must not match', () => {
  const rows = parseScopeRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, 'missing-claim': ['device.only', 'role', 'true', 'missing_claim', 'ErrScopeNoMatch'] } }))
  assert.ok(problems.some((problem) => problem.includes('must not match')))
  const wrongError = validateEvidence(evidence({ rows: { ...rows, 'missing-claim': ['device.only', 'role', 'false', 'missing_claim', 'ErrScopeProviderUnavailable'] } }))
  assert.ok(wrongError.some((problem) => problem.includes('missing claim error=ErrScopeProviderUnavailable')))
})

test('unknown dimensions and core+device claims must be rejected', () => {
  const rows = parseScopeRows(LOG)
  const problems = validateEvidence(evidence({ rows: { ...rows, invalid: ['claims.dimension', 'platform', 'ErrScopeProviderUnavailable', 'true'] } }))
  assert.ok(problems.some((problem) => problem.includes('invalid error=ErrScopeProviderUnavailable')))
  const conflict = validateEvidence(evidence({ rows: { ...rows, invalid: ['claims.dimension', 'platform', 'ErrScopeContractInvalid', 'false'] } }))
  assert.ok(conflict.some((problem) => problem.includes('claiming both core and device')))
  const field = validateEvidence(evidence({ rows: { ...rows, invalid: ['requirement.dimension', 'platform', 'ErrScopeContractInvalid', 'true'] } }))
  assert.ok(field.some((problem) => problem.includes('invalid field=requirement.dimension')))
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
  assert.match(lines[0], /route=scope package=\.\/internal\/extension\/kernel\/plugincontract run=TestScope dimensions=device,core,space,role,surface cases=8/)
  assert.match(lines[1], /^files=3 tests=2 sources=1\/1 testExcluded=true$/)
  assert.match(lines[2], /^route scope device /)
  assert.match(lines[5], /^route scope remote offline requirement=device:device-7 selected= error=ErrScopeProviderUnavailable candidates=1 localRejected=true$/)
  assert.match(lines[9], /^route scope invalid /)
  assert.match(lines[10], /^passed=8 failed=0 exit=0 race=true$/)
})

test('rendered evidence survives a total parse failure without inventing values', () => {
  const output = renderEvidence(evidence({ rows: parseScopeRows(''), passed: 0, exitCode: 1, race: false, log: '' }))
  assert.match(output, /route scope device provider= device= matched=false/)
  assert.match(output, /route scope remote offline requirement=device: selected= error= candidates=0 localRejected=false/)
  assert.match(output, /route scope invalid field= value= error= coreDeviceConflict=false/)
})
