import { createHash } from 'node:crypto'
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './toolchain-preflight.mjs'
import {
  buildBaselineSource,
  buildCoreBinary,
  diffRouteSets,
  prepareInstanceRoot,
  runCoreInstance
} from './lib/core-instance.mjs'

export { diffRouteSets, disableProviders, readFeatures, withFeatures } from './lib/core-instance.mjs'

const workDir = resolve(repoRoot, 'artifacts', 'rsi', 'm01-s03')
const baselineSource = resolve(workDir, 'baseline-src')
const baselineBinary = resolve(workDir, 'baseline', 'AmitiaCore')
const currentBinary = resolve(workDir, 'current', 'AmitiaCore')
const runsDir = resolve(workDir, 'runs')
const outcomePath = resolve(repoRoot, 'artifacts', 'rsi', 'm01-s03-featuregate-drill.json')

function check(checks, id, ok, detail) {
  checks.push({ id, ok: Boolean(ok), detail: ok ? null : detail })
}

function resetRunRoot(name) {
  const root = resolve(runsDir, name)
  rmSync(root, { recursive: true, force: true })
  mkdirSync(root, { recursive: true })
  return root
}

async function runInstance(name, binary, flags, options = {}) {
  const root = prepareInstanceRoot(resetRunRoot(name), flags)
  return runCoreInstance({ name, binary, root, flags, waitForTicks: options.waitForTicks })
}

export async function runDrill() {
  mkdirSync(runsDir, { recursive: true })
  rmSync(baselineSource, { recursive: true, force: true })
  buildCoreBinary(baselineBinary, buildBaselineSource(baselineSource))
  buildCoreBinary(currentBinary, resolve(repoRoot, 'backend'))

  const baselineOff = await runInstance('baseline-off', baselineBinary, { 'runtime-canary': false, 'route-inventory': false })
  const currentOff = await runInstance('current-off', currentBinary, { 'runtime-canary': false, 'route-inventory': false })
  const currentOffRestart = await runInstance('current-off-restart', currentBinary, { 'runtime-canary': false, 'route-inventory': false })
  const currentOn = await runInstance('current-on', currentBinary, { 'runtime-canary': true, 'route-inventory': true }, { waitForTicks: true })

  const offDelta = diffRouteSets(baselineOff.routes, currentOff.routes)
  const onDelta = diffRouteSets(currentOff.routes, currentOn.routes)
  const expectedAdded = [
    { method: 'GET', path: '/api/features' },
    { method: 'GET', path: '/api/features/routes' }
  ]
  const addedKeys = onDelta.added.map((route) => `${route.method} ${route.path}`).sort()
  const expectedKeys = expectedAdded.map((route) => `${route.method} ${route.path}`).sort()

  const checks = []
  check(checks, 'OFF_ROUTES_UNCHANGED', offDelta.added.length === 0 && offDelta.removed.length === 0 &&
    baselineOff.routeDigest === currentOff.routeDigest,
  `baseline=${baselineOff.routeCount}/${baselineOff.routeDigest.slice(0, 12)} current=${currentOff.routeCount}/${currentOff.routeDigest.slice(0, 12)} added=${JSON.stringify(offDelta.added)} removed=${JSON.stringify(offDelta.removed)}`)
  check(checks, 'OFF_RESTART_STABLE', currentOff.routeDigest === currentOffRestart.routeDigest,
    `first=${currentOff.routeDigest.slice(0, 12)} restart=${currentOffRestart.routeDigest.slice(0, 12)}`)
  check(checks, 'OFF_NO_FEATURE_ROUTE', currentOff.featureStatus === 404, `status=${currentOff.featureStatus}`)
  check(checks, 'OFF_NO_PROVIDER', currentOff.canaryProviders.length === 0, JSON.stringify(currentOff.canaryProviders))
  check(checks, 'OFF_NO_SUBSCRIBER', currentOff.subscriberRegistered === false && currentOff.disabledMarker === true,
    `subscriber=${currentOff.subscriberRegistered} marker=${currentOff.disabledMarker}`)
  check(checks, 'ON_ADDS_EXACTLY_TWO_ROUTES', JSON.stringify(addedKeys) === JSON.stringify(expectedKeys) && onDelta.removed.length === 0,
    `added=${JSON.stringify(addedKeys)} removed=${JSON.stringify(onDelta.removed)}`)
  check(checks, 'ON_FEATURE_ROUTE_OK', currentOn.featureStatus === 200 &&
    currentOn.featureBody?.data?.snapshot?.features?.every((feature) => feature.enabled === true) === true,
  `status=${currentOn.featureStatus} body=${JSON.stringify(currentOn.featureBody).slice(0, 200)}`)
  check(checks, 'ON_PROVIDER_REGISTERED', currentOn.canaryProviders.length === 1 &&
    currentOn.canaryProviders[0].capabilityId === 'featuregate.canary',
  JSON.stringify(currentOn.canaryProviders))
  check(checks, 'ON_SUBSCRIBER_REGISTERED', currentOn.subscriberRegistered === true && currentOn.enabledMarker === true,
    `subscriber=${currentOn.subscriberRegistered} marker=${currentOn.enabledMarker}`)
  const firstTicks = currentOn.featureBody?.data?.canary?.workerTicks
  const secondTicks = currentOn.featureSecondBody?.data?.canary?.workerTicks
  check(checks, 'ON_WORKER_TICKS_ADVANCE', Number.isInteger(firstTicks) && Number.isInteger(secondTicks) && secondTicks > firstTicks,
    `first=${firstTicks} second=${secondTicks}`)
  check(checks, 'ON_ROUTE_EXPORT_DIGEST', currentOn.featureBody?.data?.snapshot?.features?.some((feature) => feature.name === 'route-inventory' && feature.enabled),
    JSON.stringify(currentOn.featureBody?.data?.snapshot ?? null))

  const summary = {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
    baselineSource: 'git archive HEAD backend',
    binaries: {
      baseline: { sha256: createHash('sha256').update(readFileSync(baselineBinary)).digest('hex') },
      current: { sha256: createHash('sha256').update(readFileSync(currentBinary)).digest('hex') }
    },
    instances: [baselineOff, currentOff, currentOffRestart, currentOn].map((instance) => ({
      name: instance.name,
      routeCount: instance.routeCount,
      routeDigest: instance.routeDigest,
      featureStatus: instance.featureStatus,
      featureSecondStatus: instance.featureSecondStatus,
      canaryProviders: instance.canaryProviders,
      subscriberRegistered: instance.subscriberRegistered,
      disabledMarker: instance.disabledMarker,
      enabledMarker: instance.enabledMarker,
      canary: instance.featureBody?.data?.canary ?? null,
      canaryAfterTick: instance.featureSecondBody?.data?.canary ?? null
    })),
    offDelta,
    onDelta,
    checks,
    ok: checks.every((item) => item.ok)
  }
  writeFileSync(outcomePath, `${JSON.stringify(summary, null, 2)}\n`, 'utf8')
  return summary
}

async function main() {
  try {
    const summary = await runDrill()
    for (const item of summary.checks) {
      process.stdout.write(`${item.ok ? 'PASS' : 'FAIL'} ${item.id}${item.detail ? ` :: ${item.detail}` : ''}\n`)
    }
    process.stdout.write(`evidence=${outcomePath}\n`)
    if (!summary.ok) process.exit(12)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(12)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await main()
}
