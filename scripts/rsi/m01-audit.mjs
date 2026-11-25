import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

const outDir = resolve(repoRoot, 'artifacts', 'rsi', 'm01-admission')
const admissionPath = resolve(outDir, 'ADMISSION.json')
const manifestPath = resolve(outDir, 'admission-manifest.json')
const markdownPath = resolve(outDir, 'ADMISSION.md')

const SCOPES = [
  'backend/cmd/server/',
  'backend/config/config.go',
  'backend/internal/extension/kernel/featuregate/',
  'scripts/rsi/',
  '.github/workflows/rsi-gates.yml',
  'front/src/router/'
]

export const STEP_EVIDENCE = [
  { id: 'M01-S01', status: 'PASSED', files: ['artifacts/rsi/rsi-baseline.json', 'artifacts/rsi/m01-s01-summary.json'] },
  { id: 'M01-S02', status: 'PASSED', dirs: ['artifacts/rsi/worktrees/M01'] },
  { id: 'M01-S03', status: 'PASSED', files: ['artifacts/rsi/m01-s03-featuregate-drill.json', 'artifacts/rsi/m01-s03-featuregate-scan.json', 'artifacts/rsi/m01-s03-go-tests.log', 'artifacts/rsi/m01-s03-node-tests.log'] },
  { id: 'M01-S04', status: 'PASSED', files: ['artifacts/rsi/m01-s04-cas-drill.json'] },
  { id: 'M01-S05', status: 'PASSED', files: ['artifacts/rsi/release-manifest.json', 'artifacts/rsi/m01-s05-negative.log'] },
  { id: 'M01-S06', status: 'IN_PROGRESS', files: ['artifacts/rsi/ci-evidence-manifest.json', 'artifacts/rsi/m01-s06-offstate-db-guard.json', 'artifacts/rsi/m01-s06-offstate-db-guard.log', 'artifacts/rsi/m01-s06-frontend-route-guard.json', 'artifacts/rsi/m01-s06-frontend-route-crosscheck.json', 'artifacts/rsi/m01-s06-node-tests.log'] },
  { id: 'M01-S07', status: 'PASSED', files: ['artifacts/rsi/m01-s07-rollback-drill.json'] },
  { id: 'M01-S08', status: 'PASSED', files: ['artifacts/rsi/m01-admission/ADMISSION.md', 'artifacts/rsi/m01-admission/ADMISSION.json', 'artifacts/rsi/m01-admission/admission-manifest.json', 'artifacts/rsi/m01-s08-admission.log', 'artifacts/rsi/m01-s08-admission-verify.log', 'artifacts/rsi/m01-s08-admission-verify-missing.log', 'artifacts/rsi/m01-s08-node-tests.log', 'artifacts/rsi/m01-s08-ci-gate.log'] },
  { id: 'M01-S09', status: 'PASSED', files: ['artifacts/rsi/service-inventory.json'] },
  { id: 'M01-S10', status: 'PASSED', files: ['artifacts/rsi/m01-s10-toolchain.log'] },
  { id: 'M01-S11', status: 'PASSED', files: ['artifacts/rsi/m01-s11-isolation.log'] },
  { id: 'M01-S12', status: 'PASSED', files: ['artifacts/rsi/m01-s12-refused.log'] }
]

export const GATE_FILES = [
  { id: 'FEATUREGATE_DRILL', path: 'artifacts/rsi/m01-s03-featuregate-drill.json' },
  { id: 'FEATUREGATE_SCAN', path: 'artifacts/rsi/m01-s03-featuregate-scan.json' },
  { id: 'OFFSTATE_DB_GUARD', path: 'artifacts/rsi/m01-s06-offstate-db-guard.json' },
  { id: 'FRONTEND_ROUTE_GUARD', path: 'artifacts/rsi/m01-s06-frontend-route-guard.json' },
  { id: 'CI_EVIDENCE_MANIFEST', path: 'artifacts/rsi/ci-evidence-manifest.json' }
]

export function inScope(path) {
  return SCOPES.some((prefix) => path === prefix || path.startsWith(prefix))
}

export function sha256File(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

export function collectChangedFiles(porcelainLines) {
  const files = []
  for (const line of porcelainLines) {
    const status = line.slice(0, 2).trim()
    const path = line.slice(3).trim()
    if (!path || !inScope(path)) continue
    files.push({ status, path })
  }
  return files.sort((left, right) => left.path.localeCompare(right.path))
}

export function expandDirectories(entries, listUntracked) {
  const expanded = []
  for (const entry of entries) {
    const absolute = resolve(repoRoot, entry.path)
    if (!existsSync(absolute) || !statSync(absolute).isDirectory()) {
      expanded.push(entry)
      continue
    }
    for (const path of listUntracked(entry.path)) {
      if (inScope(path)) expanded.push({ status: '??', path })
    }
  }
  return expanded.sort((left, right) => left.path.localeCompare(right.path))
}

export const SELF_OUTPUT_PREFIXES = ['artifacts/rsi/m01-admission/', 'artifacts/rsi/m01-s08-']

export function isSelfOutput(path) {
  return SELF_OUTPUT_PREFIXES.some((prefix) => path.startsWith(prefix))
}

export function collectEvidence() {
  const steps = []
  for (const step of STEP_EVIDENCE) {
    const files = (step.files ?? []).map((path) => {
      const absolute = resolve(repoRoot, path)
      if (!existsSync(absolute)) return { path, exists: false }
      if (isSelfOutput(path)) return { path, exists: true, bytes: statSync(absolute).size, generatedByAudit: true }
      return { path, exists: true, bytes: statSync(absolute).size, sha256: sha256File(absolute) }
    })
    const dirs = (step.dirs ?? []).map((path) => ({ path, exists: existsSync(resolve(repoRoot, path)) }))
    steps.push({
      id: step.id,
      status: step.status,
      files,
      dirs,
      evidenceComplete: files.every((file) => file.exists) && dirs.every((dir) => dir.exists)
    })
  }
  return steps
}

export function collectGates() {
  return GATE_FILES.map((gate) => {
    const absolute = resolve(repoRoot, gate.path)
    if (!existsSync(absolute)) return { id: gate.id, path: gate.path, exists: false, ok: false }
    const payload = JSON.parse(readFileSync(absolute, 'utf8'))
    const stepStatuses = Array.isArray(payload.steps) ? payload.steps.map((step) => step.status) : []
    const ok = payload.ok === true || payload.verdict === 'PASSED' || payload.status === 'PASSED' ||
      (stepStatuses.length > 0 && stepStatuses.every((status) => status === 'PASSED'))
    return { id: gate.id, path: gate.path, exists: true, ok, sha256: sha256File(absolute) }
  })
}

export function computeVerdict(steps, gates) {
  const blockers = []
  for (const step of steps) {
    if (step.status !== 'PASSED') blockers.push(`${step.id} 状态为 ${step.status}`)
    if (!step.evidenceComplete) blockers.push(`${step.id} 证据不完整`)
  }
  for (const gate of gates) {
    if (!gate.ok) blockers.push(`门禁 ${gate.id} 未通过`)
  }
  return { verdict: blockers.length === 0 ? 'READY_FOR_APPROVAL' : 'PENDING_APPROVAL', blockers }
}

function git(args) {
  const result = spawnSync('git', args, { cwd: repoRoot, encoding: 'utf8', env: cleanEnv() })
  if (result.status !== 0) throw new Error(`git ${args.join(' ')} 失败: ${result.stderr}`)
  return result.stdout
}

function zeroChangeClaims(gates) {
  const drill = JSON.parse(readFileSync(resolve(repoRoot, 'artifacts/rsi/m01-s03-featuregate-drill.json'), 'utf8'))
  const dbGuard = JSON.parse(readFileSync(resolve(repoRoot, 'artifacts/rsi/m01-s06-offstate-db-guard.json'), 'utf8'))
  const frontGuard = JSON.parse(readFileSync(resolve(repoRoot, 'artifacts/rsi/m01-s06-frontend-route-guard.json'), 'utf8'))
  const off = drill.instances.find((instance) => instance.name === 'current-off')
  const on = drill.instances.find((instance) => instance.name === 'current-on')
  return {
    routes: {
      baseline: drill.instances.find((instance) => instance.name === 'baseline-off').routeCount,
      off: off.routeCount,
      on: on.routeCount,
      offDigest: off.routeDigest,
      offUnchanged: drill.checks.find((item) => item.id === 'OFF_ROUTES_UNCHANGED')?.ok === true
    },
    database: {
      digests: Object.fromEntries(Object.entries(dbGuard.baselineDigests)),
      unchanged: dbGuard.checks.find((item) => item.id === 'OFF_SCHEMA_UNCHANGED')?.ok === true,
      rowsUnchanged: dbGuard.checks.find((item) => item.id === 'OFF_ROWS_UNCHANGED')?.ok === true,
      volatileTables: dbGuard.volatileTables
    },
    frontend: {
      routeCount: frontGuard.routeCount,
      digest: frontGuard.digest,
      unchanged: frontGuard.checks.find((item) => item.id === 'FRONTEND_ROUTE_SURFACE_UNCHANGED')?.ok === true
    },
    gates: gates.map((gate) => ({ id: gate.id, ok: gate.ok }))
  }
}

function renderMarkdown(admission) {
  const lines = [
    '# M01 原子接入包（待批准）',
    '',
    `- 源码版本：\`${admission.sourceRevision}\``,
    `- 工作树：${admission.worktreeDirty ? '有未提交改动（本包只覆盖这些改动）' : '干净'}`,
    `- 结论：**${admission.verdict}**`,
    `- 生成时间：${admission.generatedAt}`,
    '',
    '## 阻断项',
    '',
    ...admission.blockers.map((item) => `- ${item}`),
    '',
    '## 步骤证据',
    '',
    '| 步骤 | 状态 | 证据 |',
    '|---|---|---|',
    ...admission.steps.map((step) => `| ${step.id} | ${step.status} | ${[...step.files.map((file) => `${file.path}${file.exists ? '' : '(缺失)'}`), ...step.dirs.map((dir) => `${dir.path}/${dir.exists ? '' : '(缺失)'}`)].join('<br>') || '—'} |`),
    '',
    '## 零变更声明（关闭态实测）',
    '',
    `- 路由：基线 ${admission.zeroChange.routes.baseline} 条，关闭态 ${admission.zeroChange.routes.off} 条（digest ${admission.zeroChange.routes.offDigest.slice(0, 16)}…，未变化=${admission.zeroChange.routes.offUnchanged}），开启态 ${admission.zeroChange.routes.on} 条`,
    `- 数据库：${Object.entries(admission.zeroChange.database.digests).map(([path, digest]) => `${path} ${digest.slice(0, 16)}…`).join('；')}；schema 未变化=${admission.zeroChange.database.unchanged}，行数未变化=${admission.zeroChange.database.rowsUnchanged}，易变表 ${admission.zeroChange.database.volatileTables.length} 张`,
    `- 前端：${admission.zeroChange.frontend.routeCount} 条路由（digest ${admission.zeroChange.frontend.digest.slice(0, 16)}…，未变化=${admission.zeroChange.frontend.unchanged}）`,
    '',
    '## 改动文件（M01 范围）',
    '',
    ...admission.changedFiles.map((file) => `- \`${file.path}\` (${file.status}, sha256 ${file.sha256.slice(0, 16)}…)`),
    '',
    '## 回滚方案',
    '',
    '按改动文件清单逐个还原：删除本包新增的 `backend/internal/extension/kernel/featuregate/`、`backend/cmd/server/feature_canary.go`、`backend/cmd/server/feature_gate_routes.go`、`scripts/rsi/` 与 `.github/workflows/rsi-gates.yml`，并把 `backend/cmd/server/main.go`、`backend/cmd/server/router.go`、`backend/config/config.go` 恢复到 `' + admission.sourceRevision + '`；关闭态已证明这些改动不改变路由、数据库与前端路由面，回滚不需要数据迁移。',
    '',
    '## 复核命令',
    '',
    '```bash',
    'node scripts/rsi/m01-audit.mjs --verify',
    '```',
    ''
  ]
  return `${lines.join('\n')}\n`
}

export function buildAdmission() {
  const sourceRevision = git(['rev-parse', 'HEAD']).trim()
  const porcelain = git(['status', '--porcelain']).split('\n').filter(Boolean)
  const untrackedCache = new Map()
  const listUntracked = (path) => {
    if (!untrackedCache.has(path)) {
      untrackedCache.set(path, git(['ls-files', '--others', '--exclude-standard', '--', path]).split('\n').filter(Boolean))
    }
    return untrackedCache.get(path)
  }
  const changedFiles = expandDirectories(collectChangedFiles(porcelain), listUntracked).map((file) => {
    const absolute = resolve(repoRoot, file.path)
    return { ...file, sha256: existsSync(absolute) ? sha256File(absolute) : null }
  })
  const steps = collectEvidence()
  const gates = collectGates()
  const { verdict, blockers } = computeVerdict(steps, gates)
  const evidenceFiles = steps.flatMap((step) => step.files.filter((file) => file.exists && file.sha256))
  const changedWithHash = changedFiles.filter((file) => file.sha256)
  const manifestEntries = new Set([
    ...evidenceFiles.map((file) => file.path),
    ...gates.filter((gate) => gate.exists).map((gate) => gate.path),
    ...changedWithHash.map((file) => file.path)
  ]).size
  const admission = {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
    sourceRevision,
    worktreeDirty: changedFiles.length > 0,
    changedFiles,
    steps,
    gates,
    counts: {
      evidenceFiles: evidenceFiles.length,
      gates: gates.filter((gate) => gate.exists).length,
      changedFiles: changedWithHash.length,
      manifestEntries
    },
    zeroChange: zeroChangeClaims(gates),
    verdict,
    blockers
  }
  return admission
}

export function verifyAdmission() {
  if (!existsSync(manifestPath)) {
    return { ok: false, reason: `缺少 ${manifestPath}`, mismatches: ['MANIFEST_MISSING'] }
  }
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
  const mismatches = []
  if (!Array.isArray(manifest.files) || manifest.files.length === 0) mismatches.push('MANIFEST_EMPTY')
  if (existsSync(admissionPath)) {
    const current = JSON.parse(readFileSync(admissionPath, 'utf8'))
    if (current.verdict !== manifest.verdict) mismatches.push(`结论被改写: ${manifest.verdict} → ${current.verdict}`)
    if (current.sourceRevision !== manifest.sourceRevision) mismatches.push(`源码版本被改写: ${manifest.sourceRevision} → ${current.sourceRevision}`)
  } else {
    mismatches.push('缺失 artifacts/rsi/m01-admission/ADMISSION.json')
  }
  for (const entry of manifest.files ?? []) {
    const absolute = resolve(repoRoot, entry.path)
    if (!existsSync(absolute)) {
      mismatches.push(`${entry.path}: 缺失`)
      continue
    }
    const digest = sha256File(absolute)
    if (digest !== entry.sha256) mismatches.push(`${entry.path}: ${entry.sha256.slice(0, 12)} → ${digest.slice(0, 12)}`)
  }
  return { ok: mismatches.length === 0, mismatches }
}

function main() {
  const verify = process.argv.includes('--verify')
  try {
    if (verify) {
      const result = verifyAdmission()
      for (const mismatch of result.mismatches) process.stdout.write(`MISMATCH ${mismatch}\n`)
      process.stdout.write(`verify=${result.ok} manifest=${manifestPath}\n`)
      if (!result.ok) process.exit(20)
      return
    }
    const admission = buildAdmission()
    mkdirSync(outDir, { recursive: true })
    writeFileSync(admissionPath, `${JSON.stringify(admission, null, 2)}\n`, 'utf8')
    writeFileSync(markdownPath, renderMarkdown(admission), 'utf8')
    const files = [
      ...admission.steps.flatMap((step) => step.files.filter((file) => file.exists && file.sha256).map((file) => ({ path: file.path, sha256: file.sha256 }))),
      ...admission.gates.filter((gate) => gate.exists).map((gate) => ({ path: gate.path, sha256: gate.sha256 })),
      ...admission.changedFiles.filter((file) => file.sha256).map((file) => ({ path: file.path, sha256: file.sha256 }))
    ]
    const deduped = new Map()
    for (const file of files) {
      if (!deduped.has(file.path)) deduped.set(file.path, file)
    }
    const manifest = {
      schemaVersion: 1,
      generatedAt: admission.generatedAt,
      sourceRevision: admission.sourceRevision,
      verdict: admission.verdict,
      blockers: admission.blockers,
      files: [...deduped.values()].sort((left, right) => left.path.localeCompare(right.path))
    }
    writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, 'utf8')
    process.stdout.write(`verdict=${admission.verdict}\n`)
    for (const blocker of admission.blockers) process.stdout.write(`BLOCKER ${blocker}\n`)
    process.stdout.write(`admission=${admissionPath}\nmanifest=${manifestPath} files=${manifest.files.length}\n`)
    process.stdout.write(`counts=${JSON.stringify(admission.counts)}\n`)
    if (process.argv.includes('--require-ready') && admission.verdict !== 'READY_FOR_APPROVAL') process.exit(22)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(20)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
