import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/agentcontext'
export const BUILD_TAG = 'rsi_agentcontext'
export const RUN_PATTERN = 'TestAgentContext'
export const EXIT_AGENT_CONTEXT = 58
export const EXPECTED_PASSED = 6
export const DEFAULT_SOURCES = ['doc.go']
export const TAGGED_SOURCES = ['builder.go', 'doc.go']
export const TAGGED_TESTS = ['builder_test.go']

export const BUILDER_GUARDS = [
  'func NewAgentContextBuilder(exposure *capability.ToolExposureManager) (*AgentContextBuilder, error) {',
  'if exposure == nil {',
  'if err := ctx.Err(); err != nil {',
  'if b.exposure.TaskSelector() == nil {',
  'views := b.exposure.GetModelTools(ctx, request.Session, budget)',
  'views, err := b.exposure.GetModelToolsForTask(ctx, request.Session, request.Query, budget, request.Limit)',
  'page capability.DetailPage',
  'page, err = b.exposure.GetModelToolDetails(ctx, request.Session, capability.DetailRequest{',
  'return AgentContext{}, fmt.Errorf("build agent context details: %w", err)'
]

export const REQUEST_GUARDS = [
  'type AgentContextRequest struct {',
  'type AgentContext struct {',
  'func (b *AgentContextBuilder) WithTokenBudget(budget int) (*AgentContextBuilder, error) {',
  'if request.Limit < 0 {',
  'if request.TokenBudget < 0 {',
  'if strings.TrimSpace(request.Query) == "" {'
]

export const ASSERT_GUARDS = [
  'if len(names) == 0 || names[0] != "report_writer" {',
  'if !reflect.DeepEqual(got.Tools, want) {',
  'if first.Tokens > request.TokenBudget {',
  'if first.Reads != int64(len(first.Tools)) {',
  'if first.Trimmed < 1 {',
  'if !errors.Is(err, context.Canceled) {',
  'if !reflect.DeepEqual(first.Tools, third.Tools) {'
]

export const ENUMERATION_TOKENS = ['registry.List(', 'registry.Register(', 'GetModelTools(ctx, sessionCtx, 0)']

export const FORBIDDEN_IMPORTS = ['internal/prompt', 'internal/chat', 'internal/character']

export const EVIDENCE_ROWS = [
  'task top1=report_writer tools=2 tokens=911 mode=task',
  'legacy views=4 equal=true mode=legacy tokens=1818',
  'detail views=2 tokens=201 budget=220 reads=2 trimmed=2 has_more=true',
  'cursor second=2 distinct=4',
  'invalid rejections=6 all_rejected=true',
  'repeat equal=true tokens=201 reads_per_call=2',
  'restart equal=true tools=2',
  'cancel passed_through=true tools=0'
]

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function parseAgentContextRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s06 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s06 '.length).trim())
  }
  return rows
}

export function parseGoListFiles(output) {
  const body = String(output ?? '').trim()
  if (!body.startsWith('[') || !body.endsWith(']')) return []
  const inner = body.slice(1, -1).trim()
  if (inner === '') return []
  return inner.split(/\s+/).sort()
}

function missingGuards(text, guards, label) {
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `${label} is missing ${guard}`)
}

export function validateBuilderSource(text) {
  return missingGuards(text, BUILDER_GUARDS, 'agent context builder')
}

export function validateRequestSource(text) {
  return missingGuards(text, REQUEST_GUARDS, 'agent context request surface')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'agent context test')
}

export function validateNoRegistryEnumeration(text) {
  const body = String(text ?? '')
  return ENUMERATION_TOKENS.filter((token) => body.includes(token)).map((token) => `agent context builder must not enumerate the registry but contains ${token}`)
}

export function validateNoPromptImports(text) {
  const body = String(text ?? '')
  return FORBIDDEN_IMPORTS.filter((token) => body.includes(token)).map((token) => `agent context builder must not import ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultFiles = Array.isArray(evidence?.defaultFiles) ? evidence.defaultFiles : []
  const taggedFiles = Array.isArray(evidence?.taggedFiles) ? evidence.taggedFiles : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultFiles.join(',') !== DEFAULT_SOURCES.join(',')) {
    problems.push(`default build files=${defaultFiles.join(',')} want ${DEFAULT_SOURCES.join(',')}`)
  }
  if (taggedFiles.join(',') !== TAGGED_SOURCES.join(',')) {
    problems.push(`tagged build files=${taggedFiles.join(',')} want ${TAGGED_SOURCES.join(',')}`)
  }
  if (taggedTests.join(',') !== TAGGED_TESTS.join(',')) {
    problems.push(`tagged tests=${taggedTests.join(',')} want ${TAGGED_TESTS.join(',')}`)
  }
  if (defaultFiles.some((file) => file.endsWith('_test.go'))) problems.push('default build leaked test files')
  return problems
}

export function validateEvidence(evidence) {
  const problems = []
  const rows = Array.isArray(evidence?.rows) ? evidence.rows : []
  if (rows.length !== EVIDENCE_ROWS.length) problems.push(`evidence rows=${rows.length} want ${EVIDENCE_ROWS.length}`)
  for (const expected of EVIDENCE_ROWS) {
    const count = rows.filter((row) => row === expected).length
    if (count !== 1) problems.push(`evidence row must print exactly once (${count}): ${expected}`)
  }
  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  if (evidence?.race !== true) problems.push('go test must run with -race')
  return problems
}

export function renderEvidence(evidence) {
  const guards = BUILDER_GUARDS.length + REQUEST_GUARDS.length + ASSERT_GUARDS.length
  const lines = [
    `invocation=agent-context package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default=[${(evidence?.defaultFiles ?? []).join(' ')}] tagged=[${(evidence?.taggedFiles ?? []).join(' ')}] tests=[${(evidence?.taggedTests ?? []).join(' ')}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m05-s06 ${row}`)
  lines.push(`passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`)
  return lines.join('\n') + '\n'
}

function goList(root, extraArgs) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}|{{.XTestGoFiles}}', ...extraArgs, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 120000
  })
  const line = `${result.stdout ?? ''}`.trim()
  const [files = '', tests = '', external = ''] = line.split('|')
  return {
    files: parseGoListFiles(files),
    tests: [...parseGoListFiles(tests), ...parseGoListFiles(external)].sort(),
    exitCode: result.status === null ? 1 : result.status
  }
}

export function runAgentContextEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const base = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'agentcontext')
  const builderSource = readFileSync(resolve(base, 'builder.go'), 'utf8')
  const testSource = readFileSync(resolve(base, 'builder_test.go'), 'utf8')
  const result = spawnSync('go', ['test', '-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 600000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    defaultFiles: defaultList.files,
    taggedFiles: taggedList.files,
    taggedTests: taggedList.tests,
    builderSource,
    testSource,
    rows: parseAgentContextRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: true,
    log
  }
}

function reportProblems(problems) {
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  return problems.length > 0
}

function main() {
  try {
    if (process.argv.includes('--list')) {
      process.stdout.write(`invocation=agent-context package=${PACKAGE_PATH} rows=${EVIDENCE_ROWS.length}\n`)
      return
    }
    const evidence = runAgentContextEvidence(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = [
      ...validateBuilderSource(evidence.builderSource),
      ...validateRequestSource(evidence.builderSource),
      ...validateNoRegistryEnumeration(evidence.builderSource),
      ...validateNoPromptImports(evidence.builderSource),
      ...validateAssertSource(evidence.testSource),
      ...validateIsolation(evidence),
      ...validateEvidence(evidence)
    ]
    if (reportProblems(problems)) process.exit(EXIT_AGENT_CONTEXT)
    if (evidence.exitCode !== 0) process.exit(EXIT_AGENT_CONTEXT)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_AGENT_CONTEXT)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
