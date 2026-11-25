import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/routee2e'
export const BUILD_TAGS = ['rsi_coreadapter', 'rsi_routee2e']
export const RUN_PATTERN = 'TestRouteE2E'
export const EXPECTED_CASES = 4
export const EXPECTED_PLUGIN_RECEIPTS = 1
export const EXPECTED_FALLBACKS = 3
export const EXPECTED_DENIALS = 2
export const EXPECTED_COMMITS = 0
export const EXPECTED_PASSED = 5
export const PLUGIN_PROVIDER_ID = 'com.amitia/emote/ai'
export const CORE_PROVIDER_ID = 'builtin.core/core/memory.search'
export const EXIT_ROUTE_E2E = 34

const CASE_PATTERN = /route e2e case=([a-z-]+) provider=(\S+) hops=(\S+) fallback=(true|false)(.*)$/
const SUMMARY_PATTERN = /route e2e summary cases=(\d+) pluginReceipts=(\d+) fallbacks=(\d+) denials=(\d+) commits=(\d+) digest=(\w+) pid=(\d+)/

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function parseCaseSummaries(log) {
  const body = typeof log === 'string' ? log : ''
  const cases = []
  for (const line of body.split('\n')) {
    const match = line.match(CASE_PATTERN)
    if (!match) continue
    const tail = match[5] ?? ''
    cases.push({
      name: match[1],
      provider: match[2],
      hops: match[3],
      fallback: match[4] === 'true',
      callbacks: numberFrom(tail, /callbacks=(\d+)/),
      attempts: numberFrom(tail, /attempts=(\d+)/),
      deniedWrites: numberFrom(tail, /deniedWrites=(\d+)/),
      deniedScopes: numberFrom(tail, /deniedScopes=(\d+)/),
      commits: numberFrom(tail, /commits=(\d+)/),
      pid: numberFrom(tail, /pid=(\d+)/),
      error: quotedFrom(tail)
    })
  }
  return cases
}

export function parseSummary(log) {
  const body = typeof log === 'string' ? log : ''
  const match = body.match(SUMMARY_PATTERN)
  if (!match) return null
  return {
    cases: Number(match[1]),
    pluginReceipts: Number(match[2]),
    fallbacks: Number(match[3]),
    denials: Number(match[4]),
    commits: Number(match[5]),
    digest: match[6],
    pid: Number(match[7])
  }
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
  if (!taggedFiles.includes('route_e2e.go')) problems.push('tagged build is missing route_e2e.go')
  if (!taggedFiles.includes('doc.go')) problems.push('tagged build dropped doc.go')
  if (!taggedTests.includes('route_e2e_test.go')) problems.push('tagged tests are missing route_e2e_test.go')
  if (!taggedTests.includes('helper_process_test.go')) problems.push('tagged tests are missing helper_process_test.go')

  const cases = Array.isArray(evidence?.cases) ? evidence.cases : []
  const names = cases.map((entry) => entry.name).join(',')
  if (names !== 'plugin-receipt,crash-fallback,denied-write,cross-scope') {
    problems.push(`cases=[${names}] want plugin-receipt,crash-fallback,denied-write,cross-scope`)
  }
  const receipt = cases.find((entry) => entry.name === 'plugin-receipt')
  if (!receipt) {
    problems.push('plugin-receipt case is missing')
  } else {
    if (receipt.provider !== PLUGIN_PROVIDER_ID) problems.push(`plugin-receipt provider=${receipt.provider} want ${PLUGIN_PROVIDER_ID}`)
    if (receipt.hops !== 'core>plugin>core') problems.push(`plugin-receipt hops=${receipt.hops} want core>plugin>core`)
    if (receipt.fallback) problems.push('plugin-receipt must not report fallback')
    if (receipt.callbacks !== 1) problems.push(`plugin-receipt callbacks=${receipt.callbacks} want 1`)
    if (!receipt.pid) problems.push('plugin-receipt did not report the plugin pid')
  }
  for (const name of ['crash-fallback', 'denied-write', 'cross-scope']) {
    const entry = cases.find((row) => row.name === name)
    if (!entry) {
      problems.push(`${name} case is missing`)
      continue
    }
    if (entry.provider !== CORE_PROVIDER_ID) problems.push(`${name} provider=${entry.provider} want ${CORE_PROVIDER_ID}`)
    if (!entry.fallback) problems.push(`${name} must report fallback`)
    if (entry.hops !== 'core') problems.push(`${name} hops=${entry.hops} want core`)
    if (entry.commits !== EXPECTED_COMMITS) problems.push(`${name} commits=${entry.commits} want ${EXPECTED_COMMITS}`)
  }
  const crash = cases.find((entry) => entry.name === 'crash-fallback')
  if (crash && !crash.error.includes('transport closed')) {
    problems.push(`crash-fallback error=${JSON.stringify(crash.error)} want original transport error`)
  }
  if (crash && crash.attempts !== 2) problems.push(`crash-fallback attempts=${crash.attempts} want 2`)
  const denied = cases.find((entry) => entry.name === 'denied-write')
  if (denied) {
    if (denied.deniedWrites !== 1) problems.push(`denied-write deniedWrites=${denied.deniedWrites} want 1`)
    if (!denied.error.includes('authoritative writes are not permitted')) {
      problems.push(`denied-write error=${JSON.stringify(denied.error)} want the core refusal`)
    }
  }
  const crossScope = cases.find((entry) => entry.name === 'cross-scope')
  if (crossScope && crossScope.deniedScopes !== 1) {
    problems.push(`cross-scope deniedScopes=${crossScope.deniedScopes} want 1`)
  }

  const summary = evidence?.summary
  if (!summary) {
    problems.push('summary line is missing')
  } else {
    if (summary.cases !== EXPECTED_CASES) problems.push(`summary cases=${summary.cases} want ${EXPECTED_CASES}`)
    if (summary.pluginReceipts !== EXPECTED_PLUGIN_RECEIPTS) problems.push(`pluginReceipts=${summary.pluginReceipts} want ${EXPECTED_PLUGIN_RECEIPTS}`)
    if (summary.fallbacks !== EXPECTED_FALLBACKS) problems.push(`fallbacks=${summary.fallbacks} want ${EXPECTED_FALLBACKS}`)
    if (summary.denials !== EXPECTED_DENIALS) problems.push(`denials=${summary.denials} want ${EXPECTED_DENIALS}`)
    if (summary.commits !== EXPECTED_COMMITS) problems.push(`commits=${summary.commits} want ${EXPECTED_COMMITS}`)
    if (summary.digest !== 'unchanged') problems.push(`digest=${summary.digest} want unchanged`)
    if (receipt && receipt.pid && summary.pid && receipt.pid === summary.pid) {
      problems.push('plugin ran inside the core process; the route is not crossing a process boundary')
    }
  }
  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  return problems
}

export function renderEvidence(evidence) {
  const cases = Array.isArray(evidence?.cases) ? evidence.cases : []
  const summary = evidence?.summary ?? {}
  return [
    `route=e2e tags=${BUILD_TAGS.join(',')} package=${PACKAGE_PATH}`,
    `isolation: default=[${(evidence?.defaultFiles ?? []).join(' ')}] tagged=[${(evidence?.taggedFiles ?? []).join(' ')}] tests=[${(evidence?.taggedTests ?? []).join(' ')}]`,
    ...cases.map(
      (entry) =>
        `case=${entry.name} provider=${entry.provider} hops=${entry.hops} fallback=${entry.fallback} commits=${entry.commits ?? 0}`
    ),
    `summary cases=${summary.cases ?? 0} pluginReceipts=${summary.pluginReceipts ?? 0} fallbacks=${summary.fallbacks ?? 0} denials=${summary.denials ?? 0} commits=${summary.commits ?? 0} digest=${summary.digest ?? 'unknown'}`,
    `processes corePid=${summary.pid ?? 0} pluginPid=${(cases.find((entry) => entry.name === 'plugin-receipt') ?? {}).pid ?? 0}`,
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1}`
  ].join('\n') + '\n'
}

function numberFrom(text, pattern) {
  const match = text.match(pattern)
  return match ? Number(match[1]) : null
}

function quotedFrom(text) {
  const match = text.match(/error="((?:[^"\\]|\\.)*)"/)
  return match ? match[1].replace(/\\(.)/g, '$1') : ''
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

export function runRouteE2E(root) {
  const tags = BUILD_TAGS.join(',')
  const base = goList(root, [])
  const tagged = goList(root, ['-tags', tags])
  const result = spawnSync('go', ['test', '-tags', tags, '-count=1', '-timeout', '180s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
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
    cases: parseCaseSummaries(log),
    summary: parseSummary(log),
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
      process.stdout.write(`route=e2e tags=${BUILD_TAGS.join(',')} package=${PACKAGE_PATH} cases=${EXPECTED_CASES}\n`)
      return
    }
    const evidence = runRouteE2E(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_ROUTE_E2E)
    process.stdout.write(`go test passed=${evidence.passed} failed=${evidence.failed} exit=${evidence.exitCode}\n`)
    if (evidence.exitCode !== 0) process.exit(EXIT_ROUTE_E2E)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_ROUTE_E2E)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
