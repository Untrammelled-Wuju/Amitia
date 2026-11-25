import { spawnSync } from 'node:child_process'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestScope'
export const EXIT_ROUTE_SCOPE = 40
export const EXPECTED_PASSED = 8
export const REQUIRED_SOURCES = ['route_scope.go']
export const REQUIRED_TESTS = ['route_scope_test.go']
export const SCOPE_CASES = [
  'device',
  'space',
  'role-surface',
  'remote-offline',
  'remote-online',
  'core-device',
  'missing-claim',
  'invalid'
]
export const SCOPE_DIMENSIONS = ['device', 'core', 'space', 'role', 'surface']
export const REMOTE_PROVIDER_ID = 'remote.7'
export const LOCAL_PROVIDER_ID = 'local.device'

const DEVICE_PATTERN = /route scope device provider=(\S+) device=(\S+) matched=(true|false)/
const SPACE_PATTERN = /route scope space provider=(\S+) space=(\S+) matched=(true|false)/
const ROLE_SURFACE_PATTERN = /route scope role surface provider=(\S+) role=(\S+) surface=(\S+) matched=(true|false)/
const REMOTE_OFFLINE_PATTERN = /route scope remote offline requirement=device:(\S+) selected=(\S*) error=(\w+) candidates=(\d+) localRejected=(true|false)/
const REMOTE_ONLINE_PATTERN = /route scope remote online requirement=device:(\S+) selected=(\S+) matched=(true|false)/
const CORE_DEVICE_PATTERN = /route scope core cannot serve device provider=(\S+) device=(\S+) matched=(true|false) reason=(\w+)/
const MISSING_CLAIM_PATTERN = /route scope missing claim provider=(\S+) dimension=(\w+) matched=(true|false) reason=(\w+) error=(\w+)/
const INVALID_PATTERN = /route scope invalid field=(\S+) value=(\S+) error=(\w+) coreDeviceConflict=(true|false)/

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

export function parseScopeRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    device: parseNamed(body, DEVICE_PATTERN),
    space: parseNamed(body, SPACE_PATTERN),
    'role-surface': parseNamed(body, ROLE_SURFACE_PATTERN),
    'remote-offline': parseNamed(body, REMOTE_OFFLINE_PATTERN),
    'remote-online': parseNamed(body, REMOTE_ONLINE_PATTERN),
    'core-device': parseNamed(body, CORE_DEVICE_PATTERN),
    'missing-claim': parseNamed(body, MISSING_CLAIM_PATTERN),
    invalid: parseNamed(body, INVALID_PATTERN)
  }
}

export function rowPattern(name) {
  switch (name) {
    case 'device':
      return DEVICE_PATTERN
    case 'space':
      return SPACE_PATTERN
    case 'role-surface':
      return ROLE_SURFACE_PATTERN
    case 'remote-offline':
      return REMOTE_OFFLINE_PATTERN
    case 'remote-online':
      return REMOTE_ONLINE_PATTERN
    case 'core-device':
      return CORE_DEVICE_PATTERN
    case 'missing-claim':
      return MISSING_CLAIM_PATTERN
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
  for (const name of SCOPE_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  const device = rows.device
  if (Array.isArray(device)) {
    if (device[1] !== 'device-a') problems.push(`device claim=${device[1]} want device-a`)
    if (device[2] !== 'true') problems.push('a device claim must match the same device requirement')
  }

  const space = rows.space
  if (Array.isArray(space)) {
    if (space[1] !== 'space-1') problems.push(`space claim=${space[1]} want space-1`)
    if (space[2] !== 'true') problems.push('a space claim must match the same space requirement')
  }

  const roleSurface = rows['role-surface']
  if (Array.isArray(roleSurface)) {
    if (roleSurface[1] !== 'butler') problems.push(`role claim=${roleSurface[1]} want butler`)
    if (roleSurface[2] !== 'desktop') problems.push(`surface claim=${roleSurface[2]} want desktop`)
    if (roleSurface[3] !== 'true') problems.push('role and surface claims must both match')
  }

  const remoteOffline = rows['remote-offline']
  if (Array.isArray(remoteOffline)) {
    if (remoteOffline[0] !== 'device-7') problems.push(`offline requirement=${remoteOffline[0]} want device-7`)
    if (remoteOffline[1] !== '') problems.push(`offline selected=${remoteOffline[1]} want no provider`)
    if (remoteOffline[2] !== 'ErrScopeProviderUnavailable') problems.push(`offline error=${remoteOffline[2]} want ErrScopeProviderUnavailable`)
    if (remoteOffline[3] !== 1) problems.push(`offline candidates=${remoteOffline[3]} want 1`)
    if (remoteOffline[4] !== 'true') problems.push('offline remote must never rebind the same-named local capability')
  }

  const remoteOnline = rows['remote-online']
  if (Array.isArray(remoteOnline)) {
    if (remoteOnline[1] !== REMOTE_PROVIDER_ID) problems.push(`online selected=${remoteOnline[1]} want ${REMOTE_PROVIDER_ID}`)
    if (remoteOnline[2] !== 'true') problems.push('an online remote provider must be selected by scope')
  }

  const coreDevice = rows['core-device']
  if (Array.isArray(coreDevice)) {
    if (coreDevice[0] !== 'builtin.core/core/memory.search') problems.push(`core device provider=${coreDevice[0]} want builtin.core/core/memory.search`)
    if (coreDevice[2] !== 'false') problems.push('a core provider must not claim to serve a remote device')
    if (coreDevice[3] !== 'core_cannot_serve_device') problems.push(`core device reason=${coreDevice[3]} want core_cannot_serve_device`)
  }

  const missingClaim = rows['missing-claim']
  if (Array.isArray(missingClaim)) {
    if (missingClaim[1] !== 'role') problems.push(`missing claim dimension=${missingClaim[1]} want role`)
    if (missingClaim[2] !== 'false') problems.push('an undeclared requirement dimension must not match')
    if (missingClaim[3] !== 'missing_claim') problems.push(`missing claim reason=${missingClaim[3]} want missing_claim`)
    if (missingClaim[4] !== 'ErrScopeNoMatch') problems.push(`missing claim error=${missingClaim[4]} want ErrScopeNoMatch`)
  }

  const invalid = rows.invalid
  if (Array.isArray(invalid)) {
    if (invalid[0] !== 'claims.dimension') problems.push(`invalid field=${invalid[0]} want claims.dimension`)
    if (invalid[1] !== 'platform') problems.push(`invalid value=${invalid[1]} want platform`)
    if (invalid[2] !== 'ErrScopeContractInvalid') problems.push(`invalid error=${invalid[2]} want ErrScopeContractInvalid`)
    if (invalid[3] !== 'true') problems.push('a contract claiming both core and device must be rejected')
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
    `route=scope package=${PACKAGE_PATH} run=${RUN_PATTERN} dimensions=${SCOPE_DIMENSIONS.join(',')} cases=${SCOPE_CASES.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((source) => goFiles.includes(source)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    `route scope device provider=${value('device', 0, '')} device=${value('device', 1, '')} matched=${value('device', 2, 'false')}`,
    `route scope space provider=${value('space', 0, '')} space=${value('space', 1, '')} matched=${value('space', 2, 'false')}`,
    `route scope role surface provider=${value('role-surface', 0, '')} role=${value('role-surface', 1, '')} surface=${value('role-surface', 2, '')} matched=${value('role-surface', 3, 'false')}`,
    `route scope remote offline requirement=device:${value('remote-offline', 0, '')} selected=${value('remote-offline', 1, '')} error=${value('remote-offline', 2, '')} candidates=${value('remote-offline', 3)} localRejected=${value('remote-offline', 4, 'false')}`,
    `route scope remote online requirement=device:${value('remote-online', 0, '')} selected=${value('remote-online', 1, '')} matched=${value('remote-online', 2, 'false')}`,
    `route scope core cannot serve device provider=${value('core-device', 0, '')} device=${value('core-device', 1, '')} matched=${value('core-device', 2, 'false')} reason=${value('core-device', 3, '')}`,
    `route scope missing claim provider=${value('missing-claim', 0, '')} dimension=${value('missing-claim', 1, '')} matched=${value('missing-claim', 2, 'false')} reason=${value('missing-claim', 3, '')} error=${value('missing-claim', 4, '')}`,
    `route scope invalid field=${value('invalid', 0, '')} value=${value('invalid', 1, '')} error=${value('invalid', 2, '')} coreDeviceConflict=${value('invalid', 3, 'false')}`,
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

export function runRouteScope(root) {
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
    rows: parseScopeRows(log),
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
      process.stdout.write(`route=scope package=${PACKAGE_PATH} cases=${SCOPE_CASES.join(',')}\n`)
      return
    }
    const evidence = runRouteScope(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_ROUTE_SCOPE)
    if (evidence.exitCode !== 0) process.exit(EXIT_ROUTE_SCOPE)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_ROUTE_SCOPE)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
