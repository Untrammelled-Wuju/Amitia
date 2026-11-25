import { spawnSync } from 'node:child_process'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestRoutePolicy'
export const EXIT_ROUTE_POLICY = 37
export const EXPECTED_PASSED = 8
export const REQUIRED_SOURCES = ['route_policy.go']
export const REQUIRED_TESTS = ['route_policy_test.go']
export const POLICY_CASES = ['auto', 'core_only', 'plugin_only', 'pin', 'pin-conflict', 'compat', 'unsatisfied', 'invalid']
export const CORE_PROVIDER_ID = 'builtin.core/core/memory.search'
export const PLUGIN_PROVIDER_ID = 'plugin.host'
export const PIN_PRIORITY = 90

const AUTO_PATTERN = /route policy mode=auto selected=(\S+) bound=(\S+) candidates=(\d+) skipped=(\d+) pinned=(true|false)/
const CORE_PATTERN = /route policy mode=core_only selected=(\S+) candidates=(\d+) skipped=(\d+) reason=(\w+)/
const PLUGIN_PATTERN = /route policy mode=plugin_only selected=(\S+) candidates=(\d+) skipped=(\d+) reason=(\w+)/
const PIN_PATTERN = /route policy pin pinned=(\S+) selected=(\S+) pluginPriority=(\d+) corePriority=(\d+) overridden=(true|false)/
const PIN_CONFLICT_PATTERN = /route policy pin conflict pinned=(\S+) mode=(\S+) error=(\w+)/
const COMPAT_PATTERN = /route policy compat selected=(\S+) candidates=(\d+) skipped=(\d+) reasons=(\w+),(\w+) fallback=(true|false)/
const UNSATISFIED_PATTERN = /route policy unsatisfied mode=(\S+) candidates=(\d+) skipped=(\d+) error=(\w+)/
const INVALID_PATTERN = /route policy invalid field=(\S+) value=(\S+) error=(\w+)/

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

export function parsePolicyRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    auto: parseNamed(body, AUTO_PATTERN),
    core_only: parseNamed(body, CORE_PATTERN),
    plugin_only: parseNamed(body, PLUGIN_PATTERN),
    pin: parseNamed(body, PIN_PATTERN),
    'pin-conflict': parseNamed(body, PIN_CONFLICT_PATTERN),
    compat: parseNamed(body, COMPAT_PATTERN),
    unsatisfied: parseNamed(body, UNSATISFIED_PATTERN),
    invalid: parseNamed(body, INVALID_PATTERN)
  }
}

function rowPattern(name) {
  switch (name) {
    case 'auto':
      return AUTO_PATTERN
    case 'core_only':
      return CORE_PATTERN
    case 'plugin_only':
      return PLUGIN_PATTERN
    case 'pin':
      return PIN_PATTERN
    case 'pin-conflict':
      return PIN_CONFLICT_PATTERN
    case 'compat':
      return COMPAT_PATTERN
    case 'unsatisfied':
      return UNSATISFIED_PATTERN
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
  for (const name of POLICY_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  const auto = rows.auto
  if (Array.isArray(auto)) {
    if (auto[0] !== PLUGIN_PROVIDER_ID) problems.push(`auto selected=${auto[0]} want ${PLUGIN_PROVIDER_ID}`)
    if (auto[0] !== auto[1]) problems.push('auto mode replaced the bound provider')
    if (auto[2] !== 2) problems.push(`auto candidates=${auto[2]} want 2`)
    if (auto[3] !== 0) problems.push(`auto skipped=${auto[3]} want 0`)
    if (auto[4] !== 'false') problems.push('auto mode must not report a pin')
  }

  const coreOnly = rows.core_only
  if (Array.isArray(coreOnly)) {
    if (coreOnly[0] !== CORE_PROVIDER_ID) problems.push(`core_only selected=${coreOnly[0]} want ${CORE_PROVIDER_ID}`)
    if (coreOnly[1] !== 1) problems.push(`core_only candidates=${coreOnly[1]} want 1`)
    if (coreOnly[2] !== 1) problems.push(`core_only skipped=${coreOnly[2]} want 1`)
    if (coreOnly[3] !== 'mode_excluded') problems.push(`core_only reason=${coreOnly[3]} want mode_excluded`)
  }

  const pluginOnly = rows.plugin_only
  if (Array.isArray(pluginOnly)) {
    if (pluginOnly[0] !== PLUGIN_PROVIDER_ID) problems.push(`plugin_only selected=${pluginOnly[0]} want ${PLUGIN_PROVIDER_ID}`)
    if (pluginOnly[2] !== 1 || pluginOnly[3] !== 'mode_excluded') problems.push(`plugin_only skipped=${pluginOnly[2]} reason=${pluginOnly[3]} want 1/mode_excluded`)
  }

  const pin = rows.pin
  if (Array.isArray(pin)) {
    if (pin[0] !== CORE_PROVIDER_ID || pin[1] !== CORE_PROVIDER_ID) problems.push(`pin pinned=${pin[0]} selected=${pin[1]} want core/core`)
    if (pin[2] !== PIN_PRIORITY) problems.push(`pin pluginPriority=${pin[2]} want ${PIN_PRIORITY}`)
    if (!(pin[3] < pin[2])) problems.push(`pin corePriority=${pin[3]} must stay below the plugin priority ${pin[2]}`)
    if (pin[4] !== 'true') problems.push('pin row must report that the evolution candidate was overridden')
  }

  const pinConflict = rows['pin-conflict']
  if (Array.isArray(pinConflict)) {
    if (pinConflict[0] !== PLUGIN_PROVIDER_ID) problems.push(`pin conflict pinned=${pinConflict[0]} want ${PLUGIN_PROVIDER_ID}`)
    if (pinConflict[1] !== 'core_only') problems.push(`pin conflict mode=${pinConflict[1]} want core_only`)
    if (pinConflict[2] !== 'ErrRoutePinConflict') problems.push(`pin conflict error=${pinConflict[2]} want ErrRoutePinConflict`)
  }

  const compat = rows.compat
  if (Array.isArray(compat)) {
    if (compat[0] !== CORE_PROVIDER_ID) problems.push(`compat selected=${compat[0]} want ${CORE_PROVIDER_ID}`)
    if (compat[1] !== 1 || compat[2] !== 2) problems.push(`compat candidates=${compat[1]} skipped=${compat[2]} want 1/2`)
    if (compat[3] !== 'contract_mismatch' || compat[4] !== 'version_mismatch') {
      problems.push(`compat reasons=${compat[3]},${compat[4]} want contract_mismatch,version_mismatch`)
    }
    if (compat[5] !== 'true') problems.push('compat row must report fallback=true')
  }

  const unsatisfied = rows.unsatisfied
  if (Array.isArray(unsatisfied)) {
    if (unsatisfied[0] !== 'core_only') problems.push(`unsatisfied mode=${unsatisfied[0]} want core_only`)
    if (unsatisfied[1] !== 0 || unsatisfied[2] !== 1) problems.push(`unsatisfied candidates=${unsatisfied[1]} skipped=${unsatisfied[2]} want 0/1`)
    if (unsatisfied[3] !== 'ErrRouteModeUnsatisfied') problems.push(`unsatisfied error=${unsatisfied[3]} want ErrRouteModeUnsatisfied`)
  }

  const invalid = rows.invalid
  if (Array.isArray(invalid)) {
    if (invalid[0] !== 'mode' || invalid[1] !== 'hybrid') problems.push(`invalid field=${invalid[0]} value=${invalid[1]} want mode/hybrid`)
    if (invalid[2] !== 'ErrRoutePolicyInvalid') problems.push(`invalid error=${invalid[2]} want ErrRoutePolicyInvalid`)
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
    `route=policy package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${POLICY_CASES.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((source) => goFiles.includes(source)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `route policy mode=auto selected=${value('auto', 0, '')} bound=${value('auto', 1, '')} candidates=${value('auto', 2)} skipped=${value('auto', 3)} pinned=${value('auto', 4, 'false')}`,
    `route policy mode=core_only selected=${value('core_only', 0, '')} candidates=${value('core_only', 1)} skipped=${value('core_only', 2)} reason=${value('core_only', 3, '')}`,
    `route policy mode=plugin_only selected=${value('plugin_only', 0, '')} candidates=${value('plugin_only', 1)} skipped=${value('plugin_only', 2)} reason=${value('plugin_only', 3, '')}`,
    `route policy pin pinned=${value('pin', 0, '')} selected=${value('pin', 1, '')} pluginPriority=${value('pin', 2)} corePriority=${value('pin', 3)} overridden=${value('pin', 4, 'false')}`,
    `route policy pin conflict pinned=${value('pin-conflict', 0, '')} mode=${value('pin-conflict', 1, '')} error=${value('pin-conflict', 2, '')}`,
    `route policy compat selected=${value('compat', 0, '')} candidates=${value('compat', 1)} skipped=${value('compat', 2)} reasons=${value('compat', 3, '')},${value('compat', 4, '')} fallback=${value('compat', 5, 'false')}`,
    `route policy unsatisfied mode=${value('unsatisfied', 0, '')} candidates=${value('unsatisfied', 1)} skipped=${value('unsatisfied', 2)} error=${value('unsatisfied', 3, '')}`,
    `route policy invalid field=${value('invalid', 0, '')} value=${value('invalid', 1, '')} error=${value('invalid', 2, '')}`,
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

export function runRoutePolicy(root) {
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
    rows: parsePolicyRows(log),
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
      process.stdout.write(`route=policy package=${PACKAGE_PATH} cases=${POLICY_CASES.join(',')}\n`)
      return
    }
    const evidence = runRoutePolicy(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_ROUTE_POLICY)
    if (evidence.exitCode !== 0) process.exit(EXIT_ROUTE_POLICY)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_ROUTE_POLICY)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
