import { spawn } from 'node:child_process'
import { createHash } from 'node:crypto'
import { closeSync, existsSync, mkdirSync, openSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { cleanEnv, repoRoot, run } from '../toolchain-preflight.mjs'
import { parseGinRoutes } from '../route-snapshot.mjs'

export const configTemplatePath = resolve(repoRoot, 'scripts', 'rsi', 'templates', 'clean-root.config.yml')

export function disableProviders(configText) {
  const start = /^providers:\s*$/m.exec(configText)
  const end = /^components:\s*$/m.exec(configText)
  if (!start || !end || end.index < start.index) {
    throw new Error('干净数据根模板缺少 providers/components 段')
  }
  const head = configText.slice(0, start.index)
  const providers = configText.slice(start.index, end.index).replace(/^([ \t]+)enabled:[ \t]*true[ \t]*$/gm, '$1enabled: false')
  return `${head}${providers}${configText.slice(end.index)}`
}

export function withFeatures(configText, flags) {
  const lines = Object.entries(flags).map(([name, enabled]) => `  ${name}: ${enabled}`)
  return `${configText.replace(/\s+$/, '')}\n\nfeatures:\n${lines.join('\n')}\n`
}

export function readFeatures(configText) {
  const lines = configText.split('\n')
  const start = lines.findIndex((line) => /^features:\s*$/.test(line))
  if (start < 0) return {}
  const flags = {}
  for (const line of lines.slice(start + 1)) {
    const match = /^\s{2}([a-z0-9-]+):\s*(true|false)\s*$/.exec(line)
    if (!match) break
    flags[match[1]] = match[2] === 'true'
  }
  return flags
}

export function routeDigest(routes) {
  return createHash('sha256')
    .update(JSON.stringify(routes.map((route) => [route.method, route.path, route.handler])))
    .digest('hex')
}

export function diffRouteSets(baselineRoutes, variantRoutes) {
  const key = (route) => `${route.method} ${route.path}`
  const baselineKeys = new Set(baselineRoutes.map(key))
  const variantKeys = new Set(variantRoutes.map(key))
  return {
    added: variantRoutes.filter((route) => !baselineKeys.has(key(route))),
    removed: baselineRoutes.filter((route) => !variantKeys.has(key(route)))
  }
}

export function prepareInstanceRoot(root, flags) {
  mkdirSync(resolve(root, 'config'), { recursive: true })
  const base = disableProviders(readFileSync(configTemplatePath, 'utf8'))
  writeFileSync(resolve(root, 'config', 'config.yml'), withFeatures(base, flags))
  return root
}

export function startCore(root, binary) {
  const logPath = resolve(root, 'core.log')
  const output = openSync(logPath, 'a')
  const child = spawn(binary, ['--runtime-profile=local'], {
    cwd: root,
    env: { ...cleanEnv(), AMITIA_RUNTIME_ROOT: root, AMITIA_DATA_DIR: root },
    stdio: ['ignore', output, output]
  })
  closeSync(output)
  return { child, logPath }
}

export function sleep(ms) {
  return new Promise((resolveSleep) => setTimeout(resolveSleep, ms))
}

export async function waitEndpoint(root, timeoutMs) {
  const deadline = Date.now() + timeoutMs
  const endpointPath = resolve(root, 'run', 'endpoint.json')
  while (Date.now() < deadline) {
    if (existsSync(endpointPath)) {
      try {
        const endpoint = JSON.parse(readFileSync(endpointPath, 'utf8'))
        if (endpoint.port > 0) return endpoint
      } catch (error) {
        if (error.name !== 'SyntaxError') throw error
      }
    }
    await sleep(150)
  }
  throw new Error(`等待 ${endpointPath} 超时`)
}

export async function waitReady(baseUrl, timeoutMs) {
  const deadline = Date.now() + timeoutMs
  let last = null
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${baseUrl}/readyz`, { signal: AbortSignal.timeout(3000) })
      const body = await response.json()
      last = { status: response.status, body }
      if (response.status === 200 && body?.data?.state === 'ready') return last
    } catch (error) {
      last = { status: 0, body: error.message }
    }
    await sleep(250)
  }
  throw new Error(`等待 ${baseUrl}/readyz 就绪超时: ${JSON.stringify(last)}`)
}

export async function fetchJson(url, token) {
  const headers = token ? { 'X-Amitia-Local-Token': token } : {}
  const response = await fetch(url, { headers, signal: AbortSignal.timeout(5000) })
  const text = await response.text()
  let body = null
  try {
    body = text ? JSON.parse(text) : null
  } catch (error) {
    body = { raw: text }
  }
  return { status: response.status, body }
}

export async function stopCore(child, timeoutMs = 15000) {
  if (!child || child.exitCode !== null) return
  const exited = new Promise((resolveExit) => child.once('exit', resolveExit))
  child.kill('SIGTERM')
  const timeout = new Promise((resolveTimeout) => setTimeout(() => resolveTimeout('timeout'), timeoutMs))
  const result = await Promise.race([exited, timeout])
  if (result === 'timeout') {
    child.kill('SIGKILL')
    await exited
  }
}

export async function runCoreInstance({ name, binary, root, flags, waitForTicks = false }) {
  prepareInstanceRoot(root, flags)
  const { child, logPath } = startCore(root, binary)
  try {
    const endpoint = await waitEndpoint(root, 45000)
    const baseUrl = `http://127.0.0.1:${endpoint.port}`
    await waitReady(baseUrl, 60000)
    const token = readFileSync(resolve(root, 'security', 'local-token'), 'utf8').trim()
    const log = readFileSync(logPath, 'utf8')
    const routes = parseGinRoutes(log)
    const featuresFirst = await fetchJson(`${baseUrl}/api/features`, token)
    const kernel = await fetchJson(`${baseUrl}/api/extensions/kernel/status`, token)
    const providers = Array.isArray(kernel.body?.providers) ? kernel.body.providers : []
    const canaryProviders = providers.filter((provider) => String(provider.id).includes('featuregate.canary'))
    let featuresSecond = null
    if (waitForTicks) {
      await sleep(6500)
      featuresSecond = await fetchJson(`${baseUrl}/api/features`, token)
    }
    return {
      name,
      root,
      logPath,
      port: endpoint.port,
      routes,
      routeCount: routes.length,
      routeDigest: routeDigest(routes),
      featureStatus: featuresFirst.status,
      featureBody: featuresFirst.body,
      featureSecondStatus: featuresSecond?.status ?? null,
      featureSecondBody: featuresSecond?.body ?? null,
      canaryProviders: canaryProviders.map((provider) => ({
        id: provider.id,
        capabilityId: provider.capabilityId,
        placement: provider.placement
      })),
      subscriberRegistered: log.includes('[MessageEventBus] subscriber featuregate-canary registered'),
      disabledMarker: log.includes('[FeatureGate] runtime-canary disabled'),
      enabledMarker: log.includes('[FeatureGate] runtime-canary enabled')
    }
  } finally {
    await stopCore(child)
  }
}

export function buildCoreBinary(output, cwd) {
  mkdirSync(dirname(output), { recursive: true })
  run('go', ['build', '-buildvcs=false', '-o', output, './cmd/server'], { cwd })
}

export function buildBaselineSource(destination) {
  mkdirSync(destination, { recursive: true })
  run('bash', ['-c', `git archive HEAD backend | tar -x -C ${JSON.stringify(destination)}`], { cwd: repoRoot })
  return resolve(destination, 'backend')
}
