import 'dart:convert';
import 'dart:math';

import '../backend_transport/backend_service_api.dart';
import '../models/worldbook.dart';
import 'device_owned_chat_service.dart';

class WorldBookService {
  WorldBookService(this._owned, this._api);

  final DeviceOwnedChatService _owned;
  final BackendServiceApi _api;

  String _requestId() {
    final values = List.generate(16, (_) => Random.secure().nextInt(256));
    values[6] = (values[6] & 15) | 64;
    values[8] = (values[8] & 63) | 128;
    final value = values
        .map((byte) => byte.toRadixString(16).padLeft(2, '0'))
        .join();
    return '${value.substring(0, 8)}-${value.substring(8, 12)}-${value.substring(12, 16)}-${value.substring(16, 20)}-${value.substring(20)}';
  }

  String _scopeStamp(Map scope) => jsonEncode(
    (scope.keys
            .where(
              (key) => !const [
                'requestId',
                'turnId',
                'executionId',
                'worldbookRoleId',
              ].contains(key),
            )
            .toList()
          ..sort())
        .map((key) => [key, scope[key]])
        .toList(),
  );

  Future<List<WorldBookDto>> _loadWithScope() async {
    if (!await _owned.refresh()) {
      throw StateError(_owned.notice.isEmpty ? '设备世界书服务尚未就绪' : _owned.notice);
    }
    final role = _owned.selectRole(null);
    final result = await _owned.data('worldbook', characterId: role);
    final scope = result['executionScope'];
    final snapshot = result['snapshot'];
    if (scope is! Map ||
        scope['coreId'] != _owned.coreId ||
        scope['roleId'] != role ||
        snapshot is! Map ||
        snapshot['ownerId'] != scope['resourceOwnerId']) {
      throw StateError('世界书设备、Core 或角色归属无效');
    }
    final rows = snapshot['resources'];
    if (rows is! List) throw StateError('世界书响应格式无效');
    return rows
        .whereType<Map>()
        .where((row) => row['kind'] == 'worldbook')
        .map((row) {
          final body = row['body'] is Map
              ? Map<String, dynamic>.from(row['body'] as Map)
              : <String, dynamic>{};
          final global =
              row['roleId'] == '@global-worldbook' || body['scope'] == 'global';
          return WorldBookDto.fromJson({
            ...body,
            'id': row['id'],
            'scope': global ? 'global' : 'role',
            'characterId': global ? '' : row['roleId'],
            'ownerId': row['ownerId'],
            'roleId': row['roleId'],
            'revision': row['revision'],
            'enabled': body['enabled'] != false,
          });
        })
        .toList(growable: false);
  }

  Future<List<WorldBookDto>> list({
    String matchType = '',
    String characterId = '',
    String keyword = '',
    int page = 1,
    int pageSize = 100,
  }) async {
    var rows = await _loadWithScope();
    final query = keyword.trim().toLowerCase();
    rows = rows
        .where((row) {
          if (matchType.isNotEmpty && row.matchType != matchType) return false;
          if (characterId.isNotEmpty && row.characterId != characterId) {
            return false;
          }
          if (query.isEmpty) return true;
          return [
            row.matchPattern,
            row.injectContent,
            row.matchType,
            row.matchScope,
          ].any((value) => value.toLowerCase().contains(query));
        })
        .toList(growable: false);
    final start = ((page < 1 ? 1 : page) - 1) * pageSize;
    if (start >= rows.length) return const [];
    return rows.skip(start).take(pageSize).toList(growable: false);
  }

  Future<WorldBookDto?> create(Map<String, dynamic> data) async {
    await _loadWithScope();
    final role = _owned.selectRole(null);
    final scope =
        (await _owned.data('worldbook', characterId: role))['executionScope']
            as Map;
    final normalized = Map<String, dynamic>.from(data);
    normalized['scope'] ??=
        (normalized['characterId'] ?? '').toString().trim().isEmpty
        ? 'global'
        : 'role';
    final result = await _manage(
      scope,
      action: 'create',
      rule: normalized,
      expectedRevision: 0,
    );
    final saved = result['rule'];
    return saved is Map
        ? WorldBookDto.fromJson(Map<String, dynamic>.from(saved))
        : null;
  }

  Future<WorldBookDto?> update(String id, Map<String, dynamic> data) async {
    final rows = await _loadWithScope();
    final existing = rows.where((row) => row.id == id).firstOrNull;
    if (existing == null) throw StateError('世界书规则已变化，请重新加载');
    final role = _owned.selectRole(null);
    final scope =
        (await _owned.data('worldbook', characterId: role))['executionScope']
            as Map;
    final result = await _manage(
      scope,
      action: 'update',
      rule: {
        ...existing.toJson(),
        ...data,
        'id': id,
        'scope': data['scope'] ?? existing.scope,
      },
      expectedRevision: existing.revision,
    );
    final saved = result['rule'];
    return saved is Map
        ? WorldBookDto.fromJson(Map<String, dynamic>.from(saved))
        : null;
  }

  Future<bool> delete(String id) async {
    final rows = await _loadWithScope();
    final existing = rows.where((row) => row.id == id).firstOrNull;
    if (existing == null) throw StateError('世界书规则已变化，请重新加载');
    final role = _owned.selectRole(null);
    final scope =
        (await _owned.data('worldbook', characterId: role))['executionScope']
            as Map;
    await _manage(
      scope,
      action: 'delete',
      rule: {'id': id, 'scope': existing.scope},
      expectedRevision: existing.revision,
    );
    return true;
  }

  Future<bool> deleteAll() async {
    final rows = await _loadWithScope();
    for (final row in rows) {
      await delete(row.id);
    }
    return true;
  }

  Future<List<WorldBookMatchDto>> testMatch(String text) async {
    final rows = await _loadWithScope();
    final role = _owned.selectRole(null);
    final view = await _owned.data('worldbook', characterId: role);
    final scope = view['executionScope'] as Map;
    final requestId = _requestId();
    final result = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/worldbook/match',
      data: {
        'requestId': requestId,
        'characterId': scope['roleId'],
        'targetDeviceId': scope['targetDeviceId'],
        'expectedExecutionScope': scope,
        'text': text,
      },
    );
    final data = result?['data'] is Map ? result!['data'] as Map : result;
    if (data?['executionScope'] is! Map ||
        _scopeStamp(
              Map<String, dynamic>.from(data!['executionScope'] as Map),
            ) !=
            _scopeStamp(Map<String, dynamic>.from(scope))) {
      throw StateError('世界书匹配归属已变化');
    }
    final matches = data['matches'];
    if (matches is! List) throw StateError('世界书匹配结果无效');
    final byId = {for (final row in rows) row.id: row};
    return matches
        .whereType<Map>()
        .map((match) {
          final entry = byId[(match['ruleId'] ?? '').toString()];
          return WorldBookMatchDto(
            entry:
                entry ??
                WorldBookDto(
                  id: (match['ruleId'] ?? '').toString(),
                  revision: (match['revision'] as num?)?.toInt() ?? 0,
                ),
            matchScope: (match['matchScope'] ?? '').toString(),
            hitText: (match['hitText'] ?? '').toString(),
          );
        })
        .toList(growable: false);
  }

  Future<Map<String, dynamic>> _manage(
    Map scope, {
    required String action,
    required Map<String, dynamic> rule,
    required int expectedRevision,
  }) async {
    final requestId = _requestId();
    final result = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/worldbook/manage',
      data: {
        'requestId': requestId,
        'characterId': scope['roleId'],
        'targetDeviceId': scope['targetDeviceId'],
        'expectedExecutionScope': scope,
        'action': action,
        'rule': {
          'id': rule['id'] ?? '',
          'scope': rule['scope'] ?? 'role',
          'characterId': rule['scope'] == 'global' ? '' : scope['roleId'],
          'matchType': rule['matchType'] ?? 'keyword',
          'matchPattern': rule['matchPattern'] ?? '',
          'matchScope': rule['matchScope'] ?? 'full_context',
          'injectContent': rule['injectContent'] ?? '',
          'priority': (rule['priority'] as num?)?.toInt() ?? 0,
          'enabled': rule['enabled'] != false,
          'secondaryKeys': rule['secondaryKeys'] ?? const <String>[],
          'caseSensitive': rule['caseSensitive'],
          'selective': rule['selective'],
          'constant': rule['constant'],
          'insertionOrder': (rule['insertionOrder'] as num?)?.toInt() ?? 0,
          'position': rule['position'],
          'extensions': rule['extensions'] ?? const <String, dynamic>{},
          'characterBookEntryName': rule['characterBookEntryName'] ?? '',
          'characterBookEntryId': rule['characterBookEntryId'],
          'sourceCardHash': rule['sourceCardHash'] ?? '',
          'sourceEntryIndex': (rule['sourceEntryIndex'] as num?)?.toInt() ?? 0,
          'createdAt': rule['createdAt'] ?? '',
          'updatedAt': rule['updatedAt'] ?? '',
        },
        'expectedRevision': expectedRevision,
      },
    );
    final data = result?['data'] is Map ? result!['data'] as Map : result;
    final acknowledgement = data?['acknowledgement'];
    final responseScope = data?['executionScope'];
    if (data?['saved'] != true ||
        acknowledgement is! Map ||
        acknowledgement['ownerId'] != scope['resourceOwnerId'] ||
        acknowledgement['requestId'] != requestId ||
        acknowledgement['versions'] is! Map ||
        (acknowledgement['versions']
                as Map)['checkpoint/worldbook/$requestId'] !=
            1 ||
        responseScope is! Map ||
        _scopeStamp(Map<String, dynamic>.from(responseScope)) !=
            _scopeStamp(Map<String, dynamic>.from(scope))) {
      throw StateError('世界书尚未获得数据所有者保存确认');
    }
    return Map<String, dynamic>.from(data!);
  }
}
