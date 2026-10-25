import '../backend_transport/backend_service_api.dart';
import '../models/memory.dart';

class MemoryPage {
  const MemoryPage({required this.items, required this.nextCursor});

  final List<MemoryDto> items;
  final String nextCursor;

  bool get hasMore => nextCursor.isNotEmpty;
}

class MemoryService {
  final BackendServiceApi _api;
  MemoryService(this._api);

  Future<List<MemoryDto>> list({
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
    final rows = <MemoryDto>[];
    var cursor = '';
    final visited = <String>{};
    final target = page * pageSize;
    do {
      final response = await _api.get<Map<String, dynamic>>(
        '/api/device-mesh/v1/business/memories',
        queryParameters: {
          if (characterId != null && characterId.isNotEmpty)
            'characterId': characterId,
          'query': keyword ?? '',
          'mode': 'keyword',
          'memoryView': 'all',
          if (memoryType != null && memoryType.isNotEmpty)
            'memoryType': memoryType,
          if (retentionLevel != null && retentionLevel > 0)
            'retentionLevel': retentionLevel,
          if (decayState != null && decayState.isNotEmpty)
            'decayState': decayState,
          if (pinned != null) 'pinned': pinned,
          if (sort != null && sort.isNotEmpty) 'sort': sort,
          'cursor': cursor,
          'limit': 128,
        },
      );
      if (response == null || response['resources'] is! List)
        throw StateError('记忆服务返回无效数据');
      for (final resource in (response['resources'] as List).whereType<Map>()) {
        if (resource['kind'] != 'memory' || resource['body'] is! Map)
          throw StateError('记忆资源类型无效');
        final body = Map<String, dynamic>.from(resource['body'] as Map);
        final content = body['content'];
        final metadata = content is Map
            ? Map<String, dynamic>.from(content)
            : <String, dynamic>{'value': content ?? ''};
        rows.add(
          MemoryDto.fromJson({
            ...metadata,
            ...body,
            'id': resource['id'],
            'characterId': resource['roleId'],
            'scope': resource['roleId'] == '@global-memory'
                ? 'user'
                : 'character',
            'value':
                metadata['value'] ??
                metadata['text'] ??
                metadata['summary'] ??
                '',
            'updatedAt': resource['updatedAt'] ?? '',
            'memoryStrength':
                body['effectiveStrength'] ?? body['memoryStrength'],
            'retentionLevel':
                body['effectiveRetentionLevel'] ?? body['retentionLevel'],
            'decayState': body['effectiveDecayState'] ?? body['decayState'],
          }),
        );
      }
      cursor = (response['nextCursor'] ?? '').toString();
      if (cursor.isNotEmpty && !visited.add(cursor))
        throw StateError('记忆分页游标重复');
      if (visited.length > 256) throw StateError('记忆结果超过分页上限');
    } while (cursor.isNotEmpty && rows.length < target);
    final offset = (page - 1) * pageSize;
    if (offset >= rows.length) return [];
    return rows.sublist(offset, target > rows.length ? rows.length : target);
  }

  Future<MemoryPage> listPage({
    String cursor = '',
    int limit = 10,
    String? characterId,
    String? sort,
  }) async {
    if (limit < 1) throw ArgumentError('记忆分页参数无效');
    final response = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memories',
      queryParameters: {
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        'query': '',
        'mode': 'keyword',
        'memoryView': 'all',
        if (sort != null && sort.isNotEmpty) 'sort': sort,
        'cursor': cursor,
        'limit': limit,
      },
    );
    if (response == null || response['resources'] is! List)
      throw StateError('记忆服务返回无效数据');
    final rows = <MemoryDto>[];
    for (final resource in (response['resources'] as List).whereType<Map>()) {
      if (resource['kind'] != 'memory' || resource['body'] is! Map)
        throw StateError('记忆资源类型无效');
      final body = Map<String, dynamic>.from(resource['body'] as Map);
      final content = body['content'];
      final metadata = content is Map
          ? Map<String, dynamic>.from(content)
          : <String, dynamic>{'value': content ?? ''};
      rows.add(
        MemoryDto.fromJson({
          ...metadata,
          ...body,
          'id': resource['id'],
          'characterId': resource['roleId'],
          'scope': resource['roleId'] == '@global-memory'
              ? 'user'
              : 'character',
          'value':
              metadata['value'] ??
              metadata['text'] ??
              metadata['summary'] ??
              '',
          'updatedAt': resource['updatedAt'] ?? '',
          'memoryStrength': body['effectiveStrength'] ?? body['memoryStrength'],
          'retentionLevel':
              body['effectiveRetentionLevel'] ?? body['retentionLevel'],
          'decayState': body['effectiveDecayState'] ?? body['decayState'],
        }),
      );
    }
    return MemoryPage(
      items: List<MemoryDto>.unmodifiable(rows),
      nextCursor: (response['nextCursor'] ?? '').toString(),
    );
  }

  Future<List<MemoryCandidateDto>> listCandidates({String? characterId}) async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memory-candidates',
      queryParameters: {
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
      },
    );
    if (response == null || response['resources'] is! List)
      throw StateError('候选记忆服务返回无效数据');
    return (response['resources'] as List)
        .whereType<Map>()
        .map((resource) {
          final body = Map<String, dynamic>.from(resource['body'] as Map);
          return MemoryCandidateDto.fromJson({
            ...body,
            'id': resource['id'],
            'characterId': resource['roleId'],
          });
        })
        .toList(growable: false);
  }
}
