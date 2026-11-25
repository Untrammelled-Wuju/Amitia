import assert from 'node:assert/strict'
import { resolve } from 'node:path'
import test from 'node:test'
import { FORBIDDEN_PORTS, detectInstanceProcesses, validateIsolation } from '../isolation-guard.mjs'
import { repoRoot } from '../toolchain-preflight.mjs'
import { assertIsolated } from '../worktree.mjs'

const baseConfig = (overrides = {}) => ({
  dataRoot: resolve(repoRoot, 'artifacts', 'rsi', 'sandbox', 'data'),
  pluginRoot: resolve(repoRoot, 'artifacts', 'rsi', 'sandbox', 'plugins'),
  cacheRoot: resolve(repoRoot, 'artifacts', 'rsi', 'sandbox', 'cache'),
  dbPath: resolve(repoRoot, 'artifacts', 'rsi', 'sandbox', 'data', 'rsi.db'),
  port: 18766,
  allowRunningInstance: true,
  ...overrides
})

test('隔离沙箱通过校验', () => {
  const report = validateIsolation(baseConfig())
  assert.equal(report.ok, true, JSON.stringify(report.violations))
})

test('拒绝生产数据根目录', () => {
  const report = validateIsolation(baseConfig({ dataRoot: resolve(repoRoot, 'AmitiaData') }))
  assert.equal(report.ok, false)
  assert.ok(report.violations.some((item) => item.code === 'PRODUCTION_PATH'))
})

test('拒绝生产插件目录', () => {
  const report = validateIsolation(baseConfig({ pluginRoot: resolve(repoRoot, 'Plugin') }))
  assert.equal(report.ok, false)
  assert.ok(report.violations.some((item) => item.code === 'PRODUCTION_PATH'))
})

test('拒绝仓库内非 artifacts/rsi 路径', () => {
  const report = validateIsolation(baseConfig({ cacheRoot: resolve(repoRoot, 'front', 'src', 'cache') }))
  assert.equal(report.ok, false)
  assert.ok(report.violations.some((item) => item.code === 'WORKSPACE_PATH'))
})

test('拒绝生产数据库文件', () => {
  const report = validateIsolation(baseConfig({ dbPath: resolve(repoRoot, 'AmitiaData', 'amitia.db') }))
  assert.equal(report.ok, false)
  assert.ok(report.violations.some((item) => item.code === 'PRODUCTION_PATH' || item.code === 'PRODUCTION_DB'))
})

test('拒绝保留端口', () => {
  for (const port of FORBIDDEN_PORTS) {
    const report = validateIsolation(baseConfig({ port }))
    assert.equal(report.ok, false, `端口 ${port} 必须被拒绝`)
    assert.ok(report.violations.some((item) => item.code === 'FORBIDDEN_PORT'))
  }
})

test('缺少配置项立即失败', () => {
  const report = validateIsolation({ dataRoot: '', pluginRoot: '', cacheRoot: '', dbPath: '', port: null })
  assert.equal(report.ok, false)
  assert.ok(report.violations.some((item) => item.code === 'MISSING_PATH'))
  assert.ok(report.violations.some((item) => item.code === 'MISSING_PORT'))
})

test('模块工作树沙箱位于 artifacts/rsi/worktrees', () => {
  const { worktreePath, sandbox } = assertIsolated('M-UNIT')
  assert.equal(worktreePath, resolve(repoRoot, 'artifacts', 'rsi', 'worktrees', 'M-UNIT'))
  assert.equal(sandbox, resolve(worktreePath, '.rsi-sandbox'))
})

test('实例进程识别只命中真实 Amitia 进程', () => {
  const core = `${resolve(repoRoot, 'desktop', 'resources', 'core', 'macos-arm64')}/AmitiaCore --runtime-profile=local`
  const electron = `${resolve(repoRoot, 'desktop', 'node_modules', '.pnpm', 'electron@43', 'node_modules', 'electron', 'dist', 'Electron.app', 'Contents', 'MacOS', 'Electron')} .`
  const gradle = `/bin/zsh -lc cd ${resolve(repoRoot, 'mobile_app', 'android')} && ./gradlew :amitia-runtime:testDebugUnitTest --tests "com.amitia.amitia_app.runtime.install.PackageVerifierTest"`
  const unrelated = '/usr/bin/node /opt/other/app.mjs'
  const detected = detectInstanceProcesses([core, electron, gradle, unrelated, `node ${resolve(repoRoot, 'scripts', 'rsi', 'perf-probe.mjs')} AmitiaCore --runtime-profile=local`])
  assert.equal(detected.length, 2)
  assert.ok(detected.includes(core))
  assert.ok(detected.includes(electron))
})
