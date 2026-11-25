import { createHash } from 'node:crypto'
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'

const baselinePath = resolve(repoRoot, 'scripts', 'rsi', 'templates', 'frontend-route-baseline.json')
const outcomePath = resolve(repoRoot, 'artifacts', 'rsi', 'm01-s06-frontend-route-guard.json')
const sources = [
  resolve(repoRoot, 'front', 'src', 'router', 'builtinRoutes.ts'),
  resolve(repoRoot, 'front', 'src', 'router', 'index.ts')
]

function stripComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1')
}

function readStringToken(text, index) {
  const quote = text[index]
  let cursor = index + 1
  let value = ''
  while (cursor < text.length) {
    const char = text[cursor]
    if (char === '\\') {
      value += text[cursor + 1]
      cursor += 2
      continue
    }
    if (char === quote) return { value, end: cursor + 1 }
    value += char
    cursor += 1
  }
  throw new Error(`未闭合的字符串字面量: ${text.slice(index, index + 40)}`)
}

const CLOSERS = { '[': ']', '{': '}', '(': ')' }

export function matchBracket(text, index) {
  const open = text[index]
  const close = CLOSERS[open]
  if (!close) throw new Error(`不支持的括号: ${open}`)
  let depth = 0
  for (let cursor = index; cursor < text.length; cursor += 1) {
    const char = text[cursor]
    if (char === '"' || char === "'" || char === '`') {
      cursor = readStringToken(text, cursor).end - 1
      continue
    }
    if (char === '/' && text[cursor + 1] === '/') {
      const newline = text.indexOf('\n', cursor)
      cursor = newline < 0 ? text.length : newline
      continue
    }
    if (char === open) depth += 1
    if (char === close) {
      depth -= 1
      if (depth === 0) return cursor
    }
  }
  throw new Error(`未闭合的 ${open}: ${text.slice(index, index + 60)}`)
}

export function splitTopLevel(text) {
  const items = []
  let start = 0
  for (let cursor = 0; cursor < text.length; cursor += 1) {
    const char = text[cursor]
    if (char === '"' || char === "'" || char === '`') {
      cursor = readStringToken(text, cursor).end - 1
      continue
    }
    if (char === '{' || char === '[' || char === '(') {
      cursor = matchBracket(text, cursor)
      continue
    }
    if (char === ',') {
      items.push(text.slice(start, cursor))
      start = cursor + 1
    }
  }
  const tail = text.slice(start)
  if (tail.trim()) items.push(tail)
  return items.map((item) => item.trim()).filter(Boolean)
}

function extractArray(text, anchorPattern) {
  const anchor = anchorPattern.exec(text)
  if (!anchor) throw new Error(`未找到路由数组锚点: ${anchorPattern}`)
  const anchorEnd = anchor.index + anchor[0].length - 1
  const bracket = text[anchorEnd] === '['
    ? anchorEnd
    : text.indexOf('[', text.indexOf('=', anchorEnd))
  if (bracket < 0) throw new Error('路由数组缺少 [')
  const end = matchBracket(text, bracket)
  return text.slice(bracket + 1, end)
}

function joinPath(parent, child) {
  if (!child) return parent ?? ''
  if (child.startsWith('/')) return child
  if (parent == null) return child
  return `${parent.replace(/\/$/, '')}/${child}`
}

function namedProp(objectText, name) {
  const pattern = new RegExp(`\\b${name}\\s*:\\s*("([^"]*)"|'([^']*)')`)
  const match = pattern.exec(objectText)
  if (!match) return null
  return match[2] ?? match[3] ?? null
}

export function routeFromObject(objectText, parentPath) {
  const childrenIndex = objectText.search(/\bchildren\s*:\s*\[/)
  let children = []
  let ownText = objectText
  if (childrenIndex >= 0) {
    const bracket = objectText.indexOf('[', childrenIndex)
    const end = matchBracket(objectText, bracket)
    children = extractRouteObjects(objectText.slice(bracket + 1, end), null)
    ownText = `${objectText.slice(0, childrenIndex)}${objectText.slice(end + 1)}`
  }
  const ownPath = namedProp(ownText, 'path')
  const fullPath = joinPath(parentPath, ownPath)
  const componentMatch = /import\(\s*["']([^"']+)["']\s*\)/.exec(ownText)
  const route = {
    path: fullPath,
    name: namedProp(ownText, 'name'),
    component: componentMatch ? componentMatch[1] : null,
    redirect: namedProp(ownText, 'redirect') ?? (/\bredirect\s*:\s*[^,}\n]+/.test(ownText) ? 'function' : null),
    runtimeCapability: namedProp(ownText, 'runtimeCapability'),
    requiresAuth: /\brequiresAuth\s*:\s*true\b/.test(ownText),
    children
  }
  return route
}

export function extractRouteObjects(arrayText, parentPath) {
  return splitTopLevel(arrayText)
    .filter((item) => item.startsWith('{'))
    .map((item) => item.slice(0, matchBracket(item, 0) + 1))
    .map((item) => routeFromObject(item, parentPath))
}

function flatten(routes, parentPath = null, output = []) {
  for (const route of routes) {
    const fullPath = joinPath(parentPath, route.path)
    output.push({
      path: fullPath,
      name: route.name,
      component: route.component,
      redirect: route.redirect,
      runtimeCapability: route.runtimeCapability,
      requiresAuth: route.requiresAuth
    })
    flatten(route.children ?? [], fullPath, output)
  }
  return output
}

export function readFrontendRoutes() {
  const routes = []
  const builtinSource = stripComments(readFileSync(sources[0], 'utf8'))
  routes.push(...flatten(extractRouteObjects(extractArray(builtinSource, /builtinBusinessRoutes\s*:/), null)))
  const indexSource = stripComments(readFileSync(sources[1], 'utf8'))
  routes.push(...flatten(extractRouteObjects(extractArray(indexSource, /routes\s*:\s*\[/), null)))
  return routes.sort((left, right) => `${left.path}|${left.name ?? ''}`.localeCompare(`${right.path}|${right.name ?? ''}`))
}

export function routeSurfaceDigest(routes) {
  return createHash('sha256')
    .update(JSON.stringify(routes.map((route) => [
      route.path,
      route.name,
      route.component,
      route.redirect,
      route.runtimeCapability,
      route.requiresAuth
    ])))
    .digest('hex')
}

export function diffSurfaces(baselineRoutes, currentRoutes) {
  const key = (route) => `${route.path}|${route.name ?? ''}`
  const baselineMap = new Map(baselineRoutes.map((route) => [key(route), route]))
  const currentMap = new Map(currentRoutes.map((route) => [key(route), route]))
  const added = currentRoutes.filter((route) => !baselineMap.has(key(route)))
  const removed = baselineRoutes.filter((route) => !currentMap.has(key(route)))
  const changed = []
  for (const [keyName, before] of baselineMap) {
    const after = currentMap.get(keyName)
    if (!after) continue
    const beforeShape = JSON.stringify([before.component, before.redirect, before.runtimeCapability, before.requiresAuth])
    const afterShape = JSON.stringify([after.component, after.redirect, after.runtimeCapability, after.requiresAuth])
    if (beforeShape !== afterShape) changed.push({ path: before.path, name: before.name, before: JSON.parse(beforeShape), after: JSON.parse(afterShape) })
  }
  return { added, removed, changed }
}

function readApproval(path) {
  if (!path) return null
  const payload = JSON.parse(readFileSync(resolve(repoRoot, path), 'utf8'))
  const verdict = payload.verdict ?? payload.status ?? payload?.data?.verdict
  return { path, verdict, payload }
}

export function runGuard(options = {}) {
  const targetPath = options.outcomePath ?? outcomePath
  const currentRoutes = readFrontendRoutes()
  const digest = routeSurfaceDigest(currentRoutes)
  const baseline = existsSync(baselinePath) ? JSON.parse(readFileSync(baselinePath, 'utf8')) : null
  const approval = readApproval(options.approval)
  const checks = []

  if (options.bootstrap) {
    if (existsSync(baselinePath)) {
      return {
        ok: false,
        accepted: false,
        digest,
        checks: [{ id: 'BOOTSTRAP_REFUSED_EXISTING_BASELINE', ok: false, detail: baselinePath }],
        reason: '基线已存在，只能通过 --accept --approval 更新'
      }
    }
    if (!options.reason) {
      return {
        ok: false,
        accepted: false,
        digest,
        checks: [{ id: 'BOOTSTRAP_REQUIRES_REASON', ok: false, detail: '缺少 --reason' }]
      }
    }
    writeFileSync(baselinePath, `${JSON.stringify({
      schemaVersion: 1,
      approvedBy: `bootstrap:${options.reason}`,
      approvedAt: new Date().toISOString(),
      digest,
      routeCount: currentRoutes.length,
      routes: currentRoutes
    }, null, 2)}\n`, 'utf8')
    return { ok: true, accepted: true, digest, routeCount: currentRoutes.length, baselinePath }
  }

  if (options.accept) {
    if (!approval || String(approval.verdict).toUpperCase() !== 'APPROVED') {
      return {
        ok: false,
        accepted: false,
        digest,
        checks: [{ id: 'ACCEPT_REQUIRES_APPROVAL', ok: false, detail: approval ? `verdict=${approval.verdict}` : '缺少 --approval' }],
        reason: '基线变更必须附带 verdict=APPROVED 的发布许可'
      }
    }
    writeFileSync(baselinePath, `${JSON.stringify({
      schemaVersion: 1,
      approvedBy: approval.path,
      approvedAt: new Date().toISOString(),
      digest,
      routeCount: currentRoutes.length,
      routes: currentRoutes
    }, null, 2)}\n`, 'utf8')
    return { ok: true, accepted: true, digest, routeCount: currentRoutes.length, baselinePath }
  }

  if (!baseline) {
    return {
      ok: false,
      digest,
      checks: [{ id: 'BASELINE_PRESENT', ok: false, detail: `缺少 ${baselinePath}` }],
      reason: '缺少冻结基线，无法判定前端路由面是否漂移'
    }
  }

  const drift = diffSurfaces(baseline.routes ?? [], currentRoutes)
  checks.push({
    id: 'FRONTEND_ROUTE_SURFACE_UNCHANGED',
    ok: drift.added.length === 0 && drift.removed.length === 0 && drift.changed.length === 0,
    detail: JSON.stringify(drift)
  })
  checks.push({
    id: 'FRONTEND_ROUTE_DIGEST_MATCH',
    ok: baseline.digest === digest,
    detail: `baseline=${baseline.digest?.slice(0, 12)} current=${digest.slice(0, 12)}`
  })
  checks.push({
    id: 'FRONTEND_ROUTE_COUNT_MATCH',
    ok: baseline.routeCount === currentRoutes.length,
    detail: `baseline=${baseline.routeCount} current=${currentRoutes.length}`
  })
  checks.push({
    id: 'FRONTEND_BASELINE_HAS_APPROVAL',
    ok: typeof baseline.approvedBy === 'string' && baseline.approvedBy.length > 0,
    detail: `approvedBy=${baseline.approvedBy ?? null}`
  })

  const summary = {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
    sources: sources.map((path) => path.replace(`${repoRoot}/`, '')),
    baselinePath,
    digest,
    routeCount: currentRoutes.length,
    baselineDigest: baseline.digest ?? null,
    baselineApprovedBy: baseline.approvedBy ?? null,
    drift,
    checks,
    ok: checks.every((item) => item.ok)
  }
  writeFileSync(targetPath, `${JSON.stringify(summary, null, 2)}\n`, 'utf8')
  return summary
}

function main() {
  const args = process.argv.slice(2)
  const accept = args.includes('--accept')
  const bootstrap = args.includes('--bootstrap')
  const approvalIndex = args.indexOf('--approval')
  const approval = approvalIndex >= 0 ? args[approvalIndex + 1] : null
  const reasonIndex = args.indexOf('--reason')
  const reason = reasonIndex >= 0 ? args[reasonIndex + 1] : null
  try {
    const summary = runGuard({ accept, approval, bootstrap, reason })
    if (summary.accepted) {
      process.stdout.write(`baseline=${baselinePath} routes=${summary.routeCount} digest=${summary.digest.slice(0, 12)}\n`)
      return
    }
    for (const item of summary.checks) {
      process.stdout.write(`${item.ok ? 'PASS' : 'FAIL'} ${item.id}${item.detail ? ` :: ${item.detail}` : ''}\n`)
    }
    process.stdout.write(`evidence=${outcomePath}\n`)
    if (!summary.ok) process.exit(18)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(18)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
