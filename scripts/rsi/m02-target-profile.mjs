import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const GOLDEN_PATH = 'backend/internal/extension/kernel/plugincontract/testdata/binding-filter-matrix.json'
export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestFilterProviderBindings|TestModuleBindingRequirements'
export const GATES = ['placement', 'platform', 'permission', 'contractVersion']
export const DECISIONS = ['candidate', 'rejected']
export const MATRIX_FIELDS = ['name', 'host', 'platform', 'decision', 'gate', 'detail']
export const EXIT_TARGET_PROFILE = 27

const WIDE_CHAR_PATTERN = /[\u1100-\u115F\u2E80-\u303E\u3041-\u33FF\u3400-\u4DBF\u4E00-\u9FFF\uA000-\uA4CF\uAC00-\uD7A3\uF900-\uFAFF\uFE30-\uFE6F\uFF00-\uFF60\uFFE0-\uFFE6]/

function displayWidth(value) {
  let width = 0
  for (const char of value) width += WIDE_CHAR_PATTERN.test(char) ? 2 : 1
  return width
}

function padCell(value, width) {
  return `${value}${' '.repeat(Math.max(0, width - displayWidth(value)))}`
}

function cellValue(value) {
  return value === undefined || value === null ? '' : String(value)
}

export function readMatrix(root) {
  const path = resolve(root, GOLDEN_PATH)
  if (!existsSync(path)) throw new Error(`binding filter matrix missing: ${path}`)
  return JSON.parse(readFileSync(path, 'utf8'))
}

export function validateMatrix(matrix) {
  const problems = []
  if (matrix === null || typeof matrix !== 'object' || Array.isArray(matrix)) {
    problems.push('matrix is not an object')
    return problems
  }
  if (matrix.schemaVersion !== 1) problems.push(`schemaVersion=${matrix.schemaVersion} want 1`)
  if (typeof matrix.providerId !== 'string' || matrix.providerId.trim() === '') problems.push('providerId is empty')
  if (!Array.isArray(matrix.cases) || matrix.cases.length === 0) {
    problems.push('cases is not a non-empty array')
    return problems
  }
  const seen = new Set()
  matrix.cases.forEach((testCase, index) => {
    if (testCase === null || typeof testCase !== 'object' || Array.isArray(testCase)) {
      problems.push(`cases[${index}] is not an object`)
      return
    }
    if (typeof testCase.name !== 'string' || testCase.name.trim() === '') problems.push(`cases[${index}] name is empty`)
    if (seen.has(testCase.name)) problems.push(`cases[${index}] duplicate name ${testCase.name}`)
    seen.add(testCase.name)
    if (!DECISIONS.includes(testCase.decision)) {
      problems.push(`cases[${index}] decision=${cellValue(testCase.decision)} is not one of ${DECISIONS.join(', ')}`)
    }
    const profile = testCase.profile
    if (profile === null || typeof profile !== 'object' || Array.isArray(profile)) {
      problems.push(`cases[${index}] profile is not an object`)
    } else {
      if (typeof profile.platform !== 'string' || profile.platform.trim() === '') problems.push(`cases[${index}] profile.platform is empty`)
      if (typeof profile.host !== 'string' || profile.host.trim() === '') problems.push(`cases[${index}] profile.host is empty`)
    }
    const candidates = Array.isArray(testCase.candidates) ? testCase.candidates : []
    if (!Array.isArray(testCase.candidates)) problems.push(`cases[${index}] candidates is not an array`)
    if (testCase.decision === 'candidate') {
      if (candidates.length === 0) problems.push(`cases[${index}] candidate decision without candidates`)
      if (testCase.gate !== undefined) problems.push(`cases[${index}] candidate decision carries gate ${testCase.gate}`)
      return
    }
    if (candidates.length !== 0) problems.push(`cases[${index}] rejected decision still lists candidates`)
    if (!GATES.includes(testCase.gate)) {
      problems.push(`cases[${index}] gate=${cellValue(testCase.gate)} is not one of ${GATES.join(', ')}`)
    }
    if (typeof testCase.detail !== 'string' || testCase.detail.trim() === '') problems.push(`cases[${index}] detail is empty`)
  })
  return problems
}

export function summarizeMatrix(matrix) {
  const cases = Array.isArray(matrix.cases) ? matrix.cases : []
  const gateCounts = Object.fromEntries(GATES.map((gate) => [gate, 0]))
  for (const testCase of cases) {
    if (testCase?.decision === 'rejected' && GATES.includes(testCase.gate)) gateCounts[testCase.gate] += 1
  }
  return {
    cases: cases.length,
    candidates: cases.filter((testCase) => testCase?.decision === 'candidate').length,
    rejected: cases.filter((testCase) => testCase?.decision === 'rejected').length,
    gateCounts
  }
}

export function renderMatrix(matrix) {
  const summary = summarizeMatrix(matrix)
  const gateBreakdown = GATES.map((gate) => `${gate} ${summary.gateCounts[gate]}`).join(' / ')
  const lines = [
    `matrix=${GOLDEN_PATH} 用例 ${summary.cases} / 通过 ${summary.candidates} / 排除 ${summary.rejected}（${gateBreakdown}）`
  ]
  const rows = (Array.isArray(matrix.cases) ? matrix.cases : []).map((testCase) =>
    MATRIX_FIELDS.map((field) => {
      if (field === 'host' || field === 'platform') return cellValue(testCase?.profile?.[field])
      return cellValue(testCase?.[field])
    })
  )
  const widths = MATRIX_FIELDS.map((field, index) =>
    Math.max(displayWidth(field), ...rows.map((row) => displayWidth(row[index])))
  )
  const renderRow = (cells) => cells.map((cell, index) => padCell(cell, widths[index])).join('  ').trimEnd()
  lines.push(renderRow([...MATRIX_FIELDS]))
  for (const row of rows) lines.push(renderRow(row))
  return `${lines.join('\n')}\n`
}

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function runTargetProfile(root, options = {}) {
  const args = ['test', '-count=1', '-run', RUN_PATTERN, '-v', PACKAGE_PATH]
  const env = cleanEnv()
  if (options.update) env.UPDATE_GOLDEN = '1'
  const result = spawnSync('go', args, { cwd: resolve(root, 'backend'), env, encoding: 'utf8', timeout: 600000 })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  const matrix = existsSync(resolve(root, GOLDEN_PATH)) ? readMatrix(root) : null
  return {
    exitCode: result.status === null ? 1 : result.status,
    passed: counts.passed,
    failed: counts.failed,
    matrix,
    summary: matrix === null ? null : summarizeMatrix(matrix),
    listing: matrix === null ? '' : renderMatrix(matrix),
    log
  }
}

function reportProblems(problems) {
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  return problems.length > 0
}

function main() {
  const args = process.argv.slice(2)
  const list = args.includes('--list')
  const update = args.includes('--update')
  try {
    if (list) {
      const matrix = readMatrix(repoRoot)
      process.stdout.write(renderMatrix(matrix))
      if (reportProblems(validateMatrix(matrix))) process.exit(EXIT_TARGET_PROFILE)
      return
    }
    const result = runTargetProfile(repoRoot, { update })
    process.stdout.write(result.listing)
    process.stdout.write(`go test passed=${result.passed} failed=${result.failed} exit=${result.exitCode}\n`)
    if (result.matrix !== null && reportProblems(validateMatrix(result.matrix))) process.exit(EXIT_TARGET_PROFILE)
    if (result.exitCode !== 0) process.exit(EXIT_TARGET_PROFILE)
    if (update) process.stdout.write(`updated=true review=${GOLDEN_PATH}\n`)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_TARGET_PROFILE)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
