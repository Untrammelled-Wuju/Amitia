import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { repoRoot } from './toolchain-preflight.mjs'

const GIN_ROUTE = /\[GIN-debug\]\s+(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|Any)\s+(\S+)\s+-->\s+(.+?)\s+\((\d+) handlers?\)/

function sha256(text) {
  return createHash('sha256').update(text).digest('hex')
}

export function parseGinRoutes(logText) {
  const entries = []
  for (const line of logText.split('\n')) {
    const match = line.match(GIN_ROUTE)
    if (!match) continue
    entries.push({
      method: match[1],
      path: match[2],
      handler: match[3].trim(),
      handlers: Number.parseInt(match[4], 10)
    })
  }
  return entries.sort((left, right) => (left.path + left.method).localeCompare(right.path + right.method))
}

export function summarizeRoutes(entries) {
  const byMethod = {}
  for (const entry of entries) byMethod[entry.method] = (byMethod[entry.method] ?? 0) + 1
  const prefixes = {}
  for (const entry of entries) {
    const parts = entry.path.split('/').filter(Boolean)
    const key = parts.length > 0 ? `/${parts[0]}` : '/'
    prefixes[key] = (prefixes[key] ?? 0) + 1
  }
  const sortedPrefixes = Object.keys(prefixes)
    .sort((left, right) => prefixes[right] - prefixes[left] || left.localeCompare(right))
    .map((prefix) => ({ prefix, routes: prefixes[prefix] }))
  return {
    total: entries.length,
    distinctPaths: new Set(entries.map((entry) => entry.path)).size,
    byMethod,
    prefixes: sortedPrefixes,
    digest: sha256(JSON.stringify(entries.map((entry) => [entry.method, entry.path, entry.handler])))
  }
}

export function readRouteSnapshot(logPath) {
  const absolute = resolve(repoRoot, logPath)
  if (!existsSync(absolute)) {
    throw new Error(`路由日志不存在：${absolute}`)
  }
  const entries = parseGinRoutes(readFileSync(absolute, 'utf8'))
  if (entries.length === 0) {
    throw new Error(`路由日志中未解析到任何 [GIN-debug] 注册记录：${absolute}`)
  }
  return { source: absolute, entries, ...summarizeRoutes(entries) }
}
