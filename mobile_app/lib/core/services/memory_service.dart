import 'dart:convert';
import 'dart:math';

import '../backend_transport/backend_service_api.dart';
import '../models/memory.dart';

class MemoryService {
  final BackendServiceApi _api;

  MemoryService(this._api);

  Future<List<MemoryDto>> list({
    String? characterId,
    String? keyword,
    String? memoryType,
    String? verifiedStatus,
    int? retentionLevel,
    String? decayState,
    String? sort,
    bool? pinned,
    int page = 1,
    int pageSize = 200,
  }) async {
    final resources = await _ownedResources(characterId, 'memory');
    final search = (keyword ?? '').trim().toLowerCase();
    final rows = <MemoryDto>[];
    for (final resource in resources) {
      final body = resource['body'] is Map
          ? Map<String, dynamic>.from(resource['body'] as Map)
          : <String, dynamic>{};
      final content = body['content'];
      final key = (body['key'] ?? '').toString();
      final value = content is Map
          ? (content['value'] ??
                    content['text'] ??
                    content['summary'] ??
                    jsonEncode(content))
                .toString()
          : (content ?? '').toString();
      final type = content is Map ? (content['memoryType'] ?? '').toString() : '';
      if (search.isNotEmpty && !'$key $value'.toLowerCase().contains(search)) {
        continue;
      }
      if (memoryType != null && memoryType.isNotEmpty && type != memoryType) {
        continue;
      }
      rows.add(_memoryFromResource(resource, body, content, key, value, type));
    }
    return rows;
  }

  Future<MemoryDto?> create(Map<String, dynamic> data) async {
    final normalized = _normalizeWritePayload(data, isCreate: true);
    final characterId = (data['characterId'] ?? '').toString();
    final key = (normalized['key'] ?? '').toString();
    final value = (normalized['value'] ?? '').toString();
    if (key.isEmpty || value.isEmpty) return null;
    final scope = await _ownedMemoryScope(characterId);
    if (scope == null) return null;
    await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memories/manage',
      data: <String, dynamic>{
        'action': 'create',
        'key': key,
        'value': value,
        'memoryType': (normalized['memoryType'] ?? 'fact').toString(),
        'importance': normalized['importance'] is num
            ? (normalized['importance'] as num).toInt()
            : 5,
        'requestId': _requestId(),
        'characterId': characterId,
        'expectedExecutionScope': scope,
      },
    );
    return MemoryDto(
      id: key,
      characterId: characterId,
      key: key,
      value: value,
      memoryType: (normalized['memoryType'] ?? 'fact').toString(),
    );
  }

  Future<MemoryDto?> update(String id, Map<String, dynamic> data) async {
    final normalized = _normalizeWritePayload(data, isCreate: false);
    final characterId = (data['characterId'] ?? '').toString();
    await _ownedEdit(
      id,
      characterId,
      changes: <String, dynamic>{
        'content': <String, dynamic>{'value': normalized['value']},
      },
    );
    return MemoryDto(
      id: id,
      characterId: characterId,
      key: (normalized['key'] ?? '').toString(),
      value: (normalized['value'] ?? '').toString(),
      memoryType: (normalized['memoryType'] ?? 'custom').toString(),
      importance: normalized['importance'] is num
          ? (normalized['importance'] as num).toInt()
          : 0,
    );
  }

  Future<MemoryDto?> restore(String id) async {
    await _ownedEdit(id, '', deleted: false);
    return MemoryDto(id: id);
  }

  Future<bool> delete(String id) async {
    await _ownedEdit(id, '', deleted: true);
    return true;
  }


  Future<List<MemoryDto>> search(
    String query, {
    String? characterId,
    int limit = 100,
    List<String>? types,
  }) async {
    final rows = await _ownedSearch(
      query,
      characterId: characterId,
      limit: limit,
      mode: 'keyword',
    );
    if (types == null || types.isEmpty) return rows;
    return rows.where((row) => types.contains(row.memoryType)).toList();
  }

  Future<List<MemoryDto>> vectorSearch(
    String query, {
    String? characterId,
    int limit = 100,
  }) async {
    return _ownedSearch(
      query,
      characterId: characterId,
      limit: limit,
      mode: 'vector',
    );
  }

  Future<List<MemoryDto>> hybridSearch(
    String query, {
    String? characterId,
    int limit = 100,
  }) async {
    return _ownedSearch(
      query,
      characterId: characterId,
      limit: limit,
      mode: 'hybrid',
    );
  }

  Future<Map<String, List<Map<String, dynamic>>>> globalSearch(
    String query, {
    int limitPerType = 5,
  }) async {
    final keyword = query.trim();
    final result = <String, List<Map<String, dynamic>>>{};
    if (keyword.isEmpty) return result;
    for (final kind in const ['memory', 'episodic', 'fact', 'profile']) {
      final resources = await _ownedResources(null, kind);
      final hits = <Map<String, dynamic>>[];
      for (final resource in resources) {
        final body = resource['body'] is Map
            ? Map<String, dynamic>.from(resource['body'] as Map)
            : <String, dynamic>{};
        final content = body['content'];
        final text = content is Map ? jsonEncode(content) : (content ?? '');
        if (!'${body['key']} $text'
            .toLowerCase()
            .contains(keyword.toLowerCase())) {
          continue;
        }
        hits.add(<String, dynamic>{
          'id': resource['id']?.toString() ?? '',
          'key': (body['key'] ?? '').toString(),
          'value': text.toString(),
          'memoryType': kind,
        });
        if (hits.length >= limitPerType) break;
      }
      if (hits.isNotEmpty) result[kind] = hits;
    }
    return result;
  }

  Future<List<Map<String, dynamic>>> timeline({
    String? characterId,
    int limit = 200,
    String? source,
    String? memoryType,
    String? type,
  }) async {
    final rows = <Map<String, dynamic>>[];
    for (final kind in const [
      'memory',
      'episodic',
      'fact',
      'graph',
      'profile',
    ]) {
      for (final resource in await _ownedResources(characterId, kind)) {
        final body = resource['body'] is Map
            ? Map<String, dynamic>.from(resource['body'] as Map)
            : <String, dynamic>{};
        final content = body['content'];
        final value = content is Map
            ? (content['value'] ??
                      content['text'] ??
                      content['event'] ??
                      content['summary'] ??
                      '')
                  .toString()
            : (content ?? '').toString();
        if (source != null && source.isNotEmpty && source != kind) continue;
        rows.add(<String, dynamic>{
          'id': resource['id']?.toString() ?? '',
          'event_type': 'memory_created',
          'key': (body['key'] ?? '').toString(),
          'value': value,
          'source': kind,
          'memory_type': content is Map
              ? (content['memoryType'] ?? '').toString()
              : '',
          'created_at': (resource['updatedAt'] ?? '').toString(),
        });
      }
    }
    rows.sort(
      (left, right) => (right['created_at']?.toString() ?? '').compareTo(
        left['created_at']?.toString() ?? '',
      ),
    );
    if (rows.length > limit) return rows.sublist(0, limit);
    return rows;
  }

  Future<Map<String, dynamic>?> rebuildIndex() async {
    final characterId = await _firstCharacterId();
    if (characterId.isEmpty) return null;
    return _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/projections/rebuild',
      data: <String, dynamic>{'characterId': characterId},
    );
  }

  Future<Map<String, dynamic>?> rebuildEmbeddings() async {
    return rebuildIndex();
  }


  Future<List<MemoryCandidateDto>> listCandidates({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memory-candidates',
      queryParameters: <String, dynamic>{
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        'targetDeviceId': '',
      },
    );
    return _candidateList(resp);
  }








  Future<List<MemoryCandidateDto>> extractCandidates() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memory-candidates',
      queryParameters: <String, dynamic>{'targetDeviceId': ''},
    );
    return _candidateList(resp);
  }

  Future<List<MemoryCandidateDto>> generateCandidates(
    String characterId, {
    int limit = 10,
  }) async {
    final scope = await _ownedMemoryScope(characterId);
    if (scope == null) return const [];
    await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memory-candidates/manage',
      data: <String, dynamic>{
        'action': 'generate',
        'requestId': _requestId(),
        'characterId': characterId,
        'expectedExecutionScope': scope,
      },
    );
    return listCandidates(characterId: characterId);
  }



  Future<List<Map<String, dynamic>>> ranked({
    String query = '',
    String? characterId,
    int limit = 50,
  }) async {
    final rows = await _ownedSearch(
      query,
      characterId: characterId,
      limit: limit,
      mode: query.trim().isEmpty ? 'keyword' : 'hybrid',
    );
    return rows
        .map(
          (row) => <String, dynamic>{
            'id': row.id,
            'key': row.key,
            'value': row.value,
            'memoryType': row.memoryType,
            'score': 1.0,
          },
        )
        .toList(growable: false);
  }



  Map<String, dynamic> _normalizeWritePayload(
    Map<String, dynamic> data, {
    required bool isCreate,
  }) {
    final result = Map<String, dynamic>.from(data);
    if (result.containsKey('content') && !result.containsKey('value')) {
      result['value'] = result.remove('content');
    }
    if (result.containsKey('type') && !result.containsKey('memoryType')) {
      result['memoryType'] = result.remove('type');
    }
    if (result.containsKey('status') && !result.containsKey('verifiedStatus')) {
      result['verifiedStatus'] = result.remove('status');
    }
    if (isCreate) {
      final value = (result['value'] ?? '').toString().trim();
      result['value'] = value;
      final key = (result['key'] ?? '').toString().trim();
      result['key'] = key.isEmpty ? _deriveKey(value) : key;
      result.putIfAbsent('memoryType', () => 'custom');
      result.putIfAbsent('source', () => 'manual');
      result.putIfAbsent('scope', () => 'character');
      result.putIfAbsent('verifiedStatus', () => 'user_verified');
      result.putIfAbsent('confidence', () => 100);
      result.putIfAbsent('allowProactiveMention', () => true);
      result.putIfAbsent('requiresConfirmation', () => false);
    }
    return result;
  }

  String _deriveKey(String value) {
    final normalized = value.replaceAll(RegExp(r'\s+'), ' ').trim();
    if (normalized.isEmpty) return 'manual-memory';
    return normalized.length <= 60 ? normalized : normalized.substring(0, 60);
  }

  bool _mapContainsQuery(Map<String, dynamic> row, String query) {
    final lower = query.toLowerCase();
    for (final value in row.values) {
      if (value == null) continue;
      if (value is Map || value is List) {
        if (jsonEncode(value).toLowerCase().contains(lower)) return true;
      } else if (value.toString().toLowerCase().contains(lower)) {
        return true;
      }
    }
    return false;
  }

  List<MemoryDto> _memoryList(dynamic resp) {
    return _mapList(
      resp,
      keys: const ['items'],
    ).map(MemoryDto.fromJson).toList(growable: false);
  }

  List<MemoryDto> _scoredMemoryList(dynamic resp) {
    return _mapList(resp, keys: const ['items'])
        .map((row) {
          final nested = row['memory'];
          if (nested is Map)
            return MemoryDto.fromJson(Map<String, dynamic>.from(nested));
          return MemoryDto.fromJson(row);
        })
        .toList(growable: false);
  }

  List<Map<String, dynamic>> _mapList(
    dynamic resp, {
    List<String> keys = const [],
  }) {
    dynamic raw = resp;
    if (raw is Map) {
      for (final key in keys) {
        if (raw[key] is List) {
          raw = raw[key];
          break;
        }
      }
    }
    if (raw is! List) return const [];
    return raw
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList(growable: false);
  }

  Future<String> _firstCharacterId() async {
    final roles = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/roles',
    );
    final list = roles?['roles'];
    return list is List && list.isNotEmpty && list.first is Map
        ? ((list.first as Map)['id'] ?? '').toString()
        : '';
  }

  List<MemoryCandidateDto> _candidateList(Map<String, dynamic>? resp) {
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final resources = payload is Map && payload['resources'] is List
        ? payload['resources'] as List
        : const [];
    final rows = <MemoryCandidateDto>[];
    for (final raw in resources) {
      if (raw is! Map) continue;
      final row = Map<String, dynamic>.from(raw);
      final body = row['body'] is Map
          ? Map<String, dynamic>.from(row['body'] as Map)
          : <String, dynamic>{};
      rows.add(
        MemoryCandidateDto.fromJson(<String, dynamic>{
          'id': row['id']?.toString() ?? '',
          'characterId': (row['roleId'] ?? '').toString(),
          'key': (body['key'] ?? body['value'] ?? '').toString(),
          'value': (body['value'] ?? '').toString(),
          'memoryType': (body['memoryType'] ?? 'custom').toString(),
          'state': (body['state'] ?? 'pending').toString(),
        }),
      );
    }
    return rows;
  }

  Future<bool> acceptCandidate(String id) async {
    await _manageCandidate('accept', id);
    return true;
  }

  Future<bool> rejectCandidate(String id) async {
    await _manageCandidate('reject', id);
    return true;
  }

  Future<void> _manageCandidate(String action, String id) async {
    final characterId = await _firstCharacterId();
    final scope = await _ownedMemoryScope(characterId);
    if (scope == null) throw StateError('记忆候选服务尚未就绪');
    await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memory-candidates/manage',
      data: <String, dynamic>{
        'action': action,
        'id': id,
        'requestId': _requestId(),
        'characterId': characterId,
        'expectedExecutionScope': scope,
      },
    );
  }

  Future<Map<String, dynamic>?> _ownedMemoryScope(String characterId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memories',
      queryParameters: <String, dynamic>{
        if (characterId.isNotEmpty) 'characterId': characterId,
        'query': '',
        'mode': 'keyword',
        'limit': 1,
        'targetDeviceId': '',
      },
    );
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final scope = payload is Map ? payload['executionScope'] : null;
    return scope is Map ? Map<String, dynamic>.from(scope) : null;
  }

  Future<bool> deleteCandidate(String id) async {
    await _manageCandidate('reject', id);
    return true;
  }

  Future<bool> batchAcceptCandidates(List<String> ids) async {
    for (final id in ids) {
      await _manageCandidate('accept', id);
    }
    return true;
  }

  Future<MemoryCandidateDto?> updateCandidate(
    String id, {
    String? key,
    String? value,
    String? memoryType,
  }) async {
    final rows = await listCandidates();
    for (final row in rows) {
      if (row.id == id) return row;
    }
    return null;
  }

  Future<bool> batchSetImportance(List<String> ids, int importance) async {
    return false;
  }

  Future<Map<String, dynamic>> checkConflict({
    required String key,
    String? value,
    String? memoryType,
    int? importance,
  }) async {
    final rows = await list(keyword: key);
    final conflicts = rows
        .where((row) => row.key == key)
        .map(
          (row) => <String, dynamic>{
            'id': row.id,
            'key': row.key,
            'value': row.value,
            'memoryType': row.memoryType,
          },
        )
        .toList();
    return <String, dynamic>{
      'hasConflict': conflicts.isNotEmpty,
      'conflicts': conflicts,
    };
  }

  Future<Map<String, dynamic>> resolveConflict({
    required String action,
    required String newKey,
    required String newValue,
    String? newType,
    int? importance,
    String? conflictId,
    String? characterId,
  }) async {
    if (action == 'cancel') return <String, dynamic>{'memoryId': ''};
    final created = await create(<String, dynamic>{
      'characterId': characterId ?? '',
      'key': action == 'keep_both' ? '$newKey (新)' : newKey,
      'value': newValue,
      'memoryType': newType ?? 'custom',
      if (importance != null) 'importance': importance,
    });
    return <String, dynamic>{'memoryId': created?.id ?? newKey};
  }

  Future<List<MemoryDto>> _ownedSearch(
    String query, {
    String? characterId,
    int limit = 100,
    String mode = 'keyword',
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memories',
      queryParameters: <String, dynamic>{
        'characterId': characterId ?? '',
        'query': query.trim(),
        'mode': mode,
        'limit': limit > 128 ? 128 : limit,
        'targetDeviceId': '',
      },
    );
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final resources = payload is Map && payload['resources'] is List
        ? payload['resources'] as List
        : const [];
    final rows = <MemoryDto>[];
    for (final raw in resources) {
      if (raw is! Map) continue;
      final resource = Map<String, dynamic>.from(raw);
      final body = resource['body'] is Map
          ? Map<String, dynamic>.from(resource['body'] as Map)
          : <String, dynamic>{};
      final content = body['content'];
      final value = content is Map
          ? (content['value'] ??
                    content['text'] ??
                    content['summary'] ??
                    jsonEncode(content))
                .toString()
          : (content ?? '').toString();
      final type = content is Map ? (content['memoryType'] ?? '').toString() : '';
      final roleId = (resource['roleId'] ?? '').toString();
      final stamp = (resource['updatedAt'] ?? '').toString();
      rows.add(
        MemoryDto(
          id: resource['id']?.toString() ?? '',
          characterId: roleId,
          memoryType: type.isEmpty ? 'custom' : type,
          source: content is Map
              ? (content['source'] ?? 'auto').toString()
              : 'auto',
          scope: roleId.startsWith('@') ? 'user' : 'character',
          key: (body['key'] ?? '').toString(),
          value: value,
          importance: content is Map && content['importance'] is num
              ? (content['importance'] as num).toInt()
              : 0,
          verifiedStatus: 'verified',
          archivedAt: body['archivedAt']?.toString(),
          allowContextUse: body['allowContextUse'] != false,
          createdAt: content is Map
              ? (content['createdAt'] ?? stamp).toString()
              : stamp,
          updatedAt: stamp,
        ),
      );
    }
    return rows;
  }


  Future<List<Map<String, dynamic>>> _ownedResources(
    String? characterId,
    String kind, {
    String memoryView = 'all',
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/data',
      queryParameters: <String, dynamic>{
        'kind': kind,
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        'memoryView': memoryView,
      },
    );
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final snapshot = payload is Map ? payload['snapshot'] : null;
    final resources = snapshot is Map && snapshot['resources'] is List
        ? snapshot['resources'] as List
        : const [];
    return resources
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .where((row) => row['kind'] == kind)
        .toList(growable: false);
  }

  MemoryDto _memoryFromResource(
    Map<String, dynamic> resource,
    Map<String, dynamic> body,
    Object? content,
    String key,
    String value,
    String type,
  ) {
    final roleId = (resource['roleId'] ?? '').toString();
    final stamp = (resource['updatedAt'] ?? '').toString();
    return MemoryDto(
      id: resource['id']?.toString() ?? '',
      characterId: roleId,
      memoryType: type.isEmpty ? 'custom' : type,
      source: content is Map ? (content['source'] ?? 'auto').toString() : 'auto',
      scope: roleId.startsWith('@') ? 'user' : 'character',
      key: key,
      value: value,
      importance: content is Map && content['importance'] is num
          ? (content['importance'] as num).toInt()
          : 0,
      verifiedStatus: 'verified',
      decayState: (body['decayState'] ?? 'active').toString(),
      archivedAt: body['archivedAt']?.toString(),
      allowContextUse: body['allowContextUse'] != false,
      createdAt: content is Map
          ? (content['createdAt'] ?? stamp).toString()
          : stamp,
      updatedAt: stamp,
    );
  }

  Future<Map<String, dynamic>> _ownedResourceById(
    String id,
    String kind,
    String characterId,
  ) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources',
      queryParameters: <String, dynamic>{
        'kind': kind,
        'id': id,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    return payload is Map
        ? Map<String, dynamic>.from(payload)
        : <String, dynamic>{};
  }

  Future<void> _ownedEdit(
    String id,
    String characterId, {
    Map<String, dynamic>? changes,
    bool? deleted,
  }) async {
    final payload = await _ownedResourceById(id, 'memory', characterId);
    final resource = payload['resource'];
    final scope = payload['executionScope'];
    if (resource is! Map || scope is! Map) {
      throw StateError('该记忆不在当前数据来源中');
    }
    await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources/edit',
      data: <String, dynamic>{
        'requestId': _requestId(),
        'kind': 'memory',
        'id': id,
        if (characterId.isNotEmpty) 'characterId': characterId,
        'expectedRevision': resource['revision'],
        'expectedExecutionScope': scope,
        if (deleted != null) 'deleted': deleted,
        if (changes != null) 'changes': changes,
      },
    );
  }

  static String _requestId() {
    final values = List<int>.generate(16, (_) => Random.secure().nextInt(256));
    values[6] = (values[6] & 15) | 64;
    values[8] = (values[8] & 63) | 128;
    final value = values
        .map((byte) => byte.toRadixString(16).padLeft(2, '0'))
        .join();
    return '${value.substring(0, 8)}-${value.substring(8, 12)}-'
        '${value.substring(12, 16)}-${value.substring(16, 20)}-'
        '${value.substring(20)}';
  }
}

