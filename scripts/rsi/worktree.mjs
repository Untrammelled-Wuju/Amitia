import { existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { validateIsolation } from './isolation-guard.mjs'
import { repoRoot, run } from './toolchain-preflight.mjs'

export const WORKTREE_ROOT = resolve(repoRoot, 'artifacts', 'rsi', 'worktrees')

export function listWorktrees() {
  return run('git', ['worktree', 'list', '--porcelain'])
    .split('\n\n')
    .filter(Boolean)
    .map((block) => {
      const lines = block.split('\n')
      const path = (lines[0] || '').replace(/^worktree\s+/, '')
      const head = (lines.find((line) => line.startsWith('HEAD ')) || '').replace(/^HEAD\s+/, '')
      const branch = (lines.find((line) => line.startsWith('branch ')) || '').replace(/^branch\s+/, '')
      const detached = lines.includes('detached')
      return { path, head, branch, detached }
    })
}

export function assertIsolated(module) {
  const worktreePath = resolve(WORKTREE_ROOT, module)
  const sandbox = resolve(worktreePath, '.rsi-sandbox')
  const report = validateIsolation({
    dataRoot: resolve(sandbox, 'data'),
    pluginRoot: resolve(sandbox, 'plugins'),
    cacheRoot: resolve(sandbox, 'cache'),
    dbPath: resolve(sandbox, 'data', 'rsi.db'),
    port: 18765,
    worktreePath
  })
  if (!report.ok) {
    throw new Error(
      `模块 ${module} 隔离校验失败：\n${report.violations.map((item) => `- [${item.code}] ${item.message}`).join('\n')}`
    )
  }
  return { worktreePath, sandbox, report }
}

export function createWorktree(module, base = 'HEAD') {
  const { worktreePath, sandbox } = assertIsolated(module)
  if (existsSync(worktreePath)) {
    const registered = listWorktrees().some((item) => resolve(item.path) === worktreePath)
    if (!registered) {
      throw new Error(`目录已存在但未注册为 worktree：${worktreePath}`)
    }
    const entry = listWorktrees().find((item) => resolve(item.path) === worktreePath)
    return { created: false, worktreePath, sandbox, head: entry.head }
  }
  const head = run('git', ['rev-parse', base])
  run('git', ['worktree', 'add', '--detach', worktreePath, head])
  return { created: true, worktreePath, sandbox, head }
}

export function removeWorktree(module) {
  const worktreePath = resolve(WORKTREE_ROOT, module)
  const registered = listWorktrees().some((item) => resolve(item.path) === worktreePath)
  if (!registered) {
    return { removed: false, worktreePath, reason: 'not-registered' }
  }
  run('git', ['worktree', 'remove', '--force', worktreePath])
  run('git', ['worktree', 'prune'])
  return { removed: true, worktreePath }
}

function parseArgs(argv) {
  const options = { action: 'create', base: 'HEAD' }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === '--module') options.module = argv[index + 1]
    else if (arg === '--base') options.base = argv[index + 1]
    else if (arg === '--remove') options.action = 'remove'
    else if (arg === '--list') options.action = 'list'
  }
  return options
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const options = parseArgs(process.argv.slice(2))
  if (options.action === 'list') {
    process.stdout.write(`${JSON.stringify({ worktrees: listWorktrees(), root: WORKTREE_ROOT }, null, 2)}\n`)
  } else {
    if (!options.module) {
      process.stderr.write('必须提供 --module，例如 --module M01\n')
      process.exit(4)
    }
    const result = options.action === 'remove' ? removeWorktree(options.module) : createWorktree(options.module, options.base)
    process.stdout.write(`${JSON.stringify(result, null, 2)}\n`)
  }
}
