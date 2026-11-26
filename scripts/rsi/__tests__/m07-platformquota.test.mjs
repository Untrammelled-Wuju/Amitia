import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAG,
  CASES,
  EVIDENCE_ROWS,
  EXIT_PLATFORMQUOTA,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  QUOTA_FILE,
  QUOTA_TEST_FILE,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateMobileGate,
  validateSource
} from '../m07-s12-platformquota.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: 12, failed: 0, exitCode: 0, race: true, mobileDevice: false }
}

function isolation() {
  return {
    defaultFiles: ['queue.go', 'service.go'],
    taggedFiles: ['queue.go', 'service.go', QUOTA_FILE].sort(),
    defaultTests: ['queue_test.go'],
    taggedTests: ['queue_test.go', QUOTA_TEST_FILE].sort()
  }
}

test('S12 退出码为该步独占', () => assert.equal(EXIT_PLATFORMQUOTA, 84))

test('S12 用例前缀与标签固定', () => {
  assert.equal(RUN_PATTERN, 'TestPlatformQuota')
  assert.equal(BUILD_TAG, 'rsi_platformquota')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/task_runtime')
  for (const name of CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S12 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateMobileGate(rowSet()), [])
})

test('S12 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 11) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S12 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 11 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S12 关闭态：能力协商不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['queue.go', QUOTA_FILE].sort() })
  assert.ok(leaked.some((p) => p.includes(`default build leaked ${QUOTA_FILE}`)))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['queue.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged build missing ${QUOTA_FILE}`)))
  const leakedTest = validateIsolation({ ...isolation(), defaultTests: ['queue_test.go', QUOTA_TEST_FILE].sort() })
  assert.ok(leakedTest.some((p) => p.includes(`default tests leaked ${QUOTA_TEST_FILE}`)))
})

test('S12 真机未接入时拒绝声称设备证据', () => {
  assert.ok(validateMobileGate({ ...rowSet(), mobileDevice: true }).some((p) => p.includes('e2e path not implemented')))
})

test('S12 源码守卫缺一即报', () => {
  assert.deepEqual(validateSource(process.cwd()), [])
  assert.equal(SOURCE_GUARDS.length, 8)
})

test('S12 十二个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  assert.equal(validateCaseNames('package task_runtime').length, CASES.length)
})

test('S12 行解析只认 m07-s12 空格前缀', () => {
  const log = [
    '--- PASS: TestPlatformQuotaAndroidFull',
    '    platform_quota_test.go:1: m07-s12 android a',
    '    platform_quota_test.go:2: m07-s12-detail noise',
    '    platform_quota_test.go:3: m07-s12 ios b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['android a', 'ios b'])
})

test('S12 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n=== RUN x\n--- FAIL: C\n'), { passed: 1, failed: 1 })
})

test('S12 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S12 渲染结果包含三平台、设备 BLOCKED 与 14 行', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=3 default_tests=1 tagged_tests=2'))
  assert.ok(text.includes('m07-s12 platforms declared=3 android,ios,desktop'))
  assert.ok(text.includes('m07-s12 mobile_e2e status=BLOCKED reason=no_device_in_environment'))
  assert.ok(text.includes('passed=12 failed=0 exit=0 race=true'))
  assert.equal(parseRows(text).length, 14)
})

test('S12 期望通过数固定', () => {
  assert.equal(EXPECTED_PASSED, 12)
  assert.equal(EXPECTED_PASSED, CASES.length)
  assert.ok(GO_TEST_FLAGS.includes('-race'))
})
