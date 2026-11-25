import assert from 'node:assert/strict'
import { lstatSync, mkdirSync, mkdtempSync, rmSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'
import { configTemplate, provisionCleanRoot } from '../clean-root.mjs'
import { repoRoot } from '../toolchain-preflight.mjs'

const scratch = resolve(repoRoot, 'artifacts', 'rsi', '__tests__', 'clean-root')

test('干净数据根配置模板禁止固定端口与生产数据目录', () => {
  const text = configTemplate()
  assert.match(text, /^\s*port:\s*0\s*$/m)
  assert.match(text, /^\s*dataDir:\s*"data"\s*$/m)
  assert.doesNotMatch(text, /lanPort\s*:/)
  assert.doesNotMatch(text, /AmitiaData/)
  assert.doesNotMatch(text, /\/Users\//)
})

test('生产路径或仓库内非 artifacts/rsi 路径被拒绝', () => {
  assert.throws(() => provisionCleanRoot(resolve(repoRoot, 'AmitiaData', 'rsi-root')), /隔离规则/)
  assert.throws(() => provisionCleanRoot(resolve(repoRoot, 'backend', 'rsi-root')), /隔离规则/)
})

test('缺少 node 运行时立即失败并给出来源', () => {
  rmSync(scratch, { recursive: true, force: true })
  assert.throws(
    () => provisionCleanRoot(scratch, { nodeSource: resolve(scratch, 'missing-node') }),
    /node 运行时缺失/
  )
})

test('重复创建需显式 --force，否则拒绝覆盖', () => {
  const base = resolve(repoRoot, 'artifacts', 'rsi', '__tests__')
  mkdirSync(base, { recursive: true })
  const root = mkdtempSync(resolve(base, 'clean-root-'))
  try {
    const first = provisionCleanRoot(root)
    assert.equal(first.configPath, resolve(root, 'config', 'config.yml'))
    assert.equal(lstatSync(resolve(root, 'node')).isSymbolicLink(), true)
    assert.throws(() => provisionCleanRoot(root), /已存在配置/)
    const second = provisionCleanRoot(root, { force: true })
    assert.equal(second.configDigest, first.configDigest)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
