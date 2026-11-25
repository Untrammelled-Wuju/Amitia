import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const INVENTORY_PATH = 'backend/internal/extension/kernel/plugincontract/testdata/core-call-inventory.json'
export const ROUTE_SNAPSHOT_PATH = 'artifacts/rsi/baseline/final-1.json'
export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = [
  'TestCoreCallInventoryCoversEveryBusinessDomain',
  'TestCoreCallInventoryMatchesGolden',
  'TestCoreCallInventoryRecordsSideEffects',
  'TestCoreCallInventoryMarksUnknownEntries',
  'TestUIHostAuditTracksWebAndFlutter',
  'TestUIHostCoverageMarksUnknownEndpoints'
].join('|')
export const HOSTS = ['web', 'flutter']
export const HANDLER_KINDS = ['method', 'function', 'closure', 'factory', 'delegation']
export const SIDE_EFFECTS = ['db.read', 'db.write', 'file.write', 'stream.write']
export const EXIT_CORE_INVENTORY = 28

const HEX_PATTERN = /^[0-9a-f]{64}$/
const WIDE_CHAR_PATTERN = /[\u1100-\u115F\u2E80-\u303E\u3041-\u33FF\u3400-\u4DBF\u4E00-\u9FFF\uA000-\uA4CF\uAC00-\uD7A3\uF900-\uFAFF\uFE30-\uFE6F\uFF00-\uFF60\uFFE0-\uFFE6]/

function displayWidth(value) {
  let width = 0
  for (const char of value) width += WIDE_CHAR_PATTERN.test(char) ? 2 : 1
  return width
}

function padCell(value, width) {
  return `${value}${' '.repeat(Math.max(0, width - displayWidth(value)))}`
}

function isPositiveInteger(value) {
  return Number.isInteger(value) && value >= 0
}

export function readInventory(root) {
  const path = resolve(root, INVENTORY_PATH)
  if (!existsSync(path)) throw new Error(`core call inventory missing: ${path}`)
  return JSON.parse(readFileSync(path, 'utf8'))
}

export function readRouteSnapshot(root) {
  const path = resolve(root, ROUTE_SNAPSHOT_PATH)
  if (!existsSync(path)) throw new Error(`route snapshot missing: ${path}`)
  const snapshot = JSON.parse(readFileSync(path, 'utf8'))
  const entries = snapshot?.routes?.entries
  return Array.isArray(entries) ? entries : []
}

export function validateInventory(inventory, routeEntries = null) {
  const problems = []
  if (inventory === null || typeof inventory !== 'object' || Array.isArray(inventory)) {
    problems.push('inventory is not an object')
    return problems
  }
  if (inventory.schemaVersion !== 1) problems.push(`schemaVersion=${inventory.schemaVersion} want 1`)
  for (const field of ['routes', 'mapped', 'unmapped']) {
    if (!isPositiveInteger(inventory[field])) problems.push(`${field}=${inventory[field]} is not a non-negative integer`)
  }
  if (inventory.mapped + inventory.unmapped !== inventory.routes) {
    problems.push(`mapped+unmapped=${inventory.mapped + inventory.unmapped} != routes=${inventory.routes}`)
  }
  if (inventory.unmapped !== 0) problems.push(`unmapped=${inventory.unmapped} blocks the full coverage claim`)
  if (!HEX_PATTERN.test(String(inventory.digest ?? ''))) problems.push('digest is not a 64 char lowercase hex string')
  const domains = Array.isArray(inventory.domains) ? inventory.domains : []
  if (domains.length === 0) problems.push('domains is empty')
  const seenDomains = new Set()
  let domainRoutes = 0
  for (const [index, domain] of domains.entries()) {
    if (domain === null || typeof domain !== 'object') {
      problems.push(`domains[${index}] is not an object`)
      continue
    }
    if (typeof domain.domain !== 'string' || domain.domain.trim() === '') problems.push(`domains[${index}] name is empty`)
    if (seenDomains.has(domain.domain)) problems.push(`domains[${index}] duplicate ${domain.domain}`)
    seenDomains.add(domain.domain)
    if (!isPositiveInteger(domain.routes) || !isPositiveInteger(domain.mapped) || !isPositiveInteger(domain.unmapped)) {
      problems.push(`domains[${index}] counters are not non-negative integers`)
      continue
    }
    if (domain.mapped === 0) problems.push(`domains[${index}] ${domain.domain} has no mapped route`)
    if (domain.mapped + domain.unmapped !== domain.routes) {
      problems.push(`domains[${index}] ${domain.domain} mapped+unmapped != routes`)
    }
    domainRoutes += domain.routes
  }
  if (domains.length > 0 && domainRoutes !== inventory.routes) {
    problems.push(`domain routes=${domainRoutes} != routes=${inventory.routes}`)
  }
  const kinds = inventory.handlerKinds ?? {}
  const kindTotal = Object.values(kinds).reduce((sum, value) => sum + value, 0)
  if (kindTotal !== inventory.routes) problems.push(`handlerKinds total=${kindTotal} != routes=${inventory.routes}`)
  for (const kind of Object.keys(kinds)) {
    if (!HANDLER_KINDS.includes(kind)) problems.push(`handlerKinds has unknown kind ${kind}`)
  }
  const effects = inventory.sideEffects ?? {}
  for (const effect of SIDE_EFFECTS) {
    if (!isPositiveInteger(effects[effect]) || effects[effect] === 0) problems.push(`sideEffects.${effect} is empty`)
  }
  const hosts = Array.isArray(inventory.uiHosts) ? inventory.uiHosts : []
  const hostNames = hosts.map((host) => host?.host)
  for (const host of HOSTS) {
    if (!hostNames.includes(host)) problems.push(`uiHosts missing ${host}`)
  }
  for (const [index, host] of hosts.entries()) {
    if (host === null || typeof host !== 'object') {
      problems.push(`uiHosts[${index}] is not an object`)
      continue
    }
    if (!HOSTS.includes(host.host)) problems.push(`uiHosts[${index}] unknown host ${host.host}`)
    for (const field of ['files', 'calls', 'linked', 'prefixes', 'unlinked']) {
      if (!isPositiveInteger(host[field])) problems.push(`uiHosts[${index}].${field} is not a non-negative integer`)
    }
    if (host.linked + host.prefixes + host.unlinked !== host.calls) {
      problems.push(`uiHosts[${index}] linked+prefixes+unlinked != calls`)
    }
    if (host.calls === 0) problems.push(`uiHosts[${index}] ${host.host} has no tracked calls`)
    if (host.linked === 0) problems.push(`uiHosts[${index}] ${host.host} links to no backend route`)
    if (!Array.isArray(host.unlinkedPaths)) {
      problems.push(`uiHosts[${index}].unlinkedPaths is not an array`)
    } else if (host.unlinked === 0 && host.unlinkedPaths.length !== 0) {
      problems.push(`uiHosts[${index}] unlinkedPaths must be empty when unlinked is 0`)
    } else if (host.unlinked > 0 && host.unlinkedPaths.length === 0) {
      problems.push(`uiHosts[${index}] unlinkedPaths is empty while unlinked=${host.unlinked}`)
    }
    if (!HEX_PATTERN.test(String(host.digest ?? ''))) problems.push(`uiHosts[${index}] digest is not a 64 char lowercase hex string`)
  }
  if (Array.isArray(routeEntries) && routeEntries.length > 0 && routeEntries.length !== inventory.routes) {
    problems.push(`route snapshot entries=${routeEntries.length} != inventory routes=${inventory.routes}`)
  }
  return problems
}

export function summarizeInventory(inventory) {
  const domains = Array.isArray(inventory.domains) ? inventory.domains : []
  const hosts = Array.isArray(inventory.uiHosts) ? inventory.uiHosts : []
  const effects = inventory.sideEffects ?? {}
  return {
    routes: inventory.routes,
    mapped: inventory.mapped,
    unmapped: inventory.unmapped,
    coverage: inventory.routes === 0 ? '0.00%' : `${((inventory.mapped / inventory.routes) * 100).toFixed(2)}%`,
    domains: domains.length,
    smallestDomain: domains.length === 0 ? null : domains.reduce((min, domain) => (domain.routes < min.routes ? domain : min), domains[0]),
    hosts: hosts.map((host) => `${host.host} ${host.linked}/${host.calls}`),
    effects
  }
}

export function renderInventory(inventory) {
  const summary = summarizeInventory(inventory)
  const effects = SIDE_EFFECTS.map((effect) => `${effect} ${summary.effects[effect] ?? 0}`).join(' / ')
  const lines = [
    `inventory=${INVENTORY_PATH} routes ${summary.mapped}/${summary.routes}（${summary.coverage}）domains ${summary.domains} unmapped ${summary.unmapped}`
  ]
  lines.push(`sideEffects: ${effects}`)
  lines.push(`uiHosts: ${summary.hosts.join(' | ')}`)
  lines.push(`digest=${inventory.digest}`)
  const rows = (Array.isArray(inventory.domains) ? inventory.domains : []).map((domain) => [
    String(domain.domain),
    String(domain.routes),
    String(domain.mapped),
    String(domain.unmapped)
  ])
  const header = ['domain', 'routes', 'mapped', 'unmapped']
  const widths = header.map((cell, index) => Math.max(displayWidth(cell), ...rows.map((row) => displayWidth(row[index]))))
  const renderRow = (cells) => cells.map((cell, index) => padCell(cell, widths[index])).join('  ').trimEnd()
  lines.push(renderRow(header))
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

export function runCoreInventory(root, options = {}) {
  const args = ['test', '-count=1', '-run', RUN_PATTERN, '-v', PACKAGE_PATH]
  const env = cleanEnv()
  if (options.update) env.UPDATE_GOLDEN = '1'
  const result = spawnSync('go', args, { cwd: resolve(root, 'backend'), env, encoding: 'utf8', timeout: 900000 })
  const log = `${result.stdout ?? ''}${result.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  const inventory = existsSync(resolve(root, INVENTORY_PATH)) ? readInventory(root) : null
  const routeEntries = existsSync(resolve(root, ROUTE_SNAPSHOT_PATH)) ? readRouteSnapshot(root) : null
  return {
    exitCode: result.status === null ? 1 : result.status,
    passed: counts.passed,
    failed: counts.failed,
    inventory,
    listing: inventory === null ? '' : renderInventory(inventory),
    problems: inventory === null ? ['inventory is missing'] : validateInventory(inventory, routeEntries),
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
      const inventory = readInventory(repoRoot)
      process.stdout.write(renderInventory(inventory))
      if (reportProblems(validateInventory(inventory, readRouteSnapshot(repoRoot)))) process.exit(EXIT_CORE_INVENTORY)
      return
    }
    const result = runCoreInventory(repoRoot, { update })
    process.stdout.write(result.listing)
    process.stdout.write(`go test passed=${result.passed} failed=${result.failed} exit=${result.exitCode}\n`)
    if (reportProblems(result.problems)) process.exit(EXIT_CORE_INVENTORY)
    if (result.exitCode !== 0) process.exit(EXIT_CORE_INVENTORY)
    if (update) process.stdout.write(`updated=true review=${INVENTORY_PATH}\n`)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(EXIT_CORE_INVENTORY)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
