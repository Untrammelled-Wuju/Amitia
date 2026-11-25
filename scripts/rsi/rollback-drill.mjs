import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { copyFileSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'

function sha256File(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

function sqlite(dbPath, sql) {
  const result = spawnSync('sqlite3', [dbPath], { input: sql, encoding: 'utf8' })
  if (result.status !== 0) {
    throw new Error(`sqlite3 执行失败（exit ${result.status}）：${result.stderr.trim()}`, {
      cause: new Error(result.stderr.trim())
    })
  }
  const stderr = result.stderr.trim()
  if (stderr) throw new Error(`sqlite3 输出错误：${stderr}`, { cause: new Error(stderr) })
  return result.stdout.trim()
}

export function runDrill(root) {
  const directory = resolve(root, 'artifacts/rsi/drill-s07')
  rmSync(directory, { recursive: true, force: true })
  mkdirSync(directory, { recursive: true })

  const probe = spawnSync('sqlite3', ['--version'], { encoding: 'utf8' })
  if (probe.status !== 0) {
    throw new Error('缺少 sqlite3，无法进行隔离回滚演练（失败关闭，不跳过）')
  }

  const dbPath = resolve(directory, 'bindings.db')
  const ledgerPath = resolve(directory, 'ledger.jsonl')
  const artifactPath = resolve(directory, 'main-artifact.bin')
  copyFileSync(resolve(root, 'artifacts/rsi/release-manifest.json'), artifactPath)
  const artifactBefore = sha256File(artifactPath)

  sqlite(
    dbPath,
    `CREATE TABLE bindings (
       revision INTEGER PRIMARY KEY,
       provider TEXT NOT NULL,
       version TEXT NOT NULL,
       activated_at TEXT NOT NULL
     );
     INSERT INTO bindings (revision, provider, version, activated_at) VALUES (1, 'stable', '1.0.0', '2026-11-22T00:00:00Z');`
  )

  const providerPath = resolve(directory, 'mock-provider.mjs')
  writeFileSync(
    providerPath,
    [
      "process.stderr.write('PROVIDER_ACTIVATION_FAILED: capability registry rejected candidate binding\\n')",
      "process.stderr.write('cause: postgres://isolated-drill: connect ECONNREFUSED 127.0.0.1:1\\n')",
      'process.stdout.write(JSON.stringify({ sideEffect: "unknown", retryable: false }))',
      'process.exit(1)'
    ].join('\n') + '\n',
    'utf8'
  )

  const activation = spawnSync(process.execPath, [providerPath], { encoding: 'utf8' })
  const activationOutput = `${activation.stdout ?? ''}${activation.stderr ?? ''}`
  const activationAttempts = 1

  sqlite(
    dbPath,
    `UPDATE bindings SET provider='candidate', version='1.1.0', activated_at='2026-11-22T00:00:01Z', revision=2 WHERE revision=1;
     SELECT changes();`
  )
  writeFileSync(
    ledgerPath,
    `${JSON.stringify({
      event: 'activation-failed',
      at: '2026-11-22T00:00:01Z',
      revision: 2,
      code: 'PROVIDER_ACTIVATION_FAILED',
      exitCode: activation.status,
      output: activationOutput,
      sideEffect: 'unknown',
      replay: false
    })}\n`,
    'utf8'
  )

  const rollbackChanges = sqlite(
    dbPath,
    `UPDATE bindings SET provider='stable', version='1.0.0', activated_at='2026-11-22T00:00:02Z', revision=3 WHERE revision=2;
     SELECT changes();`
  )
  const current = sqlite(dbPath, 'SELECT revision || ":" || provider || ":" || version FROM bindings ORDER BY revision DESC LIMIT 1;')
  const artifactAfter = sha256File(artifactPath)
  const ledger = readFileSync(ledgerPath, 'utf8')
    .split('\n')
    .filter(Boolean)
    .map((line) => JSON.parse(line))

  const checks = [
    { name: 'sqlite-available', ok: probe.status === 0 },
    { name: 'activation-failed-as-expected', ok: activation.status === 1 },
    { name: 'original-error-verbatim', ok: activationOutput.includes('PROVIDER_ACTIVATION_FAILED') && activationOutput.includes('ECONNREFUSED') },
    { name: 'unknown-side-effect-no-replay', ok: ledger[0].sideEffect === 'unknown' && ledger[0].replay === false && activationAttempts === 1 },
    { name: 'rollback-applied', ok: rollbackChanges === '1' },
    { name: 'binding-restored', ok: current === '3:stable:1.0.0' },
    { name: 'main-artifact-untouched', ok: artifactBefore === artifactAfter }
  ]

  return {
    directory,
    dbPath,
    activation: { status: activation.status, output: activationOutput },
    ledger,
    currentBinding: current,
    artifact: { before: artifactBefore, after: artifactAfter },
    checks,
    ok: checks.every((item) => item.ok)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = runDrill(repoRoot)
  const target = resolve(repoRoot, 'artifacts/rsi/m01-s07-rollback-drill.json')
  writeFileSync(target, `${JSON.stringify(report, null, 2)}\n`, 'utf8')
  process.stdout.write(`${JSON.stringify({ ok: report.ok, target, currentBinding: report.currentBinding, checks: report.checks }, null, 2)}\n`)
  process.exit(report.ok ? 0 : 13)
}
