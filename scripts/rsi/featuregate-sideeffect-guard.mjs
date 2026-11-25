import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { copyFileSync, mkdirSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { join, relative, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'
import {
  buildBaselineSource,
  buildCoreBinary,
  prepareInstanceRoot,
  runCoreInstance
} from './lib/core-instance.mjs'

const workDir = resolve(repoRoot, 'artifacts', 'rsi', 'm01-s06')
const baselineSource = resolve(workDir, 'baseline-src')
const baselineBinary = resolve(workDir, 'baseline', 'AmitiaCore')
const currentBinary = resolve(workDir, 'current', 'AmitiaCore')
const runsDir = resolve(workDir, 'runs')
const outcomePath = resolve(repoRoot, 'artifacts', 'rsi', 'm01-s06-offstate-db-guard.json')
const flags = { 'runtime-canary': false, 'route-inventory': false }

function check(checks, id, ok, detail) {
  checks.push({ id, ok: Boolean(ok), detail: ok ? null : detail })
}

function sqliteJson(dbPath, sql) {
  const result = spawnSync('sqlite3', ['-json', dbPath, sql], { encoding: 'utf8', env: cleanEnv() })
  if (result.error) {
    throw new Error(`无法执行 sqlite3: ${result.error.message}`, { cause: result.error })
  }
  if (result.status !== 0) {
    throw new Error(`sqlite3 读取 ${dbPath} 失败（exit=${result.status}）\n${result.stdout}\n${result.stderr}`)
  }
  const text = (result.stdout ?? '').trim()
  if (!text) return []
  return JSON.parse(text)
}

function listDatabases(root) {
  const found = []
  const walk = (dir) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name)
      if (entry.isDirectory()) {
        walk(full)
        continue
      }
      if (entry.isFile() && entry.name.endsWith('.db')) {
        found.push(relative(root, full).split(sep).join('/'))
      }
    }
  }
  const skipped = new Set(['config', 'run', 'logs'])
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    const full = join(root, entry.name)
    if (entry.isFile() && entry.name.endsWith('.db')) {
      found.push(entry.name)
      continue
    }
    if (!entry.isDirectory() || skipped.has(entry.name)) continue
    walk(full)
  }
  return found.sort()
}

function snapshotDatabase(dbPath) {
  const entries = sqliteJson(
    dbPath,
    "SELECT type, name, tbl_name, COALESCE(sql, '') AS sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name;"
  ).map((row) => ({ type: row.type, name: row.name, table: row.tbl_name, sql: row.sql }))
  const tables = entries.filter((entry) => entry.type === 'table').map((entry) => entry.name)
  const rowCounts = {}
  for (const table of tables) {
    const quoted = `"${table.replace(/"/g, '""')}"`
    const rows = sqliteJson(dbPath, `SELECT COUNT(*) AS total FROM ${quoted};`)
    rowCounts[table] = Number(rows[0]?.total ?? 0)
  }
  const digest = createHash('sha256')
    .update(JSON.stringify(entries.map((entry) => [entry.type, entry.name, entry.table, entry.sql])))
    .digest('hex')
  return { digest, entries, rowCounts }
}

function snapshotRoot(root) {
  const databases = {}
  for (const relativePath of listDatabases(root)) {
    databases[relativePath] = snapshotDatabase(resolve(root, relativePath))
  }
  return databases
}

export function schemaDelta(baselineEntries, currentEntries) {
  const key = (entry) => `${entry.type}:${entry.name}`
  const baselineMap = new Map(baselineEntries.map((entry) => [key(entry), entry]))
  const currentMap = new Map(currentEntries.map((entry) => [key(entry), entry]))
  const added = [...currentMap.keys()].filter((name) => !baselineMap.has(name))
  const removed = [...baselineMap.keys()].filter((name) => !currentMap.has(name))
  const changed = [...baselineMap.keys()]
    .filter((name) => currentMap.has(name) && baselineMap.get(name).sql !== currentMap.get(name).sql)
    .map((name) => ({
      name,
      baseline: baselineMap.get(name).sql,
      current: currentMap.get(name).sql
    }))
  return { added, removed, changed }
}

export function volatileTables(firstSnapshot, secondSnapshot) {
  const volatile = new Set()
  for (const [path, first] of Object.entries(firstSnapshot)) {
    const second = secondSnapshot[path]
    if (!second) {
      volatile.add(`${path}::<missing>`)
      continue
    }
    const names = new Set([...Object.keys(first.rowCounts), ...Object.keys(second.rowCounts)])
    for (const table of names) {
      if (first.rowCounts[table] !== second.rowCounts[table]) volatile.add(`${path}::${table}`)
    }
  }
  return [...volatile].sort()
}

export function compareSnapshots(baseline, current, volatile) {
  const drift = []
  const paths = new Set([...Object.keys(baseline), ...Object.keys(current)])
  for (const path of [...paths].sort()) {
    const left = baseline[path]
    const right = current[path]
    if (!left) {
      drift.push({ kind: 'DATABASE_ADDED', database: path })
      continue
    }
    if (!right) {
      drift.push({ kind: 'DATABASE_REMOVED', database: path })
      continue
    }
    if (left.digest !== right.digest) {
      drift.push({
        kind: 'SCHEMA_CHANGED',
        database: path,
        delta: schemaDelta(left.entries, right.entries)
      })
    }
    const tables = new Set([...Object.keys(left.rowCounts), ...Object.keys(right.rowCounts)])
    for (const table of [...tables].sort()) {
      if (volatile.has(`${path}::${table}`)) continue
      if (!(table in left.rowCounts) || !(table in right.rowCounts)) continue
      const before = left.rowCounts[table]
      const after = right.rowCounts[table]
      if (before !== after) {
        drift.push({ kind: 'ROWS_CHANGED', database: path, table, baseline: before ?? null, current: after ?? null })
      }
    }
  }
  return drift
}

function selfTestSchemaDetection(root) {
  const source = listDatabases(root).find((path) => path.endsWith('app.db')) ?? listDatabases(root)[0]
  if (!source) return { ok: false, detail: '没有可用的 SQLite 文件用于自检' }
  const probePath = resolve(workDir, 'selftest-probe.db')
  rmSync(probePath, { force: true })
  copyFileSync(resolve(root, source), probePath)
  const before = snapshotDatabase(probePath)
  const created = spawnSync('sqlite3', [probePath, 'CREATE TABLE guard_probe (id INTEGER PRIMARY KEY, note TEXT);'], {
    encoding: 'utf8',
    env: cleanEnv()
  })
  if (created.status !== 0) {
    return { ok: false, detail: `注入探针表失败: ${created.stderr}` }
  }
  const after = snapshotDatabase(probePath)
  const delta = schemaDelta(before.entries, after.entries)
  rmSync(probePath, { force: true })
  const detected = before.digest !== after.digest && delta.added.includes('table:guard_probe')
  return {
    ok: detected,
    detail: detected ? `digest ${before.digest.slice(0, 12)} → ${after.digest.slice(0, 12)}，新增 ${delta.added.join(',')}` : '注入 guard_probe 后指纹未变化'
  }
}

function selfTestRowDetection() {
  const fakeEntry = { type: 'table', name: 'guard_rows', table: 'guard_rows', sql: 'CREATE TABLE guard_rows (id INTEGER)' }
  const baseline = { 'app.db': { digest: 'x', entries: [fakeEntry], rowCounts: { guard_rows: 3 } } }
  const current = { 'app.db': { digest: 'x', entries: [fakeEntry], rowCounts: { guard_rows: 4 } } }
  const drift = compareSnapshots(baseline, current, new Set())
  const detected = drift.length === 1 && drift[0].kind === 'ROWS_CHANGED' && drift[0].baseline === 3 && drift[0].current === 4
  return { ok: detected, detail: detected ? JSON.stringify(drift[0]) : JSON.stringify(drift) }
}

async function runOffInstance(name, binary) {
  const root = resolve(runsDir, name)
  rmSync(root, { recursive: true, force: true })
  mkdirSync(root, { recursive: true })
  prepareInstanceRoot(root, flags)
  const instance = await runCoreInstance({ name, binary, root, flags })
  return { name, root, snapshot: snapshotRoot(root), routeDigest: instance.routeDigest, routeCount: instance.routeCount }
}

export async function runGuard() {
  mkdirSync(runsDir, { recursive: true })
  rmSync(baselineSource, { recursive: true, force: true })
  buildCoreBinary(baselineBinary, buildBaselineSource(baselineSource))
  buildCoreBinary(currentBinary, resolve(repoRoot, 'backend'))

  const baseline = await runOffInstance('baseline-off', baselineBinary)
  const currentFirst = await runOffInstance('current-off-1', currentBinary)
  const currentSecond = await runOffInstance('current-off-2', currentBinary)

  const volatile = volatileTables(currentFirst.snapshot, currentSecond.snapshot)
  const drift = compareSnapshots(baseline.snapshot, currentFirst.snapshot, new Set(volatile))
  const restartDrift = compareSnapshots(currentFirst.snapshot, currentSecond.snapshot, new Set(volatile))

  const schemaSelfTest = selfTestSchemaDetection(currentFirst.root)
  const rowSelfTest = selfTestRowDetection()

  const checks = []
  check(checks, 'SELF_TEST_SCHEMA_DETECTS', schemaSelfTest.ok, schemaSelfTest.detail)
  check(checks, 'SELF_TEST_ROWS_DETECTS', rowSelfTest.ok, rowSelfTest.detail)
  check(checks, 'DATABASES_PRESENT', Object.keys(currentFirst.snapshot).length > 0,
    JSON.stringify(Object.keys(currentFirst.snapshot)))
  check(checks, 'OFF_SCHEMA_UNCHANGED', drift.filter((item) => item.kind === 'SCHEMA_CHANGED' || item.kind === 'DATABASE_ADDED' || item.kind === 'DATABASE_REMOVED').length === 0,
    JSON.stringify(drift.filter((item) => item.kind.startsWith('SCHEMA') || item.kind.startsWith('DATABASE'))))
  check(checks, 'OFF_ROWS_UNCHANGED', drift.filter((item) => item.kind === 'ROWS_CHANGED').length === 0,
    JSON.stringify(drift.filter((item) => item.kind === 'ROWS_CHANGED')))
  check(checks, 'OFF_RESTART_STABLE', restartDrift.length === 0, JSON.stringify(restartDrift))
  check(checks, 'OFF_ROUTE_DIGEST_UNCHANGED', baseline.routeDigest === currentFirst.routeDigest,
    `baseline=${baseline.routeCount}/${baseline.routeDigest.slice(0, 12)} current=${currentFirst.routeCount}/${currentFirst.routeDigest.slice(0, 12)}`)

  const summary = {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
    baselineSource: 'git archive HEAD backend',
    flags,
    databases: Object.fromEntries(Object.entries(currentFirst.snapshot).map(([path, snapshot]) => [path, {
      digest: snapshot.digest,
      sqliteObjects: snapshot.entries.length,
      tables: Object.keys(snapshot.rowCounts).length,
      rows: snapshot.rowCounts
    }])),
    baselineDigests: Object.fromEntries(Object.entries(baseline.snapshot).map(([path, snapshot]) => [path, snapshot.digest])),
    volatileTables: volatile,
    drift,
    schemaSelfTest,
    rowSelfTest,
    checks,
    ok: checks.every((item) => item.ok)
  }
  writeFileSync(outcomePath, `${JSON.stringify(summary, null, 2)}\n`, 'utf8')
  return summary
}

async function main() {
  try {
    const summary = await runGuard()
    for (const item of summary.checks) {
      process.stdout.write(`${item.ok ? 'PASS' : 'FAIL'} ${item.id}${item.detail ? ` :: ${item.detail}` : ''}\n`)
    }
    process.stdout.write(`evidence=${outcomePath}\n`)
    if (!summary.checks[0].ok || !summary.checks[1].ok) process.exit(17)
    if (!summary.ok) process.exit(16)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(16)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await main()
}
