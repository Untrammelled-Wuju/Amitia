import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'
import { validateEvidence } from './evidence-manifest.mjs'

const GROUPS_FILE = resolve(repoRoot, 'scripts', 'rsi', 'release-groups.json')

export function sha256Text(text) {
  return createHash('sha256').update(text).digest('hex')
}

export function loadGroups(groupsPath = GROUPS_FILE) {
  const raw = readFileSync(groupsPath, 'utf8')
  const parsed = JSON.parse(raw)
  if (!Array.isArray(parsed.groups) || parsed.groups.length === 0) {
    throw new Error(`发布组定义为空：${groupsPath}`)
  }
  for (const group of parsed.groups) {
    if (!group.id || !Array.isArray(group.steps) || group.steps.length === 0) {
      throw new Error(`发布组缺少 id 或 steps：${JSON.stringify(group)}`)
    }
  }
  return { ...parsed, groupsPath, digest: sha256Text(raw) }
}

export function findGroup(groups, id) {
  const group = groups.groups.find((item) => item.id === id)
  if (!group) {
    throw new Error(`未定义的发布组：${id}（可用：${groups.groups.map((item) => item.id).join(', ')}）`)
  }
  return group
}

export function evaluateGroup(manifest, group, groups) {
  const evidence = validateEvidence(manifest)
  const byId = new Map((manifest.steps ?? []).map((step) => [step.id, step]))
  const missing = []
  const failed = []
  const steps = []

  for (const id of group.steps) {
    const step = byId.get(id)
    if (!step) {
      missing.push(id)
      steps.push({ id, status: 'MISSING' })
      continue
    }
    steps.push({ id, status: step.status, testCommand: step.testCommand ?? null, testLogSha256: step.testLogSha256 ?? null })
    if (step.status !== 'PASSED') failed.push(id)
  }

  const relatedViolations = evidence.violations.filter((item) => group.steps.some((id) => String(item.message).includes(id)))
  const ok = missing.length === 0 && failed.length === 0 && relatedViolations.length === 0

  return {
    ok,
    group: group.id,
    name: group.name,
    steps,
    missing,
    failed,
    violations: relatedViolations,
    gateDigest: sha256Text(
      JSON.stringify({
        group: group.id,
        steps: steps.map((item) => [item.id, item.status, item.testLogSha256]),
        groupsDigest: groups.digest
      })
    )
  }
}

export function buildPermit(result, manifestText, groups) {
  return {
    schemaVersion: 1,
    verdict: 'APPROVED',
    group: result.group,
    name: result.name,
    steps: result.steps,
    gateDigest: result.gateDigest,
    evidenceManifestSha256: sha256Text(manifestText),
    groupsDigest: groups.digest,
    generatedAt: new Date().toISOString()
  }
}

function parseArgs(argv) {
  const options = { groupsPath: GROUPS_FILE }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === '--manifest') options.manifest = argv[index + 1]
    else if (arg === '--group') options.group = argv[index + 1]
    else if (arg === '--groups') options.groupsPath = resolve(argv[index + 1])
    else if (arg === '--out') options.out = argv[index + 1]
  }
  return options
}

function main() {
  const options = parseArgs(process.argv.slice(2))
  if (!options.manifest || !options.group) {
    process.stderr.write('必须提供 --manifest <证据清单> 与 --group <发布组 ID>\n')
    process.exit(4)
  }
  const manifestPath = resolve(options.manifest)
  if (!existsSync(manifestPath)) {
    process.stderr.write(`证据清单不存在：${manifestPath}\n`)
    process.exit(4)
  }
  const manifestText = readFileSync(manifestPath, 'utf8')
  const groups = loadGroups(options.groupsPath)
  const group = findGroup(groups, options.group)
  const result = evaluateGroup(JSON.parse(manifestText), group, groups)

  if (!result.ok) {
    process.stdout.write(`${JSON.stringify(result, null, 2)}\n`)
    process.stderr.write(
      `发布许可被拒绝：missing=${result.missing.join(',') || '-'} failed=${result.failed.join(',') || '-'} violations=${result.violations.length}\n`
    )
    process.exit(6)
  }

  const permit = buildPermit(result, manifestText, groups)
  if (options.out) {
    const target = resolve(options.out)
    mkdirSync(dirname(target), { recursive: true })
    writeFileSync(target, `${JSON.stringify(permit, null, 2)}\n`, 'utf8')
  }
  process.stdout.write(`${JSON.stringify({ ok: true, permit }, null, 2)}\n`)
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
