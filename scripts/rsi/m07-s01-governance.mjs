import { spawnSync } from 'node:child_process'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/resource'
export const RUN_PATTERN = 'TestGovernance'
export const BUILD_TAG = 'rsi_resource'
export const EXIT_GOVERNANCE = 73
export const EXPECTED_PASSED = 9
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-run', RUN_PATTERN, '-v']
export const GOVERNANCE_FILES = ['inventory.go', 'governance.go']

export const INVENTORY = [
  { kind: 'resource_ownership', symbol: 'type ResourceOwnershipService interface', path: 'backend/internal/extension/kernel/resource/service.go', owner: 'resource' },
  { kind: 'event_subscription', symbol: 'type SubscriptionRepository interface', path: 'backend/internal/extension/kernel/event/subscription_repository.go', owner: 'event' },
  { kind: 'event_subscription', symbol: 'func NewDispatcher(', path: 'backend/internal/extension/kernel/event/event_dispatcher.go', owner: 'event' },
  { kind: 'priority_queue', symbol: 'type PriorityQueue = queue.PriorityQueue', path: 'backend/internal/scheduler/priority.go', owner: 'scheduler' },
  { kind: 'backpressure', symbol: 'func NewBackpressureController(', path: 'backend/internal/scheduler/backpressure.go', owner: 'scheduler' },
  { kind: 'outbox', symbol: 'func NewOutbox(', path: 'backend/internal/scheduler/backpressure.go', owner: 'scheduler' },
  { kind: 'task_entry', symbol: 'func ResolveTaskEntry(', path: 'backend/internal/extension/kernel/task_runtime/entry.go', owner: 'task_runtime' }
]

export const EVIDENCE_ROWS = [
  'adopt kinds=6 leases=6',
  'reject invalid=4 leases=0',
  'duplicate rejected=true leases=1',
  'close_all reverse=true order=third,second,first',
  'close_all errors=2 attempted_all=true',
  'release closes_on_cancel=true second=ErrLeaseUnknown',
  'closed rejects_adopt=true rejects_double_close=true',
  'stats exposed=true cancelled_adopt=context.Canceled',
  'inventory entries=7 kinds=6 single_owner=true'
]

export const CASES = [
  'TestGovernanceAdoptsEveryInventoryKind',
  'TestGovernanceRejectsUnknownKindAndBadSpecs',
  'TestGovernanceDuplicateRejected',
  'TestGovernanceCloseAllReverseOrder',
  'TestGovernanceCloseAllAggregatesErrors',
  'TestGovernanceReleaseSemantics',
  'TestGovernanceClosedRejectsAdopt',
  'TestGovernanceStatsAndCancelAdopt',
  'TestGovernanceInventoryCoversSingleOwners'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s01 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s01 '.length).trim())
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

export function validateInventory(root) {
  const problems = []
  for (const entry of INVENTORY) {
    const path = resolve(root, entry.path)
    let source
    try {
      source = readFileSync(path, 'utf8')
    } catch {
      problems.push(`inventory path missing: ${entry.path}`)
      continue
    }
    if (!source.includes(entry.symbol)) problems.push(`inventory symbol missing in ${entry.path}: ${entry.symbol}`)
  }
  return problems
}

export function countQueueOwners(root) {
  const hits = []
  const walk = (directory) => {
    for (const name of readdirSync(directory)) {
      const path = join(directory, name)
      const stats = statSync(path)
      if (stats.isDirectory()) {
        if (name === 'node_modules' || name === '.git' || name === 'artifacts') continue
        walk(path)
        continue
      }
      if (!name.endsWith('.go') || name.endsWith('_test.go')) continue
      if (name === 'inventory.go') continue
      if (readFileSync(path, 'utf8').includes('type PriorityQueue')) hits.push(path.slice(resolve(root).length + 1))
    }
  }
  walk(resolve(root, 'backend'))
  return hits.sort()
}

export function validateSingleQueueOwner(root) {
  const hits = countQueueOwners(root)
  if (hits.length !== 2) return [`priority queue declarations=${hits.length} want 2 (alias + implementation): ${hits.join(',')}`]
  return []
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultFiles = Array.isArray(evidence?.defaultFiles) ? evidence.defaultFiles : []
  const taggedFiles = Array.isArray(evidence?.taggedFiles) ? evidence.taggedFiles : []
  for (const file of GOVERNANCE_FILES) {
    if (defaultFiles.includes(file)) problems.push(`default build leaked ${file}`)
    if (!taggedFiles.includes(file)) problems.push(`tagged build missing ${file}`)
  }
  return problems
}

export function validateAssertSource(text) {
  const guards = [
    'if _, err := governor.Adopt(ctx, spec); !errors.Is(err, ErrLeaseDuplicate) {',
    'if strings.Join(closed, ",") != "third,second,first" {',
    'if !errors.Is(err, failureA) || !errors.Is(err, failureB) {',
    'lease, ok := governor.Lookup(string(kind))',
    'if _, err := governor.Adopt(cancelled, adoptSpec("late", ManagedOutbox, systemOwner(), noopClose)); !errors.Is(err, context.Canceled) {'
  ]
  const body = String(text ?? '')
  return guards.filter((guard) => !body.includes(guard)).map((guard) => `governance test is missing ${guard}`)
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `governance test is missing ${name}`)
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
  const guards = CASES.length + 5
  const lines = [
    `invocation=governance package=${PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} queue_owners=${evidence?.queueOwners ?? 0}`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s01 ${row}`)
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
    tests: [...parseGoListFiles(tests), ...parseGoListFiles(external)].sort()
  }
}

export function runGovernanceEvidence(root) {
  const defaultList = goList(root, [])
  const taggedList = goList(root, ['-tags', BUILD_TAG])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'resource', 'governance_test.go'), 'utf8')
  const result = spawnSync('go', ['test', ...GO_TEST_FLAGS, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 900000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    testSource,
    rows: parseRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
    race: GO_TEST_FLAGS.includes('-race'),
    defaultFiles: defaultList.files,
    taggedFiles: taggedList.files,
    queueOwners: countQueueOwners(root).length,
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runGovernanceEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateInventory(root),
    ...validateSingleQueueOwner(root),
    ...validateAssertSource(evidence.testSource),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_GOVERNANCE)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
