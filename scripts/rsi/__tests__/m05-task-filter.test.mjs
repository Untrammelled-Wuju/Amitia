import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BYPASS_TOKENS,
  EVIDENCE_ROWS,
  EXIT_TASK_FILTER,
  EXPECTED_PASSED,
  EXPOSURE_GUARDS,
  FILTER_GUARDS,
  PROFILE_GUARDS,
  RUN_PATTERN,
  parseTaskFilterRows,
  renderEvidence,
  validateAssertSource,
  validateEvidence,
  validateExposureSource,
  validateFilterSource,
  validateNoQueryGrant,
  validateProfileSource
} from '../m05-s03-task-filter.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function goFiles() {
  return { goFiles: ['task_selector.go', 'exposure.go'], testFiles: ['task_selector_test.go'] }
}

test('S03 退出码为该步独占', () => {
  assert.equal(EXIT_TASK_FILTER, 55)
})

test('S03 用例前缀不与既有前缀撞车', () => {
  for (const taken of ['TestCapabilityDescriptor', 'TestCapabilityRevision', 'TestJournal', 'TestLedger', 'TestSkill']) {
    assert.equal(RUN_PATTERN.includes(taken), false)
  }
  assert.equal(RUN_PATTERN, 'TestTaskGate')
})

test('S03 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence({ ...goFiles(), ...rowSet() }), [])
})

test('S03 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 8) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S03 未带 race 或退出非零时拦下', () => {
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), exitCode: 2 }).some((p) => p.includes('exit=2')))
  assert.ok(validateEvidence({ ...goFiles(), ...rowSet(), passed: 9 }).some((p) => p.includes('passed=9')))
})

test('S03 测试文件不得混入默认构建', () => {
  const problems = validateEvidence({ ...goFiles(), ...rowSet(), goFiles: ['task_selector.go', 'task_selector_test.go'] })
  assert.ok(problems.some((p) => p.includes('leaked test files')))
})

test('S03 分类与选择接口守卫齐全', () => {
  assert.deepEqual(validateProfileSource(PROFILE_GUARDS.join('\n')), [])
  const problems = validateProfileSource('package capability')
  assert.equal(problems.length, PROFILE_GUARDS.length)
  assert.ok(problems.every((p) => p.startsWith('task selector source is missing ')))
})

test('S03 硬过滤必须覆盖权限/离线/健康/契约', () => {
  assert.deepEqual(validateFilterSource(FILTER_GUARDS.join('\n')), [])
  for (const guard of ['if !state.PermissionGranted {', 'if !state.RuntimeReady {', 'if state.Health != HealthReady {', 'contractReasons, contractCause, err := s.contractProblems(ctx, tool)']) {
    assert.ok(FILTER_GUARDS.includes(guard))
    assert.deepEqual(validateFilterSource(FILTER_GUARDS.filter((item) => item !== guard).join('\n')).length, 1)
  }
})

test('S03 曝光链保留 feature off 路径', () => {
  assert.deepEqual(validateExposureSource(EXPOSURE_GUARDS.join('\n')), [])
  assert.ok(EXPOSURE_GUARDS.includes('if m.taskSelector == nil {'))
  assert.ok(EXPOSURE_GUARDS.includes('return nil, fmt.Errorf("select tools for task: %w", err)'))
})

test('S03 断言源必须包含最高分被拦的断言', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.ok(ASSERT_GUARDS.includes('if blockedScore <= allowedScore {'))
  assert.ok(ASSERT_GUARDS.includes('if injected.Claimed(guarded.ID) {'))
})

test('S03 授权不得从查询文本读取', () => {
  const clean = 'if !state.PermissionGranted { reasons = appendUniqueReason(reasons, ReasonPermissionMissing) }'
  assert.deepEqual(validateNoQueryGrant(clean), [])
  for (const token of BYPASS_TOKENS) {
    assert.deepEqual(validateNoQueryGrant(clean + token), [`hard filter must not read grants from the query but contains ${token}`])
  }
})

test('S03 行解析只认 m05-s03 前缀', () => {
  const log = '--- PASS: X\n    x_test.go:1: m05-s03 a\nnoise m05-s02 b\n    x_test.go:2: m05-s03 c\n'
  assert.deepEqual(parseTaskFilterRows(log), ['a', 'c'])
})

test('S03 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...goFiles(), ...rowSet(), rows: EVIDENCE_ROWS })
  assert.deepEqual(parseTaskFilterRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('passed=10 failed=0 exit=0 race=true'))
})
