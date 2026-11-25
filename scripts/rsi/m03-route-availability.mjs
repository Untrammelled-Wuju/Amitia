import { spawnSync } from 'node:child_process'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestRouteAvailability'
export const EXIT_ROUTE_AVAILABILITY = 38
export const EXPECTED_PASSED = 8
export const REQUIRED_SOURCES = ['route_availability.go', 'route_policy.go']
export const REQUIRED_TESTS = ['route_availability_test.go']
export const AVAILABILITY_CASES = [
  'core-missing',
  'core-implemented',
  'plugin-healthy',
  'plugin-crashed',
  'no-fake-fallback',
  'plugin-only-crash',
  'core-health-ignored',
  'invalid-policy'
]
export const CORE_PROVIDER_ID = 'builtin.core/core/memory.search'
export const PLUGIN_PROVIDER_ID = 'plugin.host'

const CORE_MISSING_PATTERN = /route availability core missing service=(\S+) provider=(\S+) class=(\w+) fallbackBlocked=(true|false) candidates=(\d+)/
const CORE_IMPLEMENTED_PATTERN = /route availability core implemented service=(\S+) provider=(\S+) class=(\w+) implementations=(\d+)/
const PLUGIN_HEALTHY_PATTERN = /route availability plugin healthy service=(\S+) provider=(\S+) class=(\w+) probes=(\d+)/
const PLUGIN_CRASHED_PATTERN = /route availability plugin crashed service=(\S+) provider=(\S+) class=(\w+) error=(\w+)/
const NO_FALLBACK_PATTERN = /route availability no fake fallback mode=(\S+) bound=(\S+) selected=(\S+) pluginCandidates=(\d+) fallbackBlocked=(true|false)/
const PLUGIN_ONLY_CRASH_PATTERN = /route availability plugin only crashed mode=(\S+) provider=(\S+) class=(\w+) skippedCore=(\d+)/
const CORE_HEALTH_PATTERN = /route availability core health ignored mode=(\S+) provider=(\S+) healthProbe=(\w+) class=(\w+) probeCalls=(\d+)/
const INVALID_PATTERN = /route availability invalid policy field=(\S+) value=(\S+) error=(\w+) class=(\S+)/

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

function parseNamed(text, pattern) {
  const match = text.match(pattern)
  if (!match) return null
  return match.slice(1).map((value) => (/^\d+$/.test(value) ? Number(value) : value))
}

export function parseAvailabilityRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    'core-missing': parseNamed(body, CORE_MISSING_PATTERN),
    'core-implemented': parseNamed(body, CORE_IMPLEMENTED_PATTERN),
    'plugin-healthy': parseNamed(body, PLUGIN_HEALTHY_PATTERN),
    'plugin-crashed': parseNamed(body, PLUGIN_CRASHED_PATTERN),
    'no-fake-fallback': parseNamed(body, NO_FALLBACK_PATTERN),
    'plugin-only-crash': parseNamed(body, PLUGIN_ONLY_CRASH_PATTERN),
    'core-health-ignored': parseNamed(body, CORE_HEALTH_PATTERN),
    'invalid-policy': parseNamed(body, INVALID_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'core-missing':
      return CORE_MISSING_PATTERN
    case 'core-implemented':
      return CORE_IMPLEMENTED_PATTERN
    case 'plugin-healthy':
      return PLUGIN_HEALTHY_PATTERN
    case 'plugin-crashed':
      return PLUGIN_CRASHED_PATTERN
    case 'no-fake-fallback':
      return NO_FALLBACK_PATTERN
    case 'plugin-only-crash':
      return PLUGIN_ONLY_CRASH_PATTERN
    case 'core-health-ignored':
      return CORE_HEALTH_PATTERN
    default:
      return INVALID_PATTERN
  }
}

function countMatches(text, pattern) {
  return (text.match(new RegExp(pattern.source, 'g')) ?? []).length
}

export function validateEvidence(evidence) {
  const problems = []
  const goFiles = Array.isArray(evidence?.goFiles) ? evidence.goFiles : []
  const testFiles = Array.isArray(evidence?.testFiles) ? evidence.testFiles : []
  for (const source of REQUIRED_SOURCES) {
    if (!goFiles.includes(source)) problems.push(`default build is missing ${source}`)
  }
  if (goFiles.some((file) => file.endsWith('_test.go'))) problems.push('default build leaked test files')
  for (const test of REQUIRED_TESTS) {
    if (!testFiles.includes(test)) problems.push(`test build is missing ${test}`)
  }

  const rows = evidence?.rows ?? {}
  const log = typeof evidence?.log === 'string' ? evidence.log : ''
  for (const name of AVAILABILITY_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  const coreMissing = rows['core-missing']
  if (Array.isArray(coreMissing)) {
    if (coreMissing[0] !== 'memory.search') problems.push(`core missing service=${coreMissing[0]} want memory.search`)
    if (coreMissing[1] !== CORE_PROVIDER_ID) problems.push(`core missing provider=${coreMissing[1]} want ${CORE_PROVIDER_ID}`)
    if (coreMissing[2] !== 'NO_BUILTIN') problems.push(`core missing class=${coreMissing[2]} want NO_BUILTIN`)
    if (coreMissing[3] !== 'true') problems.push('core missing must block the fallback')
    if (coreMissing[4] !== 2) problems.push(`core missing candidates=${coreMissing[4]} want 2`)
  }

  const coreImplemented = rows['core-implemented']
  if (Array.isArray(coreImplemented)) {
    if (coreImplemented[1] !== CORE_PROVIDER_ID) problems.push(`core implemented provider=${coreImplemented[1]} want core`)
    if (coreImplemented[2] !== 'NONE') problems.push(`core implemented class=${coreImplemented[2]} want NONE`)
    if (coreImplemented[3] !== 1) problems.push(`core implemented implementations=${coreImplemented[3]} want 1`)
  }

  const pluginHealthy = rows['plugin-healthy']
  if (Array.isArray(pluginHealthy)) {
    if (pluginHealthy[1] !== PLUGIN_PROVIDER_ID) problems.push(`plugin healthy provider=${pluginHealthy[1]} want ${PLUGIN_PROVIDER_ID}`)
    if (pluginHealthy[2] !== 'NONE') problems.push(`plugin healthy class=${pluginHealthy[2]} want NONE`)
    if (pluginHealthy[3] !== 1) problems.push(`plugin healthy probes=${pluginHealthy[3]} want 1`)
  }

  const pluginCrashed = rows['plugin-crashed']
  if (Array.isArray(pluginCrashed)) {
    if (pluginCrashed[1] !== PLUGIN_PROVIDER_ID) problems.push(`plugin crashed provider=${pluginCrashed[1]} want ${PLUGIN_PROVIDER_ID}`)
    if (pluginCrashed[2] !== 'UNAVAILABLE') problems.push(`plugin crashed class=${pluginCrashed[2]} want UNAVAILABLE`)
    if (pluginCrashed[3] !== 'ErrRouteUnavailable') problems.push(`plugin crashed error=${pluginCrashed[3]} want ErrRouteUnavailable`)
  }

  const noFallback = rows['no-fake-fallback']
  if (Array.isArray(noFallback)) {
    if (noFallback[0] !== 'auto') problems.push(`no fake fallback mode=${noFallback[0]} want auto`)
    if (noFallback[1] !== CORE_PROVIDER_ID || noFallback[2] !== CORE_PROVIDER_ID) {
      problems.push(`no fake fallback bound=${noFallback[1]} selected=${noFallback[2]} want core/core`)
    }
    if (noFallback[3] !== 1) problems.push(`no fake fallback pluginCandidates=${noFallback[3]} want 1`)
    if (noFallback[4] !== 'true') problems.push('no fake fallback must report fallbackBlocked=true')
  }

  const pluginOnly = rows['plugin-only-crash']
  if (Array.isArray(pluginOnly)) {
    if (pluginOnly[0] !== 'plugin_only') problems.push(`plugin only crash mode=${pluginOnly[0]} want plugin_only`)
    if (pluginOnly[2] !== 'UNAVAILABLE') problems.push(`plugin only crash class=${pluginOnly[2]} want UNAVAILABLE`)
    if (pluginOnly[3] !== 1) problems.push(`plugin only crash skippedCore=${pluginOnly[3]} want 1`)
  }

  const coreHealth = rows['core-health-ignored']
  if (Array.isArray(coreHealth)) {
    if (coreHealth[0] !== 'core_only') problems.push(`core health ignored mode=${coreHealth[0]} want core_only`)
    if (coreHealth[1] !== CORE_PROVIDER_ID) problems.push(`core health ignored provider=${coreHealth[1]} want core`)
    if (coreHealth[2] !== 'skipped') problems.push(`core health ignored probe=${coreHealth[2]} want skipped`)
    if (coreHealth[3] !== 'NONE') problems.push(`core health ignored class=${coreHealth[3]} want NONE`)
    if (coreHealth[4] !== 0) problems.push(`core health ignored probeCalls=${coreHealth[4]} want 0`)
  }

  const invalid = rows['invalid-policy']
  if (Array.isArray(invalid)) {
    if (invalid[0] !== 'mode' || invalid[1] !== 'hybrid') problems.push(`invalid policy field=${invalid[0]} value=${invalid[1]} want mode/hybrid`)
    if (invalid[2] !== 'ErrRoutePolicyInvalid') problems.push(`invalid policy error=${invalid[2]} want ErrRoutePolicyInvalid`)
    if (invalid[3] !== 'none') problems.push(`invalid policy class=${invalid[3]} want none`)
  }

  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  if (evidence?.race !== true) problems.push('go test must run with -race')
  return problems
}

export function renderEvidence(evidence) {
  const rows = evidence?.rows ?? {}
  const value = (name, index, fallback = 0) => {
    const row = rows[name]
    return Array.isArray(row) ? row[index] : fallback
  }
  const goFiles = evidence?.goFiles ?? []
  const testFiles = evidence?.testFiles ?? []
  return [
    `route=availability package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${AVAILABILITY_CASES.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((source) => goFiles.includes(source)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `route availability core missing service=${value('core-missing', 0, '')} provider=${value('core-missing', 1, '')} class=${value('core-missing', 2, '')} fallbackBlocked=${value('core-missing', 3, 'false')} candidates=${value('core-missing', 4)}`,
    `route availability core implemented service=${value('core-implemented', 0, '')} provider=${value('core-implemented', 1, '')} class=${value('core-implemented', 2, '')} implementations=${value('core-implemented', 3)}`,
    `route availability plugin healthy service=${value('plugin-healthy', 0, '')} provider=${value('plugin-healthy', 1, '')} class=${value('plugin-healthy', 2, '')} probes=${value('plugin-healthy', 3)}`,
    `route availability plugin crashed service=${value('plugin-crashed', 0, '')} provider=${value('plugin-crashed', 1, '')} class=${value('plugin-crashed', 2, '')} error=${value('plugin-crashed', 3, '')}`,
    `route availability no fake fallback mode=${value('no-fake-fallback', 0, '')} bound=${value('no-fake-fallback', 1, '')} selected=${value('no-fake-fallback', 2, '')} pluginCandidates=${value('no-fake-fallback', 3)} fallbackBlocked=${value('no-fake-fallback', 4, 'false')}`,
    `route availability plugin only crashed mode=${value('plugin-only-crash', 0, '')} provider=${value('plugin-only-crash', 1, '')} class=${value('plugin-only-crash', 2, '')} skippedCore=${value('plugin-only-crash', 3)}`,
    `route availability core health ignored mode=${value('core-health-ignored', 0, '')} provider=${value('core-health-ignored', 1, '')} healthProbe=${value('core-health-ignored', 2, '')} class=${value('core-health-ignored', 3, '')} probeCalls=${value('core-health-ignored', 4)}`,
    `route availability invalid policy field=${value('invalid-policy', 0, '')} value=${value('invalid-policy', 1, '')} error=${value('invalid-policy', 2, '')} class=${value('invalid-policy', 3, '')}`,
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`
  ].join('\n') + '\n'
}

function goList(root) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 120000
  })
  const line = `${result.stdout ?? ''}`.trim()
  const [files = '', tests = ''] = line.split('|')
  return { files: parseGoListFiles(files), tests: parseGoListFiles(tests) }
}

export function runRouteAvailability(root) {
  const listed = goList(root)
  const result = spawnSync('go', ['test', '-race', '-count=1', '-timeout', '180s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 600000
  })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    goFiles: listed.files,
    testFiles: listed.tests,
    rows: parseAvailabilityRows(log),
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
      process.stdout.write(`route=availability package=${PACKAGE_PATH} cases=${AVAILABILITY_CASES.join(',')}\n`)
      return
    }
    const evidence = runRouteAvailability(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_ROUTE_AVAILABILITY)
    if (evidence.exitCode !== 0) process.exit(EXIT_ROUTE_AVAILABILITY)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_ROUTE_AVAILABILITY)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
