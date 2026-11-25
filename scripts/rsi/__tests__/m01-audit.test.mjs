import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { collectChangedFiles, computeVerdict, expandDirectories, inScope, sha256File } from '../m01-audit.mjs'

test('inScope 只接受 M01 声明的路径前缀', () => {
  assert.equal(inScope('scripts/rsi/m01-audit.mjs'), true)
  assert.equal(inScope('backend/internal/extension/kernel/featuregate/gate.go'), true)
  assert.equal(inScope('backend/cmd/server/main.go'), true)
  assert.equal(inScope('.github/workflows/rsi-gates.yml'), true)
  assert.equal(inScope('front/src/router/builtinRoutes.ts'), true)
  assert.equal(inScope('front/src/pages/Index.vue'), false)
  assert.equal(inScope('backend/internal/other/x.go'), false)
  assert.equal(inScope('AmıtiaData/app.db'), false)
})

test('collectChangedFiles 从 porcelain 解析、过滤并排序', () => {
  const lines = [
    ' M backend/cmd/server/main.go',
    '?? scripts/rsi/m01-audit.mjs',
    ' M front/src/App.vue',
    ' M scripts/rsi/lib/core-instance.mjs',
    'D  backend/cmd/server/old.go'
  ]
  const files = collectChangedFiles(lines)
  assert.deepEqual(files.map((file) => file.path), [
    'backend/cmd/server/main.go',
    'backend/cmd/server/old.go',
    'scripts/rsi/lib/core-instance.mjs',
    'scripts/rsi/m01-audit.mjs'
  ])
  assert.deepEqual(files.map((file) => file.status), ['M', 'D', 'M', '??'])
})

test('expandDirectories 把未跟踪目录展开为文件并再次过滤范围', () => {
  const entries = [
    { status: '??', path: 'scripts/rsi/__tests__' },
    { status: 'M', path: 'backend/config/config.go' }
  ]
  const listed = []
  const expanded = expandDirectories(entries, (path) => {
    listed.push(path)
    return ['scripts/rsi/__tests__/m01-audit.test.mjs', 'docs/outside.md']
  })
  assert.deepEqual(listed, ['scripts/rsi/__tests__'])
  assert.deepEqual(expanded.map((entry) => entry.path), [
    'backend/config/config.go',
    'scripts/rsi/__tests__/m01-audit.test.mjs'
  ])
  assert.equal(expanded[1].status, '??')
})

test('computeVerdict 在全部 PASSED 且门禁通过时才 READY', () => {
  const steps = [{ id: 'M01-S01', status: 'PASSED', evidenceComplete: true }]
  const gates = [{ id: 'G', ok: true }]
  assert.equal(computeVerdict(steps, gates).verdict, 'READY_FOR_APPROVAL')
  assert.deepEqual(computeVerdict(steps, gates).blockers, [])

  const inProgress = computeVerdict([{ id: 'M01-S06', status: 'IN_PROGRESS', evidenceComplete: true }], gates)
  assert.equal(inProgress.verdict, 'PENDING_APPROVAL')
  assert.deepEqual(inProgress.blockers, ['M01-S06 状态为 IN_PROGRESS'])

  const missingEvidence = computeVerdict([{ id: 'M01-S01', status: 'PASSED', evidenceComplete: false }], gates)
  assert.deepEqual(missingEvidence.blockers, ['M01-S01 证据不完整'])

  const failingGate = computeVerdict(steps, [{ id: 'G', ok: false }])
  assert.deepEqual(failingGate.blockers, ['门禁 G 未通过'])
})

test('sha256File 与标准实现一致，篡改后必然不同', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm01-audit-'))
  try {
    const file = join(dir, 'evidence.json')
    writeFileSync(file, '{"ok":true}\n', 'utf8')
    const expected = createHash('sha256').update('{"ok":true}\n').digest('hex')
    assert.equal(sha256File(file), expected)
    writeFileSync(file, '{"ok":false}\n', 'utf8')
    assert.notEqual(sha256File(file), expected)
    mkdirSync(join(dir, 'sub'))
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})
