import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/pluginadapter'
export const BUILD_TAG = 'rsi_pluginadapter'
export const RUN_PATTERN = 'TestPluginAdapter'
export const EXPECTED_GUARDS = 13
export const EXPECTED_REJECTED = 10
export const EXPECTED_PASSED = 17
export const EXIT_PLUGIN_ADAPTER = 32

const GUARDS_PATTERN = /plugin adapter guards provider=(\S+) cases=(\d+) rejected=(\d+)/

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function parseGuardSummary(log) {
  const body = typeof log === 'string' ? log : ''
  const match = body.match(GUARDS_PATTERN)
  if (!match) return null
  return { provider: match[1], cases: Number(match[2]), rejected: Number(match[3]) }
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
  if (!taggedFiles.includes('plugin_adapter.go')) problems.push('tagged build is missing plugin_adapter.go')
  if (!taggedFiles.includes('doc.go')) problems.push('tagged build dropped doc.go')
  if (!taggedTests.includes('plugin_adapter_test.go')) problems.push('tagged tests are missing plugin_adapter_test.go')
  const guards = evidence?.guards
  if (guards === null || guards === undefined) {
    problems.push('guard summary is missing')
  } else {
    if (!guards.provider) problems.push('guard summary provider is empty')
    if (guards.cases !== EXPECTED_GUARDS) problems.push(`guard cases=${guards.cases} want ${EXPECTED_GUARDS}`)
    if (guards.rejected !== EXPECTED_REJECTED) problems.push(`guard rejected=${guards.rejected} want ${EXPECTED_REJECTED}`)
  }
  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  return problems
}

export function renderEvidence(evidence) {
  const guards = evidence?.guards ?? {}
  return [
    `adapter=plugin provider=${guards.provider ?? ''} tag=${BUILD_TAG}`,
    `isolation: default=[${(evidence?.defaultFiles ?? []).join(' ')}] tagged=[${(evidence?.taggedFiles ?? []).join(' ')}] tests=[${(evidence?.taggedTests ?? []).join(' ')}]`,
    `guards cases=${guards.cases ?? 0} rejected=${guards.rejected ?? 0} passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1}`
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

export function runPluginAdapter(root) {
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
    guards: parseGuardSummary(log),
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
      process.stdout.write(`adapter=plugin provider=com.amitia/emote/ai tag=${BUILD_TAG} package=${PACKAGE_PATH} guards=${EXPECTED_GUARDS}\n`)
      return
    }
    const evidence = runPluginAdapter(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    if (reportProblems(validateEvidence(evidence))) process.exit(EXIT_PLUGIN_ADAPTER)
    process.stdout.write(`go test passed=${evidence.passed} failed=${evidence.failed} exit=${evidence.exitCode}\n`)
    if (evidence.exitCode !== 0) process.exit(EXIT_PLUGIN_ADAPTER)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_PLUGIN_ADAPTER)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
