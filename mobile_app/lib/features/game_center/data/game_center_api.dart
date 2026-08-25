import '../../../core/backend_transport/backend_service_api.dart';
import '../domain/game_center_dto.dart';

class GameCenterApi {
  final BackendServiceApi _api;

  const GameCenterApi(this._api);

  Future<GameCenterPluginList> listPlugins({
    int page = 1,
    int pageSize = 100,
    String? search,
    String? status,
    bool? enabled,
  }) async {
    final data = await _api.get<Map<String, dynamic>>(
      '/api/game-center/plugins',
      queryParameters: <String, dynamic>{
        'page': page,
        'pageSize': pageSize,
        if (search != null && search.trim().isNotEmpty) 'search': search.trim(),
        if (status != null && status.trim().isNotEmpty) 'status': status.trim(),
        if (enabled != null) 'enabled': enabled,
      },
    );
    return GameCenterPluginList.fromJson(data ?? const <String, dynamic>{});
  }

  Future<GamePluginDetail> getPlugin(
    String pluginId, {
    required String extensionId,
  }) async {
    final data = await _api.get<Map<String, dynamic>>(
      '/api/game-center/plugins/$pluginId',
      queryParameters: {'extensionId': extensionId},
    );
    if (data == null) {
      throw StateError('Game Center plugin detail is empty');
    }
    return GamePluginDetail.fromJson(data);
  }

  Future<GameCenterRuntimeList> listRuntimes({
    int page = 1,
    int pageSize = 100,
    String? pluginId,
    String? status,
  }) async {
    final data = await _api.get<Map<String, dynamic>>(
      '/api/game-center/runtimes',
      queryParameters: <String, dynamic>{
        'page': page,
        'pageSize': pageSize,
        if (pluginId != null && pluginId.trim().isNotEmpty) 'pluginId': pluginId.trim(),
        if (status != null && status.trim().isNotEmpty) 'status': status.trim(),
      },
    );
    return GameCenterRuntimeList.fromJson(data ?? const <String, dynamic>{});
  }

  Future<GameRuntimeDetail> getRuntime(String runtimeId, {String? pluginId}) async {
    final data = await _api.get<Map<String, dynamic>>(
      '/api/game-center/runtimes/$runtimeId',
      queryParameters: <String, dynamic>{
        if (pluginId != null && pluginId.trim().isNotEmpty) 'pluginId': pluginId.trim(),
      },
    );
    if (data == null) {
      throw StateError('Game Center runtime detail is empty');
    }
    return GameRuntimeDetail.fromJson(data);
  }

  Future<GameRuntimeServicesResponse> getRuntimeServices(String runtimeId) async {
    final data = await _api.get<Map<String, dynamic>>('/api/game-center/runtimes/$runtimeId/services');
    final items = ((data?['items'] as List?) ?? const <dynamic>[])
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList(growable: false);
    return GameRuntimeServicesResponse(
      services: items.map(GameServiceSummary.fromJson).toList(growable: false),
    );
  }

  Future<void> bindAgentContext(
    String runtimeId, {
    String? serviceId,
    String? characterId,
    String? conversationId,
    String? channel = 'mobile',
    String? sessionId = 'game-center',
  }) async {
    await _api.post<void>(
      '/api/game-center/runtimes/${Uri.encodeComponent(runtimeId)}/agent-context',
      data: <String, dynamic>{
        if (serviceId != null && serviceId.trim().isNotEmpty) 'serviceId': serviceId.trim(),
        if (characterId != null && characterId.trim().isNotEmpty) 'characterId': characterId.trim(),
        if (conversationId != null && conversationId.trim().isNotEmpty) 'conversationId': conversationId.trim(),
        if (channel != null && channel.trim().isNotEmpty) 'channel': channel.trim(),
        if (sessionId != null && sessionId.trim().isNotEmpty) 'sessionId': sessionId.trim(),
      },
    );
  }

  Future<GameRuntimeHealthResponse> getRuntimeHealth(String runtimeId) async {
    final data = await _api.get<Map<String, dynamic>>('/api/game-center/runtimes/$runtimeId/health');
    return GameRuntimeHealthResponse.fromJson(data ?? const <String, dynamic>{});
  }


  Future<HealthSummary> getPluginHealth(String pluginId) async {
    final data = await _api.get<Map<String, dynamic>>(
      '/api/game-center/plugins/${Uri.encodeComponent(pluginId)}/health',
    );
    return HealthSummary.fromJson(data ?? const <String, dynamic>{});
  }

  Future<ControlAuthority> getRuntimeAuthority(String runtimeId) async {
    final data = await _api.get<Map<String, dynamic>>(
      '/api/game-center/runtimes/${Uri.encodeComponent(runtimeId)}/authority',
    );
    if (data == null) throw StateError('Game Center authority response is empty');
    return ControlAuthority.fromJson(data);
  }

  Future<ControlAuthority> getAuthority(String runtimeId) async {
    final data = await _api.get<Map<String, dynamic>>(
      '/api/game-center/authority',
      queryParameters: {'runtimeId': runtimeId},
    );
    if (data == null) throw StateError('Game Center authority response is empty');
    return ControlAuthority.fromJson(data);
  }

  Future<HandshakeSummary> getHandshake(String runtimeId) async {
    final data = await _api.get<Map<String, dynamic>>(
      '/api/game-center/handshake',
      queryParameters: {'runtimeId': runtimeId},
    );
    return HandshakeSummary.fromJson(data ?? const <String, dynamic>{});
  }

  Future<bool> developerAccess() async {
    try {
      final data = await _api.get<Map<String, dynamic>>(
        '/api/game-center/developer-access',
      );
      return data?['enabled'] == true;
    } catch (_) {
      return false;
    }
  }

  Future<List<GameHostPendingApproval>> listPendingApprovals() async {
    final data = await _api.get<Map<String, dynamic>>('/api/game-center/approvals');
    final items = ((data?['items'] as List?) ?? const <dynamic>[])
        .whereType<Map>()
        .map((item) => GameHostPendingApproval.fromJson(Map<String, dynamic>.from(item)))
        .where((item) => item.id.isNotEmpty && item.status == 'pending')
        .toList(growable: false);
    return items;
  }

  Future<void> resolveApproval(String approvalId, {required bool approve}) async {
    final action = approve ? 'approve' : 'reject';
    await _api.post<void>(
      '/api/game-center/approvals/${Uri.encodeComponent(approvalId)}/$action',
      data: <String, dynamic>{
        'reason': approve ? 'approved from mobile Game Center' : 'rejected from mobile Game Center',
      },
    );
  }

  Future<dynamic> invokeServiceRpc(
    String runtimeId,
    String serviceId, {
    required String method,
    Object? payload,
    int timeoutMs = 30000,
  }) {
    return _api.postPayload<dynamic>(
      '/api/game-center/runtimes/${Uri.encodeComponent(runtimeId)}/services/${Uri.encodeComponent(serviceId)}/rpc',
      data: {
        'method': method,
        if (payload != null) 'payload': payload,
        'timeoutMs': timeoutMs,
      },
    );
  }

  Future<GamePluginHandshakeResponse> handshake(String runtimeId) async {
    final data = await _api.get<Map<String, dynamic>>('/api/game-center/runtimes/$runtimeId/handshake');
    final json = data ?? const <String, dynamic>{};
    final state = (json['handshakeState'] ?? '').toString().toLowerCase();
    return GamePluginHandshakeResponse(
      accepted: (json['ready'] as bool?) ?? state == 'accepted' || state == 'ready' || state == 'completed',
      protocol: (json['protocol'] ?? '').toString().isEmpty ? null : json['protocol'].toString(),
      error: state == 'rejected' || state == 'failed' ? state : null,
    );
  }

  Future<GameCenterHealthResponse> health() async {
    final data = await _api.get<Map<String, dynamic>>('/api/game-center/health');
    return GameCenterHealthResponse.fromJson(data ?? const <String, dynamic>{});
  }

  Future<bool> enablePlugin(String extensionId) async {
    final data = await _api.post<Map<String, dynamic>>('/api/game-center/extensions/$extensionId/enable');
    return _packageMutationSucceeded(data);
  }

  Future<bool> disablePlugin(String extensionId) async {
    final data = await _api.post<Map<String, dynamic>>('/api/game-center/extensions/$extensionId/disable');
    return _packageMutationSucceeded(data);
  }

  Future<RuntimeMutationResult> startRuntime(String runtimeId) async {
    return _runtimeMutation(runtimeId, 'start');
  }

  Future<RuntimeMutationResult> stopRuntime(String runtimeId) async {
    return _runtimeMutation(runtimeId, 'stop');
  }

  Future<RuntimeMutationResult> restartRuntime(String runtimeId) async {
    return _runtimeMutation(runtimeId, 'restart');
  }

  Future<ControlMutationResult> takeover(String runtimeId) async {
    final data = await _api.post<Map<String, dynamic>>('/api/game-center/runtimes/$runtimeId/takeover');
    return ControlMutationResult.fromJson(data ?? const <String, dynamic>{});
  }

  Future<ControlMutationResult> release(
    String runtimeId, {
    String targetMode = 'observe',
    int expectedEpoch = 0,
  }) async {
    final data = await _api.post<Map<String, dynamic>>(
      '/api/game-center/runtimes/$runtimeId/release',
      data: {
        'targetMode': targetMode,
        'expectedEpoch': expectedEpoch,
      },
    );
    return ControlMutationResult.fromJson(data ?? const <String, dynamic>{});
  }

  Future<bool> emergencyStop(String runtimeId) async {
    final data = await _api.post<Map<String, dynamic>>('/api/game-center/runtimes/$runtimeId/emergency-stop');
    if (data == null) return true;
    final criticalFailure = data['CriticalFailure'] ?? data['criticalFailure'];
    final state = (data['State'] ?? data['state'] ?? '').toString().toLowerCase();
    if (criticalFailure is bool && criticalFailure) return false;
    return state.isEmpty || state == 'completed' || state == 'complete' || state == 'stopped';
  }

  Future<bool> rearm(String runtimeId) async {
    await _api.post('/api/game-center/runtimes/$runtimeId/rearm');
    return true;
  }

  Future<RuntimeMutationResult> _runtimeMutation(String runtimeId, String operation) async {
    final data = await _api.post<Map<String, dynamic>>('/api/game-center/runtimes/$runtimeId/$operation');
    if (data == null) throw StateError('Game Center runtime mutation response is empty');
    return RuntimeMutationResult.fromJson(data);
  }

  bool _packageMutationSucceeded(Map<String, dynamic>? data) {
    if (data == null) return true;
    final state = (data['state'] ?? '').toString().toLowerCase();
    return state.isEmpty || !{'failed', 'error', 'rejected'}.contains(state);
  }
}
