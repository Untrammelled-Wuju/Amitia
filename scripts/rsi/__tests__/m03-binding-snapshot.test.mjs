import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EXIT_BINDING_SNAPSHOT,
  GOLDEN_PATH,
  RUN_PATTERN,
  SCOPES,
  bindingDigest,
  bindingKeyLine,
  parseGoTestOutput,
  readGolden,
  renderGolden,
  summarizeGolden,
  validateGolden
} from '../m03-binding-snapshot.mjs'
import { repoRoot } from '../toolchain-preflight.mjs'

function binding(capabilityId, scope, options = {}) {
  const spaceId = options.spaceId ?? ''
  const deviceId = options.deviceId ?? ''
  const providerId = options.providerId ?? `${scope}.host`
  return {
    key: { capabilityId, spaceId, deviceId, scope },
    providerId,
    providerInstanceId: `${providerId}:instance`
  }
}

function fixture(overrides = {}) {
  const bindings = [
    binding('memory.search', 'device', { spaceId: 'space-1', deviceId: 'device-1' }),
    binding('memory.search', 'space', { spaceId: 'space-1' }),
    binding('memory.search', 'global'),
    binding('memory.delete', 'space', { spaceId: 'space-1' })
  ]
  return {
    schemaVersion: 1,
    epoch: 'boot-1',
    revision: 3,
    scopeOrder: [...SCOPES],
    bindings,
    digest: bindingDigest(bindings),
    resolutions: [
      {
        name: 'device-first',
        request: { capabilityId: 'memory.search', spaceId: 'space-1', deviceId: 'device-1' },
        scope: 'device',
        providerId: 'device.host',
        trail: ['device=hit']
      },
      {
        name: 'space-fallback',
        request: { capabilityId: 'memory.search', spaceId: 'space-1', deviceId: '' },
        scope: 'space',
        providerId: 'space.host',
        trail: ['device=skip', 'space=hit']
      },
      {
        name: 'global-fallback',
        request: { capabilityId: 'memory.search', spaceId: '', deviceId: '' },
        scope: 'global',
        providerId: 'global.host',
        trail: ['device=skip', 'space=skip', 'global=hit']
      }
    ],
    ...overrides
  }
}

test('constants pin the deliverable surface', () => {
  assert.equal(EXIT_BINDING_SNAPSHOT, 29)
  assert.deepEqual(SCOPES, ['device', 'space', 'global'])
  assert.equal(RUN_PATTERN, 'TestBindingSnapshot')
  assert.ok(GOLDEN_PATH.endsWith('testdata/binding-snapshot.json'))
})

test('valid fixture has no problems', () => {
  assert.deepEqual(validateGolden(fixture()), [])
})

test('scope order is pinned to device then space then global', () => {
  const problems = validateGolden(fixture({ scopeOrder: ['global', 'space', 'device'] }))
  assert.ok(problems.some((problem) => problem.includes('scopeOrder')), problems.join('; '))
})

test('digest must match recomputed canonical lines', () => {
  const problems = validateGolden(fixture({ digest: 'b'.repeat(64) }))
  assert.ok(problems.some((problem) => problem.includes('does not match recomputed')), problems.join('; '))
})

test('digest is order independent', () => {
  const bindings = [binding('memory.search', 'global'), binding('memory.delete', 'space', { spaceId: 'space-1' })]
  const reversed = [...bindings].reverse()
  assert.equal(bindingDigest(bindings), bindingDigest(reversed))
  assert.equal(bindingDigest(bindings).length, 64)
})

test('duplicate binding keys are rejected', () => {
  const duplicated = binding('memory.search', 'global')
  const problems = validateGolden(fixture({ bindings: [duplicated, duplicated], digest: bindingDigest([duplicated, duplicated]) }))
  assert.ok(problems.some((problem) => problem.includes('duplicate key')), problems.join('; '))
})

test('scope specific key shapes are enforced', () => {
  const cases = [
    { name: 'device without deviceId', key: binding('memory.search', 'device', { spaceId: 'space-1' }), want: 'device scope requires' },
    { name: 'space with deviceId', key: binding('memory.search', 'space', { spaceId: 'space-1', deviceId: 'device-1' }), want: 'space scope requires' },
    { name: 'global with spaceId', key: binding('memory.search', 'global', { spaceId: 'space-1' }), want: 'global scope must not' },
    { name: 'unknown scope', key: binding('memory.search', 'cluster'), want: 'is not one of' }
  ]
  for (const testCase of cases) {
    const problems = validateGolden(fixture({ bindings: [testCase.key], digest: bindingDigest([testCase.key]) }))
    assert.ok(problems.some((problem) => problem.includes(testCase.want)), `${testCase.name}: ${problems.join('; ')}`)
  }
})

test('empty identifiers and provider fields are rejected', () => {
  const broken = [
    { key: { capabilityId: '', spaceId: '', deviceId: '', scope: 'global' }, providerId: 'p', providerInstanceId: 'i' },
    { key: { capabilityId: 'memory.search', spaceId: '', deviceId: '', scope: 'global' }, providerId: '', providerInstanceId: 'i' },
    { key: { capabilityId: 'memory.search', spaceId: '', deviceId: '', scope: 'global' }, providerId: 'p', providerInstanceId: '' }
  ]
  for (const candidate of broken) {
    const problems = validateGolden(fixture({ bindings: [candidate], digest: bindingDigest([candidate]) }))
    assert.ok(problems.length > 0)
  }
})

test('trail must end on the resolved scope and hit only once', () => {
  const trailing = fixture()
  trailing.resolutions[0].trail = ['device=miss', 'space=hit']
  assert.ok(validateGolden(trailing).some((problem) => problem.includes('does not end with device=hit')))

  const repeated = fixture()
  repeated.resolutions[2].trail = ['device=hit', 'space=hit', 'global=hit']
  assert.ok(validateGolden(repeated).some((problem) => problem.includes('hits more than once')))

  const malformed = fixture()
  malformed.resolutions[0].trail = ['device=maybe']
  assert.ok(validateGolden(malformed).some((problem) => problem.includes('malformed')))
})

test('all three scopes need at least one resolution', () => {
  const problems = validateGolden(fixture({ resolutions: fixture().resolutions.slice(0, 2) }))
  assert.ok(problems.some((problem) => problem.includes('no resolution covers scope global')), problems.join('; '))
})

test('revision and epoch must be usable', () => {
  assert.ok(validateGolden(fixture({ revision: 0 })).some((problem) => problem.includes('revision=0')))
  assert.ok(validateGolden(fixture({ epoch: '  ' })).some((problem) => problem.includes('epoch is empty')))
})

test('empty golden is rejected field by field', () => {
  const problems = validateGolden({ schemaVersion: 1, epoch: '', revision: 1, scopeOrder: [], bindings: [], digest: 'x', resolutions: [] })
  for (const want of ['epoch is empty', 'bindings is empty', 'resolutions is empty', 'scopeOrder=']) {
    assert.ok(problems.some((problem) => problem.includes(want)), `${want}: ${problems.join('; ')}`)
  }
  assert.ok(validateGolden(null).length > 0)
})

test('summarize and render expose the audited numbers', () => {
  const golden = fixture()
  const summary = summarizeGolden(golden)
  assert.deepEqual(summary.scopeCounts, { device: 1, space: 2, global: 1 })
  assert.equal(summary.bindings, 4)
  assert.equal(summary.resolutions, 3)
  const rendered = renderGolden(golden)
  assert.ok(rendered.includes(golden.digest))
  assert.ok(rendered.includes('device 1 / space 2 / global 1'))
  assert.ok(rendered.includes('device=skip → space=hit'))
})

test('binding key line is stable', () => {
  assert.equal(bindingKeyLine({ capabilityId: 'memory.search', scope: 'global' }), 'memory.search|||global')
  assert.equal(bindingKeyLine({ capabilityId: 'memory.search', spaceId: 'space-1', deviceId: 'device-1', scope: 'device' }), 'memory.search|space-1|device-1|device')
})

test('go test output parsing counts real lines', () => {
  const log = '=== RUN   TestBindingSnapshotScopeOrder\n--- PASS: TestBindingSnapshotScopeOrder (0.00s)\n--- FAIL: TestBindingSnapshotStrictMiss (0.00s)\n'
  assert.deepEqual(parseGoTestOutput(log), { passed: 1, failed: 1 })
  assert.deepEqual(parseGoTestOutput(''), { passed: 0, failed: 0 })
})

test('repository golden satisfies the validator', () => {
  const golden = readGolden(repoRoot)
  assert.deepEqual(validateGolden(golden), [])
  assert.equal(golden.revision >= 1, true)
  assert.equal(golden.scopeOrder.join(','), 'device,space,global')
  const summary = summarizeGolden(golden)
  assert.equal(summary.bindings, 5)
  assert.equal(summary.resolutions, 4)
})
