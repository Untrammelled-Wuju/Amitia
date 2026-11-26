import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILD_TAG,
  CACHE_SOURCE,
  DETAIL_GUARDS,
  EVIDENCE_ROWS,
  EXIT_SCHEMA_CACHE,
  EXPECTED_PASSED,
  FORBIDDEN_TOKENS,
  GO_TEST_FLAGS,
  PACKAGE_PATH,
  REVISION_GUARDS,
  RUN_PATTERN,
  SCHEMA_CASES,
  STORE_GUARDS,
  parseGoListFiles,
  parseGoTestOutput,
  parseSchemaRows,
  renderEvidence,
  validateAssertSource,
  validateCaseNames,
  validateDetailSource,
  validateEvidence,
  validateIsolation,
  validateNoFullScan,
  validateRevisionSource,
  validateStoreSource
} from '../m05-s10-schema-cache.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  const shared = ['detail_budget_test.go', 'task_selector_test.go']
  return { defaultTests: [...shared], taggedTests: [...shared, CACHE_SOURCE].sort() }
}

test('S10 退出码为该步独占', () => {
  assert.equal(EXIT_SCHEMA_CACHE, 62)
})

test('S10 用例前缀不与既有前缀撞车', () => {
  assert.equal(RUN_PATTERN, 'TestSchemaCache')
  for (const taken of ['TestPreauth', 'TestCutover', 'TestScale', 'TestDetail']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  for (const name of SCHEMA_CASES) assert.ok(name.startsWith(RUN_PATTERN))
})

test('S10 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S10 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[3]] }).some((p) => p.includes('exactly once')))
})

test('S10 未带 race、退出非零或数量不符时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 8 }).some((p) => p.includes('passed=')))
  assert.ok(validateEvidence({ ...rowSet(), failed: 1 }).some((p) => p.includes('failed=1')))
})

test('S10 详情路径只读被选中的工具', () => {
  assert.deepEqual(validateDetailSource(DETAIL_GUARDS.join('\n')), [])
  assert.equal(validateDetailSource('package capability').length, DETAIL_GUARDS.length)
  assert.ok(DETAIL_GUARDS.includes('for _, selected := range report.Selected[offset:end] {'))
  assert.ok(DETAIL_GUARDS.includes('page.SchemaReads = m.schemaStore.Reads() - readsBefore'))
})

test('S10 内容寻址与新鲜度检查必须存在', () => {
  assert.deepEqual(validateStoreSource(STORE_GUARDS.join('\n')), [])
  assert.ok(STORE_GUARDS.includes('if NewSchemaRef(body) != ref {'))
  assert.ok(STORE_GUARDS.includes('return nil, fmt.Errorf("%w: ref=%s", ErrSchemaExpired, ref)'))
})

test('S10 版本守卫钉住断档与陈旧快照', () => {
  assert.deepEqual(validateRevisionSource(REVISION_GUARDS.join('\n')), [])
  assert.ok(REVISION_GUARDS.includes('if event.Revision > i.revision+1 {'))
  assert.ok(REVISION_GUARDS.includes('return report, fmt.Errorf("%w: snapshot=%d current=%d", ErrDescriptorRevisionStale, revision, current)'))
})

test('S10 断言守卫钉住跨 actor 与跨版本隔离', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if store.Reads() != readsBefore {'))
  assert.ok(ASSERT_GUARDS.includes('if entryV1.SchemaRef == entryV2.SchemaRef {'))
  assert.ok(ASSERT_GUARDS.includes('if page.SchemaReads != int64(len(page.Views)) || page.SchemaReads > 2 {'))
})

test('S10 九个用例名必须逐个出现', () => {
  const body = SCHEMA_CASES.map((name) => `func ${name}(t *testing.T) {`).join('\n')
  assert.deepEqual(validateCaseNames(body), [])
  const problems = validateCaseNames('package capability_test')
  assert.equal(problems.length, SCHEMA_CASES.length)
  assert.ok(problems.every((p) => p.startsWith('schema cache test is missing TestSchemaCache')))
})

test('S10 禁止整表枚举与逐条注册', () => {
  const clean = 'for _, selected := range report.Selected[offset:end] {'
  assert.deepEqual(validateNoFullScan(clean), [])
  for (const token of FORBIDDEN_TOKENS) {
    assert.deepEqual(validateNoFullScan(clean + token), [`schema cache surface must not contain ${token}`])
  }
  assert.ok(GO_TEST_FLAGS.includes(BUILD_TAG))
  assert.ok(GO_TEST_FLAGS.includes('-race'))
})

test('S10 构建标签把隔离测试挡在默认集合外', () => {
  assert.equal(BUILD_TAG, 'rsi_schema_cache')
  const leaked = validateIsolation({ ...isolation(), defaultTests: [...isolation().defaultTests, CACHE_SOURCE].sort() })
  assert.ok(leaked.some((p) => p.includes('leaked')))
  const missing = validateIsolation({ ...isolation(), taggedTests: [...isolation().defaultTests] })
  assert.ok(missing.some((p) => p.includes('missing schema_cache_test.go')))
  const extra = validateIsolation({ ...isolation(), taggedTests: [...isolation().taggedTests, 'zzz_test.go'] })
  assert.ok(extra.some((p) => p.includes('want')))
})

test('S10 行解析只认 m05-s10 空格前缀', () => {
  const log = [
    '--- PASS: TestSchemaCacheActorIndexIsolation',
    '    schema_cache_test.go:1: m05-s10 actor a',
    '    schema_cache_test.go:2: m05-s10-detail noise',
    '    schema_cache_test.go:3: m05-s10 ttl b'
  ].join('\n')
  assert.deepEqual(parseSchemaRows(log), ['actor a', 'ttl b'])
})

test('S10 测试计数只认 PASS/FAIL 行', () => {
  assert.deepEqual(parseGoTestOutput('--- PASS: A\n--- PASS: B\n=== RUN x\n--- FAIL: C\n'), { passed: 2, failed: 1 })
})

test('S10 go list 文件列表解析', () => {
  assert.deepEqual(parseGoListFiles('[b.go a.go]'), ['a.go', 'b.go'])
  assert.deepEqual(parseGoListFiles('[]'), [])
  assert.deepEqual(parseGoListFiles(''), [])
  assert.deepEqual(parseGoListFiles('garbage'), [])
})

test('S10 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseSchemaRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('invocation=schema-cache package=' + PACKAGE_PATH))
  assert.ok(text.includes('delta=[schema_cache_test.go]'))
  assert.ok(text.includes('passed=9 failed=0 exit=0 race=true'))
})
