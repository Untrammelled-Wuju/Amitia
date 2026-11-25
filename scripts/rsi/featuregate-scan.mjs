import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { dirname, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const outPath = resolve(repoRoot, 'artifacts', 'rsi', 'm01-s03-featuregate-scan.json')
const importerAllowlist = [
  'backend/cmd/server/main.go',
  'backend/cmd/server/feature_canary.go',
  'backend/cmd/server/feature_gate_routes.go',
  'backend/internal/extension/kernel/featuregate/gate_test.go'
]
const configTemplates = [
  'config/config.yml',
  'desktop/resources/config-template/config.yaml',
  'scripts/rsi/templates/clean-root.config.yml',
  'backend/config/config.yml'
]

export function collectGoFiles(root) {
  const files = []
  const walk = (dir) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const child = resolve(dir, entry.name)
      if (entry.isDirectory()) walk(child)
      else if (entry.isFile() && entry.name.endsWith('.go')) files.push(child)
    }
  }
  const backend = resolve(root, 'backend')
  if (existsSync(backend)) walk(backend)
  return files.sort()
}

function read(root, relativePath) {
  return readFileSync(resolve(root, relativePath), 'utf8')
}

function templateFlagViolations(root) {
  const violations = []
  const facts = []
  for (const relativePath of configTemplates) {
    const absolute = resolve(root, relativePath)
    if (!existsSync(absolute)) continue
    const lines = readFileSync(absolute, 'utf8').split('\n')
    const start = lines.findIndex((line) => /^features:\s*$/.test(line))
    if (start < 0) {
      facts.push({ template: relativePath, features: null })
      continue
    }
    const flags = {}
    for (const line of lines.slice(start + 1)) {
      const match = /^\s{2}([a-z0-9-]+):\s*(true|false)\s*$/.exec(line)
      if (!match) break
      flags[match[1]] = match[2] === 'true'
    }
    facts.push({ template: relativePath, features: flags })
    for (const [name, enabled] of Object.entries(flags)) {
      if (enabled) violations.push({ code: 'TEMPLATE_FEATURE_ENABLED', message: `${relativePath} 中 features.${name}=true，默认配置不得开启` })
    }
  }
  return { violations, facts }
}

export function scanFeatureGate(root = repoRoot) {
  const gatePackageDir = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'featuregate')
  const violations = []
  const facts = { importers: [], callSites: [], configReaders: [], templates: [], goFiles: 0 }

  const gateFile = resolve(gatePackageDir, 'gate.go')
  if (!existsSync(gateFile)) {
    violations.push({ code: 'GATE_MISSING', message: '缺少 featuregate/gate.go' })
  } else {
    const gate = readFileSync(gateFile, 'utf8')
    for (const symbol of ['func Parse(', 'func Configure(', 'func Enabled(', 'func Apply(', 'func Snapshot(']) {
      if (!gate.includes(symbol)) violations.push({ code: 'GATE_SYMBOL_MISSING', message: `gate.go 缺少 ${symbol}` })
    }
    if (!gate.includes('states[name] = false')) {
      violations.push({ code: 'GATE_DEFAULT_NOT_FALSE', message: 'gate.go 未将所有开关默认值初始化为 false' })
    }
    if (/states\[name\]\s*=\s*true/.test(gate)) {
      violations.push({ code: 'GATE_DEFAULT_TRUE', message: 'gate.go 存在默认开启分支' })
    }
  }

  const goFiles = collectGoFiles(root)
  facts.goFiles = goFiles.length
  const importPattern = /"github\.com\/u-ai\/backend\/internal\/extension\/kernel\/featuregate"/
  for (const file of goFiles) {
    const relativePath = relative(root, file)
    const text = readFileSync(file, 'utf8')
    if (importPattern.test(text)) {
      facts.importers.push(relativePath)
      if (!importerAllowlist.includes(relativePath)) {
        violations.push({ code: 'IMPORTER_NOT_ALLOWED', message: `${relativePath} 绕过单点 featuregate 读入口` })
      }
    }
    if (text.includes('AMITIA_FEATURE') || /FEATURE_FLAG/.test(text)) {
      violations.push({ code: 'ENV_BYPASS', message: `${relativePath} 出现环境变量式功能开关` })
    }
    if (relativePath !== 'backend/cmd/server/main.go' && /AppCfg\.Features/.test(text)) {
      violations.push({ code: 'CONFIG_READER_NOT_SINGLETON', message: `${relativePath} 直接读取 AppCfg.Features` })
    }
    if (relativePath === 'backend/cmd/server/main.go' && /AppCfg\.Features/.test(text)) {
      facts.configReaders.push(relativePath)
    }
    const callPattern = /featuregate\.Enabled\(\s*([^,\n)]+)|featuregate\.Apply\([^,]+,\s*([^,\n)]+)/g
    for (const match of text.matchAll(callPattern)) {
      const argument = (match[2] ?? match[1]).trim()
      const call = match[2] ? 'Apply' : 'Enabled'
      facts.callSites.push({ file: relativePath, call, argument })
      if (!argument.startsWith('featuregate.Name')) {
        violations.push({ code: 'CALL_SITE_RAW_NAME', message: `${relativePath} 的 featuregate.${call} 使用非目录常量 ${argument}` })
      }
    }
    if (relativePath !== 'backend/cmd/server/feature_gate_routes.go' && /"\/features(\/routes)?"/.test(text)) {
      violations.push({ code: 'ROUTE_NOT_GATED', message: `${relativePath} 出现未受 gate 保护的 /features 路由字面量` })
    }
  }

  if (facts.configReaders.length !== 1) {
    violations.push({ code: 'CONFIG_READER_COUNT', message: `AppCfg.Features 读取点数量为 ${facts.configReaders.length}，期望 1` })
  }
  if (!facts.callSites.some((site) => site.call === 'Enabled')) {
    violations.push({ code: 'NO_ENABLED_CALL_SITE', message: '缺少 featuregate.Enabled 调用点' })
  }
  if (!facts.callSites.some((site) => site.call === 'Apply')) {
    violations.push({ code: 'NO_APPLY_CALL_SITE', message: '缺少 featuregate.Apply 调用点' })
  }

  const mainPath = resolve(root, 'backend', 'cmd', 'server', 'main.go')
  if (!existsSync(mainPath)) {
    violations.push({ code: 'MAIN_MISSING', message: '缺少 backend/cmd/server/main.go' })
  } else {
    const main = readFileSync(mainPath, 'utf8')
    const configureIndex = main.indexOf('featuregate.Configure(')
    const singleInstanceIndex = main.indexOf('ensureSingleInstance(serverAddr')
    if (configureIndex < 0) {
      violations.push({ code: 'CONFIGURE_MISSING', message: 'main.go 未在启动阶段调用 featuregate.Configure' })
    } else if (singleInstanceIndex >= 0 && configureIndex > singleInstanceIndex) {
      violations.push({ code: 'CONFIGURE_ORDER', message: 'featuregate.Configure 必须早于 ensureSingleInstance' })
    }
  }

  const templates = templateFlagViolations(root)
  violations.push(...templates.violations)
  facts.templates = templates.facts

  return { ok: violations.length === 0, violations, facts }
}

function main() {
  const result = scanFeatureGate(repoRoot)
  mkdirSync(dirname(outPath), { recursive: true })
  writeFileSync(outPath, `${JSON.stringify({ schemaVersion: 1, generatedAt: new Date().toISOString(), ...result }, null, 2)}\n`, 'utf8')
  for (const item of result.violations) process.stderr.write(`- [${item.code}] ${item.message}\n`)
  process.stdout.write(`featuregate-scan importers=${result.facts.importers.length} callSites=${result.facts.callSites.length} templates=${result.facts.templates.length} ok=${result.ok}\n`)
  process.stdout.write(`evidence=${outPath}\n`)
  if (!result.ok) process.exit(15)
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
