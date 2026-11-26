import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  EVIDENCE_ROWS,
  EXIT_DESCRIPTOR_INDEX,
  EXPECTED_PASSED,
  ASSERT_GUARDS,
  INDEX_GUARDS,
  RUN_PATTERN,
  parseDescriptorRows,
  renderEvidence,
  validateEvidence,
  validateIndexSource,
  validateQueryReadOnly
} from '../m05-s01-descriptor-index.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function goFiles() {
  return { goFiles: ['descriptor_index.go', 'schema_store.go'], testFiles: ['descriptor_index_test.go'] }
}

test('S01 退出码为该步独占', () => {
  assert.equal(EXIT_DESCRIPTOR_INDEX, 53)
})

test('S01 用例前缀不与既有前缀撞车', () => {
  for (const taken of ['TestLedger', 'TestJournal', 'TestEffect']) {
    assert.equal(taken.startsWith(RUN_PATTERN), false)
    assert.equal(RUN_PATTERN.includes(taken), false)
  }
  assert.equal(RUN_PATTERN, 'TestCapabilityDescriptor')
})

test('S01 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence({ ...goFiles(), ...rowSet() }), [])
})

test('S01 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS.slice(1) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[1]] }).some((p) => p.includes('exactly once')))
})

test('S01 未带 race 或退出非零时拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), exitCode: 1 }).some((p) => p.includes('exit=1')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), passed: 7 }).some((p) => p.includes('passed=7')))
})

test('S01 源文件缺项会被点名', () => {
  assert.deepEqual(validateIndexSource(INDEX_GUARDS.join('\n')), [])
  const problems = validateIndexSource('package capability')
  assert.ok(problems.length >= INDEX_GUARDS.length)
  assert.ok(problems.every((p) => p.startsWith('descriptor index source is missing ')))
})

test('S01 目录查询必须完全不碰 Schema 存储', () => {
  const clean = 'func (i *DescriptorIndex) Query(filter ToolFilter) []DescriptorEntry {\n\treturn i.entries\n}\n'
  assert.deepEqual(validateQueryReadOnly(clean), [])
  const reader = 'func (i *DescriptorIndex) Query(filter ToolFilter) []DescriptorEntry {\n\tstore.Load(entry.SchemaRef, false)\n\treturn nil\n}\n'
  assert.deepEqual(validateQueryReadOnly(reader), ['descriptor query must stay schema-free but contains store.Load('])
  const writer = 'func (i *DescriptorIndex) Query(filter ToolFilter) []DescriptorEntry {\n\tstore.Put(nil)\n\treturn nil\n}\n'
  assert.equal(validateQueryReadOnly(writer).length, 1)
  assert.deepEqual(validateQueryReadOnly('func Other() {}'), ['descriptor index source is missing Query'])
})

test('S01 断言守卫覆盖零读与不可覆写', () => {
  assert.ok(ASSERT_GUARDS.includes('if store.Writes() != 1000 || store.Reads() != 0 {'))
  assert.ok(ASSERT_GUARDS.includes('if store.Reads() != before {'))
})

test('S01 行解析只认 m05-s01 前缀', () => {
  const log = '--- PASS: X\n    x_test.go:1: m05-s01 a\nnoise m04-s12 b\n    x_test.go:2: m05-s01 c\n'
  assert.deepEqual(parseDescriptorRows(log), ['a', 'c'])
})

test('S01 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS })
  assert.deepEqual(parseDescriptorRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('passed=8 failed=0 exit=0 race=true'))
})
