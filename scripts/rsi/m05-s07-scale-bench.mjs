import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/capability'
export const RUN_PATTERN = 'TestScale'
export const BUILD_TAG = 'rsi_scale'
export const EXIT_SCALE_BENCH = 59
export const EXPECTED_PASSED = 6
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '900s', '-run', RUN_PATTERN, '-v']
export const SCALE_SOURCES = ['scale_bench_test.go']
export const SCALE_CASES = [
  'TestScaleColdIndexBuild',
  'TestScaleHotQueryLatency',
  'TestScaleTop1Top5Quality',
  'TestScaleSchemaTokenCurve',
  'TestScalePermissionChange',
  'TestScaleRepeatCancelAndRestart'
]

export const SCALE_GUARDS = [
  'scalePlugins      = 2000',
  'scaleSlots        = 5',
  'scaleOperations   = 200',
  'scaleQualityCases = 20',
  'registry.BatchRegister(context.Background(), defs)',
  'capability.BuildDescriptorIndex(store, defs)',
  'manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{'
]

export const ASSERT_GUARDS = [
  'if store.Reads() != 0 {',
  'if first != want {',
  'if p99 > p50*8+8*time.Millisecond {',
  'if top1 != scaleQualityCases {',
  'if injected != 0 {',
  'if !equal {',
  'if !taskGateHasReason(rejected.Reasons, capability.ReasonPermissionMissing) {',
  'if _, err := manager.GetModelToolDetails(ctx, detailInvocation(), capability.DetailRequest{Query: query, Limit: -1}); !errors.Is(err, capability.ErrDetailRequestInvalid) {'
]

export const FORBIDDEN_TOKENS = ['registry.Register(ctx', 'registry.List(', 'GetModelTools(ctx']

export const EVIDENCE_ROWS = [
  'cold plugins=2000 tools=10000 entries=10000 schema_reads=0',
  'hot ops=200 top1=tool-00615 tail_guard=true',
  'quality queries=20 top1=20 top5=20 false_injection=0 rate=0.000',
  'tokens n200=1988 n1000=1988 n10000=1988 views=3 reads=4,4,4 equal=true',
  'permission before=tool-00615 after=tool-00015 rejected=1 reason=permission_missing zone_kept=true',
  'repeat equal=true restart_equal=true cancel=context.Canceled invalid_limit=ErrDetailRequestInvalid'
]

export function parseScaleRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m05-s07 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm05-s07 '.length).trim())
  }
  return rows
}

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
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

export function validateScaleSource(text) {
  return missingGuards(text, SCALE_GUARDS, 'scale benchmark')
}

export function validateAssertSource(text) {
  return missingGuards(text, ASSERT_GUARDS, 'scale assertions')
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return SCALE_CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `scale benchmark is missing ${name}`)
}

export function validateNoSequentialRegistration(text) {
  const body = String(text ?? '')
  return FORBIDDEN_TOKENS.filter((token) => body.includes(token)).map((token) => `scale benchmark must batch-load the registry but contains ${token}`)
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  const scale = SCALE_SOURCES[0]
  if (defaultTests.includes(scale)) problems.push(`default build leaked scale tests=${scale}`)
  if (!taggedTests.includes(scale)) problems.push(`tagged tests missing ${scale}`)
  const expected = [...defaultTests, scale].sort().join(',')
  if (taggedTests.join(',') !== expected) {
    problems.push(`tagged tests=${taggedTests.join(',')} want ${expected}`)
  }
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
  const guards = SCALE_GUARDS.length + ASSERT_GUARDS.length + SCALE_CASES.length
  const lines = [
    `invocation=scale-bench package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_tests=[${(evidence?.defaultTests ?? []).join(' ')}] tagged_tests=[${(evidence?.taggedTests ?? []).join(' ')}]`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m05-s07 ${row}`)
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

export function runScaleEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const sourcePath = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'capability', SCALE_SOURCES[0])
  const source = readFileSync(sourcePath, 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 1200000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    source,
    sourcePath,
    rows: parseScaleRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultFiles: defaultList.files,
    defaultTests: defaultList.tests,
    taggedFiles: taggedList.files,
    taggedTests: taggedList.tests,
    listExitCode: defaultList.exitCode + taggedList.exitCode,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runScaleEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateScaleSource(evidence.source),
    ...validateAssertSource(evidence.source),
    ...validateCaseNames(evidence.source),
    ...validateNoSequentialRegistration(evidence.source)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_SCALE_BENCH)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
