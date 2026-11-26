import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ASSERT_GUARDS,
  BUILDER_GUARDS,
  BUILD_TAG,
  DEFAULT_SOURCES,
  ENUMERATION_TOKENS,
  EVIDENCE_ROWS,
  EXIT_AGENT_CONTEXT,
  EXPECTED_PASSED,
  FORBIDDEN_IMPORTS,
  REQUEST_GUARDS,
  RUN_PATTERN,
  TAGGED_SOURCES,
  TAGGED_TESTS,
  parseAgentContextRows,
  renderEvidence,
  validateAssertSource,
  validateBuilderSource,
  validateEvidence,
  validateIsolation,
  validateNoPromptImports,
  validateNoRegistryEnumeration,
  validateRequestSource
} from '../m05-s06-agent-context.mjs'

function rowSet() {
  return { rows: [...EVIDENCE_ROWS], passed: EXPECTED_PASSED, failed: 0, exitCode: 0, race: true }
}

function isolation() {
  return { defaultFiles: [...DEFAULT_SOURCES], taggedFiles: [...TAGGED_SOURCES], taggedTests: [...TAGGED_TESTS] }
}

test('S06 退出码为该步独占', () => {
  assert.equal(EXIT_AGENT_CONTEXT, 58)
})

test('S06 用例前缀不与既有前缀撞车', () => {
  for (const taken of ['TestAgentContexts', 'TestDetail', 'TestRanking', 'TestTaskGate']) {
    assert.equal(RUN_PATTERN === taken, false)
  }
  assert.equal(RUN_PATTERN, 'TestAgentContext')
})

test('S06 证据达标时无问题', () => {
  assert.deepEqual(validateEvidence(rowSet()), [])
  assert.deepEqual(validateIsolation(isolation()), [])
})

test('S06 证据行缺失或重复都会被拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), rows: EVIDENCE_ROWS.slice(0, 7) }).some((p) => p.includes('evidence rows')))
  assert.ok(validateEvidence({ ...rowSet(), rows: [...EVIDENCE_ROWS, EVIDENCE_ROWS[0]] }).some((p) => p.includes('exactly once')))
})

test('S06 未带 race 或退出非零时拦下', () => {
  assert.ok(validateEvidence({ ...rowSet(), race: false }).some((p) => p.includes('-race')))
  assert.ok(validateEvidence({ ...rowSet(), exitCode: 2 }).some((p) => p.includes('exit=2')))
  assert.ok(validateEvidence({ ...rowSet(), passed: 5 }).some((p) => p.includes('passed=5')))
})

test('S06 默认构建必须只剩 doc.go', () => {
  assert.equal(BUILD_TAG, 'rsi_agentcontext')
  const leaked = validateIsolation({ ...isolation(), defaultFiles: ['builder.go', 'doc.go'] })
  assert.ok(leaked.some((p) => p.includes('default build files')))
  const missing = validateIsolation({ ...isolation(), taggedFiles: ['doc.go'] })
  assert.ok(missing.some((p) => p.includes('tagged build files')))
  const noTests = validateIsolation({ ...isolation(), taggedTests: [] })
  assert.ok(noTests.some((p) => p.includes('tagged tests')))
})

test('S06 构建器守卫缺一即报', () => {
  assert.deepEqual(validateBuilderSource(BUILDER_GUARDS.join('\n')), [])
  const problems = validateBuilderSource('package agentcontext')
  assert.equal(problems.length, BUILDER_GUARDS.length)
  assert.ok(problems.every((p) => p.startsWith('agent context builder is missing ')))
})

test('S06 feature off 分支保留旧路径', () => {
  assert.ok(BUILDER_GUARDS.includes('if b.exposure.TaskSelector() == nil {'))
  assert.ok(BUILDER_GUARDS.includes('views := b.exposure.GetModelTools(ctx, request.Session, budget)'))
})

test('S06 详情分支错误必须原样包装', () => {
  assert.ok(BUILDER_GUARDS.some((guard) => guard.includes('build agent context details: %w')))
  assert.ok(BUILDER_GUARDS.some((guard) => guard.includes('GetModelToolDetails(')))
})

test('S06 详情分页契约以显式类型消费', () => {
  assert.ok(BUILDER_GUARDS.includes('page capability.DetailPage'))
  assert.ok(BUILDER_GUARDS.includes('page, err = b.exposure.GetModelToolDetails(ctx, request.Session, capability.DetailRequest{'))
})

test('S06 请求面守卫覆盖三类非法输入', () => {
  assert.deepEqual(validateRequestSource(REQUEST_GUARDS.join('\n')), [])
  assert.ok(REQUEST_GUARDS.includes('if request.Limit < 0 {'))
  assert.ok(REQUEST_GUARDS.includes('if request.TokenBudget < 0 {'))
  assert.ok(REQUEST_GUARDS.includes('if strings.TrimSpace(request.Query) == "" {'))
})

test('S06 构建器不得整表枚举注册表', () => {
  const clean = 'func (b *AgentContextBuilder) Build() {'
  assert.deepEqual(validateNoRegistryEnumeration(clean), [])
  for (const token of ENUMERATION_TOKENS) {
    assert.deepEqual(validateNoRegistryEnumeration(clean + token), [`agent context builder must not enumerate the registry but contains ${token}`])
  }
})

test('S06 构建器不得触碰生产 Prompt 包', () => {
  const clean = 'import "context"'
  assert.deepEqual(validateNoPromptImports(clean), [])
  for (const token of FORBIDDEN_IMPORTS) {
    assert.deepEqual(validateNoPromptImports(clean + token), [`agent context builder must not import ${token}`])
  }
})

test('S06 断言守卫缺一即报', () => {
  assert.deepEqual(validateAssertSource(ASSERT_GUARDS.join('\n')), [])
  assert.equal(validateAssertSource('').length, ASSERT_GUARDS.length)
  assert.ok(ASSERT_GUARDS.some((guard) => guard.includes('reflect.DeepEqual(got.Tools, want)')))
})

test('S06 行解析只认 m05-s06 前缀', () => {
  const log = '--- PASS: X\n    x_test.go:1: m05-s06 a\nnoise m05-s05 b\n    x_test.go:2: m05-s06 c\n'
  assert.deepEqual(parseAgentContextRows(log), ['a', 'c'])
})

test('S06 渲染出的行能被自己解析回原样', () => {
  const text = renderEvidence({ ...rowSet(), ...isolation() })
  assert.deepEqual(parseAgentContextRows(text), EVIDENCE_ROWS)
  assert.ok(text.includes('passed=6 failed=0 exit=0 race=true'))
})
