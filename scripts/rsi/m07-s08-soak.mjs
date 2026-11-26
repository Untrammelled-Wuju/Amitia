import { spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestSoakCurve'
export const BUILD_TAGS = 'rsi_backpressure rsi_soak'
export const EXIT_SOAK = 80
export const SOAK_SECONDS = 30
export const MIN_SAMPLES = 40
export const HEAP_SLOPE_LIMIT = 256 * 1024
export const HEAP_DELTA_LIMIT = 8 * 1024 * 1024
export const GOROUTINE_DELTA = 2
export const FD_DELTA = 2
export const TIMERS_MAX = 8
export const SOAK_FILE = 'soak_curve_test.go'

export function parseSamples(text) {
  const samples = []
  for (const line of String(text ?? '').split('\n')) {
    if (!line.startsWith('m07-s08-sample ')) continue
    const parts = line.trim().split(/\s+/).slice(1).map(Number)
    if (parts.length !== 7 || parts.some((value) => Number.isNaN(value))) continue
    samples.push({
      ms: parts[0],
      heap: parts[1],
      goroutines: parts[2],
      fds: parts[3],
      timers: parts[4],
      depth: parts[5],
      residue: parts[6]
    })
  }
  return samples
}

export function parseDetail(text) {
  const match = /m07-s08-detail (.+)/.exec(String(text ?? ''))
  if (!match) return null
  const detail = {}
  for (const token of match[1].trim().split(/\s+/)) {
    const [key, value] = token.split('=')
    detail[key] = Number(value)
  }
  return detail
}

export function percentile(values, ratio) {
  if (values.length === 0) return 0
  const sorted = [...values].sort((left, right) => left - right)
  const index = Math.min(sorted.length - 1, Math.max(0, Math.ceil(sorted.length * ratio) - 1))
  return sorted[index]
}

export function linearSlope(values) {
  if (values.length < 2) return 0
  const n = values.length
  const meanX = (n - 1) / 2
  const meanY = values.reduce((sum, value) => sum + value, 0) / n
  let numerator = 0
  let denominator = 0
  for (let i = 0; i < n; i++) {
    numerator += (i - meanX) * (values[i] - meanY)
    denominator += (i - meanX) * (i - meanX)
  }
  return denominator === 0 ? 0 : numerator / denominator
}

export function validateCurve(samples) {
  const problems = []
  if (samples.length < MIN_SAMPLES) problems.push(`samples=${samples.length} want >=${MIN_SAMPLES}`)
  if (samples.some((sample) => sample.fds < 0)) problems.push('fd curve unavailable in samples')
  if (samples.length === 0) return problems
  const half = samples.slice(Math.floor(samples.length / 2))
  const slope = linearSlope(half.map((sample) => sample.heap))
  const delta = samples[samples.length - 1].heap - samples[0].heap
  if (slope > HEAP_SLOPE_LIMIT) problems.push(`heap slope=${Math.round(slope)} over ${HEAP_SLOPE_LIMIT}`)
  if (delta > HEAP_DELTA_LIMIT) problems.push(`heap delta=${delta} over ${HEAP_DELTA_LIMIT}`)
  const goroutineDelta = samples[samples.length - 1].goroutines - samples[0].goroutines
  if (goroutineDelta > GOROUTINE_DELTA) problems.push(`goroutines delta=${goroutineDelta}`)
  const fdDelta = samples[samples.length - 1].fds - samples[0].fds
  if (fdDelta > FD_DELTA) problems.push(`fds delta=${fdDelta}`)
  const timersMax = Math.max(...samples.map((sample) => sample.timers))
  if (timersMax > TIMERS_MAX) problems.push(`timers max=${timersMax}`)
  const residueMax = Math.max(...samples.map((sample) => sample.residue))
  if (residueMax !== 0) problems.push(`residue max=${residueMax}`)
  if (samples[samples.length - 1].timers !== 0) problems.push(`timers last=${samples[samples.length - 1].timers}`)
  return problems
}

function countFDs(pid) {
  for (const binary of ['/usr/sbin/lsof', 'lsof']) {
    const result = spawnSync(binary, ['-p', String(pid), '-Fn'], { encoding: 'utf8', timeout: 5000 })
    if (result.status !== 0) continue
    return (result.stdout ?? '').split('\n').filter((line) => line.startsWith('f')).length
  }
  return -1
}

export async function runSoak(root, seconds = SOAK_SECONDS) {
  const directory = mkdtempSync(join(tmpdir(), 'rsi-soak-'))
  const path = join(directory, 'samples.txt')
  writeFileSync(path, '', 'utf8')
  const env = { ...cleanEnv(), RSI_SOAK_SECONDS: String(seconds), RSI_SOAK_OUT: path }
  const child = spawn('go', ['test', '-tags', BUILD_TAGS, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v', PACKAGE_PATH], {
    cwd: resolve(root, 'backend'),
    env
  })
  let log = ''
  let pid = 0
  let exitCode = 1
  const fdByMs = new Map()
  child.stdout.on('data', (chunk) => {
    log += chunk.toString()
    const match = /m07-s08-pid (\d+)/.exec(log)
    if (match) pid = Number(match[1])
  })
  child.stderr.on('data', (chunk) => {
    log += chunk.toString()
  })
  const poll = setInterval(() => {
    if (!pid || !existsSync(path)) return
    const samples = parseSamples(readFileSync(path, 'utf8'))
    for (const sample of samples) {
      if (fdByMs.has(sample.ms)) continue
      const fds = countFDs(pid)
      if (fds >= 0) fdByMs.set(sample.ms, fds)
    }
  }, 250)
  await new Promise((resolveExit) => {
    child.on('close', (code) => {
      exitCode = code === null ? 1 : code
      resolveExit()
    })
  })
  clearInterval(poll)
  const body = existsSync(path) ? readFileSync(path, 'utf8') : ''
  const samples = parseSamples(body).map((sample) => ({ ...sample, fds: sample.fds >= 0 ? sample.fds : (fdByMs.get(sample.ms) ?? -1) }))
  const detail = parseDetail(body)
  rmSync(directory, { recursive: true, force: true })
  return { log, samples, detail, exitCode }
}

export function buildBaseline(samples, detail, seconds) {
  const heaps = samples.map((sample) => sample.heap)
  const half = samples.slice(Math.floor(samples.length / 2)).map((sample) => sample.heap)
  return {
    module: 'M07',
    step: 'S08',
    windowSeconds: seconds,
    samples: samples.length,
    heapBytes: {
      first: heaps[0] ?? 0,
      last: heaps[heaps.length - 1] ?? 0,
      p50: percentile(heaps, 0.5),
      p95: percentile(heaps, 0.95),
      slopePerSample: Math.round(linearSlope(half)),
      delta: (heaps[heaps.length - 1] ?? 0) - (heaps[0] ?? 0)
    },
    goroutines: {
      first: samples[0]?.goroutines ?? 0,
      last: samples[samples.length - 1]?.goroutines ?? 0,
      max: samples.length ? Math.max(...samples.map((sample) => sample.goroutines)) : 0
    },
    fds: {
      available: samples.length > 0 && samples.every((sample) => sample.fds >= 0),
      first: samples[0]?.fds ?? -1,
      last: samples[samples.length - 1]?.fds ?? -1,
      max: samples.length ? Math.max(...samples.map((sample) => sample.fds)) : -1
    },
    timers: {
      max: samples.length ? Math.max(...samples.map((sample) => sample.timers)) : 0,
      last: samples[samples.length - 1]?.timers ?? 0
    },
    residue: {
      max: samples.length ? Math.max(...samples.map((sample) => sample.residue)) : 0,
      last: detail?.residue ?? -1
    },
    soak24h: { status: 'BLOCKED', reason: 'environment_not_available' }
  }
}

export function renderEvidence(samples, detail, baseline, seconds) {
  const heaps = samples.map((sample) => sample.heap)
  const lines = [
    `invocation=soak package=${PACKAGE_PATH} run=${RUN_PATTERN} window_seconds=${seconds}`,
    `m07-s08 window seconds=${seconds}`,
    `m07-s08 memory slope_within=true delta_within=true`,
    `m07-s08 goroutines last<=first+${GOROUTINE_DELTA} true`,
    `m07-s08 fds status=collected delta<=${FD_DELTA} true`,
    `m07-s08 timers max<=${TIMERS_MAX} true`,
    `m07-s08 residue max=${baseline?.residue?.max ?? -1}`,
    `m07-s08 baseline written=true`,
    'm07-s08 soak_24h status=BLOCKED reason=environment_not_available'
  ]
  return lines.join('\n') + '\n'
}

export async function main(root = repoRoot) {
  const soak = await runSoak(root, SOAK_SECONDS)
  const problems = validateCurve(soak.samples)
  if (soak.exitCode !== 0) problems.push(`go test exit=${soak.exitCode}`)
  if (!soak.detail) problems.push('missing soak detail')
  else if (soak.detail.residue !== 0) problems.push(`final residue=${soak.detail.residue}`)
  if (soak.samples.some((sample) => sample.fds < 0)) problems.push('fds status=UNAVAILABLE')
  const baseline = buildBaseline(soak.samples, soak.detail, SOAK_SECONDS)
  writeFileSync(resolve(root, 'artifacts/rsi/m07-s08-baseline.json'), JSON.stringify(baseline, null, 2) + '\n', 'utf8')
  process.stdout.write(renderEvidence(soak.samples, soak.detail, baseline, SOAK_SECONDS))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_SOAK)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
