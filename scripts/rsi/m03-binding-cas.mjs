import { spawnSync } from 'node:child_process'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestBindingCommit'
export const EXIT_BINDING_CAS = 36
export const EXPECTED_PASSED = 7
export const EXPECTED_WORKERS = 32
export const REQUIRED_SOURCES = ['binding_commit.go', 'binding_snapshot.go']
export const REQUIRED_TESTS = ['binding_commit_test.go']
export const CAS_CASES = ['cas', 'stale', 'invalid', 'empty', 'pinned', 'cancellation', 'concurrent']

const CAS_PATTERN = /binding commit cas epoch=(\S+) from=(\d+) to=(\d+) installed=(true|false) digestChanged=(true|false)/
const STALE_PATTERN = /binding commit stale expected=(\d+) actual=(\d+) conflicts=(\d+) chain=(\S+) unchanged=(true|false)/
const INVALID_PATTERN = /binding commit invalid field=(\S+) rejected=(\d+) revisionBefore=(\d+) revisionAfter=(\d+) retry=(\d+)/
const EMPTY_PATTERN = /binding commit empty from=(\d+) to=(\d+) installedBefore=(true|false)/
const PINNED_PATTERN = /binding commit pinned provider=(\S+) revision=(\d+) currentProvider=(\S+) currentRevision=(\d+) retained=(\d+) releasedRetained=(\d+) stable=(true|false)/
const CANCELLATION_PATTERN = /binding commit cancellation resolvedAfterRelease=(\S+) doubleRelease=(\S+) acquireAfterRelease=(\d+)/
const CONCURRENT_PATTERN = /binding commit concurrent workers=(\d+) successes=(\d+) conflicts=(\d+) revision=(\d+) winners=(\d+)/

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

function parseNumbered(text, pattern) {
  const match = text.match(pattern)
  return match ? match.slice(1).map((value) => (/^\d+$/.test(value) ? Number(value) : value)) : null
}

export function parseCommitRows(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    cas: parseNumbered(body, CAS_PATTERN),
    stale: parseNumbered(body, STALE_PATTERN),
    invalid: parseNumbered(body, INVALID_PATTERN),
    empty: parseNumbered(body, EMPTY_PATTERN),
    pinned: parseNumbered(body, PINNED_PATTERN),
    cancellation: parseNumbered(body, CANCELLATION_PATTERN),
    concurrent: parseNumbered(body, CONCURRENT_PATTERN)
  }
}

function countMatches(text, pattern) {
  return (text.match(new RegExp(pattern.source, pattern.flags.includes('g') ? pattern.flags : `${pattern.flags}g`)) ?? []).length
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
  for (const name of CAS_CASES) {
    if (!Array.isArray(rows[name])) problems.push(`evidence row ${name} is missing`)
    if (countMatches(log, rowPattern(name)) !== 1) problems.push(`evidence row ${name} must print exactly once`)
  }

  const cas = rows.cas
  if (Array.isArray(cas)) {
    if (cas[0] === '') problems.push('cas row lost the epoch')
    if (cas[1] !== 1 || cas[2] !== 2) problems.push(`cas from=${cas[1]} to=${cas[2]} want 1->2`)
    if (cas[3] !== 'true') problems.push(`cas installed=${cas[3]} want true`)
    if (cas[4] !== 'true') problems.push(`cas digestChanged=${cas[4]} want true`)
  }

  const stale = rows.stale
  if (Array.isArray(stale)) {
    if (stale[0] !== 1 || stale[1] !== 2) problems.push(`stale expected=${stale[0]} actual=${stale[1]} want 1/2`)
    if (stale[2] !== 1) problems.push(`stale conflicts=${stale[2]} want 1`)
    if (stale[3] !== 'ok') problems.push(`stale chain=${stale[3]} want ok`)
    if (stale[4] !== 'true') problems.push(`stale unchanged=${stale[4]} want true`)
  }

  const invalid = rows.invalid
  if (Array.isArray(invalid)) {
    if (invalid[0] !== 'providerId') problems.push(`invalid field=${invalid[0]} want providerId`)
    if (invalid[1] !== 1) problems.push(`invalid rejected=${invalid[1]} want 1`)
    if (invalid[2] !== invalid[3]) problems.push(`invalid revisionBefore=${invalid[2]} revisionAfter=${invalid[3]} must not move`)
    if (invalid[4] !== invalid[2] + 1) problems.push(`invalid retry=${invalid[4]} want ${invalid[2] + 1}`)
  }

  const empty = rows.empty
  if (Array.isArray(empty)) {
    if (empty[0] !== 0 || empty[1] !== 1) problems.push(`empty from=${empty[0]} to=${empty[1]} want 0->1`)
    if (empty[2] !== 'false') problems.push(`empty installedBefore=${empty[2]} want false`)
  }

  const pinned = rows.pinned
  if (Array.isArray(pinned)) {
    if (pinned[0] !== 'global.host') problems.push(`pinned provider=${pinned[0]} want global.host`)
    if (pinned[1] !== 1) problems.push(`pinned revision=${pinned[1]} want 1`)
    if (pinned[2] !== 'next.host') problems.push(`pinned currentProvider=${pinned[2]} want next.host`)
    if (pinned[3] !== 2) problems.push(`pinned currentRevision=${pinned[3]} want 2`)
    if (pinned[4] !== 1) problems.push(`pinned retained=${pinned[4]} want 1`)
    if (pinned[5] !== 0) problems.push(`pinned releasedRetained=${pinned[5]} want 0`)
    if (pinned[6] !== 'true') problems.push(`pinned stable=${pinned[6]} want true`)
    if (pinned[2] === pinned[0]) problems.push('pinned row moved the provider; the old request was not pinned')
  }

  const cancellation = rows.cancellation
  if (Array.isArray(cancellation)) {
    if (cancellation[0] !== 'ErrSnapshotRelease') problems.push(`cancellation resolvedAfterRelease=${cancellation[0]} want ErrSnapshotRelease`)
    if (cancellation[1] !== 'idempotent') problems.push(`cancellation doubleRelease=${cancellation[1]} want idempotent`)
    if (cancellation[2] !== 2) problems.push(`cancellation acquireAfterRelease=${cancellation[2]} want 2`)
  }

  const concurrent = rows.concurrent
  if (Array.isArray(concurrent)) {
    if (concurrent[0] !== EXPECTED_WORKERS) problems.push(`concurrent workers=${concurrent[0]} want ${EXPECTED_WORKERS}`)
    if (concurrent[1] !== 1) problems.push(`concurrent successes=${concurrent[1]} want 1`)
    if (concurrent[2] !== EXPECTED_WORKERS - 1) problems.push(`concurrent conflicts=${concurrent[2]} want ${EXPECTED_WORKERS - 1}`)
    if (concurrent[1] + concurrent[2] !== concurrent[0]) problems.push('concurrent successes+conflicts do not add up to workers')
    if (concurrent[3] !== 2) problems.push(`concurrent revision=${concurrent[3]} want 2`)
    if (concurrent[4] !== 1) problems.push(`concurrent winners=${concurrent[4]} want 1`)
  }

  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  if (evidence?.race !== true) problems.push('go test must run with -race')
  return problems
}

function rowPattern(name) {
  switch (name) {
    case 'cas':
      return CAS_PATTERN
    case 'stale':
      return STALE_PATTERN
    case 'invalid':
      return INVALID_PATTERN
    case 'empty':
      return EMPTY_PATTERN
    case 'pinned':
      return PINNED_PATTERN
    case 'cancellation':
      return CANCELLATION_PATTERN
    default:
      return CONCURRENT_PATTERN
  }
}

export function renderEvidence(evidence) {
  const rows = evidence?.rows ?? {}
  const goFiles = evidence?.goFiles ?? []
  const testFiles = evidence?.testFiles ?? []
  const line = (name) => {
    const values = Array.isArray(rows[name]) ? rows[name] : []
    switch (name) {
      case 'cas':
        return `binding commit cas epoch=${values[0] ?? ''} from=${values[1] ?? 0} to=${values[2] ?? 0} installed=${values[3] ?? 'false'} digestChanged=${values[4] ?? 'false'}`
      case 'stale':
        return `binding commit stale expected=${values[0] ?? 0} actual=${values[1] ?? 0} conflicts=${values[2] ?? 0} chain=${values[3] ?? 'unknown'} unchanged=${values[4] ?? 'false'}`
      case 'invalid':
        return `binding commit invalid field=${values[0] ?? ''} rejected=${values[1] ?? 0} revisionBefore=${values[2] ?? 0} revisionAfter=${values[3] ?? 0} retry=${values[4] ?? 0}`
      case 'empty':
        return `binding commit empty from=${values[0] ?? 0} to=${values[1] ?? 0} installedBefore=${values[2] ?? 'false'}`
      case 'pinned':
        return `binding commit pinned provider=${values[0] ?? ''} revision=${values[1] ?? 0} currentProvider=${values[2] ?? ''} currentRevision=${values[3] ?? 0} retained=${values[4] ?? 0} releasedRetained=${values[5] ?? 0} stable=${values[6] ?? 'false'}`
      case 'cancellation':
        return `binding commit cancellation resolvedAfterRelease=${values[0] ?? ''} doubleRelease=${values[1] ?? ''} acquireAfterRelease=${values[2] ?? 0}`
      default:
        return `binding commit concurrent workers=${values[0] ?? 0} successes=${values[1] ?? 0} conflicts=${values[2] ?? 0} revision=${values[3] ?? 0} winners=${values[4] ?? 0}`
    }
  }
  return [
    `binding=cas package=${PACKAGE_PATH} run=${RUN_PATTERN} cases=${CAS_CASES.length}`,
    `files=${goFiles.length} tests=${testFiles.length} sources=${REQUIRED_SOURCES.filter((source) => goFiles.includes(source)).length}/${REQUIRED_SOURCES.length} testExcluded=${goFiles.length > 0 && goFiles.every((file) => !file.endsWith('_test.go'))}`,
    ...CAS_CASES.map(line),
    `passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`
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

export function runBindingCAS(root) {
  const listed = goList(root, [])
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
    rows: parseCommitRows(log),
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
      process.stdout.write(`binding=cas package=${PACKAGE_PATH} cases=${CAS_CASES.join(',')}\n`)
      return
    }
    const evidence = runBindingCAS(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_BINDING_CAS)
    if (evidence.exitCode !== 0) process.exit(EXIT_BINDING_CAS)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_BINDING_CAS)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
