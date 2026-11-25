import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BUILD_TAG,
  EXIT_PLUGIN_ADAPTER,
  EXPECTED_GUARDS,
  EXPECTED_PASSED,
  EXPECTED_REJECTED,
  PACKAGE_PATH,
  parseGuardSummary,
  parseGoListFiles,
  parseGoTestOutput,
  renderEvidence,
  validateEvidence
} from '../m03-plugin-adapter.mjs'

const PASS_LOG = `=== RUN   TestPluginAdapterGuardsPluginProvider
=== RUN   TestPluginAdapterGuardsPluginProvider/allow-global
--- PASS: TestPluginAdapterGuardsPluginProvider/allow-global (0.00s)
--- PASS: TestPluginAdapterGuardsPluginProvider/reject-contract-drift (0.00s)
    plugin_adapter_test.go:248: plugin adapter guards provider=com.amitia/emote/emote-runtime/com.amitia/emote/ai cases=13 rejected=10
--- PASS: TestPluginAdapterGuardsPluginProvider (0.00s)
--- PASS: TestPluginAdapterKeepsOriginalErrorChain (0.00s)
--- PASS: TestPluginAdapterRejectsForeignBinding (0.00s)
--- PASS: TestPluginAdapterLeavesCoreStorageUntouched (0.26s)
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel/pluginadapter	0.85s
`

function evidence(overrides = {}) {
  return {
    defaultFiles: ['doc.go'],
    taggedFiles: ['doc.go', 'plugin_adapter.go'],
    taggedTests: ['plugin_adapter_test.go'],
    guards: { provider: 'com.amitia/emote/emote-runtime/com.amitia/emote/ai', cases: 13, rejected: 10 },
    passed: 17,
    failed: 0,
    exitCode: 0,
    ...overrides
  }
}

test('constants pin the plugin adapter surface', () => {
  assert.equal(EXIT_PLUGIN_ADAPTER, 32)
  assert.equal(BUILD_TAG, 'rsi_pluginadapter')
  assert.equal(EXPECTED_GUARDS, 13)
  assert.equal(EXPECTED_REJECTED, 10)
  assert.equal(EXPECTED_PASSED, 17)
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/pluginadapter')
})

test('go test log counts passing and failing cases', () => {
  const counts = parseGoTestOutput(PASS_LOG)
  assert.equal(counts.passed, 6)
  assert.equal(counts.failed, 0)
  assert.equal(parseGoTestOutput(`${PASS_LOG}--- FAIL: TestPluginAdapterGuardsPluginProvider (0.00s)\n`).failed, 1)
})

test('guard summary is parsed from the test log', () => {
  assert.deepEqual(parseGuardSummary(PASS_LOG), {
    provider: 'com.amitia/emote/emote-runtime/com.amitia/emote/ai',
    cases: 13,
    rejected: 10
  })
  assert.equal(parseGuardSummary('no summary'), null)
})

test('go list output is parsed into sorted file names', () => {
  assert.deepEqual(parseGoListFiles('[doc.go]'), ['doc.go'])
  assert.deepEqual(parseGoListFiles('[plugin_adapter.go doc.go]'), ['doc.go', 'plugin_adapter.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('doc.go'), [])
})

test('complete evidence has no problems', () => {
  assert.deepEqual(validateEvidence(evidence()), [])
})

test('default build must expose doc.go only', () => {
  const problems = validateEvidence(evidence({ defaultFiles: ['doc.go', 'plugin_adapter.go'] }))
  assert.ok(problems.some((problem) => problem.includes('default build files')), problems.join('; '))
})

test('tagged build must include the plugin adapter', () => {
  const problems = validateEvidence(evidence({ taggedFiles: ['doc.go'] }))
  assert.ok(problems.some((problem) => problem.includes('plugin_adapter.go')), problems.join('; '))
})

test('tagged build must keep doc.go so the package stays non-empty', () => {
  const problems = validateEvidence(evidence({ taggedFiles: ['plugin_adapter.go'] }))
  assert.ok(problems.some((problem) => problem.includes('dropped doc.go')), problems.join('; '))
})

test('tagged tests must include the guard suite', () => {
  const problems = validateEvidence(evidence({ taggedTests: [] }))
  assert.ok(problems.some((problem) => problem.includes('plugin_adapter_test.go')), problems.join('; '))
})

test('guard summary must be present with a provider', () => {
  const missing = validateEvidence(evidence({ guards: null }))
  assert.ok(missing.some((problem) => problem.includes('guard summary is missing')), missing.join('; '))
  const blank = validateEvidence(evidence({ guards: { provider: '', cases: 13, rejected: 10 } }))
  assert.ok(blank.some((problem) => problem.includes('provider is empty')), blank.join('; '))
})

test('guard summary counts must match the matrix', () => {
  const wrongCases = validateEvidence(evidence({ guards: { provider: 'p', cases: 12, rejected: 10 } }))
  assert.ok(wrongCases.some((problem) => problem.includes('guard cases=12')), wrongCases.join('; '))
  const wrongRejected = validateEvidence(evidence({ guards: { provider: 'p', cases: 13, rejected: 9 } }))
  assert.ok(wrongRejected.some((problem) => problem.includes('guard rejected=9')), wrongRejected.join('; '))
})

test('failed or short test runs are rejected', () => {
  assert.ok(validateEvidence(evidence({ passed: 16 })).some((problem) => problem.includes('passed=16')))
  assert.ok(validateEvidence(evidence({ failed: 1 })).some((problem) => problem.includes('failed=1')))
  assert.ok(validateEvidence(evidence({ exitCode: 1 })).some((problem) => problem.includes('exit=1')))
})

test('rendered evidence reports isolation and guard counts', () => {
  const text = renderEvidence(evidence())
  assert.ok(text.includes('isolation: default=[doc.go] tagged=[doc.go plugin_adapter.go] tests=[plugin_adapter_test.go]'))
  assert.ok(text.includes('guards cases=13 rejected=10 passed=17 failed=0 exit=0'))
  assert.ok(text.includes(`tag=${BUILD_TAG}`))
})
