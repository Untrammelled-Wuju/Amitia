import assert from 'node:assert/strict'
import test from 'node:test'
import { parseGinRoutes, summarizeRoutes } from '../route-snapshot.mjs'
import { healthSnapshot, openapiSummary, shapeOf } from '../instance-surface.mjs'

const LOG = [
  '[AmitiaCore] [GIN-debug] [WARNING] Running in "debug" mode.',
  '[AmitiaCore] [GIN-debug] GET    /api/game-center/health   --> github.com/u-ai/backend/internal/gamehost/management.(*Handler).GetCenterHealth-fm (3 handlers)',
  '[AmitiaCore] [GIN-debug] POST   /api/extensions/kernel/extensions/enable --> github.com/u-ai/backend/internal/extension.(*Router).EnableExtension-fm (4 handlers)',
  '[AmitiaCore] [GIN-debug] GET    /api/health               --> github.com/u-ai/backend/internal/system.(*Handler).Health-fm (2 handlers)',
  'unrelated line'
].join('\n')

test('GIN 日志解析出方法、路径、处理器与中间件数量', () => {
  const entries = parseGinRoutes(LOG)
  assert.equal(entries.length, 3)
  assert.deepEqual(entries.map((entry) => `${entry.method} ${entry.path}`), [
    'POST /api/extensions/kernel/extensions/enable',
    'GET /api/game-center/health',
    'GET /api/health'
  ])
  assert.equal(entries[0].handlers, 4)
  assert.match(entries[0].handler, /EnableExtension-fm$/)
})

test('路由摘要按方法与前缀计数并生成稳定摘要', () => {
  const entries = parseGinRoutes(LOG)
  const summary = summarizeRoutes(entries)
  assert.equal(summary.total, 3)
  assert.equal(summary.distinctPaths, 3)
  assert.equal(summary.byMethod.GET, 2)
  assert.equal(summary.byMethod.POST, 1)
  assert.equal(summary.prefixes[0].prefix, '/api')
  assert.equal(summary.digest, summarizeRoutes(parseGinRoutes(LOG)).digest)
  assert.notEqual(summary.digest, summarizeRoutes(parseGinRoutes(LOG.replace('/api/health ', '/api/healthz '))).digest)
})

test('健康快照保留原载荷并对结构求摘要', () => {
  const body = {
    code: 200,
    data: {
      health: true,
      ready: true,
      database: 'ok',
      deployMode: 'desktop-local',
      checks: {
        runtime: { state: 'ready', readyCount: 8, ready: [], modules: [{ name: 'a' }, { name: 'b' }] },
        orchestrator: { ready: true }
      }
    }
  }
  const snapshot = healthSnapshot(body)
  assert.equal(snapshot.verdict.readyCount, 8)
  assert.equal(snapshot.verdict.runtimeState, 'ready')
  assert.equal(snapshot.verdict.orchestratorReady, true)
  assert.equal(snapshot.verdict.storageWriterOpen, null)
  assert.deepEqual(snapshot.shape.checks.runtime.readyCount, 'number')
  assert.equal(snapshot.shape.checks.runtime.modules, 'array')
  const again = healthSnapshot({ ...body, data: { ...body.data, checks: { ...body.data.checks, runtime: { ...body.data.checks.runtime, readyCount: 9 } } } })
  assert.equal(again.shapeDigest, snapshot.shapeDigest)
})

test('shapeOf 对数组折叠为类型而不暴露长度', () => {
  assert.equal(shapeOf([1, 2, 3]), 'array')
  assert.deepEqual(shapeOf({ b: 1, a: null }), { a: 'null', b: 'number' })
})

test('OpenAPI 摘要统计路径与操作且忽略描述文本变化', () => {
  const document = {
    openapi: '3.0.3',
    paths: {
      '/extensions/kernel/status': { get: { summary: 'x' } },
      '/extensions/pet/plugins': { get: {}, post: {} }
    }
  }
  const summary = openapiSummary(document)
  assert.equal(summary.pathCount, 2)
  assert.equal(summary.operationCount, 3)
  assert.equal(summary.openapi, '3.0.3')
  const changed = openapiSummary({ ...document, paths: { ...document.paths, '/extensions/kernel/status': { get: { summary: 'y' } } } })
  assert.equal(changed.digest, summary.digest)
})
