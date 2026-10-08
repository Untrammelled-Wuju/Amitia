import 'dart:convert';
import 'dart:math';
import 'package:dio/dio.dart';
import '../backend_transport/backend_service_api.dart';
import '../backend_transport/errors/backend_transport_error.dart';

class OwnedMemoryConflict implements Exception {
  final List<Map<String, dynamic>> resources;
  const OwnedMemoryConflict(this.resources);
  @override
  String toString() => '同一角色存在同键记忆，请确认原记录后选择处理方式';
}

class DeviceOwnedMemoryService {
  final BackendServiceApi api;
  final bool Function() isCurrent;
  final Object _authority = Object();
  DeviceOwnedMemoryService(this.api, {required this.isCurrent});

  static String scopeStamp(Map scope) => jsonEncode(
    (scope.keys
            .where(
              (key) =>
                  !const ['requestId', 'turnId', 'executionId'].contains(key),
            )
            .toList()
          ..sort())
        .map((key) => [key, scope[key]])
        .toList(),
  );
  static String requestId() {
    final values = List.generate(16, (_) => Random.secure().nextInt(256));
    values[6] = (values[6] & 15) | 64;
    values[8] = (values[8] & 63) | 128;
    final value = values
        .map((byte) => byte.toRadixString(16).padLeft(2, '0'))
        .join();
    return '${value.substring(0, 8)}-${value.substring(8, 12)}-${value.substring(12, 16)}-${value.substring(16, 20)}-${value.substring(20)}';
  }

  Future<Map<String, dynamic>> _state([Map<String, dynamic>? original]) async {
    if (!isCurrent() ||
        api.generation < 1 ||
        original != null &&
            !identical(original['_memoryAuthority'], _authority))
      throw StateError('记忆服务归属已变化，请重新打开原列表');
    final state = await api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    if (!isCurrent() ||
        state?['coordinationAvailable'] != true ||
        state?['policy'] is! Map ||
        state?['coreId'] is! String)
      throw StateError('记忆 Core 服务尚未就绪');
    if (original != null) _validateScope(original['executionScope'], state!);
    return state!;
  }

  void _validateScope(dynamic scope, Map state) {
    if (scope is! Map ||
        scope['coreId'] != state['coreId'] ||
        scope['roleId'] is! String ||
        scope['resourceOwnerId'] is! String ||
        const [
          'providerEpoch',
          'modeRevision',
          'permissionRevision',
          'coordinated',
        ].any((key) => scope[key] != state['policy'][key]))
      throw StateError('记忆原始执行范围已变化');
    for (final field in const [
      'spaceId',
      'initiatorDeviceId',
      'targetDeviceId',
      'authorizationRealm',
      'roleId',
      'roleOwnerId',
      'resourceOwnerId',
    ]) {
      if (scope[field] is! String || (scope[field] as String).isEmpty)
        throw StateError('记忆执行范围缺少数据归属');
    }
    for (final field in const [
      'providerEpoch',
      'targetProviderEpoch',
      'modeRevision',
      'permissionRevision',
      'targetPermissionRevision',
      'roleRevision',
    ]) {
      if (scope[field] is! int || scope[field] < 1)
        throw StateError('记忆权限或角色版本无效');
    }
    if (scope['coordinated'] is! bool) throw StateError('记忆统筹状态无效');
  }

  Map<String, dynamic> _result(
    Map<String, dynamic>? value,
    Map state,
    String role,
  ) {
    if (value == null ||
        value['resources'] is! List ||
        (value['resources'] as List).length > 4096)
      throw StateError('记忆结果无效');
    _validateScope(value['executionScope'], state);
    final scope = Map<String, dynamic>.unmodifiable(
      value['executionScope'] as Map,
    );
    if (scope['roleId'] != role) throw StateError('记忆角色已变化');
    final rows = <Map<String, dynamic>>[];
    final seen = <String>{};
    for (final raw in value['resources'] as List) {
      if (raw is! Map ||
          raw['ownerId'] != scope['resourceOwnerId'] ||
          raw['roleId'] != role ||
          raw['id'] is! String ||
          (raw['id'] as String).isEmpty ||
          raw['revision'] is! int ||
          raw['revision'] < 1 ||
          !seen.add('${raw['kind']}/${raw['id']}'))
        throw StateError('记忆资源归属或版本无效');
      rows.add(
        Map<String, dynamic>.unmodifiable({
          ...Map<String, dynamic>.from(raw),
          'executionScope': scope,
          '_memoryAuthority': _authority,
        }),
      );
    }
    return Map<String, dynamic>.unmodifiable({
      ...value,
      'resources': List<Map<String, dynamic>>.unmodifiable(rows),
      'executionScope': scope,
      '_memoryAuthority': _authority,
    });
  }

  Future<Map<String, dynamic>> query(
    String characterId, {
    bool candidates = false,
    String query = '',
    String mode = 'keyword',
    String memoryType = '',
    String source = '',
    String sort = '',
    String cursor = '',
    Map<String, dynamic>? original,
  }) async {
    if (cursor.isNotEmpty && original == null) throw StateError('分页缺少原始记忆范围');
    final state = await _state(original);
    final result = await api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/${candidates ? 'memory-candidates' : 'memories'}',
      queryParameters: {
        'characterId': characterId,
        'query': query,
        'mode': mode,
        'memoryType': memoryType,
        'source': source,
        'sort': sort,
        'cursor': cursor,
        'limit': 128,
      },
    );
    final current = await _state(original);
    if (state['coreId'] != current['coreId'] ||
        const [
          'providerEpoch',
          'modeRevision',
          'permissionRevision',
          'coordinated',
        ].any((key) => state['policy'][key] != current['policy'][key]))
      throw StateError('记忆列表加载期间服务状态已变化');
    final page = _result(result, state, characterId);
    if (original != null &&
        scopeStamp(page['executionScope'] as Map) !=
            scopeStamp(original['executionScope'] as Map))
      throw StateError('记忆列表原范围已变化');
    if (cursor.isNotEmpty && page['nextCursor'] == cursor)
      throw StateError('记忆分页游标重复');
    return page;
  }

  Future<Map<String, dynamic>> manage(
    Map<String, dynamic> original,
    Map<String, dynamic> input, {
    bool candidates = false,
    String? operationId,
  }) async {
    if (candidates &&
        input['action'] != 'generate' &&
        (original['id'] != input['id'] ||
            original['revision'] != input['expectedRevision']))
      throw StateError('候选记忆 ID 或版本与原记录不一致');
    final state = await _state(original);
    final scope = original['executionScope'] as Map;
    final id = operationId ?? requestId();
    Map<String, dynamic>? result;
    try {
      result = await api.post<Map<String, dynamic>>(
        '/api/device-mesh/v1/business/${candidates ? 'memory-candidates' : 'memories'}/manage',
        data: {
          ...input,
          'requestId': id,
          'characterId': scope['roleId'],
          'targetDeviceId': scope['targetDeviceId'],
          'expectedExecutionScope': scope,
        },
      );
    } on BackendTransportError catch (error) {
      await _state(original);
      final cause = error.cause;
      final body = cause is DioException ? cause.response?.data : null;
      if (error.statusCode == 409 && body is Map && body['conflicts'] is List) {
        final conflicts = _result(
          {'resources': body['conflicts'], 'executionScope': scope},
          state,
          scope['roleId'] as String,
        );
        throw OwnedMemoryConflict(
          (conflicts['resources'] as List).cast<Map<String, dynamic>>(),
        );
      }
      rethrow;
    }
    await _state(original);
    final value = _result(result, state, scope['roleId'] as String);
    final responseScope = value['executionScope'] as Map;
    final ack = value['acknowledgement'];
    final generated = candidates && input['action'] == 'generate';
    final proof =
        'checkpoint/${candidates ? 'memory-candidate-operation' : 'memory-management'}/$id';
    final expected = <String, dynamic>{proof: generated ? 2 : 1};
    for (final row in value['resources'] as List<Map<String, dynamic>>)
      expected['${row['kind']}/${row['id']}'] = row['revision'];
    if (value['saved'] != true ||
        scopeStamp(responseScope) != scopeStamp(scope) ||
        responseScope['requestId'] != id ||
        (responseScope['turnId'] ?? '').toString().isEmpty ||
        (responseScope['executionId'] ?? '').toString().isEmpty ||
        ack is! Map ||
        ack['ownerId'] != scope['resourceOwnerId'] ||
        ack['requestId'] != (generated ? '$id|candidate-result' : id) ||
        ack['versions'] is! Map ||
        jsonEncode((ack['versions'] as Map).keys.toList()..sort()) !=
            jsonEncode(expected.keys.toList()..sort()) ||
        expected.entries.any(
          (entry) => ack['versions'][entry.key] != entry.value,
        ))
      throw StateError('记忆所有者尚未精确确认保存结果');
    return value;
  }
}
