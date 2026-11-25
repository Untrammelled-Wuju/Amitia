import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import test from 'node:test'
import { disableProviders, diffRouteSets, readFeatures, withFeatures } from '../featuregate-drill.mjs'
import { scanFeatureGate } from '../featuregate-scan.mjs'

const GATE_SOURCE = `package featuregate

const (
	NameRuntimeCanary  = "runtime-canary"
	NameRouteInventory = "route-inventory"
)

func Parse(raw map[string]any) (*Registry, error) { return nil, nil }
func Configure(raw map[string]any) error { return nil }
func Enabled(name string) bool { return false }
func Apply(ctx any, name string, hooks Hooks) (Outcome, error) { return Outcome{}, nil }
func Snapshot() Report { return Report{} }

type Hooks struct{}
type Outcome struct{ Enabled bool }
type Report struct{}
type Registry struct{ states map[string]bool }

func newStateMap() map[string]bool {
	states := map[string]bool{}
	for _, name := range []string{NameRuntimeCanary} {
		states[name] = false
	}
	return states
}
`

const MAIN_SOURCE = `package main

import "github.com/u-ai/backend/internal/extension/kernel/featuregate"

var runtimeFeatures = config.AppCfg.Features

func main() {
	_ = featuregate.Configure(runtimeFeatures)
	_ = featuregate.Enabled(featuregate.NameRouteInventory)
	_, _ = featuregate.Apply(nil, featuregate.NameRuntimeCanary, featuregate.Hooks{})
	ensureSingleInstance(serverAddr)
}

func ensureSingleInstance(addr string) {}
`

const ROUTES_SOURCE = `package main

import "github.com/u-ai/backend/internal/extension/kernel/featuregate"

func registerFeatureGateRoutes() {
	if !featuregate.Enabled(featuregate.NameRouteInventory) {
		return
	}
	_ = "/features"
}
`

function fixture() {
  const root = mkdtempSync(resolve(tmpdir(), 'rsi-featuregate-'))
  const write = (relativePath, contents) => {
    const absolute = resolve(root, relativePath)
    mkdirSync(resolve(absolute, '..'), { recursive: true })
    writeFileSync(absolute, contents)
  }
  write('backend/internal/extension/kernel/featuregate/gate.go', GATE_SOURCE)
  write('backend/cmd/server/main.go', MAIN_SOURCE)
  write('backend/cmd/server/feature_gate_routes.go', ROUTES_SOURCE)
  write('backend/cmd/server/unrelated.go', 'package main\n')
  write('scripts/rsi/templates/clean-root.config.yml', 'server:\n  port: 0\n')
  return { root, write }
}

test('扫描真实仓库通过且记录单点读入口', () => {
  const result = scanFeatureGate()
  assert.deepEqual(result.violations, [])
  assert.deepEqual(result.facts.configReaders, ['backend/cmd/server/main.go'])
  assert.ok(result.facts.callSites.some((site) => site.call === 'Enabled'))
  assert.ok(result.facts.callSites.some((site) => site.call === 'Apply'))
})

test('扫描能阻断模板默认开启、环境变量旁路、原始名称与越权导入', () => {
  const { root, write } = fixture()
  try {
    assert.equal(scanFeatureGate(root).ok, true)

    write('scripts/rsi/templates/clean-root.config.yml', 'server:\n  port: 0\nfeatures:\n  route-inventory: true\n')
    write('backend/cmd/server/unrelated.go', 'package main\n\nconst bypass = "AMITIA_FEATURE_RUNTIME"\n')
    write('backend/cmd/server/extra_importer.go', 'package main\n\nimport "github.com/u-ai/backend/internal/extension/kernel/featuregate"\n\nvar _ = featuregate.Enabled("route-inventory")\n')
    const result = scanFeatureGate(root)
    const codes = result.violations.map((item) => item.code)
    assert.ok(codes.includes('TEMPLATE_FEATURE_ENABLED'))
    assert.ok(codes.includes('ENV_BYPASS'))
    assert.ok(codes.includes('IMPORTER_NOT_ALLOWED'))
    assert.ok(codes.includes('CALL_SITE_RAW_NAME'))
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('扫描能阻断缺失 gate、默认开启与启动顺序错误', () => {
  const { root, write } = fixture()
  try {
    write('backend/internal/extension/kernel/featuregate/gate.go', GATE_SOURCE.replace('states[name] = false', 'states[name] = true'))
    write('backend/cmd/server/main.go', MAIN_SOURCE.replace('_ = featuregate.Configure(runtimeFeatures)', 'ensureSingleInstance(serverAddr)'))
    const result = scanFeatureGate(root)
    const codes = result.violations.map((item) => item.code)
    assert.ok(codes.includes('GATE_DEFAULT_TRUE'))
    assert.ok(codes.includes('GATE_DEFAULT_NOT_FALSE'))
    assert.ok(codes.includes('CONFIGURE_MISSING'))
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('关闭全部 providers 时保留 components 与 runtime 段', () => {
  const template = ['providers:', '  vectorStore:', '    enabled: true', 'components:', '  taskHost:', '    enabled: true', 'runtime:', '  taskHost:', '    enabled: true', ''].join('\n')
  const output = disableProviders(template)
  const lines = output.split('\n')
  assert.equal(lines[2], '    enabled: false')
  assert.equal(lines[5], '    enabled: true')
  assert.equal(lines[8], '    enabled: true')
})

test('features 段生成与解析可往返，默认缺省为空', () => {
  const text = withFeatures('server:\n  port: 0\n', { 'runtime-canary': true, 'route-inventory': false })
  assert.deepEqual(readFeatures(text), { 'runtime-canary': true, 'route-inventory': false })
  assert.deepEqual(readFeatures('server:\n  port: 0\n'), {})
})

test('路由差分能识别新增与删除', () => {
  const baseline = [
    { method: 'GET', path: '/api/a', handler: 'a' },
    { method: 'GET', path: '/api/b', handler: 'b' }
  ]
  const variant = [
    { method: 'GET', path: '/api/a', handler: 'a' },
    { method: 'POST', path: '/api/c', handler: 'c' }
  ]
  const delta = diffRouteSets(baseline, variant)
  assert.deepEqual(delta.added.map((route) => `${route.method} ${route.path}`), ['POST /api/c'])
  assert.deepEqual(delta.removed.map((route) => `${route.method} ${route.path}`), ['GET /api/b'])
})
