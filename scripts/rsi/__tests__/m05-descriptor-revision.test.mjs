import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  EVIDENCE_ROWS,
  EXIT_DESCRIPTOR_REVISION,
  EXPECTED_PASSED,
  APPLY_GUARDS,
  EVENT_GUARDS,
  RUN_PATTERN,
  parseRevisionRows,
  renderEvidence,
  validateApplySource,
  validateEvidence,
  validateEventSource,
  validateNoTimerRebuild
} from '../m05-s02-descriptor-revision.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function goFiles() {
  return { goFiles: ['descriptor_index.go', 'descriptor_revision.go'], testFiles: ['descriptor_revision_test.go'] }
}

test('S02 退出码为该步独占', () => {
  assert.equal(EXIT_DESCRIPTOR_REVISION, 54)
})

test('S02 用例前缀不与既有前缀撞车', () => {
  for (const taken of ['TestCapabilityDescriptor', 'TestLedger', 'TestJournal']) {
    assert.equal(RUN_PATTERN.includes(taken), false)
  }
  assert.equal(RUN_PATTERN, 'TestCapabilityRevision')
})

test('S02 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence({ ...goFiles(), ...rowSet() }), [])
})

test('S02 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 6) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[2]] }).some((p) => p.includes('exactly once')))
})

test('S02 未带 race 或退出非零时拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), exitCode: 2 }).some((p) => p.includes('exit=2')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), passed: 6 }).some((p) => p.includes('passed=6')))
})

test('S02 源文件缺项会被点名', () => {
  assert.deepEqual(validateEventSource(EVENT_GUARDS.join('\n')), [])
  const problems = validateEventSource('package capability')
  assert.equal(problems.length, EVENT_GUARDS.length)
  assert.ok(problems.every((p) => p.startsWith('descriptor revision source is missing ')))
})

test('S02 增量语义守卫齐全', () => {
  assert.deepEqual(validateApplySource(APPLY_GUARDS.join('\n')), [])
  assert.ok(APPLY_GUARDS.includes('if event.Revision <= i.revision {'))
  assert.ok(APPLY_GUARDS.includes('if event.Revision > i.revision+1 {'))
})

test('S02 断档修复不得依赖定时器', () => {
  const clean = 'func Repair() { if revision <= current { return } }'
  assert.deepEqual(validateNoTimerRebuild(clean), [])
  for (const token of ['time.NewTicker', 'time.AfterFunc', 'time.Tick(', 'time.After(']) {
    assert.deepEqual(validateNoTimerRebuild(clean + token), [`revision repair must stay event-driven but contains ${token}`])
  }
})

test('S02 行解析只认 m05-s02 前缀', () => {
  const log = '--- PASS: X\n    x_test.go:1: m05-s02 a\nnoise m05-s01 b\n    x_test.go:2: m05-s02 c\n'
  assert.deepEqual(parseRevisionRows(log), ['a', 'c'])
})

test('S02 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS })
  assert.deepEqual(parseRevisionRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('passed=7 failed=0 exit=0 race=true'))
})
