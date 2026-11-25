import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const GOLDEN_PATH = 'backend/internal/extension/kernel/plugincontract/testdata/provider-selection.json'
export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestSelectProviderCandidates|TestProviderSelectionMatchesGolden'
export const SCOPES = ['device', 'space', 'global']
export const EXIT_PROVIDER_SELECTION = 30

const TRAIL_PATTERN = /^(device|space|global)=(hit|miss|skip)$/
const WIDE_CHAR_PATTERN = /[\u1100-\u115F\u2E80-\u303E\u3041-\u33FF\u3400-\u4DBF\u4E00-\u9FFF\uA000-\uA4CF\uAC00-\uD7A3\uF900-\uFAFF\uFE30-\uFE6F\uFF00-\uFF60\uFFE0-\uFFE6]/

function displayWidth(value) {
  let width = 0
  for (const char of value) width += WIDE_CHAR_PATTERN.test(char) ? 2 : 1
  return width
}

function padCell(value, width) {
  return `${value}${' '.repeat(Math.max(0, width - displayWidth(value)))}`
}

function isNonEmptyString(value) {
  return typeof value === 'string' && value.trim() !== ''
}

export function readGolden(root) {
  const path = resolve(root, GOLDEN_PATH)
  if (!existsSync(path)) throw new Error(`provider selection golden missing: ${path}`)
  return JSON.parse(readFileSync(path, 'utf8'))
}

export function validateGolden(golden) {
  const problems = []
  if (golden === null || typeof golden !== 'object' || Array.isArray(golden)) {
    problems.push('golden is not an object')
    return problems
  }
  if (golden.schemaVersion !== 1) problems.push(`schemaVersion=${golden.schemaVersion} want 1`)
  if (!isNonEmptyString(golden.serviceId)) problems.push('serviceId is empty')
  if (!Number.isInteger(golden.revision) || golden.revision < 1) problems.push(`revision=${golden.revision} want a positive integer`)
  if (!SCOPES.includes(golden.scope)) problems.push(`scope=${golden.scope} is not one of ${SCOPES.join(', ')}`)
  const candidates = Array.isArray(golden.candidates) ? golden.candidates : []
  if (candidates.length === 0) problems.push('candidates is empty')
  const seen = new Set()
  candidates.forEach((candidate, index) => {
    if (!isNonEmptyString(candidate?.providerId)) {
      problems.push(`candidates[${index}] providerId is empty`)
      return
    }
    if (seen.has(candidate.providerId)) problems.push(`candidates[${index}] duplicate provider ${candidate.providerId}`)
    seen.add(candidate.providerId)
    if (!Number.isInteger(candidate.priority)) problems.push(`candidates[${index}] priority is not an integer`)
    if (index > 0) {
      const previous = candidates[index - 1]
      const previousPriority = previous?.priority ?? 0
      const priority = candidate.priority ?? 0
      if (previousPriority < priority) problems.push(`candidates[${index}] priority ${priority} outranks previous ${previousPriority}`)
      if (previousPriority === priority && String(previous?.providerId ?? '') > String(candidate.providerId)) {
        problems.push(`candidates[${index}] tie break not ascending at ${candidate.providerId}`)
      }
    }
  })
  if (!isNonEmptyString(golden.selected)) problems.push('selected is empty')
  if (seen.size > 0 && !seen.has(golden.selected)) problems.push(`selected=${golden.selected} is not among candidates`)
  const trail = Array.isArray(golden.trail) ? golden.trail : []
  if (trail.length === 0) problems.push('trail is empty')
  trail.forEach((step) => {
    if (!TRAIL_PATTERN.test(String(step))) problems.push(`trail step ${step} is malformed`)
  })
  if (trail.length > 0 && trail[trail.length - 1] !== `${golden.scope}=hit`) {
    problems.push(`trail does not end with ${golden.scope}=hit`)
  }
  if (trail.slice(0, -1).some((step) => String(step).endsWith('=hit'))) problems.push('trail hits more than once')
  return problems
}

export function summarizeGolden(golden) {
  const candidates = Array.isArray(golden?.candidates) ? golden.candidates : []
  return {
    candidates: candidates.length,
    selected: golden?.selected ?? '',
    topPriority: candidates.length > 0 ? candidates[0].priority : null,
    scope: golden?.scope ?? ''
  }
}

export function renderGolden(golden) {
  const summary = summarizeGolden(golden)
  const lines = [
    `golden=${GOLDEN_PATH} service=${golden?.serviceId} revision=${golden?.revision} scope=${summary.scope} 候选 ${summary.candidates} / 命中 ${summary.selected}`,
    `trail=${(Array.isArray(golden?.trail) ? golden.trail : []).join(' → ')}`
  ]
  const rows = (Array.isArray(golden?.candidates) ? golden.candidates : []).map((candidate) => [
    String(candidate?.providerId ?? ''),
    String(candidate?.priority ?? ''),
    candidate?.providerId === golden?.selected ? '命中' : '备选'
  ])
  const fields = ['Provider', '优先级', '结果']
  const widths = fields.map((field, index) => Math.max(displayWidth(field), ...rows.map((row) => displayWidth(row[index]))))
  const renderRow = (cells) => cells.map((cell, index) => padCell(cell, widths[index])).join('  ').trimEnd()
  lines.push(renderRow(fields))
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

export function runProviderSelection(root, options = {}) {
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
    listing: golden === null ? '' : renderGolden(golden),
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
      process.stdout.write(renderGolden(golden))
      if (reportProblems(validateGolden(golden))) process.exit(EXIT_PROVIDER_SELECTION)
      return
    }
    const result = runProviderSelection(repoRoot, { update })
    process.stdout.write(result.listing)
    process.stdout.write(`go test passed=${result.passed} failed=${result.failed} exit=${result.exitCode}\n`)
    if (result.golden !== null && reportProblems(validateGolden(result.golden))) process.exit(EXIT_PROVIDER_SELECTION)
    if (result.exitCode !== 0) process.exit(EXIT_PROVIDER_SELECTION)
    if (update) process.stdout.write(`updated=true review=${GOLDEN_PATH}\n`)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_PROVIDER_SELECTION)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
