import 'dart:convert';
import 'dart:math';
import '../backend_transport/backend_service_api.dart';
import '../models/continuity.dart';

class ContinuityService {
  ContinuityService(this._api);

  final BackendServiceApi _api;

  String selectedRole = '';
  final _threadRoles = <String, String>{};
  Map<String, dynamic>? _coordination;
  String _roleOwner = '';
  String _providerStamp = '';
  List<Map<String, dynamic>> roles = [];

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
    _roleOwner = (response?['roleOwnerId'] ?? '').toString();
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
    if (_coordination?['coordinationAvailable'] != true) return null;
    final role = requested?.isNotEmpty == true ? requested! : selectedRole;
    if (role.isEmpty || !roles.any((value) => value['id'] == role)) {
      throw StateError('请先选择持续事项使用的有效角色');
    }
    return role;
  }

  Future<Map<String, dynamic>?> _ownedDocument(String id) async {
    final role = await _ownedRole(_threadRoles[id]);
    if (role == null) return null;
    final document = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/continuity',
      queryParameters: {'id': id, 'characterId': role},
    );
    if (document == null || document['thread'] is! Map) {
      throw StateError('持续事项数据无效');
    }
    _threadRoles[id] = (document['thread']['characterId'] ?? role).toString();
    return document;
  }

  Future<Map<String, dynamic>?> _ownedMutation(
    String action,
    Map<String, dynamic> data, [
    Map<String, dynamic>? current,
  ]) async {
    final role = await _ownedRole(
      (current?['thread']?['characterId'] ?? data['characterId'])?.toString(),
    );
    if (role == null) return null;
    final response = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/continuity',
      data: {
        ...data,
        'action': action,
        'characterId': role,
        'requestId': base64UrlEncode(
          List<int>.generate(32, (_) => Random.secure().nextInt(256)),
        ).replaceAll('=', ''),
        'expectedRevision': current?['thread']?['revision'] ?? 0,
        'expectedCoreId': current?['coreId'] ?? _coordination?['coreId'],
        'expectedOwnerId': current?['ownerId'] ?? _roleOwner,
        'expectedModeRevision':
            current?['modeRevision'] ??
            _coordination?['policy']?['modeRevision'],
      },
    );
    final document = response?['document'];
    final ack = response?['acknowledgement'];
    if (document is! Map ||
        ack is! Map ||
        document['thread'] is! Map ||
        ack['ownerId'] != document['ownerId'] ||
        ack['versions']?['continuity/${document['thread']['id']}'] !=
            document['thread']['revision']) {
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
        final stamp = jsonEncode(currentScope);
        if (scope.isNotEmpty && scope != stamp) {
          throw StateError('持续事项的数据归属已变化，请重新加载');
        }
        scope = stamp;
        for (final document in page['documents'] as List) {
          if (document is! Map || document['thread'] is! Map) {
            throw StateError('持续事项数据无效');
          }
          documents[document['thread']['id'].toString()] = document;
        }
        if (documents.length > 32768) throw StateError('持续事项数量超过加载上限，请缩小查询范围');
        cursor = (page['nextCursor'] ?? '').toString();
        if (cursor.isNotEmpty && !cursors.add(cursor)) {
          throw StateError('持续事项分页游标重复');
        }
      } while (cursor.isNotEmpty);
      final result = <ContinuityThreadDto>[];
      for (final document in documents.values) {
        if (document['thread'] is! Map) {
          throw StateError('持续事项数据无效');
        }
        final item = ContinuityThreadDto.fromJson(
          Map<String, dynamic>.from(document['thread'] as Map),
        );
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

  Future<ContinuityDetailDto> get(String id) async {
    final document = await _ownedDocument(id);
    if (document != null) {
      return ContinuityDetailDto.fromJson({...document, 'bindings': []});
    }
    final response = await _api.get<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(id)}',
    );
    return ContinuityDetailDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityThreadDto> create(Map<String, dynamic> data) async {
    final document = await _ownedMutation('create', data);
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
    Map<String, dynamic> data,
  ) async {
    final current = await _ownedDocument(id);
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
  }) async {
    final current = await _ownedDocument(id);
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
    Map<String, dynamic> data,
  ) async {
    final current = await _ownedDocument(threadId);
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
  }) async {
    final current = await _ownedDocument(threadId);
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
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(threadId)}/waits/${Uri.encodeComponent(waitId)}/resolve',
      data: {'resume': resume},
    );
    return ContinuityWaitDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityWaitDto> cancelWait(String threadId, String waitId) async {
    final current = await _ownedDocument(threadId);
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
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(threadId)}/waits/${Uri.encodeComponent(waitId)}/cancel',
    );
    return ContinuityWaitDto.fromJson(response ?? const <String, dynamic>{});
  }
}
