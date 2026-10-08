import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

import '../backend_transport/backend_service_api.dart';
import 'device_owned_speech_service.dart';

String ownedRealtimeRequestId() {
  final random = Random.secure();
  final bytes = List.generate(16, (_) => random.nextInt(256));
  bytes[6] = (bytes[6] & 15) | 64;
  bytes[8] = (bytes[8] & 63) | 128;
  final hex = bytes.map((v) => v.toRadixString(16).padLeft(2, '0')).join();
  return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}';
}

String ownedRealtimeAuthority(Map scope) {
  final keys =
      scope.keys
          .map((v) => v.toString())
          .where(
            (v) => !const ['requestId', 'turnId', 'executionId'].contains(v),
          )
          .toList()
        ..sort();
  for (final key in const [
    'spaceId',
    'initiatorDeviceId',
    'targetDeviceId',
    'coreId',
    'roleId',
    'roleOwnerId',
    'resourceOwnerId',
    'authorizationRealm',
  ]) {
    if (scope[key] is! String || (scope[key] as String).isEmpty)
      throw StateError('通话缺少角色与数据归属');
  }
  for (final key in const [
    'providerEpoch',
    'targetProviderEpoch',
    'modeRevision',
    'permissionRevision',
    'targetPermissionRevision',
    'roleRevision',
  ]) {
    if (scope[key] is! int || scope[key] < 1 || scope[key] > 9007199254740991)
      throw StateError('通话缺少有效权限版本');
  }
  if (scope['coordinated'] is! bool) throw StateError('通话缺少统筹状态');
  return jsonEncode({for (final key in keys) key: scope[key]});
}

class DeviceOwnedRealtimeService {
  final BackendServiceApi api;
  final bool Function() isCurrent;
  final Map<String, dynamic> scope;
  final String characterId;
  final String conversationId;
  final Map<String, String>? conversationOrigin;
  final String historicalRoleId;
  bool _closed = false;

  DeviceOwnedRealtimeService({
    required this.api,
    required this.isCurrent,
    required Map<String, dynamic> scope,
    required this.characterId,
    required this.conversationId,
    this.conversationOrigin,
    this.historicalRoleId = '',
  }) : scope = Map.unmodifiable(scope) {
    ownedRealtimeAuthority(scope);
    if (scope['roleId'] != characterId) throw StateError('通话角色与原范围不一致');
  }

  void assertConnection() {
    if (_closed || !isCurrent()) throw StateError('Core 连接已切换，实时通话已停止');
  }

  void close() {
    _closed = true;
  }

  Map<String, dynamic> _data(Map<String, dynamic>? result) {
    if (result == null) throw StateError('Core 未返回有效通话数据');
    return result['data'] is Map
        ? Map<String, dynamic>.from(result['data'] as Map)
        : result;
  }

  Future<void> assertCurrent() async {
    assertConnection();
    final result = _data(
      await api.get<Map<String, dynamic>>(
        '/api/device-mesh/v1/business/data',
        queryParameters: {
          'kind': 'memory',
          'characterId': characterId,
          'targetDeviceId': scope['targetDeviceId'],
        },
      ),
    );
    assertConnection();
    if (result['executionScope'] is! Map ||
        ownedRealtimeAuthority(result['executionScope'] as Map) !=
            ownedRealtimeAuthority(scope))
      throw StateError('Core、角色、统筹模式或权限已变化，通话已中断');
  }

  Future<Map<String, dynamic>> ticket() async {
    await assertCurrent();
    final result = _data(
      await api.post<Map<String, dynamic>>(
        '/api/device-mesh/v1/business/realtime/tickets',
        data: {
          'requestId': ownedRealtimeRequestId(),
          'characterId': characterId,
          'targetDeviceId': scope['targetDeviceId'],
          'conversationId': conversationId,
          if (conversationOrigin != null)
            'conversationOrigin': conversationOrigin,
          if (historicalRoleId.isNotEmpty) 'historicalRoleId': historicalRoleId,
          'expectedExecutionScope': scope,
        },
      ),
    );
    assertConnection();
    validateScope(result);
    if (result['wsPath'] != '/api/device-mesh/v1/business/realtime/session' ||
        result['ticket'] is! String ||
        !RegExp(r'^[A-Za-z0-9_-]{43}$').hasMatch(result['ticket'] as String))
      throw StateError('通话票据无效');
    return Map.unmodifiable(result);
  }

  void validateScope(Map response) {
    assertConnection();
    if (response['executionScope'] is! Map ||
        ownedRealtimeAuthority(response['executionScope'] as Map) !=
            ownedRealtimeAuthority(scope))
      throw StateError('旧通话结果已丢弃');
  }

  void validateCompleted(Map result, String requestId) {
    validateScope(result);
    final execution = result['executionScope'] as Map;
    if (result['saved'] != true ||
        result['requestId'] != requestId ||
        execution['requestId'] != requestId ||
        result['userRevision'] != 2 ||
        result['conversationId'] is! String ||
        (result['conversationId'] as String).isEmpty ||
        result['transcription'] is! String ||
        (result['transcription'] as String).isEmpty)
      throw StateError('当前对话或语音转写尚未确认保存');
  }

  Future<OwnedSpeechAudio> audio(
    Map<String, dynamic> result,
    String requestId,
  ) async {
    assertConnection();
    validateScope(result);
    final verifier = DeviceOwnedSpeechService(
      api: api,
      currentApi: () => api,
      providerKey: () => scope['coreId'] as String,
    );
    return verifier.verifyResponse(
      result: result,
      scope: scope,
      requestId: requestId,
      assertCurrent: assertCurrent,
    );
  }

  Map<String, dynamic> image(Uint8List bytes, String mime) {
    assertConnection();
    if (bytes.isEmpty ||
        bytes.length > 1048576 ||
        !const ['image/jpeg', 'image/png'].contains(mime))
      throw StateError('实时视觉帧必须为 PNG/JPEG，最大 1 MiB');
    return {
      'kind': 'image',
      'name': 'realtime.${mime == 'image/png' ? 'png' : 'jpg'}',
      'mimeType': mime,
      'data': base64Encode(bytes),
      'sha256': sha256.convert(bytes).toString(),
    };
  }
}
