import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/routecache'
export const BUILD_TAG = 'rsi_routecache'
export const RUN_PATTERN = 'TestRouteCache'
export const EXPECTED_LOOKUPS = 1000
export const EXPECTED_GRAPH_LOADS = 1
export const EXPECTED_COMPUTATIONS = 1
export const EXPECTED_CONCURRENCY = 100
export const EXPECTED_MIXED = 0
export const EXPECTED_INVALIDATED = 1
export const EXPECTED_CARRIED = 1
export const EXPECTED_TIERS = [0, 1, 3, 5, 10]
export const EXPECTED_REALSCALE = [56, 57]
export const EXPECTED_PASSED = 9
export const EXIT_ROUTE_CACHE = 33

const LOOKUPS_PATTERN = /route cache evidence lookups=(\d+) graphLoads=(\d+) computations=(\d+)/
const CONCURRENCY_PATTERN = /route cache evidence concurrent=(\d+) revisions=(\d+) mixed=(\d+)/
const INVALIDATION_PATTERN = /route cache evidence invalidated=(\d+) carried=(\d+) recomputed=(\d+)/
const TIER_PATTERN = /route cache tier plugins=(\d+) providers=(\d+) graphLoads=(\d+)/g
const REALSCALE_PATTERN = /route cache realscale providers=(\d+) graphLoads=(\d+) lookups=(\d+)/g

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function parseLookupSummary(log) {
  const body = typeof log === 'string' ? log : ''
  const match = body.match(LOOKUPS_PATTERN)
  if (!match) return null
  return { lookups: Number(match[1]), graphLoads: Number(match[2]), computations: Number(match[3]) }
}

export function parseConcurrencySummary(log) {
  const body = typeof log === 'string' ? log : ''
  const match = body.match(CONCURRENCY_PATTERN)
  if (!match) return null
  return { concurrent: Number(match[1]), revisions: Number(match[2]), mixed: Number(match[3]) }
}

export function parseInvalidationSummary(log) {
  const body = typeof log === 'string' ? log : ''
  const match = body.match(INVALIDATION_PATTERN)
  if (!match) return null
  return { invalidated: Number(match[1]), carried: Number(match[2]), recomputed: Number(match[3]) }
}

export function parseTierSummaries(log) {
  const body = typeof log === 'string' ? log : ''
  const tiers = []
  for (const match of body.matchAll(TIER_PATTERN)) {
    tiers.push({ plugins: Number(match[1]), providers: Number(match[2]), graphLoads: Number(match[3]) })
  }
  return tiers
}

export function parseRealScaleSummaries(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const match of body.matchAll(REALSCALE_PATTERN)) {
    rows.push({ providers: Number(match[1]), graphLoads: Number(match[2]), lookups: Number(match[3]) })
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

export function validateEvidence(evidence) {
  const problems = []
  const defaultFiles = Array.isArray(evidence?.defaultFiles) ? evidence.defaultFiles : []
  const taggedFiles = Array.isArray(evidence?.taggedFiles) ? evidence.taggedFiles : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultFiles.length !== 1 || defaultFiles[0] !== 'doc.go') {
    problems.push(`default build files=${defaultFiles.join(',')} want doc.go only`)
  }
  if (!taggedFiles.includes('route_cache.go')) problems.push('tagged build is missing route_cache.go')
  if (!taggedFiles.includes('doc.go')) problems.push('tagged build dropped doc.go')
  if (!taggedTests.includes('route_cache_test.go')) problems.push('tagged tests are missing route_cache_test.go')
  const lookups = evidence?.lookups
  if (!lookups) {
    problems.push('lookup summary is missing')
  } else {
    if (lookups.lookups !== EXPECTED_LOOKUPS) problems.push(`lookups=${lookups.lookups} want ${EXPECTED_LOOKUPS}`)
    if (lookups.graphLoads !== EXPECTED_GRAPH_LOADS) problems.push(`graphLoads=${lookups.graphLoads} want ${EXPECTED_GRAPH_LOADS}`)
    if (lookups.computations !== EXPECTED_COMPUTATIONS) problems.push(`computations=${lookups.computations} want ${EXPECTED_COMPUTATIONS}`)
  }
  const concurrency = evidence?.concurrency
  if (!concurrency) {
    problems.push('concurrency summary is missing')
  } else {
    if (concurrency.concurrent !== EXPECTED_CONCURRENCY) problems.push(`concurrent=${concurrency.concurrent} want ${EXPECTED_CONCURRENCY}`)
    if (concurrency.revisions < 2) problems.push(`revisions=${concurrency.revisions} want at least 2`)
    if (concurrency.mixed !== EXPECTED_MIXED) problems.push(`mixed=${concurrency.mixed} want ${EXPECTED_MIXED}`)
  }
  const invalidation = evidence?.invalidation
  if (!invalidation) {
    problems.push('invalidation summary is missing')
  } else {
    if (invalidation.invalidated !== EXPECTED_INVALIDATED) problems.push(`invalidated=${invalidation.invalidated} want ${EXPECTED_INVALIDATED}`)
    if (invalidation.carried !== EXPECTED_CARRIED) problems.push(`carried=${invalidation.carried} want ${EXPECTED_CARRIED}`)
  }
  const tiers = Array.isArray(evidence?.tiers) ? evidence.tiers : []
  const observed = tiers.map((tier) => tier.plugins)
  if (observed.join(',') !== EXPECTED_TIERS.join(',')) {
    problems.push(`tiers=[${observed.join(',')}] want [${EXPECTED_TIERS.join(',')}]`)
  } else {
    for (const tier of tiers) {
      if (tier.graphLoads !== 1) problems.push(`tier plugins=${tier.plugins} graphLoads=${tier.graphLoads} want 1`)
      if (tier.providers !== tier.plugins) problems.push(`tier plugins=${tier.plugins} providers=${tier.providers} want equal`)
    }
  }
  const realscale = Array.isArray(evidence?.realscale) ? evidence.realscale : []
  const realProviders = realscale.map((row) => row.providers)
  if (realProviders.join(',') !== EXPECTED_REALSCALE.join(',')) {
    problems.push(`realscale=[${realProviders.join(',')}] want [${EXPECTED_REALSCALE.join(',')}]`)
  } else {
    for (const row of realscale) {
      if (row.graphLoads !== 1) problems.push(`realscale providers=${row.providers} graphLoads=${row.graphLoads} want 1`)
      if (row.lookups !== EXPECTED_LOOKUPS) problems.push(`realscale providers=${row.providers} lookups=${row.lookups} want ${EXPECTED_LOOKUPS}`)
    }
  }
  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  return problems
}

export function renderEvidence(evidence) {
  const lookups = evidence?.lookups ?? {}
  const concurrency = evidence?.concurrency ?? {}
  const invalidation = evidence?.invalidation ?? {}
  const tiers = Array.isArray(evidence?.tiers) ? evidence.tiers : []
  const realscale = Array.isArray(evidence?.realscale) ? evidence.realscale : []
  return [
    `cache=route tag=${BUILD_TAG} package=${PACKAGE_PATH}`,
    `isolation: default=[${(evidence?.defaultFiles ?? []).join(' ')}] tagged=[${(evidence?.taggedFiles ?? []).join(' ')}] tests=[${(evidence?.taggedTests ?? []).join(' ')}]`,
    `lookups=${lookups.lookups ?? 0} graphLoads=${lookups.graphLoads ?? 0} computations=${lookups.computations ?? 0}`,
    `concurrent=${concurrency.concurrent ?? 0} revisions=${concurrency.revisions ?? 0} mixed=${concurrency.mixed ?? 0}`,
    `invalidated=${invalidation.invalidated ?? 0} carried=${invalidation.carried ?? 0} recomputed=${invalidation.recomputed ?? 0}`,
    `tiers=[${tiers.map((tier) => tier.plugins).join(',')}] tierGraphLoads=[${tiers.map((tier) => tier.graphLoads).join(',')}]`,
    `realscale=[${realscale.map((row) => row.providers).join(',')}] realscaleGraphLoads=[${realscale.map((row) => row.graphLoads).join(',')}]`,
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1}`
  ].join('\n') + '\n'
}

function goList(root, extraArgs) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}', ...extraArgs, PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 120000
  })
  const line = `${result.stdout ?? ''}`.trim()
  const [files = '', tests = ''] = line.split('|')
  return { files: parseGoListFiles(files), tests: parseGoListFiles(tests) }
}

export function runRouteCache(root) {
  const base = goList(root, [])
  const tagged = goList(root, ['-tags', BUILD_TAG])
  const result = spawnSync('go', ['test', '-tags', BUILD_TAG, '-count=1', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 600000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    defaultFiles: base.files,
    taggedFiles: tagged.files,
    taggedTests: tagged.tests,
    lookups: parseLookupSummary(log),
    concurrency: parseConcurrencySummary(log),
    invalidation: parseInvalidationSummary(log),
    tiers: parseTierSummaries(log),
    realscale: parseRealScaleSummaries(log),
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
      process.stdout.write(`cache=route tag=${BUILD_TAG} package=${PACKAGE_PATH} tiers=${EXPECTED_TIERS.join('/')}\n`)
      return
    }
    const evidence = runRouteCache(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_ROUTE_CACHE)
    process.stdout.write(`go test passed=${evidence.passed} failed=${evidence.failed} exit=${evidence.exitCode}\n`)
    if (evidence.exitCode !== 0) process.exit(EXIT_ROUTE_CACHE)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_ROUTE_CACHE)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
