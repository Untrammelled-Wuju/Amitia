import { spawnSync } from 'node:child_process'
import { existsSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PRODUCTION_OBJECTS = ['AmitiaData', 'Plugin', '.amitia-secrets', 'desktop/release', 'backend/build']

export const FORBIDDEN_PORTS = [3000, 5178, 18880, 5037]

export const ALLOWED_WORKSPACE_PREFIX = 'artifacts/rsi'

const PROCESS_PATTERNS = [
  new RegExp(`${escapeRegExp(resolve(repoRoot, 'desktop', 'resources', 'core'))}[/\\\\][^\\s]+[/\\\\]AmitiaCore(\\s|$)`),
  new RegExp(`${escapeRegExp(resolve(repoRoot, 'desktop'))}[/\\\\]node_modules[/\\\\][^\\s]*Electron\\.app[/\\\\]Contents[/\\\\]MacOS[/\\\\]Electron(\\s|$)`),
  new RegExp(`${escapeRegExp(resolve(repoRoot, 'desktop', 'release'))}[/\\\\]`),
  /\bAmitiaCore\s+--runtime-profile=/
]

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function matchesAmitiaProcess(line) {
  return PROCESS_PATTERNS.some((pattern) => pattern.test(line))
}

function normalized(target) {
  const absolute = resolve(target)
  try {
    return realpathSync(absolute)
  } catch {
    return absolute
  }
}

function isInside(child, parent) {
  const base = normalized(parent)
  const target = normalized(child)
  return target === base || target.startsWith(`${base}${sep}`)
}

export function productionPaths() {
  return PRODUCTION_OBJECTS.map((relativePath) => resolve(repoRoot, relativePath))
}

export function listeningPorts() {
  const result = spawnSync('lsof', ['-nP', '-iTCP', '-sTCP:LISTEN'], { encoding: 'utf8', env: cleanEnv() })
  if (result.error || result.status !== 0) return new Set()
  const ports = new Set()
  for (const line of result.stdout.split('\n')) {
    const match = line.match(/:(\d+) \(LISTEN\)/)
    if (match) ports.add(Number.parseInt(match[1], 10))
  }
  return ports
}

export function detectInstanceProcesses(lines) {
  return lines.filter((line) => matchesAmitiaProcess(line) && !line.includes('scripts/rsi/')).map((line) => line.trim())
}

export function runningAmitiaProcesses() {
  const result = spawnSync('ps', ['-Ao', 'pid,command'], { encoding: 'utf8', env: cleanEnv() })
  if (result.error || result.status !== 0) return []
  return detectInstanceProcesses(result.stdout.split('\n'))
}

export function validateIsolation(config) {
  const violations = []
  const requiredPaths = ['dataRoot', 'pluginRoot', 'cacheRoot', 'dbPath']
  for (const key of requiredPaths) {
    if (typeof config[key] !== 'string' || config[key].trim() === '') {
      violations.push({ code: 'MISSING_PATH', message: `隔离配置缺少 ${key}` })
    }
  }
  if (!Number.isInteger(config.port)) {
    violations.push({ code: 'MISSING_PORT', message: '隔离配置缺少整数 port' })
  }

  const workspaceRoot = resolve(repoRoot, ALLOWED_WORKSPACE_PREFIX)
  for (const key of [...requiredPaths, 'worktreePath']) {
    const value = config[key]
    if (typeof value !== 'string' || value.trim() === '') continue
    const absolute = normalized(value)
    for (const production of productionPaths()) {
      if (isInside(absolute, production)) {
        violations.push({
          code: 'PRODUCTION_PATH',
          message: `${key} 指向生产资源 ${production}: ${absolute}`
        })
      }
    }
    if (isInside(absolute, repoRoot) && !isInside(absolute, workspaceRoot)) {
      violations.push({
        code: 'WORKSPACE_PATH',
        message: `${key} 位于仓库内但不在 ${workspaceRoot}: ${absolute}`
      })
    }
  }

  if (Number.isInteger(config.port)) {
    if (FORBIDDEN_PORTS.includes(config.port)) {
      violations.push({ code: 'FORBIDDEN_PORT', message: `端口 ${config.port} 属于生产/开发保留端口` })
    }
    if (listeningPorts().has(config.port)) {
      violations.push({ code: 'PORT_IN_USE', message: `端口 ${config.port} 已被真实进程监听` })
    }
  }

  if (typeof config.dbPath === 'string' && config.dbPath.trim() !== '') {
    const dbPath = normalized(config.dbPath)
    for (const name of ['amitia.db', 'app.db', 'kernel.db']) {
      for (const production of productionPaths()) {
        const candidate = resolve(production, name)
        if (dbPath === normalized(candidate) && existsSync(candidate)) {
          violations.push({ code: 'PRODUCTION_DB', message: `dbPath 指向生产数据库 ${candidate}` })
        }
      }
    }
  }

  const processes = runningAmitiaProcesses()
  if (config.allowRunningInstance !== true) {
    for (const line of processes) {
      violations.push({ code: 'INSTANCE_RUNNING', message: `检测到真实 Amitia 实例进程：${line}` })
    }
  }

  return { ok: violations.length === 0, repoRoot, workspaceRoot, violations }
}

function parseArgs(argv) {
  const options = { allowRunningInstance: false }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    const next = argv[index + 1]
    if (arg === '--data-root') options.dataRoot = next
    else if (arg === '--plugin-root') options.pluginRoot = next
    else if (arg === '--cache-root') options.cacheRoot = next
    else if (arg === '--db-path') options.dbPath = next
    else if (arg === '--port') options.port = Number.parseInt(next, 10)
    else if (arg === '--worktree') options.worktreePath = next
    else if (arg === '--allow-running-instance') options.allowRunningInstance = true
  }
  return options
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = validateIsolation(parseArgs(process.argv.slice(2)))
  process.stdout.write(`${JSON.stringify(report, null, 2)}\n`)
  if (!report.ok) {
    process.stderr.write(`隔离校验失败：\n${report.violations.map((item) => `- [${item.code}] ${item.message}`).join('\n')}\n`)
    process.exit(3)
  }
}
