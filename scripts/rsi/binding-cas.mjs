import { createHash } from 'node:crypto'
import { appendFileSync, existsSync, mkdirSync, readFileSync, readdirSync, renameSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

export class BindingError extends Error {
  constructor(code, message, details = {}) {
    super(message, details.cause ? { cause: details.cause } : undefined)
    this.name = 'BindingError'
    this.code = code
    this.details = details
  }
}

function canonical(value) {
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`
  if (value && typeof value === 'object') {
    return `{${Object.keys(value)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${canonical(value[key])}`)
      .join(',')}}`
  }
  return JSON.stringify(value)
}

export function digestOf(value) {
  return createHash('sha256').update(canonical(value)).digest('hex')
}

function atomicWrite(path, text) {
  mkdirSync(dirname(path), { recursive: true })
  const temporary = `${path}.${process.pid}.tmp`
  writeFileSync(temporary, text, 'utf8')
  renameSync(temporary, path)
}

function appendLedger(ledgerPath, record) {
  if (!ledgerPath) return
  mkdirSync(dirname(ledgerPath), { recursive: true })
  appendFileSync(ledgerPath, `${JSON.stringify(record)}\n`, 'utf8')
}

export function writePlan(directory, plan) {
  const digest = digestOf(plan)
  mkdirSync(directory, { recursive: true })
  const existing = readdirSync(directory).filter((name) => name.startsWith(`${plan.planId}-`) && name.endsWith('.json'))
  for (const name of existing) {
    const candidate = resolve(directory, name)
    const stored = JSON.parse(readFileSync(candidate, 'utf8'))
    const { digest: storedDigest, ...storedPlan } = stored
    if (digestOf(storedPlan) === digest && storedDigest === digest) {
      return { path: candidate, digest, created: false }
    }
  }
  if (existing.length > 0) {
    throw new BindingError('IMMUTABLE_CONFLICT', `不可变发布计划冲突：planId=${plan.planId} 已存在不同内容（${existing.join(', ')}）`)
  }
  const path = resolve(directory, `${plan.planId}-${digest.slice(0, 12)}.json`)
  atomicWrite(path, `${JSON.stringify({ ...plan, digest }, null, 2)}\n`)
  return { path, digest, created: true }
}

export function readState(statePath) {
  if (!existsSync(statePath)) {
    throw new BindingError('STATE_MISSING', `绑定状态文件不存在：${statePath}`)
  }
  const state = JSON.parse(readFileSync(statePath, 'utf8'))
  if (!Number.isInteger(state.revision) || !state.binding) {
    throw new BindingError('STATE_INVALID', `绑定状态格式非法：${statePath}`)
  }
  return state
}

export function casSwitch({ statePath, expectedRevision, nextBinding, ledgerPath, at = new Date().toISOString() }) {
  const current = readState(statePath)
  if (current.revision !== expectedRevision) {
    throw new BindingError('CAS_CONFLICT', `CAS 版本冲突：期望 ${expectedRevision}，实际 ${current.revision}`, {
      expected: expectedRevision,
      actual: current.revision,
      binding: current.binding
    })
  }
  const next = { revision: current.revision + 1, binding: nextBinding }
  atomicWrite(statePath, `${JSON.stringify(next, null, 2)}\n`)
  appendLedger(ledgerPath, {
    event: 'switch',
    at,
    fromRevision: current.revision,
    toRevision: next.revision,
    from: current.binding,
    to: nextBinding
  })
  return { revision: next.revision, previousRevision: current.revision, previousBinding: current.binding, binding: nextBinding }
}

export function rollback({ statePath, receipt, ledgerPath, at = new Date().toISOString() }) {
  const current = readState(statePath)
  if (current.revision !== receipt.revision) {
    throw new BindingError(
      'ROLLBACK_STALE',
      `回滚基线已过期：回执修订 ${receipt.revision}，当前 ${current.revision}，拒绝盲目回退`,
      { receiptRevision: receipt.revision, actual: current.revision }
    )
  }
  const next = { revision: current.revision + 1, binding: receipt.previousBinding }
  atomicWrite(statePath, `${JSON.stringify(next, null, 2)}\n`)
  appendLedger(ledgerPath, {
    event: 'rollback',
    at,
    fromRevision: current.revision,
    toRevision: next.revision,
    from: current.binding,
    to: receipt.previousBinding
  })
  return { revision: next.revision, binding: receipt.previousBinding }
}

export function markUnknown({ ledgerPath, receipt, error, at = new Date().toISOString() }) {
  const record = {
    event: 'unknown',
    at,
    receiptRevision: receipt?.revision ?? null,
    code: error?.code ?? 'UNKNOWN',
    message: error?.message ?? String(error),
    cause: error?.cause ? String(error.cause?.message ?? error.cause) : null,
    replay: false
  }
  appendLedger(ledgerPath, record)
  return record
}

export function readLedger(ledgerPath) {
  if (!existsSync(ledgerPath)) return []
  return readFileSync(ledgerPath, 'utf8')
    .split('\n')
    .filter(Boolean)
    .map((line) => JSON.parse(line))
}

function main() {
  const [command, ...rest] = process.argv.slice(2)
  const options = {}
  for (let index = 0; index < rest.length; index += 2) options[rest[index].replace(/^--/, '')] = rest[index + 1]
  try {
    if (command === 'switch') {
      const result = casSwitch({
        statePath: resolve(options.state),
        expectedRevision: Number(options.expected),
        nextBinding: JSON.parse(options.binding),
        ledgerPath: options.ledger ? resolve(options.ledger) : null
      })
      process.stdout.write(`${JSON.stringify(result, null, 2)}\n`)
      return
    }
    if (command === 'state') {
      process.stdout.write(`${JSON.stringify(readState(resolve(options.state)), null, 2)}\n`)
      return
    }
    process.stderr.write('用法：binding-cas.mjs state --state <path> | switch --state <path> --expected <n> --binding <json> [--ledger <path>]\n')
    process.exit(4)
  } catch (error) {
    process.stderr.write(`${JSON.stringify({ code: error.code ?? 'ERROR', message: error.message, details: error.details ?? null }, null, 2)}\n`)
    process.exit(error.code === 'CAS_CONFLICT' ? 9 : 10)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
