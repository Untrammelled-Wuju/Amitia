import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const GOLDEN_PATH = 'backend/internal/extension/kernel/plugincontract/testdata/binding-snapshot.json'
export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestBindingSnapshot'
export const SCOPES = ['device', 'space', 'global']
export const EXIT_BINDING_SNAPSHOT = 29

const HEX_PATTERN = /^[0-9a-f]{64}$/
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
  if (!existsSync(path)) throw new Error(`binding snapshot golden missing: ${path}`)
  return JSON.parse(readFileSync(path, 'utf8'))
}

export function bindingKeyLine(key) {
  return [key?.capabilityId, key?.spaceId ?? '', key?.deviceId ?? '', key?.scope].join('|')
}

export function bindingDigest(bindings) {
  const lines = bindings.map((binding) =>
    [binding?.key?.capabilityId, binding?.key?.spaceId ?? '', binding?.key?.deviceId ?? '', binding?.key?.scope, binding?.providerId, binding?.providerInstanceId].join('|')
  )
  lines.sort()
  return createHash('sha256').update(lines.join('\n')).digest('hex')
}

export function validateGolden(golden) {
  const problems = []
  if (golden === null || typeof golden !== 'object' || Array.isArray(golden)) {
    problems.push('golden is not an object')
    return problems
  }
  if (golden.schemaVersion !== 1) problems.push(`schemaVersion=${golden.schemaVersion} want 1`)
  if (!isNonEmptyString(golden.epoch)) problems.push('epoch is empty')
  if (!Number.isInteger(golden.revision) || golden.revision < 1) problems.push(`revision=${golden.revision} want a positive integer`)
  if (!Array.isArray(golden.scopeOrder) || golden.scopeOrder.length !== SCOPES.length || golden.scopeOrder.some((scope, index) => scope !== SCOPES[index])) {
    problems.push(`scopeOrder=${JSON.stringify(golden.scopeOrder)} want ${JSON.stringify(SCOPES)}`)
  }
  const bindings = Array.isArray(golden.bindings) ? golden.bindings : []
  if (bindings.length === 0) problems.push('bindings is empty')
  const seen = new Set()
  bindings.forEach((binding, index) => {
    const key = binding?.key
    if (key === null || typeof key !== 'object' || Array.isArray(key)) {
      problems.push(`bindings[${index}] key is not an object`)
      return
    }
    if (!isNonEmptyString(key.capabilityId)) problems.push(`bindings[${index}] capabilityId is empty`)
    if (!isNonEmptyString(binding.providerId)) problems.push(`bindings[${index}] providerId is empty`)
    if (!isNonEmptyString(binding.providerInstanceId)) problems.push(`bindings[${index}] providerInstanceId is empty`)
    if (!SCOPES.includes(key.scope)) {
      problems.push(`bindings[${index}] scope=${key.scope} is not one of ${SCOPES.join(', ')}`)
      return
    }
    if (key.scope === 'device' && (!isNonEmptyString(key.spaceId) || !isNonEmptyString(key.deviceId))) {
      problems.push(`bindings[${index}] device scope requires spaceId and deviceId`)
    }
    if (key.scope === 'space' && (!isNonEmptyString(key.spaceId) || (key.deviceId ?? '') !== '')) {
      problems.push(`bindings[${index}] space scope requires spaceId and no deviceId`)
    }
    if (key.scope === 'global' && ((key.spaceId ?? '') !== '' || (key.deviceId ?? '') !== '')) {
      problems.push(`bindings[${index}] global scope must not carry spaceId or deviceId`)
    }
    const line = bindingKeyLine(key)
    if (seen.has(line)) problems.push(`bindings[${index}] duplicate key ${line}`)
    seen.add(line)
  })
  if (!HEX_PATTERN.test(String(golden.digest ?? ''))) {
    problems.push('digest is not a 64 char lowercase hex string')
  } else if (bindings.length > 0 && golden.digest !== bindingDigest(bindings)) {
    problems.push(`digest=${golden.digest} does not match recomputed ${bindingDigest(bindings)}`)
  }
  const resolutions = Array.isArray(golden.resolutions) ? golden.resolutions : []
  if (resolutions.length === 0) problems.push('resolutions is empty')
  const names = new Set()
  const covered = new Set()
  resolutions.forEach((resolution, index) => {
    if (!isNonEmptyString(resolution?.name)) problems.push(`resolutions[${index}] name is empty`)
    if (names.has(resolution?.name)) problems.push(`resolutions[${index}] duplicate name ${resolution?.name}`)
    names.add(resolution?.name)
    if (!isNonEmptyString(resolution?.request?.capabilityId)) problems.push(`resolutions[${index}] request.capabilityId is empty`)
    if (!SCOPES.includes(resolution?.scope)) {
      problems.push(`resolutions[${index}] scope=${resolution?.scope} is not one of ${SCOPES.join(', ')}`)
      return
    }
    covered.add(resolution.scope)
    if (!isNonEmptyString(resolution.providerId)) problems.push(`resolutions[${index}] providerId is empty`)
    const trail = Array.isArray(resolution.trail) ? resolution.trail : []
    if (trail.length === 0) problems.push(`resolutions[${index}] trail is empty`)
    trail.forEach((step) => {
      if (!TRAIL_PATTERN.test(String(step))) problems.push(`resolutions[${index}] trail step ${step} is malformed`)
    })
    if (trail.length > 0 && trail[trail.length - 1] !== `${resolution.scope}=hit`) {
      problems.push(`resolutions[${index}] trail does not end with ${resolution.scope}=hit`)
    }
    if (trail.slice(0, -1).some((step) => String(step).endsWith('=hit'))) {
      problems.push(`resolutions[${index}] trail hits more than once`)
    }
  })
  for (const scope of SCOPES) if (!covered.has(scope)) problems.push(`no resolution covers scope ${scope}`)
  return problems
}

export function summarizeGolden(golden) {
  const bindings = Array.isArray(golden?.bindings) ? golden.bindings : []
  const resolutions = Array.isArray(golden?.resolutions) ? golden.resolutions : []
  const scopeCounts = Object.fromEntries(SCOPES.map((scope) => [scope, 0]))
  for (const binding of bindings) if (SCOPES.includes(binding?.key?.scope)) scopeCounts[binding.key.scope] += 1
  return { bindings: bindings.length, resolutions: resolutions.length, scopeCounts }
}

export function renderGolden(golden) {
  const summary = summarizeGolden(golden)
  const scopeBreakdown = SCOPES.map((scope) => `${scope} ${summary.scopeCounts[scope]}`).join(' / ')
  const lines = [
    `golden=${GOLDEN_PATH} revision=${golden?.revision} epoch=${golden?.epoch} bindings ${summary.bindings}（${scopeBreakdown}）/ 解析 ${summary.resolutions}`,
    `digest=${golden?.digest}`
  ]
  const rows = (Array.isArray(golden?.resolutions) ? golden.resolutions : []).map((resolution) => [
    String(resolution?.name ?? ''),
    String(resolution?.request?.capabilityId ?? ''),
    String(resolution?.scope ?? ''),
    String(resolution?.providerId ?? ''),
    (Array.isArray(resolution?.trail) ? resolution.trail : []).join(' → ')
  ])
  const fields = ['用例', '能力', '命中作用域', 'Provider', '查找轨迹']
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

export function runBindingSnapshot(root, options = {}) {
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
      if (reportProblems(validateGolden(golden))) process.exit(EXIT_BINDING_SNAPSHOT)
      return
    }
    const result = runBindingSnapshot(repoRoot, { update })
    process.stdout.write(result.listing)
    process.stdout.write(`go test passed=${result.passed} failed=${result.failed} exit=${result.exitCode}\n`)
    if (result.golden !== null && reportProblems(validateGolden(result.golden))) process.exit(EXIT_BINDING_SNAPSHOT)
    if (result.exitCode !== 0) process.exit(EXIT_BINDING_SNAPSHOT)
    if (update) process.stdout.write(`updated=true review=${GOLDEN_PATH}\n`)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_BINDING_SNAPSHOT)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
