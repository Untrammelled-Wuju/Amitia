import { spawn, spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, openSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'
import { ALLOWED_WORKSPACE_PREFIX, validateIsolation } from './isolation-guard.mjs'

export const DEFAULT_READY_TIMEOUT_MS = 180000
export const DEFAULT_SAMPLE_SECONDS = 20
export const SAMPLE_INTERVAL_MS = 2000

function sleep(ms) {
  return new Promise((done) => setTimeout(done, ms))
}

export function parseProcessSnapshot(output) {
  const rows = []
  for (const line of output.split('\n')) {
    const match = line.match(/^\s*(\d+)\s+(\d+)\s+([\d.]+)\s+([\d.]+)\s+(.*)$/)
    if (!match) continue
    rows.push({
      pid: Number.parseInt(match[1], 10),
      ppid: Number.parseInt(match[2], 10),
      rssKb: Number.parseFloat(match[3]),
      cpuPercent: Number.parseFloat(match[4]),
      command: match[5].trim()
    })
  }
  return rows
}

export function processSnapshot() {
  const result = spawnSync('ps', ['-Ao', 'pid,ppid,rss,%cpu,command'], { encoding: 'utf8', env: cleanEnv() })
  if (result.error) {
    throw new Error(`无法读取进程快照: ${result.error.message}`, { cause: result.error })
  }
  if (result.status !== 0) {
    throw new Error(`ps 退出码 ${result.status}\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`)
  }
  return parseProcessSnapshot(result.stdout)
}

export function collectTree(rows, rootPids) {
  const children = new Map()
  for (const row of rows) {
    if (!children.has(row.ppid)) children.set(row.ppid, [])
    children.get(row.ppid).push(row.pid)
  }
  const seen = new Set()
  const queue = [...rootPids]
  while (queue.length > 0) {
    const pid = queue.shift()
    if (seen.has(pid)) continue
    seen.add(pid)
    for (const child of children.get(pid) ?? []) queue.push(child)
  }
  return rows.filter((row) => seen.has(row.pid))
}

export function summarizeSamples(samples) {
  if (samples.length === 0) return { samples: 0 }
  const totals = samples.map((sample) => sample.rssKb)
  const cpu = samples.map((sample) => sample.cpuPercent)
  return {
    samples: samples.length,
    rssPeakMb: Math.round((Math.max(...totals) / 1024) * 10) / 10,
    rssAverageMb: Math.round((totals.reduce((sum, value) => sum + value, 0) / totals.length / 1024) * 10) / 10,
    cpuAveragePercent: Math.round((cpu.reduce((sum, value) => sum + value, 0) / cpu.length) * 100) / 100,
    cpuPeakPercent: Math.round(Math.max(...cpu) * 100) / 100,
    processPeakCount: Math.max(...samples.map((sample) => sample.processCount))
  }
}

export function endpointPath(dataRoot) {
  return resolve(dataRoot, 'run', 'endpoint.json')
}

export function readEndpoint(dataRoot) {
  const path = endpointPath(dataRoot)
  if (!existsSync(path)) return null
  const raw = readFileSync(path, 'utf8')
  try {
    return { ...JSON.parse(raw), endpointFile: path, raw }
  } catch (error) {
    throw new Error(`无法解析 ${path}: ${error.message}`, { cause: error })
  }
}

export function localToken(dataRoot) {
  const path = resolve(dataRoot, 'security', 'local-token')
  if (!existsSync(path)) return null
  return readFileSync(path, 'utf8').trim()
}

async function fetchJson(url, options = {}) {
  const response = await fetch(url, { ...options, signal: AbortSignal.timeout(5000) })
  const text = await response.text()
  let body = null
  try {
    body = JSON.parse(text)
  } catch {
    body = text
  }
  return { status: response.status, body }
}

function authHeaders(token) {
  const headers = { Accept: 'application/json', Origin: 'http://127.0.0.1:5178' }
  if (token) headers['X-Amitia-Local-Token'] = token
  return headers
}

export async function waitReady(baseUrl, timeoutMs) {
  const startedAt = Date.now()
  let last = null
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const probe = await fetchJson(`${baseUrl}/api/health`)
      last = probe
      const data = probe.body?.data
      if (probe.status === 200 && data?.health === true && data?.ready === true) {
        return { ready: true, elapsedMs: Date.now() - startedAt, payload: data }
      }
    } catch (error) {
      last = { status: 0, body: error.message }
    }
    await sleep(250)
  }
  return { ready: false, elapsedMs: Date.now() - startedAt, payload: last?.body ?? null }
}

export async function collectInventory(baseUrl, token) {
  const headers = authHeaders(token)
  const [pets, kernel, snapshot, events] = await Promise.all([
    fetchJson(`${baseUrl}/api/extensions/pet/plugins?pageSize=200`, { headers }),
    fetchJson(`${baseUrl}/api/extensions/kernel/status`, { headers }),
    fetchJson(`${baseUrl}/api/extensions/desktop/snapshot`, { headers }),
    fetchJson(`${baseUrl}/api/extensions/events/stats`, { headers })
  ])
  const petPlugins = Array.isArray(pets.body?.data?.plugins) ? pets.body.data.plugins : []
  const providers = Array.isArray(kernel.body?.providers) ? kernel.body.providers : []
  const instances = providers.flatMap((provider) => provider.instances ?? [])
  const stats = events.body ?? {}
  return {
    petPlugins: {
      total: pets.body?.data?.total ?? petPlugins.length,
      enabled: petPlugins.filter((item) => item.enabled === true).length,
      installed: petPlugins.filter((item) => item.installState === 'installed').length
    },
    kernelProviders: {
      providers: providers.length,
      runningInstances: instances.filter((instance) => instance.state === 'running' || instance.status === 'running').length,
      placementCore: providers.filter((provider) => provider.placement === 'core').length,
      placementRemote: providers.filter((provider) => provider.placement !== 'core').length
    },
    desktopSnapshot: {
      generation: snapshot.body?.generation ?? null,
      contributions: Array.isArray(snapshot.body?.contributions) ? snapshot.body.contributions.length : null,
      shortcuts: Array.isArray(snapshot.body?.shortcuts) ? snapshot.body.shortcuts.length : null,
      conflicts: Array.isArray(snapshot.body?.conflicts) ? snapshot.body.conflicts.length : null
    },
    events: {
      dispatchedOutbox: stats.dispatchedOutbox ?? null,
      pendingOutbox: stats.pendingOutbox ?? null,
      deadLetterOutbox: stats.deadLetterOutbox ?? null,
      activeSubscriptions: stats.activeSubscriptions ?? null
    },
    httpStatuses: { petPlugins: pets.status, kernel: kernel.status, snapshot: snapshot.status, events: events.status }
  }
}

export async function sampleFamily(baseUrl, token, rootPids, seconds) {
  const samples = []
  const rounds = Math.max(1, Math.round((seconds * 1000) / SAMPLE_INTERVAL_MS))
  let firstOutbox = null
  let lastOutbox = null
  for (let index = 0; index < rounds; index += 1) {
    const rows = processSnapshot()
    const tree = collectTree(rows, rootPids.filter((pid) => rows.some((row) => row.pid === pid)))
    samples.push({
      at: new Date().toISOString(),
      rssKb: tree.reduce((sum, row) => sum + row.rssKb, 0),
      cpuPercent: tree.reduce((sum, row) => sum + row.cpuPercent, 0),
      processCount: tree.length
    })
    try {
      const stats = await fetchJson(`${baseUrl}/api/extensions/events/stats`, { headers: authHeaders(token) })
      const dispatched = stats.body?.dispatchedOutbox
      if (Number.isFinite(dispatched)) {
        if (firstOutbox === null) firstOutbox = dispatched
        lastOutbox = dispatched
      }
    } catch (error) {
      lastOutbox = lastOutbox
    }
    if (index < rounds - 1) await sleep(SAMPLE_INTERVAL_MS)
  }
  const elapsedSeconds = ((rounds - 1) * SAMPLE_INTERVAL_MS) / 1000
  const dispatchedDelta = firstOutbox !== null && lastOutbox !== null ? lastOutbox - firstOutbox : null
  return {
    ...summarizeSamples(samples),
    windowSeconds: elapsedSeconds,
    dispatchedDelta,
    eventsPerSecond: dispatchedDelta !== null && elapsedSeconds > 0 ? Math.round((dispatchedDelta / elapsedSeconds) * 100) / 100 : null
  }
}

function spawnInstance(dataRoot, logPath) {
  const desktopDir = resolve(repoRoot, 'desktop')
  const output = openSync(logPath, 'a')
  const env = cleanEnv()
  env.AMITIA_DATA_DIR = dataRoot
  const child = spawn(resolve(desktopDir, 'node_modules', '.bin', 'electron'), ['.'], {
    cwd: desktopDir,
    env,
    detached: true,
    stdio: ['ignore', output, output]
  })
  child.unref()
  return child.pid
}

function killTree(rootPids, signal) {
  const rows = processSnapshot()
  const tree = collectTree(rows, rootPids)
  const pids = tree.map((row) => row.pid).reverse()
  for (const pid of pids) {
    try {
      process.kill(pid, signal)
    } catch (error) {
      if (error.code !== 'ENOENT') throw error
    }
  }
  return pids
}

async function waitExit(pids, timeoutMs) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const rows = processSnapshot()
    if (rows.filter((row) => pids.includes(row.pid)).length === 0) return true
    await sleep(500)
  }
  return false
}

async function waitEndpoint(dataRoot, launchEpoch, timeoutMs) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const candidate = readEndpoint(dataRoot)
    if (candidate && Date.parse(candidate.startedAt) >= launchEpoch - 5000) return candidate
    await sleep(250)
  }
  return null
}

function parseArgs(argv) {
  const options = { attach: false, shutdown: true, stop: false, sampleSeconds: DEFAULT_SAMPLE_SECONDS, readyTimeoutMs: DEFAULT_READY_TIMEOUT_MS }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    const next = argv[index + 1]
    if (arg === '--data-root') options.dataRoot = next
    else if (arg === '--label') options.label = next
    else if (arg === '--out') options.out = next
    else if (arg === '--sample-seconds') options.sampleSeconds = Number.parseInt(next, 10)
    else if (arg === '--ready-timeout') options.readyTimeoutMs = Number.parseInt(next, 10)
    else if (arg === '--attach') options.attach = true
    else if (arg === '--no-shutdown') options.shutdown = false
    else if (arg === '--stop') options.stop = true
  }
  return options
}

async function stopInstance(dataRoot) {
  const endpoint = readEndpoint(dataRoot)
  if (!endpoint) {
    process.stdout.write(`实例未运行（无 ${endpointPath(dataRoot)}）\n`)
    return
  }
  const rows = processSnapshot()
  const rootPids = [endpoint.pid]
  const desktopDir = resolve(repoRoot, 'desktop')
  for (const row of rows) {
    if (!row.command.includes('Electron.app/Contents/MacOS/Electron') || row.command.includes('Helper')) continue
    if (row.command.includes(desktopDir)) rootPids.push(row.pid)
  }
  const rootCount = rootPids.length
  const killed = killTree(rootPids, 'SIGTERM')
  let exited = await waitExit(killed, 20000)
  if (!exited) {
    killTree(rootPids, 'SIGKILL')
    exited = await waitExit(killed, 10000)
  }
  process.stdout.write(`实例停止：pid=${endpoint.pid} 根进程=${rootCount} 进程数=${killed.length} 优雅=${exited}\n`)
  if (!exited) process.exit(11)
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  if (options.stop) {
    if (!options.dataRoot) {
      process.stderr.write('用法：node scripts/rsi/perf-probe.mjs --stop --data-root <路径>\n')
      process.exit(2)
    }
    await stopInstance(resolve(options.dataRoot))
    return
  }
  if (!options.dataRoot || !options.label || !options.out) {
    process.stderr.write('用法：node scripts/rsi/perf-probe.mjs --data-root <路径> --label <名称> --out <文件> [--attach] [--sample-seconds 20] [--no-shutdown] [--stop]\n')
    process.exit(2)
  }
  const dataRoot = resolve(options.dataRoot)
  const outPath = resolve(options.out)
  mkdirSync(dirname(outPath), { recursive: true })

  const probe = {
    schemaVersion: 1,
    label: options.label,
    mode: options.attach ? 'attach' : 'launch',
    dataRoot,
    startedAt: new Date().toISOString(),
    host: {
      platform: process.platform,
      arch: process.arch,
      release: spawnSync('uname', ['-r'], { encoding: 'utf8' }).stdout.trim(),
      cpuCount: spawnSync('sysctl', ['-n', 'hw.ncpu'], { encoding: 'utf8' }).stdout.trim()
    }
  }

  let launchedPid = null
  let launchEpoch = 0
  if (!options.attach) {
    const isolation = validateIsolation({
      dataRoot,
      pluginRoot: resolve(dataRoot, 'Plugin'),
      cacheRoot: resolve(dataRoot, 'cache'),
      dbPath: resolve(dataRoot, 'app.db'),
      port: 0,
      worktreePath: resolve(repoRoot, ALLOWED_WORKSPACE_PREFIX)
    })
    if (!isolation.ok) {
      process.stderr.write(`隔离校验失败：\n${isolation.violations.map((item) => `- [${item.code}] ${item.message}`).join('\n')}\n`)
      process.exit(3)
    }
    mkdirSync(dataRoot, { recursive: true })
    const logPath = `${outPath.replace(/\.json$/, '')}.instance.log`
    launchEpoch = Date.now()
    launchedPid = spawnInstance(dataRoot, logPath)
    probe.launch = { pid: launchedPid, launchEpoch, logPath }
  }

  const endpoint = await waitEndpoint(dataRoot, launchEpoch || 0, options.readyTimeoutMs)
  if (!endpoint) {
    throw new Error(`等待 ${endpointPath(dataRoot)} 超时（${options.readyTimeoutMs}ms）`)
  }
  if (probe.launch) probe.launch.corePid = endpoint.pid
  probe.endpoint = {
    pid: endpoint.pid,
    port: endpoint.port,
    version: endpoint.version,
    instanceId: endpoint.instanceId,
    coreStartedAt: endpoint.startedAt
  }
  if (probe.launch) probe.startupMs = Date.parse(endpoint.startedAt) - probe.launch.launchEpoch

  const health = await waitReady(endpoint.httpBaseURL, options.readyTimeoutMs)
  probe.health = {
    ready: health.ready,
    elapsedMs: health.elapsedMs,
    state: health.payload?.checks?.runtime?.state ?? null,
    readyCount: health.payload?.checks?.runtime?.readyCount ?? null,
    database: health.payload?.database ?? null,
    deployMode: health.payload?.deployMode ?? null
  }
  if (!health.ready) {
    throw new Error(`实例健康检查未就绪：${JSON.stringify(health.payload)}`)
  }

  const token = localToken(dataRoot)
  probe.inventory = await collectInventory(endpoint.httpBaseURL, token)

  const rows = processSnapshot()
  const rootPids = []
  if (Number.isInteger(endpoint.pid)) rootPids.push(endpoint.pid)
  const desktopDir = resolve(repoRoot, 'desktop')
  for (const row of rows) {
    if (!row.command.includes('Electron.app/Contents/MacOS/Electron') || row.command.includes('Helper')) continue
    if (row.command.includes(desktopDir)) rootPids.push(row.pid)
  }
  if (launchedPid !== null) rootPids.push(launchedPid)
  probe.processRoots = [...new Set(rootPids)]
  probe.processTree = collectTree(rows, probe.processRoots).map((row) => ({
    pid: row.pid,
    rssMb: Math.round((row.rssKb / 1024) * 10) / 10,
    cpuPercent: row.cpuPercent,
    command: row.command.slice(0, 140)
  }))

  probe.sampling = await sampleFamily(endpoint.httpBaseURL, token, probe.processRoots, options.sampleSeconds)
  probe.finishedAt = new Date().toISOString()

  const digest = createHash('sha256').update(JSON.stringify(probe)).digest('hex')
  writeFileSync(outPath, `${JSON.stringify({ ...probe, digest }, null, 2)}\n`)

  process.stdout.write(`性能探针完成：${options.label}\n`)
  process.stdout.write(`  启动耗时   ${probe.startupMs ?? 'n/a'} ms\n`)
  process.stdout.write(`  健康就绪   ${probe.health.ready} (${probe.health.state}, ready=${probe.health.readyCount})\n`)
  process.stdout.write(`  RSS 峰值   ${probe.sampling.rssPeakMb ?? 'n/a'} MB / 均值 ${probe.sampling.rssAverageMb ?? 'n/a'} MB\n`)
  process.stdout.write(`  CPU 均值   ${probe.sampling.cpuAveragePercent ?? 'n/a'} %\n`)
  process.stdout.write(`  事件吞吐   ${probe.sampling.eventsPerSecond ?? 'n/a'} 次/秒（窗口 ${probe.sampling.windowSeconds}s）\n`)
  process.stdout.write(`  插件数量   pet=${probe.inventory.petPlugins.total} providers=${probe.inventory.kernelProviders.providers} contributions=${probe.inventory.desktopSnapshot.contributions}\n`)
  process.stdout.write(`  证据文件   ${outPath}\n`)

  if (probe.launch && options.shutdown) {
    const killed = killTree(probe.processRoots, 'SIGTERM')
    const exited = await waitExit(killed, 20000)
    if (!exited) killTree(probe.processRoots, 'SIGKILL')
    process.stdout.write(`  实例已关闭 ${exited ? '（优雅退出）' : '（强制退出）'}\n`)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    process.stderr.write(`性能探针失败：${error.message}\n${error.stack ?? ''}\n`)
    process.exit(7)
  })
}
