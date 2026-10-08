import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

import '../backend_transport/backend_service_api.dart';

const _speechAuthorityFields = [
  'spaceId',
  'initiatorDeviceId',
  'targetDeviceId',
  'coreId',
  'providerEpoch',
  'targetProviderEpoch',
  'coordinated',
  'modeRevision',
  'permissionRevision',
  'targetPermissionRevision',
  'roleId',
  'roleRevision',
  'authorizationRealm',
  'roleOwnerId',
  'resourceOwnerId',
];

class OwnedSpeechAudio {
  final Uint8List _bytes;
  final String sha256Hash;
  final String requestId;
  final Map<String, dynamic> executionScope;
  final Future<void> Function() assertCurrent;

  OwnedSpeechAudio._(
    this._bytes,
    this.sha256Hash,
    this.requestId,
    Map<String, dynamic> scope,
    this.assertCurrent,
  ) : executionScope = Map.unmodifiable(scope);

  Uint8List get bytes => Uint8List.fromList(_bytes);
}

class DeviceOwnedSpeechService {
  final BackendServiceApi api;
  final BackendServiceApi? Function() currentApi;
  final String Function() providerKey;

  const DeviceOwnedSpeechService({
    required this.api,
    required this.currentApi,
    required this.providerKey,
  });

  Map<String, dynamic> _data(Map<String, dynamic>? result) {
    if (result == null) throw StateError('Core 未返回有效的语音业务结果');
    return result['data'] is Map
        ? Map<String, dynamic>.from(result['data'] as Map)
        : result;
  }

  String _authority(Map scope) {
    for (final field in _speechAuthorityFields) {
      final value = scope[field];
      if (field == 'coordinated') {
        if (value is! bool) throw StateError('语音执行范围缺少统筹状态');
      } else if (field.endsWith('Revision') || field.endsWith('Epoch')) {
        if (value is! int || value < 1 || value > 9007199254740991) {
          throw StateError('语音执行范围缺少有效权限版本');
        }
      } else if (value is! String || value.isEmpty || value.length > 512) {
        throw StateError('语音执行范围缺少角色或数据归属');
      }
    }
    return jsonEncode(_speechAuthorityFields.map((key) => scope[key]).toList());
  }

  String _requestId() {
    final random = Random.secure();
    final bytes = List.generate(16, (_) => random.nextInt(256));
    bytes[6] = (bytes[6] & 15) | 64;
    bytes[8] = (bytes[8] & 63) | 128;
    final hex = bytes
        .map((value) => value.toRadixString(16).padLeft(2, '0'))
        .join();
    return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-'
        '${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}';
  }

  Future<OwnedSpeechAudio> synthesize(String characterId, String text) async {
    if (characterId.trim().isEmpty ||
        text.trim().isEmpty ||
        utf8.encode(text).length > 8192) {
      throw StateError('请选择有效角色，朗读文本不能超过 8192 字节');
    }
    final initialApi = currentApi();
    final binding = providerKey();
    if (binding.isEmpty || initialApi == null || initialApi.generation <= 0) {
      throw StateError('云端语音连接尚未就绪');
    }
    void assertConnection() {
      if (providerKey() != binding || !identical(currentApi(), initialApi)) {
        throw StateError('Core 连接已切换，旧语音结果已丢弃');
      }
    }

    final state = _data(
      await api.get<Map<String, dynamic>>(
        '/api/device-mesh/v1/coordination/me',
      ),
    );
    assertConnection();
    if (state['coordinationAvailable'] != true || state['policy'] is! Map) {
      throw StateError('云端语音业务尚未就绪');
    }
    final queried = _data(
      await api.get<Map<String, dynamic>>(
        '/api/device-mesh/v1/business/data',
        queryParameters: {'kind': 'memory', 'characterId': characterId},
      ),
    );
    assertConnection();
    if (queried['executionScope'] is! Map) throw StateError('当前角色缺少语音调用范围');
    final scope = Map<String, dynamic>.from(queried['executionScope'] as Map);
    _authority(scope);
    if (scope['coreId'] != state['coreId'] || scope['roleId'] != characterId) {
      throw StateError('服务提供者或角色已变化，请重新加载');
    }

    Future<void> assertCurrent() async {
      assertConnection();
      final now = _data(
        await api.get<Map<String, dynamic>>(
          '/api/device-mesh/v1/coordination/me',
        ),
      );
      assertConnection();
      final policy = now['policy'];
      if (now['coordinationAvailable'] != true ||
          now['coreId'] != scope['coreId'] ||
          policy is! Map ||
          policy['deviceId'] != scope['initiatorDeviceId'] ||
          policy['coordinated'] != scope['coordinated'] ||
          policy['providerEpoch'] != scope['providerEpoch'] ||
          policy['modeRevision'] != scope['modeRevision'] ||
          policy['permissionRevision'] != scope['permissionRevision'] ||
          policy['deviceId'] != scope['targetDeviceId'] ||
          policy['providerEpoch'] != scope['targetProviderEpoch'] ||
          policy['permissionRevision'] != scope['targetPermissionRevision']) {
        throw StateError('Core、统筹模式或权限已变化，语音已中断');
      }
      final roles = _data(
        await api.get<Map<String, dynamic>>(
          '/api/device-mesh/v1/business/roles',
        ),
      );
      assertConnection();
      final currentRole = (roles['roles'] as List? ?? [])
          .whereType<Map>()
          .where(
            (role) =>
                role['id'] == scope['roleId'] &&
                role['revision'] == scope['roleRevision'],
          );
      final roleScope = roles['executionScope'];
      if (currentRole.length != 1 ||
          roles['roleOwnerId'] != scope['roleOwnerId'] ||
          roleScope is! Map ||
          _speechAuthorityFields
              .where((field) => field != 'roleId' && field != 'roleRevision')
              .any((field) => roleScope[field] != scope[field])) {
        throw StateError('角色已删除、变更或归属已变化，语音已中断');
      }
    }

    await assertCurrent();
    final requestId = _requestId();
    final result = _data(
      await api.post<Map<String, dynamic>>(
        '/api/device-mesh/v1/business/speech',
        data: {
          'requestId': requestId,
          'characterId': characterId,
          'text': text,
          'expectedExecutionScope': scope,
        },
      ),
    );
    assertConnection();
    return verifyResponse(
      result: result,
      scope: scope,
      requestId: requestId,
      assertCurrent: assertCurrent,
    );
  }

  Future<OwnedSpeechAudio> verifyResponse({
    required Map<String, dynamic> result,
    required Map<String, dynamic> scope,
    required String requestId,
    required Future<void> Function() assertCurrent,
  }) async {
    final authority = _authority(scope);
    final responseScope = result['executionScope'];
    if (result['saved'] != true ||
        result['requestId'] != requestId ||
        responseScope is! Map ||
        _authority(responseScope) != authority ||
        responseScope['requestId'] != 'speech/$requestId' ||
        (responseScope['turnId'] ?? '').toString().isEmpty ||
        (responseScope['executionId'] ?? '').toString().isEmpty) {
      throw StateError('语音所有者尚未确认保存，或执行范围已变化');
    }
    final digest = sha256
        .convert(
          utf8.encode(
            '${scope['coreId']}\u0000${scope['initiatorDeviceId']}\u0000$requestId',
          ),
        )
        .toString();
    final resourceId = 'speech/${digest.substring(0, 32)}';
    final acknowledgement = result['acknowledgement'];
    final versions = acknowledgement is Map
        ? acknowledgement['versions']
        : null;
    if (acknowledgement is! Map ||
        acknowledgement['ownerId'] != scope['resourceOwnerId'] ||
        acknowledgement['requestId'] != responseScope['requestId'] ||
        versions is! Map ||
        versions.length != 2 ||
        versions['tool-result/$resourceId'] != 1 ||
        versions['checkpoint/$resourceId'] != 2) {
      throw StateError('语音保存确认与数据所有者不一致');
    }
    final audio = result['audio'];
    if (audio is! Map ||
        audio['mime'] != 'audio/mpeg' ||
        audio['data'] is! String ||
        audio['sha256'] is! String ||
        !RegExp(r'^[a-f0-9]{64}$').hasMatch(audio['sha256'] as String)) {
      throw StateError('Core 返回的语音格式无效');
    }
    final encoded = audio['data'] as String;
    if (encoded.isEmpty ||
        encoded.length > 1398104 ||
        !RegExp(r'^[A-Za-z0-9+/]+={0,2}$').hasMatch(encoded)) {
      throw StateError('语音大小或编码无效');
    }
    final bytes = base64Decode(encoded);
    if (bytes.isEmpty ||
        bytes.length > 1048576 ||
        base64Encode(bytes) != encoded ||
        sha256.convert(bytes).toString() != audio['sha256']) {
      throw StateError('语音完整性校验失败');
    }
    await assertCurrent();
    return OwnedSpeechAudio._(
      bytes,
      audio['sha256'] as String,
      requestId,
      Map<String, dynamic>.from(responseScope),
      assertCurrent,
    );
  }
}
