import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { preflight, repoRoot, run } from './toolchain-preflight.mjs'
import { probeSurface } from './instance-surface.mjs'
import { readRouteSnapshot } from './route-snapshot.mjs'
import { localToken, readEndpoint } from './perf-probe.mjs'

const BASELINE_FILES = [
  'U-AI-source-20261122-003647.zip',
  'AGENTS.md',
  'backend/go.mod',
  'backend/go.sum',
  'front/package.json',
  'mobile_app/pubspec.yaml',
  'mobile_app/pubspec.lock',
  'desktop/package.json'
]

const ENTRY_ANCHORS = [
  'backend/cmd/server',
  'backend/cmd/admin-server',
  'backend/cmd/amitia-ext',
  'backend/cmd/extension-registry-keygen',
  'backend/internal/agent',
  'backend/internal/agentrun',
  'backend/internal/mindruntime',
  'backend/internal/continuity',
  'backend/internal/devicemesh/business',
  'backend/internal/uiagent',
  'backend/internal/extension/kernel/host_api',
  'backend/internal/extension/kernel/runtime_supervisor',
  'backend/internal/extension/kernel/task_runtime',
  'backend/internal/extension/kernel/plugincontract',
  'backend/internal/extension/kernel/capability',
  'backend/internal/extension/kernel/trusted_service',
  'front/src/ui-runtime',
  'front/src/router',
  'mobile_app/lib/core/ui_runtime',
  'plugins-code',
  'Plugin'
]

const ISOLATION_OBJECTS = ['AmitiaData', 'Plugin', 'plugins-code', '.amitia-secrets', 'desktop/release']

function sha256(buffer) {
  return createHash('sha256').update(buffer).digest('hex')
}

function exists(relativePath) {
  return existsSync(resolve(repoRoot, relativePath))
}

function countFiles(relativePath) {
  const absolute = resolve(repoRoot, relativePath)
  if (!existsSync(absolute)) return null
  const stack = [absolute]
  let files = 0
  while (stack.length > 0) {
    const current = stack.pop()
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const child = resolve(current, entry.name)
      if (entry.isDirectory()) stack.push(child)
      else if (entry.isFile()) files += 1
    }
  }
  return files
}

function hashFile(relativePath) {
  const absolute = resolve(repoRoot, relativePath)
  if (!existsSync(absolute)) return null
  const buffer = readFileSync(absolute)
  return { path: relativePath, sha256: sha256(buffer), bytes: buffer.length }
}

export function buildBaseline(options = {}) {
  const sourceHashes = BASELINE_FILES.map(hashFile).filter(Boolean)
  const entries = ENTRY_ANCHORS.map((relativePath) => ({
    path: relativePath,
    exists: exists(relativePath),
    files: countFiles(relativePath)
  }))
  const isolationObjects = ISOLATION_OBJECTS.map((relativePath) => ({
    path: relativePath,
    absolute: resolve(repoRoot, relativePath),
    exists: exists(relativePath)
  }))
  const toolchain = preflight({ allowAnyNode: process.env.RSI_ALLOW_ANY_NODE === '1' })

  const missing = entries.filter((entry) => !entry.exists).map((entry) => entry.path)
  if (missing.length > 0) {
    throw new Error(`入口锚点不存在，基线无效：${missing.join(', ')}`)
  }
  if (!toolchain.ok) {
    throw new Error(`工具链不满足要求，基线标记 BLOCKED：\n${toolchain.failures.join('\n')}`)
  }

  const payload = {
    schemaVersion: 1,
    repoRoot,
    git: {
      branch: run('git', ['rev-parse', '--abbrev-ref', 'HEAD']),
      head: run('git', ['rev-parse', 'HEAD'])
    },
    sourceHashes,
    entries,
    isolationObjects,
    toolchain: { ok: toolchain.ok, checks: toolchain.checks },
    instance: options.instance ?? null,
    health: options.health ?? null,
    openapi: options.openapi ?? null,
    routes: options.routes ?? null,
    probes: options.probes ?? null
  }
  const digest = sha256(JSON.stringify(payload))
  return { ...payload, digest, fingerprint: fingerprintOf(payload) }
}

export function fingerprintOf(payload) {
  return sha256(
    JSON.stringify({
      git: payload.git,
      sourceHashes: payload.sourceHashes,
      entries: payload.entries,
      isolationObjects: (payload.isolationObjects ?? []).map((item) => [item.path, item.exists]),
      toolchain: (payload.toolchain?.checks ?? []).map((item) => [item.name, item.actual, item.ok]),
      instance: payload.instance ? [payload.instance.version, payload.instance.instanceId] : null,
      health: payload.health
        ? [payload.health.shapeDigest, payload.health.verdict.ready, payload.health.verdict.database, payload.health.verdict.deployMode]
        : null,
      routes: payload.routes ? [payload.routes.total, payload.routes.digest] : null,
      openapi: payload.openapi ? [payload.openapi.pathCount, payload.openapi.operationCount, payload.openapi.digest] : null,
      probes: payload.probes ? payload.probes.map((probe) => [probe.name, probe.status]) : null
    })
  )
}

export async function captureInstance(instanceUrl, dataRoot) {
  const endpoint = readEndpoint(dataRoot)
  if (!endpoint) throw new Error(`未找到 ${dataRoot}/run/endpoint.json，实例尚未启动`)
  const baseUrl = instanceUrl ?? endpoint.httpBaseURL
  const token = localToken(dataRoot)
  if (!token) throw new Error(`未找到 ${dataRoot}/security/local-token，无法鉴权探测`)
  const surface = await probeSurface(baseUrl, token)
  return {
    instance: {
      pid: endpoint.pid,
      port: endpoint.port,
      version: endpoint.version ?? null,
      instanceId: endpoint.instanceId ?? null,
      startedAt: endpoint.startedAt,
      endpointFile: endpoint.endpointFile
    },
    health: surface.health,
    openapi: surface.openapi,
    probes: surface.probes
  }
}

function parseArgs(argv) {
  const options = { out: 'artifacts/rsi/rsi-baseline.json', deterministic: false }
  for (let index = 0; index < argv.length; index += 1) {
    if (argv[index] === '--out') options.out = argv[index + 1]
    else if (argv[index] === '--deterministic') options.deterministic = true
    else if (argv[index] === '--data-root') options.dataRoot = argv[index + 1]
    else if (argv[index] === '--instance') options.instanceUrl = argv[index + 1]
    else if (argv[index] === '--route-log') options.routeLog = argv[index + 1]
  }
  return options
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  const surface = options.dataRoot ? await captureInstance(options.instanceUrl, resolve(options.dataRoot)) : {}
  const routes = options.routeLog ? readRouteSnapshot(options.routeLog) : null
  const baseline = buildBaseline({ ...surface, routes })
  const record = options.deterministic ? baseline : { ...baseline, generatedAt: new Date().toISOString() }
  const target = resolve(repoRoot, options.out)
  mkdirSync(dirname(target), { recursive: true })
  writeFileSync(target, `${JSON.stringify(record, null, 2)}\n`, 'utf8')
  process.stdout.write(
    `${JSON.stringify(
      {
        out: target,
        digest: baseline.digest,
        fingerprint: baseline.fingerprint,
        git: baseline.git,
        health: baseline.health?.verdict ?? null,
        healthShapeDigest: baseline.health?.shapeDigest ?? null,
        routes: baseline.routes ? { total: baseline.routes.total, digest: baseline.routes.digest } : null,
        openapi: surface.openapi ?? null
      },
      null,
      2
    )}\n`
  )
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    process.stderr.write(`基线生成失败：${error.message}\n${error.stack ?? ''}\n`)
    process.exit(9)
  })
}
