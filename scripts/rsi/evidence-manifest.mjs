import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import { isAbsolute, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'

const STATUSES = ['NOT_STARTED', 'IN_PROGRESS', 'PASSED', 'BLOCKED', 'FAILED', 'REOPENED']

const BASE_FIELDS = ['id', 'status']

const PASSED_REQUIRED_FIELDS = ['testCommand', 'exitCode', 'testLogSha256', 'isolationProof', 'reviewer', 'finishedAt']

const ISO_PATTERN = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/

const SHA256_PATTERN = /^[0-9a-f]{64}$/

export function sha256File(absolutePath) {
  return createHash('sha256').update(readFileSync(absolutePath)).digest('hex')
}

export function loadTaskIds(absolutePath) {
  return new Set(JSON.parse(readFileSync(absolutePath, 'utf8')).taskIds)
}

export function validateEvidence(manifest, options = {}) {
  const violations = []
  const taskIds = options.taskIds ?? null

  if (!manifest || !Array.isArray(manifest.steps) || manifest.steps.length === 0) {
    return { ok: false, violations: [{ code: 'EMPTY_MANIFEST', message: '证据清单为空，无法判定任何步骤' }] }
  }

  const seen = new Set()
  let passed = 0
  for (const step of manifest.steps) {
    for (const field of BASE_FIELDS) {
      if (step[field] === undefined || step[field] === null || step[field] === '') {
        violations.push({ code: 'FIELD_MISSING', message: `${step.id ?? '<无ID>'} 缺少字段 ${field}` })
      }
    }
    if (step.id) {
      if (seen.has(step.id)) violations.push({ code: 'DUPLICATE_STEP', message: `${step.id} 在清单中重复` })
      seen.add(step.id)
      if (taskIds && !taskIds.has(step.id)) {
        violations.push({ code: 'UNKNOWN_STEP', message: `${step.id} 不在冻结的 452 步集合内` })
      }
    }
    if (step.status && !STATUSES.includes(step.status)) {
      violations.push({ code: 'BAD_STATUS', message: `${step.id} 状态非法：${step.status}` })
    }
    if (step.status !== 'PASSED') continue
    passed += 1

    for (const field of PASSED_REQUIRED_FIELDS) {
      if (step[field] === undefined || step[field] === null || step[field] === '') {
        violations.push({ code: 'FIELD_MISSING', message: `${step.id} 标记 PASSED 但缺少字段 ${field}` })
      }
    }

    if (typeof step.exitCode !== 'number' || step.exitCode !== 0) {
      violations.push({ code: 'NONZERO_EXIT', message: `${step.id} 标记 PASSED 但 exitCode=${step.exitCode}` })
    }
    if (!SHA256_PATTERN.test(String(step.testLogSha256 ?? ''))) {
      violations.push({ code: 'LOG_HASH_INVALID', message: `${step.id} 的 testLogSha256 不是合法 sha256` })
    } else if (step.testLogPath) {
      const absolute = isAbsolute(step.testLogPath) ? step.testLogPath : resolve(repoRoot, step.testLogPath)
      if (!existsSync(absolute)) {
        violations.push({ code: 'LOG_MISSING', message: `${step.id} 引用的测试日志不存在：${absolute}` })
      } else {
        const actual = sha256File(absolute)
        if (actual !== step.testLogSha256) {
          violations.push({ code: 'LOG_TAMPERED', message: `${step.id} 测试日志哈希不匹配：清单 ${step.testLogSha256}，实际 ${actual}` })
        }
      }
    } else {
      violations.push({ code: 'LOG_PATH_MISSING', message: `${step.id} 标记 PASSED 但未提供 testLogPath，无法复核` })
    }
    if (typeof step.testCommand !== 'string' || step.testCommand.trim() === '') {
      violations.push({ code: 'NO_TEST_COMMAND', message: `${step.id} 没有可复核的测试命令` })
    }
    if (typeof step.isolationProof !== 'string' || step.isolationProof.trim() === '') {
      violations.push({ code: 'NO_ISOLATION_PROOF', message: `${step.id} 缺少零影响隔离证明` })
    }
    if (typeof step.reviewer !== 'string' || step.reviewer.trim() === '') {
      violations.push({ code: 'NO_REVIEWER', message: `${step.id} 缺少 reviewer` })
    }
    if (!ISO_PATTERN.test(String(step.finishedAt ?? ''))) {
      violations.push({ code: 'BAD_TIMESTAMP', message: `${step.id} finishedAt 不是 ISO 时间：${step.finishedAt}` })
    }
  }

  return { ok: violations.length === 0, violations, passed }
}

export function validatePromotion(manifest, requiredSteps, options = {}) {
  const base = validateEvidence(manifest, options)
  const violations = [...base.violations]
  const byId = new Map(manifest.steps.map((step) => [step.id, step]))
  for (const required of requiredSteps) {
    const step = byId.get(required)
    if (!step) {
      violations.push({ code: 'PREREQUISITE_MISSING', message: `发布前置 ${required} 未出现在证据清单` })
    } else if (step.status !== 'PASSED') {
      violations.push({ code: 'PREREQUISITE_NOT_PASSED', message: `发布前置 ${required} 状态为 ${step.status}` })
    }
  }
  return { ok: violations.length === 0, violations, passed: base.passed }
}

function parseArgs(argv) {
  const options = { action: 'validate' }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === '--manifest') options.manifest = argv[index + 1]
    else if (arg === '--taskids') options.taskids = argv[index + 1]
    else if (arg === '--for-promotion') {
      options.action = 'promotion'
      options.required = (argv[index + 1] || '').split(',').filter(Boolean)
    }
  }
  return options
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const options = parseArgs(process.argv.slice(2))
  if (!options.manifest) {
    process.stderr.write('必须提供 --manifest <证据清单路径>\n')
    process.exit(4)
  }
  const manifest = JSON.parse(readFileSync(resolve(options.manifest), 'utf8'))
  const taskIds = options.taskids ? loadTaskIds(resolve(options.taskids)) : null
  const result =
    options.action === 'promotion'
      ? validatePromotion(manifest, options.required ?? [], { taskIds })
      : validateEvidence(manifest, { taskIds })
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`)
  if (!result.ok) {
    process.stderr.write(`证据校验失败：\n${result.violations.map((item) => `- [${item.code}] ${item.message}`).join('\n')}\n`)
    process.exit(6)
  }
}
