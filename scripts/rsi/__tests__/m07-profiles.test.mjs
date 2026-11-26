import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAGS,
  CASES,
  EVIDENCE_ROWS,
  EXIT_PROFILES,
  EXPECTED_PASSED,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  PROFILE_FILES,
  RUN_PATTERN,
  SOURCE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseRows,
  renderEvidence,
  validateAbsoluteBound,
  validateCaseNames,
  validateEvidence,
  validateIsolation,
  validateSource
} from '../m07-s05-profiles.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return {
    defaultFiles: ['queue.go', 'service.go'],
    taggedFiles: ['queue.go', 'service.go', 'quota_schema.go', PROFILE_FILES[0]].sort(),
    defaultTests: ['queue_test.go'],
    taggedTests: ['queue_test.go', 'quota_schema_test.go', PROFILE_FILES[1]].sort()
  }
}

test('S05 退出码为该步独占', () => {
  assert.equal(EXIT_PROFILES, 77)
})

test('S05 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestQuotaProfile')
  for (const taken of ['TestQuotaRejectsNonFinite', 'TestBoundedQueue', 'TestScoped']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S05 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S05 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 11) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S05 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 9 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S05 关闭态：档位文件不得进入默认构建', () => {
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['queue.go', PROFILE_FILES[0]].sort() })
  assert.ok(leaked.some((p) => p.includes(`default build leaked ${PROFILE_FILES[0]}`)))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['queue.go'] })
  assert.ok(missing.some((p) => p.includes(`tagged build missing ${PROFILE_FILES[0]}`)))
  const leakedTest = validateIsolation({ ...isolation(), defaultTests: ['queue_test.go', PROFILE_FILES[1]].sort() })
  assert.ok(leakedTest.some((p) => p.includes(`default tests leaked ${PROFILE_FILES[1]}`)))
})

test('S05 双标签构建必须恰好新增两个生产文件', () => {
  const extra = validateIsolation({ ...isolation(), taggedFiles: ['queue.go', 'service.go', 'quota_schema.go', PROFILE_FILES[0], 'extra.go'].sort() })
  assert.ok(extra.some((p) => p.includes('tagged files=')))
  const missingQuota = validateIsolation({ ...isolation(), taggedFiles: ['queue.go', 'service.go', PROFILE_FILES[0]].sort() })
  assert.ok(missingQuota.some((p) => p.includes('tagged files=3')))
})

test('S05 源码守卫缺一即报', () => {
  assert.deepEqual(validateSource(process.cwd()), [])
  assert.equal(SOURCE_GUARDS.length, 8)
})

test('S05 绝对上限必须被三处引用', () => {
  assert.deepEqual(validateAbsoluteBound(process.cwd()), [])
})

test('S05 十个用例名必须逐个出现', () => {
  const body = CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package task_runtime')
  assert.equal(problems.length, CASES.length)
  assert.ok(problems.every((p) => p.startsWith('quota profile test is missing TestQuotaProfile')))
})

test('S05 行解析只认 m07-s05 空格前缀', () => {
  const log = [
    '--- PASS: TestQuotaProfileResolveDefaults',
    '    quota_profiles_test.go:1: m07-s05 resolve a',
    '    quota_profiles_test.go:2: m07-s05-detail noise',
    '    quota_profiles_test.go:3: m07-s05 deny b'
  ].join('\n')
  assert.deepEqual(parseRows(log), ['resolve a', 'deny b'])
})

test('S05 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S05 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S05 标签组合与包路径固定', () => {
  assert.equal(BUILD_TAGS, 'rsi_quota rsi_profiles')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/task_runtime')
  assert.ok(GO_TEST_FLAGS.includes('-race'))
  assert.ok(GO_TEST_FLAGS.includes('rsi_quota rsi_profiles'))
})

test('S05 渲染结果包含双标签与 12 行证据', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.ok(text.includes('isolation: default_files=2 tagged_files=4 default_tests=1 tagged_tests=3'))
  assert.ok(text.includes('invocation=profiles package=' + PACKAGE_PATH))
  assert.ok(text.includes('passed=10 failed=0 exit=0 race=true'))
  assert.deepEqual(parseRows(text), EVIDENCE_ROWS)
})
