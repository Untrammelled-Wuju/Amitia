import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'
import { casSwitch, readLedger, readState, rollback, writePlan } from './binding-cas.mjs'

export function runDrill(root) {
  const directory = resolve(root, 'artifacts/rsi/drill-s04')
  rmSync(directory, { recursive: true, force: true })
  mkdirSync(directory, { recursive: true })
  const statePath = resolve(directory, 'binding-state.json')
  const ledgerPath = resolve(directory, 'binding-ledger.jsonl')
  writeFileSync(statePath, `${JSON.stringify({ revision: 1, binding: { providerId: 'stable', version: '1.0.0', module: 'M01' } }, null, 2)}\n`)
  const before = readState(statePath)

  const plan = writePlan(resolve(directory, 'plans'), {
    planId: 'M01-drill',
    module: 'M01',
    createdAt: '2026-11-22T00:00:00Z',
    binding: { providerId: 'candidate', version: '1.1.0', module: 'M01' },
    declaredEffects: ['provider.register']
  })

  const switched = casSwitch({
    statePath,
    expectedRevision: 1,
    nextBinding: { providerId: 'candidate', version: '1.1.0', module: 'M01' },
    ledgerPath,
    at: '2026-11-22T00:00:01Z'
  })

  let staleSwitch = null
  try {
    casSwitch({ statePath, expectedRevision: 1, nextBinding: { providerId: 'blind', version: '9.9.9' }, ledgerPath })
    staleSwitch = { refused: false }
  } catch (error) {
    staleSwitch = { refused: true, code: error.code, message: error.message, details: error.details ?? null }
  }

  const restored = rollback({ statePath, receipt: switched, ledgerPath, at: '2026-11-22T00:00:02Z' })

  let staleRollback = null
  try {
    rollback({ statePath, receipt: switched, ledgerPath })
    staleRollback = { refused: false }
  } catch (error) {
    staleRollback = { refused: true, code: error.code, message: error.message, details: error.details ?? null }
  }

  const finalState = readState(statePath)
  const ledger = readLedger(ledgerPath)
  const checks = [
    { name: 'plan-immutable', ok: plan.created === true },
    { name: 'switch-revision', ok: switched.revision === 2 && switched.previousRevision === 1 },
    { name: 'stale-switch-refused', ok: staleSwitch.refused === true && staleSwitch.code === 'CAS_CONFLICT' },
    { name: 'rollback-restored', ok: restored.revision === 3 && restored.binding.providerId === 'stable' },
    { name: 'stale-rollback-refused', ok: staleRollback.refused === true && staleRollback.code === 'ROLLBACK_STALE' },
    { name: 'binding-equals-baseline', ok: JSON.stringify(finalState.binding) === JSON.stringify(before.binding) },
    { name: 'ledger-switch-then-rollback', ok: ledger.map((item) => item.event).join(',') === 'switch,rollback' }
  ]
  return { directory, plan, switched, staleSwitch, restored, staleRollback, finalState, ledger, checks, ok: checks.every((item) => item.ok) }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = runDrill(repoRoot)
  const target = resolve(repoRoot, 'artifacts/rsi/m01-s04-cas-drill.json')
  writeFileSync(target, `${JSON.stringify(report, null, 2)}\n`, 'utf8')
  process.stdout.write(`${JSON.stringify({ ok: report.ok, target, checks: report.checks }, null, 2)}\n`)
  process.exit(report.ok ? 0 : 12)
}
