import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EXIT_PROVIDER_SELECTION,
  GOLDEN_PATH,
  RUN_PATTERN,
  SCOPES,
  parseGoTestOutput,
  readGolden,
  renderGolden,
  summarizeGolden,
  validateGolden
} from '../m03-provider-selection.mjs'
import { repoRoot } from '../toolchain-preflight.mjs'

function fixture(overrides = {}) {
  return {
    schemaVersion: 1,
    serviceId: 'memory.search',
    revision: 4,
    scope: 'global',
    candidates: [
      { providerId: 'provider.a', priority: 10 },
      { providerId: 'provider.b', priority: 5 }
    ],
    selected: 'provider.b',
    trail: ['device=skip', 'space=skip', 'global=hit'],
    ...overrides
  }
}

test('constants pin the deliverable surface', () => {
  assert.equal(EXIT_PROVIDER_SELECTION, 30)
  assert.deepEqual(SCOPES, ['device', 'space', 'global'])
  assert.ok(RUN_PATTERN.includes('TestSelectProviderCandidates'))
  assert.ok(GOLDEN_PATH.endsWith('testdata/provider-selection.json'))
})

test('valid fixture has no problems', () => {
  assert.deepEqual(validateGolden(fixture()), [])
})

test('candidate order must be priority descending', () => {
  const problems = validateGolden(fixture({ candidates: [{ providerId: 'provider.b', priority: 5 }, { providerId: 'provider.a', priority: 10 }] }))
  assert.ok(problems.some((problem) => problem.includes('outranks previous')), problems.join('; '))
})

test('tie break must be provider id ascending', () => {
  const problems = validateGolden(
    fixture({
      candidates: [
        { providerId: 'provider.c', priority: 5 },
        { providerId: 'provider.b', priority: 5 }
      ]
    })
  )
  assert.ok(problems.some((problem) => problem.includes('tie break')), problems.join('; '))
})

test('selected provider must be among candidates', () => {
  const problems = validateGolden(fixture({ selected: 'provider.ghost' }))
  assert.ok(problems.some((problem) => problem.includes('is not among candidates')), problems.join('; '))
})

test('duplicate candidates are rejected', () => {
  const duplicated = [
    { providerId: 'provider.a', priority: 10 },
    { providerId: 'provider.a', priority: 10 }
  ]
  const problems = validateGolden(fixture({ candidates: duplicated, selected: 'provider.a' }))
  assert.ok(problems.some((problem) => problem.includes('duplicate provider')), problems.join('; '))
})

test('scope and revision must be usable', () => {
  assert.ok(validateGolden(fixture({ scope: 'cluster' })).some((problem) => problem.includes('scope=cluster')))
  assert.ok(validateGolden(fixture({ revision: 0 })).some((problem) => problem.includes('revision=0')))
  assert.ok(validateGolden(fixture({ serviceId: ' ' })).some((problem) => problem.includes('serviceId is empty')))
})

test('trail must end on the resolved scope', () => {
  const wrongTail = validateGolden(fixture({ trail: ['device=hit'] }))
  assert.ok(wrongTail.some((problem) => problem.includes('does not end with global=hit')))
  const repeated = validateGolden(fixture({ trail: ['device=hit', 'global=hit'] }))
  assert.ok(repeated.some((problem) => problem.includes('hits more than once')))
  const malformed = validateGolden(fixture({ trail: ['global=maybe'] }))
  assert.ok(malformed.some((problem) => problem.includes('malformed')))
})

test('empty candidates or trail are rejected', () => {
  const problems = validateGolden(fixture({ candidates: [], selected: '', trail: [] }))
  for (const want of ['candidates is empty', 'selected is empty', 'trail is empty']) {
    assert.ok(problems.some((problem) => problem.includes(want)), `${want}: ${problems.join('; ')}`)
  }
  assert.ok(validateGolden(null).length > 0)
})

test('non integer priority is rejected', () => {
  const problems = validateGolden(fixture({ candidates: [{ providerId: 'provider.a', priority: 'high' }], selected: 'provider.a' }))
  assert.ok(problems.some((problem) => problem.includes('priority is not an integer')), problems.join('; '))
})

test('summarize and render expose the selection', () => {
  const golden = fixture()
  const summary = summarizeGolden(golden)
  assert.deepEqual(summary, { candidates: 2, selected: 'provider.b', topPriority: 10, scope: 'global' })
  const rendered = renderGolden(golden)
  assert.ok(rendered.includes('候选 2 / 命中 provider.b'))
  assert.ok(rendered.includes('device=skip → space=skip → global=hit'))
  assert.ok(rendered.includes('命中'))
  assert.ok(rendered.includes('备选'))
})

test('go test output parsing counts real lines', () => {
  const log = '--- PASS: TestSelectProviderCandidatesPrefersBoundProvider (0.00s)\n--- FAIL: TestProviderSelectionMatchesGolden (0.01s)\n'
  assert.deepEqual(parseGoTestOutput(log), { passed: 1, failed: 1 })
  assert.deepEqual(parseGoTestOutput(''), { passed: 0, failed: 0 })
})

test('repository golden satisfies the validator', () => {
  const golden = readGolden(repoRoot)
  assert.deepEqual(validateGolden(golden), [])
  assert.equal(golden.serviceId, 'memory.search')
  assert.equal(golden.selected, 'provider.b')
  assert.equal(golden.candidates.length, 2)
  assert.equal(golden.candidates[0].providerId, 'provider.a')
})
