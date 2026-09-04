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
    bool? pinned,
    int page = 1,
    int pageSize = 200,
  }) async {
    final resp = await _api.get<dynamic>(
      '/api/memories',
      queryParameters: <String, dynamic>{
        'page': page,
        'pageSize': pageSize,
        if (characterId != null && characterId.isNotEmpty) 'characterId': characterId,
        if (keyword != null && keyword.trim().isNotEmpty) 'keyword': keyword.trim(),
        if (memoryType != null && memoryType.isNotEmpty) 'memoryType': memoryType,
        if (verifiedStatus != null && verifiedStatus.isNotEmpty) 'verifiedStatus': verifiedStatus,
        if (retentionLevel != null && retentionLevel >= 1 && retentionLevel <= 5) 'retentionLevel': retentionLevel,
        if (decayState != null && decayState.isNotEmpty) 'decayState': decayState,
        if (pinned != null) 'pinned': pinned,
      },
    );
    return _memoryList(resp);
  }

  Future<MemoryDto?> create(Map<String, dynamic> data) async {
    final normalized = _normalizeWritePayload(data, isCreate: true);
    final resp = await _api.post<Map<String, dynamic>>('/api/memories', data: normalized);
    if (resp == null) return null;
    return MemoryDto.fromJson(resp);
  }

  Future<MemoryDto?> update(String id, Map<String, dynamic> data) async {
    final normalized = _normalizeWritePayload(data, isCreate: false);
    final resp = await _api.put<Map<String, dynamic>>('/api/memories/$id', data: normalized);
    if (resp == null) return null;
    return MemoryDto.fromJson(resp);
  }

  Future<MemoryDto?> restore(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/memories/$id/restore');
    if (resp == null) return null;
    return MemoryDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/memories/$id');
    return true;
  }

  Future<bool> deleteAll({String? characterId}) async {
    await _api.delete(
      '/api/memories',
      queryParameters: characterId == null || characterId.isEmpty ? null : {'characterId': characterId},
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
        if (characterId != null && characterId.isNotEmpty) 'characterId': characterId,
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
        if (characterId != null && characterId.isNotEmpty) 'characterId': characterId,
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
        if (characterId != null && characterId.isNotEmpty) 'characterId': characterId,
        'limit': limit,
      },
    );
    return _scoredMemoryList(resp);
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
        if (characterId != null && characterId.isNotEmpty) 'characterId': characterId,
        if (source != null && source.isNotEmpty) 'source': source,
        if (memoryType != null && memoryType.isNotEmpty) 'memoryType': memoryType,
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
    final all = _mapList(resp, keys: const ['candidates'])
        .map(MemoryCandidateDto.fromJson)
        .toList(growable: false);
    if (characterId == null || characterId.isEmpty) return all;
    return all.where((e) => e.characterId == characterId).toList(growable: false);
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

  Future<bool> batchVerify(List<String> ids, {String status = 'user_verified'}) async {
    await _api.post('/api/memories/batch-verify', data: {'ids': ids, 'status': status});
    return true;
  }

  Future<bool> batchSetImportance(List<String> ids, int importance) async {
    await _api.post('/api/memories/batch-importance', data: {'ids': ids, 'importance': importance});
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

  Future<List<MemoryCandidateDto>> generateCandidates(String conversationId) async {
    final resp = await _api.post<dynamic>(
      '/api/memory-candidates/generate',
      data: {'conversationId': conversationId},
    );
    return _mapList(resp, keys: const ['candidates'])
        .map(MemoryCandidateDto.fromJson)
        .toList(growable: false);
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
    String? userId,
    String query = '',
    int limit = 20,
  }) async {
    final resp = await _api.get<dynamic>(
      '/api/memories/ranked',
      queryParameters: {
        if (characterId != null && characterId.isNotEmpty) 'characterId': characterId,
        if (userId != null && userId.isNotEmpty) 'userId': userId,
        if (query.trim().isNotEmpty) 'query': query.trim(),
        'limit': limit,
      },
    );
    return _mapList(resp);
  }

  Future<Map<String, dynamic>> retrievalStats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/memory/retrieval/stats');
    return resp ?? <String, dynamic>{};
  }

  Future<bool> recordUse(String id) async {
    await _api.post('/api/memories/$id/use');
    return true;
  }

  Map<String, dynamic> _normalizeWritePayload(Map<String, dynamic> data, {required bool isCreate}) {
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

  List<MemoryDto> _memoryList(dynamic resp) {
    return _mapList(resp, keys: const ['items'])
        .map(MemoryDto.fromJson)
        .toList(growable: false);
  }

  List<MemoryDto> _scoredMemoryList(dynamic resp) {
    return _mapList(resp, keys: const ['items'])
        .map((row) {
          final nested = row['memory'];
          if (nested is Map) return MemoryDto.fromJson(Map<String, dynamic>.from(nested));
          return MemoryDto.fromJson(row);
        })
        .toList(growable: false);
  }

  List<Map<String, dynamic>> _mapList(dynamic resp, {List<String> keys = const []}) {
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
}
