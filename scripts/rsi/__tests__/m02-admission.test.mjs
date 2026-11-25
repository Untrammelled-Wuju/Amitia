import test from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { repoRoot } from '../toolchain-preflight.mjs'
import {
  GATES,
  INVOCATION_BRIDGE_SITES,
  INVOCATION_SYMBOLS,
  LEDGER_PACKAGE_DIR,
  LEDGER_SYMBOLS,
  READ_FALLBACK_SYMBOLS,
  EFFECT_TOKEN_SYMBOLS,
  RECONCILIATION_SYMBOLS,
  reconciliationWiringProblems,
  scanReconciliationSymbols,
  SCOPED_BREAKER_SYMBOLS,
  TRUSTED_SERVICE_PACKAGE_DIR,
  scopedBreakerWiringProblems,
  scanScopedBreakerSymbols,
  EXECUTIONJOURNAL_PACKAGE_DIR,
  FAULT_MATRIX_SYMBOLS,
  faultMatrixWiringProblems,
  scanFaultMatrixSymbols,
  ledgerWiringProblems,
  effectTokenWiringProblems,
  scanEffectTokenSymbols,
  readFallbackWiringProblems,
  scanReadFallbackSymbols,
  scanLedgerSymbols,
  PRESERVED_EVIDENCE,
  ROUTE_CACHE_PACKAGE_DIR,
  ROUTE_E2E_PACKAGE_DIR,
  ROUTE_CONFORMANCE_PACKAGE_DIR,
  ROUTE_CACHE_ENTRY_FILE,
  ROUTE_CACHE_PRODUCTION_SYMBOLS,
  UNWIRED_SYMBOLS,
  WIRED_BY_ROUTE_CACHE,
  computeVerdict,
  invocationBridgeProblems,
  packageSources,
  renderMarkdown,
  routeCacheWiringProblems,
  scanInvocationSymbols,
  scanUnwiredSymbols,
  scanWiredSymbols,
  sha256File,
  verifyManifest
} from '../m02-admission.mjs'

test('sha256File 对空文件给出标准空摘要', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-sha-'))
  const path = join(dir, 'empty.txt')
  writeFileSync(path, '', 'utf8')
  assert.equal(sha256File(path), 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855')
})

test('packageSources 按路径排序且每个源文件都是 64 位摘要', () => {
  const sources = packageSources(repoRoot)
  assert.ok(sources.length >= 10, `expected at least 10 source files, got ${sources.length}`)
  const paths = sources.map((entry) => entry.path)
  assert.deepEqual(paths, [...paths].sort())
  for (const entry of sources) {
    assert.match(entry.path, /^backend\/internal\/extension\/kernel\/plugincontract\/([a-z_]+\.go|testdata\/[a-z-]+\.json)$/)
    assert.match(entry.sha256, /^[0-9a-f]{64}$/)
    assert.ok(entry.bytes > 0)
  }
  assert.ok(paths.some((path) => path.endsWith('/testdata/id-stability.json')), 'golden snapshot must be hashed')
})

test('真实仓库里目录 API 在包外零引用', () => {
  assert.deepEqual(scanUnwiredSymbols(repoRoot), [])
})

test('包外引用能被扫描抓出而包内引用被忽略', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-scan-'))
  const inside = join(dir, 'backend', 'internal', 'extension', 'kernel', 'plugincontract')
  mkdirSync(inside, { recursive: true })
  writeFileSync(join(inside, 'self.go'), 'package plugincontract\n\nfunc use() { _ = DescriptorCatalog{} }\n', 'utf8')
  const outside = join(dir, 'backend', 'internal', 'extension', 'other')
  mkdirSync(outside, { recursive: true })
  writeFileSync(join(outside, 'consumer.go'), 'package other\n\nvar _ = plugincontract.DiffCatalogs\n', 'utf8')
  writeFileSync(join(outside, 'harmless.go'), 'package other\n\nvar _ = plugincontract\n', 'utf8')
  const hits = scanUnwiredSymbols(dir)
  assert.deepEqual(hits, [{ path: 'backend/internal/extension/other/consumer.go', symbol: 'DiffCatalogs' }])
})

test('computeVerdict 全通过时给出 READY_FOR_APPROVAL', () => {
  const result = computeVerdict([{ id: 'A', ok: true }, { id: 'B', ok: true }])
  assert.equal(result.verdict, 'READY_FOR_APPROVAL')
  assert.deepEqual(result.blockers, [])
})

test('computeVerdict 单点失败时给出 PENDING_APPROVAL 并点名阻断项', () => {
  const result = computeVerdict([{ id: 'A', ok: true }, { id: 'CATALOG_API_NOT_WIRED', ok: false }])
  assert.equal(result.verdict, 'PENDING_APPROVAL')
  assert.deepEqual(result.blockers, ['CATALOG_API_NOT_WIRED'])
})

test('verifyManifest 报告缺失文件与摘要漂移', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-verify-'))
  mkdirSync(join(dir, 'evidence'), { recursive: true })
  writeFileSync(join(dir, 'evidence', 'kept.log'), 'kept\n', 'utf8')
  writeFileSync(join(dir, 'evidence', 'drifted.log'), 'changed\n', 'utf8')
  const manifest = {
    schemaVersion: 1,
    files: [
      { path: 'evidence/kept.log', sha256: sha256File(join(dir, 'evidence', 'kept.log')) },
      { path: 'evidence/drifted.log', sha256: 'a'.repeat(64) },
      { path: 'evidence/absent.log', sha256: 'b'.repeat(64) }
    ]
  }
  writeFileSync(join(dir, 'manifest.json'), `${JSON.stringify(manifest)}\n`, 'utf8')
  const result = verifyManifest(dir, 'manifest.json')
  assert.equal(result.ok, false)
  assert.equal(result.files, 3)
  assert.equal(result.mismatches.length, 2)
  assert.ok(result.mismatches.includes('evidence/absent.log missing'))
  assert.ok(result.mismatches.some((line) => /^evidence\/drifted\.log sha [0-9a-f]{64} != a{64}$/.test(line)))
})

test('verifyManifest 在摘要全等时通过', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-verify-ok-'))
  writeFileSync(join(dir, 'gate.log'), 'exit=0\n', 'utf8')
  writeFileSync(
    join(dir, 'manifest.json'),
    `${JSON.stringify({ schemaVersion: 1, files: [{ path: 'gate.log', sha256: sha256File(join(dir, 'gate.log')) }] })}\n`,
    'utf8'
  )
  assert.deepEqual(verifyManifest(dir, 'manifest.json'), { ok: true, mismatches: [], files: 1 })
})

test('renderMarkdown 输出结论、门禁表与断言明细', () => {
  const markdown = renderMarkdown({
    stepId: 'M02-S08',
    sourceRevision: 'af1a11f8',
    verdict: 'READY_FOR_APPROVAL',
    generatedAt: '2026-11-22T00:00:00.000Z',
    gates: [{ id: 'GO_BUILD', command: 'go build -buildvcs=false ./...', exitCode: 0, logSha256: 'c'.repeat(64) }],
    checks: [{ id: 'GO_BUILD', ok: true, detail: 'exit=0' }],
    files: [{ path: 'backend/x.go', sha256: 'd'.repeat(64), bytes: 42 }],
    blockers: []
  })
  assert.match(markdown, /^# M02 独立目录功能接入包\n/)
  assert.match(markdown, /结论：`READY_FOR_APPROVAL`/)
  assert.match(markdown, /\| GO_BUILD \| `go build -buildvcs=false \.\/\.\.\.` \| 0 \| `cccccccc` \|/)
  assert.match(markdown, /- PASS GO_BUILD :: exit=0/)
  assert.match(markdown, /- `backend\/x\.go` `dddddddd` \(42 B\)/)
  assert.doesNotMatch(markdown, /## 阻断项/)
})

test('UNWIRED_SYMBOLS 覆盖 S06/S07 新增 API 且无重复', () => {
  assert.equal(new Set(UNWIRED_SYMBOLS).size, UNWIRED_SYMBOLS.length)
  for (const symbol of ['CatalogRevision', 'DiffCatalogs', 'SelectByPermission', 'HasPermission']) {
    assert.ok(UNWIRED_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
})

test('S10 的路由策略 API 仍冻结在包内且门禁已登记', () => {
  for (const symbol of ['RouteMode', 'RoutePolicy', 'RouteDecision', 'RouteSkip', 'ErrRoutePolicyInvalid', 'ErrRouteModeUnsatisfied', 'ErrRoutePinConflict', 'SelectRoute']) {
    assert.ok(UNWIRED_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'ROUTE_POLICY')
  assert.ok(gate, 'missing ROUTE_POLICY gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m03-route-policy.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm03-s10-route-policy.log')
})

test('S09 的 binding CAS 类型仍冻结在包内且门禁已登记', () => {
  for (const symbol of ['BindingVersion', 'BindingCommitConflictError', 'ErrBindingCommitConflict']) {
    assert.ok(UNWIRED_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'BINDING_CAS')
  assert.ok(gate, 'missing BINDING_CAS gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m03-binding-cas.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm03-s09-binding-cas.log')
})

test('S11 的路由可用性 API 仍冻结在包内且门禁已登记', () => {
  for (const symbol of ['RouteFailureClass', 'RouteFailureError', 'CoreImplementationIndex', 'NewCoreImplementationIndex', 'ProviderHealth', 'SelectRouteChecked', 'CheckRouteAvailability', 'ErrRouteNoBuiltin', 'ErrRouteUnavailable']) {
    assert.ok(UNWIRED_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'ROUTE_AVAILABILITY')
  assert.ok(gate, 'missing ROUTE_AVAILABILITY gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m03-route-availability.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm03-s11-route-availability.log')
})
test('S12 的安全回退真值表 API 仍冻结在包内且门禁已登记', () => {
  for (const symbol of ['RouteOperation', 'RouteOutcome', 'RouteFallbackAction', 'RouteFallbackDecision', 'RouteFallbackInvalidError', 'ErrRouteFallbackInvalid', 'DecideRouteFallback']) {
    assert.ok(UNWIRED_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'ROUTE_FALLBACK')
  assert.ok(gate, 'missing ROUTE_FALLBACK gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m03-route-fallback.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm03-s12-route-fallback.log')
})

test('S13 的跨端绑定 scope 合约仍冻结在包内且门禁已登记', () => {
  for (const symbol of ['ScopeDimension', 'ScopeClaims', 'ScopeRequirement', 'ProviderScopeContract', 'ScopeMatch', 'ScopeProviderAvailability', 'ScopeContractInvalidError', 'ScopeProviderUnavailableError', 'ScopeNoMatchError', 'ErrScopeContractInvalid', 'ErrScopeProviderUnavailable', 'ErrScopeNoMatch', 'SelectScopedProvider']) {
    assert.ok(UNWIRED_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'ROUTE_SCOPE')
  assert.ok(gate, 'missing ROUTE_SCOPE gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m03-route-scope.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm03-s13-route-scope.log')
})

test('S01 的 InvocationID 传播链只在已冻结的桥接点被引用且门禁已登记', () => {
  for (const symbol of ['InvocationTrace', 'InvocationContract', 'NewInvocationContract', 'UnknownInvocationOutcomeError', 'SideEffectClass', 'ErrInvocationOutcomeUnknown', 'ErrInvocationContractInvalid', 'ErrInvocationTransportLost', 'ClassifyInvocationOutcome', 'IsAmbiguousInvocationFailure']) {
    assert.ok(INVOCATION_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  assert.deepEqual(INVOCATION_BRIDGE_SITES, [
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
  ])
  const gate = GATES.find((entry) => entry.id === 'INVOCATION_CONTRACT')
  assert.ok(gate, 'missing INVOCATION_CONTRACT gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m04-invocation-contract.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm04-s01-invocation-contract.log')
})

test('S01 的调用链契约已真实桥接且没有越界引用', () => {
  assert.deepEqual(invocationBridgeProblems(repoRoot), [])
  const hits = scanInvocationSymbols(repoRoot)
  const sites = [...new Set(hits.map((hit) => hit.path))].sort()
  assert.deepEqual(sites, [...INVOCATION_BRIDGE_SITES].sort())
  assert.ok(hits.some((hit) => hit.path.endsWith('host_api/invocation_bridge.go') && hit.symbol === 'InvocationTrace'))
  assert.ok(hits.some((hit) => hit.path.endsWith('capability/provider_invocation.go') && hit.symbol === 'NewInvocationContract'))
})

test('S01 的结果分类只被 S02、S03、S04、S05 与 S07 消费', () => {
  const classified = scanInvocationSymbols(repoRoot, ['ClassifyInvocationOutcome', 'IsAmbiguousInvocationFailure'])
  assert.deepEqual([...new Set(classified.map((hit) => hit.path))], [
    'backend/internal/devicemesh/executionjournal/ledger.go',
    'backend/internal/devicemesh/executionjournal/reconciliation.go'
  ])
  const unknown = scanInvocationSymbols(repoRoot, ['UnknownInvocationOutcomeError', 'ErrInvocationOutcomeUnknown'])
  assert.deepEqual([...new Set(unknown.map((hit) => hit.path))].sort(), [
    'backend/internal/devicemesh/executionjournal/effect_token_test.go',
    'backend/internal/devicemesh/executionjournal/fault_matrix_test.go',
    'backend/internal/devicemesh/executionjournal/ledger_test.go',
    'backend/internal/extension/kernel/plugincontract/route_read_fallback.go',
    'backend/internal/extension/kernel/plugincontract/route_read_fallback_test.go'
  ])
})

test('S02 的幂等账本门禁已登记且作用域键面被冻结', () => {
  assert.equal(LEDGER_PACKAGE_DIR, 'backend/internal/devicemesh/executionjournal/')
  assert.equal(LEDGER_SYMBOLS.length, 16)
  for (const symbol of ['InvocationLedger', 'NewLedger', 'LedgerSchema', 'LedgerRequest', 'LedgerScope', 'EffectState', 'ErrLedgerKeyInvalid', 'ErrLedgerPayloadConflict', 'ErrLedgerBindingFrozen', 'ErrLedgerEffectUnknown']) {
    assert.ok(LEDGER_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'LEDGER_IDEMPOTENCY')
  assert.ok(gate, 'missing LEDGER_IDEMPOTENCY gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m04-ledger.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm04-s02-ledger.log')
})

test('S02 的幂等账本尚未接入生产调用点', () => {
  assert.deepEqual(scanLedgerSymbols(repoRoot), [])
  assert.deepEqual(ledgerWiringProblems(repoRoot), [])
})

test('S03 的只读回退门禁已登记且契约面被冻结', () => {
  assert.equal(READ_FALLBACK_SYMBOLS.length, 13)
  for (const symbol of ['InvokeReadWithFallback', 'ReadFallbackRequest', 'ReadFallbackCandidate', 'ReadFallbackOptions', 'ReadFallbackAttempt', 'ReadFallbackResult', 'ReadFallbackPanicError', 'ReadFallbackExhaustedError', 'ErrReadFallbackMutating', 'ErrReadFallbackPermissionDenied']) {
    assert.ok(READ_FALLBACK_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'READ_FALLBACK')
  assert.ok(gate, 'missing READ_FALLBACK gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m04-read-fallback.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm04-s03-read-fallback.log')
})

test('S03 的只读回退尚未接入生产调用点', () => {
  assert.deepEqual(scanReadFallbackSymbols(repoRoot), [])
  assert.deepEqual(readFallbackWiringProblems(repoRoot), [])
})

test('S04 的效果凭据门禁已登记且凭据面被冻结', () => {
  assert.equal(EFFECT_TOKEN_SYMBOLS.length, 17)
  for (const symbol of ['EffectToken', 'EffectRecord', 'EffectAuthority', 'EffectTokenSchema', 'ErrEffectTokenInvalid', 'ErrEffectRecordConflict', 'BusinessKey', 'IssueEffectToken', 'VerifyEffectToken', 'Reconcile', 'CommitWithEffect']) {
    assert.ok(EFFECT_TOKEN_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'EFFECT_TOKEN')
  assert.ok(gate, 'missing EFFECT_TOKEN gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m04-effect-token.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm04-s04-effect-token.log')
})

test('S04 的效果凭据尚未接入生产调用点', () => {
  assert.deepEqual(scanEffectTokenSymbols(repoRoot), [])
  assert.deepEqual(effectTokenWiringProblems(repoRoot), [])
})

test('S05 的人工核对门禁已登记且核对面被冻结', () => {
  assert.equal(RECONCILIATION_SYMBOLS.length, 22)
  for (const symbol of [
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
    'ErrReconciliationPending',
    'ReserveIrreversible',
    'DispatchIrreversible',
    'ResolveReconciliation',
    'RecoverInterrupted',
    'OpenReconciliations'
  ]) {
    assert.ok(RECONCILIATION_SYMBOLS.includes(symbol), `missing ${symbol}`)
  }
  const gate = GATES.find((entry) => entry.id === 'RECONCILIATION')
  assert.ok(gate, 'missing RECONCILIATION gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m04-reconciliation.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm04-s05-reconciliation.log')
})

test('S05 的人工核对尚未接入生产调用点', () => {
  assert.deepEqual(scanReconciliationSymbols(repoRoot), [])
  assert.deepEqual(reconciliationWiringProblems(repoRoot), [])
})

test('S06 的作用域熔断门禁已登记且熔断面被冻结', () => {
  assert.equal(TRUSTED_SERVICE_PACKAGE_DIR, 'backend/internal/extension/kernel/trusted_service/')
  assert.deepEqual(SCOPED_BREAKER_SYMBOLS, [
    'BreakerKey',
    'ErrBreakerKeyInvalid',
    'ErrCircuitHalfOpenExhausted',
    'ScopedBreakers',
    'NewScopedBreakers',
    'BreakerSnapshot'
  ])
  const gate = GATES.find((entry) => entry.id === 'SCOPED_BREAKER')
  assert.ok(gate, 'missing SCOPED_BREAKER gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m04-scoped-breaker.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm04-s06-scoped-breaker.log')
})

test('S06 的作用域熔断器尚未接入生产调用点', () => {
  assert.deepEqual(scanScopedBreakerSymbols(repoRoot), [])
  assert.deepEqual(scopedBreakerWiringProblems(repoRoot), [])
})

test('S06 的冻结扫描只认包限定引用并且不吞掉同后缀符号', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-breaker-'))
  const supervisor = join(dir, 'backend', 'internal', 'extension', 'kernel', 'supervisor')
  mkdirSync(supervisor, { recursive: true })
  writeFileSync(join(supervisor, 'guard.go'), 'package supervisor\n\nvar _ = trusted_service.ScopedBreakers{}\nvar _ = trusted_service.NewScopedBreakers\nvar _ = ScopedBreakers{}\nvar _ = other.NewScopedBreakers{}\n', 'utf8')
  const hits = scanScopedBreakerSymbols(dir)
  assert.deepEqual(hits, [
    { path: 'backend/internal/extension/kernel/supervisor/guard.go', symbol: 'NewScopedBreakers' },
    { path: 'backend/internal/extension/kernel/supervisor/guard.go', symbol: 'ScopedBreakers' }
  ])
  assert.deepEqual(scopedBreakerWiringProblems(dir), [
    'scoped breaker reached a site before its wiring step: backend/internal/extension/kernel/supervisor/guard.go -> NewScopedBreakers',
    'scoped breaker reached a site before its wiring step: backend/internal/extension/kernel/supervisor/guard.go -> ScopedBreakers'
  ])
})

test('S07 的故障注入矩阵门禁已登记且 panic 守卫面被冻结', () => {
  assert.equal(EXECUTIONJOURNAL_PACKAGE_DIR, 'backend/internal/devicemesh/executionjournal/')
  assert.deepEqual(FAULT_MATRIX_SYMBOLS, ['RunPanicError'])
  const gate = GATES.find((entry) => entry.id === 'FAULT_MATRIX')
  assert.ok(gate, 'missing FAULT_MATRIX gate')
  assert.deepEqual(gate.args, ['scripts/rsi/m04-fault-matrix.mjs'])
  assert.equal(gate.command, 'node')
  assert.equal(gate.file, 'm04-s07-fault-matrix.log')
})

test('S07 的 panic 守卫尚未接入生产调用点', () => {
  assert.deepEqual(scanFaultMatrixSymbols(repoRoot), [])
  assert.deepEqual(faultMatrixWiringProblems(repoRoot), [])
})

test('S07 的冻结扫描只认包限定引用并且不吞掉同后缀符号', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-matrix-'))
  const supervisor = join(dir, 'backend', 'internal', 'extension', 'kernel', 'supervisor')
  mkdirSync(supervisor, { recursive: true })
  writeFileSync(join(supervisor, 'guard.go'), 'package supervisor\n\nvar _ = executionjournal.RunPanicError{}\nvar _ = RunPanicError{}\nvar _ = other.RunPanicError{}\n', 'utf8')
  const hits = scanFaultMatrixSymbols(dir)
  assert.deepEqual(hits, [{ path: 'backend/internal/extension/kernel/supervisor/guard.go', symbol: 'RunPanicError' }])
  assert.deepEqual(faultMatrixWiringProblems(dir), [
    'fault matrix guard reached a site before its wiring step: backend/internal/extension/kernel/supervisor/guard.go -> RunPanicError'
  ])
})

test('PRESERVED_EVIDENCE 指向 M01 守卫的两份真实证据文件', () => {
  assert.deepEqual(PRESERVED_EVIDENCE, [
    'artifacts/rsi/m01-s06-offstate-db-guard.json',
    'artifacts/rsi/m01-s06-frontend-route-guard.json'
  ])
  for (const path of PRESERVED_EVIDENCE) assert.ok(existsSync(join(repoRoot, path)), `missing ${path}`)
})

test('解冻名单与冻结名单不重叠且每个符号只登记在一处', () => {
  const frozen = new Set(UNWIRED_SYMBOLS)
  assert.equal(frozen.size, UNWIRED_SYMBOLS.length)
  assert.equal(new Set(WIRED_BY_ROUTE_CACHE).size, WIRED_BY_ROUTE_CACHE.length)
  for (const symbol of WIRED_BY_ROUTE_CACHE) {
    assert.equal(frozen.has(symbol), false, `${symbol} cannot be both frozen and wired`)
  }
  for (const symbol of ROUTE_CACHE_PRODUCTION_SYMBOLS) {
    assert.ok(WIRED_BY_ROUTE_CACHE.includes(symbol), `${symbol} must be part of the unfrozen set`)
  }
})

test('真实仓库里绑定/选择 API 只在 routecache、routee2e 与 routeconformance 内被引用', () => {
  const hits = scanWiredSymbols(repoRoot)
  assert.ok(hits.length >= WIRED_BY_ROUTE_CACHE.length, `expected references for every symbol, got ${hits.length}`)
  for (const hit of hits) {
    assert.ok(hit.inRouteCache || hit.inRouteE2E || hit.inRouteConformance, `${hit.path} -> ${hit.symbol} escaped the routing packages`)
  }
  assert.deepEqual(routeCacheWiringProblems(repoRoot), [])
  for (const symbol of ROUTE_CACHE_PRODUCTION_SYMBOLS) {
    assert.ok(hits.some((hit) => hit.path === ROUTE_CACHE_ENTRY_FILE && hit.symbol === symbol), `${symbol} not wired by route_cache.go`)
  }
})

test('绑定 API 逃出路由包会被定向断言点名', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-wired-'))
  const cache = join(dir, 'backend', 'internal', 'extension', 'kernel', 'routecache')
  mkdirSync(cache, { recursive: true })
  writeFileSync(join(cache, 'route_cache.go'), 'package routecache\n\nvar _ = plugincontract.BindingSnapshot{}\n', 'utf8')
  const stray = join(dir, 'backend', 'internal', 'extension', 'kernel', 'other')
  mkdirSync(stray, { recursive: true })
  writeFileSync(join(stray, 'consumer.go'), 'package other\n\nvar _ = plugincontract.SelectProviderCandidates\n', 'utf8')
  const problems = routeCacheWiringProblems(dir)
  assert.ok(
    problems.some((problem) => problem.includes('binding API used outside the routing packages') && problem.includes('SelectProviderCandidates')),
    problems.join('; ')
  )
})

test('routee2e 与 routeconformance 内引用绑定 API 属于允许范围', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-wired-e2e-'))
  const cache = join(dir, ROUTE_CACHE_PACKAGE_DIR)
  mkdirSync(cache, { recursive: true })
  writeFileSync(
    join(cache, 'route_cache.go'),
    'package routecache\n\nvar _ = plugincontract.BindingSnapshot{}\nvar _ = plugincontract.BindingRequest{}\nvar _ = plugincontract.BindingInvalidError{}\nvar _ = plugincontract.ProviderCandidates{}\nvar _ = plugincontract.SelectProviderCandidates\n',
    'utf8'
  )
  const e2e = join(dir, ROUTE_E2E_PACKAGE_DIR)
  mkdirSync(e2e, { recursive: true })
  writeFileSync(join(e2e, 'route_e2e.go'), 'package routee2e\n\nvar _ = plugincontract.SelectProviderCandidates\n', 'utf8')
  const conformance = join(dir, ROUTE_CONFORMANCE_PACKAGE_DIR)
  mkdirSync(conformance, { recursive: true })
  writeFileSync(join(conformance, 'contract.go'), 'package routeconformance\n\nvar _ = plugincontract.BindingRequest{}\n', 'utf8')
  assert.deepEqual(routeCacheWiringProblems(dir), [])
})

test('route_cache.go 少接一个 API 会被定向断言点名', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-wired-missing-'))
  const cache = join(dir, 'backend', 'internal', 'extension', 'kernel', 'routecache')
  mkdirSync(cache, { recursive: true })
  writeFileSync(
    join(cache, 'route_cache.go'),
    'package routecache\n\nvar _ = plugincontract.BindingSnapshot{}\nvar _ = plugincontract.BindingRequest{}\n',
    'utf8'
  )
  const problems = routeCacheWiringProblems(dir)
  for (const symbol of ['ProviderCandidates', 'SelectProviderCandidates', 'BindingInvalidError']) {
    assert.ok(problems.some((problem) => problem.includes(`does not wire plugincontract.${symbol}`)), `${symbol}: ${problems.join('; ')}`)
  }
})

test('限定引用按标识符边界匹配而不是裸子串', () => {
  const dir = mkdtempSync(join(tmpdir(), 'm02-admission-boundary-'))
  const cache = join(dir, 'backend', 'internal', 'extension', 'kernel', 'routecache')
  mkdirSync(cache, { recursive: true })
  writeFileSync(join(cache, 'route_cache_test.go'), 'package routecache\n\nvar _ = plugincontract.BindingScopeOrder\n', 'utf8')
  const hits = scanWiredSymbols(dir)
  assert.deepEqual(hits.map((hit) => hit.symbol), [])
})
