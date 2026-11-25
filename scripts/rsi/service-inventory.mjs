import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'

const FORBIDDEN = [
  { pattern: /\/Users\/[^"\s]+/, reason: '绝对用户路径' },
  { pattern: /local-token/i, reason: '本地认证令牌文件名' },
  { pattern: /bearer\s+[A-Za-z0-9._-]+/i, reason: 'Bearer 凭据' },
  { pattern: /sk-[A-Za-z0-9]{16,}/, reason: '疑似 API Key' }
]

export function scanPrivacy(value, path = '$') {
  const violations = []
  if (typeof value === 'string') {
    for (const rule of FORBIDDEN) {
      if (rule.pattern.test(value)) violations.push({ path, reason: rule.reason, sample: value.slice(0, 12) })
    }
    return violations
  }
  if (Array.isArray(value)) {
    value.forEach((item, index) => violations.push(...scanPrivacy(item, `${path}[${index}]`)))
    return violations
  }
  if (value && typeof value === 'object') {
    for (const [key, item] of Object.entries(value)) violations.push(...scanPrivacy(item, `${path}.${key}`))
  }
  return violations
}

export function buildInventory(baseline) {
  if (!baseline?.routes || !baseline?.health || !baseline?.toolchain) {
    throw new Error('基线缺少 routes/health/toolchain，无法生成服务清单')
  }
  const verdict = baseline.health.verdict
  const instanceDigest = baseline.instance?.instanceId
    ? createHash('sha256').update(baseline.instance.instanceId).digest('hex').slice(0, 16)
    : null

  const services = [
    {
      id: 'SVC-CORE-HTTP',
      kind: 'process',
      detail: { version: baseline.instance?.version ?? null, instanceDigest, port: 'dynamic' }
    },
    {
      id: 'SVC-ROUTER',
      kind: 'route-table',
      detail: {
        total: baseline.routes.total,
        distinctPaths: baseline.routes.distinctPaths,
        byMethod: baseline.routes.byMethod,
        prefixes: baseline.routes.prefixes,
        digest: baseline.routes.digest
      }
    },
    {
      id: 'SVC-EXTENSION-OPENAPI',
      kind: 'contract',
      detail: { pathCount: baseline.openapi.pathCount, operationCount: baseline.openapi.operationCount, digest: baseline.openapi.digest }
    },
    {
      id: 'SVC-HEALTH',
      kind: 'probe',
      detail: {
        ready: verdict.ready,
        database: verdict.database,
        deployMode: verdict.deployMode,
        runtimeState: verdict.runtimeState,
        readyCount: verdict.readyCount,
        blockingCount: verdict.blockingCount,
        failedCount: verdict.failedCount,
        shapeDigest: baseline.health.shapeDigest
      }
    },
    {
      id: 'SVC-ORCHESTRATOR',
      kind: 'runtime',
      detail: { ready: verdict.orchestratorReady }
    },
    {
      id: 'SVC-UNIFIED-ENTRY',
      kind: 'runtime',
      detail: { ready: verdict.unifiedEntryReady, backpressure: verdict.backpressure }
    },
    {
      id: 'SVC-STORAGE',
      kind: 'storage',
      detail: {
        writerOpen: verdict.storageWriterOpen,
        readerOpen: verdict.storageReaderOpen,
        busyErrors: verdict.storageBusyErrors,
        ioErrors: verdict.storageIOErrors
      }
    },
    {
      id: 'SVC-TOOLCHAIN',
      kind: 'toolchain',
      detail: { checks: baseline.toolchain.checks.length, ok: baseline.toolchain.ok }
    },
    {
      id: 'SVC-ENTRYPOINTS',
      kind: 'source',
      detail: { anchors: baseline.entries.length, missing: baseline.entries.filter((item) => !item.exists).length }
    },
    {
      id: 'SVC-PROBES',
      kind: 'probe',
      detail: { statuses: (baseline.probes ?? []).map((item) => [item.name, item.status]) }
    }
  ]

  const ids = services.map((item) => item.id)
  const problems = []
  if (new Set(ids).size !== ids.length) problems.push({ code: 'DUPLICATE_SERVICE_ID', message: '服务 ID 重复' })
  for (const id of ids) {
    if (!/^SVC-[A-Z0-9-]+$/.test(id)) problems.push({ code: 'BAD_SERVICE_ID', message: `服务 ID 非法：${id}` })
  }

  const inventory = {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
    sourceBaseline: { digest: baseline.digest, fingerprint: baseline.fingerprint ?? null },
    services,
    problems
  }
  const privacy = scanPrivacy(inventory)
  return { inventory, privacy, ok: problems.length === 0 && privacy.length === 0 }
}

function parseArgs(argv) {
  const options = { baseline: 'artifacts/rsi/rsi-baseline.json', out: 'artifacts/rsi/service-inventory.json' }
  for (let index = 0; index < argv.length; index += 1) {
    if (argv[index] === '--baseline') options.baseline = argv[index + 1]
    else if (argv[index] === '--out') options.out = argv[index + 1]
  }
  return options
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const options = parseArgs(process.argv.slice(2))
  const baselinePath = resolve(repoRoot, options.baseline)
  if (!existsSync(baselinePath)) {
    process.stderr.write(`基线不存在：${baselinePath}\n`)
    process.exit(4)
  }
  const result = buildInventory(JSON.parse(readFileSync(baselinePath, 'utf8')))
  const target = resolve(repoRoot, options.out)
  mkdirSync(dirname(target), { recursive: true })
  writeFileSync(target, `${JSON.stringify({ ...result.inventory, privacy: result.privacy, ok: result.ok }, null, 2)}\n`, 'utf8')
  process.stdout.write(`${JSON.stringify({ ok: result.ok, out: target, services: result.inventory.services.length, privacy: result.privacy }, null, 2)}\n`)
  if (!result.ok) {
    process.stderr.write(`服务清单不可用：problems=${result.inventory.problems.length} privacy=${result.privacy.length}\n`)
    process.exit(15)
  }
}
