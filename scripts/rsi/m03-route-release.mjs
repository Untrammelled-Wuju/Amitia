import { spawnSync } from 'node:child_process'
import { existsSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/routeconformance'
export const BUILD_TAG = 'rsi_routeconformance'
export const ROUTING_PACKAGES = [
  { name: 'coreadapter', path: './internal/extension/kernel/coreadapter', tag: 'rsi_coreadapter', entry: 'memory_search_adapter.go' },
  { name: 'pluginadapter', path: './internal/extension/kernel/pluginadapter', tag: 'rsi_pluginadapter', entry: 'plugin_adapter.go' },
  { name: 'routecache', path: './internal/extension/kernel/routecache', tag: 'rsi_routecache', entry: 'route_cache.go' },
  { name: 'routee2e', path: './internal/extension/kernel/routee2e', tag: 'rsi_routee2e', entry: 'route_e2e.go' },
  { name: 'routeconformance', path: PACKAGE_PATH, tag: BUILD_TAG, entry: 'contract.go' }
]
export const ROUTING_EVIDENCE = [
  'm03-s06-route-cache.log',
  'm03-s06-kernel-suite.log',
  'm03-s07-route-e2e.log',
  'm03-s07-kernel-suite.log'
]
export const EXPECTED_CONTRACT = { bindingFields: 3, keyFields: 4, requestFields: 3, scopes: 3, methods: 7 }
export const EXPECTED_PERMISSION = { rows: 10, allowed: 6, denied: 4 }
export const EXPECTED_SELECTION = { providers: 64, candidates: 64, selections: 5000 }
export const EXPECTED_CONCURRENCY = { goroutines: 128, iterations: 200 }
export const MAX_ALLOCS_PER_OP = 200
export const MAX_SELECTION_MS = 5000
export const EXPECTED_PASSED = 5
export const EXIT_ROUTE_RELEASE = 35

const CONTRACT_PATTERN = /route conformance contract bindingFields=(\d+) keyFields=(\d+) requestFields=(\d+) scopes=(\d+) methods=(\d+) golden=(\w+)/
const PERMISSION_PATTERN = /route conformance permission rows=(\d+) allowed=(\d+) denied=(\d+) sentinels=(\w+)/
const SELECTION_PATTERN = /route conformance selection providers=(\d+) candidates=(\d+) selections=(\d+) elapsedMs=(\d+) allocsPerOp=(\d+) ordered=(\w+) deterministic=(\w+)/
const CONCURRENCY_PATTERN = /route conformance concurrency goroutines=(\d+) iterations=(\d+) mismatches=(\d+)/
const SNAPSHOT_PATTERN = /route conformance isolation bindings=(\d+) digest=(\w+) mutate=(\w+)/

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function parseContract(log) {
  const match = String(log ?? '').match(CONTRACT_PATTERN)
  if (!match) return null
  return {
    bindingFields: Number(match[1]),
    keyFields: Number(match[2]),
    requestFields: Number(match[3]),
    scopes: Number(match[4]),
    methods: Number(match[5]),
    golden: match[6]
  }
}

export function parsePermission(log) {
  const match = String(log ?? '').match(PERMISSION_PATTERN)
  if (!match) return null
  return { rows: Number(match[1]), allowed: Number(match[2]), denied: Number(match[3]), sentinels: match[4] }
}

export function parseSelection(log) {
  const match = String(log ?? '').match(SELECTION_PATTERN)
  if (!match) return null
  return {
    providers: Number(match[1]),
    candidates: Number(match[2]),
    selections: Number(match[3]),
    elapsedMs: Number(match[4]),
    allocsPerOp: Number(match[5]),
    ordered: match[6],
    deterministic: match[7]
  }
}

export function parseConcurrency(log) {
  const match = String(log ?? '').match(CONCURRENCY_PATTERN)
  if (!match) return null
  return { goroutines: Number(match[1]), iterations: Number(match[2]), mismatches: Number(match[3]) }
}

export function parseSnapshot(log) {
  const match = String(log ?? '').match(SNAPSHOT_PATTERN)
  if (!match) return null
  return { bindings: Number(match[1]), digest: match[2], mutate: match[3] }
}

export function parseGoListFiles(output) {
  const body = String(output ?? '').trim()
  if (!body.startsWith('[') || !body.endsWith(']')) return []
  const inner = body.slice(1, -1).trim()
  if (inner === '') return []
  return inner.split(/\s+/).sort()
}

export function validateEvidence(evidence) {
  const problems = []
  const packages = Array.isArray(evidence?.packages) ? evidence.packages : []
  if (packages.length !== ROUTING_PACKAGES.length) {
    problems.push(`packages=${packages.length} want ${ROUTING_PACKAGES.length}`)
  }
  for (const record of packages) {
    if (record.defaultFiles.length !== 1 || record.defaultFiles[0] !== 'doc.go') {
      problems.push(`${record.name} default build files=${record.defaultFiles.join(',')} want doc.go only`)
    }
    if (record.defaultTests.length !== 0) {
      problems.push(`${record.name} default test files=${record.defaultTests.join(',')} want empty`)
    }
    if (!record.taggedFiles.includes(record.entry)) {
      problems.push(`${record.name} tagged build is missing ${record.entry}`)
    }
  }
  for (const record of packages) {
    if (record.name !== 'routeconformance') continue
    if (!record.taggedFiles.includes('contract.go')) problems.push('routeconformance tagged build is missing contract.go')
    if (!record.taggedFiles.includes('permission.go')) problems.push('routeconformance tagged build is missing permission.go')
    if (!record.taggedFiles.includes('selection.go')) problems.push('routeconformance tagged build is missing selection.go')
    if (!record.taggedFiles.includes('doc.go')) problems.push('routeconformance tagged build dropped doc.go')
    if (!record.taggedTests.includes('route_conformance_test.go')) problems.push('routeconformance tagged tests are missing route_conformance_test.go')
  }

  const contract = evidence?.contract
  if (!contract) {
    problems.push('contract line is missing')
  } else {
    for (const key of Object.keys(EXPECTED_CONTRACT)) {
      if (contract[key] !== EXPECTED_CONTRACT[key]) problems.push(`contract ${key}=${contract[key]} want ${EXPECTED_CONTRACT[key]}`)
    }
    if (contract.golden !== 'ok') problems.push(`contract golden=${contract.golden} want ok`)
  }

  const permission = evidence?.permission
  if (!permission) {
    problems.push('permission line is missing')
  } else {
    for (const key of Object.keys(EXPECTED_PERMISSION)) {
      if (permission[key] !== EXPECTED_PERMISSION[key]) problems.push(`permission ${key}=${permission[key]} want ${EXPECTED_PERMISSION[key]}`)
    }
    if (permission.sentinels !== 'ok') problems.push(`permission sentinels=${permission.sentinels} want ok`)
  }

  const selection = evidence?.selection
  if (!selection) {
    problems.push('selection line is missing')
  } else {
    if (selection.providers !== EXPECTED_SELECTION.providers) problems.push(`selection providers=${selection.providers} want ${EXPECTED_SELECTION.providers}`)
    if (selection.candidates !== EXPECTED_SELECTION.candidates) problems.push(`selection candidates=${selection.candidates} want ${EXPECTED_SELECTION.candidates}`)
    if (selection.selections !== EXPECTED_SELECTION.selections) problems.push(`selection selections=${selection.selections} want ${EXPECTED_SELECTION.selections}`)
    if (selection.ordered !== 'ok') problems.push(`selection ordered=${selection.ordered} want ok`)
    if (selection.deterministic !== 'ok') problems.push(`selection deterministic=${selection.deterministic} want ok`)
    if (selection.allocsPerOp > MAX_ALLOCS_PER_OP) problems.push(`selection allocsPerOp=${selection.allocsPerOp} want <= ${MAX_ALLOCS_PER_OP}`)
    if (selection.elapsedMs > MAX_SELECTION_MS) problems.push(`selection elapsedMs=${selection.elapsedMs} want <= ${MAX_SELECTION_MS}`)
  }

  const concurrency = evidence?.concurrency
  if (!concurrency) {
    problems.push('concurrency line is missing')
  } else {
    if (concurrency.goroutines !== EXPECTED_CONCURRENCY.goroutines) problems.push(`concurrency goroutines=${concurrency.goroutines} want ${EXPECTED_CONCURRENCY.goroutines}`)
    if (concurrency.iterations !== EXPECTED_CONCURRENCY.iterations) problems.push(`concurrency iterations=${concurrency.iterations} want ${EXPECTED_CONCURRENCY.iterations}`)
    if (concurrency.mismatches !== 0) problems.push(`concurrency mismatches=${concurrency.mismatches} want 0`)
  }

  const snapshot = evidence?.snapshot
  if (!snapshot) {
    problems.push('snapshot isolation line is missing')
  } else {
    if (snapshot.bindings !== 1) problems.push(`snapshot bindings=${snapshot.bindings} want 1`)
    if (snapshot.digest !== 'unchanged') problems.push(`snapshot digest=${snapshot.digest} want unchanged`)
    if (snapshot.mutate !== 'blocked') problems.push(`snapshot mutate=${snapshot.mutate} want blocked`)
  }

  const evidenceFiles = Array.isArray(evidence?.evidenceFiles) ? evidence.evidenceFiles : []
  for (const name of ROUTING_EVIDENCE) {
    const entry = evidenceFiles.find((row) => row.name === name)
    if (!entry || !entry.present || entry.size <= 0) problems.push(`routing evidence ${name} is missing or empty`)
  }
  if (evidence?.race !== true) problems.push('race detector was not enabled')
  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  return problems
}

export function renderEvidence(evidence) {
  const contract = evidence?.contract ?? {}
  const permission = evidence?.permission ?? {}
  const selection = evidence?.selection ?? {}
  const concurrency = evidence?.concurrency ?? {}
  const snapshot = evidence?.snapshot ?? {}
  const packages = Array.isArray(evidence?.packages) ? evidence.packages : []
  const evidenceFiles = Array.isArray(evidence?.evidenceFiles) ? evidence.evidenceFiles : []
  const missing = evidenceFiles.filter((row) => !row.present || row.size <= 0).length
  return [
    `route=release tag=${BUILD_TAG} package=${PACKAGE_PATH}`,
    `isolation: packages=[${packages.map((row) => row.name).join(',')}] defaultDocOnly=${packages.every((row) => row.defaultFiles.length === 1 && row.defaultFiles[0] === 'doc.go')}`,
    `contract bindingFields=${contract.bindingFields ?? 0} keyFields=${contract.keyFields ?? 0} requestFields=${contract.requestFields ?? 0} scopes=${contract.scopes ?? 0} methods=${contract.methods ?? 0} golden=${contract.golden ?? 'unknown'}`,
    `permission rows=${permission.rows ?? 0} allowed=${permission.allowed ?? 0} denied=${permission.denied ?? 0} sentinels=${permission.sentinels ?? 'unknown'}`,
    `selection providers=${selection.providers ?? 0} candidates=${selection.candidates ?? 0} selections=${selection.selections ?? 0} elapsedMs=${selection.elapsedMs ?? 0} allocsPerOp=${selection.allocsPerOp ?? 0} ordered=${selection.ordered ?? 'unknown'} deterministic=${selection.deterministic ?? 'unknown'}`,
    `concurrency goroutines=${concurrency.goroutines ?? 0} iterations=${concurrency.iterations ?? 0} mismatches=${concurrency.mismatches ?? 0}`,
    `snapshot-isolation bindings=${snapshot.bindings ?? 0} digest=${snapshot.digest ?? 'unknown'} mutate=${snapshot.mutate ?? 'unknown'}`,
    `evidence ${ROUTING_EVIDENCE.map((name) => `${name}=${evidenceFiles.some((row) => row.name === name && row.present) ? 'present' : 'missing'}`).join(' ')} missing=${missing}`,
    `race enabled=${evidence?.race === true}`,
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1}`
  ].join('\n') + '\n'
}

function goList(root, packagePath, extraArgs) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}', ...extraArgs, packagePath], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 120000
  })
  const line = `${result.stdout ?? ''}`.trim()
  const [files = '', tests = ''] = line.split('|')
  return { files: parseGoListFiles(files), tests: parseGoListFiles(tests) }
}

export function runRouteRelease(root) {
  const packages = ROUTING_PACKAGES.map((record) => {
    const base = goList(root, record.path, [])
    const tagged = goList(root, record.path, ['-tags', record.tag])
    return {
      name: record.name,
      entry: record.entry,
      defaultFiles: base.files,
      defaultTests: base.tests,
      taggedFiles: tagged.files,
      taggedTests: tagged.tests
    }
  })
  const result = spawnSync(
    'go',
    ['test', '-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '300s', '-v', PACKAGE_PATH],
    {
      cwd: resolve(root, 'backend'),
      env: cleanEnv(),
      encoding: 'utf8',
      timeout: 900000
    }
  )
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  const evidenceFiles = ROUTING_EVIDENCE.map((name) => {
    const path = resolve(root, 'artifacts/rsi', name)
    const present = existsSync(path)
    return { name, present, size: present ? statSync(path).size : 0 }
  })
  return {
    packages,
    contract: parseContract(log),
    permission: parsePermission(log),
    selection: parseSelection(log),
    concurrency: parseConcurrency(log),
    snapshot: parseSnapshot(log),
    evidenceFiles,
    race: result.status === 0 && !log.includes('WARNING: DATA RACE'),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status,
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
      process.stdout.write(`route=release tag=${BUILD_TAG} package=${PACKAGE_PATH} packages=${ROUTING_PACKAGES.length}\n`)
      return
    }
    const evidence = runRouteRelease(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_ROUTE_RELEASE)
    process.stdout.write(`go test passed=${evidence.passed} failed=${evidence.failed} exit=${evidence.exitCode}\n`)
    if (evidence.exitCode !== 0) process.exit(EXIT_ROUTE_RELEASE)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_ROUTE_RELEASE)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
