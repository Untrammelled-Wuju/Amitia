import { createHash } from 'node:crypto'
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { ALLOWED_WORKSPACE_PREFIX, validateIsolation } from './isolation-guard.mjs'
import { repoRoot, run } from './toolchain-preflight.mjs'

export const CONFIG_TEMPLATE = resolve(repoRoot, 'scripts', 'rsi', 'templates', 'clean-root.config.yml')

export function configTemplate() {
  const text = readFileSync(CONFIG_TEMPLATE, 'utf8')
  if (/lanPort\s*:/.test(text)) {
    throw new Error(`干净数据根配置不得固定 lanPort：${CONFIG_TEMPLATE}`)
  }
  if (!/^\s*port:\s*0\s*$/m.test(text)) {
    throw new Error(`干净数据根配置必须使用动态 HTTP 端口 port: 0：${CONFIG_TEMPLATE}`)
  }
  if (!/^\s*dataDir:\s*"data"\s*$/m.test(text)) {
    throw new Error(`干净数据根配置的 storage.dataDir 必须为相对 "data"：${CONFIG_TEMPLATE}`)
  }
  return text
}

export function provisionCleanRoot(root, options = {}) {
  const target = resolve(root)
  const isolation = validateIsolation({
    dataRoot: target,
    pluginRoot: resolve(target, 'Plugin'),
    cacheRoot: resolve(target, 'cache'),
    dbPath: resolve(target, 'app.db'),
    port: 0,
    worktreePath: target,
    allowRunningInstance: options.allowRunningInstance === true
  })
  if (!isolation.ok) {
    throw new Error(`干净数据根违反隔离规则：\n${isolation.violations.map((item) => `- [${item.code}] ${item.message}`).join('\n')}`)
  }

  const nodeSource = resolve(options.nodeSource ?? resolve(repoRoot, 'AmitiaData', 'node'))
  const nodeBinary = resolve(nodeSource, 'bin', 'node')
  if (!existsSync(nodeBinary)) {
    throw new Error(`node 运行时缺失：${nodeBinary}（实例必须在启动前具备 amitia://runtime/node 来源，否则插件宿主 RUNTIME_START_FAILED）`)
  }

  const configPath = resolve(target, 'config', 'config.yml')
  if (existsSync(configPath) && options.force !== true) {
    throw new Error(`干净数据根已存在配置：${configPath}（如需重建请显式 --force）`)
  }

  const text = configTemplate()
  mkdirSync(dirname(configPath), { recursive: true })
  writeFileSync(configPath, text, 'utf8')

  const nodeTarget = resolve(target, 'node')
  rmSync(nodeTarget, { recursive: true, force: true })
  const linkNode = options.linkNode !== false
  if (linkNode) symlinkSync(nodeSource, nodeTarget, 'dir')
  else cpSync(nodeSource, nodeTarget, { recursive: true })

  return {
    root: target,
    workspacePrefix: resolve(repoRoot, ALLOWED_WORKSPACE_PREFIX),
    configPath,
    configDigest: createHash('sha256').update(text).digest('hex'),
    node: { source: nodeSource, target: nodeTarget, mode: linkNode ? 'symlink' : 'copy', binary: nodeBinary }
  }
}

function parseArgs(argv) {
  const options = { force: false, linkNode: true }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === '--root') options.root = argv[index + 1]
    else if (arg === '--node-source') options.nodeSource = argv[index + 1]
    else if (arg === '--copy-node') options.linkNode = false
    else if (arg === '--force') options.force = true
  }
  return options
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const options = parseArgs(process.argv.slice(2))
  if (!options.root) {
    process.stderr.write('用法：node scripts/rsi/clean-root.mjs --root <artifacts/rsi 下路径> [--node-source <AmitiaData/node>] [--copy-node] [--force]\n')
    process.exit(2)
  }
  try {
    const report = provisionCleanRoot(options.root, options)
    process.stdout.write(`${JSON.stringify(report, null, 2)}\n`)
  } catch (error) {
    process.stderr.write(`干净数据根创建失败：${error.message}\n${error.stack ?? ''}\n`)
    process.exit(6)
  }
}
