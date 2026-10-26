import 'dart:convert';
import 'dart:math';
import 'package:dio/dio.dart';
import '../backend_transport/backend_service_api.dart';
import '../backend_transport/errors/backend_transport_error.dart';
import '../models/memory.dart';
import 'owned_memory_scope.dart';

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
  final Map<String, Map<String, dynamic>> _pageScopes = {};
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
    if (role.isNotEmpty && scope['roleId'] != role) throw StateError('记忆角色已变化');
    final allView = scope['memoryViewAll'] == true;
    final rows = <Map<String, dynamic>>[];
    final seen = <String>{};
    for (final raw in value['resources'] as List) {
      final kind = raw is Map ? (raw['kind'] ?? '').toString() : '';
      final partition = raw is Map ? raw['roleId'] : null;
      final memory = isMemoryKind(kind);
      final global =
          !allView &&
          scope['memoryRoleId'] != globalMemoryRole &&
          partition == globalMemoryRole &&
          memory;
      if (raw is! Map ||
          raw['ownerId'] != scope['resourceOwnerId'] ||
          partition is! String ||
          partition.isEmpty ||
          partition != memoryStorageRole(scope, kind) &&
              !global &&
              !(allView && memory) ||
          raw['id'] is! String ||
          (raw['id'] as String).isEmpty ||
          raw['revision'] is! int ||
          raw['revision'] < 1 ||
          !seen.add('$kind/${raw['id']}'))
        throw StateError('记忆资源归属或版本无效');
      rows.add(
        Map<String, dynamic>.unmodifiable({
          ...Map<String, dynamic>.from(raw),
          'executionScope': memory
              ? memoryRecordScope(scope, partition)
              : scope,
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
    int retentionLevel = 0,
    bool? pinned,
    String decayState = '',
    String cursor = '',
    String memoryView = '',
    int limit = 128,
    Map<String, dynamic>? original,
  }) async {
    if (cursor.isNotEmpty && original == null) throw StateError('分页缺少原始记忆范围');
    if (limit < 1 || limit > 128) throw ArgumentError('记忆分页参数无效');
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
        if (retentionLevel > 0) 'retentionLevel': retentionLevel,
        if (pinned != null) 'pinned': pinned,
        if (decayState.isNotEmpty) 'decayState': decayState,
        'cursor': cursor,
        'limit': limit,
        if (memoryView.isNotEmpty) 'memoryView': memoryView,
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

  Future<List<MemoryDto>> listMemories({
    String? characterId,
    String? keyword,
    String? memoryType,
    int? retentionLevel,
    String? decayState,
    bool? pinned,
    String? sort,
    int page = 1,
    int pageSize = 200,
  }) async {
    if (page < 1 || pageSize < 1) throw ArgumentError('记忆分页参数无效');
    final role = characterId ?? '';
    final view = role.isEmpty ? 'all' : 'role:$role';
    final rows = <MemoryDto>[];
    var cursor = '';
    Map<String, dynamic>? original;
    final visited = <String>{};
    final target = page * pageSize;
    do {
      final result = await query(
        role,
        query: keyword ?? '',
        memoryType: memoryType ?? '',
        sort: sort ?? '',
        retentionLevel: retentionLevel ?? 0,
        pinned: pinned,
        decayState: decayState ?? '',
        cursor: cursor,
        memoryView: view,
        original: original,
      );
      rows.addAll(
        (result['resources'] as List).cast<Map<String, dynamic>>().map(
          _memoryDto,
        ),
      );
      original = result;
      cursor = (result['nextCursor'] ?? '').toString();
      if (cursor.isNotEmpty && !visited.add(cursor))
        throw StateError('记忆分页游标重复');
      if (visited.length > 256) throw StateError('记忆结果超过分页上限');
    } while (cursor.isNotEmpty && rows.length < target);
    final offset = (page - 1) * pageSize;
    if (offset >= rows.length) return const <MemoryDto>[];
    return rows.sublist(offset, target > rows.length ? rows.length : target);
  }

  Future<MemoryPage> pageMemories({
    String cursor = '',
    int limit = 10,
    String? characterId,
    String? sort,
  }) async {
    if (limit < 1 || limit > 128) throw ArgumentError('记忆分页参数无效');
    final role = characterId ?? '';
    final original = cursor.isEmpty ? null : _pageScopes[cursor];
    if (cursor.isNotEmpty && original == null) throw StateError('记忆分页缺少原始记忆范围');
    final page = await query(
      role,
      sort: sort ?? '',
      cursor: cursor,
      limit: limit,
      memoryView: role.isEmpty ? 'all' : 'role:$role',
      original: original,
    );
    final next = (page['nextCursor'] ?? '').toString();
    if (next.isNotEmpty) {
      _pageScopes[next] = page;
      if (_pageScopes.length > 64) _pageScopes.remove(_pageScopes.keys.first);
    }
    return MemoryPage(
      items: List<MemoryDto>.unmodifiable(
        (page['resources'] as List).cast<Map<String, dynamic>>().map(
          _memoryDto,
        ),
      ),
      nextCursor: next,
    );
  }

  Future<List<MemoryCandidateDto>> listMemoryCandidates({
    String? characterId,
  }) async {
    final role = characterId ?? '';
    final page = await query(
      role,
      candidates: true,
      memoryView: role.isEmpty ? 'all' : 'role:$role',
    );
    return (page['resources'] as List)
        .cast<Map<String, dynamic>>()
        .map(_memoryCandidateDto)
        .toList(growable: false);
  }

  static MemoryDto _memoryDto(Map<String, dynamic> resource) {
    if (resource['kind'] != 'memory' || resource['body'] is! Map)
      throw StateError('记忆资源类型无效');
    final body = Map<String, dynamic>.from(resource['body'] as Map);
    final content = body['content'];
    final metadata = content is Map
        ? Map<String, dynamic>.from(content)
        : <String, dynamic>{'value': content ?? ''};
    return MemoryDto.fromJson({
      ...metadata,
      ...body,
      'id': resource['id'],
      'characterId': resource['roleId'],
      'scope': resource['roleId'] == globalMemoryRole ? 'user' : 'character',
      'value':
          metadata['value'] ?? metadata['text'] ?? metadata['summary'] ?? '',
      'updatedAt': resource['updatedAt'] ?? '',
      'memoryStrength': body['effectiveStrength'] ?? body['memoryStrength'],
      'retentionLevel':
          body['effectiveRetentionLevel'] ?? body['retentionLevel'],
      'decayState': body['effectiveDecayState'] ?? body['decayState'],
    });
  }

  static MemoryCandidateDto _memoryCandidateDto(Map<String, dynamic> resource) {
    if (resource['kind'] != 'checkpoint' || resource['body'] is! Map)
      throw StateError('候选记忆资源类型无效');
    final body = Map<String, dynamic>.from(resource['body'] as Map);
    return MemoryCandidateDto.fromJson({
      ...body,
      'id': resource['id'],
      'characterId': resource['roleId'],
    });
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
        (responseScope['turnId'] ?? '').toString() !=
            (scope['turnId'] ?? '').toString() ||
        (responseScope['executionId'] ?? '').toString() !=
            (scope['executionId'] ?? '').toString() ||
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
