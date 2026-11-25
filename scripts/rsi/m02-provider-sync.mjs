import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const GOLDEN_PATH = 'backend/internal/extension/kernel/testdata/provider-conformance.json'
export const PACKAGE_PATH = './internal/extension/kernel'
export const RUN_PATTERN = 'TestProviderConformance'
export const BLOCKING_KIND = 'runtime.undeclared'
export const ADVISORY_KIND = 'declared.missing'
export const GAP_KINDS = [BLOCKING_KIND, ADVISORY_KIND]
export const GAP_REQUIRED_FIELDS = ['kind', 'extensionId', 'detail']
export const LISTING_FIELDS = ['extensionId', 'moduleId', 'capabilityId', 'providerId', 'kind', 'detail']
export const EXIT_PROVIDER_SYNC = 26

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

export function readGolden(root) {
  const path = resolve(root, GOLDEN_PATH)
  if (!existsSync(path)) throw new Error(`provider conformance golden missing: ${path}`)
  return JSON.parse(readFileSync(path, 'utf8'))
}

export function validateGolden(golden) {
  const problems = []
  if (golden === null || typeof golden !== 'object' || Array.isArray(golden)) {
    problems.push('golden is not an object')
    return problems
  }
  if (golden.schemaVersion !== 1) problems.push(`schemaVersion=${golden.schemaVersion} want 1`)
  if (typeof golden.extensionId !== 'string' || golden.extensionId.trim() === '') problems.push('extensionId is empty')
  for (const field of ['declaredProviders', 'runtimeProviders']) {
    if (!Number.isInteger(golden[field]) || golden[field] < 0) {
      problems.push(`${field}=${cellValue(golden[field])} is not a non-negative integer`)
    }
  }
  if (!Array.isArray(golden.gaps)) {
    problems.push('gaps is not an array')
    return problems
  }
  golden.gaps.forEach((gap, index) => {
    if (gap === null || typeof gap !== 'object' || Array.isArray(gap)) {
      problems.push(`gaps[${index}] is not an object`)
      return
    }
    if (!GAP_KINDS.includes(gap.kind)) {
      problems.push(`gaps[${index}] kind=${cellValue(gap.kind)} is not one of ${GAP_KINDS.join(', ')}`)
    }
    for (const field of GAP_REQUIRED_FIELDS) {
      if (typeof gap[field] !== 'string' || gap[field].trim() === '') problems.push(`${field} missing on gap ${index}`)
    }
  })
  return problems
}

export function summarizeGolden(golden) {
  const gaps = Array.isArray(golden.gaps) ? golden.gaps : []
  const blockingCount = gaps.filter((gap) => gap?.kind === BLOCKING_KIND).length
  const advisoryCount = gaps.filter((gap) => gap?.kind === ADVISORY_KIND).length
  return {
    declaredProviders: golden.declaredProviders ?? 0,
    runtimeProviders: golden.runtimeProviders ?? 0,
    gapCount: gaps.length,
    blockingCount,
    advisoryCount
  }
}

export function renderListing(golden) {
  const summary = summarizeGolden(golden)
  const gaps = Array.isArray(golden.gaps) ? golden.gaps : []
  const lines = [
    `golden=${GOLDEN_PATH} 声明 ${summary.declaredProviders} / 运行 ${summary.runtimeProviders} / 差异 ${summary.gapCount}（阻断 ${summary.blockingCount}，提醒 ${summary.advisoryCount}）`
  ]
  if (gaps.length === 0) {
    lines.push('无差异')
    return `${lines.join('\n')}\n`
  }
  const rows = gaps.map((gap) => LISTING_FIELDS.map((field) => cellValue(gap?.[field])))
  const widths = LISTING_FIELDS.map((field, index) =>
    Math.max(displayWidth(field), ...rows.map((row) => displayWidth(row[index])))
  )
  const renderRow = (cells) => cells.map((cell, index) => padCell(cell, widths[index])).join('  ').trimEnd()
  lines.push(renderRow([...LISTING_FIELDS]))
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

export function runProviderSync(root, options = {}) {
  const args = ['test', '-count=1', '-run', RUN_PATTERN, '-v', PACKAGE_PATH]
  const env = cleanEnv()
  if (options.update) env.UPDATE_GOLDEN = '1'
  const result = spawnSync('go', args, { cwd: resolve(root, 'backend'), env, encoding: 'utf8', timeout: 600000 })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  const golden = existsSync(resolve(root, GOLDEN_PATH)) ? readGolden(root) : null
  return {
    exitCode: result.status === null ? 1 : result.status,
    passed: counts.passed,
    failed: counts.failed,
    golden,
    summary: golden === null ? null : summarizeGolden(golden),
    listing: golden === null ? '' : renderListing(golden),
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
      const golden = readGolden(repoRoot)
      const problems = validateGolden(golden)
      process.stdout.write(renderListing(golden))
      if (reportProblems(problems)) process.exit(EXIT_PROVIDER_SYNC)
      if (summarizeGolden(golden).blockingCount > 0) process.exit(EXIT_PROVIDER_SYNC)
      return
    }
    const result = runProviderSync(repoRoot, { update })
    process.stdout.write(result.listing)
    process.stdout.write(`go test passed=${result.passed} failed=${result.failed} exit=${result.exitCode}\n`)
    if (result.golden !== null) {
      if (reportProblems(validateGolden(result.golden))) process.exit(EXIT_PROVIDER_SYNC)
      if (result.summary.blockingCount > 0) process.exit(EXIT_PROVIDER_SYNC)
    }
    if (result.exitCode !== 0) process.exit(EXIT_PROVIDER_SYNC)
    if (update) process.stdout.write(`updated=true review=${GOLDEN_PATH}\n`)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_PROVIDER_SYNC)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
