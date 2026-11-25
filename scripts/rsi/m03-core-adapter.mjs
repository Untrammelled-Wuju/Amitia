import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/coreadapter'
export const BUILD_TAG = 'rsi_coreadapter'
export const RUN_PATTERN = 'TestMemorySearchAdapter'
export const PROVIDER_ID = 'builtin.core/core/memory.search'
export const EXPECTED_CASES = 100
export const EXPECTED_PASSED = 7
export const EXIT_CORE_ADAPTER = 31

const CASES_PATTERN = /equivalence cases=(\d+) provider=(\S+) tag=(\S+)/

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function parseEquivalence(log) {
  const body = typeof log === 'string' ? log : ''
  const match = body.match(CASES_PATTERN)
  if (!match) return null
  return { cases: Number(match[1]), provider: match[2], tag: match[3] }
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
  if (!taggedFiles.includes('memory_search_adapter.go')) problems.push('tagged build is missing memory_search_adapter.go')
  if (taggedFiles.includes('doc.go') === false) problems.push('tagged build dropped doc.go')
  if (!taggedTests.includes('memory_search_adapter_test.go')) problems.push('tagged tests are missing memory_search_adapter_test.go')
  const equivalence = evidence?.equivalence
  if (equivalence === null || equivalence === undefined) {
    problems.push('equivalence summary is missing')
  } else {
    if (equivalence.cases !== EXPECTED_CASES) problems.push(`equivalence cases=${equivalence.cases} want ${EXPECTED_CASES}`)
    if (equivalence.provider !== PROVIDER_ID) problems.push(`equivalence provider=${equivalence.provider} want ${PROVIDER_ID}`)
    if (equivalence.tag !== BUILD_TAG) problems.push(`equivalence tag=${equivalence.tag} want ${BUILD_TAG}`)
  }
  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  return problems
}

export function renderEvidence(evidence) {
  const defaultFiles = (evidence?.defaultFiles ?? []).join(' ')
  const taggedFiles = (evidence?.taggedFiles ?? []).join(' ')
  const taggedTests = (evidence?.taggedTests ?? []).join(' ')
  const equivalence = evidence?.equivalence ?? {}
  return [
    `adapter=${equivalence.provider ?? ''} service=memory.search tag=${BUILD_TAG}`,
    `isolation: default=[${defaultFiles}] tagged=[${taggedFiles}] tests=[${taggedTests}]`,
    `equivalence cases=${equivalence.cases ?? 0} passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1}`
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
  return { files: parseGoListFiles(files), tests: parseGoListFiles(tests), exitCode: result.status === null ? 1 : result.status, stderr: `${result.stderr ?? ''}` }
}

export function runCoreAdapter(root) {
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
  const evidence = {
    defaultFiles: base.files,
    taggedFiles: tagged.files,
    taggedTests: tagged.tests,
    equivalence: parseEquivalence(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: result.status === null ? 1 : result.status
  }
  return { ...evidence, log }
}

function reportProblems(problems) {
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  return problems.length > 0
}

function main() {
  try {
    if (process.argv.includes('--list')) {
      process.stdout.write(`adapter=${PROVIDER_ID} service=memory.search tag=${BUILD_TAG} package=${PACKAGE_PATH}\n`)
      return
    }
    const evidence = runCoreAdapter(repoRoot)
    process.stdout.write(renderEvidence(evidence))
    const problems = validateEvidence(evidence)
    if (reportProblems(problems)) process.exit(EXIT_CORE_ADAPTER)
    process.stdout.write(`go test passed=${evidence.passed} failed=${evidence.failed} exit=${evidence.exitCode}\n`)
    if (evidence.exitCode !== 0) process.exit(EXIT_CORE_ADAPTER)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_CORE_ADAPTER)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
