import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  EVIDENCE_ROWS,
  EXIT_METADATA,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  INSTALL_SOURCE,
  INSTALL_TEST,
  METADATA_CASES,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOURCE_GUARDS,
  STATE_TEST,
  parseGoListFiles,
  parseGoTestOutput,
  parseMetadataRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateNoRuntimeWork,
  validateSource
} from '../m06-s02-metadata.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return { defaultTests: ['enablement_test.go'], taggedTests: ['enablement_test.go', INSTALL_TEST, STATE_TEST].sort(), consumers: [] }
}

test('S02 退出码为该步独占', () => {
  assert.equal(EXIT_METADATA, 66)
})

test('S02 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestPluginMetadata')
  for (const taken of ['TestLifecycle', 'TestQuality', 'TestExactRoute', 'TestSchemaCache']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of METADATA_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S02 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
  assert.deepEqual(validateSource(SOURCE_GUARDS.join('\n')), [])
})

test('S02 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 9) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S02 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 9 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S02 元数据路径守卫缺一即报', () => {
  assert.equal(validateSource('package enablement').length, SOURCE_GUARDS.length)
  assert.ok(SOURCE_GUARDS.includes('type RuntimeSpawner interface {'))
  assert.ok(SOURCE_GUARDS.includes('if machine.State() != LifecycleEnabled {'))
  assert.ok(SOURCE_GUARDS.includes('SpawnCalls: i.spawns,'))
})

test('S02 断言守卫钉住冷态与激活边界', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if snapshot.Plugins != 1000 || snapshot.Processes != 0 || snapshot.Workers != 0 || snapshot.SpawnCalls != 0 {'))
  assert.ok(ASSERT_GUARDS.includes('if spawner.Calls() != 1 || installer.Snapshot().Processes != 1 {'))
})

test('S02 十个用例名必须逐个出现', () => {
  const body = METADATA_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package enablement')
  assert.equal(problems.length, METADATA_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('metadata test is missing TestPluginMetadata')))
})

test('S02 安装路径不得包含后台协程或 reconcile', () => {
  const clean = 'func install() {}'
  assert.deepEqual(validateNoRuntimeWork(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoRuntimeWork(clean + token), [`install path must not contain ${token}`])
  }
})

test('S02 构建标签把安装器挡在默认构建外', () => {
  assert.equal(BUILD_TAG, 'rsi_lifecycle')
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
  const leaked = validateIsolation({ ...isolation(), defaultTests: ['enablement_test.go', INSTALL_TEST].sort() })
  assert.ok(leaked.some((p) => p.includes(`leaked ${INSTALL_TEST}`)))
  const missing = validateIsolation({ ...isolation(), taggedTests: ['enablement_test.go'] })
  assert.ok(missing.some((p) => p.includes(`missing ${INSTALL_TEST}`)))
})

test('S02 未接线证明：包外 NewInstaller 调用必须为 0', () => {
  assert.deepEqual(validateIsolation({ ...isolation(), consumers: ['backend/internal/x.go'] }), ['installer is wired before the integration step: backend/internal/x.go'])
})

test('S02 行解析只认 m06-s02 空格前缀', () => {
  const log = [
    '--- PASS: TestPluginMetadataThousandStaysCold',
    '    install_metadata_test.go:1: m06-s02 cold a',
    '    install_metadata_test.go:2: m06-s02-detail noise',
    '    install_metadata_test.go:3: m06-s02 cancel b'
  ].join('\n')
  assert.deepEqual(parseMetadataRows(log), ['cold a', 'cancel b'])
})

test('S02 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S02 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S02 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseMetadataRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=metadata package=' + PACKAGE_PATH))
  assert.ok(text.includes('consumers=0'))
  assert.ok(text.includes('passed=10 failed=0 exit=0 race=true'))
})

test('S02 源码文件名与包路径固定', () => {
  assert.equal(INSTALL_SOURCE, 'install.go')
  assert.equal(INSTALL_TEST, 'install_metadata_test.go')
  assert.equal(STATE_TEST, 'lifecycle_state_test.go')
})
