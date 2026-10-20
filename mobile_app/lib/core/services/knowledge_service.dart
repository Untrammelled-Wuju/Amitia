import '../backend_transport/backend_service_api.dart';
import '../models/knowledge.dart';

class KnowledgeService {
  KnowledgeService(this._api);

  final BackendServiceApi _api;

  Future<List<KnowledgeSourceDto>> listSources() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/knowledge/sources');
    final items = resp?['items'];
    if (items is! List) return const [];
    return items
        .whereType<Map>()
        .map((item) => KnowledgeSourceDto.fromJson(Map<String, dynamic>.from(item)))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>?> upload({
    required String filePath,
    String logicalName = '',
  }) {
    return _api.postMultipart<Map<String, dynamic>>(
      '/api/knowledge/sources',
      fields: logicalName.trim().isEmpty ? const {} : {'logicalName': logicalName.trim()},
      files: {'file': [filePath]},
    );
  }


  Future<KnowledgeSourceDetailDto> getSourceDetail(String id) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/sources/${Uri.encodeComponent(id)}',
    );
    return KnowledgeSourceDetailDto.fromJson(resp ?? const {});
  }

  Future<Map<String, dynamic>?> uploadRevision({
    required String sourceId,
    required String filePath,
  }) {
    return _api.postMultipart<Map<String, dynamic>>(
      '/api/knowledge/sources/${Uri.encodeComponent(sourceId)}/revisions',
      files: {'file': [filePath]},
    );
  }

  Future<KnowledgeJobDto?> reindexSource(String id) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/sources/${Uri.encodeComponent(id)}/reindex',
    );
    return resp == null ? null : KnowledgeJobDto.fromJson(resp);
  }

  Future<KnowledgeJobDto?> rollbackRevision(String sourceId, int revision) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/sources/${Uri.encodeComponent(sourceId)}/revisions/$revision/rollback',
    );
    return resp == null ? null : KnowledgeJobDto.fromJson(resp);
  }

  Future<KnowledgeSourceRevisionDto?> purgeRevision(String sourceId, int revision) async {
    final resp = await _api.deleteWithResponse<Map<String, dynamic>>(
      '/api/knowledge/sources/${Uri.encodeComponent(sourceId)}/revisions/$revision',
    );
    return resp == null ? null : KnowledgeSourceRevisionDto.fromJson(resp);
  }

  Future<void> deleteSource(String id) {
    return _api.delete('/api/knowledge/sources/${Uri.encodeComponent(id)}');
  }

  Future<List<KnowledgeJobDto>> listJobs({String sourceId = ''}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/jobs',
      queryParameters: sourceId.trim().isEmpty ? null : {'sourceId': sourceId.trim()},
    );
    final items = resp?['items'];
    if (items is! List) return const [];
    return items
        .whereType<Map>()
        .map((item) => KnowledgeJobDto.fromJson(Map<String, dynamic>.from(item)))
        .toList(growable: false);
  }

  Future<KnowledgeJobDto?> retryJob(String id) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/jobs/${Uri.encodeComponent(id)}/retry',
    );
    return resp == null ? null : KnowledgeJobDto.fromJson(resp);
  }

  Future<KnowledgeJobDto?> cancelJob(String id) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/jobs/${Uri.encodeComponent(id)}/cancel',
    );
    return resp == null ? null : KnowledgeJobDto.fromJson(resp);
  }

  Future<KnowledgeSearchResultDto> search(
    String query, {
    int limit = 8,
    List<String> sourceIds = const [],
    double minScore = 0,
    String retrievalLevel = 'auto',
    String contextualQuery = '',
    String temporalMode = 'current',
    DateTime? asOf,
  }) async {
    final value = query.trim();
    if (value.isEmpty) {
      return const KnowledgeSearchResultDto(
        evidence: [],
        sufficient: false,
        answerState: 'insufficient',
        sufficiencyReason: 'no_query',
        query: '',
        originalQuery: '',
        effectiveQuery: '',
        queryComplexity: 'simple',
        retrievalLevel: 'none',
        queryHistory: [],
        degraded: false,
        temporalMode: 'current',
        asOf: '',
        ambiguous: false,
        conflictGroups: [],
        warnings: [],
      );
    }
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/search',
      data: {
        'query': value,
        'limit': limit,
        'sourceIds': sourceIds,
        'minScore': minScore,
        'retrievalLevel': retrievalLevel,
        if (contextualQuery.trim().isNotEmpty) 'contextualQuery': contextualQuery.trim(),
        'temporalMode': temporalMode,
        if (temporalMode == 'historical' && asOf != null) 'asOf': asOf.toUtc().toIso8601String(),
      },
    );
    return KnowledgeSearchResultDto.fromJson(resp ?? const {});
  }

  Future<KnowledgeReadResponseDto> read(KnowledgeReadRequestDto request) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/read',
      data: request.toJson(),
    );
    return KnowledgeReadResponseDto.fromJson(resp ?? const {});
  }

  Future<void> reconcile() async {
    await _api.post<Object?>('/api/knowledge/reconcile');
  }

  Future<int> garbageCollect() async {
    final resp = await _api.post<Map<String, dynamic>>('/api/knowledge/gc');
    return (resp?['purged'] as num?)?.toInt() ?? 0;
  }
}
