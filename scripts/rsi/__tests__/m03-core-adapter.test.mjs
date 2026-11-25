import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BUILD_TAG,
  EXIT_CORE_ADAPTER,
  EXPECTED_CASES,
  EXPECTED_PASSED,
  PACKAGE_PATH,
  PROVIDER_ID,
  RUN_PATTERN,
  parseEquivalence,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateEvidence
} from '../m03-core-adapter.mjs'

const PASS_LOG = `=== RUN   TestMemorySearchAdapterMatchesCoreOverOneHundredInputs
    memory_search_adapter_test.go:386: equivalence cases=100 provider=builtin.core/core/memory.search tag=rsi_coreadapter
--- PASS: TestMemorySearchAdapterMatchesCoreOverOneHundredInputs (0.42s)
=== RUN   TestMemorySearchAdapterKeepsOriginalErrorChain
--- PASS: TestMemorySearchAdapterKeepsOriginalErrorChain (0.01s)
=== RUN   TestMemorySearchAdapterRejectsUnsupportedBinding
--- PASS: TestMemorySearchAdapterRejectsUnsupportedBinding (0.00s)
=== RUN   TestMemorySearchAdapterHealth
--- PASS: TestMemorySearchAdapterHealth (0.00s)
=== RUN   TestMemorySearchAdapterRejectsInvalidInput
--- PASS: TestMemorySearchAdapterRejectsInvalidInput (0.00s)
=== RUN   TestMemorySearchAdapterInvocationMapping
--- PASS: TestMemorySearchAdapterInvocationMapping (0.00s)
=== RUN   TestMemorySearchAdapterLeavesNoMutations
--- PASS: TestMemorySearchAdapterLeavesNoMutations (0.03s)
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/coreadapter	0.51s
`

function evidence(overrides = {}) {
  return {
    defaultFiles: ['doc.go'],
    taggedFiles: ['doc.go', 'memory_search_adapter.go'],
    taggedTests: ['memory_search_adapter_test.go'],
    equivalence: { cases: 100, provider: PROVIDER_ID, tag: BUILD_TAG },
    passed: 7,
    failed: 0,
    exitCode: 0,
    ...overrides
  }
}

test('constants pin the deliverable surface', () => {
  assert.equal(EXIT_CORE_ADAPTER, 31)
  assert.equal(BUILD_TAG, 'rsi_coreadapter')
  assert.equal(EXPECTED_CASES, 100)
  assert.equal(EXPECTED_PASSED, 7)
  assert.equal(PROVIDER_ID, 'builtin.core/core/memory.search')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/coreadapter')
  assert.ok(RUN_PATTERN.includes('TestMemorySearchAdapter'))
})

test('go test log counts passing and failing cases', () => {
  const counts = parseGoTestOutput(PASS_LOG)
  assert.equal(counts.passed, 7)
  assert.equal(counts.failed, 0)
  assert.equal(parseGoTestOutput(`${PASS_LOG}--- FAIL: TestMemorySearchAdapterHealth (0.00s)\n`).failed, 1)
})

test('equivalence summary is parsed from the test log', () => {
  assert.deepEqual(parseEquivalence(PASS_LOG), { cases: 100, provider: PROVIDER_ID, tag: BUILD_TAG })
  assert.equal(parseEquivalence('no summary here'), null)
})

test('go list output is parsed into sorted file names', () => {
  assert.deepEqual(parseGoListFiles('[doc.go]'), ['doc.go'])
  assert.deepEqual(parseGoListFiles('[memory_search_adapter.go doc.go]'), ['doc.go', 'memory_search_adapter.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('doc.go'), [])
})

test('complete evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must expose doc.go only', () => {
  const problems = validateEvidence(evidence({ defaultFiles: ['doc.go', 'memory_search_adapter.go'] }))
  assert.ok(problems.some((problem) => problem.includes('default build files')), problems.join('; '))
})

test('tagged build must include the adapter', () => {
  const problems = validateEvidence(evidence({ taggedFiles: ['doc.go'] }))
  assert.ok(problems.some((problem) => problem.includes('memory_search_adapter.go')), problems.join('; '))
})

test('tagged build must keep doc.go so the package stays non-empty', () => {
  const problems = validateEvidence(evidence({ taggedFiles: ['memory_search_adapter.go'] }))
  assert.ok(problems.some((problem) => problem.includes('dropped doc.go')), problems.join('; '))
})

test('tagged tests must include the equivalence suite', () => {
  const problems = validateEvidence(evidence({ taggedTests: [] }))
  assert.ok(problems.some((problem) => problem.includes('memory_search_adapter_test.go')), problems.join('; '))
})

test('equivalence summary must cover exactly one hundred inputs', () => {
  const problems = validateEvidence(evidence({ equivalence: { cases: 98, provider: PROVIDER_ID, tag: BUILD_TAG } }))
  assert.ok(problems.some((problem) => problem.includes('equivalence cases=98')), problems.join('; '))
})

test('equivalence summary must prove the provider identity', () => {
  const problems = validateEvidence(evidence({ equivalence: { cases: 100, provider: 'builtin.core/other', tag: BUILD_TAG } }))
  assert.ok(problems.some((problem) => problem.includes('equivalence provider=')), problems.join('; '))
})

test('equivalence summary must prove the build tag', () => {
  const problems = validateEvidence(evidence({ equivalence: { cases: 100, provider: PROVIDER_ID, tag: 'default' } }))
  assert.ok(problems.some((problem) => problem.includes('equivalence tag=default')), problems.join('; '))
})

test('missing equivalence summary is rejected', () => {
  const problems = validateEvidence(evidence({ equivalence: null }))
  assert.ok(problems.some((problem) => problem.includes('equivalence summary is missing')), problems.join('; '))
})

test('failed or short test runs are rejected', () => {
  assert.ok(validateEvidence(evidence({ passed: 6 })).some((problem) => problem.includes('passed=6')))
  assert.ok(validateEvidence(evidence({ failed: 1 })).some((problem) => problem.includes('failed=1')))
  assert.ok(validateEvidence(evidence({ exitCode: 1 })).some((problem) => problem.includes('exit=1')))
})

test('rendered evidence reports isolation and equivalence counts', () => {
  const text = renderEvidence(evidence())
  assert.ok(text.includes('isolation: default=[doc.go] tagged=[doc.go memory_search_adapter.go] tests=[memory_search_adapter_test.go]'))
  assert.ok(text.includes('equivalence cases=100 passed=7 failed=0 exit=0'))
  assert.ok(text.includes(`tag=${BUILD_TAG}`))
})
