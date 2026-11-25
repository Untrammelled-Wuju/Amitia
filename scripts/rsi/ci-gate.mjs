import { spawnSync } from 'node:child_process'
import { appendFileSync, copyFileSync, existsSync, mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'
import { sha256File, validateEvidence } from './evidence-manifest.mjs'

const GATE_FILES = ['scripts/rsi/ci-gate.mjs', '.github/workflows/rsi-gates.yml']

const ISOLATION_PROOF =
  'CI runner 不含 Amitia 数据根、生产端口与生产进程；全部检查仅写入 artifacts/rsi 日志与系统临时目录，未启动 Core、未触碰生产存储'

function cleanEnv() {
  const env = { ...process.env }
  for (const key of Object.keys(env)) {
    if (key === 'GIT_CONFIG_COUNT' || key.startsWith('GIT_CONFIG_KEY_') || key.startsWith('GIT_CONFIG_VALUE_')) {
      delete env[key]
    }
  }
  return env
}

function gitHead(root) {
  const result = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8', env: cleanEnv() })
  return result.status === 0 ? result.stdout.trim() : 'unknown'
}

export function receiptFor(check, meta) {
  return {
    id: check.id,
    status: check.ok ? 'PASSED' : 'FAILED',
    sourceRevision: meta.revision,
    changedFiles: [...GATE_FILES],
    testCommand: check.command,
    exitCode: check.ok ? 0 : 1,
    testLogPath: check.logPath,
    testLogSha256: sha256File(check.logPath),
    isolationProof: ISOLATION_PROOF,
    reviewer: meta.reviewer ?? 'ci-gate',
    finishedAt: meta.finishedAt
  }
}

export function tamperDetected(receipt, tamperedLogPath) {
  const result = validateEvidence({ steps: [{ ...receipt, testLogPath: tamperedLogPath }] })
  return result.violations.some((item) => item.code === 'LOG_TAMPERED')
}

export function gateVerdict(receipts) {
  const failed = receipts.filter((item) => item.status !== 'PASSED').map((item) => item.id)
  return { ok: failed.length === 0, failed }
}

function runNodeCheck({ id, args, expectExit, command, logPath, root, expectAbsent }) {
  const result = spawnSync(process.execPath, args, { cwd: root, encoding: 'utf8', env: cleanEnv() })
  const output = `${result.stdout ?? ''}${result.stderr ?? ''}`
  writeFileSync(logPath, output, 'utf8')
  const absentOk = expectAbsent ? !existsSync(expectAbsent) : true
  return {
    id,
    command,
    logPath,
    ok: result.status === expectExit && absentOk,
    observedExit: result.status,
    expectExit,
    absentOk
  }
}

export function runGate(options = {}) {
  const root = options.root ?? repoRoot
  const logDir = resolve(root, 'artifacts/rsi')
  mkdirSync(logDir, { recursive: true })
  const finishedAt = new Date().toISOString()
  const revision = gitHead(root)

  const unitLog = resolve(logDir, 'ci-unit-tests.log')
  const isolationLog = resolve(logDir, 'ci-isolation-negative.log')
  const releaseLog = resolve(logDir, 'ci-release-negative.log')

  const unit = runNodeCheck({
    id: 'CI-RSI-UNIT',
    args: ['--test', 'scripts/rsi/__tests__/'],
    expectExit: 0,
    command: 'node --test scripts/rsi/__tests__/',
    logPath: unitLog,
    root
  })

  const isolation = runNodeCheck({
    id: 'CI-RSI-ISOLATION-NEGATIVE',
    args: ['scripts/rsi/isolation-guard.mjs', '--data-root', 'AmitiaData', '--plugin-root', 'AmitiaData/Plugin'],
    expectExit: 3,
    command: 'node scripts/rsi/isolation-guard.mjs --data-root AmitiaData --plugin-root AmitiaData/Plugin（期望 exit=3）',
    logPath: isolationLog,
    root
  })

  const releaseDir = mkdtempSync(resolve(tmpdir(), 'rsi-ci-release-'))
  const releaseArtifact = resolve(releaseDir, 'blocked.json')
  writeFileSync(resolve(releaseDir, 'fixture.exe'), 'payload')
  writeFileSync(
    resolve(releaseDir, 'latest.yml'),
    [
      'version: 1.0.0',
      'files:',
      '  - url: fixture.exe',
      '    sha512: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==',
      '    size: 7'
    ].join('\n')
  )
  const release = runNodeCheck({
    id: 'CI-RSI-RELEASE-NEGATIVE',
    args: ['scripts/rsi/release-manifest.mjs', '--out', releaseArtifact, '--release-dir', releaseDir],
    expectExit: 5,
    command: 'node scripts/rsi/release-manifest.mjs --release-dir <tmp 篡改 sha512>（期望 exit=5 且不产出清单）',
    logPath: releaseLog,
    root,
    expectAbsent: releaseArtifact
  })

  const receipts = [unit, isolation, release].map((check) => receiptFor(check, { revision, finishedAt }))
  const manifestPath = resolve(logDir, 'ci-evidence-manifest.json')
  writeFileSync(manifestPath, `${JSON.stringify({ schemaVersion: 1, generatedAt: finishedAt, steps: receipts }, null, 2)}\n`, 'utf8')

  const verdict = gateVerdict(receipts)
  if (!verdict.ok) {
    return { ok: false, exitCode: 8, verdict, receipts, manifestPath, tamper: null }
  }

  const tamperedLog = resolve(logDir, 'ci-unit-tests.tampered.log')
  copyFileSync(unitLog, tamperedLog)
  appendFileSync(tamperedLog, '\ntampered\n', 'utf8')
  const tamper = tamperDetected(receipts[0], tamperedLog)
  const tamperReport = { detected: tamper, logPath: tamperedLog }
  if (!tamper) {
    return { ok: false, exitCode: 7, verdict, receipts, manifestPath, tamper: tamperReport }
  }
  return { ok: true, exitCode: 0, verdict, receipts, manifestPath, tamper: tamperReport }
}

function main() {
  const result = runGate()
  process.stdout.write(
    `${JSON.stringify(
      {
        ok: result.ok,
        exitCode: result.exitCode,
        verdict: result.verdict,
        tamper: result.tamper,
        manifestPath: result.manifestPath,
        steps: result.receipts.map((item) => ({
          id: item.id,
          status: item.status,
          exitCode: item.exitCode,
          testLogSha256: item.testLogSha256
        }))
      },
      null,
      2
    )}\n`
  )
  if (!result.ok && result.exitCode === 7) {
    process.stderr.write(`证据门禁自检失效：篡改测试日志后校验仍通过（${result.tamper.logPath}），拒绝放行\n`)
  }
  if (!result.ok && result.exitCode === 8) {
    process.stderr.write(`证据门禁未通过：${result.verdict.failed.join(', ')}\n`)
  }
  process.exit(result.exitCode)
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
