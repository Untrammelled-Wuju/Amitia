import { createHash } from 'node:crypto'

const DEFAULT_TIMEOUT_MS = 8000

export const HEALTH_PROBES = [
  { name: 'health', path: '/api/health', auth: false },
  { name: 'circuitBreakers', path: '/api/health/circuit-breakers', auth: false },
  { name: 'coreInfo', path: '/api/public/core/info', auth: false },
  { name: 'kernelStatus', path: '/api/extensions/kernel/status', auth: true },
  { name: 'extensionOpenapi', path: '/api/extensions/openapi.json', auth: true }
]

function sha256(value) {
  return createHash('sha256').update(typeof value === 'string' ? value : JSON.stringify(value)).digest('hex')
}

export function shapeOf(value) {
  if (Array.isArray(value)) return 'array'
  if (value === null) return 'null'
  if (typeof value === 'object') {
    const shape = {}
    for (const key of Object.keys(value).sort()) shape[key] = shapeOf(value[key])
    return shape
  }
  return typeof value
}

export function healthSnapshot(body) {
  const data = body?.data ?? null
  const shape = shapeOf(data)
  const storage = data?.storage ?? null
  return {
    payload: body,
    shape,
    shapeDigest: sha256(shape),
    verdict: {
      code: body?.code ?? null,
      health: data?.health ?? null,
      ready: data?.ready ?? null,
      database: data?.database ?? null,
      deployMode: data?.deployMode ?? null,
      version: data?.version ?? null,
      model: data?.model ?? null,
      web: data?.web ?? null,
      uptimeSeconds: data?.uptime ?? null,
      runtimeState: data?.checks?.runtime?.state ?? null,
      readyCount: data?.checks?.runtime?.readyCount ?? null,
      blockingCount: data?.checks?.runtime?.blockingCount ?? null,
      failedCount: data?.checks?.runtime?.failedCount ?? null,
      orchestratorReady: data?.checks?.orchestrator?.ready ?? null,
      unifiedEntryReady: data?.checks?.unifiedEntry?.ready ?? null,
      backpressure: data?.checks?.unifiedEntry?.backpressure ?? null,
      storageWriterOpen: storage?.writerOpen ?? null,
      storageReaderOpen: storage?.readerOpen ?? null,
      storageBusyErrors: storage?.busyErrors ?? null,
      storageIOErrors: storage?.ioErrors ?? null
    }
  }
}

export function openapiSummary(document) {
  const paths = document?.paths && typeof document.paths === 'object' ? document.paths : {}
  const entries = []
  for (const path of Object.keys(paths).sort()) {
    for (const method of Object.keys(paths[path] ?? {}).sort()) {
      entries.push(`${method.toUpperCase()} ${path}`)
    }
  }
  return {
    openapi: document?.openapi ?? null,
    pathCount: Object.keys(paths).length,
    operationCount: entries.length,
    digest: sha256(entries)
  }
}

async function request(baseUrl, probe, token, timeoutMs) {
  const headers = { Accept: 'application/json', Origin: 'http://127.0.0.1:5178' }
  if (probe.auth && token) headers['X-Amitia-Local-Token'] = token
  const startedAt = Date.now()
  try {
    const response = await fetch(`${baseUrl}${probe.path}`, { headers, signal: AbortSignal.timeout(timeoutMs) })
    const text = await response.text()
    let body = null
    try {
      body = JSON.parse(text)
    } catch {
      body = text.slice(0, 500)
    }
    return { status: response.status, body, elapsedMs: Date.now() - startedAt }
  } catch (error) {
    return { status: 0, body: { error: error.message }, elapsedMs: Date.now() - startedAt }
  }
}

export async function probeSurface(baseUrl, token, options = {}) {
  const timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS
  const results = {}
  for (const probe of HEALTH_PROBES) {
    results[probe.name] = { path: probe.path, auth: probe.auth, ...(await request(baseUrl, probe, token, timeoutMs)) }
  }

  const healthProbe = results.health
  if (healthProbe.status !== 200) {
    throw new Error(`/api/health 未返回 200：status=${healthProbe.status} body=${JSON.stringify(healthProbe.body)}`)
  }

  const openapiProbe = results.extensionOpenapi
  const openapi = openapiProbe.status === 200 ? openapiSummary(openapiProbe.body) : null

  const probes = Object.entries(results).map(([name, value]) => ({
    name,
    path: value.path,
    auth: value.auth,
    status: value.status,
    elapsedMs: value.elapsedMs
  }))

  return {
    capturedAt: new Date().toISOString(),
    baseUrl,
    health: healthSnapshot(healthProbe.body),
    openapi,
    probes
  }
}
