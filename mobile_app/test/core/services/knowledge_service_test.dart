import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/knowledge_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _KnowledgeApi extends Fake implements BackendServiceApi {
  String? getPath;
  String? postPath;
  Object? postData;
  Map<String, dynamic>? getQuery;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    getPath = path;
    getQuery = queryParameters;
    if (path == '/api/device-mesh/v1/coordination/me') {
      final state = <String, dynamic>{
        'coreId': 'core-a',
        'canAdminister': true,
        'policy': <String, dynamic>{
          'coordinated': true,
          'administrator': true,
          'providerEpoch': 1,
          'modeRevision': 2,
          'permissionRevision': 3,
        },
      };
      return state as T;
    }
    final value = <String, dynamic>{
      'configHash': 'config-a',
      'retrievalPolicyEnabled': true,
      'retrievalPolicyShadow': true,
      'canonicalReaderEnabled': true,
      'agenticInvestigationEnabled': true,
      'status': 'healthy',
      'levels': <dynamic>[],
      'items': <dynamic>[],
    };
    return value as T;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    postPath = path;
    postData = data;
    final value = <String, dynamic>{
      'evidence': <dynamic>[],
      'sufficient': false,
      'answerState': 'insufficient',
      'requestedRetrievalLevel': 'auto',
      'retrievalLevel': 'agentic',
      'routingMode': 'chat',
      'policyMode': 'active',
      'routeReason': 'chat_document_intent',
      'routeDegraded': false,
      'degraded': false,
    };
    return value as T;
  }
}

void main() {
  test('自动知识检索使用聊天服务端有效路由', () async {
    final api = _KnowledgeApi();
    final service = KnowledgeService(api);
    final result = await service.search('核对 PDF 第37页原文');
    expect(api.postPath, '/api/knowledge/search');
    expect((api.postData as Map<String, dynamic>)['routingMode'], 'chat');
    expect(result.routingMode, 'chat');
    expect(result.retrievalLevel, 'agentic');
    expect(result.policyMode, 'active');
  });

  test('手动检索级别明确走诊断路由', () async {
    final api = _KnowledgeApi();
    final service = KnowledgeService(api);
    await service.search('核对 PDF', retrievalLevel: 'deep');
    expect((api.postData as Map<String, dynamic>)['routingMode'], 'diagnostic');
  });

  test('运行时、索引健康和诊断列表复用统一知识接口', () async {
    final api = _KnowledgeApi();
    final service = KnowledgeService(api);
    await service.runtimeStatus();
    expect(api.getPath, '/api/knowledge/runtime');
    await service.indexHealth();
    expect(api.getPath, '/api/knowledge/index-health');
    await service.traces(limit: 12);
    expect(api.getPath, '/api/knowledge/traces');
    expect(api.getQuery, {'limit': 12});
  });

  test('恢复审查使用当前 Core 与管理员权限版本', () async {
    final api = _KnowledgeApi();
    final service = KnowledgeService(api);
    expect(await service.recoveryAuthorityKey(), 'core-a|1:2:3');
  });
}
