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
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/memories',
      data: normalized,
    );
    if (resp == null) return null;
    return MemoryDto.fromJson(resp);
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

  Future<bool> deleteAll({String? characterId}) async {
    await _api.delete(
      '/api/memories',
      queryParameters: characterId == null || characterId.isEmpty
          ? null
          : {'characterId': characterId},
    );
    return true;
  }

  Future<List<MemoryDto>> search(
    String query, {
    String? characterId,
    int limit = 100,
    List<String>? types,
  }) async {
    final resp = await _api.post<dynamic>(
      '/api/memories/search',
      data: <String, dynamic>{
        'keyword': query.trim(),
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        'limit': limit,
        if (types != null && types.isNotEmpty) 'types': types,
      },
    );
    return _memoryList(resp);
  }

  Future<List<MemoryDto>> vectorSearch(
    String query, {
    String? characterId,
    int limit = 100,
  }) async {
    final resp = await _api.post<dynamic>(
      '/api/memories/vector-search',
      data: <String, dynamic>{
        'query': query,
        'keyword': query,
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        'limit': limit,
      },
    );
    return _scoredMemoryList(resp);
  }

  Future<List<MemoryDto>> hybridSearch(
    String query, {
    String? characterId,
    int limit = 100,
  }) async {
    final resp = await _api.post<dynamic>(
      '/api/memories/hybrid-search',
      data: <String, dynamic>{
        'query': query,
        'keyword': query,
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        'limit': limit,
      },
    );
    return _scoredMemoryList(resp);
  }

  Future<Map<String, List<Map<String, dynamic>>>> globalSearch(
    String query, {
    int limitPerType = 5,
  }) async {
    final keyword = query.trim();
    if (keyword.isEmpty) {
      return const {
        'memories': <Map<String, dynamic>>[],
        'profiles': <Map<String, dynamic>>[],
        'episodics': <Map<String, dynamic>>[],
        'worldBooks': <Map<String, dynamic>>[],
      };
    }

    var memories = <Map<String, dynamic>>[];
    var profiles = <Map<String, dynamic>>[];
    var episodics = <Map<String, dynamic>>[];
    var worldBooks = <Map<String, dynamic>>[];

    try {
      final memoryResp = await _api.post<dynamic>(
        '/api/memories/hybrid-search',
        data: {'keyword': keyword, 'query': keyword, 'limit': limitPerType},
      );
      memories = _mapList(memoryResp, keys: const ['items'])
          .map((row) {
            final nested = row['memory'];
            final memory = nested is Map
                ? Map<String, dynamic>.from(nested)
                : Map<String, dynamic>.from(row);
            memory['score'] = row['score'];
            memory['matchType'] = row['matchType'];
            memory['memoryLayer'] = row['memoryLayer'];
            return memory;
          })
          .take(limitPerType)
          .toList(growable: false);
    } catch (_) {}

    Future<List<Map<String, dynamic>>> fetchAndFilter(String path) async {
      try {
        final resp = await _api.get<dynamic>(
          path,
          queryParameters: {'keyword': keyword, 'page': 1, 'pageSize': 100},
        );
        return _mapList(resp, keys: const ['items'])
            .where((row) => _mapContainsQuery(row, keyword))
            .take(limitPerType)
            .toList(growable: false);
      } catch (_) {
        return <Map<String, dynamic>>[];
      }
    }

    profiles = await fetchAndFilter('/api/profiles');
    episodics = await fetchAndFilter('/api/episodic');
    worldBooks = await fetchAndFilter('/api/world-book');

    return {
      'memories': memories,
      'profiles': profiles,
      'episodics': episodics,
      'worldBooks': worldBooks,
    };
  }

  Future<List<Map<String, dynamic>>> timeline({
    String? characterId,
    int page = 1,
    int pageSize = 100,
    String? source,
    String? memoryType,
    String? type,
  }) async {
    final resp = await _api.get<dynamic>(
      '/api/memories/timeline',
      queryParameters: <String, dynamic>{
        'page': page,
        'pageSize': pageSize,
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        if (source != null && source.isNotEmpty) 'source': source,
        if (memoryType != null && memoryType.isNotEmpty)
          'memoryType': memoryType,
        if (type != null && type.isNotEmpty) 'type': type,
      },
    );
    return _mapList(resp, keys: const ['items']);
  }

  Future<Map<String, dynamic>?> rebuildIndex() {
    return _api.post<Map<String, dynamic>>('/api/memories/rebuild-index');
  }

  Future<Map<String, dynamic>?> rebuildEmbeddings() {
    return _api.post<Map<String, dynamic>>('/api/memories/rebuild-embeddings');
  }

  Future<Map<String, dynamic>?> vectorStatus() {
    return _api.get<Map<String, dynamic>>('/api/memories/vector-status');
  }

  Future<List<MemoryCandidateDto>> listCandidates({String? characterId}) async {
    final resp = await _api.get<dynamic>('/api/memory-candidates');
    final all = _mapList(
      resp,
      keys: const ['candidates'],
    ).map(MemoryCandidateDto.fromJson).toList(growable: false);
    if (characterId == null || characterId.isEmpty) return all;
    return all
        .where((e) => e.characterId == characterId)
        .toList(growable: false);
  }

  Future<bool> acceptCandidate(String id) async {
    await _api.post('/api/memory-candidates/$id/accept');
    return true;
  }

  Future<bool> rejectCandidate(String id) async {
    await _api.post('/api/memory-candidates/$id/reject');
    return true;
  }

  Future<bool> batchAcceptCandidates(List<String> ids) async {
    await _api.post('/api/memory-candidates/batch-accept', data: {'ids': ids});
    return true;
  }

  Future<bool> batchVerify(
    List<String> ids, {
    String status = 'user_verified',
  }) async {
    await _api.post(
      '/api/memories/batch-verify',
      data: {'ids': ids, 'status': status},
    );
    return true;
  }

  Future<bool> batchSetImportance(List<String> ids, int importance) async {
    await _api.post(
      '/api/memories/batch-importance',
      data: {'ids': ids, 'importance': importance},
    );
    return true;
  }

  Future<Map<String, dynamic>> checkConflict({
    required String key,
    required String value,
    required String memoryType,
    required int importance,
    String characterId = '',
  }) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/memories/check-conflict',
      data: {
        'key': key,
        'value': value,
        'memoryType': memoryType,
        'importance': importance,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    return resp ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> resolveConflict({
    required String action,
    required String newKey,
    required String newValue,
    required String newType,
    required int importance,
    required String conflictId,
    String characterId = '',
  }) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/memories/resolve-conflict',
      data: {
        'action': action,
        'newKey': newKey,
        'newValue': newValue,
        'newType': newType,
        'importance': importance,
        'conflictId': conflictId,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    return resp ?? <String, dynamic>{};
  }

  Future<List<MemoryCandidateDto>> extractCandidates() async {
    final resp = await _api.post<dynamic>('/api/memories/extract-candidates');
    return _mapList(
      resp,
      keys: const ['candidates'],
    ).map(MemoryCandidateDto.fromJson).toList(growable: false);
  }

  Future<List<MemoryCandidateDto>> generateCandidates(
    String conversationId,
  ) async {
    final resp = await _api.post<dynamic>(
      '/api/memory-candidates/generate',
      data: {'conversationId': conversationId},
    );
    return _mapList(
      resp,
      keys: const ['candidates'],
    ).map(MemoryCandidateDto.fromJson).toList(growable: false);
  }

  Future<MemoryCandidateDto?> updateCandidate(
    String id, {
    String? key,
    String? value,
    String? memoryType,
    int? importance,
  }) async {
    final resp = await _api.put<Map<String, dynamic>>(
      '/api/memory-candidates/$id',
      data: {
        if (key != null) 'key': key,
        if (value != null) 'value': value,
        if (memoryType != null) 'memoryType': memoryType,
        if (importance != null) 'importance': importance,
      },
    );
    return resp == null ? null : MemoryCandidateDto.fromJson(resp);
  }

  Future<bool> deleteCandidate(String id) async {
    await _api.delete('/api/memory-candidates/$id');
    return true;
  }

  Future<List<Map<String, dynamic>>> ranked({
    String? characterId,
    String? spaceId,
    String query = '',
    int limit = 20,
  }) async {
    final resp = await _api.get<dynamic>(
      '/api/memories/ranked',
      queryParameters: {
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        if (spaceId != null && spaceId.isNotEmpty) 'spaceId': spaceId,
        if (query.trim().isNotEmpty) 'query': query.trim(),
        'limit': limit,
      },
    );
    return _mapList(resp);
  }

  Future<Map<String, dynamic>> retrievalStats() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/memory/retrieval/stats',
    );
    return resp ?? <String, dynamic>{};
  }

  Future<bool> recordUse(String id) async {
    await _api.post('/api/memories/$id/use');
    return true;
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

