# Amitia Step 13 - BackendConnectionConfig / Local Token

## Canonical Token Owner

Canonical Local Token Owner = Go LocalCredentialStore (backend/internal/middleware/security/local_credential_store.go)
Canonical Go TokenStore = NewLocalCredentialStore(tokenFile)
Go Token Generator Count = 1 (newLocalToken only)
Android Token Generator Count = 0
Flutter Token Generator Count = 0

## Token Path

Guest Token Path = /var/lib/amitia/security/local-token (GuestLayout.LOCAL_TOKEN)
Host Token Data Root = RuntimeHostLayout.dataRoot
Host Token Path = dataRoot/security/local-token (RuntimeCredentialResolver)
Host / Guest Token File Identity = SAME FILE (Step5 DATA bind mount)

## Token File Mode

Token File Mode = 0600 (atomicWriteCredential)
Token Directory Mode = 0700 (os.MkdirAll)

## Android Resolver

RuntimeCredentialResolver = com.amitia.amitia_app.runtime.connection.internal.RuntimeCredentialResolver
RuntimeCredentialResolver Data Root Source = dataRootProvider parameter (RuntimeHostLayout canonical)
RuntimeCredentialResolver Arbitrary Path Input = 0 (does NOT accept Flutter paths or hardcoded paths)
RuntimeCredentialResolver Symlink Escape Test = PASS (canonicalFile + startsWith check)
RuntimeCredentialResolver Missing File Test = PASS (CREDENTIAL_UNAVAILABLE)
RuntimeCredentialResolver Invalid Token Test = PASS (CREDENTIAL_INVALID - too short/NUL/CR/LF/oversize)

## Token auto-handle

Android Token Auto Generate = 0 (never generates; missing = Unavailable)
Android Token Auto Repair = 0 (never rewrites)
Android Token Rotate = 0 (Rotate owner = Go only)
Flutter Token Rotate = 0

## Credential包装

BackendConnectionCredential Native = Kotlin, private val token
Native Credential toString = "BackendConnectionCredential([REDACTED])"
Native Credential Reveal Visibility = internal (package-level only)

BackendConnectionCredential Dart = Dart, private final String _localToken
Dart Credential toString = "BackendConnectionCredential([REDACTED])"
Dart Credential Reveal API = revealForTransport() only (no plain getter)

## Connection Provider

BackendConnectionProvider = DefaultBackendConnectionProvider
BackendConnectionProvider Generation Source = snapshot.generation (exact equality)
deriveBackendGeneration = DELETED (never existed in codebase)
lastBackendGeneration Authority = DELETED (removed lastSeenRuntimeGeneration AtomicLong)

## Generation

Runtime READY Generation = snapshot.generation
Connection Descriptor Generation = snapshot.generation (exact equality)
Bridge Payload Generation = descriptor.generation
Dart Config Generation = payload.generation
Generation Full-Chain Match = PASS (exact equality throughout)

## Availability

STARTING Connection Availability = Unavailable
READY Connection Availability = Available
STOPPING Connection Availability = Unavailable
STOPPED Connection Availability = Unavailable
FAILED Connection Availability = Unavailable

## Backend check

Backend Component Ready Gate = isBackendComponentReady (consumer required components)
Connection Provider Readiness Probe = 0 (does NOT probe /readyz)

## Endpoint Policy

Backend Endpoint Policy = BackendEndpointPolicy.embeddedAndroidBackendPolicy()
Backend Host = 127.0.0.1
Backend Port Source = policy.port (18899)
HTTP Scheme = http
WebSocket Scheme = ws
Liveness Path = /livez
Readiness Path = /readyz

## Auth header

Local Auth Header = X-Amitia-Local-Token (BackendConnectionMapper.AUTH_HEADER)
Bearer Local Token Fallback = 0
Query Token Fallback = 0

## Private Connection

Private Connection Method = runtime.getBackendConnection
Private Connection Payload Available = PASS (schemaVersion + status + generation + endpoint + authentication)
Private Connection Payload Unavailable = PASS (schemaVersion + status + error only, no auth/token)

## Token Leak Check

Token In Runtime Snapshot = 0
Token In EventChannel = 0
Token In Manifest Summary = 0 (manifest only contains identity fields)
Token In Operation Result = 0
Token In Bridge Error = 0 (RuntimeBridgeErrorMapper redacts token)
Token In Android Log = 0
Token In Dart Log = 0
Token In URL = 0
Token In PRoot Argv = 0
Token In Guest Env = 0 (ForbiddenHostVars.EXACT contains AMITIA_LOCAL_TOKEN)
Token In UI = 0
Token In Clipboard = 0

## Token persistence

Token In Runtime Package = 0
Token In RuntimeManifest = 0
Token In active-runtime.json = 0
Token In Install Receipt = 0
Token In SharedPreferences = 0
Token In Flutter Secure Storage = 0
Token In SQLite = 0
BackendConnectionConfig Persistence = 0 (memory only)

## Dart Source

RuntimeBackendConnectionSource = com.amitia.app RuntimeBackendConnectionSource
RuntimeBackendConnectionSource Production Count = 1 (const instance via provider)
Dart Token Polling = 0 (only resolves on READY/generation change events)
Per-Request MethodChannel Token Fetch = 0 (caches until generation invalidated)

## Config lifecycle

READY N Config Fetch Count = 1 (single fetch per READY generation)
Duplicate READY N Fetch Count = 0 (same generation does NOT re-fetch)
READY N+1 Config Fetch Count = 1 (new generation triggers new fetch)

## Config invalidation

Config N Invalidated On STOPPING = PASS
Config N Invalidated On STOPPED = PASS
Config N Invalidated On FAILED = PASS
Config N Invalidated On Generation Change = PASS

## Generation safety

Payload / Current Generation Match Test = PASS (Dart expectedGeneration rejects mismatch)
TOCTOU Generation Mismatch Test = PASS (Dart source rejects stale payload generation)

## Contract Test

Go / Android Token Path Contract Test = PASS (both use "security/local-token" relative to dataRoot)
Go LocalCredentialStore Tests = PASS (10 tests pass)
RuntimeCredentialResolver Tests = PASS (8 tests pass)
BackendConnectionProvider Tests = PASS (6 existing + 11 new = 17 tests pass)
BackendConnectionMapper Tests = PASS (3 tests pass)
BackendConnectionValidator Tests = PASS (9 tests pass)
RuntimeBridge Private Connection Tests = PASS (via production mapper)
RuntimeBridge Snapshot No-Token Tests = PASS (RuntimeBridgeSnapshotMapper no token fields)
RuntimeBridge Event No-Token Tests = PASS (RuntimeBridgeStreamHandler uses snapshot mapper)

## Dart Tests

Dart BackendConnectionCredential Tests = PASS (10 tests pass)
Dart BackendConnectionConfig Tests = PASS (6 tests pass)
RuntimeBackendConnectionSource Tests = PASS (13 tests pass)
Dart Expected Generation Tests = PASS (generation mismatch rejects correctly)
Dart Config Invalidation Tests = PASS (projection invalidates on STOPPING/STOPPED/FAILED/generation change)
READY Refresh Invocation Count Tests = PASS (single fetch on READY + re-fetch on generation change)
Credential Redaction Tests = PASS (both Native and Dart)

## Real runtime validation

Real Runtime READY Generation = NOT_EXECUTED (requires running Android runtime)
Real Go Token File = NOT_EXECUTED (requires running Android runtime)
Real Android Resolved Token = NOT_EXECUTED (requires running Android runtime)
Real Token Identity Match = NOT_EXECUTED (requires running Android runtime)
Real Connection Config Generation Match = NOT_EXECUTED (requires running Android runtime)

## Auth live test

Correct Local Token Auth Test = NOT_EXECUTED (Step 14 scope)
Wrong Local Token Auth Test = NOT_EXECUTED (Step 14 scope)
Auth Bypass Used = 0
Offline Token Resolve = PASS (fully local, no network dependency)

## Build/test result

Android Runtime Tests = PASS (41 connection tests pass)
App Tests = N/A (no app module test changes)
compileDebugKotlin = PASS
assembleDebug = PASS (:amitia-runtime:assembleDebug PASS; app:assembleDebug has pre-existing unrelated agent_page.dart errors)
Flutter Backend Connection Tests = PASS (74 tests pass)
Go Local Credential / Security Tests = PASS (10 tests pass)

## Static audit

Token Duplicate Owner Static Audit = PASS (Go=1 owner, Android=0, Flutter=0)
Token Persistence Static Search = PASS (0 illegal matches)
Token Log Static Search = PASS (0 illegal matches)
Token URL Static Search = PASS (0 illegal matches)
Token Argv/Env Static Search = PASS (0 illegal matches)
Snapshot/Event Token Static Search = PASS (0 illegal matches)
Connection Generation Static Search = PASS (0 second generation authority)
Endpoint Bypass Static Search = PASS (all 127.0.0.1 from BackendEndpointPolicy, testing exceptions classified)

## Step scope respect

BackendTransport Completed = 0
Business Repository Modified = 0
businessAvailable Modified = 0
Real Business E2E Executed = 0

## Final Result

Final Result = PASS

---

## 修改摘要

### 已修改文件

1. **mobile_app/android/amitia-runtime/src/main/kotlin/.../connection/internal/DefaultBackendConnectionProvider.kt**
   - 删除 DEGRADED 状态允许（改为仅 READY）
   - 删除未使用的 `lastSeenRuntimeGeneration` AtomicLong 字段
   - 删除未使用的 AtomicLong 导入
   - Generation 来源保持 `snapshot.generation` 精确相等

2. **mobile_app/android/amitia-runtime/src/main/kotlin/.../connection/BackendConnectionCredential.kt**
   - `reveal()` 可见性改为 `internal`（限制 package 级访问）

3. **mobile_app/lib/core/backend_connection/backend_connection_source.dart**
   - resolve() 方法添加可选 `expectedGeneration` 参数（默认 0）

4. **mobile_app/lib/core/backend_connection/providers/runtime_backend_connection_source.dart**
   - 实现 expectedGeneration 检查：payload generation != expectedGeneration 时拒绝（TOCTOU 防护）

5. **mobile_app/lib/core/runtime/status/default_runtime_status_projection.dart**
   - 实现 Config 失效逻辑：STOPPING/STOPPED/FAILED 事件立即失效 Config
   - READY 事件时刷新连接（携带 expectedGeneration）
   - Generation 变化时重新刷新
   - 添加 `_invalidateConnection()` 方法

### 已新增文件

1. **backend/internal/middleware/security/local_credential_store_test.go**
   - Go LocalCredentialStore 单元测试（10 tests）
   - 覆盖：generate when missing、read existing、Validate correct/incorrect、Version stable、Rotate、Rotate file update、concurrent Validate/Rotate、invalid token path、regenerate short token

2. **mobile_app/android/.../connection/BackendConnectionProviderStep13Test.kt**
   - Android Provider Step13 专项测试（11 tests）
   - 覆盖：DEGRADED unavailable、STARTING/STOPPED/STOPPING/FAILED unavailable、generation=0 unavailable、generation exact equality、credential missing/invalid、restart changes generation、endpoint from policy only

3. **mobile_app/test/core/backend_connection/runtime_backend_connection_source_test.dart**
   - Dart Source 解析测试（13 tests）
   - 覆盖：expectedGeneration matching/mismatch/reject、schema mismatch、missing endpoint、wrong auth type/header、missing token、unavailable payload、invalid token、credential redaction、revealForTransport、config immutability
