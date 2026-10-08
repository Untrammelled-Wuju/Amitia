import 'dart:convert';
import 'dart:math';
import '../backend_transport/backend_service_api.dart';
import '../models/continuity.dart';

class ContinuityService {
  ContinuityService(this._api, {bool Function()? isBound})
    : _isBound = isBound ?? (() => false);

  final BackendServiceApi _api;
  final bool Function() _isBound;

  String selectedRole = '';
  final _threadRoles = <String, String>{};
  Map<String, dynamic>? _coordination;
  String _providerStamp = '';
  List<Map<String, dynamic>> roles = [];

  static String _scopeStamp(Map scope) => jsonEncode(
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

  Future<List<Map<String, dynamic>>> availableRoles() async {
    final current = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    if (current == null) throw StateError('无法读取持续事项的数据归属');
    final stamp = jsonEncode([current['coreId'], current['policy']]);
    if (_providerStamp.isNotEmpty && stamp != _providerStamp) {
      _threadRoles.clear();
      selectedRole = '';
    }
    _providerStamp = stamp;
    _coordination = current;
    if (current['coordinationAvailable'] != true) {
      roles = [];
      return roles;
    }
    final response = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/roles',
    );
    roles = (response?['roles'] as List? ?? [])
        .whereType<Map>()
        .map((value) => Map<String, dynamic>.from(value))
        .toList();
    final policy = current['policy'];
    if (selectedRole.isEmpty && policy is Map) {
      selectedRole = (policy['selectedRole'] ?? '').toString();
    }
    if (selectedRole.isEmpty && roles.length == 1) {
      selectedRole = roles.single['id'].toString();
    }
    return roles;
  }

  Future<String?> _ownedRole([String? requested]) async {
    await availableRoles();
    if (_coordination?['coordinationAvailable'] != true) {
      if (_isBound()) throw StateError('绑定 Core 尚未就绪，不能使用本机持续事项接口');
      return null;
    }
    final role = requested?.isNotEmpty == true ? requested! : selectedRole;
    if (role.isEmpty || !roles.any((value) => value['id'] == role)) {
      throw StateError('请先选择持续事项使用的有效角色');
    }
    return role;
  }

  Map<String, dynamic> _managementDocument(Map document, [Map? expected]) {
    final scope = document['managementExecutionScope'];
    if (scope is! Map ||
        document['ownerId'] != scope['resourceOwnerId'] ||
        document['thread']?['characterId'] != scope['roleId'] ||
        (expected != null && _scopeStamp(scope) != _scopeStamp(expected))) {
      throw StateError('持续事项的管理范围或数据归属无效，请重新加载');
    }
    return {
      ...Map<String, dynamic>.from(document),
      'persistedExecutionScope': document['executionScope'],
      'executionScope': Map<String, dynamic>.from(scope),
    };
  }

  Future<Map<String, dynamic>?> _ownedDocument(
    String id, {
    Map<String, dynamic>? expectedDocument,
  }) async {
    if (expectedDocument?['readOnly'] == true &&
        expectedDocument?['thread']?['id'] == id)
      return expectedDocument;
    final role = await _ownedRole(
      expectedDocument?['executionScope']?['roleId']?.toString(),
    );
    if (role == null) return null;
    if (expectedDocument == null || expectedDocument['thread']?['id'] != id)
      throw StateError('请从持续事项列表打开并确认原始归属');
    final document = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/continuity',
      queryParameters: {'id': id, 'characterId': role},
    );
    if (document == null || document['thread'] is! Map) {
      throw StateError('持续事项数据无效');
    }
    final current = _managementDocument(document);
    if (current['thread']?['id'] != id ||
        current['ownerId'] != expectedDocument['ownerId'] ||
        _scopeStamp(current['executionScope'] as Map) !=
            _scopeStamp(expectedDocument['executionScope'] as Map))
      throw StateError('原持续事项的服务、角色或权限已变化，请返回列表重新打开');
    _threadRoles[id] = (document['thread']['characterId'] ?? role).toString();
    return current;
  }

  Future<Map<String, dynamic>?> _ownedMutation(
    String action,
    Map<String, dynamic> data, [
    Map<String, dynamic>? current,
  ]) async {
    final role = await _ownedRole(
      (current?['thread']?['characterId'] ?? data['characterId'])?.toString(),
    );
    if (role == null) {
      if (current != null) throw StateError('原 Core 持续事项不能写入本机');
      return null;
    }
    if (action != 'create' && current?['thread']?['id'] != data['id'])
      throw StateError('持续事项编号与原归属不一致');
    final scope = current?['executionScope'];
    final policy = _coordination?['policy'];
    final roleValue = roles.where((item) => item['id'] == role).firstOrNull;
    if (scope is! Map ||
        current?['readOnly'] == true ||
        scope['coreId'] != _coordination?['coreId'] ||
        scope['roleId'] != role ||
        scope['roleRevision'] != roleValue?['revision'] ||
        policy is! Map ||
        [
          'providerEpoch',
          'modeRevision',
          'permissionRevision',
          'coordinated',
        ].any((key) => scope[key] != policy[key])) {
      throw StateError('原持续事项的服务、角色或权限已变化，请重新打开页面');
    }
    final requestId = base64UrlEncode(
      List<int>.generate(32, (_) => Random.secure().nextInt(256)),
    ).replaceAll('=', '');
    final response = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/continuity',
      data: {
        ...data,
        'action': action,
        'characterId': role,
        'requestId': requestId,
        'expectedExecutionScope': scope,
        'expectedRevision': current?['thread']?['revision'] ?? 0,
        'expectedCoreId': scope['coreId'],
        'expectedOwnerId': scope['resourceOwnerId'],
        'expectedModeRevision': scope['modeRevision'],
      },
    );
    final document = response?['document'];
    final ack = response?['acknowledgement'];
    if (document is! Map ||
        ack is! Map ||
        ack['requestId'] != requestId ||
        document['thread'] is! Map ||
        document['executionScope'] is! Map ||
        _scopeStamp(document['executionScope'] as Map) != _scopeStamp(scope) ||
        document['executionScope']['requestId'] != requestId ||
        document['thread']['id'] !=
            (current?['thread']?['id'] ?? 'continuity/$requestId') ||
        document['thread']['revision'] !=
            (current?['thread']?['revision'] ?? 0) + 1 ||
        document['ownerId'] != scope['resourceOwnerId'] ||
        ack['ownerId'] != document['ownerId'] ||
        ack['versions']?['continuity/${document['thread']['id']}'] !=
            document['thread']['revision'] ||
        ack['versions']?['checkpoint/continuity-operation/$requestId'] != 1) {
      throw StateError('持续事项的数据持有方尚未确认保存');
    }
    final result = Map<String, dynamic>.from(document);
    _threadRoles[document['thread']['id'].toString()] = role;
    return result;
  }

  Future<List<ContinuityThreadDto>> list({
    String query = '',
    String status = '',
    int limit = 100,
  }) async {
    final role = await _ownedRole();
    if (role != null) {
      final documents = <String, Map>{};
      final cursors = <String>{};
      var cursor = '';
      var scope = '';
      do {
        final page = await _api.get<Map<String, dynamic>>(
          '/api/device-mesh/v1/business/continuity',
          queryParameters: {
            'characterId': role,
            'pagination': '1',
            'cursor': cursor,
          },
        );
        if (page == null ||
            page['documents'] is! List ||
            page['executionScope'] is! Map) {
          throw StateError('持续事项分页数据无效');
        }
        final currentScope =
            Map<String, dynamic>.from(page['executionScope'] as Map)
              ..remove('requestId')
              ..remove('turnId')
              ..remove('executionId');
        final stamp = _scopeStamp(currentScope);
        if (scope.isNotEmpty && scope != stamp) {
          throw StateError('持续事项的数据归属已变化，请重新加载');
        }
        scope = stamp;
        for (final document in page['documents'] as List) {
          if (document is! Map || document['thread'] is! Map) {
            throw StateError('持续事项数据无效');
          }
          documents['${document['ownerId']}/${document['thread']['id']}'] =
              _managementDocument(document, page['executionScope'] as Map);
        }
        if (documents.length > 32768) throw StateError('持续事项数量超过加载上限，请缩小查询范围');
        cursor = (page['nextCursor'] ?? '').toString();
        if (cursor.isNotEmpty && !cursors.add(cursor)) {
          throw StateError('持续事项分页游标重复');
        }
      } while (cursor.isNotEmpty);
      final result = <ContinuityThreadDto>[];
      if (_coordination?['policy']?['coordinated'] == true) {
        final history = await _api.get<Map<String, dynamic>>(
          '/api/device-mesh/v1/business/historical-roles',
          queryParameters: {'characterId': role},
        );
        for (final oldRole
            in (history?['roles'] as List? ?? []).whereType<Map>()) {
          var historicalCursor = '';
          final visited = <String>{};
          do {
            final page = await _api.get<Map<String, dynamic>>(
              '/api/device-mesh/v1/business/data',
              queryParameters: {
                'kind': 'continuity',
                'management': '1',
                'characterId': role,
                'historicalRoleId': oldRole['id'],
                'historicalCursor': historicalCursor,
              },
            );
            final snapshot = page?['historicalSnapshot'];
            if (page?['executionScope'] is! Map ||
                snapshot is! Map ||
                _scopeStamp(page!['executionScope'] as Map) != scope)
              throw StateError('历史持续事项归属不可用');
            for (final resource
                in (snapshot['resources'] as List? ?? []).whereType<Map>()) {
              if (resource['kind'] != 'continuity' ||
                  resource['body'] is! Map ||
                  resource['deleted'] == true)
                continue;
              final document = Map<String, dynamic>.from(
                resource['body'] as Map,
              );
              document['readOnly'] = true;
              document['ownerId'] = snapshot['ownerId'];
              documents['historical/${snapshot['ownerId']}/${resource['id']}'] =
                  document;
            }
            historicalCursor = (snapshot['nextCursors']?['continuity'] ?? '')
                .toString();
            if (historicalCursor.isNotEmpty && !visited.add(historicalCursor))
              throw StateError('历史持续事项分页游标重复');
            if (documents.length > 32768) throw StateError('历史持续事项超过加载上限');
          } while (historicalCursor.isNotEmpty);
        }
      }
      for (final document in documents.values) {
        if (document['thread'] is! Map) {
          throw StateError('持续事项数据无效');
        }
        final item = ContinuityThreadDto.fromJson({
          ...Map<String, dynamic>.from(document['thread'] as Map),
          'sourceDocument': document,
        });
        _threadRoles[item.id] = item.characterId;
        if ((status.isEmpty || item.status == status) &&
            (query.isEmpty ||
                '${item.title} ${item.goal} ${item.summary}'
                    .toLowerCase()
                    .contains(query.toLowerCase()))) {
          result.add(item);
        }
      }
      return result;
    }
    final response = await _api.get<List<dynamic>>(
      '/api/continuity/threads',
      queryParameters: {
        if (query.trim().isNotEmpty) 'q': query.trim(),
        if (status.trim().isNotEmpty) 'status': status.trim(),
        'limit': limit,
      },
    );
    return (response ?? const <dynamic>[])
        .whereType<Map>()
        .map(
          (item) =>
              ContinuityThreadDto.fromJson(Map<String, dynamic>.from(item)),
        )
        .toList(growable: false);
  }

  Future<ContinuityDetailDto> get(
    String id, {
    Map<String, dynamic>? expectedDocument,
  }) async {
    final document = await _ownedDocument(
      id,
      expectedDocument: expectedDocument,
    );
    if (document != null) {
      return ContinuityDetailDto.fromJson({...document, 'bindings': []});
    }
    final response = await _api.get<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(id)}',
    );
    return ContinuityDetailDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<Map<String, dynamic>?> prepareCreate() async {
    final role = await _ownedRole();
    if (role == null) return null;
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/data',
      queryParameters: {'kind': 'memory', 'characterId': role},
    );
    final scope = result?['executionScope'];
    if (scope is! Map ||
        scope['coreId'] != _coordination?['coreId'] ||
        scope['roleId'] != role)
      throw StateError('无法确认持续事项创建的数据归属');
    return {
      'executionScope': Map<String, dynamic>.from(scope),
      'coreId': scope['coreId'],
      'ownerId': scope['resourceOwnerId'],
      'modeRevision': scope['modeRevision'],
    };
  }

  Future<ContinuityThreadDto> create(
    Map<String, dynamic> data, {
    Map<String, dynamic>? expectedDocument,
  }) async {
    final document = await _ownedMutation('create', data, expectedDocument);
    if (document != null) {
      return ContinuityThreadDto.fromJson(
        Map<String, dynamic>.from(document['thread'] as Map),
      );
    }
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads',
      data: data,
    );
    return ContinuityThreadDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityThreadDto> update(
    String id,
    Map<String, dynamic> data, {
    Map<String, dynamic>? expectedDocument,
  }) async {
    final current = expectedDocument;
    if (current != null) {
      final action = switch (data['status']) {
        'paused' => 'pause',
        'active' => 'resume',
        'completed' => 'complete',
        'cancelled' => 'cancel',
        _ => 'update',
      };
      final document = await _ownedMutation(action, {
        ...data,
        'id': id,
        'updateGoal': data.containsKey('goal'),
        'updateNextAction': data.containsKey('nextAction'),
      }, current);
      if (document == null) throw StateError('服务提供者已变化，请重新加载');
      return ContinuityThreadDto.fromJson(
        Map<String, dynamic>.from(document['thread'] as Map),
      );
    }
    if (await _ownedRole() != null) throw StateError('请先加载持续事项的原始归属后再修改');
    final response = await _api.patch<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(id)}',
      data: data,
    );
    return ContinuityThreadDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<void> confirmExecution(
    String id,
    String leaseId,
    String outcome, {
    String result = '',
    Map<String, dynamic>? expectedDocument,
  }) async {
    final current = expectedDocument;
    if (current == null ||
        current['lease']?['id'] != leaseId ||
        current['lease']?['state'] != 'unknown') {
      throw StateError('执行状态已变化，请刷新后再确认');
    }
    if (await _ownedMutation('confirm_execution', {
          'id': id,
          'leaseId': leaseId,
          'outcome': outcome,
          'result': result,
        }, current) ==
        null) {
      throw StateError('服务提供者已变化，请重新加载');
    }
  }

  Future<ContinuityWaitDto> createWait(
    String threadId,
    Map<String, dynamic> data, {
    Map<String, dynamic>? expectedDocument,
  }) async {
    final current = expectedDocument;
    if (current != null) {
      final document = await _ownedMutation('add_wait', {
        'id': threadId,
        'wait': {
          ...data,
          'waitType': data['waitType'] ?? data['type'],
          'conditionJson':
              data['conditionJson'] ?? jsonEncode(data['condition'] ?? {}),
          'autoResume': data['autoResume'] ?? false,
        },
      }, current);
      final waits = document?['waits'];
      if (waits is! List || waits.isEmpty) throw StateError('等待条件尚未确认保存');
      return ContinuityWaitDto.fromJson(
        Map<String, dynamic>.from(waits.last as Map),
      );
    }
    if (await _ownedRole() != null) throw StateError('请先加载持续事项的原始归属后再添加等待');
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(threadId)}/waits',
      data: data,
    );
    return ContinuityWaitDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityWaitDto> resolveWait(
    String threadId,
    String waitId, {
    bool resume = true,
    Map<String, dynamic>? expectedDocument,
  }) async {
    final current = expectedDocument;
    if (current != null) {
      final document = await _ownedMutation('resolve_wait', {
        'id': threadId,
        'waitId': waitId,
        'resume': resume,
      }, current);
      final waits = document?['waits'];
      if (waits is! List) throw StateError('等待条件已变化');
      return ContinuityWaitDto.fromJson(
        Map<String, dynamic>.from(
          waits.firstWhere((value) => value is Map && value['id'] == waitId)
              as Map,
        ),
      );
    }
    if (await _ownedRole() != null) throw StateError('请先加载持续事项的原始归属后再处理等待');
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(threadId)}/waits/${Uri.encodeComponent(waitId)}/resolve',
      data: {'resume': resume},
    );
    return ContinuityWaitDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityWaitDto> cancelWait(
    String threadId,
    String waitId, {
    Map<String, dynamic>? expectedDocument,
  }) async {
    final current = expectedDocument;
    if (current != null) {
      final document = await _ownedMutation('cancel_wait', {
        'id': threadId,
        'waitId': waitId,
      }, current);
      final waits = document?['waits'];
      if (waits is! List) throw StateError('等待条件已变化');
      return ContinuityWaitDto.fromJson(
        Map<String, dynamic>.from(
          waits.firstWhere((value) => value is Map && value['id'] == waitId)
              as Map,
        ),
      );
    }
    if (await _ownedRole() != null) throw StateError('请先加载持续事项的原始归属后再取消等待');
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(threadId)}/waits/${Uri.encodeComponent(waitId)}/cancel',
    );
    return ContinuityWaitDto.fromJson(response ?? const <String, dynamic>{});
  }
}
