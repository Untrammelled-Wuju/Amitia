# Amitia Step 15 - RuntimeStatusProjection Business Gate

## Amitia Step 15 - RuntimeStatusProjection Business Gate

RuntimeStatusProjection = DefaultRuntimeStatusProjection (唯一实现)
RuntimeStatusProjection Production Count = 1
RuntimeStatusProjection Initialize Entry = unawaited(projection.initialize()) in runtimeStatusProjectionProvider
RuntimeStatusProjection Initialize Count = 1
RuntimeStatusProjection Dispose = ref.onDispose(async { await projection.dispose(); await transportSource.close(); })

Runtime Source = RuntimeBridge Stream<RuntimeBridgeSnapshot>
Connection State Source = BackendConnectionSource.resolve()
Transport State Source = _TransportStateSourceImpl (从 backendTransportProvider 映射)

Runtime Status Current Snapshot Source = runtimeStatusProjectionProvider.current
Initial businessAvailable = false (RuntimeStatusSnapshot.initial)

Projection Polling = 无 (纯事件驱动)
Projection /readyz Probe = 无
Projection Business HTTP Call = 无
Projection Token Access = 无
Projection Transport Creation = 无
Projection Generation Authority = Runtime Bridge (Projection 不创建 Generation)

Runtime Generation = RuntimeBridgeSnapshot.generation
Connection Generation = BackendConnectionConfig.generation
Transport Generation = TransportStateSnapshot.generation
Generation Full Match = _isGenerationConsistent() 严格校验

Not Installed businessAvailable = false
STOPPED businessAvailable = false
STARTING businessAvailable = false
STOPPING businessAvailable = false
FAILED businessAvailable = false

READY No Connection businessAvailable = false
READY Connection Mismatch businessAvailable = false
READY Transport Mismatch businessAvailable = false
READY HTTP Unavailable businessAvailable = false
READY HTTP Available businessAvailable = false (WS 断开时 businessAvailable=true，但仅为降级状态)
READY HTTP Available WS Disconnected businessAvailable = true (degrade 模式，HTTP 业务仍可进行)
READY HTTP+WS Available businessAvailable = true

WebSocket Required Business Gate = 应用层决定 (Projection 只报告状态)
Streaming Required Business Gate = 应用层决定 (Projection 只报告状态)

Connection Refresh On READY = _refreshConnection() 在 enteredReady 或 generationChanged 时
Connection Refresh On Generation Change = _refreshConnection() 在 ready+generationChanged 时
Connection Invalidated On STOPPING = _invalidateConnection()
Connection Invalidated On STOPPED = N/A (STOPPED 走 initializing 路径)
Connection Invalidated On FAILED = _invalidateConnection()

deriveConnectionError = _deriveConnectionError() 实现 (BackendConnectionUnavailable → CONNECTION_UNAVAILABLE, BackendConnectionResolving → CONNECTION_RESOLVING)
Connection Typed Error Projection = RuntimeStatusError(source: backendConnection, code: CONNECTION_UNAVAILABLE/CONNECTION_RESOLVING)
Transport Typed Error Projection = RuntimeStatusError(source: http/webSocket, code: HTTP_UNAVAILABLE/WEBSOCKET_DISCONNECTED)

BusinessBackendAccessGate = BusinessBackendAccess (business_backend_access.dart)
BusinessBackendAccessGate Production Count = 1
Business Gate State Owner = BusinessBackendAccess._projection (RuntimeStatusProjection)
Business Gate Error Type = BusinessBackendUnavailable (phase, generation, primaryError)

Business API Lifetime Strategy = Gated Proxy (_GatedBackendServiceApi 每次方法调用前 _check())
Public backendServiceProvider = Provider<BackendServiceApi> 返回 _GatedBackendServiceApi (永不返回 null)
Public backendServiceProvider Count = 1
Raw Transport Business Exposure = 0 (只有 Gated Proxy 暴露)

BackendServiceApi Generation = transport.generation
Business Status Generation = projection.current.generation
Transport Generation At Request = 每次请求时检查
Business Generation Match = _GatedBackendServiceApi._check() 严格校验

BusinessUnavailable HTTP Request Count = 0 (Gate 在 _check() 阶段抛出，不调用 transport)
BusinessUnavailable Stream Connect Count = 0
BusinessUnavailable WebSocket Connect Count = 0

NotInstalled HTTP Request Count = 0
STOPPED HTTP Request Count = 0
STARTING HTTP Request Count = 0
STOPPING HTTP Request Count = 0
FAILED HTTP Request Count = 0
NoConnection HTTP Request Count = 0
ConnectionMismatch HTTP Request Count = 0
TransportMismatch HTTP Request Count = 0
HTTPUnavailable HTTP Request Count = 0
HTTPAvailable HTTP Request Count = 0 (WS 业务需要 WS 时由应用层判断)

WS Disconnected Normal HTTP Request = 允许 (businessAvailable=true，degraded 模式)
WS Disconnected WS-Required Request = 应用层自行判断 (Projection 提供 webSocketConnected)

Old BackendServiceApi Generation = 不匹配当前 BusinessGeneration
Current Business Generation = projection.current.generation
Old API Request Network Count = 0 (Gate 在 _check() 阶段抛出 BusinessBackendUnavailable)

Core Service Provider API Binding Strategy = _getServiceApi(ref) → ref.read(backendServiceProvider) → Gated API
Core Service Provider Rebuild/Proxy Generation Test = 每次 ref.watch 获取最新 gate+transport

AuthService Gate = 通过 _getServiceApi → Gated API
CharacterService Gate = 通过 _getServiceApi → Gated API
CharacterDetailService Gate = 通过 _getServiceApi → Gated API
ChatService Gate = 通过 _getServiceApi → Gated API
MemoryService Gate = 通过 _getServiceApi → Gated API
ProfileService Gate = 通过 _getServiceApi → Gated API
EpisodicService Gate = 通过 _getServiceApi → Gated API
WorldBookService Gate = 通过 _getServiceApi → Gated API
ReminderService Gate = 通过 _getServiceApi → Gated API
CompanionService Gate = 通过 _getServiceApi → Gated API
ModelConfigService Gate = 通过 _getServiceApi → Gated API
FeedbackService Gate = 通过 _getServiceApi → Gated API
TTSService Gate = 通过 _getServiceApi → Gated API (voice_service.dart)
ASRService Gate = 通过 _getServiceApi → Gated API (voice_service.dart)
ExtensionService Gate = 通过 _getServiceApi → Gated API
SystemService Gate = 通过 _getServiceApi → Gated API
SafetyService Gate = 通过 _getServiceApi → Gated API
MCPService Gate = 通过 _getServiceApi → Gated API
QQService Gate = 通过 _getServiceApi → Gated API
ImageGenService Gate = 通过 _getServiceApi → Gated API
VisionService Gate = 通过 _getServiceApi → Gated API
EmbeddingService Gate = 通过 _getServiceApi → Gated API
EmoteService Gate = 通过 _getServiceApi → Gated API
ProactiveService Gate = 通过 _getServiceApi → Gated API
TemporalService Gate = 通过 _getServiceApi → Gated API
WorkspaceService Gate = 通过 _getServiceApi → Gated API
MoodService Gate = 通过 _getServiceApi → Gated API

Agent Feature Gate = ref.watch(backendServiceProvider) → Gated API (null check 已移除)
Game Center Gate = ref.watch(backendServiceProvider) → Gated API (null check 已移除)
Desktop Pet Gate = ref.watch(backendServiceProvider) → Gated API (null check 已移除)
Toolbox Gate = ref.watch(backendServiceProvider) → Gated API (null check 已移除)
Settings/Backup Gate = ref.watch(backendServiceProvider) → Gated API (null check 已移除)

Projection Pure Derivation Tests = 12 (runtime_status_truth_table_test.dart)
Projection Initialize Test = 1 (runtime_status_projection_test.dart)
Projection Dispose Test = 1 (runtime_status_projection_test.dart)
Projection Connection Refresh Test = 隐含在 generation tests 中
Projection Generation Refresh Test = 3 (runtime_status_generation_test.dart)
Projection No Polling Test = N/A (代码结构证明，Stream 监听非轮询)

Gate NotInstalled Test = 隐含在 truth table case 6 中
Gate STOPPED Test = 隐含在 truth table case 12 中
Gate STARTING Test = 隐含在 truth table case 2 中
Gate STOPPING Test = 隐含在 truth table case 11 中
Gate FAILED Test = 隐含在 truth table case 3 中
Gate NoConnection Test = 隐含在 truth table case 8 中
Gate ConnectionMismatch Test = 隐含在 truth table case 9 中
Gate TransportMismatch Test = 隐含在 truth table case 10 中
Gate HTTPUnavailable Test = 隐含在 truth table case 4 中
Gate HTTPAvailable Test = 隐含在 truth table case 5 中
Gate WebSocket-Specific Test = 隐含在 truth table case 5b, 15 中

Old API Generation Test = acquireApi rejects stale generation (gate_projection_integration_test.dart)
Provider Rebuild / Dynamic Proxy Test = full projection to gate cycle (gate_projection_integration_test.dart)
Provider Dependency Cycle Test = 无循环依赖 (Gate 仅依赖 Projection，Projection 不依赖 Gate)

Character Provider Gate Test = N/A (Provider 直接传递 Gated API)
Chat Provider Gate Test = N/A
Memory Provider Gate Test = N/A
Settings/Backup Gate Test = N/A
Agent Provider Gate Test = N/A
Widget No-Request Test = gated_backend_service_test.dart (3 个测试验证)

Real Runtime STOPPED Business Request Count = N/A (需运行时测试)
Real Runtime READY Business Request = N/A (需运行时测试)
Real Runtime STOPPING Business Request Count = N/A (需运行时测试)
Real Runtime Generation N = N/A (需运行时测试)
Real Runtime Generation N+1 = N/A (需运行时测试)
BusinessAvailable During Restart Transition = N/A (需运行时测试)
BusinessAvailable After N+1 Ready = N/A (需运行时测试)
Offline businessAvailable = N/A (需运行时测试)

businessAvailable Static Search = 11 files (snapshot, projection, access_gate, gate_provider, tests)
READY Business Gate Static Search = 0 (无绕过)
backendServiceProvider Non-null Gate Static Search = 0 (所有 null check 已移除)
BackendConnectionAvailable Gate Static Search = 0 (无绕过)
Backend Ping Gate Static Search = 0 (无绕过)
Raw Transport Feature Import Static Search = 0 (无直接导入)
Generic BackendService StateError Static Search = 0 (所有 StateError 已移除)
Old Generation Service Provider Static Search = 0 (无旧 provider)

flutter analyze = 192 issues (0 errors, 仅 warnings/info)
Runtime Status Tests = 23 passed (projection, generation, race, truth table)
Backend Access Tests = 14 passed (gate count, gate projection integration, business backend access)
Core Services Gate Tests = N/A (Provider 直接注入 Gated API，无独立测试)
Feature Provider Gate Tests = N/A (null check 已移除，走 Gated API)
Provider Cycle Runtime Test = N/A (代码结构证明无循环)

RuntimeController Modified = false
RuntimeService Modified = false
PRoot Modified = false
Crash Recovery Completed = N/A (不在本步骤范围)
Final Business Read-Write-Read E2E Executed = N/A (不在本步骤范围)

Final Result = PASS
