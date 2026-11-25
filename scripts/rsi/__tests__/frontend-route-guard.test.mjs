import assert from 'node:assert/strict'
import { existsSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import {
  diffSurfaces,
  readFrontendRoutes,
  routeSurfaceDigest,
  runGuard
} from '../frontend-route-guard.mjs'

test('真实前端路由面可解析为绝对路径且命名不重复', () => {
  const routes = readFrontendRoutes()
  assert.ok(routes.length >= 90, `解析到的路由过少: ${routes.length}`)
  const relative = routes.filter((route) => !route.path.startsWith('/'))
  assert.deepEqual(relative, [], `存在相对路径: ${JSON.stringify(relative.slice(0, 3))}`)
  const named = routes.map((route) => route.name).filter(Boolean)
  assert.equal(new Set(named).size, named.length, '命名路由存在重复')
})

test('冻结的前端路由基线必须与当前源码一致', () => {
  const dir = mkdtempSync(join(tmpdir(), 'front-route-guard-'))
  try {
    const summary = runGuard({ outcomePath: join(dir, 'outcome.json') })
    assert.equal(summary.ok, true, JSON.stringify(summary.checks))
    assert.ok(summary.routeCount >= 90)
    assert.equal(summary.digest, summary.baselineDigest)
    assert.ok(existsSync(join(dir, 'outcome.json')), '守卫应写入指定的证据路径')
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('路由面差分能识别新增、删除与组件替换', () => {
  const baseline = [
    { path: '/a', name: 'a', component: '@/a.vue', redirect: null, runtimeCapability: null, requiresAuth: true },
    { path: '/b', name: 'b', component: '@/b.vue', redirect: null, runtimeCapability: null, requiresAuth: true }
  ]
  const current = [
    { path: '/a', name: 'a', component: '@/a2.vue', redirect: null, runtimeCapability: null, requiresAuth: true },
    { path: '/c', name: 'c', component: '@/c.vue', redirect: null, runtimeCapability: null, requiresAuth: true }
  ]
  const drift = diffSurfaces(baseline, current)
  assert.deepEqual(drift.added.map((route) => route.path), ['/c'])
  assert.deepEqual(drift.removed.map((route) => route.path), ['/b'])
  assert.deepEqual(drift.changed.map((route) => route.path), ['/a'])
})

test('路由指纹对 runtimeCapability 与鉴权标记敏感', () => {
  const base = [{ path: '/x', name: 'x', component: '@/x.vue', redirect: null, runtimeCapability: null, requiresAuth: true }]
  const gated = [{ ...base[0], runtimeCapability: 'gameMode' }]
  const unauthenticated = [{ ...base[0], requiresAuth: false }]
  const digest = routeSurfaceDigest(base)
  assert.notEqual(routeSurfaceDigest(gated), digest)
  assert.notEqual(routeSurfaceDigest(unauthenticated), digest)
})
