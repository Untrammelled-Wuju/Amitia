import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))

export const repoRoot = resolve(here, '..', '..')

const GIT_ENV_PATTERN = /^GIT_CONFIG_(COUNT|KEY_\d+|VALUE_\d+)$/

export function cleanEnv(source = process.env) {
  const env = { ...source }
  for (const key of Object.keys(env)) {
    if (GIT_ENV_PATTERN.test(key)) delete env[key]
  }
  return env
}

export function run(command, args = [], options = {}) {
  const result = spawnSync(command, args, {
    encoding: 'utf8',
    env: cleanEnv(),
    ...options
  })
  if (result.error) {
    throw new Error(`无法执行命令 ${command} ${args.join(' ')}: ${result.error.message}`, { cause: result.error })
  }
  if (result.status !== 0) {
    throw new Error(
      `命令 ${command} ${args.join(' ')} 退出码 ${result.status}\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`
    )
  }
  return `${result.stdout}${result.stderr}`.trim()
}

export function readJson(relativePath) {
  return JSON.parse(readFileSync(resolve(repoRoot, relativePath), 'utf8'))
}

export function readText(relativePath) {
  return readFileSync(resolve(repoRoot, relativePath), 'utf8')
}

function parseTuple(version) {
  return version.split('.').map((part) => Number.parseInt(part, 10))
}

function compareVersions(left, right) {
  const a = parseTuple(left)
  const b = parseTuple(right)
  for (let index = 0; index < Math.max(a.length, b.length); index += 1) {
    const delta = (a[index] ?? 0) - (b[index] ?? 0)
    if (delta !== 0) return delta < 0 ? -1 : 1
  }
  return 0
}

function matchesNodeRange(version, range) {
  return range
    .split(/\s+/)
    .filter(Boolean)
    .every((clause) => {
      if (clause.startsWith('>=')) return compareVersions(version, clause.slice(2)) >= 0
      if (clause.startsWith('<=')) return compareVersions(version, clause.slice(2)) <= 0
      if (clause.startsWith('>')) return compareVersions(version, clause.slice(1)) > 0
      if (clause.startsWith('<')) return compareVersions(version, clause.slice(1)) < 0
      return version === clause
    })
}

function matchesCaretRange(version, range) {
  const floor = range.replace(/^[^\d]*/, '')
  return compareVersions(version, floor) >= 0 && parseTuple(version)[0] === parseTuple(floor)[0]
}

function check(checks, name, required, actual, predicate, detail) {
  const ok = Boolean(predicate(actual))
  checks.push({ name, required, actual: actual ?? null, ok, detail: ok ? null : detail })
  return ok
}

export function preflight(options = {}) {
  const checks = []

  const requiredGo = (readText('backend/go.mod').match(/^go\s+(\d+\.\d+\.\d+)/m) || [])[1]
  const goActual = (run('go', ['version']).match(/go(\d+\.\d+\.\d+)/) || [])[1]
  check(checks, 'go', requiredGo, goActual, (value) => value === requiredGo, `Go 版本必须为 ${requiredGo}`)

  const front = readJson('front/package.json')
  const requiredNode = front.engines?.node ?? ''
  const nodeActual = process.version.replace(/^v/, '')
  check(
    checks,
    'node',
    requiredNode,
    nodeActual,
    (value) => matchesNodeRange(value, requiredNode),
    `Node 版本必须满足 ${requiredNode}`
  )

  const requiredPnpm = (front.packageManager || '').match(/^pnpm@(\d+)/)
  const pnpmActual = run('pnpm', ['-v']).split('\n').pop()
  check(
    checks,
    'pnpm',
    requiredPnpm ? `${requiredPnpm[1]}.x` : 'any',
    pnpmActual,
    (value) => (requiredPnpm ? parseTuple(value)[0] === Number.parseInt(requiredPnpm[1], 10) : true),
    `pnpm 主版本必须为 ${requiredPnpm ? requiredPnpm[1] : '任意'}`
  )

  const requiredDart = (readText('mobile_app/pubspec.yaml').match(/sdk:\s*\^?(\d+\.\d+\.\d+)/) || [])[1]
  const dartActual = (run('dart', ['--version']).match(/Dart SDK version:\s*(\d+\.\d+\.\d+)/) || [])[1]
  check(
    checks,
    'dart',
    `^${requiredDart}`,
    dartActual,
    (value) => matchesCaretRange(value, requiredDart),
    `Dart 版本必须满足 ^${requiredDart}`
  )

  const flutterPath = spawnSync('which', ['flutter'], { encoding: 'utf8', env: cleanEnv() }).stdout.trim()
  check(
    checks,
    'flutter',
    'executable',
    flutterPath,
    (value) => typeof value === 'string' && value.length > 0 && existsSync(value),
    '未找到 flutter 可执行文件'
  )

  const projectNode = resolve(repoRoot, 'AmitiaData', 'node', 'bin', 'node')
  check(
    checks,
    'node_binary',
    'AmitiaData/node/bin/node',
    process.execPath,
    (value) => options.allowAnyNode === true || resolve(value) === projectNode,
    `必须使用项目内 Node：${projectNode}，当前为 ${process.execPath}`
  )

  const failures = checks
    .filter((item) => !item.ok)
    .map((item) => `${item.name}: 期望 ${item.required}，实际 ${item.actual}，${item.detail}`)

  return { ok: failures.length === 0, checkedAt: new Date().toISOString(), checks, failures }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = preflight({ allowAnyNode: process.env.RSI_ALLOW_ANY_NODE === '1' })
  process.stdout.write(`${JSON.stringify(report, null, 2)}\n`)
  if (!report.ok) {
    process.stderr.write(`工具链预检失败（BLOCKED）：\n${report.failures.join('\n')}\n`)
    process.exit(2)
  }
}
