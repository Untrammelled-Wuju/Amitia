import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:dio/dio.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../backend_transport/backend_service_api.dart';
import '../models/conversation.dart';
import '../models/project.dart';
import 'device_owned_attachments.dart';
import 'owned_conversation_reference.dart';

const _scopeFields = [
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
  'turnId',
  'executionId',
  'roleOwnerId',
  'resourceOwnerId',
  'requestId',
];

String _scopeStamp(Map<String, dynamic> scope, String requestId) {
  if (scope['requestId'] != requestId ||
      const [
        'coreId',
        'roleId',
        'resourceOwnerId',
        'roleOwnerId',
        'turnId',
        'executionId',
      ].any((key) => (scope[key] ?? '').toString().isEmpty) ||
      const [
        'providerEpoch',
        'modeRevision',
        'roleRevision',
      ].any((key) => scope[key] is! int || (scope[key] as int) < 1)) {
    throw StateError('回复缺少有效的服务提供者和数据归属信息');
  }
  return jsonEncode(_scopeFields.map((key) => scope[key]).toList());
}

Stream<Map<String, dynamic>> decodeOwnedChatStream(
  Stream<List<int>> source,
  String requestId,
) async* {
  var buffer = '';
  var scopeStamp = '';
  var terminal = false;
  var saved = false;
  var total = 0;
  String? transcription;
  await for (final chunk in source.transform(utf8.decoder)) {
    total += utf8.encode(chunk).length;
    if (total > 4 * 1024 * 1024) throw StateError('回复数据超出接收上限');
    buffer += chunk;
    while (true) {
      final boundary = RegExp(r'\r?\n\r?\n').firstMatch(buffer);
      if (boundary == null) break;
      final frame = buffer.substring(0, boundary.start);
      buffer = buffer.substring(boundary.end);
      final lines = frame
          .split(RegExp(r'\r?\n'))
          .where((line) => line.startsWith('data:'))
          .map((line) => line.substring(5).trimLeft())
          .toList();
      if (lines.isEmpty) continue;
      if (terminal) throw StateError('回复结束后仍收到数据，已拦截');
      final raw = jsonDecode(lines.join('\n'));
      if (raw is! Map) throw StateError('回复事件格式无效');
      final event = Map<String, dynamic>.from(raw);
      final type = event['type'];
      if (!const [
        'started',
        'transcribed',
        'delta',
        'completed',
        'interrupted',
        'failed',
      ].contains(type))
        throw StateError('未知的回复事件');
      if (type == 'started' || type == 'delta' || type == 'transcribed') {
        final rawScope = event['executionScope'];
        if (rawScope is! Map) throw StateError('回复缺少执行范围');
        final current = _scopeStamp(
          Map<String, dynamic>.from(rawScope),
          requestId,
        );
        if (type == 'started') {
          if (scopeStamp.isNotEmpty) throw StateError('重复的回复开始事件');
          scopeStamp = current;
        } else if (scopeStamp.isEmpty || current != scopeStamp) {
          throw StateError('服务提供者、角色或数据归属已变化，迟到回复已拦截');
        }
        if (type == 'transcribed') {
          final text = event['text'];
          if (transcription != null ||
              text is! String ||
              text.trim().isEmpty ||
              utf8.encode(text).length > 65536)
            throw StateError('语音转写事件无效');
          transcription = text;
        }
      } else {
        final result = event['data'];
        if (result is Map && result['executionScope'] is Map) {
          final current = _scopeStamp(
            Map<String, dynamic>.from(result['executionScope'] as Map),
            requestId,
          );
          if (scopeStamp.isNotEmpty && current != scopeStamp)
            throw StateError('回复结果与当前服务提供者不一致');
        }
        if (type == 'completed') {
          if (result is! Map ||
              result['saved'] != true ||
              result['requestId'] != requestId ||
              result['executionScope'] is! Map)
            throw StateError('回复尚未确认保存');
          _scopeStamp(
            Map<String, dynamic>.from(result['executionScope'] as Map),
            requestId,
          );
          if (transcription != null || result.containsKey('transcription')) {
            final text = result['transcription'];
            if (text is! String ||
                text.trim().isEmpty ||
                utf8.encode(text).length > 65536 ||
                result['userRevision'] != 2 ||
                (transcription != null && text != transcription)) {
              throw StateError('语音转写结果与保存确认不一致');
            }
          } else if (result.containsKey('userRevision') &&
              result['userRevision'] != 1) {
            throw StateError('消息版本与保存确认不一致');
          }
          saved = true;
        }
        terminal = true;
      }
      yield event;
      if (type == 'failed' || type == 'interrupted')
        throw StateError((event['message'] ?? '当前回复已中断').toString());
    }
    if (buffer.length > 512 * 1024) throw StateError('回复事件超出接收上限');
  }
  if (buffer.trim().isNotEmpty || !terminal || !saved)
    throw StateError('回复连接已断开，结果尚未确认；不会自动重新发送');
}

class DeviceOwnedChatService {
  DeviceOwnedChatService(
    this._api, {
    String Function()? providerKey,
    Future<Map<String, dynamic>?> Function()? providerTransition,
  }) : _providerKey = providerKey ?? (() => ''),
       _providerTransition = providerTransition;

  final BackendServiceApi _api;
  final String Function() _providerKey;
  final Future<Map<String, dynamic>?> Function()? _providerTransition;
  bool enabled = false;
  String coreId = '';
  String notice = '';
  String _providerChangeId = '';
  Map<String, dynamic>? policy;
  List<Map<String, dynamic>> roles = [];
  int revision = 0;
  CancelToken? _active;
  String _requestId = '';
  String _binding = '';
  bool _refreshing = false;
  final _resources = <String, Map<String, dynamic>>{};
  final _summaryViews = <String, Map<String, dynamic>>{};
  final _summaries = <String, String>{};
  final _historyCursors = <String, Map<String, String>>{};
  final _historyScopes = <String, String>{};
  final _historyRequests = <String, Object>{};
  final _conversationLoads = <String, Future<List<ConversationDto>>>{};
  bool _providerStateLoaded = false;

  Future<void> _loadProviderState() async {
    if (_providerStateLoaded) return;
    _providerStateLoaded = true;
    try {
      final preferences = await SharedPreferences.getInstance();
      final state = jsonDecode(
        preferences.getString('amitia.mesh.provider-state') ?? '{}',
      );
      if (state is Map) {
        coreId = (state['coreId'] ?? '').toString();
        notice = (state['notice'] ?? '').toString();
        _providerChangeId = (state['changeId'] ?? '').toString();
      }
    } catch (_) {}
  }

  Future<void> _saveProviderState() async {
    try {
      final preferences = await SharedPreferences.getInstance();
      await preferences.setString(
        'amitia.mesh.provider-state',
        jsonEncode({
          'coreId': coreId,
          'notice': notice,
          'changeId': _providerChangeId,
        }),
      );
    } catch (_) {}
  }

  String? previousSummary(String previousCore, String conversationId) =>
      _summaries['$previousCore/$conversationId'];

  String _stamp(String core, Map<String, dynamic>? state) =>
      '$core:${state?['providerEpoch']}:${state?['modeRevision']}:${state?['permissionRevision']}';

  Future<bool> refresh() async {
    final transition = await _providerTransition?.call();
    if (transition?['providerChangePending'] == true) {
      enabled = true;
      roles = [];
      stopLocal(
        '云端服务提供者正在从「${transition?['coreId'] ?? coreId}」切换至「${transition?['successorCoreId'] ?? '新 Core'}」，当前回复已中断，等待新服务批准并连接。',
      );
      throw StateError(notice);
    }
    final binding = _providerKey();
    if (binding.isEmpty) {
      if (enabled || _active != null) stopLocal('云端服务连接已停止，当前回复已中断。');
      enabled = false;
      return false;
    }
    if (_binding.isNotEmpty && _binding != binding)
      stopLocal('云端服务地址已变化，当前回复已中断；连接就绪后再继续发送。');
    _binding = binding;
    await _loadProviderState();
    final current = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    if (_providerKey() != binding) throw StateError('服务提供者已变化，旧状态已丢弃');
    if (current == null || current['policy'] is! Map)
      throw StateError('设备服务状态无效');
    final nextPolicy = Map<String, dynamic>.from(current['policy'] as Map);
    final nextCore = (current['coreId'] ?? '').toString();
    if ((enabled && _stamp(coreId, policy) != _stamp(nextCore, nextPolicy)) ||
        (coreId.isNotEmpty && coreId != nextCore)) {
      stopLocal(
        coreId != nextCore
            ? '云端服务提供者已从「$coreId」切换为「$nextCore」，当前回复已中断。'
            : '统筹模式或调用权限已变化，当前回复已中断；继续发送时使用当前服务和数据归属。',
      );
    }
    final changeId = (transition?['providerChangeId'] ?? '').toString();
    final previousCore = (transition?['previousCoreId'] ?? '').toString();
    if (transition?['coreId'] == nextCore &&
        changeId.isNotEmpty &&
        previousCore.isNotEmpty &&
        previousCore != nextCore &&
        _providerChangeId != changeId) {
      _providerChangeId = changeId;
      stopLocal('云端服务提供者已从「$previousCore」切换为「$nextCore」，当前回复已中断。');
    }
    coreId = nextCore;
    unawaited(_saveProviderState());
    policy = nextPolicy;
    enabled = current['coordinationAvailable'] == true;
    if (!enabled) {
      stopLocal('绑定的 Core 服务尚未就绪，当前请求已中断，请恢复连接后重试。');
      throw StateError(notice);
    }
    final available = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/roles',
    );
    if (_providerKey() != binding) throw StateError('服务提供者已变化，旧角色已丢弃');
    roles = (available?['roles'] as List? ?? [])
        .whereType<Map>()
        .map((role) => Map<String, dynamic>.from(role))
        .toList();
    return true;
  }

  Future<void> poll() async {
    if (_refreshing) return;
    _refreshing = true;
    try {
      await refresh();
    } finally {
      _refreshing = false;
    }
  }

  String selectRole(String? requested) {
    final selected =
        (requested?.trim().isNotEmpty == true
                ? requested
                : policy?['selectedRole'] ?? '')
            .toString();
    if (selected.isNotEmpty) {
      if (!roles.any((role) => role['id'] == selected))
        throw StateError('所选角色不存在或已停用，请重新选择角色');
      return selected;
    }
    if (roles.isEmpty) throw StateError('没有可用角色，拒绝调用');
    if (roles.length != 1) throw StateError('存在多个角色，请先选择角色');
    return roles.single['id'].toString();
  }

  String _historyKey(
    String conversationId,
    String role, [
    String keyword = '',
  ]) => jsonEncode([conversationId, role, keyword]);

  Future<Map<String, dynamic>> query(
    String conversationId, {
    String? characterId,
    bool older = false,
    String keyword = '',
  }) async {
    final origin = parseConversationReference(conversationId);
    final role = selectRole(characterId);
    final captured = revision;
    final key = _historyKey(conversationId, role, keyword);
    final ticket = Object();
    _historyRequests[key] = ticket;
    if (_historyRequests.length > 128)
      _historyRequests.remove(_historyRequests.keys.first);
    final cursors = older
        ? _historyCursors[key] ?? <String, String>{}
        : <String, String>{};
    final kind = conversationId.isEmpty ? 'conversation' : 'message';
    final legacyKind = conversationId.isEmpty
        ? 'legacyConversation'
        : 'legacyMessage';
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/conversations${conversationId.isEmpty ? '' : '/${Uri.encodeComponent(origin?['id'] ?? conversationId)}'}',
      queryParameters: {
        'characterId': role,
        if (keyword.isNotEmpty) 'keyword': keyword,
        if (origin != null) 'conversationOwnerId': origin['ownerId'],
        if (older) 'resourceKind': kind,
        ...cursors,
      },
    );
    if (captured != revision ||
        result == null ||
        !identical(_historyRequests[key], ticket))
      throw StateError('服务状态已变化，旧上下文已丢弃');
    final scope = result['executionScope'];
    if (scope is! Map) throw StateError('聊天历史缺少数据归属信息');
    final scopeStamp = jsonEncode(
      [
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
        'roleOwnerId',
        'resourceOwnerId',
        'roleId',
        'roleRevision',
      ].map((field) => scope[field]).toList(),
    );
    if (older && _historyScopes[key] != scopeStamp)
      throw StateError('服务或角色已变化，请重新加载当前对话');
    final next = <String, String>{};
    for (final source in [
      ['cursor', 'snapshot', kind],
      ['legacyCursor', 'snapshot', legacyKind],
      ['historicalCursor', 'historicalSnapshot', kind],
      ['historicalLegacyCursor', 'historicalSnapshot', legacyKind],
    ]) {
      if (older && !cursors.containsKey(source[0])) continue;
      final snapshot = result[source[1]];
      if (snapshot is! Map || snapshot['nextCursors'] is! Map) continue;
      final value =
          (snapshot['nextCursors'] as Map)[source[2]]?.toString() ?? '';
      if (value.isNotEmpty) next[source[0]] = value;
    }
    _historyCursors[key] = next;
    if (conversationId.isEmpty &&
        (!older || cursors.containsKey('historicalListCursor'))) {
      final historicalCursor =
          result['nextHistoricalListCursor']?.toString() ?? '';
      if (historicalCursor.isNotEmpty)
        next['historicalListCursor'] = historicalCursor;
    }
    _historyScopes[key] = scopeStamp;
    if (_historyScopes.length > 64) {
      final oldest = _historyScopes.keys.first;
      _historyScopes.remove(oldest);
      _historyCursors.remove(oldest);
      _historyRequests.remove(oldest);
    }
    final snapshot = result['snapshot'];
    if (snapshot is Map) {
      for (final row
          in (snapshot['resources'] as List? ?? []).whereType<Map>()) {
        _resources['${row['kind']}/${row['id']}'] = Map<String, dynamic>.from(
          row,
        )..['executionScope'] = result['executionScope'];
        if (row['kind'] == 'conversation' && row['body'] is Map) {
          final reference = ownedConversationRow(
            row['body'] as Map,
            snapshot['ownerId'].toString(),
          )['id'];
          _resources['conversation/$reference'] =
              _resources['conversation/${row['id']}']!;
        }
        if (conversationId.isNotEmpty &&
            row['kind'] == 'summary' &&
            row['body'] is Map) {
          final content = (row['body'] as Map)['content'];
          final text =
              (content is Map
                      ? content['summary'] ?? content['text'] ?? ''
                      : content?.toString() ?? '')
                  .toString();
          if (text.isNotEmpty) {
            _summaries['$coreId/$conversationId'] = text.length > 16000
                ? text.substring(0, 16000)
                : text;
            if (_summaries.length > 32)
              _summaries.remove(_summaries.keys.first);
          }
        }
      }
    }
    return {
      ...result,
      if (conversationId.isNotEmpty) 'conversationReference': conversationId,
    };
  }

  bool hasMore(
    String conversationId, {
    String? characterId,
    String keyword = '',
  }) {
    final role = selectRole(characterId);
    return _historyCursors[_historyKey(conversationId, role, keyword)]
            ?.isNotEmpty ==
        true;
  }

  Future<Map<String, dynamic>> data(
    String kind, {
    required String characterId,
    String conversationId = '',
    String cursor = '',
    String legacyCursor = '',
    String historicalRoleId = '',
    String historicalCursor = '',
    String historicalLegacyCursor = '',
  }) async {
    if (!enabled) throw StateError('设备数据服务尚未就绪');
    final captured = revision;
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/data',
      queryParameters: {
        'kind': kind,
        'characterId': selectRole(characterId),
        'conversationId': conversationId,
        'cursor': cursor,
        'legacyCursor': legacyCursor,
        'historicalRoleId': historicalRoleId,
        'historicalCursor': historicalCursor,
        'historicalLegacyCursor': historicalLegacyCursor,
      },
    );
    if (captured != revision ||
        result == null ||
        result['executionScope'] is! Map ||
        result['executionScope']['coreId'] != coreId ||
        result['snapshot'] is! Map)
      throw StateError('服务状态已变化，请重新加载记忆');
    for (final row
        in (result['snapshot']['resources'] as List? ?? []).whereType<Map>()) {
      _resources['${row['kind']}/${row['id']}'] = Map<String, dynamic>.from(row)
        ..['executionScope'] = result['executionScope'];
    }
    return result;
  }

  Future<List<Map<String, dynamic>>> historicalRoles(String characterId) async {
    final captured = revision;
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/historical-roles',
      queryParameters: {'characterId': selectRole(characterId)},
    );
    if (captured != revision ||
        result?['executionScope'] is! Map ||
        result?['executionScope']['coreId'] != coreId ||
        result?['roles'] is! List)
      throw StateError('服务状态已变化，旧记忆目录已丢弃');
    return (result!['roles'] as List)
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .toList();
  }

  Future<void> edit(
    String kind,
    String id, {
    Map<String, dynamic> changes = const {},
    bool deleted = false,
    bool clear = false,
    String? characterId,
    Map<String, dynamic>? expectedScope,
    String? expectedOwnerId,
    int? expectedRevision,
  }) async {
    if (!enabled) throw StateError('设备数据服务尚未就绪');
    final captured = revision;
    var resource = _resources['$kind/$id'];
    final origin = kind == 'conversation'
        ? parseConversationReference(id)
        : null;
    final role = selectRole(characterId ?? resource?['roleId']?.toString());
    if (resource == null) {
      if (origin != null) throw StateError('请先加载该会话并确认当前数据来源后再修改');
      final result = await _api.get<Map<String, dynamic>>(
        '/api/device-mesh/v1/business/resources',
        queryParameters: {'kind': kind, 'id': id, 'characterId': role},
      );
      if (result?['resource'] is Map) {
        resource = Map<String, dynamic>.from(result!['resource'] as Map);
        resource['executionScope'] = result['executionScope'];
      }
    }
    if (captured != revision) throw StateError('服务状态已变化，请重新加载后再操作');
    if (resource == null || resource['deleted'] == true)
      throw StateError('该记录不在当前数据来源中，请在原设备管理历史数据');
    final resourceId = resource['id'].toString();
    if (expectedOwnerId != null && resource['ownerId'] != expectedOwnerId)
      throw StateError('记录所属设备已变化，请在原设备管理历史数据');
    if (resource['executionScope'] is! Map)
      throw StateError('记录缺少数据来源版本，请重新加载');
    if (expectedScope != null &&
        _authorityStamp(expectedScope) !=
            _authorityStamp(resource['executionScope'] as Map)) {
      throw StateError('记录原始归属已变化，请重新加载后再修改');
    }
    final requestId =
        'edit-${DateTime.now().microsecondsSinceEpoch}-${_editSequence++}';
    final ack = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources/edit',
      data: {
        'requestId': requestId,
        'kind': kind,
        'id': resourceId,
        'characterId': role,
        'expectedRevision': expectedRevision ?? resource['revision'],
        'expectedExecutionScope': expectedScope ?? resource['executionScope'],
        'changes': changes,
        'deleted': deleted,
        'clear': clear,
      },
    );
    if (captured != revision) throw StateError('服务状态已变化，请重新加载确认操作结果');
    if (((kind == 'project' || changes.containsKey('projectId')) &&
            ack?['requestId'] != requestId) ||
        ack?['ownerId'] != resource['ownerId'] ||
        ack?['versions'] is! Map ||
        (ack!['versions'] as Map)['$kind/$resourceId'] !=
            (expectedRevision ?? resource['revision'] as int) + 1)
      throw StateError('数据所有者尚未确认操作结果');
    _resources.clear();
  }

  static int _editSequence = 0;

  static String _authorityStamp(Map scope) => jsonEncode(
    _scopeFields
        .where(
          (key) => !const ['requestId', 'turnId', 'executionId'].contains(key),
        )
        .map((key) => scope[key])
        .toList(),
  );

  Future<List<ProjectDto>> projects({String? characterId}) async {
    if (!enabled) throw StateError('设备数据服务尚未就绪');
    final role = selectRole(characterId);
    final captured = revision;
    final rows = <ProjectDto>[];
    final seen = <String>{};
    var cursor = '';
    var historicalCursor = '';
    final historyRoles = policy?['coordinated'] == true
        ? (await historicalRoles(role))
              .map((row) => (row['id'] ?? '').toString())
              .where((id) => id.isNotEmpty)
              .toList()
        : <String>[];
    var historyIndex = 0;
    var currentDone = false;
    final visitedCursors = <String>{};
    Map<String, dynamic>? originalScope;
    for (;;) {
      final result = await _api.get<Map<String, dynamic>>(
        '/api/device-mesh/v1/business/projects',
        queryParameters: {
          'characterId': role,
          'cursor': cursor,
          if (historyIndex < historyRoles.length)
            'historicalRoleId': historyRoles[historyIndex],
          if (historicalCursor.isNotEmpty) 'historicalCursor': historicalCursor,
        },
      );
      final rawScope = result?['executionScope'];
      if (captured != revision ||
          rawScope is! Map ||
          rawScope['coreId'] != coreId ||
          rawScope['roleId'] != role) {
        throw StateError('项目数据来源已变化，请重新加载');
      }
      final scope = Map<String, dynamic>.from(rawScope);
      if (originalScope != null &&
          _authorityStamp(scope) != _authorityStamp(originalScope)) {
        throw StateError('项目数据来源在分页期间变化，请重新加载');
      }
      originalScope = scope;
      for (final collection in ['projects', 'historicalProjects']) {
        if (collection == 'projects' && currentDone) continue;
        for (final row
            in (result![collection] as List? ?? []).whereType<Map>()) {
          if (row['id'] is! String ||
              row['ownerId'] is! String ||
              row['roleId'] is! String ||
              row['revision'] is! int ||
              row['revision'] < 1) {
            throw StateError('项目归属或版本无效');
          }
          final readOnly =
              collection == 'historicalProjects' || row['readOnly'] == true;
          if (!readOnly &&
              (row['ownerId'] != scope['resourceOwnerId'] ||
                  row['roleId'] != role)) {
            throw StateError('项目归属与当前服务不一致');
          }
          final key = '${row['ownerId']}/${row['roleId']}/${row['id']}';
          if (!seen.add(key)) continue;
          rows.add(
            ProjectDto.fromJson({
              ...Map<String, dynamic>.from(row),
              'logical': true,
              'readOnly': readOnly,
              'executionScope': scope,
            }),
          );
          if (!readOnly) {
            _resources['project/${row['id']}'] = {
              ...Map<String, dynamic>.from(row),
              'kind': 'project',
              'executionScope': scope,
            };
          }
        }
      }
      final next = currentDone ? '' : (result?['nextCursor'] ?? '').toString();
      if (next.isEmpty) currentDone = true;
      final nextHistorical = (result?['nextHistoricalCursor'] ?? '').toString();
      if (nextHistorical.isEmpty && historyIndex < historyRoles.length)
        historyIndex++;
      if (next.isEmpty &&
          nextHistorical.isEmpty &&
          historyIndex >= historyRoles.length)
        return rows;
      final cursorKey = '$historyIndex/$next/$nextHistorical';
      if (!visitedCursors.add(cursorKey) || rows.length > 32768)
        throw StateError('项目分页结果无效');
      cursor = next;
      historicalCursor = nextHistorical;
    }
  }

  Future<ProjectDto> createProject(
    String title, {
    String? characterId,
    Map<String, dynamic>? expectedScope,
  }) async {
    final role = selectRole(characterId);
    final captured = revision;
    final source = await data('memory', characterId: role);
    final currentScope = Map<String, dynamic>.from(
      source['executionScope'] as Map,
    );
    if (expectedScope != null &&
        _authorityStamp(expectedScope) != _authorityStamp(currentScope))
      throw StateError('项目创建表单的数据归属已变化，请重新打开');
    final scope = expectedScope ?? currentScope;
    final random = Random.secure();
    final requestId =
        'project-${List.generate(16, (_) => random.nextInt(256).toRadixString(16).padLeft(2, '0')).join()}';
    final result = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/projects',
      data: {
        'requestId': requestId,
        'title': title.trim(),
        'characterId': role,
        'expectedExecutionScope': scope,
      },
    );
    final returned = result?['executionScope'];
    final project = result?['project'];
    final ack = result?['acknowledgement'];
    if (captured != revision ||
        result?['saved'] != true ||
        returned is! Map ||
        _authorityStamp(returned) != _authorityStamp(scope) ||
        returned['requestId'] != requestId ||
        project is! Map ||
        project['id'] is! String ||
        ack is! Map ||
        ack['requestId'] != requestId ||
        ack['ownerId'] != scope['resourceOwnerId'] ||
        ack['versions'] is! Map ||
        ack['versions']['project/${project['id']}'] != 1 ||
        ack['versions']['checkpoint/project/$requestId'] != 1) {
      throw StateError('项目所有者尚未确认保存或服务已切换');
    }
    return ProjectDto.fromJson({
      ...Map<String, dynamic>.from(project),
      'ownerId': scope['resourceOwnerId'],
      'roleId': role,
      'revision': 1,
      'logical': true,
      'executionScope': returned,
    });
  }

  Future<void> editProject(
    ProjectDto project, {
    Map<String, dynamic> changes = const {},
    bool deleted = false,
  }) async {
    if (!project.logical ||
        project.readOnly ||
        project.executionScope == null ||
        project.ownerId.isEmpty ||
        project.revision < 1) {
      throw StateError('历史项目只能在原设备管理，请重新加载当前项目');
    }
    await edit(
      'project',
      project.id,
      characterId: project.roleId,
      expectedOwnerId: project.ownerId,
      expectedScope: project.executionScope,
      expectedRevision: project.revision,
      changes: changes,
      deleted: deleted,
    );
  }

  Future<Map<String, dynamic>?> conversationSummary(
    String conversationId,
  ) async {
    final result = await query(conversationId);
    _summaryViews.remove(conversationId);
    for (final name in ['snapshot', 'historicalSnapshot']) {
      final snapshot = result[name];
      if (snapshot is! Map) continue;
      for (final row
          in (snapshot['resources'] as List? ?? []).whereType<Map>()) {
        if (row['kind'] != 'summary' ||
            row['deleted'] == true ||
            row['body'] is! Map)
          continue;
        final content = (row['body'] as Map)['content'];
        final value = <String, dynamic>{
          'summaryViewId': 'summary-view:${++_editSequence}',
          'summaryText': content is Map
              ? content['summary'] ?? content['text'] ?? ''
              : content?.toString() ?? '',
          'sourceOwnerId': snapshot['ownerId'],
          'sourceResourceId': row['id'],
          'sourceRevision': row['revision'],
          'sourceScope': Map<String, dynamic>.from(
            result['executionScope'] as Map,
          ),
          'editable':
              name == 'snapshot' &&
              row['ownerId'] == result['executionScope']['resourceOwnerId'],
        };
        _summaryViews[conversationId] = value;
        if (_summaryViews.length > 64)
          _summaryViews.remove(_summaryViews.keys.first);
        return Map<String, dynamic>.from(jsonDecode(jsonEncode(value)) as Map);
      }
      if (snapshot['legacySummary'] is Map) {
        return {
          ...Map<String, dynamic>.from(snapshot['legacySummary'] as Map),
          'sourceOwnerId': snapshot['ownerId'],
          'editable': false,
        };
      }
    }
    return null;
  }

  Future<void> editConversationSummary(
    String conversationId, {
    String? text,
    bool deleted = false,
    String? displayedViewId,
  }) async {
    final displayed = _summaryViews[conversationId];
    if (displayed == null ||
        displayed['editable'] != true ||
        displayed['sourceRevision'] is! int ||
        displayed['sourceScope'] is! Map) {
      throw StateError('请先加载当前所有者的摘要，历史摘要需在原设备管理');
    }
    if (displayedViewId == null ||
        displayedViewId != displayed['summaryViewId']) {
      throw StateError('摘要视图已变化，请重新打开编辑');
    }
    await edit(
      'summary',
      displayed['sourceResourceId'].toString(),
      characterId: displayed['sourceScope']['roleId'].toString(),
      expectedOwnerId: displayed['sourceOwnerId'].toString(),
      expectedRevision: displayed['sourceRevision'] as int,
      expectedScope: Map<String, dynamic>.from(displayed['sourceScope'] as Map),
      changes: deleted
          ? const {}
          : {
              'content': {'summary': text?.trim() ?? ''},
            },
      deleted: deleted,
    );
    _summaryViews.remove(conversationId);
  }

  Future<Map<String, dynamic>> generateConversationSummary(
    String conversationId, {
    String? characterId,
  }) async {
    if (!enabled) throw StateError('设备摘要服务尚未就绪');
    final captured = revision;
    final result = await query(conversationId, characterId: characterId);
    final scope = Map<String, dynamic>.from(result['executionScope'] as Map);
    final origin = parseConversationReference(conversationId);
    final resourceId =
        '${result['conversationId'] ?? origin?['id'] ?? conversationId}/summary';
    final current = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources',
      queryParameters: {
        'kind': 'summary',
        'id': resourceId,
        'characterId': scope['roleId'],
      },
    );
    String stamp(Map value) => jsonEncode(
      _scopeFields
          .where(
            (key) =>
                !const ['requestId', 'turnId', 'executionId'].contains(key),
          )
          .map((key) => value[key])
          .toList(),
    );
    if (captured != revision ||
        current == null ||
        current['executionScope'] is! Map ||
        stamp(current['executionScope'] as Map) != stamp(scope))
      throw StateError('摘要数据来源已变化，请重新加载');
    final resource = current['resource'];
    if (resource != null &&
        (resource is! Map ||
            resource['ownerId'] != scope['resourceOwnerId'] ||
            resource['roleId'] != scope['roleId']))
      throw StateError('摘要数据来源不一致');
    final expectedRevision = resource is Map ? resource['revision'] : 0;
    if (expectedRevision is! int || expectedRevision < 0)
      throw StateError('摘要数据版本无效');
    final random = Random.secure();
    final requestId =
        'summary-${List.generate(16, (_) => random.nextInt(256).toRadixString(16).padLeft(2, '0')).join()}';
    final generated = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/conversations/${Uri.encodeComponent(origin?['id'] ?? conversationId)}/summary/generate',
      data: {
        'requestId': requestId,
        'characterId': scope['roleId'],
        'expectedRevision': expectedRevision,
        'expectedExecutionScope': scope,
        if (origin != null) 'conversationOrigin': origin,
      },
    );
    if (captured != revision ||
        generated == null ||
        generated['executionScope'] is! Map ||
        stamp(generated['executionScope'] as Map) != stamp(scope))
      throw StateError('服务状态已变化，旧摘要已丢弃');
    final ack = generated['acknowledgement'];
    if (generated['saved'] != true ||
        generated['executionScope']['requestId'] != requestId ||
        generated['sourceResourceId'] != resourceId ||
        generated['sourceOwnerId'] != scope['resourceOwnerId'] ||
        generated['sourceRevision'] != expectedRevision + 1 ||
        ack is! Map ||
        ack['ownerId'] != scope['resourceOwnerId'] ||
        ack['requestId'] != '$requestId|summary-result' ||
        ack['versions'] is! Map ||
        ack['versions']['summary/$resourceId'] != expectedRevision + 1)
      throw StateError('摘要所有者尚未确认保存');
    final view = <String, dynamic>{
      'summaryViewId': 'summary-view:${++_editSequence}',
      'summaryText': generated['summaryText'],
      'sourceOwnerId': generated['sourceOwnerId'],
      'sourceResourceId': resourceId,
      'sourceRevision': generated['sourceRevision'],
      'sourceScope': Map<String, dynamic>.from(
        generated['executionScope'] as Map,
      ),
      'editable': true,
    };
    _summaryViews[conversationId] = view;
    if (_summaryViews.length > 64)
      _summaryViews.remove(_summaryViews.keys.first);
    return {
      ...generated,
      ...Map<String, dynamic>.from(jsonDecode(jsonEncode(view)) as Map),
    };
  }

  Future<Map<String, dynamic>> projections(
    String characterId, {
    Map<String, dynamic>? expectedScope,
  }) async {
    if (!enabled) throw StateError('设备索引服务尚未就绪');
    final captured = revision;
    final role = selectRole(characterId);
    final result = expectedScope == null
        ? await _api.get<Map<String, dynamic>>(
            '/api/device-mesh/v1/business/projections',
            queryParameters: {'characterId': role},
          )
        : await _api.post<Map<String, dynamic>>(
            '/api/device-mesh/v1/business/projections/rebuild',
            data: {
              'characterId': role,
              'expectedExecutionScope': expectedScope,
            },
          );
    if (captured != revision ||
        result?['executionScope'] is! Map ||
        result?['executionScope']['coreId'] != coreId ||
        result?['executionScope']['roleId'] != role ||
        result?['status'] is! Map ||
        result?['status']['ownerId'] !=
            result?['executionScope']['resourceOwnerId'] ||
        result?['status']['roleId'] != role ||
        result?['status']['layers'] is! List) {
      throw StateError('服务状态已变化，索引状态已丢弃');
    }
    return result!;
  }

  List<MessageDto> messages(Map<String, dynamic> query) {
    final result = <MessageDto>[];
    final seen = <String>{};
    for (final snapshot in [
      query['historicalSnapshot'],
      query['snapshot'],
    ].whereType<Map>()) {
      final owner = (snapshot['ownerId'] ?? '').toString();
      final rows = <Map>[
        ...(snapshot['legacyMessages'] as List? ?? []).whereType<Map>().map(
          (row) => {...row, 'sourceRevision': null},
        ),
        ...(snapshot['resources'] as List? ?? [])
            .whereType<Map>()
            .where((row) => row['kind'] == 'message')
            .where((row) => row['body'] is Map)
            .map(
              (row) => {
                ...row['body'] as Map,
                'sourceRevision': row['revision'],
              },
            ),
      ];
      for (final row in rows) {
        final file = ownedFileMetadata(row['attachments']);
        final id = (row['id'] ?? '').toString();
        if (id.isEmpty ||
            !const ['user', 'assistant'].contains(row['role']) ||
            !seen.add('$owner:$id'))
          continue;
        result.add(
          MessageDto.fromJson({
            ...Map<String, dynamic>.from(row),
            if (query['conversationReference'] != null)
              'conversationId': query['conversationReference'],
            'imageUrl':
                ownedImageURL(row['attachments']) ?? row['imageUrl'] ?? '',
            'content': ownedMessageText(row),
            'audioUrl':
                ownedAudioURL(row['attachments']) ?? row['audioUrl'] ?? '',
            if (file != null) ...{
              'msgType': file['kind'],
              'resourceUri': file['uri'],
              'fileName': file['name'],
              'mimeType': file['mimeType'],
              'fileSizeBytes': file['sizeBytes'],
              if (file['kind'] == 'video') 'videoUrl': file['uri'],
            },
            'sourceOwnerId': owner,
            'sourceConversationId': row['conversationId'],
            'sourceScope': query['executionScope'],
          }),
        );
      }
    }
    final scope = query['executionScope'];
    if (scope is Map) {
      final failures = (query['deliveryFailures'] as List? ?? [])
          .whereType<Map>()
          .where(
            (row) =>
                row['ownerId'] == scope['resourceOwnerId'] &&
                const [
                  'mesh.owned_resource_version',
                  'mesh.owned_request_conflict',
                ].contains(row['errorCode']),
          )
          .toList();
      if (failures.isNotEmpty) {
        result.add(
          MessageDto.fromJson({
            'id':
                'save-failures:${scope['coreId']}:${scope['resourceOwnerId']}:${scope['roleId']}:${failures.first['conversationId'] ?? ''}',
            'role': 'system',
            'type': 'system_notice',
            'content':
                '有 ${failures.length} 项保存请求被数据所有者拒绝，未确认保存。请重新加载记录后处理版本或请求编号冲突；系统不会继续重试这些旧请求。',
            'createdAt': failures.first['failedAt'],
            'sourceOwnerId': scope['resourceOwnerId'],
            'sourceScope': scope,
          }),
        );
      }
    }
    result.sort((a, b) => a.createdAt.compareTo(b.createdAt));
    return result;
  }

  Future<List<ConversationDto>> conversations({
    String? characterId,
    String keyword = '',
  }) async {
    final role = selectRole(characterId);
    keyword = keyword.trim().toLowerCase();
    final key = _historyKey('', role, keyword);
    final previous = _conversationLoads[key];
    if (previous != null) return previous;
    final pending = _loadConversations(role, keyword);
    _conversationLoads[key] = pending;
    try {
      return await pending;
    } finally {
      if (identical(_conversationLoads[key], pending))
        _conversationLoads.remove(key);
    }
  }

  Future<List<MessageDto>> allMessages(
    String conversationId, {
    String? characterId,
  }) async {
    final role = selectRole(characterId);
    final captured = revision;
    final rows = <String, MessageDto>{};
    final sizes = <String, int>{};
    final visited = <String>{};
    var bytes = 0;
    var result = await query(conversationId, characterId: role);
    for (;;) {
      if (captured != revision) throw StateError('服务状态已变化，旧历史已丢弃');
      for (final row in messages(result)) {
        final key = '${row.sourceOwnerId}:${row.id}';
        final previous = rows[key];
        if (previous != null && previous.sourceRevision != row.sourceRevision) {
          throw StateError('历史记录在读取期间已变化，请重新加载');
        }
        final size = utf8
            .encode(
              row.content + row.reasoningContent + row.imageUrl + row.audioUrl,
            )
            .length;
        bytes += size - (sizes[key] ?? 0);
        sizes[key] = size;
        rows[key] = row;
      }
      if (rows.length > 4096 || bytes > 32 * 1024 * 1024)
        throw StateError('历史记录超过单次读取上限，请分段读取');
      if (!hasMore(conversationId, characterId: role)) {
        return rows.values.toList()
          ..sort((a, b) => a.createdAt.compareTo(b.createdAt));
      }
      final cursor = jsonEncode(
        _historyCursors[_historyKey(conversationId, role)],
      );
      if (!visited.add(cursor)) throw StateError('历史分页结果无效，请重新加载');
      result = await query(conversationId, characterId: role, older: true);
    }
  }

  Future<List<ConversationDto>> _loadConversations(
    String role,
    String keyword,
  ) async {
    var result = await query('', characterId: role, keyword: keyword);
    final rows = <String, ConversationDto>{};
    final visited = <String>{};
    for (;;) {
      for (final row
          in (result['historicalConversations'] as List? ?? [])
              .whereType<Map>()) {
        final value = ConversationDto.fromJson(
          ownedConversationRow(
            row,
            (row['ownerId'] ??
                    (result['executionScope'] as Map)['targetDeviceId'])
                .toString(),
          ),
        );
        if (value.id.isNotEmpty) rows.putIfAbsent(value.id, () => value);
      }
      for (final snapshot in [
        result['historicalSnapshot'],
        result['snapshot'],
      ].whereType<Map>()) {
        final values = <Map>[
          ...(snapshot['legacyConversations'] as List? ?? []).whereType<Map>(),
          ...(snapshot['resources'] as List? ?? [])
              .whereType<Map>()
              .where((row) => row['kind'] == 'conversation')
              .map((row) => row['body'])
              .whereType<Map>(),
        ];
        for (final row in values) {
          final value = ConversationDto.fromJson(
            ownedConversationRow(row, snapshot['ownerId'].toString()),
          );
          if (value.id.isNotEmpty) rows[value.id] = value;
        }
      }
      if (!hasMore('', characterId: role, keyword: keyword)) {
        return rows.values.toList()
          ..sort((a, b) => b.updatedAt.compareTo(a.updatedAt));
      }
      final cursor = jsonEncode(
        _historyCursors[_historyKey('', role, keyword)],
      );
      if (!visited.add(cursor) || rows.length > 32768) {
        throw StateError('会话分页结果无效，请重新加载');
      }
      result = await query(
        '',
        characterId: role,
        older: true,
        keyword: keyword,
      );
    }
  }

  Stream<Map<String, dynamic>> send({
    required String requestId,
    required String message,
    String? conversationId,
    String? characterId,
    Map<String, dynamic>? context,
    List<Map<String, dynamic>>? attachments,
    Map<String, dynamic>? quote,
  }) async* {
    if (_active != null || !enabled) throw StateError('当前回复尚未结束或设备服务尚未就绪');
    final origin = parseConversationReference(conversationId ?? '');
    final token = CancelToken();
    final captured = revision;
    _active = token;
    _requestId = requestId;
    try {
      final role = selectRole(characterId);
      final view = await data('memory', characterId: role);
      final expectedScope = Map<String, dynamic>.from(
        view['executionScope'] as Map,
      );
      if (_scopeFields.any((key) => expectedScope[key] == null) ||
          captured != revision ||
          token.isCancelled) {
        throw StateError('发送前无法确认当前角色、权限与数据归属');
      }
      if (quote != null &&
          (quote['expectedExecutionScope'] is! Map ||
              _authorityStamp(quote['expectedExecutionScope'] as Map) !=
                  _authorityStamp(expectedScope))) {
        throw StateError('引用消息的 Core、角色或归属已变化，请重新选择');
      }
      final stream = await _api.postStream(
        '/api/device-mesh/v1/business/messages',
        data: {
          'requestId': requestId,
          'message': message,
          'characterId': role,
          'expectedExecutionScope': expectedScope,
          if (quote != null) 'quote': quote,
          if (quote != null &&
              quote['ownerId'] != expectedScope['resourceOwnerId'])
            'historicalRoleId': quote['characterId'],
          if (conversationId?.isNotEmpty == true)
            'conversationId': origin?['id'] ?? conversationId,
          if (origin != null) 'conversationOrigin': origin,
          if (context != null)
            'context': {
              ...context,
              if (origin != null) 'conversationId': origin['id'],
            },
          if (attachments != null) 'attachments': attachments,
        },
        headers: {'Accept': 'text/event-stream'},
        cancelToken: token,
      );
      await for (final event in decodeOwnedChatStream(stream, requestId)) {
        if (captured != revision || token.isCancelled)
          throw StateError('服务状态已变化，迟到回复已拦截');
        final result = event['data'];
        final scope = result is Map
            ? result['executionScope']
            : event['executionScope'];
        final id = result is Map
            ? result['conversationId']
            : event['conversationId'];
        if (scope is Map && id is String && id.isNotEmpty) {
          final reference = conversationReference(
            result is Map && result['conversationOrigin'] is Map
                ? result['conversationOrigin'] as Map
                : origin ?? {'ownerId': scope['resourceOwnerId'], 'id': id},
          );
          yield {
            ...event,
            'sourceConversationId': id,
            'conversationId': reference,
            if (result is Map)
              'data': {
                ...result,
                'sourceConversationId': id,
                'conversationId': reference,
              },
          };
        } else {
          yield event;
        }
      }
    } finally {
      if (identical(_active, token)) {
        _active = null;
        _requestId = '';
      }
    }
  }

  Future<void> interrupt() async {
    if (_requestId.isEmpty) return;
    await _api.post(
      '/api/device-mesh/v1/business/messages/${Uri.encodeComponent(_requestId)}/interrupt',
    );
  }

  void stopLocal(String message) {
    revision++;
    _resources.clear();
    _historyCursors.clear();
    _summaryViews.clear();
    _historyScopes.clear();
    _historyRequests.clear();
    _conversationLoads.clear();
    _active?.cancel(message);
    _active = null;
    _requestId = '';
    notice = message;
    unawaited(_saveProviderState());
  }
}
