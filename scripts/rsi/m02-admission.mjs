import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_DIR = 'backend/internal/extension/kernel/plugincontract'
export const OUT_DIR = 'artifacts/rsi/m02-admission'
export const ADMISSION_PATH = `${OUT_DIR}/M02-ADMISSION.json`
export const ADMISSION_MARKDOWN_PATH = `${OUT_DIR}/M02-ADMISSION.md`
export const MANIFEST_PATH = `${OUT_DIR}/m02-admission-manifest.json`

export const UNWIRED_SYMBOLS = [
  'SelectByPermission',
  'HasPermission',
  'CatalogRevision',
  'DiffCatalogs',
  'DetectCatalogConflicts',
  'ManifestDescriptorCatalog',
  'ManifestCapabilityDescriptorCatalog',
  'ManifestSkillDescriptorCatalog',
  'ManifestUIProviderDescriptorCatalog',
  'DefaultCoreDescriptorCatalog',
  'TargetProfile',
  'RestoreDescriptorIndex',
  'ColdStartDigest',
  'MarshalColdStart',
  'FilterProviderBindings',
  'BindingFilterResult',
  'BindingRejection',
  'LoadCoreCallSources',
  'BuildCoreCallInventory',
  'CoreCallInventory',
  'CoreCallRow',
  'RouteRef',
  'FunctionSurface',
  'LoadUIHostCalls',
  'BuildUIHostCoverage',
  'UIHostCoverage',
  'UIHostCall',
  'BindingSnapshotStore',
  'BindingResolution',
  'BindingHandle',
  'BindingNotFoundError',
  'BindingScopeOrder',
  'BindingVersion',
  'BindingCommitConflictError',
  'ErrBindingCommitConflict',
  'RouteMode',
  'RoutePolicy',
  'RouteDecision',
  'RouteSkip',
  'ErrRoutePolicyInvalid',
  'ErrRouteModeUnsatisfied',
  'ErrRoutePinConflict',
  'SelectRoute',
  'RouteFailureClass',
  'RouteFailureError',
  'CoreImplementationIndex',
  'NewCoreImplementationIndex',
  'ProviderHealth',
  'SelectRouteChecked',
  'CheckRouteAvailability',
  'ErrRouteNoBuiltin',
  'ErrRouteUnavailable',
  'RouteOperation',
  'RouteOutcome',
  'RouteFallbackAction',
  'RouteFallbackDecision',
  'RouteFallbackInvalidError',
  'ErrRouteFallbackInvalid',
  'DecideRouteFallback',
  'ScopeDimension',
  'ScopeClaims',
  'ScopeRequirement',
  'ProviderScopeContract',
  'ScopeMatch',
  'ScopeProviderAvailability',
  'ScopeContractInvalidError',
  'ScopeProviderUnavailableError',
  'ScopeNoMatchError',
  'ErrScopeContractInvalid',
  'ErrScopeProviderUnavailable',
  'ErrScopeNoMatch',
  'SelectScopedProvider',
  'ProviderNotSelectedError',
  'ServiceGraphInvalidError',
  'ErrProviderNotSelected',
  'DescriptorCatalog',
  'DescriptorEntry',
  'CoreDescriptorCatalog'
]

export const ROUTE_CACHE_PACKAGE_DIR = 'backend/internal/extension/kernel/routecache/'
export const INVOCATION_PACKAGE_DIR = 'backend/internal/execution/'
export const INVOCATION_SYMBOLS = [
  'InvocationTrace',
  'InvocationContract',
  'NewInvocationContract',
  'InvocationOutcome',
  'InvocationOutcomeSuccess',
  'InvocationOutcomeFailed',
  'InvocationOutcomeUnknown',
  'InvocationContractInvalidError',
  'UnknownInvocationOutcomeError',
  'SideEffectClass',
  'SideEffectClassNone',
  'SideEffectClassRead',
  'SideEffectClassWrite',
  'SideEffectClassExternal',
  'ErrInvocationContractInvalid',
  'ErrInvocationOutcomeUnknown',
  'ErrInvocationTransportLost',
  'ClassifyInvocationOutcome',
  'IsAmbiguousInvocationFailure'
]
export const INVOCATION_BRIDGE_SITES = [
  'backend/internal/extension/kernel/host_api/invocation_bridge.go',
  'backend/internal/extension/kernel/capability/provider_invocation.go',
  'backend/internal/devicemesh/executionjournal/ledger.go',
  'backend/internal/devicemesh/executionjournal/ledger_test.go',
  'backend/internal/extension/kernel/plugincontract/route_read_fallback.go',
  'backend/internal/extension/kernel/plugincontract/route_read_fallback_test.go',
  'backend/internal/devicemesh/executionjournal/effect_token_test.go',
  'backend/internal/devicemesh/executionjournal/reconciliation.go',
  'backend/internal/devicemesh/executionjournal/reconciliation_test.go',
  'backend/internal/devicemesh/executionjournal/run_guard.go',
  'backend/internal/devicemesh/executionjournal/fault_matrix_test.go'
]
export const LEDGER_PACKAGE_DIR = 'backend/internal/devicemesh/executionjournal/'
export const LEDGER_SYMBOLS = [
  'InvocationLedger',
  'NewLedger',
  'LedgerSchema',
  'LedgerScope',
  'LedgerRequest',
  'LedgerEntry',
  'EffectState',
  'EffectPending',
  'EffectCommitted',
  'EffectRejected',
  'EffectUnknown',
  'ErrLedgerKeyInvalid',
  'ErrLedgerPayloadConflict',
  'ErrLedgerBindingFrozen',
  'ErrLedgerEffectUnknown',
  'ErrLedgerRejected'
]
export const EFFECT_TOKEN_SYMBOLS = [
  'EffectToken',
  'EffectRecord',
  'EffectAuthority',
  'EffectResolution',
  'EffectResolvedCommitted',
  'EffectResolvedAbsent',
  'EffectResolvedRejected',
  'EffectResolvedUndetermined',
  'Reconciliation',
  'EffectTokenSchema',
  'ErrEffectTokenInvalid',
  'ErrEffectRecordConflict',
  'BusinessKey',
  'IssueEffectToken',
  'VerifyEffectToken',
  'Reconcile',
  'CommitWithEffect'
]
export const RECONCILIATION_SYMBOLS = [
  'ReconciliationSchema',
  'ReconciliationState',
  'ReconciliationAttempting',
  'ReconciliationNeedsCheck',
  'ReconciliationDelivered',
  'ReconciliationNotSent',
  'ReconciliationVerdict',
  'VerdictDelivered',
  'VerdictNotSent',
  'IrreversibleRequest',
  'ReconciliationCase',
  'ErrReconciliationKeyInvalid',
  'ErrReconciliationPending',
  'ErrReconciliationDelivered',
  'ErrReconciliationResolved',
  'ErrReconciliationVerdictInvalid',
  'ErrReconciliationNotFound',
  'ReserveIrreversible',
  'DispatchIrreversible',
  'ResolveReconciliation',
  'RecoverInterrupted',
  'OpenReconciliations'
]
export const TRUSTED_SERVICE_PACKAGE_DIR = 'backend/internal/extension/kernel/trusted_service/'
export const SCOPED_BREAKER_SYMBOLS = [
  'BreakerKey',
  'ErrBreakerKeyInvalid',
  'ErrCircuitHalfOpenExhausted',
  'ScopedBreakers',
  'NewScopedBreakers',
  'BreakerSnapshot'
]
export const EXECUTIONJOURNAL_PACKAGE_DIR = 'backend/internal/devicemesh/executionjournal/'
export const FAULT_MATRIX_SYMBOLS = ['RunPanicError']
export const READ_FALLBACK_SYMBOLS = [
  'InvokeReadWithFallback',
  'ReadFallbackRequest',
  'ReadFallbackCandidate',
  'ReadFallbackOptions',
  'ReadFallbackAttempt',
  'ReadFallbackResult',
  'ReadFallbackInvalidError',
  'ReadFallbackPanicError',
  'ReadFallbackExhaustedError',
  'ErrReadFallbackInvalid',
  'ErrReadFallbackMutating',
  'ErrReadFallbackPermissionDenied',
  'ErrReadFallbackExhausted'
]
export const ROUTE_CACHE_ENTRY_FILE = `${ROUTE_CACHE_PACKAGE_DIR}route_cache.go`
export const ROUTE_E2E_PACKAGE_DIR = 'backend/internal/extension/kernel/routee2e/'
export const ROUTE_CONFORMANCE_PACKAGE_DIR = 'backend/internal/extension/kernel/routeconformance/'

export const WIRED_BY_ROUTE_CACHE = [
  'Binding',
  'BindingInvalidError',
  'BindingKey',
  'BindingRequest',
  'BindingScope',
  'BindingSnapshot',
  'ErrServiceGraphInvalid',
  'NewBindingSnapshot',
  'NewBindingSnapshotStore',
  'ProviderCandidates',
  'SelectProviderCandidates'
]

export const ROUTE_CACHE_PRODUCTION_SYMBOLS = [
  'BindingInvalidError',
  'BindingRequest',
  'BindingSnapshot',
  'ProviderCandidates',
  'SelectProviderCandidates'
]

export const PRESERVED_EVIDENCE = [
  'artifacts/rsi/m01-s06-offstate-db-guard.json',
  'artifacts/rsi/m01-s06-frontend-route-guard.json'
]

export const GATES = [
  { id: 'GO_BUILD', file: 'm02-s08-go-build.log', command: 'go', args: ['build', '-buildvcs=false', './...'], cwd: 'backend' },
  { id: 'GO_TESTS', file: 'm02-s08-go-tests-verbose.log', command: 'go', args: ['test', '-count=1', '-v', './internal/extension/kernel/plugincontract', './internal/extension/kernel/capability'], cwd: 'backend' },
  { id: 'CONTRACT_REGRESSION', file: 'm02-s08-contract-regression.log', command: 'go', args: ['test', '-count=1', '-run', 'TestContractRegression', '-v', './internal/extension/kernel/plugincontract'], cwd: 'backend' },
  { id: 'NODE_SUITE', file: 'm02-s08-node-tests.log', command: 'node', args: ['--test', 'scripts/rsi/__tests__/*.test.mjs'], cwd: '.' },
  { id: 'OFFSTATE_DB_GUARD', file: 'm02-s08-db-guard.log', command: 'node', args: ['scripts/rsi/featuregate-sideeffect-guard.mjs'], cwd: '.' },
  { id: 'FRONTEND_ROUTE_GUARD', file: 'm02-s08-frontend-route-guard.log', command: 'node', args: ['scripts/rsi/frontend-route-guard.mjs'], cwd: '.' },
  { id: 'CONTRACT_INVENTORY', file: 'm02-s08-inventory.log', command: 'node', args: ['scripts/rsi/m02-contract-inventory.mjs'], cwd: '.' },
  { id: 'TARGET_PROFILE', file: 'm02-s11-target-profile.log', command: 'node', args: ['scripts/rsi/m02-target-profile.mjs'], cwd: '.' },
  { id: 'COLD_START', file: 'm02-s12-cold-start.log', command: 'go', args: ['test', '-count=1', '-run', 'TestColdStart', '-v', './internal/extension/kernel/plugincontract'], cwd: 'backend' },
  { id: 'CORE_INVENTORY', file: 'm03-s01-core-inventory.log', command: 'node', args: ['scripts/rsi/m03-core-inventory.mjs'], cwd: '.' },
  { id: 'BINDING_SNAPSHOT', file: 'm03-s02-binding.log', command: 'node', args: ['scripts/rsi/m03-binding-snapshot.mjs'], cwd: '.' },
  { id: 'PROVIDER_SELECTION', file: 'm03-s03-provider.log', command: 'node', args: ['scripts/rsi/m03-provider-selection.mjs'], cwd: '.' },
  { id: 'CORE_ADAPTER', file: 'm03-s04-core-adapter.log', command: 'node', args: ['scripts/rsi/m03-core-adapter.mjs'], cwd: '.' },
  { id: 'PLUGIN_ADAPTER', file: 'm03-s05-plugin-adapter.log', command: 'node', args: ['scripts/rsi/m03-plugin-adapter.mjs'], cwd: '.' },
  { id: 'ROUTE_CACHE', file: 'm03-s06-route-cache.log', command: 'node', args: ['scripts/rsi/m03-route-cache.mjs'], cwd: '.' },
  { id: 'ROUTE_E2E', file: 'm03-s07-route-e2e.log', command: 'node', args: ['scripts/rsi/m03-route-e2e.mjs'], cwd: '.' },
  { id: 'ROUTE_RELEASE', file: 'm03-s08-route-release.log', command: 'node', args: ['scripts/rsi/m03-route-release.mjs'], cwd: '.' },
  { id: 'BINDING_CAS', file: 'm03-s09-binding-cas.log', command: 'node', args: ['scripts/rsi/m03-binding-cas.mjs'], cwd: '.' },
  { id: 'ROUTE_POLICY', file: 'm03-s10-route-policy.log', command: 'node', args: ['scripts/rsi/m03-route-policy.mjs'], cwd: '.' },
  { id: 'ROUTE_AVAILABILITY', file: 'm03-s11-route-availability.log', command: 'node', args: ['scripts/rsi/m03-route-availability.mjs'], cwd: '.' },
  { id: 'ROUTE_FALLBACK', file: 'm03-s12-route-fallback.log', command: 'node', args: ['scripts/rsi/m03-route-fallback.mjs'], cwd: '.' },
  { id: 'ROUTE_SCOPE', file: 'm03-s13-route-scope.log', command: 'node', args: ['scripts/rsi/m03-route-scope.mjs'], cwd: '.' },
  { id: 'INVOCATION_CONTRACT', file: 'm04-s01-invocation-contract.log', command: 'node', args: ['scripts/rsi/m04-invocation-contract.mjs'], cwd: '.' },
  { id: 'LEDGER_IDEMPOTENCY', file: 'm04-s02-ledger.log', command: 'node', args: ['scripts/rsi/m04-ledger.mjs'], cwd: '.' },
  { id: 'READ_FALLBACK', file: 'm04-s03-read-fallback.log', command: 'node', args: ['scripts/rsi/m04-read-fallback.mjs'], cwd: '.' },
  { id: 'EFFECT_TOKEN', file: 'm04-s04-effect-token.log', command: 'node', args: ['scripts/rsi/m04-effect-token.mjs'], cwd: '.' },
  { id: 'RECONCILIATION', file: 'm04-s05-reconciliation.log', command: 'node', args: ['scripts/rsi/m04-reconciliation.mjs'], cwd: '.' },
  { id: 'SCOPED_BREAKER', file: 'm04-s06-scoped-breaker.log', command: 'node', args: ['scripts/rsi/m04-scoped-breaker.mjs'], cwd: '.' },
  { id: 'FAULT_MATRIX', file: 'm04-s07-fault-matrix.log', command: 'node', args: ['scripts/rsi/m04-fault-matrix.mjs'], cwd: '.' }
]

export const POST_RESTORE_GATE = { id: 'M01_ADMISSION_VERIFY', file: 'm02-s08-m01-verify.log', command: 'node', args: ['scripts/rsi/m01-audit.mjs', '--verify'], cwd: '.' }

export function sha256File(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

export function relativeFiles(dir, extension) {
  const out = []
  const walk = (current) => {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const full = join(current, entry.name)
      if (entry.isDirectory()) {
        walk(full)
      } else if (entry.isFile() && entry.name.endsWith(extension)) {
        out.push(full)
      }
    }
  }
  walk(dir)
  return out.sort()
}

export function packageSources(root) {
  const dir = resolve(root, PACKAGE_DIR)
  const sources = relativeFiles(dir, '.go')
  const testdataDir = join(dir, 'testdata')
  if (existsSync(testdataDir)) sources.push(...relativeFiles(testdataDir, '.json'))
  return sources.sort().map((path) => ({
    path: path.slice(resolve(root).length + 1),
    sha256: sha256File(path),
    bytes: statSync(path).size
  }))
}

export function qualifiedRef(symbol) {
  return new RegExp(`plugincontract\\.${symbol}\\b`)
}

export function scanUnwiredSymbols(root, symbols = UNWIRED_SYMBOLS) {
  const backend = resolve(root, 'backend')
  const packagePrefix = resolve(root, PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    if (path.startsWith(packagePrefix)) continue
    if (path.endsWith('_test.go')) continue
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (qualifiedRef(symbol).test(source)) {
        hits.push({ path: path.slice(resolve(root).length + 1), symbol })
      }
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function scanInvocationSymbols(root, symbols = INVOCATION_SYMBOLS) {
  const backend = resolve(root, 'backend')
  const packagePrefix = resolve(root, INVOCATION_PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    if (path.startsWith(packagePrefix)) continue
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (!new RegExp(`execution\\.${symbol}\\b`).test(source)) continue
      hits.push({ path: path.slice(resolve(root).length + 1), symbol })
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function invocationBridgeProblems(root) {
  const problems = []
  const hits = scanInvocationSymbols(root)
  for (const hit of hits) {
    if (INVOCATION_BRIDGE_SITES.includes(hit.path)) continue
    problems.push(`invocation contract reached an unfrozen site: ${hit.path} -> ${hit.symbol}`)
  }
  for (const site of INVOCATION_BRIDGE_SITES) {
    if (!hits.some((hit) => hit.path === site)) problems.push(`${site} does not bridge the invocation contract`)
  }
  return problems
}

export function scanLedgerSymbols(root, symbols = LEDGER_SYMBOLS) {
  const backend = resolve(root, 'backend')
  const packagePrefix = resolve(root, LEDGER_PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    if (path.startsWith(packagePrefix)) continue
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (!new RegExp(`executionjournal\\.${symbol}\\b`).test(source)) continue
      hits.push({ path: path.slice(resolve(root).length + 1), symbol })
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function ledgerWiringProblems(root) {
  return scanLedgerSymbols(root).map((hit) => `invocation ledger reached a site before its wiring step: ${hit.path} -> ${hit.symbol}`)
}

export function scanReadFallbackSymbols(root, symbols = READ_FALLBACK_SYMBOLS) {
  const backend = resolve(root, 'backend')
  const packagePrefix = resolve(root, PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    if (path.startsWith(packagePrefix)) continue
    if (path.endsWith('_test.go')) continue
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (!new RegExp(`plugincontract\\.${symbol}\\b`).test(source)) continue
      hits.push({ path: path.slice(resolve(root).length + 1), symbol })
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function readFallbackWiringProblems(root) {
  return scanReadFallbackSymbols(root).map((hit) => `read fallback reached a site before its wiring step: ${hit.path} -> ${hit.symbol}`)
}

export function scanEffectTokenSymbols(root, symbols = EFFECT_TOKEN_SYMBOLS) {
  const backend = resolve(root, 'backend')
  const packagePrefix = resolve(root, LEDGER_PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    if (path.startsWith(packagePrefix)) continue
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (!new RegExp(`executionjournal\\.${symbol}\\b`).test(source)) continue
      hits.push({ path: path.slice(resolve(root).length + 1), symbol })
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function effectTokenWiringProblems(root) {
  return scanEffectTokenSymbols(root).map((hit) => `effect token reached a site before its wiring step: ${hit.path} -> ${hit.symbol}`)
}

export function scanReconciliationSymbols(root, symbols = RECONCILIATION_SYMBOLS) {
  const backend = resolve(root, 'backend')
  const packagePrefix = resolve(root, LEDGER_PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    if (path.startsWith(packagePrefix)) continue
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (!new RegExp(`executionjournal\\.${symbol}\\b`).test(source)) continue
      hits.push({ path: path.slice(resolve(root).length + 1), symbol })
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function reconciliationWiringProblems(root) {
  return scanReconciliationSymbols(root).map((hit) => `irreversible reconciliation reached a site before its wiring step: ${hit.path} -> ${hit.symbol}`)
}

export function scanScopedBreakerSymbols(root, symbols = SCOPED_BREAKER_SYMBOLS) {
  const backend = resolve(root, 'backend')
  const packagePrefix = resolve(root, TRUSTED_SERVICE_PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    if (path.startsWith(packagePrefix)) continue
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (!new RegExp(`trusted_service\\.${symbol}\\b`).test(source)) continue
      hits.push({ path: path.slice(resolve(root).length + 1), symbol })
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function scopedBreakerWiringProblems(root) {
  return scanScopedBreakerSymbols(root).map((hit) => `scoped breaker reached a site before its wiring step: ${hit.path} -> ${hit.symbol}`)
}

export function scanFaultMatrixSymbols(root, symbols = FAULT_MATRIX_SYMBOLS) {
  const backend = resolve(root, 'backend')
  const packagePrefix = resolve(root, EXECUTIONJOURNAL_PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    if (path.startsWith(packagePrefix)) continue
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (!new RegExp(`executionjournal\\.${symbol}\\b`).test(source)) continue
      hits.push({ path: path.slice(resolve(root).length + 1), symbol })
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function faultMatrixWiringProblems(root) {
  return scanFaultMatrixSymbols(root).map((hit) => `fault matrix guard reached a site before its wiring step: ${hit.path} -> ${hit.symbol}`)
}

export function scanWiredSymbols(root, symbols = WIRED_BY_ROUTE_CACHE) {
  const backend = resolve(root, 'backend')
  const cachePrefix = resolve(root, ROUTE_CACHE_PACKAGE_DIR) + '/'
  const e2ePrefix = resolve(root, ROUTE_E2E_PACKAGE_DIR) + '/'
  const conformancePrefix = resolve(root, ROUTE_CONFORMANCE_PACKAGE_DIR) + '/'
  const hits = []
  for (const path of relativeFiles(backend, '.go')) {
    const source = readFileSync(path, 'utf8')
    for (const symbol of symbols) {
      if (!qualifiedRef(symbol).test(source)) continue
      hits.push({
        path: path.slice(resolve(root).length + 1),
        symbol,
        inRouteCache: path.startsWith(cachePrefix),
        inRouteE2E: path.startsWith(e2ePrefix),
        inRouteConformance: path.startsWith(conformancePrefix)
      })
    }
  }
  return hits.sort((left, right) => (left.path === right.path ? (left.symbol < right.symbol ? -1 : 1) : left.path < right.path ? -1 : 1))
}

export function routeCacheWiringProblems(root) {
  const problems = []
  const hits = scanWiredSymbols(root)
  for (const hit of hits) {
    if (hit.inRouteCache || hit.inRouteE2E || hit.inRouteConformance) continue
    problems.push(`binding API used outside the routing packages: ${hit.path} -> ${hit.symbol}`)
  }
  const production = hits.filter((hit) => hit.inRouteCache && hit.path === ROUTE_CACHE_ENTRY_FILE)
  for (const symbol of ROUTE_CACHE_PRODUCTION_SYMBOLS) {
    if (!production.some((hit) => hit.symbol === symbol)) problems.push(`route_cache.go does not wire plugincontract.${symbol}`)
  }
  return problems
}

export function runGate(gate, root, logPath) {
  const executable = gate.command === 'node' ? process.execPath : gate.command
  const result = spawnSync(executable, gate.args, {
    cwd: resolve(root, gate.cwd),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 600000
  })
  const body = `${result.stdout ?? ''}${result.stderr ?? ''}`
  writeFileSync(logPath, body, 'utf8')
  return {
    id: gate.id,
    command: [gate.command, ...gate.args].join(' '),
    exitCode: result.status === null ? 1 : result.status,
    log: logPath.slice(resolve(root).length + 1),
    logSha256: sha256File(logPath)
  }
}

export function computeVerdict(checks) {
  const blockers = checks.filter((check) => !check.ok).map((check) => check.id)
  return { verdict: blockers.length === 0 ? 'READY_FOR_APPROVAL' : 'PENDING_APPROVAL', blockers }
}

export function verifyManifest(root, manifestPath) {
  const manifest = JSON.parse(readFileSync(resolve(root, manifestPath), 'utf8'))
  const mismatches = []
  for (const file of manifest.files) {
    const target = resolve(root, file.path)
    if (!existsSync(target)) {
      mismatches.push(`${file.path} missing`)
      continue
    }
    const actual = sha256File(target)
    if (actual !== file.sha256) mismatches.push(`${file.path} sha ${actual} != ${file.sha256}`)
  }
  return { ok: mismatches.length === 0, mismatches, files: manifest.files.length }
}

export function renderMarkdown(admission) {
  const lines = []
  lines.push('# M02 独立目录功能接入包', '')
  lines.push(`- 步骤：${admission.stepId}`)
  lines.push(`- 源码版本：\`${admission.sourceRevision}\``)
  lines.push(`- 结论：\`${admission.verdict}\``)
  lines.push(`- 生成时间：${admission.generatedAt}`)
  lines.push('', '## 门禁', '')
  lines.push('| 门禁 | 命令 | 退出码 | 日志 sha256 |')
  lines.push('|---|---|---|---|')
  for (const gate of admission.gates) {
    lines.push(`| ${gate.id} | \`${gate.command}\` | ${gate.exitCode} | \`${gate.logSha256.slice(0, 8)}\` |`)
  }
  lines.push('', '## 断言', '')
  for (const check of admission.checks) {
    lines.push(`- ${check.ok ? 'PASS' : 'FAIL'} ${check.id} :: ${check.detail}`)
  }
  lines.push('', '## 纳入文件', '')
  for (const file of admission.files) {
    lines.push(`- \`${file.path}\` \`${file.sha256.slice(0, 8)}\` (${file.bytes} B)`)
  }
  if (admission.blockers.length > 0) {
    lines.push('', '## 阻断项', '')
    for (const blocker of admission.blockers) lines.push(`- ${blocker}`)
  }
  return `${lines.join('\n')}\n`
}

export function buildAdmission(root) {
  mkdirSync(resolve(root, OUT_DIR), { recursive: true })
  const preserved = PRESERVED_EVIDENCE.filter((path) => existsSync(resolve(root, path))).map((path) => ({
    path,
    sha256: sha256File(resolve(root, path)),
    snapshot: readFileSync(resolve(root, path))
  }))
  let gates
  try {
    gates = GATES.map((gate) => {
      const logPath = resolve(root, 'artifacts', 'rsi', gate.file)
      return runGate(gate, root, logPath)
    })
  } finally {
    for (const entry of preserved) writeFileSync(resolve(root, entry.path), entry.snapshot)
  }
  gates.push(runGate(POST_RESTORE_GATE, root, resolve(root, 'artifacts', 'rsi', POST_RESTORE_GATE.file)))
  const preservedEvidence = preserved.map((entry) => ({
    path: entry.path,
    sha256: entry.sha256,
    afterSha256: sha256File(resolve(root, entry.path))
  }))
  const preservedOk = preservedEvidence.every((entry) => entry.sha256 === entry.afterSha256)
  const gateById = Object.fromEntries(gates.map((gate) => [gate.id, gate]))
  const contractLog = readFileSync(resolve(root, 'artifacts', 'rsi', 'm02-s08-contract-regression.log'), 'utf8')
  const pinnedCases = (contractLog.match(/--- PASS: TestContractRegression/g) ?? []).length
  const unwired = scanUnwiredSymbols(root)
  const wiringProblems = routeCacheWiringProblems(root)
  const bridgeProblems = invocationBridgeProblems(root)
  const ledgerProblems = ledgerWiringProblems(root)
  const readFallbackProblems = readFallbackWiringProblems(root)
  const effectTokenProblems = effectTokenWiringProblems(root)
  const reconciliationProblems = reconciliationWiringProblems(root)
  const scopedBreakerProblems = scopedBreakerWiringProblems(root)
  const faultMatrixProblems = faultMatrixWiringProblems(root)
  const checks = [
    { id: 'GO_BUILD', ok: gateById.GO_BUILD.exitCode === 0, detail: `exit=${gateById.GO_BUILD.exitCode}` },
    { id: 'GO_TESTS', ok: gateById.GO_TESTS.exitCode === 0, detail: `exit=${gateById.GO_TESTS.exitCode}` },
    { id: 'CONTRACT_REGRESSION', ok: gateById.CONTRACT_REGRESSION.exitCode === 0 && pinnedCases === 6, detail: `exit=${gateById.CONTRACT_REGRESSION.exitCode} pinnedCases=${pinnedCases}` },
    { id: 'NODE_SUITE', ok: gateById.NODE_SUITE.exitCode === 0, detail: `exit=${gateById.NODE_SUITE.exitCode}` },
    { id: 'OFFSTATE_DB_GUARD', ok: gateById.OFFSTATE_DB_GUARD.exitCode === 0, detail: `exit=${gateById.OFFSTATE_DB_GUARD.exitCode}` },
    { id: 'FRONTEND_ROUTE_GUARD', ok: gateById.FRONTEND_ROUTE_GUARD.exitCode === 0, detail: `exit=${gateById.FRONTEND_ROUTE_GUARD.exitCode}` },
    { id: 'CONTRACT_INVENTORY', ok: gateById.CONTRACT_INVENTORY.exitCode === 0, detail: `exit=${gateById.CONTRACT_INVENTORY.exitCode}` },
    { id: 'TARGET_PROFILE', ok: gateById.TARGET_PROFILE.exitCode === 0, detail: `exit=${gateById.TARGET_PROFILE.exitCode}` },
    { id: 'COLD_START', ok: gateById.COLD_START.exitCode === 0, detail: `exit=${gateById.COLD_START.exitCode}` },
    { id: 'CORE_INVENTORY', ok: gateById.CORE_INVENTORY.exitCode === 0, detail: `exit=${gateById.CORE_INVENTORY.exitCode}` },
    { id: 'BINDING_SNAPSHOT', ok: gateById.BINDING_SNAPSHOT.exitCode === 0, detail: `exit=${gateById.BINDING_SNAPSHOT.exitCode}` },
    { id: 'PROVIDER_SELECTION', ok: gateById.PROVIDER_SELECTION.exitCode === 0, detail: `exit=${gateById.PROVIDER_SELECTION.exitCode}` },
    { id: 'CORE_ADAPTER', ok: gateById.CORE_ADAPTER.exitCode === 0, detail: `exit=${gateById.CORE_ADAPTER.exitCode}` },
    { id: 'PLUGIN_ADAPTER', ok: gateById.PLUGIN_ADAPTER.exitCode === 0, detail: `exit=${gateById.PLUGIN_ADAPTER.exitCode}` },
    { id: 'ROUTE_CACHE', ok: gateById.ROUTE_CACHE.exitCode === 0, detail: `exit=${gateById.ROUTE_CACHE.exitCode}` },
    { id: 'ROUTE_E2E', ok: gateById.ROUTE_E2E.exitCode === 0, detail: `exit=${gateById.ROUTE_E2E.exitCode}` },
    { id: 'ROUTE_RELEASE', ok: gateById.ROUTE_RELEASE.exitCode === 0, detail: `exit=${gateById.ROUTE_RELEASE.exitCode}` },
    { id: 'BINDING_CAS', ok: gateById.BINDING_CAS.exitCode === 0, detail: `exit=${gateById.BINDING_CAS.exitCode}` },
    { id: 'ROUTE_POLICY', ok: gateById.ROUTE_POLICY.exitCode === 0, detail: `exit=${gateById.ROUTE_POLICY.exitCode}` },
    { id: 'ROUTE_AVAILABILITY', ok: gateById.ROUTE_AVAILABILITY.exitCode === 0, detail: `exit=${gateById.ROUTE_AVAILABILITY.exitCode}` },
    { id: 'ROUTE_FALLBACK', ok: gateById.ROUTE_FALLBACK.exitCode === 0, detail: `exit=${gateById.ROUTE_FALLBACK.exitCode}` },
    { id: 'ROUTE_SCOPE', ok: gateById.ROUTE_SCOPE.exitCode === 0, detail: `exit=${gateById.ROUTE_SCOPE.exitCode}` },
    { id: 'INVOCATION_CONTRACT', ok: gateById.INVOCATION_CONTRACT.exitCode === 0, detail: `exit=${gateById.INVOCATION_CONTRACT.exitCode}` },
    { id: 'M01_EVIDENCE_PRESERVED', ok: preservedOk, detail: `files=${preservedEvidence.length}` },
    { id: 'M01_ADMISSION_VERIFY', ok: gateById.M01_ADMISSION_VERIFY.exitCode === 0, detail: `exit=${gateById.M01_ADMISSION_VERIFY.exitCode}` },
    { id: 'CATALOG_API_NOT_WIRED', ok: unwired.length === 0, detail: `qualifiedRefsOutsidePackage=${unwired.length}` },
    { id: 'ROUTE_CACHE_OWNS_BINDING_APIS', ok: wiringProblems.length === 0, detail: `wiredSymbols=${WIRED_BY_ROUTE_CACHE.length} problems=${wiringProblems.length}` },
    { id: 'INVOCATION_BRIDGE_FROZEN', ok: bridgeProblems.length === 0, detail: `sites=${INVOCATION_BRIDGE_SITES.length} problems=${bridgeProblems.length}` },
    { id: 'LEDGER_IDEMPOTENCY', ok: gateById.LEDGER_IDEMPOTENCY.exitCode === 0, detail: `exit=${gateById.LEDGER_IDEMPOTENCY.exitCode}` },
    { id: 'LEDGER_UNWIRED', ok: ledgerProblems.length === 0, detail: `symbols=${LEDGER_SYMBOLS.length} problems=${ledgerProblems.length}` },
    { id: 'READ_FALLBACK', ok: gateById.READ_FALLBACK.exitCode === 0, detail: `exit=${gateById.READ_FALLBACK.exitCode}` },
    { id: 'READ_FALLBACK_UNWIRED', ok: readFallbackProblems.length === 0, detail: `symbols=${READ_FALLBACK_SYMBOLS.length} problems=${readFallbackProblems.length}` },
    { id: 'EFFECT_TOKEN', ok: gateById.EFFECT_TOKEN.exitCode === 0, detail: `exit=${gateById.EFFECT_TOKEN.exitCode}` },
    { id: 'EFFECT_TOKEN_UNWIRED', ok: effectTokenProblems.length === 0, detail: `symbols=${EFFECT_TOKEN_SYMBOLS.length} problems=${effectTokenProblems.length}` },
    { id: 'RECONCILIATION', ok: gateById.RECONCILIATION.exitCode === 0, detail: `exit=${gateById.RECONCILIATION.exitCode}` },
    { id: 'RECONCILIATION_UNWIRED', ok: reconciliationProblems.length === 0, detail: `symbols=${RECONCILIATION_SYMBOLS.length} problems=${reconciliationProblems.length}` },
    { id: 'SCOPED_BREAKER', ok: gateById.SCOPED_BREAKER.exitCode === 0, detail: `exit=${gateById.SCOPED_BREAKER.exitCode}` },
    { id: 'SCOPED_BREAKER_UNWIRED', ok: scopedBreakerProblems.length === 0, detail: `symbols=${SCOPED_BREAKER_SYMBOLS.length} problems=${scopedBreakerProblems.length}` },
    { id: 'FAULT_MATRIX', ok: gateById.FAULT_MATRIX.exitCode === 0, detail: `exit=${gateById.FAULT_MATRIX.exitCode}` },
    { id: 'FAULT_MATRIX_UNWIRED', ok: faultMatrixProblems.length === 0, detail: `symbols=${FAULT_MATRIX_SYMBOLS.length} problems=${faultMatrixProblems.length}` }
  ]
  const { verdict, blockers } = computeVerdict(checks)
  const sources = packageSources(root)
  const admission = {
    schemaVersion: 1,
    stepId: 'M02-S08',
    generatedAt: new Date().toISOString(),
    sourceRevision: readSourceRevision(root),
    verdict,
    blockers,
    counts: { gates: gates.length, checks: checks.length, pinnedCases, unwiredSymbols: unwired.length },
    gates,
    checks,
    unwired,
    preservedEvidence,
    files: sources
  }
  const admissionPath = resolve(root, ADMISSION_PATH)
  writeFileSync(admissionPath, `${JSON.stringify(admission, null, 2)}\n`, 'utf8')
  const markdownPath = resolve(root, ADMISSION_MARKDOWN_PATH)
  writeFileSync(markdownPath, renderMarkdown(admission), 'utf8')
  const evidence = [
    ...sources,
    ...gates.map((gate) => ({ path: gate.log, sha256: gate.logSha256, bytes: statSync(resolve(root, gate.log)).size })),
    { path: ADMISSION_PATH, sha256: sha256File(admissionPath), bytes: statSync(admissionPath).size },
    { path: ADMISSION_MARKDOWN_PATH, sha256: sha256File(markdownPath), bytes: statSync(markdownPath).size }
  ]
  const deduped = new Map()
  for (const file of evidence) if (!deduped.has(file.path)) deduped.set(file.path, file)
  const files = [...deduped.values()].sort((left, right) => (left.path < right.path ? -1 : left.path > right.path ? 1 : 0))
  const manifest = {
    schemaVersion: 1,
    generatedAt: admission.generatedAt,
    sourceRevision: admission.sourceRevision,
    verdict: admission.verdict,
    blockers: admission.blockers,
    files
  }
  writeFileSync(resolve(root, MANIFEST_PATH), `${JSON.stringify(manifest, null, 2)}\n`, 'utf8')
  return { admission, manifest }
}

function readSourceRevision(root) {
  const result = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: resolve(root), env: cleanEnv(), encoding: 'utf8' })
  return (result.stdout ?? '').trim() || 'unknown'
}

function main() {
  const verify = process.argv.includes('--verify')
  try {
    if (verify) {
      const result = verifyManifest(repoRoot, MANIFEST_PATH)
      for (const mismatch of result.mismatches) process.stdout.write(`MISMATCH ${mismatch}\n`)
      process.stdout.write(`verify=${result.ok} manifest=${resolve(repoRoot, MANIFEST_PATH)} files=${result.files}\n`)
      if (!result.ok) process.exit(20)
      return
    }
    const { admission, manifest } = buildAdmission(repoRoot)
    process.stdout.write(`verdict=${admission.verdict}\n`)
    for (const blocker of admission.blockers) process.stdout.write(`BLOCKER ${blocker}\n`)
    process.stdout.write(`admission=${resolve(repoRoot, ADMISSION_PATH)}\nmanifest=${resolve(repoRoot, MANIFEST_PATH)} files=${manifest.files.length}\n`)
    process.stdout.write(`counts=${JSON.stringify(admission.counts)}\n`)
    if (process.argv.includes('--require-ready') && admission.verdict !== 'READY_FOR_APPROVAL') process.exit(22)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(20)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
