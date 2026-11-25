import assert from 'node:assert/strict'
import test from 'node:test'
import { buildInventory, scanPrivacy } from '../service-inventory.mjs'

function baseline() {
  return {
    digest: 'baseline-digest',
    fingerprint: 'fingerprint',
    instance: { version: '26.2.0-beta.23', instanceId: 'space_local_001' },
    routes: { total: 1201, distinctPaths: 973, byMethod: { GET: 483 }, prefixes: { '/api': 1170 }, digest: 'routes-digest' },
    openapi: { pathCount: 264, operationCount: 296, digest: 'openapi-digest' },
    health: {
      shapeDigest: 'shape-digest',
      verdict: {
        ready: true,
        database: 'ok',
        deployMode: 'desktop-local',
        runtimeState: 'ready',
        readyCount: 8,
        blockingCount: 0,
        failedCount: 0,
        orchestratorReady: true,
        unifiedEntryReady: true,
        backpressure: 'normal',
        storageWriterOpen: true,
        storageReaderOpen: true,
        storageBusyErrors: 0,
        storageIOErrors: 0
      }
    },
    toolchain: { ok: true, checks: [{ name: 'node' }] },
    entries: [{ path: 'backend', exists: true }],
    probes: [{ name: 'health', status: 200 }]
  }
}

test('服务清单为每个默认调用分配固定 ID 且不泄露实例标识', () => {
  const { inventory, privacy, ok } = buildInventory(baseline())
  assert.equal(ok, true)
  assert.equal(privacy.length, 0)
  const ids = inventory.services.map((item) => item.id)
  assert.deepEqual(ids, [
    'SVC-CORE-HTTP',
    'SVC-ROUTER',
    'SVC-EXTENSION-OPENAPI',
    'SVC-HEALTH',
    'SVC-ORCHESTRATOR',
    'SVC-UNIFIED-ENTRY',
    'SVC-STORAGE',
    'SVC-TOOLCHAIN',
    'SVC-ENTRYPOINTS',
    'SVC-PROBES'
  ])
  const core = inventory.services.find((item) => item.id === 'SVC-CORE-HTTP')
  assert.equal(core.detail.instanceDigest.length, 16)
  assert.equal(JSON.stringify(inventory).includes('space_local_001'), false)
})

test('清单保留真实路由与健康数值', () => {
  const { inventory } = buildInventory(baseline())
  const router = inventory.services.find((item) => item.id === 'SVC-ROUTER')
  assert.equal(router.detail.total, 1201)
  assert.equal(router.detail.digest, 'routes-digest')
  const health = inventory.services.find((item) => item.id === 'SVC-HEALTH')
  assert.equal(health.detail.readyCount, 8)
})

test('绝对路径与凭据出现时隐私扫描必须报出', () => {
  const violations = scanPrivacy({ note: 'data=/Users/someone/AmitiaData', header: 'Authorization: Bearer abc.def.ghi' })
  assert.ok(violations.some((item) => item.reason === '绝对用户路径'))
  assert.ok(violations.some((item) => item.reason === 'Bearer 凭据'))
})

test('基线缺少必需分区时拒绝生成', () => {
  const broken = baseline()
  delete broken.routes
  assert.throws(() => buildInventory(broken), /基线缺少 routes\/health\/toolchain/)
})
