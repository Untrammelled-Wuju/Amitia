import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const GOLDEN_PATH = 'backend/internal/extension/kernel/plugincontract/testdata/id-stability.json'
export const PACKAGE_PATH = './internal/extension/kernel/plugincontract'
export const RUN_PATTERN = 'TestIDStability'

export function readGolden(root) {
  const path = resolve(root, GOLDEN_PATH)
  if (!existsSync(path)) throw new Error(`id stability golden missing: ${path}`)
  return JSON.parse(readFileSync(path, 'utf8'))
}

export function validateGolden(snapshot) {
  const problems = []
  if (snapshot.schemaVersion !== 1) problems.push(`schemaVersion=${snapshot.schemaVersion} want 1`)
  if (!Array.isArray(snapshot.entries)) {
    problems.push('entries is not an array')
    return problems
  }
  if (snapshot.count !== snapshot.entries.length) problems.push(`count=${snapshot.count} entries=${snapshot.entries.length}`)
  if (!Array.isArray(snapshot.scope) || snapshot.scope.length === 0) problems.push('scope is empty')
  const seen = new Set()
  for (const entry of snapshot.entries) {
    for (const field of ['source', 'providerId', 'serviceId', 'capabilityId', 'version']) {
      if (typeof entry[field] !== 'string' || entry[field].trim() === '') problems.push(`${field} missing on ${JSON.stringify(entry)}`)
    }
    const key = `${entry.source}|${entry.capabilityId}|${entry.providerId}`
    if (seen.has(key)) problems.push(`duplicate entry ${key}`)
    seen.add(key)
    if (!/^\d+(\.\d+)*$/.test(String(entry.version))) problems.push(`version ${entry.version} is not dotted numeric`)
  }
  return problems
}

export function summarizeGolden(snapshot) {
  const sources = {}
  for (const entry of snapshot.entries) sources[entry.source] = (sources[entry.source] ?? 0) + 1
  return {
    count: snapshot.entries.length,
    declaredCount: snapshot.count,
    sources,
    scope: snapshot.scope ?? [],
    capabilityIds: snapshot.entries.map((entry) => entry.capabilityId)
  }
}

export function renderListing(snapshot) {
  const lines = []
  lines.push(`golden=${GOLDEN_PATH} entries=${snapshot.entries.length}`)
  for (const entry of snapshot.entries) {
    lines.push(`${entry.source}\t${entry.capabilityId}\t${entry.version}\t${entry.providerId}`)
  }
  return `${lines.join('\n')}\n`
}

export function runStability(root, options = {}) {
  const args = ['test', '-count=1', '-run', RUN_PATTERN, '-v', PACKAGE_PATH]
  const env = cleanEnv()
  if (options.update) env.UPDATE_GOLDEN = '1'
  const result = spawnSync('go', args, { cwd: resolve(root, 'backend'), env, encoding: 'utf8', timeout: 600000 })
  return { exitCode: result.status === null ? 1 : result.status, body: `${result.stdout ?? ''}${result.stderr ?? ''}` }
}

function main() {
  const args = process.argv.slice(2)
  const update = args.includes('--update')
  const list = args.includes('--list')
  try {
    const snapshot = readGolden(repoRoot)
    const problems = validateGolden(snapshot)
    const summary = summarizeGolden(snapshot)
    if (list) {
      process.stdout.write(renderListing(snapshot))
      if (problems.length > 0) for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
      if (problems.length > 0) process.exit(21)
      return
    }
    const result = runStability(repoRoot, { update })
    process.stdout.write(result.body)
    process.stdout.write(`golden=${GOLDEN_PATH} entries=${summary.count} sources=${JSON.stringify(summary.sources)} exit=${result.exitCode}\n`)
    if (problems.length > 0) {
      for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
      process.exit(21)
    }
    if (result.exitCode !== 0) process.exit(21)
    if (update) process.stdout.write(`updated=true review=${GOLDEN_PATH}\n`)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(21)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
