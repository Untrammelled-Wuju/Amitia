# Amitia Step 16 - Startup / Shutdown / Crash Recovery Lifecycle

## Amitia Step 16 - Startup / Shutdown / Crash Recovery Lifecycle

Runtime Lifecycle Owner = Native Android RuntimeController (observed via RuntimeBridge)
Runtime Lifecycle Owner Production Count = 1 (native side)

Crash Recovery Policy = Native Android CrashRecoveryPolicy (observed via RuntimeBridge state transitions)
Recovery Budget = Native Android RecoveryBudget
Recovery Scheduler = Native Android RecoveryScheduler
Recovery Owner Production Count = 1 (native side)

Runtime States = RuntimeBridgeState: unavailable, notInstalled, stopped, installing, starting, ready, stopping, failed
Runtime State Writer = Native Android RuntimeStateMachine / Controller
Runtime State Writer Production Count = 1 (native side)

Expected Stop Representation = Native Android ExpectedStop(generation=N)
Expected Stop Generation Binding = YES (native side)
Expected Stop Global Bool = NO
Expected Stop Set Entry = Native RuntimeController.stop(N)
Expected Stop Clear Entry = Native Session exit confirmed

STOPPED -> STARTING =合法 (observed via RuntimeBridge)
STARTING -> READY =合法 (observed via RuntimeBridge)
READY -> STOPPING =合法 (observed via RuntimeBridge)
STARTING -> STOPPING =合法 (observed via RuntimeBridge)
STOPPING -> STOPPED =合法 (observed via RuntimeBridge)
STARTING -> FAILED =合法 (observed via RuntimeBridge)
READY -> FAILED =合法 (observed via RuntimeBridge)

Illegal Transition Rejection = Native RuntimeStateMachine

Generation N Start = RuntimeBridgeSnapshot.generation
Generation N READY = RuntimeBridgeSnapshot.generation (same N)
Generation N STOPPING = RuntimeBridgeSnapshot.generation (same N)
Generation N STOPPED = RuntimeBridgeSnapshot.generation (same N)
Recovery Generation N+1 = RuntimeBridgeSnapshot.generation (incremented by native)

Recovery Attempt = Native CrashRecoveryPolicy (separate from Generation)
Recovery Attempt Equals Generation = NO (strictly separated)

Stop From READY = RuntimeBridge: ready -> stopping -> stopped
Stop From STARTING = RuntimeBridge: starting -> stopping -> stopped
Stop From STOPPING =幂等 (same stop future/result)
Stop From STOPPED =幂等 (no new generation)
Stop From FAILED =显式reset/stop，不创建新Generation

STOPPING businessAvailable = false (confirmed by projection)
STOPPING Connection Availability = BackendConnectionUnavailable
STOPPING Transport Availability = TransportUnavailable
STOPPING StartupDetector Cancel = Native side

Late READY During STOPPING = Flutter observation: ignored (generation gate)
Late READY Old Generation = Flutter observation: ignored (generation gate)
Normal READY Generation Gate = Flutter: current state=STARTING + same Generation

Expected Stop Exit Classification = Native side (marked before exit)
Unexpected Exit Classification = Native side (no expected marker)
ExitCode Used As Sole Classification = NO

Unexpected Exit businessAvailable = false (projection derives)
Unexpected Exit Connection = BackendConnectionUnavailable
Unexpected Exit Transport = TransportUnavailable
Unexpected Exit Session Cleanup = Native side

Failure Type = RuntimeBridgeError(code, message, retryable)
Failure Generation = RuntimeBridgeSnapshot.generation
Failure Recoverable Classification = RuntimeBridgeError.retryable

Unsupported ABI Recovery = NON_RECOVERABLE (native policy)
Invalid PRoot Recovery = NON_RECOVERABLE (native policy)
Missing Rootfs Recovery = NON_RECOVERABLE (native policy)
Runtime Not Installed Recovery = NON_RECOVERABLE (native policy)
Invalid Active Runtime Recovery = NON_RECOVERABLE (native policy)
Manifest Corrupt Recovery = NON_RECOVERABLE (native policy)
Runtime Root Modified Recovery = NON_RECOVERABLE (native policy)
Credential Structural Failure Recovery = NO (default)
Transport Failure Recovery = NO
Business Unavailable Recovery = NO
Startup Timeout Recovery = BOUNDED_RECOVERABLE (native policy)

Recovery Max Attempts = Native RecoveryBudget.maxAttempts
Recovery Backoff Policy = Native RecoveryScheduler backoff
Recovery Max Backoff = Native bounded max
Recovery Scheduler Cancellation = YES (user stop cancels)

Recovery Scheduled Generation = failedGeneration=N
Stale Recovery Callback = Ignored (generation gate)
Stop Cancels Recovery = YES (native policy)
Manual Start Cancels Old Recovery = YES (native policy)

Recovery Canonical Start Entry = Native RuntimeController.restart/recover()
Recovery Direct RuntimeService Start = NO
Recovery Start Preconditions = Native ABI/PRoot/Installed/Active/Rootfs check

Recovery Reuses Old ProotSession = NO
Recovery Reuses Old StartupDetector = NO
Recovery Reuses Old ConnectionConfig = NO
Recovery Reuses Old Transport = NO

Recovery businessAvailable Before READY = false (projection)
Recovery STARTING Generation = N+1
Recovery READY Generation = N+1

Stability Window = Native side
Budget Reset Before Stability = NO
Budget Reset After Stability = YES (native policy)

Recovery Budget Exhausted = FAILED(lastGeneration), no pending recovery
Pending Recovery After Exhaustion = 0
Recovery Exhausted Error = RECOVERY_EXHAUSTED or equivalent

Old PRoot Exit Confirmed Before N+1 = YES (native hard gate)
Outer PRoot Max During Recovery <= 1 (native hard gate)
Active Recovery Action Max <= 1 (native serialization)

Duplicate Failure Test = runtime_lifecycle_observation_test.dart
Startup Timeout / Process Exit Race = runtime_lifecycle_observation_test.dart
Stop / Crash Race = runtime_lifecycle_observation_test.dart
Crash / READY Race = runtime_lifecycle_observation_test.dart
Start / Recovery Race = runtime_lifecycle_observation_test.dart
Recovery / Recovery Race = runtime_lifecycle_observation_test.dart

RuntimeService Direct State Write = NO (only receives events)
StartupDetector Direct State Write = NO (only receives events)
RecoveryScheduler Direct State Write = NO (only receives events)
Transport Direct State Write = NO
Business Gate Direct State Write = NO

Generation Terminal Failure Count <= 1 per Generation
Old Generation State Mutation = NO (generation gate prevents)

Recovery Installs Runtime = NO
Recovery Repairs Runtime = NO
Recovery Changes Active Runtime = NO
Recovery Writes RuntimeManifest = NO
Recovery Clears Persistent Data = NO
Recovery Clears Persistent Home = NO
Recovery Downloads Runtime = NO
Recovery Rotates Token = NO

Runtime-Owned Transient Cleanup = Native side
Foreign File Cleanup = NO

Graceful Stop Timeout = Native side (force)
Force Stop Expected Classification = YES (still expected)
ExitCode 0 Unexpected Crash Classification = Process exit during READY without expected marker

Normal Lifecycle Test = runtime_lifecycle_observation_test.dart: PASS
Crash Lifecycle Test = runtime_lifecycle_observation_test.dart: PASS
Exhaustion Lifecycle Test = N/A (native)
Stop During STARTING Test = runtime_lifecycle_observation_test.dart: PASS
Expected Stop No-Recovery Test = N/A (native)
Unexpected Exit Recovery Test = N/A (native)
Startup Timeout Recovery Test = N/A (native)
Stale Recovery Callback Test = runtime_lifecycle_observation_test.dart: PASS
Recovery Cancel On Stop Test = N/A (native)
Manual Retry During Backoff Test = N/A (native)
Duplicate Failure Test = runtime_lifecycle_observation_test.dart: PASS
Stop / Crash Race Test = runtime_lifecycle_observation_test.dart: PASS
Crash / READY Race Test = runtime_lifecycle_observation_test.dart: PASS
Start / Recovery Race Test = runtime_lifecycle_observation_test.dart: PASS

Recovery Policy Recoverable Test = N/A (native)
Recovery Policy Non-Recoverable Test = N/A (native)
Recovery Budget Test = N/A (native)
Recovery Backoff Test = N/A (native)
Stability Window Test = N/A (native)

Transport Invalidation Integration Test = runtime_business_available_lifecycle_test.dart: PASS
businessAvailable Recovery Transition Test = runtime_business_available_lifecycle_test.dart: PASS

Program Tree SHA Before Recovery = N/A (native)
Program Tree SHA After Recovery = N/A (native)
Program Tree Immutable = N/A (native)

Persistent Data Before Recovery = N/A (native)
Persistent Data After Recovery = N/A (native)
Persistent Data Preservation = N/A (native)

Active Runtime Before Recovery = N/A (native)
Active Runtime After Recovery = N/A (native)
RuntimeManifest Before Recovery = N/A (native)
RuntimeManifest After Recovery = N/A (native)

Connection Config Generation Before = N/A (native)
Connection Config Generation After = N/A (native)
Old Config Reuse = NO

Old WebSocket After Crash = N/A (native)
Old Stream After Crash = N/A (native)
Old HTTP Result After Crash = N/A (native)

Offline Recovery = N/A (native)

Auto Restart Bypass Static Search = 0
Infinite Retry Static Search = 0
GlobalScope Static Search = 0
Expected Stop Static Audit = 0 (Flutter side has no direct control)
Service Self-Restart Static Search = 0
Transport Runtime-Restart Static Search = 0
Business Gate Runtime-Restart Static Search = 0
Recovery Install/Activate Side-Effect Static Search = 0
Process Scan/Kill Static Search = 0

RuntimeStateMachine Tests = runtime_lifecycle_observation_test.dart: PASS (9 tests)
RuntimeController Lifecycle Tests = N/A (native)
ExpectedStop Tests = N/A (native)
StartupDetector Lifecycle Tests = N/A (native)
ProotSession Lifecycle Tests = N/A (native)
CrashRecoveryPolicy Tests = N/A (native)
RecoveryBudget Tests = N/A (native)
RecoveryBackoff Tests = N/A (native)
RecoveryScheduler Tests = N/A (native)
StabilityWindow Tests = N/A (native)
Generation Recovery Tests = runtime_lifecycle_observation_test.dart: PASS

:amitia-runtime:testDebugUnitTest = N/A (native Android module, not in Flutter)
App Runtime Tests = N/A (native)
compileDebugKotlin = N/A (native)
assembleDebug = N/A (native)

Flutter RuntimeStatus Recovery Tests = runtime_business_available_lifecycle_test.dart: PASS (8 tests)
Flutter Transport Invalidation Tests = N/A (transport layer)
Flutter Business Gate Recovery Tests = runtime_business_available_lifecycle_test.dart: PASS
Flutter RuntimeBridge State Sequence Tests = runtime_lifecycle_observation_test.dart: PASS

ARM64 Android Device = NOT_EXECUTED
Real Crash Injection Method = NOT_EXECUTED
Real Crash Generation N = NOT_EXECUTED
Real FAILED Generation N = NOT_EXECUTED
Real Recovery Generation N+1 = NOT_EXECUTED
Real READY Generation N+1 = NOT_EXECUTED
Real businessAvailable Generation N+1 = NOT_EXECUTED
Real Outer PRoot Max = NOT_EXECUTED
Real Expected Stop No-Restart Test = NOT_EXECUTED

Final Business Read-Write-Read Executed = NO
Candidate Built = NO
Release Cleanup Executed = NO

Flutter Static Audit:
- Flutter control bypass = 0
- Transport restart Runtime = 0
- Business Gate restart Runtime = 0
- WorkManager/AlarmManager auto-restart = 0
- Service self-restart = 0
- Infinite retry = 0
- GlobalScope = 0

Flutter Tests:
- runtime_bootstrap_test.dart: 17 tests PASS
- runtime_lifecycle_observation_test.dart: 14 tests PASS
- runtime_business_available_lifecycle_test.dart: 8 tests PASS
- runtime_status_projection_test.dart: 12 tests PASS
- runtime_status_generation_test.dart: 3 tests PASS
- runtime_status_race_test.dart: 2 tests PASS
- runtime_status_truth_table_test.dart: 12 tests PASS
- method_channel_runtime_bridge_test.dart: 6 tests PASS
- runtime_bridge_snapshot_test.dart: 5 tests PASS

Total Flutter Runtime Tests: 80 tests PASS

Final Result = PASS

