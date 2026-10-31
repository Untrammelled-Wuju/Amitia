import 'dart:math';

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
        .map(
          (item) =>
              KnowledgeSourceDto.fromJson(Map<String, dynamic>.from(item)),
        )
        .toList(growable: false);
  }

  Future<Map<String, dynamic>?> upload({
    required String filePath,
    String logicalName = '',
  }) {
    return _api.postMultipart<Map<String, dynamic>>(
      '/api/knowledge/sources',
      fields: logicalName.trim().isEmpty
          ? const {}
          : {'logicalName': logicalName.trim()},
      files: {
        'file': [filePath],
      },
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
      files: {
        'file': [filePath],
      },
    );
  }

  Future<KnowledgeJobDto?> reindexSource(String id) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/sources/${Uri.encodeComponent(id)}/reindex',
    );
    return resp == null ? null : KnowledgeJobDto.fromJson(resp);
  }

  Future<KnowledgeJobDto?> rollbackRevision(
    String sourceId,
    int revision,
  ) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/sources/${Uri.encodeComponent(sourceId)}/revisions/$revision/rollback',
    );
    return resp == null ? null : KnowledgeJobDto.fromJson(resp);
  }

  Future<KnowledgeSourceRevisionDto?> purgeRevision(
    String sourceId,
    int revision,
  ) async {
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
      queryParameters: sourceId.trim().isEmpty
          ? null
          : {'sourceId': sourceId.trim()},
    );
    final items = resp?['items'];
    if (items is! List) return const [];
    return items
        .whereType<Map>()
        .map(
          (item) => KnowledgeJobDto.fromJson(Map<String, dynamic>.from(item)),
        )
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

  Future<KnowledgeJobDto> getJob(String id) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/jobs/${Uri.encodeComponent(id)}',
    );
    if (resp == null) throw StateError('知识库任务不存在');
    return KnowledgeJobDto.fromJson(resp);
  }

  Future<KnowledgeSearchResultDto> search(
    String query, {
    int limit = 8,
    List<String> sourceIds = const [],
    double minScore = 0,
    String retrievalLevel = 'auto',
    String? routingMode,
    String contextualQuery = '',
    String temporalMode = 'current',
    DateTime? asOf,
  }) async {
    final value = query.trim();
    if (value.isEmpty) {
      return KnowledgeSearchResultDto(
        evidence: [],
        sufficient: false,
        answerState: 'insufficient',
        sufficiencyReason: 'no_query',
        query: '',
        originalQuery: '',
        effectiveQuery: '',
        queryComplexity: 'simple',
        requestedRetrievalLevel: 'auto',
        retrievalLevel: 'none',
        routingMode: routingMode ?? 'chat',
        policyMode: '',
        routeReason: 'no_query',
        shadowLevel: '',
        shadowReason: '',
        routeDegraded: false,
        routeDegradationReason: '',
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
        'routingMode':
            routingMode ?? (retrievalLevel == 'auto' ? 'chat' : 'diagnostic'),
        if (contextualQuery.trim().isNotEmpty)
          'contextualQuery': contextualQuery.trim(),
        'temporalMode': temporalMode,
        if (temporalMode == 'historical' && asOf != null)
          'asOf': asOf.toUtc().toIso8601String(),
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

  Future<KnowledgeRuntimeStatusDto> runtimeStatus() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/knowledge/runtime');
    return KnowledgeRuntimeStatusDto.fromJson(resp ?? const {});
  }

  Future<KnowledgeIndexHealthDto> indexHealth() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/index-health',
    );
    return KnowledgeIndexHealthDto.fromJson(resp ?? const {});
  }

  Future<KnowledgeVectorAuditDto> auditVectors({int limit = 50}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-audit',
      queryParameters: {'limit': limit},
    );
    return KnowledgeVectorAuditDto.fromJson(resp ?? const {});
  }

  Future<KnowledgeOrphanAuditDto> auditOrphans({int limit = 50}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans',
      queryParameters: {'limit': limit},
    );
    return KnowledgeOrphanAuditDto.fromJson(resp ?? const {});
  }

  Future<KnowledgeOrphanAuditDto> auditOrphanPage({
    String cursor = '',
    int limit = 25,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/page',
      queryParameters: {
        'limit': limit,
        if (cursor.isNotEmpty) 'cursor': cursor,
      },
    );
    return KnowledgeOrphanAuditDto.fromJson(resp ?? const {});
  }

  Future<List<KnowledgeOrphanAuditDto>> orphanAuditSessions({
    int limit = 10,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/audits',
      queryParameters: {'limit': limit},
    );
    return (resp?['items'] as List? ?? const [])
        .whereType<Map>()
        .map(
          (item) =>
              KnowledgeOrphanAuditDto.fromJson(Map<String, dynamic>.from(item)),
        )
        .toList(growable: false);
  }

  Future<KnowledgeOrphanAuditDto> recheckOrphanLatest(String auditId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/audits/${Uri.encodeComponent(auditId)}/recheck',
    );
    return KnowledgeOrphanAuditDto.fromJson(resp ?? const {});
  }

  Future<Map<String, dynamic>> orphanAuditReceipts(
    String auditId, {
    int after = 0,
    int limit = 25,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/audits/${Uri.encodeComponent(auditId)}/pages',
      queryParameters: {'after': after, 'limit': limit},
    );
    return resp ?? const {};
  }

  Future<List<Map<String, dynamic>>> orphanRecheckHistory(
    String auditId, {
    int limit = 10,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/audits/${Uri.encodeComponent(auditId)}/rechecks',
      queryParameters: {'limit': limit},
    );
    return (resp?['items'] as List? ?? const [])
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<KnowledgeSloSummaryDto> sloSummary() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/knowledge/slo');
    return KnowledgeSloSummaryDto.fromJson(resp ?? const {});
  }

  Future<List<KnowledgeRetrievalTraceDto>> traces({int limit = 20}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/traces',
      queryParameters: {'limit': limit},
    );
    return (resp?['items'] as List? ?? const [])
        .whereType<Map>()
        .map(
          (item) => KnowledgeRetrievalTraceDto.fromJson(
            Map<String, dynamic>.from(item),
          ),
        )
        .toList(growable: false);
  }

  Future<List<KnowledgeBenchmarkRunDto>> benchmarkRuns({int limit = 20}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/benchmark/runs',
      queryParameters: {'limit': limit},
    );
    return (resp?['items'] as List? ?? const [])
        .whereType<Map>()
        .map(
          (item) => KnowledgeBenchmarkRunDto.fromJson(
            Map<String, dynamic>.from(item),
          ),
        )
        .toList(growable: false);
  }

  Future<KnowledgeOutlineResponseDto> sourceOutline({
    required String sourceId,
    required int revision,
    int? afterOrdinal,
    int limit = 100,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/sources/${Uri.encodeComponent(sourceId)}/outline',
      queryParameters: {
        'revision': revision,
        'afterOrdinal': ?afterOrdinal,
        'limit': limit,
      },
    );
    return KnowledgeOutlineResponseDto.fromJson(resp ?? const {});
  }

  Future<List<KnowledgeRecoveryReviewDto>> recoveryReviews(
    String auditId,
  ) async {
    final headers = await _recoveryHeaders();
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/audits/${Uri.encodeComponent(auditId)}/recovery-reviews',
      queryParameters: {'limit': 10},
      headers: headers,
    );
    if (resp?['executable'] != false) {
      throw StateError('人工审查列表响应无效');
    }
    return (resp?['items'] as List? ?? const [])
        .whereType<Map>()
        .map(
          (item) => KnowledgeRecoveryReviewDto.fromJson(
            Map<String, dynamic>.from(item),
          ),
        )
        .where((item) => item.auditId == auditId)
        .toList(growable: false);
  }

  Future<Map<String, dynamic>> recoveryReviewEvents(String reviewId) async {
    final headers = await _recoveryHeaders();
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/recovery-reviews/${Uri.encodeComponent(reviewId)}/events',
      headers: headers,
    );
    if (resp?['executable'] != false) throw StateError('人工审查轨迹响应无效');
    return resp ?? const {};
  }

  Future<String> recoveryAuthorityKey() async {
    final headers = await _recoveryHeaders();
    return '${headers['X-Amitia-Expected-Core-ID']}|${headers['X-Amitia-Expected-Configuration-Policy']}';
  }

  Future<Map<String, dynamic>> recoveryReviewPlan(String reviewId) async {
    final headers = await _recoveryHeaders();
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/recovery-reviews/${Uri.encodeComponent(reviewId)}/plan',
      headers: headers,
    );
    if (resp?['executable'] != false) throw StateError('人工审查计划响应无效');
    return resp ?? const {};
  }

  Future<void> requestRecoveryReview({
    required String auditId,
    required String recheckId,
    required String requestKey,
  }) async {
    final headers = await _recoveryHeaders();
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/audits/${Uri.encodeComponent(auditId)}/recovery-reviews',
      data: {'recheckId': recheckId, 'requestKey': requestKey},
      headers: headers,
    );
    if (resp?['executable'] != false) throw StateError('人工审查申请响应无效');
  }

  Future<void> transitionRecoveryReview({
    required String reviewId,
    required String action,
  }) async {
    final headers = await _recoveryHeaders();
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/knowledge/vector-orphans/recovery-reviews/${Uri.encodeComponent(reviewId)}/${Uri.encodeComponent(action)}',
      headers: headers,
    );
    if (resp?['executable'] != false) throw StateError('人工审查状态响应无效');
  }

  Future<Map<String, String>> _recoveryHeaders() async {
    final state = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    final policy = Map<String, dynamic>.from(
      state?['policy'] is Map ? state!['policy'] as Map : const {},
    );
    final coreId = (state?['coreId'] ?? '').toString();
    final versions = [
      policy['providerEpoch'],
      policy['modeRevision'],
      policy['permissionRevision'],
    ];
    if (coreId.isEmpty ||
        state?['canAdminister'] != true ||
        policy['coordinated'] != true ||
        policy['administrator'] != true ||
        versions.any((value) => value is! int || value < 1)) {
      throw StateError('当前设备不具备 Core 知识恢复审查权限');
    }
    return {
      'X-Amitia-Expected-Core-ID': coreId,
      'X-Amitia-Expected-Configuration-Policy':
          '${policy['providerEpoch']}:${policy['modeRevision']}:${policy['permissionRevision']}',
    };
  }

  static String newReviewRequestKey() {
    final random = Random.secure();
    final bytes = List<int>.generate(16, (_) => random.nextInt(256));
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    final hex = bytes
        .map((value) => value.toRadixString(16).padLeft(2, '0'))
        .join();
    return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}';
  }
}
